package search

import (
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"regexp"
	"strings"
)

var diagnosticQuoted = regexp.MustCompile(`'[^']*'|"[^"]*"`)
var diagnosticURL = regexp.MustCompile(`(?i)(postgres(?:ql)?|https?)://[^\s]+`)
var diagnosticCredential = regexp.MustCompile(`(?i)\b(password|token|secret|authorization)\s*[=:]\s*[^\s,;]+`)

// ErrorDiagnostic supplies an actionable server-side error message while
// omitting SQL values, driver details/hints, URLs and credential assignments.
// Never use it as a public error response.
func ErrorDiagnostic(err error) string {
	if err == nil {
		return ""
	}
	var pg *pgconn.PgError
	message := err.Error()
	if errors.As(err, &pg) {
		message = fmt.Sprintf("SQLSTATE %s: %s", pg.Code, pg.Message)
	}
	message = diagnosticURL.ReplaceAllString(message, "[redacted connection]")
	message = diagnosticQuoted.ReplaceAllString(message, "[redacted value]")
	message = diagnosticCredential.ReplaceAllString(message, "$1=[redacted]")
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > 512 {
		message = message[:512] + "…"
	}
	return message
}
