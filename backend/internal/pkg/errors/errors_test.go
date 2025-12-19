package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(CodeValidation, "invalid input")

	if err.Code != CodeValidation {
		t.Errorf("expected code=%s, got %s", CodeValidation, err.Code)
	}
	if err.Message != "invalid input" {
		t.Errorf("expected message='invalid input', got %s", err.Message)
	}
	if len(err.Stack) == 0 {
		t.Error("expected stack trace to be captured")
	}
}

func TestNewf(t *testing.T) {
	err := Newf(CodeNotFound, "user %s not found", "john")

	if err.Code != CodeNotFound {
		t.Errorf("expected code=%s, got %s", CodeNotFound, err.Code)
	}
	if err.Message != "user john not found" {
		t.Errorf("expected formatted message, got %s", err.Message)
	}
}

func TestErrorString(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		contains []string
	}{
		{
			name:     "simple error",
			err:      New(CodeValidation, "invalid"),
			contains: []string{"VALIDATION_ERROR", "invalid"},
		},
		{
			name: "error with op",
			err: &Error{
				Code:    CodeInternal,
				Message: "db failed",
				Op:      "user.create",
			},
			contains: []string{"user.create", "INTERNAL_ERROR", "db failed"},
		},
		{
			name: "error with underlying",
			err: &Error{
				Code:    CodeInternal,
				Message: "wrapper",
				Err:     fmt.Errorf("underlying error"),
			},
			contains: []string{"wrapper", "underlying error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			str := tt.err.Error()
			for _, c := range tt.contains {
				if !strings.Contains(str, c) {
					t.Errorf("expected error string to contain %q, got: %s", c, str)
				}
			}
		})
	}
}

func TestWrap(t *testing.T) {
	original := fmt.Errorf("original error")
	wrapped := Wrap(original, "service.call", "service call failed")

	if wrapped == nil {
		t.Fatal("expected wrapped error to be non-nil")
	}
	if wrapped.Code != CodeInternal {
		t.Errorf("expected code=%s, got %s", CodeInternal, wrapped.Code)
	}
	if wrapped.Op != "service.call" {
		t.Errorf("expected op='service.call', got %s", wrapped.Op)
	}
	if wrapped.Err != original {
		t.Error("expected underlying error to be preserved")
	}

	// Test Unwrap
	if errors.Unwrap(wrapped) != original {
		t.Error("Unwrap should return original error")
	}
}

func TestWrapNil(t *testing.T) {
	wrapped := Wrap(nil, "op", "message")
	if wrapped != nil {
		t.Error("Wrap(nil) should return nil")
	}
}

func TestWrapPreservesCode(t *testing.T) {
	original := New(CodeNotFound, "not found")
	wrapped := Wrap(original, "handler", "handler failed")

	if wrapped.Code != CodeNotFound {
		t.Errorf("expected code to be preserved as %s, got %s", CodeNotFound, wrapped.Code)
	}
}

func TestWrapWithCode(t *testing.T) {
	original := fmt.Errorf("timeout")
	wrapped := WrapWithCode(original, CodeTimeout, "api.call", "request timed out")

	if wrapped.Code != CodeTimeout {
		t.Errorf("expected code=%s, got %s", CodeTimeout, wrapped.Code)
	}
}

func TestWithField(t *testing.T) {
	err := New(CodeValidation, "invalid").
		WithField("field", "email").
		WithField("value", "not-an-email")

	if err.Fields["field"] != "email" {
		t.Errorf("expected field='email', got %v", err.Fields["field"])
	}
	if err.Fields["value"] != "not-an-email" {
		t.Errorf("expected value='not-an-email', got %v", err.Fields["value"])
	}
}

func TestWithFields(t *testing.T) {
	err := New(CodeValidation, "invalid").
		WithFields(map[string]any{
			"field1": "value1",
			"field2": "value2",
		})

	if len(err.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(err.Fields))
	}
}

func TestHTTPStatus(t *testing.T) {
	tests := []struct {
		code   Code
		status int
	}{
		{CodeValidation, 400},
		{CodeBadRequest, 400},
		{CodeUnauthorized, 401},
		{CodeForbidden, 403},
		{CodeNotFound, 404},
		{CodeConflict, 409},
		{CodeAlreadyExists, 409},
		{CodeFailedPrecond, 412},
		{CodeResourceExhaust, 429},
		{CodeInternal, 500},
		{CodeUnavailable, 503},
		{CodeTimeout, 504},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			err := New(tt.code, "test")
			if err.HTTPStatus() != tt.status {
				t.Errorf("expected status=%d, got %d", tt.status, err.HTTPStatus())
			}
		})
	}
}

