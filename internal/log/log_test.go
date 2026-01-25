package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"  debug  ", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	// Clear environment variables for this test.
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("LOG_FORMAT")

	cfg := DefaultConfig()

	if cfg.Level != slog.LevelInfo {
		t.Errorf("DefaultConfig().Level = %v, want %v", cfg.Level, slog.LevelInfo)
	}
	if cfg.Format != "text" {
		t.Errorf("DefaultConfig().Format = %q, want %q", cfg.Format, "text")
	}
	if cfg.Output != os.Stderr {
		t.Errorf("DefaultConfig().Output = %v, want os.Stderr", cfg.Output)
	}
}

func TestDefaultConfigWithEnv(t *testing.T) {
	// Set environment variables.
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LOG_FORMAT", "JSON")
	defer func() {
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("LOG_FORMAT")
	}()

	cfg := DefaultConfig()

	if cfg.Level != slog.LevelDebug {
		t.Errorf("DefaultConfig().Level = %v, want %v", cfg.Level, slog.LevelDebug)
	}
	if cfg.Format != "json" {
		t.Errorf("DefaultConfig().Format = %q, want %q", cfg.Format, "json")
	}
}

func TestInitWithConfig(t *testing.T) {
	var buf bytes.Buffer

	InitWithConfig(Config{
		Level:  slog.LevelInfo,
		Format: "text",
		Output: &buf,
	})

	Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("output should contain message, got: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("output should contain key=value, got: %s", output)
	}
}

func TestJSONFormat(t *testing.T) {
	var buf bytes.Buffer

	InitWithConfig(Config{
		Level:  slog.LevelInfo,
		Format: "json",
		Output: &buf,
	})

	Info("json test", "number", 42, "flag", true)

	// Parse the JSON output.
	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON output: %v, got: %s", err, buf.String())
	}

	if logEntry["msg"] != "json test" {
		t.Errorf("msg = %v, want %q", logEntry["msg"], "json test")
	}
	if logEntry["number"] != float64(42) {
		t.Errorf("number = %v, want 42", logEntry["number"])
	}
	if logEntry["flag"] != true {
		t.Errorf("flag = %v, want true", logEntry["flag"])
	}
}

func TestLogLevelFiltering(t *testing.T) {
	var buf bytes.Buffer

	InitWithConfig(Config{
		Level:  slog.LevelWarn,
		Format: "text",
		Output: &buf,
	})

	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")

	output := buf.String()

	if strings.Contains(output, "debug message") {
		t.Error("debug message should be filtered out")
	}
	if strings.Contains(output, "info message") {
		t.Error("info message should be filtered out")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("warn message should be present")
	}
	if !strings.Contains(output, "error message") {
		t.Error("error message should be present")
	}
}

func TestWith(t *testing.T) {
	var buf bytes.Buffer

	InitWithConfig(Config{
		Level:  slog.LevelInfo,
		Format: "text",
		Output: &buf,
	})

	componentLogger := With("component", "ssh")
	componentLogger.Info("connection attempt", "host", "example.com")

	output := buf.String()
	if !strings.Contains(output, "component=ssh") {
		t.Errorf("output should contain component=ssh, got: %s", output)
	}
	if !strings.Contains(output, "host=example.com") {
		t.Errorf("output should contain host=example.com, got: %s", output)
	}
}

func TestWithGroup(t *testing.T) {
	var buf bytes.Buffer

	InitWithConfig(Config{
		Level:  slog.LevelInfo,
		Format: "json",
		Output: &buf,
	})

	groupLogger := WithGroup("request")
	groupLogger.Info("test", "id", "123", "user", "alice")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// In JSON format, grouped attributes should be nested.
	requestGroup, ok := logEntry["request"].(map[string]any)
	if !ok {
		t.Fatalf("expected request group in output, got: %v", logEntry)
	}
	if requestGroup["id"] != "123" {
		t.Errorf("request.id = %v, want %q", requestGroup["id"], "123")
	}
}

