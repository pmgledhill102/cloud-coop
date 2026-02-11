// cspell:disable -- test file contains fake SSH key material
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

func TestProvider_CreateVM(t *testing.T) {
	tests := []struct {
		name       string
		config     cloud.VMCreateConfig
		mock       *mockInstancesClient
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "successful create",
			config: cloud.VMCreateConfig{
				Name:           "new-vm",
				MachineType:    "c4a-highcpu-4",
				DiskSizeGB:     50,
				Image:          "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-arm64",
				Spot:           true,
				Network:        "default",
				Tags:           []string{"cloudcoop"},
				ServiceAccount: "cloudcoop-vm@test-project.iam.gserviceaccount.com",
			},
			mock: &mockInstancesClient{},
		},
		{
			name: "create without tags",
			config: cloud.VMCreateConfig{
				Name:           "no-tags-vm",
				MachineType:    "e2-micro",
				DiskSizeGB:     10,
				Image:          "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-arm64",
				Spot:           false,
				Network:        "default",
				ServiceAccount: "cloudcoop-vm@test-project.iam.gserviceaccount.com",
			},
			mock: &mockInstancesClient{},
		},
		{
			name: "missing service account",
			config: cloud.VMCreateConfig{
				Name:        "no-sa-vm",
				MachineType: "c4a-highcpu-4",
				DiskSizeGB:  50,
				Image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-arm64",
				Network:     "default",
			},
			mock:       &mockInstancesClient{},
			wantErr:    true,
			wantErrMsg: "service_account is required",
		},
		{
			name: "insert error",
			config: cloud.VMCreateConfig{
				Name:           "error-vm",
				MachineType:    "c4a-highcpu-4",
				DiskSizeGB:     50,
				Image:          "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-arm64",
				Network:        "default",
				ServiceAccount: "cloudcoop-vm@test-project.iam.gserviceaccount.com",
			},
			mock: &mockInstancesClient{
				insertError: errors.New("insert failed"),
			},
			wantErr:    true,
			wantErrMsg: "create instance error-vm",
		},
		{
			name: "wait error",
			config: cloud.VMCreateConfig{
				Name:           "wait-error-vm",
				MachineType:    "c4a-highcpu-4",
				DiskSizeGB:     50,
				Image:          "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-arm64",
				Network:        "default",
				ServiceAccount: "cloudcoop-vm@test-project.iam.gserviceaccount.com",
			},
			mock: &mockInstancesClient{
				waitError: errors.New("operation failed"),
			},
			wantErr:    true,
			wantErrMsg: "wait for create wait-error-vm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newWithClient("test-project", "test-zone", tt.mock)
			err := p.CreateVM(context.Background(), tt.config)

			if (err != nil) != tt.wantErr {
				t.Fatalf("CreateVM() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				if tt.wantErrMsg != "" && !contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("CreateVM() error = %v, want containing %q", err, tt.wantErrMsg)
				}
				return
			}

			if !tt.mock.insertCalled {
				t.Error("Insert() was not called")
			}

			req := tt.mock.lastInsertReq
			if req.GetProject() != "test-project" {
				t.Errorf("Insert() project = %q, want %q", req.GetProject(), "test-project")
			}
			if req.GetZone() != "test-zone" {
				t.Errorf("Insert() zone = %q, want %q", req.GetZone(), "test-zone")
			}

			inst := req.GetInstanceResource()
			if inst.GetName() != tt.config.Name {
				t.Errorf("Instance name = %q, want %q", inst.GetName(), tt.config.Name)
			}
		})
	}
}

func TestProvider_CreateVM_SpotScheduling(t *testing.T) {
	mock := &mockInstancesClient{}
	p := newWithClient("test-project", "test-zone", mock)

	config := cloud.VMCreateConfig{
		Name:           "spot-vm",
		MachineType:    "c4a-highcpu-4",
		DiskSizeGB:     50,
		Image:          "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-arm64",
		Spot:           true,
		Network:        "default",
		ServiceAccount: "cloudcoop-vm@test-project.iam.gserviceaccount.com",
	}

	err := p.CreateVM(context.Background(), config)
	if err != nil {
		t.Fatalf("CreateVM() error = %v", err)
	}

	inst := mock.lastInsertReq.GetInstanceResource()
	scheduling := inst.GetScheduling()
	if scheduling == nil {
		t.Fatal("Scheduling is nil, expected spot configuration")
	}
	if scheduling.GetProvisioningModel() != "SPOT" {
		t.Errorf("ProvisioningModel = %q, want %q", scheduling.GetProvisioningModel(), "SPOT")
	}
	if scheduling.GetInstanceTerminationAction() != "STOP" {
		t.Errorf("InstanceTerminationAction = %q, want %q", scheduling.GetInstanceTerminationAction(), "STOP")
	}
}

