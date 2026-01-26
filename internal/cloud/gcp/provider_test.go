package gcp

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
)

func TestProvider_Name(t *testing.T) {
	p := newWithClient("proj", "zone", &mockInstancesClient{})
	if got := p.Name(); got != "gcp" {
		t.Errorf("Name() = %q, want %q", got, "gcp")
	}
}

func TestProvider_GetVMInfo(t *testing.T) {
	tests := []struct {
		name       string
		vmName     string
		mock       *mockInstancesClient
		wantInfo   *cloud.VMInfo
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:   "running instance with IPs",
			vmName: "test-vm",
			mock: &mockInstancesClient{
				getInstance: newTestInstanceWithIPs("test-vm", "RUNNING", "us-central1-a", "c4a-highcpu-4", "10.0.0.1", "34.1.2.3"),
			},
			wantInfo: &cloud.VMInfo{
				Name:        "test-vm",
				Status:      cloud.VMStatusRunning,
				Zone:        "us-central1-a",
				MachineType: "c4a-highcpu-4",
				InternalIP:  "10.0.0.1",
				ExternalIP:  "34.1.2.3",
			},
		},
		{
			name:   "stopped instance",
			vmName: "stopped-vm",
			mock: &mockInstancesClient{
				getInstance: newTestInstance("stopped-vm", "STOPPED", "europe-north2-a", "e2-micro"),
			},
			wantInfo: &cloud.VMInfo{
				Name:        "stopped-vm",
				Status:      cloud.VMStatusStopped,
				Zone:        "europe-north2-a",
				MachineType: "e2-micro",
			},
		},
		{
			name:   "terminated instance",
			vmName: "term-vm",
			mock: &mockInstancesClient{
				getInstance: newTestInstance("term-vm", "TERMINATED", "us-west1-b", "n1-standard-1"),
			},
			wantInfo: &cloud.VMInfo{
				Name:        "term-vm",
				Status:      cloud.VMStatusStopped,
				Zone:        "us-west1-b",
				MachineType: "n1-standard-1",
			},
		},
		{
			name:   "staging instance",
			vmName: "staging-vm",
			mock: &mockInstancesClient{
				getInstance: newTestInstance("staging-vm", "STAGING", "us-east1-c", "e2-small"),
			},
			wantInfo: &cloud.VMInfo{
				Name:        "staging-vm",
				Status:      cloud.VMStatusStarting,
				Zone:        "us-east1-c",
				MachineType: "e2-small",
			},
		},
		{
			name:   "stopping instance",
			vmName: "stopping-vm",
			mock: &mockInstancesClient{
				getInstance: newTestInstance("stopping-vm", "STOPPING", "asia-east1-a", "c4a-highcpu-8"),
			},
			wantInfo: &cloud.VMInfo{
				Name:        "stopping-vm",
				Status:      cloud.VMStatusStopping,
				Zone:        "asia-east1-a",
				MachineType: "c4a-highcpu-8",
			},
		},
		{
			name:   "instance not found",
			vmName: "missing-vm",
			mock: &mockInstancesClient{
				getError: newNotFoundError(),
			},
			wantInfo: &cloud.VMInfo{
				Name:   "missing-vm",
				Status: cloud.VMStatusNotFound,
			},
		},
		{
			name:   "permission denied",
			vmName: "forbidden-vm",
			mock: &mockInstancesClient{
				getError: newForbiddenError(),
			},
			wantErr:    true,
			wantErrMsg: "get instance forbidden-vm",
		},
		{
			name:   "unknown error",
			vmName: "error-vm",
			mock: &mockInstancesClient{
				getError: errors.New("network error"),
			},
			wantErr:    true,
			wantErrMsg: "get instance error-vm",
		},
		{
			name:   "instance with internal IP only",
			vmName: "internal-only",
			mock: &mockInstancesClient{
				getInstance: func() *computepb.Instance {
					inst := newTestInstance("internal-only", "RUNNING", "us-central1-a", "e2-micro")
					inst.NetworkInterfaces = []*computepb.NetworkInterface{
						{NetworkIP: strPtr("10.0.0.5")},
					}
					return inst
				}(),
			},
			wantInfo: &cloud.VMInfo{
				Name:        "internal-only",
				Status:      cloud.VMStatusRunning,
				Zone:        "us-central1-a",
				MachineType: "e2-micro",
				InternalIP:  "10.0.0.5",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newWithClient("test-project", "test-zone", tt.mock)
			info, err := p.GetVMInfo(context.Background(), tt.vmName)

			if (err != nil) != tt.wantErr {
				t.Fatalf("GetVMInfo() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				if tt.wantErrMsg != "" && !contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("GetVMInfo() error = %v, want containing %q", err, tt.wantErrMsg)
				}
				return
			}

			if info.Name != tt.wantInfo.Name {
				t.Errorf("Name = %q, want %q", info.Name, tt.wantInfo.Name)
			}
			if info.Status != tt.wantInfo.Status {
				t.Errorf("Status = %q, want %q", info.Status, tt.wantInfo.Status)
			}
			if info.Zone != tt.wantInfo.Zone {
				t.Errorf("Zone = %q, want %q", info.Zone, tt.wantInfo.Zone)
			}
			if info.MachineType != tt.wantInfo.MachineType {
				t.Errorf("MachineType = %q, want %q", info.MachineType, tt.wantInfo.MachineType)
			}
			if info.InternalIP != tt.wantInfo.InternalIP {
				t.Errorf("InternalIP = %q, want %q", info.InternalIP, tt.wantInfo.InternalIP)
			}
			if info.ExternalIP != tt.wantInfo.ExternalIP {
				t.Errorf("ExternalIP = %q, want %q", info.ExternalIP, tt.wantInfo.ExternalIP)
			}

			// Verify the request was made correctly
			if !tt.mock.getCalled {
				t.Error("Get() was not called")
			}
			if tt.mock.lastGetReq.GetInstance() != tt.vmName {
				t.Errorf("Get() called with instance = %q, want %q", tt.mock.lastGetReq.GetInstance(), tt.vmName)
			}
		})
	}
}

