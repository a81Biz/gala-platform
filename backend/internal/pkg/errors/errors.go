// Package errors provides error handling utilities for GALA platform.
// Includes error wrapping with context, stack traces, and error codes.
package errors

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// Code represents an error code for categorization.
type Code string

// Error codes for the platform.
const (
	CodeInternal       Code = "INTERNAL_ERROR"
	CodeValidation     Code = "VALIDATION_ERROR"
	CodeNotFound       Code = "NOT_FOUND"
	CodeConflict       Code = "CONFLICT"
	CodeUnauthorized   Code = "UNAUTHORIZED"
	CodeForbidden      Code = "FORBIDDEN"
	CodeTimeout        Code = "TIMEOUT"
	CodeUnavailable    Code = "UNAVAILABLE"
	CodeBadRequest     Code = "BAD_REQUEST"
	CodeAlreadyExists  Code = "ALREADY_EXISTS"
	CodeFailedPrecond  Code = "FAILED_PRECONDITION"
	CodeResourceExhaust Code = "RESOURCE_EXHAUSTED"
)

// Error is a custom error type with additional context.
type Error struct {
	// Code is the error code for categorization.
	Code Code
	// Message is the human-readable error message.
	Message string
	// Op is the operation that failed (e.g., "job.create").
	Op string
	// Err is the underlying error.
	Err error
	// Fields contains additional context fields.
	Fields map[string]any
	// Stack contains the stack trace at error creation.
	Stack []Frame
}

// Frame represents a single stack frame.
type Frame struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	var b strings.Builder

	if e.Op != "" {
		b.WriteString(e.Op)
		b.WriteString(": ")
	}

	if e.Code != "" {
		b.WriteString("[")
		b.WriteString(string(e.Code))
		b.WriteString("] ")
	}

	b.WriteString(e.Message)

	if e.Err != nil {
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}

	return b.String()
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Err
}

// Is reports whether target matches this error.
func (e *Error) Is(target error) bool {
	if t, ok := target.(*Error); ok {
		return e.Code == t.Code
	}
	return false
}

// WithField adds a field to the error.
func (e *Error) WithField(key string, value any) *Error {
	if e.Fields == nil {
		e.Fields = make(map[string]any)
	}
	e.Fields[key] = value
	return e
}

// WithFields adds multiple fields to the error.
func (e *Error) WithFields(fields map[string]any) *Error {
	if e.Fields == nil {
		e.Fields = make(map[string]any)
	}
	for k, v := range fields {
		e.Fields[k] = v
	}
	return e
}

// HTTPStatus returns the appropriate HTTP status code for this error.
func (e *Error) HTTPStatus() int {
	switch e.Code {
	case CodeValidation, CodeBadRequest:
		return 400
	case CodeUnauthorized:
		return 401
	case CodeForbidden:
		return 403
	case CodeNotFound:
		return 404
	case CodeConflict, CodeAlreadyExists:
		return 409
	case CodeFailedPrecond:
		return 412
	case CodeResourceExhaust:
		return 429
	case CodeTimeout:
		return 504
	case CodeUnavailable:
		return 503
	default:
		return 500
	}
}

// StackTrace returns the stack trace as a formatted string.
func (e *Error) StackTrace() string {
	if len(e.Stack) == 0 {
		return ""
	}

	var b strings.Builder
	for _, f := range e.Stack {
		fmt.Fprintf(&b, "  %s:%d %s\n", f.File, f.Line, f.Function)
	}
	return b.String()
}

// New creates a new error with the given code and message.
func New(code Code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Stack:   captureStack(2),
	}
}

// Newf creates a new error with formatted message.
func Newf(code Code, format string, args ...any) *Error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Stack:   captureStack(2),
	}
}

// Wrap wraps an existing error with additional context.
func Wrap(err error, op string, message string) *Error {
	if err == nil {
		return nil
	}

	// If it's already our error type, preserve the code
	var e *Error
	if errors.As(err, &e) {
		return &Error{
			Code:    e.Code,
			Message: message,
			Op:      op,
			Err:     err,
			Fields:  e.Fields,
			Stack:   captureStack(2),
		}
	}

	return &Error{
		Code:    CodeInternal,
		Message: message,
		Op:      op,
		Err:     err,
		Stack:   captureStack(2),
	}
}