func TestProvider_CreateVM_NetworkTags(t *testing.T) {
	mock := &mockInstancesClient{}
	p := newWithClient("test-project", "test-zone", mock)

	config := cloud.VMCreateConfig{
		Name:           "tagged-vm",
		MachineType:    "c4a-highcpu-4",
		DiskSizeGB:     50,
		Image:          "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-arm64",
		Network:        "default",
		Tags:           []string{"cloudcoop", "ssh-allowed"},
		ServiceAccount: "cloudcoop-vm@test-project.iam.gserviceaccount.com",
	}

	err := p.CreateVM(context.Background(), config)
	if err != nil {
		t.Fatalf("CreateVM() error = %v", err)
	}

	inst := mock.lastInsertReq.GetInstanceResource()
	tags := inst.GetTags()
	if tags == nil {
		t.Fatal("Tags is nil, expected network tags")
	}
	if len(tags.GetItems()) != 2 {
		t.Errorf("Tags count = %d, want 2", len(tags.GetItems()))
	}
}

func TestProvider_CreateVM_ServiceAccount(t *testing.T) {
	mock := &mockInstancesClient{}
	p := newWithClient("test-project", "test-zone", mock)

	config := cloud.VMCreateConfig{
		Name:           "sa-vm",
		MachineType:    "c4a-highcpu-4",
		DiskSizeGB:     50,
		Image:          "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-arm64",
		Network:        "default",
		ServiceAccount: "cloudcoop-vm@test-project.iam.gserviceaccount.com",
	}

	err := p.CreateVM(context.Background(), config)
	if err != nil {
		t.Fatalf("CreateVM() error = %v", err)
	}

	inst := mock.lastInsertReq.GetInstanceResource()
	sas := inst.GetServiceAccounts()
	if len(sas) != 1 {
		t.Fatalf("ServiceAccounts count = %d, want 1", len(sas))
	}
	if sas[0].GetEmail() != "cloudcoop-vm@test-project.iam.gserviceaccount.com" {
		t.Errorf("ServiceAccount email = %q, want %q", sas[0].GetEmail(), "cloudcoop-vm@test-project.iam.gserviceaccount.com")
	}
	if len(sas[0].GetScopes()) != 1 || sas[0].GetScopes()[0] != "https://www.googleapis.com/auth/cloud-platform" {
		t.Errorf("ServiceAccount scopes = %v, want [https://www.googleapis.com/auth/cloud-platform]", sas[0].GetScopes())
	}
}

func TestProvider_CreateVM_Subnet(t *testing.T) {
	mock := &mockInstancesClient{}
	p := newWithClient("test-project", "us-central1-a", mock)

	config := cloud.VMCreateConfig{
		Name:           "subnet-vm",
		MachineType:    "c4a-highcpu-4",
		DiskSizeGB:     50,
		Image:          "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-arm64",
		Network:        "my-vpc",
		Subnet:         "my-subnet",
		ServiceAccount: "cloudcoop-vm@test-project.iam.gserviceaccount.com",
	}

	err := p.CreateVM(context.Background(), config)
	if err != nil {
		t.Fatalf("CreateVM() error = %v", err)
	}

	inst := mock.lastInsertReq.GetInstanceResource()
	nis := inst.GetNetworkInterfaces()
	if len(nis) != 1 {
		t.Fatalf("NetworkInterfaces count = %d, want 1", len(nis))
	}

	wantSubnet := "regions/us-central1/subnetworks/my-subnet"
	if nis[0].GetSubnetwork() != wantSubnet {
		t.Errorf("Subnetwork = %q, want %q", nis[0].GetSubnetwork(), wantSubnet)
	}
}

