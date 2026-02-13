package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// contextKey is used to store logger in context.
type contextKey struct{}

var (
	// defaultLogger is the package-level logger used by the convenience functions.
	defaultLogger *slog.Logger
	// mu protects defaultLogger during initialization.
	mu sync.RWMutex
)

func init() {
	// Initialise with a basic logger; call Init() for full configuration.
	defaultLogger = slog.New(slog.NewTextHandler(os.Stderr, nil))
}

// Config holds logger configuration options.
type Config struct {
	// Level sets the minimum log level. Default is LevelInfo.
	Level slog.Level

	// Format specifies the output format: "json" or "text".
	// Default is "text".
	Format string

	// Output is the writer for log output. Default is os.Stderr.
	Output io.Writer

	// AddSource adds source file and line number to log entries.
	// Useful for debugging but adds overhead.
	AddSource bool
}

// DefaultConfig returns a Config with sensible defaults.
// It reads LOG_LEVEL and LOG_FORMAT from environment variables.
func DefaultConfig() Config {
	cfg := Config{
		Level:  slog.LevelInfo,
		Format: "text",
		Output: os.Stderr,
	}

	// Parse LOG_LEVEL environment variable.
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.Level = ParseLevel(level)
	}

	// Parse LOG_FORMAT environment variable.
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		cfg.Format = strings.ToLower(format)
	}

	return cfg
}

// ParseLevel converts a string level name to slog.Level.
// Supported values: debug, info, warn, warning, error.
// Returns LevelInfo for unrecognised values.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
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

// Init initialises the default logger with settings from environment variables.
// This should be called early in main() before any logging occurs.
func Init() {
	InitWithConfig(DefaultConfig())
}

// SetVerbose reinitialises the logger at debug level, preserving other settings.
func SetVerbose() {
	cfg := DefaultConfig()
	cfg.Level = slog.LevelDebug
	InitWithConfig(cfg)
}

// InitWithConfig initialises the default logger with the given configuration.
func InitWithConfig(cfg Config) {
	mu.Lock()
	defer mu.Unlock()

	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.AddSource,
	}

	output := cfg.Output
	if output == nil {
		output = os.Stderr
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

// Logger returns the default logger instance.
// Use this when you need to pass a logger to other packages.
func Logger() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return defaultLogger
}

// With creates a new Logger with the given attributes.
// Use this to create component-specific loggers.
//
// Example:
//
//	var logger = log.With("component", "ssh")
func With(args ...any) *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return defaultLogger.With(args...)
}

// WithGroup creates a new Logger with the given group name.
// All attributes added to the returned logger will be nested under this group.
func WithGroup(name string) *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return defaultLogger.WithGroup(name)
}

// NewContext returns a context with the given logger attached.
func NewContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

// FromContext returns the logger from the context, or the default logger if none.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(contextKey{}).(*slog.Logger); ok {
		return logger
	}
	mu.RLock()
	defer mu.RUnlock()
	return defaultLogger
}

// WithContext returns a context with a logger that includes the given attributes.
// This is useful for adding request-scoped attributes to all subsequent log calls.
//
// Example:
//
//	ctx = log.WithContext(ctx, "request_id", reqID, "user", userID)
//	log.InfoContext(ctx, "processing") // includes request_id and user
func WithContext(ctx context.Context, args ...any) context.Context {
	logger := FromContext(ctx).With(args...)
	return NewContext(ctx, logger)
}

// Debug logs at LevelDebug.
func Debug(msg string, args ...any) {
	mu.RLock()
	logger := defaultLogger
	mu.RUnlock()
	logger.Debug(msg, args...)
}

// Info logs at LevelInfo.
func Info(msg string, args ...any) {
	mu.RLock()
	logger := defaultLogger
	mu.RUnlock()
	logger.Info(msg, args...)
}

// Warn logs at LevelWarn.
func Warn(msg string, args ...any) {
	mu.RLock()
	logger := defaultLogger
	mu.RUnlock()
	logger.Warn(msg, args...)
}

// Error logs at LevelError.
func Error(msg string, args ...any) {
	mu.RLock()
	logger := defaultLogger
	mu.RUnlock()
	logger.Error(msg, args...)
}

// DebugContext logs at LevelDebug with context.
func DebugContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).DebugContext(ctx, msg, args...)
}

// InfoContext logs at LevelInfo with context.
func InfoContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).InfoContext(ctx, msg, args...)
}

// WarnContext logs at LevelWarn with context.
func WarnContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).WarnContext(ctx, msg, args...)
}

// ErrorContext logs at LevelError with context.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).ErrorContext(ctx, msg, args...)
}

// Err is a convenience function that returns an slog.Attr for an error.
// This makes it easy to add errors to log calls with a consistent key.
//
// Example:
//
//	log.Error("operation failed", log.Err(err), "operation", "connect")
func Err(err error) slog.Attr {
	return slog.Any("error", err)
}
