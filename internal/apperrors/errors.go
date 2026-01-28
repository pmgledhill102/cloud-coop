// Package apperrors provides consistent error handling patterns for cloudcoop.
// It defines exit codes, custom error types for different failure modes,
// and error wrapping helpers that preserve context.
package apperrors

import (
	"errors"
	"fmt"
)

// Exit codes for the CLI. These follow standard Unix conventions.
const (
	// ExitSuccess indicates the command completed successfully.
	ExitSuccess = 0
	// ExitError indicates a general error occurred.
	ExitError = 1
	// ExitUsage indicates incorrect command usage (bad flags, missing args, etc.).
	ExitUsage = 2
)

// Sentinel errors for common failure cases.
// Use errors.Is to check for these errors.
var (
	// ErrNotFound indicates a requested resource does not exist.
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists indicates a resource already exists when trying to create.
	ErrAlreadyExists = errors.New("already exists")
	// ErrInvalidInput indicates the provided input failed validation.
	ErrInvalidInput = errors.New("invalid input")
	// ErrTimeout indicates an operation timed out.
	ErrTimeout = errors.New("timeout")
	// ErrCanceled indicates an operation was canceled.
	ErrCanceled = errors.New("canceled")
	// ErrPermissionDenied indicates insufficient permissions for the operation.
	ErrPermissionDenied = errors.New("permission denied")
	// ErrUnavailable indicates a service or resource is temporarily unavailable.
	ErrUnavailable = errors.New("unavailable")
)

// CloudError represents errors from cloud provider operations.
// It captures the provider, operation, and underlying cause.
type CloudError struct {
	// Provider is the cloud provider name (e.g., "gcp", "aws", "azure").
	Provider string
	// Operation is the failed operation (e.g., "create-vm", "start-vm").
	Operation string
	// Resource is the resource identifier if applicable.
	Resource string
	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *CloudError) Error() string {
	if e.Resource != "" {
		return fmt.Sprintf("%s %s failed for %s: %v", e.Provider, e.Operation, e.Resource, e.Err)
	}
	return fmt.Sprintf("%s %s failed: %v", e.Provider, e.Operation, e.Err)
}

// Unwrap returns the underlying error for errors.Is and errors.As.
func (e *CloudError) Unwrap() error {
	return e.Err
}

// NewCloudError creates a new CloudError with the given details.
func NewCloudError(provider, operation, resource string, err error) *CloudError {
	return &CloudError{
		Provider:  provider,
		Operation: operation,
		Resource:  resource,
		Err:       err,
	}
}

// SSHError represents errors from SSH operations.
// It captures the host, command, and underlying cause.
type SSHError struct {
	// Host is the remote host where the operation failed.
	Host string
	// Command is the command that failed, if applicable.
	Command string
	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *SSHError) Error() string {
	if e.Command != "" {
		return fmt.Sprintf("ssh to %s failed executing %q: %v", e.Host, e.Command, e.Err)
	}
	return fmt.Sprintf("ssh to %s failed: %v", e.Host, e.Err)
}

// Unwrap returns the underlying error for errors.Is and errors.As.
func (e *SSHError) Unwrap() error {
	return e.Err
}

// NewSSHError creates a new SSHError with the given details.
func NewSSHError(host, command string, err error) *SSHError {
	return &SSHError{
		Host:    host,
		Command: command,
		Err:     err,
	}
}

// ConfigError represents errors related to configuration.
// It captures the configuration file/field and underlying cause.
type ConfigError struct {
	// File is the configuration file path, if applicable.
	File string
	// Field is the configuration field name, if applicable.
	Field string
	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *ConfigError) Error() string {
	switch {
	case e.File != "" && e.Field != "":
		return fmt.Sprintf("config error in %s field %q: %v", e.File, e.Field, e.Err)
	case e.File != "":
		return fmt.Sprintf("config error in %s: %v", e.File, e.Err)
	case e.Field != "":
		return fmt.Sprintf("config error in field %q: %v", e.Field, e.Err)
	default:
		return fmt.Sprintf("config error: %v", e.Err)
	}
}

// Unwrap returns the underlying error for errors.Is and errors.As.
func (e *ConfigError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a new ConfigError with the given details.
func NewConfigError(file, field string, err error) *ConfigError {
	return &ConfigError{
		File:  file,
		Field: field,
		Err:   err,
	}
}

// Wrap wraps an error with additional context message.
// If err is nil, Wrap returns nil.
// This is a convenience wrapper around fmt.Errorf with %w.
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// Wrapf wraps an error with a formatted context message.
// If err is nil, Wrapf returns nil.
// This is a convenience wrapper around fmt.Errorf with %w.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// ExitCodeFor returns the appropriate exit code for an error.
// It returns ExitSuccess for nil errors, and attempts to determine
// the most appropriate exit code based on the error type.
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitSuccess
	}
	if errors.Is(err, ErrInvalidInput) {
		return ExitUsage
	}
	return ExitError
}

// Is reports whether any error in err's tree matches target.
// This is a re-export of errors.Is for convenience.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's tree that matches target,
// and if so, sets target to that error value and returns true.
// This is a re-export of errors.As for convenience.
func As(err error, target any) bool {
	return errors.As(err, target)
}

// New returns an error that formats as the given text.
// This is a re-export of errors.New for convenience.
func New(text string) error {
	return errors.New(text)
}

// Join returns an error that wraps the given errors.
// Any nil error values are discarded.
// This is a re-export of errors.Join for convenience.
func Join(errs ...error) error {
	return errors.Join(errs...)
}

// UsageError represents a CLI usage error (invalid flags, unknown commands, etc.).
// These errors are already printed by Cobra, so main.go should not log them again.
type UsageError struct {
	// Err is the underlying error from Cobra.
	Err error
}

// Error implements the error interface.
func (e *UsageError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error for errors.Is and errors.As.
func (e *UsageError) Unwrap() error {
	return e.Err
}

// NewUsageError creates a new UsageError.
func NewUsageError(message string, suggestions []string, err error) *UsageError {
	return &UsageError{Err: err}
}

// IsUsageError checks if err is a UsageError.
func IsUsageError(err error) bool {
	var usageErr *UsageError
	return errors.As(err, &usageErr)
}