func TestProvider_CreateVM_NoSubnet(t *testing.T) {
	mock := &mockInstancesClient{}
	p := newWithClient("test-project", "us-central1-a", mock)

	config := cloud.VMCreateConfig{
		Name:           "no-subnet-vm",
		MachineType:    "c4a-highcpu-4",
		DiskSizeGB:     50,
		Image:          "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-arm64",
		Network:        "default",
		ServiceAccount: "cloudcoop-vm@test-project.iam.gserviceaccount.com",
	}

	err := p.CreateVM(context.Background(), config)
	if err != nil {
		t.Fatalf("CreateVM() error = %v", err)
	}

	inst := mock.lastInsertReq.GetInstanceResource()
	nis := inst.GetNetworkInterfaces()
	if len(nis) != 1 {
		t.Fatalf("NetworkInterfaces count = %d, want 1", len(nis))
	}

	if nis[0].GetSubnetwork() != "" {
		t.Errorf("Subnetwork = %q, want empty (not set)", nis[0].GetSubnetwork())
	}
}

func TestRegionFromZone(t *testing.T) {
	tests := []struct {
		zone string
		want string
	}{
		{"us-central1-a", "us-central1"},
		{"europe-north2-b", "europe-north2"},
		{"asia-east1-c", "asia-east1"},
	}

	for _, tt := range tests {
		t.Run(tt.zone, func(t *testing.T) {
			if got := regionFromZone(tt.zone); got != tt.want {
				t.Errorf("regionFromZone(%q) = %q, want %q", tt.zone, got, tt.want)
			}
		})
	}
}

func TestProvider_DeleteVM(t *testing.T) {
	tests := []struct {
		name       string
		vmName     string
		mock       *mockInstancesClient
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:   "successful delete",
			vmName: "test-vm",
			mock:   &mockInstancesClient{},
		},
		{
			name:   "delete error",
			vmName: "error-vm",
			mock: &mockInstancesClient{
				deleteError: errors.New("delete failed"),
			},
			wantErr:    true,
			wantErrMsg: "delete instance error-vm",
		},
		{
			name:   "wait error",
			vmName: "wait-error-vm",
			mock: &mockInstancesClient{
				waitError: errors.New("operation failed"),
			},
			wantErr:    true,
			wantErrMsg: "wait for delete wait-error-vm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newWithClient("test-project", "test-zone", tt.mock)
			err := p.DeleteVM(context.Background(), tt.vmName)

			if (err != nil) != tt.wantErr {
				t.Fatalf("DeleteVM() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				if tt.wantErrMsg != "" && !contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("DeleteVM() error = %v, want containing %q", err, tt.wantErrMsg)
				}
				return
			}

			if !tt.mock.deleteCalled {
				t.Error("Delete() was not called")
			}
			if tt.mock.lastDeleteReq.GetInstance() != tt.vmName {
				t.Errorf("Delete() called with instance = %q, want %q", tt.mock.lastDeleteReq.GetInstance(), tt.vmName)
			}
		})
	}
}

func TestProvider_EnsureFirewallAllowsSSH_Create(t *testing.T) {
	fw := &mockFirewallsClient{} // default: Get returns 404
	p := newWithClients("test-project", "test-zone", &mockInstancesClient{}, fw)

	changed, err := p.EnsureFirewallAllowsSSH(context.Background(), cloud.FirewallConfig{
		SourceIP: "203.0.113.50",
		Port:     2222,
		Network:  "default",
	})
	if err != nil {
		t.Fatalf("EnsureFirewallAllowsSSH() error = %v", err)
	}
	if !changed {
		t.Error("expected changed=true when creating new rule")
	}
	if !fw.getCalled {
		t.Error("Get() was not called")
	}
	if !fw.insertCalled {
		t.Error("Insert() was not called")
	}

	// Verify the insert request
	req := fw.lastInsertReq
	rule := req.GetFirewallResource()
	if rule.GetName() != "cloudcoop-allow-ssh" {
		t.Errorf("rule name = %q, want %q", rule.GetName(), "cloudcoop-allow-ssh")
	}
	ranges := rule.GetSourceRanges()
	if len(ranges) != 1 || ranges[0] != "203.0.113.50/32" {
		t.Errorf("source ranges = %v, want [203.0.113.50/32]", ranges)
	}
	ports := rule.GetAllowed()[0].GetPorts()
	if len(ports) != 1 || ports[0] != "2222" {
		t.Errorf("ports = %v, want [2222]", ports)
	}
}