func TestProvider_StartVM(t *testing.T) {
	tests := []struct {
		name       string
		vmName     string
		mock       *mockInstancesClient
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:   "successful start",
			vmName: "test-vm",
			mock:   &mockInstancesClient{},
		},
		{
			name:   "start error",
			vmName: "error-vm",
			mock: &mockInstancesClient{
				startError: errors.New("start failed"),
			},
			wantErr:    true,
			wantErrMsg: "start instance error-vm",
		},
		{
			name:   "wait error",
			vmName: "wait-error-vm",
			mock: &mockInstancesClient{
				waitError: errors.New("operation failed"),
			},
			wantErr:    true,
			wantErrMsg: "wait for start wait-error-vm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newWithClient("test-project", "test-zone", tt.mock)
			err := p.StartVM(context.Background(), tt.vmName)

			if (err != nil) != tt.wantErr {
				t.Fatalf("StartVM() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				if tt.wantErrMsg != "" && !contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("StartVM() error = %v, want containing %q", err, tt.wantErrMsg)
				}
				return
			}

			if !tt.mock.startCalled {
				t.Error("Start() was not called")
			}
			if tt.mock.lastStartReq.GetInstance() != tt.vmName {
				t.Errorf("Start() called with instance = %q, want %q", tt.mock.lastStartReq.GetInstance(), tt.vmName)
			}
		})
	}
}

func TestProvider_StopVM(t *testing.T) {
	tests := []struct {
		name       string
		vmName     string
		mock       *mockInstancesClient
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:   "successful stop",
			vmName: "test-vm",
			mock:   &mockInstancesClient{},
		},
		{
			name:   "stop error",
			vmName: "error-vm",
			mock: &mockInstancesClient{
				stopError: errors.New("stop failed"),
			},
			wantErr:    true,
			wantErrMsg: "stop instance error-vm",
		},
		{
			name:   "wait error",
			vmName: "wait-error-vm",
			mock: &mockInstancesClient{
				waitError: errors.New("operation failed"),
			},
			wantErr:    true,
			wantErrMsg: "wait for stop wait-error-vm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newWithClient("test-project", "test-zone", tt.mock)
			err := p.StopVM(context.Background(), tt.vmName)

			if (err != nil) != tt.wantErr {
				t.Fatalf("StopVM() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				if tt.wantErrMsg != "" && !contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("StopVM() error = %v, want containing %q", err, tt.wantErrMsg)
				}
				return
			}

			if !tt.mock.stopCalled {
				t.Error("Stop() was not called")
			}
			if tt.mock.lastStopReq.GetInstance() != tt.vmName {
				t.Errorf("Stop() called with instance = %q, want %q", tt.mock.lastStopReq.GetInstance(), tt.vmName)
			}
		})
	}
}

func TestProvider_Close(t *testing.T) {
	t.Run("successful close", func(t *testing.T) {
		mock := &mockInstancesClient{}
		p := newWithClient("proj", "zone", mock)

		err := p.Close()
		if err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}
		if !mock.closeCalled {
			t.Error("Close() was not called on client")
		}
	})

	t.Run("close error", func(t *testing.T) {
		mock := &mockInstancesClient{closeError: errors.New("close failed")}
		p := newWithClient("proj", "zone", mock)

		err := p.Close()
		if err == nil {
			t.Error("Close() error = nil, want error")
		}
	})

	t.Run("close with nil client", func(t *testing.T) {
		p := &Provider{project: "proj", zone: "zone", client: nil}
		err := p.Close()
		if err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}
	})
}

func TestMapStatus(t *testing.T) {
	tests := []struct {
		gcpStatus string
		want      cloud.VMStatus
	}{
		{"RUNNING", cloud.VMStatusRunning},
		{"TERMINATED", cloud.VMStatusStopped},
		{"STOPPED", cloud.VMStatusStopped},
		{"STOPPING", cloud.VMStatusStopping},
		{"SUSPENDING", cloud.VMStatusStopping},
		{"STAGING", cloud.VMStatusStarting},
		{"PROVISIONING", cloud.VMStatusStarting},
		{"WEIRD_STATUS", cloud.VMStatusUnknown},
		{"", cloud.VMStatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.gcpStatus, func(t *testing.T) {
			if got := mapStatus(tt.gcpStatus); got != tt.want {
				t.Errorf("mapStatus(%q) = %v, want %v", tt.gcpStatus, got, tt.want)
			}
		})
	}
}

func TestExtractZoneName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"projects/my-project/zones/us-central1-a", "us-central1-a"},
		{"projects/my-project/zones/europe-north1-b", "europe-north1-b"},
		{"us-central1-a", "us-central1-a"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := extractZoneName(tt.input); got != tt.want {
				t.Errorf("extractZoneName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractMachineTypeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"projects/my-project/zones/us-central1-a/machineTypes/c4a-highcpu-4", "c4a-highcpu-4"},
		{"projects/my-project/zones/us-central1-a/machineTypes/n1-standard-1", "n1-standard-1"},
		{"c4a-highcpu-4", "c4a-highcpu-4"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := extractMachineTypeName(tt.input); got != tt.want {
				t.Errorf("extractMachineTypeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestProviderInterface(t *testing.T) {
	// Ensure Provider implements cloud.Provider
	var _ cloud.Provider = (*Provider)(nil)
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func strPtr(s string) *string {
	return &s
}
