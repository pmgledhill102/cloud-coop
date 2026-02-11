package ssh

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// mockCloser implements the closer interface for testing.
type mockCloser struct{}

func (mockCloser) Close() error { return nil }

func TestWaitForSSH_ImmediateSuccess(t *testing.T) {
	orig := newClientFunc
	defer func() { newClientFunc = orig }()

	newClientFunc = func(cfg Config) (closer, error) {
		return mockCloser{}, nil
	}

	cfg := DefaultConfig("127.0.0.1", "test")
	if err := WaitForSSH(cfg, 5*time.Second); err != nil {
		t.Fatalf("WaitForSSH() unexpected error: %v", err)
	}
}

func TestWaitForSSH_RetryThenSuccess(t *testing.T) {
	orig := newClientFunc
	defer func() { newClientFunc = orig }()

	var attempts atomic.Int32
	newClientFunc = func(cfg Config) (closer, error) {
		n := attempts.Add(1)
		if n < 3 {
			return nil, errors.New("connection refused")
		}
		return mockCloser{}, nil
	}

	cfg := DefaultConfig("127.0.0.1", "test")
	if err := WaitForSSH(cfg, 30*time.Second); err != nil {
		t.Fatalf("WaitForSSH() unexpected error: %v", err)
	}

	got := attempts.Load()
	if got < 3 {
		t.Errorf("expected at least 3 attempts, got %d", got)
	}
}

func TestWaitForSSH_Timeout(t *testing.T) {
	orig := newClientFunc
	defer func() { newClientFunc = orig }()

	newClientFunc = func(cfg Config) (closer, error) {
		return nil, errors.New("connection refused")
	}

	cfg := DefaultConfig("127.0.0.1", "test")
	start := time.Now()
	err := WaitForSSH(cfg, 3*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("WaitForSSH() expected error on timeout, got nil")
	}

	// Should have timed out roughly after the specified duration.
	if elapsed < 2*time.Second {
		t.Errorf("WaitForSSH() returned too quickly: %v", elapsed)
	}
	if elapsed > 6*time.Second {
		t.Errorf("WaitForSSH() took too long: %v", elapsed)
	}
}

func TestWaitForSSH_UsesShortAttemptTimeout(t *testing.T) {
	orig := newClientFunc
	defer func() { newClientFunc = orig }()

	newClientFunc = func(cfg Config) (closer, error) {
		// Verify the per-attempt timeout was overridden to 3s.
		if cfg.Timeout != 3*time.Second {
			t.Errorf("attempt timeout = %v, want 3s", cfg.Timeout)
		}
		return mockCloser{}, nil
	}

	cfg := DefaultConfig("127.0.0.1", "test")
	cfg.Timeout = 10 * time.Second // original timeout should be overridden
	if err := WaitForSSH(cfg, 5*time.Second); err != nil {
		t.Fatalf("WaitForSSH() unexpected error: %v", err)
	}
}
