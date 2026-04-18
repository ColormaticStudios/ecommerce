package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

type outputFormat string

const (
	outputFormatText outputFormat = "text"
	outputFormatJSON outputFormat = "json"
)

type localHandlerRequest struct {
	Method     string
	Path       string
	Body       any
	PathParams map[string]string
	Subject    string
}

type handlerErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func addOutputFormatFlag(cmd *cobra.Command, format *string, defaultValue string) {
	cmd.Flags().StringVar(format, "format", defaultValue, "Output format: text or json")
}

func normalizeOutputFormat(raw string) (outputFormat, error) {
	switch outputFormat(strings.TrimSpace(strings.ToLower(raw))) {
	case outputFormatText:
		return outputFormatText, nil
	case outputFormatJSON:
		return outputFormatJSON, nil
	default:
		return "", fmt.Errorf("invalid output format %q: expected text or json", raw)
	}
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func printJSON(value any) {
	if err := writeJSON(os.Stdout, value); err != nil {
		log.Fatalf("failed to encode JSON: %v", err)
	}
}

func newMediaService() *media.Service {
	cfg := getConfig()
	svc := media.NewService(getDBWithConfig(cfg), cfg.MediaRoot, cfg.MediaPublicURL, log.Default())
	if err := svc.EnsureDirs(); err != nil {
		log.Fatalf("Failed to initialize media directories: %v", err)
	}
	return svc
}

func closeMediaService(svc *media.Service) {
	if svc == nil || svc.DB == nil {
		return
	}
	closeDB(svc.DB)
}

func invokeLocalHandler(handler gin.HandlerFunc, req localHandlerRequest) (int, []byte, error) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	var body io.Reader = http.NoBody
	if req.Body != nil {
		payload, err := json.Marshal(req.Body)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(payload)
	}

	httpReq := httptest.NewRequest(req.Method, req.Path, body)
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	ctx.Request = httpReq

	if req.Subject != "" {
		ctx.Set("userID", req.Subject)
	}
	for key, value := range req.PathParams {
		ctx.Params = append(ctx.Params, gin.Param{Key: key, Value: value})
	}

	handler(ctx)
	return recorder.Code, recorder.Body.Bytes(), nil
}

func invokeLocalJSON[T any](handler gin.HandlerFunc, req localHandlerRequest) (T, error) {
	var zero T

	status, body, err := invokeLocalHandler(handler, req)
	if err != nil {
		return zero, err
	}
	if status >= http.StatusBadRequest {
		return zero, decodeHandlerError(status, body)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return zero, nil
	}

	var value T
	if err := json.Unmarshal(body, &value); err != nil {
		return zero, fmt.Errorf("decode handler response: %w", err)
	}
	return value, nil
}

func invokeLocalSuccess(handler gin.HandlerFunc, req localHandlerRequest) error {
	status, body, err := invokeLocalHandler(handler, req)
	if err != nil {
		return err
	}
	if status >= http.StatusBadRequest {
		return decodeHandlerError(status, body)
	}
	return nil
}

func decodeHandlerError(status int, body []byte) error {
	var payload handlerErrorResponse
	if err := json.Unmarshal(body, &payload); err == nil {
		switch {
		case strings.TrimSpace(payload.Error) != "":
			return fmt.Errorf("request failed (%d): %s", status, payload.Error)
		case strings.TrimSpace(payload.Message) != "":
			return fmt.Errorf("request failed (%d): %s", status, payload.Message)
		}
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		trimmed = http.StatusText(status)
	}
	return fmt.Errorf("request failed (%d): %s", status, trimmed)
}

func requireUser(db *gorm.DB, id uint, email string, username string) (models.User, error) {
	var user models.User
	var err error

	switch {
	case id != 0:
		err = db.First(&user, id).Error
	case strings.TrimSpace(email) != "":
		err = db.Where("email = ?", strings.TrimSpace(email)).First(&user).Error
	case strings.TrimSpace(username) != "":
		err = db.Where("username = ?", strings.TrimSpace(username)).First(&user).Error
	default:
		return user, errors.New("provide --user-id, --email, or --username")
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user, errors.New("user not found")
		}
		return user, err
	}
	return user, nil
}

func addUserSelectorFlags(cmd *cobra.Command, userID *uint, email *string, username *string) {
	cmd.Flags().UintVar(userID, "user-id", 0, "User ID")
	cmd.Flags().StringVar(email, "email", "", "User email address")
	cmd.Flags().StringVar(username, "username", "", "User username")
}

func parseBoolPointerSet(cmd *cobra.Command, name string, value bool) *bool {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	result := value
	return &result
}

func loadJSONFile(path string, target any) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode JSON from %s: %w", path, err)
	}
	return nil
}
