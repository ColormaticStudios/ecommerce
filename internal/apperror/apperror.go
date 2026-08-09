// Package apperror defines transport-neutral, classified application errors.
// HTTP adapters may map Kind values to status codes and Problem responses, but
// this package intentionally has no dependency on an HTTP framework.
package apperror

import "errors"

// Kind classifies an application failure without prescribing a transport status.
type Kind string

const (
	KindInvalidInput       Kind = "invalid_input"
	KindValidation         Kind = "validation"
	KindUnauthenticated    Kind = "unauthenticated"
	KindForbidden          Kind = "forbidden"
	KindNotFound           Kind = "not_found"
	KindConflict           Kind = "conflict"
	KindFailedPrecondition Kind = "failed_precondition"
	KindRateLimited        Kind = "rate_limited"
	KindUnavailable        Kind = "unavailable"
	KindTimeout            Kind = "timeout"
	KindInternal           Kind = "internal"
)

// Issue describes a request value that caused an application error. Path is a
// JSON Pointer (RFC 6901); an empty path identifies the request document root.
type Issue struct {
	Path   string
	Code   string
	Detail string
}

// Error is a safe application-facing error. Detail and Issues may be returned
// to a caller; the wrapped cause is retained for errors.Is/errors.As and logs.
type Error struct {
	Kind   Kind
	Code   string
	Detail string
	Issues []Issue
	cause  error
}

// New constructs a classified application error.
func New(kind Kind, code, detail string) *Error {
	return &Error{Kind: kind, Code: code, Detail: detail}
}

// Wrap constructs a classified application error while retaining its internal
// cause. Error deliberately does not include the cause text in its output.
func Wrap(kind Kind, code, detail string, cause error) *Error {
	return &Error{Kind: kind, Code: code, Detail: detail, cause: cause}
}

// Validation constructs a semantic validation error with field-level issues.
func Validation(code, detail string, issues ...Issue) *Error {
	return New(KindValidation, code, detail).WithIssues(issues...)
}

// WithIssues returns a copy of err containing a defensive copy of issues.
func (err *Error) WithIssues(issues ...Issue) *Error {
	if err == nil {
		return nil
	}
	copyErr := *err
	copyErr.Issues = append([]Issue(nil), issues...)
	return &copyErr
}

// Error returns only the safe application detail and never exposes the cause.
func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	if err.Detail != "" {
		return err.Detail
	}
	if err.Code != "" {
		return err.Code
	}
	return string(err.Kind)
}

// Unwrap exposes the internal cause to errors.Is and errors.As.
func (err *Error) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

// As returns the first classified application error in err's chain.
func As(err error) (*Error, bool) {
	var appErr *Error
	if !errors.As(err, &appErr) {
		return nil, false
	}
	return appErr, true
}

// KindOf returns the classified kind, defaulting unknown errors to KindInternal.
func KindOf(err error) Kind {
	if appErr, ok := As(err); ok {
		return appErr.Kind
	}
	return KindInternal
}

// CodeOf returns the stable application code, or an empty string for an
// unclassified error.
func CodeOf(err error) string {
	if appErr, ok := As(err); ok {
		return appErr.Code
	}
	return ""
}

// IssuesOf returns a defensive copy of any validation issues in err's chain.
func IssuesOf(err error) []Issue {
	if appErr, ok := As(err); ok {
		return append([]Issue(nil), appErr.Issues...)
	}
	return nil
}

// IsKind reports whether err contains a classified application error of kind.
func IsKind(err error, kind Kind) bool {
	return KindOf(err) == kind
}
