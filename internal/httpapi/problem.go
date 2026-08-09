package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/requestctx"
)

const ProblemMediaType = "application/problem+json"

const (
	TypeInvalidRequest         = "https://ecommerce.local/problems/invalid-request"
	TypeAuthenticationRequired = "https://ecommerce.local/problems/authentication-required"
	TypeForbidden              = "https://ecommerce.local/problems/forbidden"
	TypeNotFound               = "https://ecommerce.local/problems/not-found"
	TypeConflict               = "https://ecommerce.local/problems/conflict"
	TypeValidation             = "https://ecommerce.local/problems/validation"
	TypeInternal               = "https://ecommerce.local/problems/internal"
)

// FieldError is a stable, machine-readable validation issue.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

// Problem is the boundary-owned RFC 9457 representation used until the same
// schema is generated from OpenAPI. Its JSON field names intentionally match
// the documented target contract so AdaptContract can consume a future
// apicontract.Problem without a source change here.
type Problem struct {
	Type             string       `json:"type"`
	Title            string       `json:"title"`
	Status           int          `json:"status"`
	Detail           string       `json:"detail,omitempty"`
	Instance         string       `json:"instance,omitempty"`
	Code             string       `json:"code"`
	CorrelationID    string       `json:"correlation_id,omitempty"`
	Errors           []FieldError `json:"errors,omitempty"`
	LegacyError      string       `json:"error,omitempty"`
	ProductVariantID uint         `json:"product_variant_id,omitempty"`
	Requested        int          `json:"requested,omitempty"`
	Available        int          `json:"available,omitempty"`
}

func (p Problem) Error() string {
	if p.Detail != "" {
		return p.Detail
	}
	return p.Title
}

// ProblemError lets services or strict handlers return a safe problem plus an
// optional internal cause. The cause is available to reporting but is never
// serialized.
type ProblemError struct {
	Problem Problem
	Cause   error
}

func (e *ProblemError) Error() string {
	if e == nil {
		return ""
	}
	return e.Problem.Error()
}

