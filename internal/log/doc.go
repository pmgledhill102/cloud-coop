// Package log provides structured logging for the cloudcoop project.
//
// This package wraps Go's stdlib slog (log/slog) to provide a consistent
// logging interface across the codebase. It supports both JSON output
// (for production/structured logging) and text output (for development).
//
// # Configuration
//
// The log level can be configured via the LOG_LEVEL environment variable:
//
//	LOG_LEVEL=debug  # Enable debug logging
//	LOG_LEVEL=info   # Default level
//	LOG_LEVEL=warn   # Only warnings and errors
//	LOG_LEVEL=error  # Only errors
//
// The log format can be configured via the LOG_FORMAT environment variable:
//
//	LOG_FORMAT=json  # JSON output (default for production)
//	LOG_FORMAT=text  # Human-readable text output
//
// # Basic Usage
//
//	import "github.com/cloud-coop/cloudcoop/internal/log"
//
//	func main() {
//	    // Initialise with default settings (reads from environment)
//	    log.Init()
//
//	    // Basic logging
//	    log.Info("server started", "port", 8080)
//	    log.Error("connection failed", "error", err, "host", "example.com")
//
//	    // Debug logging (only shown when LOG_LEVEL=debug)
//	    log.Debug("processing request", "id", requestID)
//	}
//
// # Context-Aware Logging
//
// For request-scoped or operation-scoped logging, use context:
//
//	func handleRequest(ctx context.Context, req *Request) {
//	    // Add fields to context for all subsequent log calls
//	    ctx = log.With(ctx, "request_id", req.ID, "user", req.User)
//
//	    // These logs will include request_id and user
//	    log.InfoContext(ctx, "processing request")
//	    processOrder(ctx, req.OrderID)
//	}
//
//	func processOrder(ctx context.Context, orderID string) {
//	    // Inherits request_id and user from context
//	    log.InfoContext(ctx, "processing order", "order_id", orderID)
//	}
//
// # Creating Sub-Loggers
//
// For component-specific logging, create a sub-logger with default attributes:
//
//	var logger = log.With("component", "ssh")
//
//	func connect(host string) {
//	    logger.Info("connecting", "host", host)
//	    // Output: ... component=ssh msg=connecting host=example.com
//	}
package log
