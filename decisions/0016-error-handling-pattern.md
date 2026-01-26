# ADR-0016: Error Handling Pattern

## Status

Accepted

## Context

cloudcoop needs a consistent error handling strategy across its codebase. The application interacts
with multiple external systems (cloud providers, SSH connections, configuration files) and must
provide clear, actionable error messages while properly propagating errors up the call stack.

Key requirements:

- Errors should preserve context as they propagate
- Domain-specific errors need structured information (e.g., which cloud provider failed, which host)
- CLI exit codes must follow Unix conventions
- Standard Go idioms should be used (`errors.Is`, `errors.As`, error wrapping)

## Decision

Implement a custom `apperrors` package that provides:

1. **Sentinel errors** for common failure cases
2. **Domain-specific error types** with structured fields
3. **Wrap/Wrapf helpers** for adding context
4. **Exit code mapping** for CLI integration
5. **Re-exported standard library functions** for convenience

## Implementation

### Exit Codes

Follow standard Unix conventions:

```go
const (
    ExitSuccess = 0  // Command completed successfully
    ExitError   = 1  // General error occurred
    ExitUsage   = 2  // Incorrect command usage
)
```

### Sentinel Errors

Define common failure modes as package-level variables that can be checked with `errors.Is`:

```go
var (
    ErrNotFound         = errors.New("not found")
    ErrAlreadyExists    = errors.New("already exists")
    ErrInvalidInput     = errors.New("invalid input")
    ErrTimeout          = errors.New("timeout")
    ErrCanceled         = errors.New("canceled")
    ErrPermissionDenied = errors.New("permission denied")
    ErrUnavailable      = errors.New("unavailable")
)
```

### Domain-Specific Error Types

#### CloudError

For cloud provider operations:

```go
type CloudError struct {
    Provider  string  // "gcp", "aws", "azure"
    Operation string  // "create-vm", "start-vm", etc.
    Resource  string  // Resource identifier (optional)
    Err       error   // Underlying error
}
```

Produces messages like: `gcp create-vm failed for my-instance: quota exceeded`

#### SSHError

For SSH and remote execution operations:

```go
type SSHError struct {
    Host    string  // Remote host
    Command string  // Failed command (optional)
    Err     error   // Underlying error
}
```

Produces messages like: `ssh to 10.0.0.1 failed executing "tmux list-sessions": connection refused`

#### ConfigError

For configuration-related errors:

```go
type ConfigError struct {
    File  string  // Configuration file path (optional)
    Field string  // Configuration field name (optional)
    Err   error   // Underlying error
}
```

Produces messages like:
`config error in ~/.config/cloudcoop/cloudcoop.toml field "cloud.provider": must be one of: gcp, aws, azure`

All domain-specific types implement `Unwrap()` to support error chain traversal.

### Wrap Helpers

Convenience functions for adding context while preserving the error chain:

```go
// Wrap adds a string context prefix
err := apperrors.Wrap(originalErr, "loading configuration")
// Result: "loading configuration: <original error>"

// Wrapf adds a formatted context prefix
err := apperrors.Wrapf(originalErr, "failed to connect to %s", host)
// Result: "failed to connect to example.com: <original error>"
```

Both return `nil` if the input error is `nil`.

### Exit Code Mapping

The `ExitCodeFor` function maps errors to appropriate exit codes:

```go
func ExitCodeFor(err error) int {
    if err == nil {
        return ExitSuccess
    }
    if errors.Is(err, ErrInvalidInput) {
        return ExitUsage
    }
    return ExitError
}
```

### Usage Patterns

**Creating domain errors:**

```go
return apperrors.NewCloudError("gcp", "create-vm", instanceName, err)
return apperrors.NewSSHError(host, command, err)
return apperrors.NewConfigError(configPath, fieldName, err)
```

**Wrapping with context:**

```go
if err := doSomething(); err != nil {
    return apperrors.Wrap(err, "doing something")
}
```

**Checking error types:**

```go
// Check for sentinel errors
if apperrors.Is(err, apperrors.ErrNotFound) {
    // Handle not found case
}

// Extract domain-specific error
var cloudErr *apperrors.CloudError
if apperrors.As(err, &cloudErr) {
    log.Printf("Cloud operation %s failed on %s", cloudErr.Operation, cloudErr.Provider)
}
```

**CLI exit handling:**

```go
func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(apperrors.ExitCodeFor(err))
    }
}
```

## Options Considered

### Option 1: Standard Library Only

Use only `fmt.Errorf` with `%w` and raw error strings.

**Pros:**

- No custom code to maintain
- Familiar to all Go developers

**Cons:**

- No structured error information
- Repetitive wrapping code
- Exit code logic scattered across codebase
- Harder to extract context for logging/telemetry

### Option 2: Third-Party Error Library (e.g., pkg/errors, cockroachdb/errors)

Use an established error handling library.

**Pros:**

- Battle-tested code
- Often includes stack traces
- Rich feature set

**Cons:**

- External dependency
- May include features we don't need
- Stack traces add overhead and can leak internal details

### Option 3: Custom apperrors Package (Chosen)

Build a focused package tailored to cloudcoop's needs.

**Pros:**

- Exactly the features we need, nothing more
- Domain-specific error types match our architecture
- No external dependencies
- Simple, easy to understand

**Cons:**

- Code to maintain (minimal, ~200 lines)
- Team must learn package conventions

## Consequences

### Positive

- Consistent error handling across the codebase
- Structured errors enable better logging and debugging
- Error chains preserve full context
- Exit codes follow Unix conventions
- Domain errors make failures actionable (e.g., "GCP operation failed" vs generic error)

### Negative

- Team must learn to use apperrors consistently
- Slight overhead compared to raw error strings

### Neutral

- All standard library error functions are re-exported for convenience (`Is`, `As`, `New`, `Join`)
- Domain error types can be extended as new failure modes are identified