func TestProvider_EnsureFirewallAllowsSSH_NoOp(t *testing.T) {
	fw := &mockFirewallsClient{
		rule: &computepb.Firewall{
			SourceRanges: []string{"203.0.113.50/32"},
			Allowed: []*computepb.Allowed{
				{
					IPProtocol: strPtr("tcp"),
					Ports:      []string{"2222"},
				},
			},
		},
	}
	p := newWithClients("test-project", "test-zone", &mockInstancesClient{}, fw)

	changed, err := p.EnsureFirewallAllowsSSH(context.Background(), cloud.FirewallConfig{
		SourceIP: "203.0.113.50",
		Port:     2222,
		Network:  "default",
	})
	if err != nil {
		t.Fatalf("EnsureFirewallAllowsSSH() error = %v", err)
	}
	if changed {
		t.Error("expected changed=false when rule already matches")
	}
	if fw.insertCalled {
		t.Error("Insert() should not be called")
	}
	if fw.patchCalled {
		t.Error("Patch() should not be called")
	}
}

func TestProvider_EnsureFirewallAllowsSSH_UpdateIP(t *testing.T) {
	fw := &mockFirewallsClient{
		rule: &computepb.Firewall{
			SourceRanges: []string{"198.51.100.1/32"},
			Allowed: []*computepb.Allowed{
				{
					IPProtocol: strPtr("tcp"),
					Ports:      []string{"2222"},
				},
			},
		},
	}
	p := newWithClients("test-project", "test-zone", &mockInstancesClient{}, fw)

	changed, err := p.EnsureFirewallAllowsSSH(context.Background(), cloud.FirewallConfig{
		SourceIP: "203.0.113.50",
		Port:     2222,
		Network:  "default",
	})
	if err != nil {
		t.Fatalf("EnsureFirewallAllowsSSH() error = %v", err)
	}
	if !changed {
		t.Error("expected changed=true when IP differs")
	}
	if !fw.patchCalled {
		t.Error("Patch() was not called")
	}

	req := fw.lastPatchReq
	rule := req.GetFirewallResource()
	ranges := rule.GetSourceRanges()
	if len(ranges) != 1 || ranges[0] != "203.0.113.50/32" {
		t.Errorf("patched source ranges = %v, want [203.0.113.50/32]", ranges)
	}
}

func TestProvider_EnsureFirewallAllowsSSH_UpdatePort(t *testing.T) {
	fw := &mockFirewallsClient{
		rule: &computepb.Firewall{
			SourceRanges: []string{"203.0.113.50/32"},
			Allowed: []*computepb.Allowed{
				{
					IPProtocol: strPtr("tcp"),
					Ports:      []string{"22"},
				},
			},
		},
	}
	p := newWithClients("test-project", "test-zone", &mockInstancesClient{}, fw)

	changed, err := p.EnsureFirewallAllowsSSH(context.Background(), cloud.FirewallConfig{
		SourceIP: "203.0.113.50",
		Port:     2222,
		Network:  "default",
	})
	if err != nil {
		t.Fatalf("EnsureFirewallAllowsSSH() error = %v", err)
	}
	if !changed {
		t.Error("expected changed=true when port differs")
	}
	if !fw.patchCalled {
		t.Error("Patch() was not called")
	}
}

func TestProvider_EnsureFirewallAllowsSSH_GetError(t *testing.T) {
	fw := &mockFirewallsClient{
		getError: newForbiddenError(),
	}
	p := newWithClients("test-project", "test-zone", &mockInstancesClient{}, fw)

	_, err := p.EnsureFirewallAllowsSSH(context.Background(), cloud.FirewallConfig{
		SourceIP: "203.0.113.50",
		Port:     22,
		Network:  "default",
	})
	if err == nil {
		t.Error("expected error for non-404 Get failure")
	}
	if !contains(err.Error(), "get firewall rule") {
		t.Errorf("error = %v, want containing 'get firewall rule'", err)
	}
}

func TestProvider_EnsureFirewallAllowsSSH_InsertError(t *testing.T) {
	fw := &mockFirewallsClient{
		insertError: errors.New("insert failed"),
	}
	p := newWithClients("test-project", "test-zone", &mockInstancesClient{}, fw)

	_, err := p.EnsureFirewallAllowsSSH(context.Background(), cloud.FirewallConfig{
		SourceIP: "203.0.113.50",
		Port:     22,
		Network:  "default",
	})
	if err == nil {
		t.Error("expected error for Insert failure")
	}
}

