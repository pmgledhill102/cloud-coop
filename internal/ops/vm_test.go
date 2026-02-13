package ops

import (
	"errors"
	"testing"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
)

func TestValidateForStart(t *testing.T) {
	tests := []struct {
		name    string
		status  cloud.VMStatus
		wantErr error
		wantNil bool
	}{
		{"stopped allows start", cloud.VMStatusStopped, nil, true},
		{"running returns already running", cloud.VMStatusRunning, ErrVMAlreadyRunning, false},
		{"starting returns starting", cloud.VMStatusStarting, ErrVMStarting, false},
		{"stopping returns stopping", cloud.VMStatusStopping, ErrVMStopping, false},
		{"not found returns not found", cloud.VMStatusNotFound, ErrVMNotFound, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateForStart(&cloud.VMInfo{Status: tt.status})
			if tt.wantNil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateForStop(t *testing.T) {
	tests := []struct {
		name    string
		status  cloud.VMStatus
		wantErr error
		wantNil bool
	}{
		{"running allows stop", cloud.VMStatusRunning, nil, true},
		{"stopped returns already stopped", cloud.VMStatusStopped, ErrVMAlreadyStopped, false},
		{"stopping returns stopping", cloud.VMStatusStopping, ErrVMStopping, false},
		{"starting returns starting", cloud.VMStatusStarting, ErrVMStarting, false},
		{"not found returns not found", cloud.VMStatusNotFound, ErrVMNotFound, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateForStop(&cloud.VMInfo{Status: tt.status})
			if tt.wantNil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateForCreate(t *testing.T) {
	tests := []struct {
		name    string
		status  cloud.VMStatus
		wantErr error
		wantNil bool
	}{
		{"not found allows create", cloud.VMStatusNotFound, nil, true},
		{"running returns exists", cloud.VMStatusRunning, ErrVMExists, false},
		{"stopped returns exists", cloud.VMStatusStopped, ErrVMExists, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateForCreate(&cloud.VMInfo{Status: tt.status})
			if tt.wantNil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateForDelete(t *testing.T) {
	tests := []struct {
		name    string
		status  cloud.VMStatus
		wantErr error
		wantNil bool
	}{
		{"stopped allows delete", cloud.VMStatusStopped, nil, true},
		{"not found returns not found", cloud.VMStatusNotFound, ErrVMNotFound, false},
		{"running returns running", cloud.VMStatusRunning, ErrVMRunning, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateForDelete(&cloud.VMInfo{Status: tt.status})
			if tt.wantNil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