func (e *ProblemError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func ErrorProblem(problem Problem, cause error) error {
	return &ProblemError{Problem: problem, Cause: cause}
}

// AdaptContract converts the current generated legacy Error and any future
// generated Problem with RFC 9457 JSON fields into the boundary representation.
func AdaptContract(value any, fallbackStatus int) (Problem, error) {
	switch value := value.(type) {
	case Problem:
		return normalizeProblem(value, fallbackStatus), nil
	case *Problem:
		if value == nil {
			return Problem{}, errors.New("nil problem")
		}
		return normalizeProblem(*value, fallbackStatus), nil
	case apicontract.Error:
		return adaptLegacyError(value, fallbackStatus), nil
	case *apicontract.Error:
		if value == nil {
			return Problem{}, errors.New("nil contract error")
		}
		return adaptLegacyError(*value, fallbackStatus), nil
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return Problem{}, fmt.Errorf("marshal generated problem: %w", err)
	}
	var problem Problem
	if err := json.Unmarshal(encoded, &problem); err != nil {
		return Problem{}, fmt.Errorf("decode generated problem: %w", err)
	}
	if problem.Type == "" && problem.Title == "" && problem.Code == "" && problem.Status == 0 {
		return Problem{}, fmt.Errorf("%T does not have problem-details fields", value)
	}
	return normalizeProblem(problem, fallbackStatus), nil
}

func adaptLegacyError(legacy apicontract.Error, status int) Problem {
	problem := defaultProblem(status)
	problem.Detail = legacy.Error
	if legacy.Code != nil && *legacy.Code != "" {
		problem.Code = *legacy.Code
	}
	return problem
}

func normalizeProblem(problem Problem, fallbackStatus int) Problem {
	defaults := defaultProblem(fallbackStatus)
	if problem.Status == 0 {
		problem.Status = defaults.Status
	}
	statusDefaults := defaultProblem(problem.Status)
	if problem.Type == "" {
		problem.Type = statusDefaults.Type
	}
	if problem.Title == "" {
		problem.Title = statusDefaults.Title
	}
	if problem.Code == "" {
		problem.Code = statusDefaults.Code
	}
	return problem
}

func defaultProblem(status int) Problem {
	if status < 400 || status > 599 {
		status = http.StatusInternalServerError
	}
	problem := Problem{Status: status, Title: http.StatusText(status)}
	switch status {
	case http.StatusBadRequest:
		problem.Type, problem.Code = TypeInvalidRequest, "invalid_request"
	case http.StatusUnauthorized:
		problem.Type, problem.Code = TypeAuthenticationRequired, "authentication_required"
	case http.StatusForbidden:
		problem.Type, problem.Code = TypeForbidden, "forbidden"
	case http.StatusNotFound:
		problem.Type, problem.Code = TypeNotFound, "not_found"
	case http.StatusConflict:
		problem.Type, problem.Code = TypeConflict, "state_conflict"
	case http.StatusUnprocessableEntity:
		problem.Type, problem.Code = TypeValidation, "validation_failed"
	default:
		problem.Type, problem.Code = TypeInternal, "internal_error"
	}
	return problem
}

// Renderer centralizes safe problem creation, reporting, and serialization.
type Renderer struct {
	Report func(context.Context, error, Problem)
}

func (renderer Renderer) FromError(ctx context.Context, status int, err error) Problem {
	var problemError *ProblemError
	var problem Problem
	if errors.As(err, &problemError) {
		problem = normalizeProblem(problemError.Problem, status)
	} else {
		problem = defaultProblem(status)
		if problem.Status < http.StatusInternalServerError {
			problem.Detail = safeClientDetail(problem.Status)
		} else {
			problem.Detail = "An unexpected error occurred."
		}
	}

	problem = enrichProblem(ctx, problem)
	if renderer.Report != nil && err != nil {
		renderer.Report(ctx, err, problem)
	}
	return problem
}

func (renderer Renderer) Render(w http.ResponseWriter, ctx context.Context, status int, err error) {
	renderer.Write(w, renderer.FromError(ctx, status, err))
}

// RenderContract adapts and writes either the legacy generated Error or a
// generated RFC 9457 Problem shape. Adaptation failures become a safe internal
// problem and are returned for observability.
func (renderer Renderer) RenderContract(w http.ResponseWriter, ctx context.Context, status int, value any) error {
	problem, err := AdaptContract(value, status)
	if err != nil {
		renderer.Render(w, ctx, http.StatusInternalServerError, err)
		return err
	}
	renderer.Write(w, enrichProblem(ctx, problem))
	return nil
}

func (renderer Renderer) Write(w http.ResponseWriter, problem Problem) {
	problem = normalizeProblem(problem, problem.Status)
	w.Header().Set("Content-Type", ProblemMediaType)
	w.WriteHeader(problem.Status)
	_ = json.NewEncoder(w).Encode(problem)
}

func enrichProblem(ctx context.Context, problem Problem) Problem {
	if metadata, ok := requestctx.MetadataFrom(ctx); ok {
		if problem.CorrelationID == "" {
			problem.CorrelationID = metadata.CorrelationID
		}
		if problem.Instance == "" && metadata.RequestID != "" {
			problem.Instance = "urn:request:" + metadata.RequestID
		}
	}
	return problem
}

func safeClientDetail(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "The request could not be parsed or contains invalid parameters."
	case http.StatusUnauthorized:
		return "Authentication is required."
	case http.StatusForbidden:
		return "The authenticated principal is not allowed to perform this operation."
	case http.StatusNotFound:
		return "The requested resource was not found."
	case http.StatusConflict:
		return "The request conflicts with the current resource state."
	case http.StatusUnprocessableEntity:
		return "The request failed validation."
	default:
		text := strings.TrimSpace(http.StatusText(status))
		if text == "" {
			return "The request could not be completed."
		}
		return text + "."
	}
}
