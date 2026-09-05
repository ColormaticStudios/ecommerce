package reliability

import (
	"errors"
	"fmt"
)

type ErrorClass string

const (
	ClassRetryable ErrorClass = "retryable"
	ClassTerminal  ErrorClass = "terminal"
	ClassDegraded  ErrorClass = "degraded"
)

type classifiedError struct {
	class ErrorClass
	err   error
}

func (e classifiedError) Error() string     { return e.err.Error() }
func (e classifiedError) Unwrap() error     { return e.err }
func (e classifiedError) Class() ErrorClass { return e.class }

// Classify annotates an error for retry and alert policy while preserving its
// errors.Is/errors.As chain. A nil error remains nil. Unknown classes safely
// fall back to terminal so invalid runtime input can never trigger retries or
// crash a worker.
func Classify(err error, class ErrorClass) error {
	if err == nil {
		return nil
	}
	switch class {
	case ClassRetryable, ClassTerminal, ClassDegraded:
		return classifiedError{class: class, err: err}
	default:
		return classifiedError{class: ClassTerminal, err: fmt.Errorf("unknown reliability error class %q: %w", class, err)}
	}
}

// ErrorClassOf returns the outermost reliability classification in an error
// chain. Unclassified errors return ok=false.
func ErrorClassOf(err error) (class ErrorClass, ok bool) {
	var classified interface{ Class() ErrorClass }
	if !errors.As(err, &classified) {
		return "", false
	}
	return classified.Class(), true
}
