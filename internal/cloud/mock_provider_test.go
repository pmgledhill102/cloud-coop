package cloud

import (
	"context"
	"errors"
	"testing"
)

func TestMockProvider(t *testing.T) {
	t.Run("NewMockProvider has sensible defaults", func(t *testing.T) {
		mock := NewMockProvider()

		if mock.ProviderName != "mock" {
			t.Errorf("Name = %q, want %q", mock.ProviderName, "mock")
		}
		if mock.VMInfoResponse == nil {
			t.Error("VMInfoResponse is nil, want non-nil")
		}
		if mock.VMInfoResponse.Status != VMStatusRunning {
			t.Errorf("Status = %v, want %v", mock.VMInfoResponse.Status, VMStatusRunning)
		}
	})

	t.Run("Name returns configured name", func(t *testing.T) {
		mock := &MockProvider{ProviderName: "test-provider"}
		if got := mock.Name(); got != "test-provider" {
			t.Errorf("Name() = %q, want %q", got, "test-provider")
		}
	})

	t.Run("GetVMInfo returns configured response", func(t *testing.T) {
		mock := NewMockProvider()
		info, err := mock.GetVMInfo(context.Background(), "test-vm")

		if err != nil {
			t.Errorf("GetVMInfo() error = %v", err)
		}
		if info.Status != VMStatusRunning {
			t.Errorf("Status = %v, want %v", info.Status, VMStatusRunning)
		}
	})

	t.Run("GetVMInfo returns configured error", func(t *testing.T) {
		mock := NewMockProvider().WithVMInfoError(errors.New("test error"))
		_, err := mock.GetVMInfo(context.Background(), "test-vm")

		if err == nil {
			t.Error("GetVMInfo() expected error, got nil")
		}
	})

	t.Run("StartVM returns configured error", func(t *testing.T) {
		mock := NewMockProvider().WithStartVMError(errors.New("start failed"))
		err := mock.StartVM(context.Background(), "test-vm")

		if err == nil {
			t.Error("StartVM() expected error, got nil")
		}
	})

	t.Run("StopVM returns configured error", func(t *testing.T) {
		mock := NewMockProvider().WithStopVMError(errors.New("stop failed"))
		err := mock.StopVM(context.Background(), "test-vm")

		if err == nil {
			t.Error("StopVM() expected error, got nil")
		}
	})

	t.Run("WithVMStatus modifies status", func(t *testing.T) {
		mock := NewMockProvider().WithVMStatus(VMStatusStopped)
		info, _ := mock.GetVMInfo(context.Background(), "test-vm")

		if info.Status != VMStatusStopped {
			t.Errorf("Status = %v, want %v", info.Status, VMStatusStopped)
		}
	})

	t.Run("call log records method calls", func(t *testing.T) {
		mock := NewMockProvider()
		mock.Name()
		mock.GetVMInfo(context.Background(), "vm1")
		mock.StartVM(context.Background(), "vm2")
		mock.StopVM(context.Background(), "vm3")

		calls := mock.GetCalls()
		if len(calls) != 4 {
			t.Errorf("len(calls) = %d, want 4", len(calls))
		}

		expected := []string{"Name", "GetVMInfo", "StartVM", "StopVM"}
		for i, call := range calls {
			if call.Method != expected[i] {
				t.Errorf("calls[%d].Method = %q, want %q", i, call.Method, expected[i])
			}
		}
	})

	t.Run("Reset clears call log", func(t *testing.T) {
		mock := NewMockProvider()
		mock.Name()
		mock.Reset()

		if len(mock.GetCalls()) != 0 {
			t.Error("Reset() should clear call log")
		}
	})

	t.Run("method chaining works", func(t *testing.T) {
		mock := NewMockProvider().
			WithVMStatus(VMStatusStopped).
			WithStartVMError(errors.New("err"))

		info, _ := mock.GetVMInfo(context.Background(), "vm")
		if info.Status != VMStatusStopped {
			t.Error("chaining WithVMStatus failed")
		}

		err := mock.StartVM(context.Background(), "vm")
		if err == nil {
			t.Error("chaining WithStartVMError failed")
		}
	})
}