func TestContextLogging(t *testing.T) {
	var buf bytes.Buffer

	InitWithConfig(Config{
		Level:  slog.LevelInfo,
		Format: "text",
		Output: &buf,
	})

	ctx := context.Background()
	ctx = WithContext(ctx, "request_id", "req-123")

	InfoContext(ctx, "processing request", "action", "fetch")

	output := buf.String()
	if !strings.Contains(output, "request_id=req-123") {
		t.Errorf("output should contain request_id, got: %s", output)
	}
	if !strings.Contains(output, "action=fetch") {
		t.Errorf("output should contain action, got: %s", output)
	}
}

func TestFromContext(t *testing.T) {
	var buf1 bytes.Buffer
	var buf2 bytes.Buffer

	// Create a custom logger.
	customLogger := slog.New(slog.NewTextHandler(&buf1, nil))

	InitWithConfig(Config{
		Level:  slog.LevelInfo,
		Format: "text",
		Output: &buf2,
	})

	// Without custom logger in context, should use default.
	ctx := context.Background()
	logger := FromContext(ctx)
	logger.Info("default logger test")

	if buf2.Len() == 0 {
		t.Error("expected output in default logger buffer")
	}

	// With custom logger in context, should use it.
	buf2.Reset()
	ctx = NewContext(ctx, customLogger)
	logger = FromContext(ctx)
	logger.Info("custom logger test")

	if buf1.Len() == 0 {
		t.Error("expected output in custom logger buffer")
	}
	if buf2.Len() != 0 {
		t.Error("expected no output in default logger buffer")
	}
}

func TestErr(t *testing.T) {
	var buf bytes.Buffer

	InitWithConfig(Config{
		Level:  slog.LevelInfo,
		Format: "text",
		Output: &buf,
	})

	testErr := errors.New("connection refused")
	Error("operation failed", Err(testErr), "host", "example.com")

	output := buf.String()
	if !strings.Contains(output, "error=") {
		t.Errorf("output should contain error=, got: %s", output)
	}
	if !strings.Contains(output, "connection refused") {
		t.Errorf("output should contain error message, got: %s", output)
	}
}

func TestAllLogLevels(t *testing.T) {
	var buf bytes.Buffer

	InitWithConfig(Config{
		Level:  slog.LevelDebug,
		Format: "text",
		Output: &buf,
	})

	Debug("debug msg", "key", "d")
	Info("info msg", "key", "i")
	Warn("warn msg", "key", "w")
	Error("error msg", "key", "e")

	output := buf.String()
	levels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	messages := []string{"debug msg", "info msg", "warn msg", "error msg"}

	for i, level := range levels {
		if !strings.Contains(output, level) {
			t.Errorf("output should contain %s level", level)
		}
		if !strings.Contains(output, messages[i]) {
			t.Errorf("output should contain %q", messages[i])
		}
	}
}

func TestAllLogLevelsContext(t *testing.T) {
	var buf bytes.Buffer

	InitWithConfig(Config{
		Level:  slog.LevelDebug,
		Format: "text",
		Output: &buf,
	})

	ctx := context.Background()

	DebugContext(ctx, "debug ctx")
	InfoContext(ctx, "info ctx")
	WarnContext(ctx, "warn ctx")
	ErrorContext(ctx, "error ctx")

	output := buf.String()
	messages := []string{"debug ctx", "info ctx", "warn ctx", "error ctx"}

	for _, msg := range messages {
		if !strings.Contains(output, msg) {
			t.Errorf("output should contain %q, got: %s", msg, output)
		}
	}
}

func TestLogger(t *testing.T) {
	var buf bytes.Buffer

	InitWithConfig(Config{
		Level:  slog.LevelInfo,
		Format: "text",
		Output: &buf,
	})

	logger := Logger()
	if logger == nil {
		t.Fatal("Logger() returned nil")
	}

	logger.Info("from Logger()")
	if !strings.Contains(buf.String(), "from Logger()") {
		t.Error("output should contain message from Logger()")
	}
}

func TestNilOutput(t *testing.T) {
	// Should not panic with nil output, defaults to stderr.
	InitWithConfig(Config{
		Level:  slog.LevelInfo,
		Format: "text",
		Output: nil,
	})

	// Should not panic.
	Info("test with nil output")
}