func TestConvenienceConstructors(t *testing.T) {
	t.Run("Internal", func(t *testing.T) {
		err := Internal("something broke")
		if err.Code != CodeInternal {
			t.Errorf("expected code=%s, got %s", CodeInternal, err.Code)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		err := NotFound("user", "123")
		if err.Code != CodeNotFound {
			t.Errorf("expected code=%s, got %s", CodeNotFound, err.Code)
		}
		if err.Fields["resource"] != "user" {
			t.Errorf("expected resource='user', got %v", err.Fields["resource"])
		}
		if err.Fields["id"] != "123" {
			t.Errorf("expected id='123', got %v", err.Fields["id"])
		}
	})

	t.Run("Validation", func(t *testing.T) {
		err := Validation("invalid input")
		if err.Code != CodeValidation {
			t.Errorf("expected code=%s, got %s", CodeValidation, err.Code)
		}
	})

	t.Run("ValidationField", func(t *testing.T) {
		err := ValidationField("email", "must be valid email")
		if err.Code != CodeValidation {
			t.Errorf("expected code=%s, got %s", CodeValidation, err.Code)
		}
		if err.Fields["field"] != "email" {
			t.Errorf("expected field='email', got %v", err.Fields["field"])
		}
	})

	t.Run("Conflict", func(t *testing.T) {
		err := Conflict("resource in use")
		if err.Code != CodeConflict {
			t.Errorf("expected code=%s, got %s", CodeConflict, err.Code)
		}
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		err := AlreadyExists("template", "tpl_123")
		if err.Code != CodeAlreadyExists {
			t.Errorf("expected code=%s, got %s", CodeAlreadyExists, err.Code)
		}
	})

	t.Run("Timeout", func(t *testing.T) {
		err := Timeout("db query")
		if err.Code != CodeTimeout {
			t.Errorf("expected code=%s, got %s", CodeTimeout, err.Code)
		}
	})

	t.Run("Unavailable", func(t *testing.T) {
		err := Unavailable("renderer")
		if err.Code != CodeUnavailable {
			t.Errorf("expected code=%s, got %s", CodeUnavailable, err.Code)
		}
	})
}

func TestGetCode(t *testing.T) {
	t.Run("from gala error", func(t *testing.T) {
		err := New(CodeNotFound, "not found")
		if GetCode(err) != CodeNotFound {
			t.Errorf("expected code=%s, got %s", CodeNotFound, GetCode(err))
		}
	})

	t.Run("from standard error", func(t *testing.T) {
		err := fmt.Errorf("standard error")
		if GetCode(err) != CodeInternal {
			t.Errorf("expected code=%s, got %s", CodeInternal, GetCode(err))
		}
	})

	t.Run("from wrapped error", func(t *testing.T) {
		original := New(CodeValidation, "invalid")
		wrapped := Wrap(original, "handler", "wrapped")
		if GetCode(wrapped) != CodeValidation {
			t.Errorf("expected code=%s, got %s", CodeValidation, GetCode(wrapped))
		}
	})
}

func TestGetHTTPStatus(t *testing.T) {
	err := New(CodeNotFound, "not found")
	if GetHTTPStatus(err) != 404 {
		t.Errorf("expected status=404, got %d", GetHTTPStatus(err))
	}

	stdErr := fmt.Errorf("standard")
	if GetHTTPStatus(stdErr) != 500 {
		t.Errorf("expected status=500 for standard error, got %d", GetHTTPStatus(stdErr))
	}
}

func TestGetFields(t *testing.T) {
	err := New(CodeValidation, "invalid").
		WithField("field", "email")

	fields := GetFields(err)
	if fields["field"] != "email" {
		t.Errorf("expected field='email', got %v", fields["field"])
	}

	stdErr := fmt.Errorf("standard")
	if GetFields(stdErr) != nil {
		t.Error("expected nil fields for standard error")
	}
}

func TestIsCode(t *testing.T) {
	err := New(CodeNotFound, "not found")

	if !IsCode(err, CodeNotFound) {
		t.Error("expected IsCode to return true")
	}
	if IsCode(err, CodeValidation) {
		t.Error("expected IsCode to return false")
	}
}

func TestIsNotFound(t *testing.T) {
	notFound := New(CodeNotFound, "not found")
	other := New(CodeValidation, "invalid")

	if !IsNotFound(notFound) {
		t.Error("expected IsNotFound to return true")
	}
	if IsNotFound(other) {
		t.Error("expected IsNotFound to return false")
	}
}

func TestIsValidation(t *testing.T) {
	validation := New(CodeValidation, "invalid")
	other := New(CodeNotFound, "not found")

	if !IsValidation(validation) {
		t.Error("expected IsValidation to return true")
	}
	if IsValidation(other) {
		t.Error("expected IsValidation to return false")
	}
}

func TestIsConflict(t *testing.T) {
	conflict := New(CodeConflict, "conflict")
	exists := New(CodeAlreadyExists, "exists")
	other := New(CodeNotFound, "not found")

	if !IsConflict(conflict) {
		t.Error("expected IsConflict to return true for CodeConflict")
	}
	if !IsConflict(exists) {
		t.Error("expected IsConflict to return true for CodeAlreadyExists")
	}
	if IsConflict(other) {
		t.Error("expected IsConflict to return false")
	}
}

func TestStackTrace(t *testing.T) {
	err := New(CodeInternal, "test error")

	stack := err.StackTrace()
	if stack == "" {
		t.Error("expected non-empty stack trace")
	}

	// Should contain file references
	if !strings.Contains(stack, ".go:") {
		t.Errorf("expected stack trace to contain file references, got: %s", stack)
	}
}

func TestErrorIs(t *testing.T) {
	err1 := New(CodeNotFound, "error 1")
	err2 := New(CodeNotFound, "error 2")
	err3 := New(CodeValidation, "error 3")

	if !errors.Is(err1, err2) {
		t.Error("expected errors with same code to match with Is")
	}
	if errors.Is(err1, err3) {
		t.Error("expected errors with different codes to not match")
	}
}

func TestAsAndIs(t *testing.T) {
	original := New(CodeNotFound, "not found")
	wrapped := fmt.Errorf("wrapped: %w", original)

	var target *Error
	if !As(wrapped, &target) {
		t.Error("expected As to find Error in chain")
	}
	if target.Code != CodeNotFound {
		t.Errorf("expected code=%s, got %s", CodeNotFound, target.Code)
	}

	if !Is(wrapped, original) {
		t.Error("expected Is to match original error")
	}
}