// Wrapf wraps an error with formatted message.
func Wrapf(err error, op string, format string, args ...any) *Error {
	return Wrap(err, op, fmt.Sprintf(format, args...))
}

// WrapWithCode wraps an error with a specific code.
func WrapWithCode(err error, code Code, op string, message string) *Error {
	if err == nil {
		return nil
	}

	return &Error{
		Code:    code,
		Message: message,
		Op:      op,
		Err:     err,
		Stack:   captureStack(2),
	}
}

// Internal creates an internal error.
func Internal(message string) *Error {
	return New(CodeInternal, message)
}

// Internalf creates an internal error with formatted message.
func Internalf(format string, args ...any) *Error {
	return Newf(CodeInternal, format, args...)
}

// NotFound creates a not found error.
func NotFound(resource string, id string) *Error {
	return New(CodeNotFound, fmt.Sprintf("%s not found: %s", resource, id)).
		WithField("resource", resource).
		WithField("id", id)
}

// Validation creates a validation error.
func Validation(message string) *Error {
	return New(CodeValidation, message)
}

// Validationf creates a validation error with formatted message.
func Validationf(format string, args ...any) *Error {
	return Newf(CodeValidation, format, args...)
}

// ValidationField creates a validation error for a specific field.
func ValidationField(field string, message string) *Error {
	return New(CodeValidation, message).WithField("field", field)
}

// Conflict creates a conflict error.
func Conflict(message string) *Error {
	return New(CodeConflict, message)
}

// AlreadyExists creates an already exists error.
func AlreadyExists(resource string, id string) *Error {
	return New(CodeAlreadyExists, fmt.Sprintf("%s already exists: %s", resource, id)).
		WithField("resource", resource).
		WithField("id", id)
}

// Timeout creates a timeout error.
func Timeout(operation string) *Error {
	return New(CodeTimeout, fmt.Sprintf("operation timed out: %s", operation)).
		WithField("operation", operation)
}

// Unavailable creates an unavailable error.
func Unavailable(service string) *Error {
	return New(CodeUnavailable, fmt.Sprintf("service unavailable: %s", service)).
		WithField("service", service)
}

// GetCode extracts the error code from an error.
func GetCode(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeInternal
}

// GetHTTPStatus extracts the HTTP status from an error.
func GetHTTPStatus(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.HTTPStatus()
	}
	return 500
}

// GetFields extracts fields from an error.
func GetFields(err error) map[string]any {
	var e *Error
	if errors.As(err, &e) && e.Fields != nil {
		return e.Fields
	}
	return nil
}

// IsCode checks if an error has a specific code.
func IsCode(err error, code Code) bool {
	return GetCode(err) == code
}

// IsNotFound checks if an error is a not found error.
func IsNotFound(err error) bool {
	return IsCode(err, CodeNotFound)
}

// IsValidation checks if an error is a validation error.
func IsValidation(err error) bool {
	return IsCode(err, CodeValidation)
}

// IsConflict checks if an error is a conflict error.
func IsConflict(err error) bool {
	return IsCode(err, CodeConflict) || IsCode(err, CodeAlreadyExists)
}

// captureStack captures the current stack trace.
func captureStack(skip int) []Frame {
	const maxDepth = 32
	var pcs [maxDepth]uintptr
	n := runtime.Callers(skip+1, pcs[:])

	frames := make([]Frame, 0, n)
	callersFrames := runtime.CallersFrames(pcs[:n])

	for {
		frame, more := callersFrames.Next()
		
		// Skip runtime frames
		if strings.Contains(frame.File, "runtime/") {
			if !more {
				break
			}
			continue
		}

		frames = append(frames, Frame{
			File:     frame.File,
			Line:     frame.Line,
			Function: frame.Function,
		})

		if !more || len(frames) >= 10 {
			break
		}
	}

	return frames
}

// As is a convenience wrapper for errors.As.
func As(err error, target any) bool {
	return errors.As(err, target)
}

// Is is a convenience wrapper for errors.Is.
func Is(err, target error) bool {
	return errors.Is(err, target)
}
