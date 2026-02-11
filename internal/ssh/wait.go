package ssh

import (
	"fmt"
	"time"
)

// newClientFunc is the function used to create SSH clients.
// It defaults to NewClient but can be overridden in tests.
var newClientFunc = func(cfg Config) (closer, error) {
	return NewClient(cfg)
}

// closer is the minimal interface needed to close a probe connection.
type closer interface {
	Close() error
}

// WaitForSSH polls until an SSH connection can be established or timeout expires.
// Each attempt uses a short per-attempt timeout (3s) to fail fast on unreachable hosts.
// On success the probe connection is closed and nil is returned.
// On timeout the last connection error is returned.
func WaitForSSH(cfg Config, timeout time.Duration) error {
	const (
		attemptTimeout = 3 * time.Second
		retryInterval  = 2 * time.Second
	)

	deadline := time.Now().Add(timeout)
	attemptCfg := cfg
	attemptCfg.Timeout = attemptTimeout

	var lastErr error
	for time.Now().Before(deadline) {
		client, err := newClientFunc(attemptCfg)
		if err == nil {
			_ = client.Close()
			return nil
		}
		lastErr = err

		// Sleep until next attempt, but respect the deadline.
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		sleep := retryInterval
		if sleep > remaining {
			sleep = remaining
		}
		time.Sleep(sleep)
	}

	return fmt.Errorf("SSH not ready after %s: %w", timeout, lastErr)
}
