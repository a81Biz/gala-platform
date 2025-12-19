package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// RequestIDKey is the context key for request IDs.
	RequestIDKey contextKey = "request_id"
	// JobIDKey is the context key for job IDs.
	JobIDKey contextKey = "job_id"
)

// Logger wraps slog.Logger with GALA-specific functionality.
type Logger struct {
	*slog.Logger
}

// Config holds logger configuration.
type Config struct {
	// Level is the minimum log level (debug, info, warn, error).
	Level string
	// Format is the output format (json, text).
	Format string
	// Output is the writer for log output (defaults to os.Stdout).
	Output io.Writer
	// AddSource adds source file and line to logs.
	AddSource bool
	// ServiceName is the name of the service for identification.
	ServiceName string
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		Level:       getEnv("LOG_LEVEL", "info"),
		Format:      getEnv("LOG_FORMAT", "json"),
		Output:      os.Stdout,
		AddSource:   getEnv("LOG_SOURCE", "false") == "true",
		ServiceName: getEnv("SERVICE_NAME", "gala"),
	}
}

// New creates a new Logger with the given configuration.
func New(cfg Config) *Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}

	level := parseLevel(cfg.Level)

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize time format
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.UTC().Format(time.RFC3339Nano))
				}
			}
			return a
		},
	}

	var handler slog.Handler
	if cfg.Format == "text" {
		handler = slog.NewTextHandler(cfg.Output, opts)
	} else {
		handler = slog.NewJSONHandler(cfg.Output, opts)
	}

	// Add service name as default attribute
	if cfg.ServiceName != "" {
		handler = handler.WithAttrs([]slog.Attr{
			slog.String("service", cfg.ServiceName),
		})
	}

	return &Logger{
		Logger: slog.New(handler),
	}
}

// NewDefault creates a logger with default configuration.
func NewDefault() *Logger {
	return New(DefaultConfig())
}

// WithRequestID returns a new logger with the request ID attached.
func (l *Logger) WithRequestID(requestID string) *Logger {
	return &Logger{
		Logger: l.Logger.With(slog.String("request_id", requestID)),
	}
}

// WithJobID returns a new logger with the job ID attached.
func (l *Logger) WithJobID(jobID string) *Logger {
	return &Logger{
		Logger: l.Logger.With(slog.String("job_id", jobID)),
	}
}

// WithComponent returns a new logger with the component name attached.
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With(slog.String("component", component)),
	}
}

// WithError returns a new logger with the error attached.
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return &Logger{
		Logger: l.Logger.With(slog.String("error", err.Error())),
	}
}

// WithFields returns a new logger with additional fields.
func (l *Logger) WithFields(fields map[string]any) *Logger {
	attrs := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	return &Logger{
		Logger: l.Logger.With(attrs...),
	}
}

// FromContext extracts logger context values and returns an enriched logger.
func (l *Logger) FromContext(ctx context.Context) *Logger {
	result := l
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
		result = result.WithRequestID(reqID)
	}
	if jobID, ok := ctx.Value(JobIDKey).(string); ok && jobID != "" {
		result = result.WithJobID(jobID)
	}
	return result
}

// LogError logs an error with stack trace information.
func (l *Logger) LogError(ctx context.Context, msg string, err error, args ...any) {
	if err == nil {
		return
	}

	// Get caller info
	_, file, line, ok := runtime.Caller(1)
	if ok {
		args = append(args, "source", slog.GroupValue(
			slog.String("file", file),
			slog.Int("line", line),
		))
	}

	args = append(args, "error", err.Error())
	l.FromContext(ctx).Error(msg, args...)
}

// LogFatal logs a fatal error and exits.
func (l *Logger) LogFatal(msg string, err error, args ...any) {
	if err != nil {
		args = append(args, "error", err.Error())
	}
	l.Error(msg, args...)
	os.Exit(1)
}

// ContextWithRequestID adds a request ID to the context.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// ContextWithJobID adds a job ID to the context.
func ContextWithJobID(ctx context.Context, jobID string) context.Context {
	return context.WithValue(ctx, JobIDKey, jobID)
}

// parseLevel converts a string level to slog.Level.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// getEnv gets an environment variable with a default value.
func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
