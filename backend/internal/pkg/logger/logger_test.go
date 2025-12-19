package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "default config",
			config: Config{
				Level:       "info",
				Format:      "json",
				ServiceName: "test-service",
			},
		},
		{
			name: "debug level",
			config: Config{
				Level:       "debug",
				Format:      "json",
				ServiceName: "test-service",
			},
		},
		{
			name: "text format",
			config: Config{
				Level:       "info",
				Format:      "text",
				ServiceName: "test-service",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := New(tt.config)
			if log == nil {
				t.Fatal("expected logger to be non-nil")
			}
		})
	}
}

func TestLoggerOutput(t *testing.T) {
	var buf bytes.Buffer

	log := New(Config{
		Level:       "debug",
		Format:      "json",
		Output:      &buf,
		ServiceName: "test-service",
	})

	log.Info("test message", "key", "value")

	output := buf.String()
	if output == "" {
		t.Fatal("expected output to be non-empty")
	}

	// Parse JSON to verify structure
	var entry map[string]any
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	// Check required fields
	if entry["msg"] != "test message" {
		t.Errorf("expected msg='test message', got %v", entry["msg"])
	}
	if entry["key"] != "value" {
		t.Errorf("expected key='value', got %v", entry["key"])
	}
	if entry["service"] != "test-service" {
		t.Errorf("expected service='test-service', got %v", entry["service"])
	}
}

func TestLoggerLevels(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		logFn     func(*Logger)
		shouldLog bool
	}{
		{
			name:      "info level logs info",
			level:     "info",
			logFn:     func(l *Logger) { l.Info("test") },
			shouldLog: true,
		},
		{
			name:      "info level does not log debug",
			level:     "info",
			logFn:     func(l *Logger) { l.Debug("test") },
			shouldLog: false,
		},
		{
			name:      "debug level logs debug",
			level:     "debug",
			logFn:     func(l *Logger) { l.Debug("test") },
			shouldLog: true,
		},
		{
			name:      "error level logs error",
			level:     "error",
			logFn:     func(l *Logger) { l.Error("test") },
			shouldLog: true,
		},
		{
			name:      "error level does not log info",
			level:     "error",
			logFn:     func(l *Logger) { l.Info("test") },
			shouldLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			log := New(Config{
				Level:  tt.level,
				Format: "json",
				Output: &buf,
			})

			tt.logFn(log)

			hasOutput := buf.Len() > 0
			if hasOutput != tt.shouldLog {
				t.Errorf("expected shouldLog=%v, got hasOutput=%v", tt.shouldLog, hasOutput)
			}
		})
	}
}

func TestWithRequestID(t *testing.T) {
	var buf bytes.Buffer

	log := New(Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	})

	logWithID := log.WithRequestID("req-123")
	logWithID.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "req-123") {
		t.Errorf("expected output to contain request_id, got: %s", output)
	}
}

func TestWithJobID(t *testing.T) {
	var buf bytes.Buffer

	log := New(Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	})

	logWithID := log.WithJobID("job-456")
	logWithID.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "job-456") {
		t.Errorf("expected output to contain job_id, got: %s", output)
	}
}

func TestWithComponent(t *testing.T) {
	var buf bytes.Buffer

	log := New(Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	})

	logWithComponent := log.WithComponent("api")
	logWithComponent.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "api") {
		t.Errorf("expected output to contain component, got: %s", output)
	}
}

func TestWithError(t *testing.T) {
	var buf bytes.Buffer

	log := New(Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	})

	logWithErr := log.WithError(nil)
	if logWithErr != log {
		t.Error("WithError(nil) should return same logger")
	}

	buf.Reset()
	logWithErr = log.WithError(context.DeadlineExceeded)
	logWithErr.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "deadline exceeded") {
		t.Errorf("expected output to contain error, got: %s", output)
	}
}

func TestWithFields(t *testing.T) {
	var buf bytes.Buffer

	log := New(Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	})

	logWithFields := log.WithFields(map[string]any{
		"user_id": "usr-123",
		"action":  "create",
	})
	logWithFields.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "usr-123") {
		t.Errorf("expected output to contain user_id, got: %s", output)
	}
	if !strings.Contains(output, "create") {
		t.Errorf("expected output to contain action, got: %s", output)
	}
}

func TestContextWithRequestID(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithRequestID(ctx, "req-789")

	val := ctx.Value(RequestIDKey)
	if val != "req-789" {
		t.Errorf("expected request_id='req-789', got %v", val)
	}
}

func TestContextWithJobID(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithJobID(ctx, "job-101")

	val := ctx.Value(JobIDKey)
	if val != "job-101" {
		t.Errorf("expected job_id='job-101', got %v", val)
	}
}

func TestFromContext(t *testing.T) {
	var buf bytes.Buffer

	log := New(Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	})

	ctx := context.Background()
	ctx = ContextWithRequestID(ctx, "req-abc")
	ctx = ContextWithJobID(ctx, "job-xyz")

	logFromCtx := log.FromContext(ctx)
	logFromCtx.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "req-abc") {
		t.Errorf("expected output to contain request_id, got: %s", output)
	}
	if !strings.Contains(output, "job-xyz") {
		t.Errorf("expected output to contain job_id, got: %s", output)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"debug", "DEBUG"},
		{"DEBUG", "DEBUG"},
		{"info", "INFO"},
		{"INFO", "INFO"},
		{"warn", "WARN"},
		{"warning", "WARN"},
		{"error", "ERROR"},
		{"ERROR", "ERROR"},
		{"unknown", "INFO"}, // defaults to INFO
		{"", "INFO"},        // defaults to INFO
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level := parseLevel(tt.input)
			if level.String() != tt.expected {
				t.Errorf("parseLevel(%q) = %s, expected %s", tt.input, level.String(), tt.expected)
			}
		})
	}
}
