package cloud

import "testing"

func TestVMStatus_String(t *testing.T) {
	tests := []struct {
		status VMStatus
		want   string
	}{
		{VMStatusRunning, "running"},
		{VMStatusStopped, "stopped"},
		{VMStatusNotFound, "not_found"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("VMStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVMStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		status   VMStatus
		terminal bool
	}{
		{VMStatusRunning, true},
		{VMStatusStopped, true},
		{VMStatusNotFound, true},
		{VMStatusStarting, false},
		{VMStatusStopping, false},
		{VMStatusUnknown, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.terminal {
				t.Errorf("VMStatus.IsTerminal() = %v, want %v", got, tt.terminal)
			}
		})
	}
}
