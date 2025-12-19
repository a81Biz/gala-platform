// Package middleware provides HTTP middleware for GALA platform.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"runtime/debug"
	"time"

	"gala/internal/pkg/errors"
	"gala/internal/pkg/logger"
)

// RequestIDHeader is the header name for request IDs.
const RequestIDHeader = "X-Request-ID"

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	size        int
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// RequestID adds a unique request ID to each request.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Add to response header
		w.Header().Set(RequestIDHeader, requestID)

		// Add to context
		ctx := logger.ContextWithRequestID(r.Context(), requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logging logs HTTP requests with structured logging.
func Logging(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := wrapResponseWriter(w)

			// Get request ID from context
			reqLog := log.FromContext(r.Context())

			// Log request start at debug level
			reqLog.Debug("request started",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)

			// Process request
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)

			// Determine log level based on status
			logFn := reqLog.Info
			if wrapped.status >= 500 {
				logFn = reqLog.Error
			} else if wrapped.status >= 400 {
				logFn = reqLog.Warn
			}

			// Log request completion
			logFn("request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.status,
				"size", wrapped.size,
				"duration_ms", duration.Milliseconds(),
			)
		})
	}
}

// Recovery recovers from panics and logs them.
func Recovery(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					// Get stack trace
					stack := debug.Stack()

					// Log the panic
					reqLog := log.FromContext(r.Context())
					reqLog.Error("panic recovered",
						"panic", rec,
						"stack", string(stack),
						"method", r.Method,
						"path", r.URL.Path,
					)

					// Return 500 error
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"internal server error"}}`))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// Timeout adds a timeout to requests.
func Timeout(duration time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), duration)
			defer cancel()

			// Create a channel to signal completion
			done := make(chan struct{})
			
			go func() {
				next.ServeHTTP(w, r.WithContext(ctx))
				close(done)
			}()

			select {
			case <-done:
				// Request completed normally
			case <-ctx.Done():
				// Timeout occurred
				if ctx.Err() == context.DeadlineExceeded {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusGatewayTimeout)
					_, _ = w.Write([]byte(`{"error":{"code":"TIMEOUT","message":"request timeout"}}`))
				}
			}
		})
	}
}

// ErrorHandler creates a middleware that handles errors from handlers.
// It expects handlers to return errors via context.
type ErrorHandlerFunc func(w http.ResponseWriter, r *http.Request) error

// WrapHandler wraps a handler function that returns an error.
func WrapHandler(log *logger.Logger, fn ErrorHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			HandleError(w, r, log, err)
		}
	}
}

// HandleError handles an error and writes the appropriate response.
func HandleError(w http.ResponseWriter, r *http.Request, log *logger.Logger, err error) {
	reqLog := log.FromContext(r.Context())

	// Get error details
	code := errors.GetCode(err)
	status := errors.GetHTTPStatus(err)
	fields := errors.GetFields(err)

	// Log the error
	logFields := []any{
		"error", err.Error(),
		"code", string(code),
		"status", status,
		"method", r.Method,
		"path", r.URL.Path,
	}
	for k, v := range fields {
		logFields = append(logFields, k, v)
	}

	if status >= 500 {
		// Include stack trace for server errors
		var galaErr *errors.Error
		if errors.As(err, &galaErr) && len(galaErr.Stack) > 0 {
			logFields = append(logFields, "stack", galaErr.StackTrace())
		}
		reqLog.Error("request failed", logFields...)
	} else {
		reqLog.Warn("request error", logFields...)
	}

	// Write error response
	WriteErrorResponse(w, code, err.Error(), fields)
}

// WriteErrorResponse writes a JSON error response.
func WriteErrorResponse(w http.ResponseWriter, code errors.Code, message string, details map[string]any) {
	status := (&errors.Error{Code: code}).HTTPStatus()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Build response
	response := `{"error":{"code":"` + string(code) + `","message":"` + escapeJSON(message) + `"`
	if len(details) > 0 {
		response += `,"details":{`
		first := true
		for k, v := range details {
			if !first {
				response += ","
			}
			response += `"` + escapeJSON(k) + `":"` + escapeJSON(toString(v)) + `"`
			first = false
		}
		response += "}"
	}
	response += "}}"

	_, _ = w.Write([]byte(response))
}

// generateRequestID generates a unique request ID.
func generateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// escapeJSON escapes a string for JSON output.
func escapeJSON(s string) string {
	result := ""
	for _, c := range s {
		switch c {
		case '"':
			result += `\"`
		case '\\':
			result += `\\`
		case '\n':
			result += `\n`
		case '\r':
			result += `\r`
		case '\t':
			result += `\t`
		default:
			result += string(c)
		}
	}
	return result
}

// toString converts a value to string.
func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case error:
		return val.Error()
	default:
		return ""
	}
}