func TestProvider_EnsureFirewallAllowsSSH_PatchError(t *testing.T) {
	fw := &mockFirewallsClient{
		rule: &computepb.Firewall{
			SourceRanges: []string{"198.51.100.1/32"},
			Allowed: []*computepb.Allowed{
				{
					IPProtocol: strPtr("tcp"),
					Ports:      []string{"22"},
				},
			},
		},
		patchError: errors.New("patch failed"),
	}
	p := newWithClients("test-project", "test-zone", &mockInstancesClient{}, fw)

	_, err := p.EnsureFirewallAllowsSSH(context.Background(), cloud.FirewallConfig{
		SourceIP: "203.0.113.50",
		Port:     22,
		Network:  "default",
	})
	if err == nil {
		t.Error("expected error for Patch failure")
	}
}

func TestProvider_EnsureFirewallAllowsSSH_NilFirewalls(t *testing.T) {
	p := newWithClient("test-project", "test-zone", &mockInstancesClient{})

	_, err := p.EnsureFirewallAllowsSSH(context.Background(), cloud.FirewallConfig{
		SourceIP: "203.0.113.50",
		Port:     22,
		Network:  "default",
	})
	if err == nil {
		t.Error("expected error for nil firewalls client")
	}
}

func TestProvider_EnsureSSHKeyOnVM_AddNew(t *testing.T) {
	mock := &mockInstancesClient{
		getInstance: func() *computepb.Instance {
			inst := newTestInstance("test-vm", "RUNNING", "us-central1-a", "c4a-highcpu-4")
			inst.Metadata = &computepb.Metadata{
				Fingerprint: strPtr("abc123"),
				Items: []*computepb.Items{
					{Key: strPtr("startup-script"), Value: strPtr("#!/bin/bash\necho hello")},
				},
			}
			return inst
		}(),
	}
	p := newWithClient("test-project", "test-zone", mock)

	err := p.EnsureSSHKeyOnVM(context.Background(), "test-vm", "paul", "ssh-ed25519 AAAAC3 paul@host")
	if err != nil {
		t.Fatalf("EnsureSSHKeyOnVM() error = %v", err)
	}

	if !mock.setMetadataCalled {
		t.Fatal("SetMetadata() was not called")
	}

	req := mock.lastSetMetadataReq
	if req.GetInstance() != "test-vm" {
		t.Errorf("Instance = %q, want %q", req.GetInstance(), "test-vm")
	}

	md := req.GetMetadataResource()
	if md.GetFingerprint() != "abc123" {
		t.Errorf("Fingerprint = %q, want %q", md.GetFingerprint(), "abc123")
	}

	// Should have startup-script + ssh-keys
	if len(md.GetItems()) != 2 {
		t.Fatalf("Items count = %d, want 2", len(md.GetItems()))
	}

	// Find ssh-keys item
	var sshKeysValue string
	for _, item := range md.GetItems() {
		if item.GetKey() == "ssh-keys" {
			sshKeysValue = item.GetValue()
		}
	}
	if sshKeysValue != "paul:ssh-ed25519 AAAAC3 paul@host" {
		t.Errorf("ssh-keys = %q, want %q", sshKeysValue, "paul:ssh-ed25519 AAAAC3 paul@host")
	}
}

func TestProvider_EnsureSSHKeyOnVM_AlreadyPresent(t *testing.T) {
	mock := &mockInstancesClient{
		getInstance: func() *computepb.Instance {
			inst := newTestInstance("test-vm", "RUNNING", "us-central1-a", "c4a-highcpu-4")
			inst.Metadata = &computepb.Metadata{
				Fingerprint: strPtr("abc123"),
				Items: []*computepb.Items{
					{Key: strPtr("ssh-keys"), Value: strPtr("paul:ssh-ed25519 AAAAC3 paul@host")},
				},
			}
			return inst
		}(),
	}
	p := newWithClient("test-project", "test-zone", mock)

	err := p.EnsureSSHKeyOnVM(context.Background(), "test-vm", "paul", "ssh-ed25519 AAAAC3 paul@host")
	if err != nil {
		t.Fatalf("EnsureSSHKeyOnVM() error = %v", err)
	}

	if mock.setMetadataCalled {
		t.Error("SetMetadata() should not be called when key already present")
	}
}

