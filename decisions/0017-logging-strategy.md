# ADR-0017: Logging Strategy

## Status

Accepted

## Context

cloudcoop needs consistent, structured logging across all components for debugging, monitoring, and
operational visibility. The codebase includes various modules (TUI, cloud providers, SSH operations,
agent management) that all need to log diagnostic information.

Key requirements:

- Structured logging for machine-parseable output in production
- Human-readable output during development
- Configurable log levels without code changes
- Context propagation for request/operation tracing
- Minimal dependencies (prefer stdlib)

## Decision

Use Go's standard library `log/slog` package wrapped in an internal `internal/log` package that
provides:

1. **Environment-based configuration** via LOG_LEVEL and LOG_FORMAT variables
2. **Context-aware logging** for propagating attributes through call chains
3. **Component sub-loggers** for adding consistent attributes per module
4. **Thread-safe initialization** with sensible defaults

## Options Considered

### Option 1: Direct slog Usage

Use `log/slog` directly throughout the codebase without a wrapper.

**Pros:**

- Zero abstraction overhead
- No internal package to maintain
- Developers already familiar with stdlib

**Cons:**

- No centralized configuration
- Each package must handle its own logger setup
- Inconsistent initialization patterns across codebase

### Option 2: Third-party Library (zerolog, zap)

Use a mature third-party structured logging library.

**Pros:**

- Battle-tested implementations
- Rich feature sets (sampling, hooks, etc.)
- Strong community support

**Cons:**

- External dependency to track and update
- Different API from stdlib (learning curve)
- May include features we don't need

### Option 3: Internal slog Wrapper (Chosen)

Wrap slog in an internal package with configuration and convenience functions.

**Pros:**

- Builds on stdlib (no external dependencies)
- Centralized configuration via environment variables
- Context-aware logging with attribute propagation
- Consistent API across codebase
- Easy to swap implementation if needed

**Cons:**

- Small amount of wrapper code to maintain
- Thin abstraction layer

## Implementation

### Environment Variables

| Variable | Values | Default | Description |
|----------|--------|---------|-------------|
| LOG_LEVEL | debug, info, warn, error | info | Minimum log level |
| LOG_FORMAT | text, json | text | Output format |

### Basic Usage

```go
import "github.com/cloud-coop/cloudcoop/internal/log"

func main() {
    // Initialize with environment-based configuration
    log.Init()

    // Basic structured logging
    log.Info("server started", "port", 8080)
    log.Error("connection failed", log.Err(err), "host", hostname)
    log.Debug("verbose detail", "bytes", len(data))
}
```

### Context-Aware Logging

For operations that span multiple function calls, use context to propagate attributes:

```go
func handleOperation(ctx context.Context, vmID string) error {
    // Add operation-scoped attributes to context
    ctx = log.WithContext(ctx, "vm_id", vmID, "operation", "start")

    // All subsequent logs include vm_id and operation
    log.InfoContext(ctx, "beginning operation")

    if err := performStep(ctx); err != nil {
        log.ErrorContext(ctx, "step failed", log.Err(err))
        return err
    }

    log.InfoContext(ctx, "operation complete")
    return nil
}
```

### Component Sub-Loggers

For module-specific logging with consistent attributes:

```go
package ssh

import "github.com/cloud-coop/cloudcoop/internal/log"

var logger = log.With("component", "ssh")

func Connect(host string) error {
    logger.Info("connecting", "host", host)
    // Output: ... component=ssh msg=connecting host=example.com
}
```

### Output Examples

Text format (LOG_FORMAT=text):

```text
time=2024-01-15T10:30:00.000Z level=INFO msg="server started" port=8080
time=2024-01-15T10:30:01.000Z level=ERROR msg="connection failed" error="dial timeout" host=example.com
```

JSON format (LOG_FORMAT=json):

```json
{"time":"2024-01-15T10:30:00.000Z","level":"INFO","msg":"server started","port":8080}
{"time":"2024-01-15T10:30:01.000Z","level":"ERROR","msg":"connection failed","error":"dial timeout","host":"example.com"}
```

## Consequences

### Positive

- Consistent logging across all components
- Zero external dependencies (stdlib only)
- Easy configuration via environment variables
- Context propagation enables request tracing
- JSON format ready for log aggregation systems
- Familiar slog-based API for Go developers

### Negative

- Thin wrapper adds small maintenance burden
- No advanced features (sampling, hooks, log rotation)

### Neutral

- Logs to stderr by default (standard practice for CLI tools)
- TUI operations may need to suppress or redirect logs to avoid display corruption
