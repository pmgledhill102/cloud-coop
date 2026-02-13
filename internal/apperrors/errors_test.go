package apperrors

import (
	"errors"
	"testing"
)

// =============================================================================
// Exit Code Tests
// =============================================================================

func TestExitCodes(t *testing.T) {
	// Verify exit codes match Unix conventions
	if ExitSuccess != 0 {
		t.Errorf("ExitSuccess = %d; want 0", ExitSuccess)
	}
	if ExitError != 1 {
		t.Errorf("ExitError = %d; want 1", ExitError)
	}
	if ExitUsage != 2 {
		t.Errorf("ExitUsage = %d; want 2", ExitUsage)
	}
}

func TestExitCodeFor(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{name: "nil error", err: nil, expected: ExitSuccess},
		{name: "general error", err: errors.New("some error"), expected: ExitError},
		{name: "invalid input", err: ErrInvalidInput, expected: ExitUsage},
		{name: "wrapped invalid input", err: Wrap(ErrInvalidInput, "bad flag"), expected: ExitUsage},
		{name: "not found", err: ErrNotFound, expected: ExitError},
		{name: "timeout", err: ErrTimeout, expected: ExitError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExitCodeFor(tt.err)
			if result != tt.expected {
				t.Errorf("ExitCodeFor(%v) = %d; want %d", tt.err, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Sentinel Error Tests
// =============================================================================

func TestSentinelErrors(t *testing.T) {
	// Verify sentinel errors are distinct
	sentinels := []error{
		ErrNotFound,
		ErrAlreadyExists,
		ErrInvalidInput,
		ErrTimeout,
		ErrCanceled,
		ErrPermissionDenied,
		ErrUnavailable,
	}

	for i, err1 := range sentinels {
		for j, err2 := range sentinels {
			if i != j && errors.Is(err1, err2) {
				t.Errorf("sentinel errors should be distinct: %v == %v", err1, err2)
			}
		}
	}
}

// =============================================================================
// CloudError Tests
// =============================================================================

func TestCloudError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *CloudError
		expected string
	}{
		{
			name: "with resource",
			err: &CloudError{
				Provider:  "gcp",
				Operation: "create-vm",
				Resource:  "my-instance",
				Err:       errors.New("quota exceeded"),
			},
			expected: "gcp create-vm failed for my-instance: quota exceeded",
		},
		{
			name: "without resource",
			err: &CloudError{
				Provider:  "aws",
				Operation: "list-instances",
				Err:       errors.New("access denied"),
			},
			expected: "aws list-instances failed: access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Error() = %q; want %q", result, tt.expected)
			}
		})
	}
}

func TestCloudError_Unwrap(t *testing.T) {
	underlying := ErrPermissionDenied
	err := NewCloudError("gcp", "start-vm", "instance-1", underlying)

	if !errors.Is(err, underlying) {
		t.Error("CloudError should unwrap to underlying error")
	}

	var cloudErr *CloudError
	if !errors.As(err, &cloudErr) {
		t.Error("errors.As should find CloudError")
	}
	if cloudErr.Provider != "gcp" {
		t.Errorf("Provider = %q; want %q", cloudErr.Provider, "gcp")
	}
}

func TestNewCloudError(t *testing.T) {
	err := NewCloudError("azure", "delete-vm", "vm-123", ErrNotFound)

	if err.Provider != "azure" {
		t.Errorf("Provider = %q; want %q", err.Provider, "azure")
	}
	if err.Operation != "delete-vm" {
		t.Errorf("Operation = %q; want %q", err.Operation, "delete-vm")
	}
	if err.Resource != "vm-123" {
		t.Errorf("Resource = %q; want %q", err.Resource, "vm-123")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Error("should wrap ErrNotFound")
	}
}

// =============================================================================
// SSHError Tests
// =============================================================================

func TestSSHError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *SSHError
		expected string
	}{
		{
			name: "with command",
			err: &SSHError{
				Host:    "10.0.0.1",
				Command: "tmux list-sessions",
				Err:     errors.New("connection refused"),
			},
			expected: `ssh to 10.0.0.1 failed executing "tmux list-sessions": connection refused`,
		},
		{
			name: "without command",
			err: &SSHError{
				Host: "example.com",
				Err:  ErrTimeout,
			},
			expected: "ssh to example.com failed: timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Error() = %q; want %q", result, tt.expected)
			}
		})
	}
}

func TestSSHError_Unwrap(t *testing.T) {
	underlying := ErrTimeout
	err := NewSSHError("host.example.com", "ls", underlying)

	if !errors.Is(err, underlying) {
		t.Error("SSHError should unwrap to underlying error")
	}

	var sshErr *SSHError
	if !errors.As(err, &sshErr) {
		t.Error("errors.As should find SSHError")
	}
	if sshErr.Host != "host.example.com" {
		t.Errorf("Host = %q; want %q", sshErr.Host, "host.example.com")
	}
}

func TestNewSSHError(t *testing.T) {
	err := NewSSHError("192.168.1.1", "uptime", ErrUnavailable)

	if err.Host != "192.168.1.1" {
		t.Errorf("Host = %q; want %q", err.Host, "192.168.1.1")
	}
	if err.Command != "uptime" {
		t.Errorf("Command = %q; want %q", err.Command, "uptime")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Error("should wrap ErrUnavailable")
	}
}

// =============================================================================
// ConfigError Tests
// =============================================================================