func TestProvider_EnsureSSHKeyOnVM_AppendToExisting(t *testing.T) {
	mock := &mockInstancesClient{
		getInstance: func() *computepb.Instance {
			inst := newTestInstance("test-vm", "RUNNING", "us-central1-a", "c4a-highcpu-4")
			inst.Metadata = &computepb.Metadata{
				Fingerprint: strPtr("abc123"),
				Items: []*computepb.Items{
					{Key: strPtr("ssh-keys"), Value: strPtr("alice:ssh-rsa AAAAB3 alice@host")},
				},
			}
			return inst
		}(),
	}
	p := newWithClient("test-project", "test-zone", mock)

	err := p.EnsureSSHKeyOnVM(context.Background(), "test-vm", "paul", "ssh-ed25519 AAAAC3 paul@host")
	if err != nil {
		t.Fatalf("EnsureSSHKeyOnVM() error = %v", err)
	}

	if !mock.setMetadataCalled {
		t.Fatal("SetMetadata() was not called")
	}

	var sshKeysValue string
	for _, item := range mock.lastSetMetadataReq.GetMetadataResource().GetItems() {
		if item.GetKey() == "ssh-keys" {
			sshKeysValue = item.GetValue()
		}
	}
	want := "alice:ssh-rsa AAAAB3 alice@host\npaul:ssh-ed25519 AAAAC3 paul@host"
	if sshKeysValue != want {
		t.Errorf("ssh-keys = %q, want %q", sshKeysValue, want)
	}
}

func TestProvider_EnsureSSHKeyOnVM_GetError(t *testing.T) {
	mock := &mockInstancesClient{
		getError: errors.New("network error"),
	}
	p := newWithClient("test-project", "test-zone", mock)

	err := p.EnsureSSHKeyOnVM(context.Background(), "test-vm", "paul", "ssh-ed25519 AAAAC3 paul@host")
	if err == nil {
		t.Error("expected error")
	}
	if !contains(err.Error(), "get instance test-vm") {
		t.Errorf("error = %v, want containing 'get instance test-vm'", err)
	}
}

func TestProvider_EnsureSSHKeyOnVM_SetMetadataError(t *testing.T) {
	mock := &mockInstancesClient{
		getInstance: func() *computepb.Instance {
			inst := newTestInstance("test-vm", "RUNNING", "us-central1-a", "c4a-highcpu-4")
			inst.Metadata = &computepb.Metadata{
				Fingerprint: strPtr("abc123"),
			}
			return inst
		}(),
		setMetadataError: errors.New("permission denied"),
	}
	p := newWithClient("test-project", "test-zone", mock)

	err := p.EnsureSSHKeyOnVM(context.Background(), "test-vm", "paul", "ssh-ed25519 AAAAC3 paul@host")
	if err == nil {
		t.Error("expected error")
	}
	if !contains(err.Error(), "set metadata on test-vm") {
		t.Errorf("error = %v, want containing 'set metadata on test-vm'", err)
	}
}

func TestProvider_CreateVM_SSHKeyInMetadata(t *testing.T) {
	mock := &mockInstancesClient{}
	p := newWithClient("test-project", "test-zone", mock)

	cfg := cloud.VMCreateConfig{
		Name:           "ssh-vm",
		MachineType:    "c4a-highcpu-4",
		DiskSizeGB:     50,
		Image:          "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-arm64",
		Network:        "default",
		SSHUser:        "paul",
		SSHPublicKey:   "ssh-ed25519 AAAAC3 paul@host",
		ServiceAccount: "cloudcoop-vm@test-project.iam.gserviceaccount.com",
	}

	err := p.CreateVM(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CreateVM() error = %v", err)
	}

	inst := mock.lastInsertReq.GetInstanceResource()
	md := inst.GetMetadata()

	var sshKeysValue string
	for _, item := range md.GetItems() {
		if item.GetKey() == "ssh-keys" {
			sshKeysValue = item.GetValue()
		}
	}

	want := "paul:ssh-ed25519 AAAAC3 paul@host"
	if sshKeysValue != want {
		t.Errorf("ssh-keys metadata = %q, want %q", sshKeysValue, want)
	}
}

func TestProvider_CreateVM_NoSSHKeyWithoutPublicKey(t *testing.T) {
	mock := &mockInstancesClient{}
	p := newWithClient("test-project", "test-zone", mock)

	cfg := cloud.VMCreateConfig{
		Name:           "no-ssh-vm",
		MachineType:    "c4a-highcpu-4",
		DiskSizeGB:     50,
		Image:          "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-arm64",
		Network:        "default",
		ServiceAccount: "cloudcoop-vm@test-project.iam.gserviceaccount.com",
	}

	err := p.CreateVM(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CreateVM() error = %v", err)
	}

	inst := mock.lastInsertReq.GetInstanceResource()
	md := inst.GetMetadata()

	for _, item := range md.GetItems() {
		if item.GetKey() == "ssh-keys" {
			t.Error("ssh-keys should not be present when SSHPublicKey is empty")
		}
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
