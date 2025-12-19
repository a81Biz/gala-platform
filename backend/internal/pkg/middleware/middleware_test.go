package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gala/internal/pkg/errors"
	"gala/internal/pkg/logger"
)

func TestRequestID(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that request ID is in context
		reqID := r.Context().Value(logger.RequestIDKey)
		if reqID == nil || reqID == "" {
			t.Error("expected request ID in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("generates new request ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		reqID := rec.Header().Get(RequestIDHeader)
		if reqID == "" {
			t.Error("expected X-Request-ID header to be set")
		}
		if len(reqID) != 32 { // hex encoded 16 bytes
			t.Errorf("expected request ID length 32, got %d", len(reqID))
		}
	})

	t.Run("preserves existing request ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set(RequestIDHeader, "existing-id-123")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		reqID := rec.Header().Get(RequestIDHeader)
		if reqID != "existing-id-123" {
			t.Errorf("expected preserved request ID 'existing-id-123', got %s", reqID)
		}
	})
}

func TestLogging(t *testing.T) {
	var logBuf bytes.Buffer
	log := logger.New(logger.Config{
		Level:  "info",
		Format: "json",
		Output: &logBuf,
	})

	handler := Logging(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()

	// Should contain request completed log
	if !strings.Contains(logOutput, "request completed") {
		t.Errorf("expected 'request completed' in log, got: %s", logOutput)
	}

	// Should contain method
	if !strings.Contains(logOutput, "GET") {
		t.Errorf("expected method in log, got: %s", logOutput)
	}

	// Should contain path
	if !strings.Contains(logOutput, "/test") {
		t.Errorf("expected path in log, got: %s", logOutput)
	}

	// Should contain status
	if !strings.Contains(logOutput, "200") {
		t.Errorf("expected status in log, got: %s", logOutput)
	}

	// Should contain duration_ms
	if !strings.Contains(logOutput, "duration_ms") {
		t.Errorf("expected duration_ms in log, got: %s", logOutput)
	}
}

func TestLoggingLevels(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		expectedLevel string
	}{
		{"2xx logs info", 200, "INFO"},
		{"3xx logs info", 302, "INFO"},
		{"4xx logs warn", 404, "WARN"},
		{"5xx logs error", 500, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuf bytes.Buffer
			log := logger.New(logger.Config{
				Level:  "debug",
				Format: "json",
				Output: &logBuf,
			})

			handler := Logging(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			logOutput := logBuf.String()
			if !strings.Contains(logOutput, tt.expectedLevel) {
				t.Errorf("expected log level %s, got: %s", tt.expectedLevel, logOutput)
			}
		})
	}
}

func TestRecovery(t *testing.T) {
	var logBuf bytes.Buffer
	log := logger.New(logger.Config{
		Level:  "info",
		Format: "json",
		Output: &logBuf,
	})

	handler := Recovery(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(rec, req)

	// Should return 500
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}

	// Should return JSON error
	body := rec.Body.String()
	if !strings.Contains(body, "INTERNAL_ERROR") {
		t.Errorf("expected INTERNAL_ERROR in body, got: %s", body)
	}

	// Should log the panic
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "panic recovered") {
		t.Errorf("expected 'panic recovered' in log, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "test panic") {
		t.Errorf("expected panic message in log, got: %s", logOutput)
	}
}

func TestResponseWriter(t *testing.T) {
	t.Run("captures status code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := wrapResponseWriter(rec)

		rw.WriteHeader(http.StatusCreated)

		if rw.status != http.StatusCreated {
			t.Errorf("expected status 201, got %d", rw.status)
		}
	})

	t.Run("captures size", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := wrapResponseWriter(rec)

		rw.Write([]byte("hello world"))

		if rw.size != 11 {
			t.Errorf("expected size 11, got %d", rw.size)
		}
	})

	t.Run("defaults to 200", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := wrapResponseWriter(rec)

		rw.Write([]byte("hello"))

		if rw.status != http.StatusOK {
			t.Errorf("expected default status 200, got %d", rw.status)
		}
	})

	t.Run("only writes header once", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := wrapResponseWriter(rec)

		rw.WriteHeader(http.StatusCreated)
		rw.WriteHeader(http.StatusOK) // Should be ignored

		if rw.status != http.StatusCreated {
			t.Errorf("expected status 201, got %d", rw.status)
		}
	})
}

func TestWrapHandler(t *testing.T) {
	var logBuf bytes.Buffer
	log := logger.New(logger.Config{
		Level:  "info",
		Format: "json",
		Output: &logBuf,
	})

	t.Run("successful handler", func(t *testing.T) {
		handler := WrapHandler(log, func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
			return nil
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("handler with error", func(t *testing.T) {
		logBuf.Reset()

		handler := WrapHandler(log, func(w http.ResponseWriter, r *http.Request) error {
			return errors.NotFound("user", "123")
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "NOT_FOUND") {
			t.Errorf("expected NOT_FOUND in body, got: %s", body)
		}
	})
}

func TestWriteErrorResponse(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.Code
		message  string
		details  map[string]any
		expected int
	}{
		{
			name:     "validation error",
			code:     errors.CodeValidation,
			message:  "invalid email",
			details:  map[string]any{"field": "email"},
			expected: 400,
		},
		{
			name:     "not found",
			code:     errors.CodeNotFound,
			message:  "user not found",
			details:  nil,
			expected: 404,
		},
		{
			name:     "internal error",
			code:     errors.CodeInternal,
			message:  "unexpected error",
			details:  nil,
			expected: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			WriteErrorResponse(rec, tt.code, tt.message, tt.details)

			if rec.Code != tt.expected {
				t.Errorf("expected status %d, got %d", tt.expected, rec.Code)
			}

			body := rec.Body.String()
			if !strings.Contains(body, string(tt.code)) {
				t.Errorf("expected code in body, got: %s", body)
			}
			if !strings.Contains(body, tt.message) {
				t.Errorf("expected message in body, got: %s", body)
			}
		})
	}
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	if id1 == id2 {
		t.Error("expected unique request IDs")
	}

	if len(id1) != 32 {
		t.Errorf("expected length 32, got %d", len(id1))
	}
}

func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`hello`, `hello`},
		{`hello "world"`, `hello \"world\"`},
		{"hello\nworld", `hello\nworld`},
		{"hello\tworld", `hello\tworld`},
		{`back\slash`, `back\\slash`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeJSON(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Helper to discard response body
func discardBody(r *http.Response) {
	if r.Body != nil {
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
	}
}