func TestConfigError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ConfigError
		expected string
	}{
		{
			name: "with file and field",
			err: &ConfigError{
				File:  "~/.config/cloudcoop/config.yaml",
				Field: "cloud.provider",
				Err:   errors.New("must be one of: gcp, aws, azure"),
			},
			expected: `config error in ~/.config/cloudcoop/config.yaml field "cloud.provider": must be one of: gcp, aws, azure`,
		},
		{
			name: "with file only",
			err: &ConfigError{
				File: "/etc/cloudcoop.yaml",
				Err:  errors.New("parse error"),
			},
			expected: "config error in /etc/cloudcoop.yaml: parse error",
		},
		{
			name: "with field only",
			err: &ConfigError{
				Field: "timeout",
				Err:   errors.New("must be positive"),
			},
			expected: `config error in field "timeout": must be positive`,
		},
		{
			name: "with neither",
			err: &ConfigError{
				Err: errors.New("configuration not initialised"),
			},
			expected: "config error: configuration not initialised",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Error() = %q; want %q", result, tt.expected)
			}
		})
	}
}

func TestConfigError_Unwrap(t *testing.T) {
	underlying := ErrInvalidInput
	err := NewConfigError("config.yaml", "port", underlying)

	if !errors.Is(err, underlying) {
		t.Error("ConfigError should unwrap to underlying error")
	}

	var configErr *ConfigError
	if !errors.As(err, &configErr) {
		t.Error("errors.As should find ConfigError")
	}
	if configErr.File != "config.yaml" {
		t.Errorf("File = %q; want %q", configErr.File, "config.yaml")
	}
}

func TestNewConfigError(t *testing.T) {
	err := NewConfigError("/path/to/config", "api_key", ErrNotFound)

	if err.File != "/path/to/config" {
		t.Errorf("File = %q; want %q", err.File, "/path/to/config")
	}
	if err.Field != "api_key" {
		t.Errorf("Field = %q; want %q", err.Field, "api_key")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Error("should wrap ErrNotFound")
	}
}

// =============================================================================
// Wrap Helper Tests
// =============================================================================

func TestWrap(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		result := Wrap(nil, "context")
		if result != nil {
			t.Errorf("Wrap(nil, ...) = %v; want nil", result)
		}
	})

	t.Run("wraps with context", func(t *testing.T) {
		original := ErrNotFound
		wrapped := Wrap(original, "loading config")

		if !errors.Is(wrapped, original) {
			t.Error("wrapped error should match original with errors.Is")
		}

		expected := "loading config: not found"
		if wrapped.Error() != expected {
			t.Errorf("wrapped.Error() = %q; want %q", wrapped.Error(), expected)
		}
	})
}

func TestWrapf(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		result := Wrapf(nil, "context %d", 42)
		if result != nil {
			t.Errorf("Wrapf(nil, ...) = %v; want nil", result)
		}
	})

	t.Run("wraps with formatted context", func(t *testing.T) {
		original := ErrTimeout
		wrapped := Wrapf(original, "operation %s after %d retries", "fetch", 3)

		if !errors.Is(wrapped, original) {
			t.Error("wrapped error should match original with errors.Is")
		}

		expected := "operation fetch after 3 retries: timeout"
		if wrapped.Error() != expected {
			t.Errorf("wrapped.Error() = %q; want %q", wrapped.Error(), expected)
		}
	})
}

// =============================================================================
// Re-exported Functions Tests
// =============================================================================

func TestIs(t *testing.T) {
	wrapped := Wrap(ErrNotFound, "context")
	if !Is(wrapped, ErrNotFound) {
		t.Error("Is should find wrapped error")
	}
	if Is(wrapped, ErrTimeout) {
		t.Error("Is should not match different error")
	}
}

func TestAs(t *testing.T) {
	cloudErr := NewCloudError("gcp", "create", "vm-1", ErrPermissionDenied)
	wrapped := Wrap(cloudErr, "context")

	var target *CloudError
	if !As(wrapped, &target) {
		t.Error("As should find CloudError in chain")
	}
	if target.Provider != "gcp" {
		t.Errorf("Provider = %q; want %q", target.Provider, "gcp")
	}
}

func TestNew(t *testing.T) {
	err := New("test error")
	if err == nil {
		t.Error("New should return non-nil error")
	}
	if err.Error() != "test error" {
		t.Errorf("Error() = %q; want %q", err.Error(), "test error")
	}
}

func TestJoin(t *testing.T) {
	err1 := New("error 1")
	err2 := New("error 2")

	joined := Join(err1, nil, err2)
	if joined == nil {
		t.Error("Join should return non-nil error")
	}
	if !Is(joined, err1) {
		t.Error("joined error should contain err1")
	}
	if !Is(joined, err2) {
		t.Error("joined error should contain err2")
	}
}

// =============================================================================
// Error Chain Integration Tests
// =============================================================================

func TestErrorChaining(t *testing.T) {
	// Create a realistic error chain like what would happen in the application
	// 1. Start with a sentinel error
	// 2. Wrap in a typed error
	// 3. Wrap with context

	baseErr := ErrPermissionDenied
	sshErr := NewSSHError("10.0.0.5", "sudo systemctl restart agent", baseErr)
	contextErr := Wrapf(sshErr, "failed to restart agent on %s", "sandbox-vm")

	// Can still detect the base error
	if !Is(contextErr, ErrPermissionDenied) {
		t.Error("should detect ErrPermissionDenied through chain")
	}

	// Can extract the SSHError
	var extracted *SSHError
	if !As(contextErr, &extracted) {
		t.Error("should extract SSHError from chain")
	}
	if extracted.Host != "10.0.0.5" {
		t.Errorf("Host = %q; want %q", extracted.Host, "10.0.0.5")
	}

	// Error message has full context
	expectedMsg := `failed to restart agent on sandbox-vm: ssh to 10.0.0.5 failed executing "sudo systemctl restart agent": permission denied`
	if contextErr.Error() != expectedMsg {
		t.Errorf("Error() = %q; want %q", contextErr.Error(), expectedMsg)
	}
}
