package gcp

import (
	"context"

	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/googleapi"
	"google.golang.org/protobuf/proto"
)

// mockInstancesClient is a mock implementation of instancesClient for testing.
type mockInstancesClient struct {
	getInstance        *computepb.Instance
	getError           error
	startError         error
	stopError          error
	insertError        error
	deleteError        error
	setMetadataError   error
	closeError         error
	waitError          error
	getCalled          bool
	startCalled        bool
	stopCalled         bool
	insertCalled       bool
	deleteCalled       bool
	setMetadataCalled  bool
	closeCalled        bool
	lastGetReq         *computepb.GetInstanceRequest
	lastStartReq       *computepb.StartInstanceRequest
	lastStopReq        *computepb.StopInstanceRequest
	lastInsertReq      *computepb.InsertInstanceRequest
	lastDeleteReq      *computepb.DeleteInstanceRequest
	lastSetMetadataReq *computepb.SetMetadataInstanceRequest
}

func (m *mockInstancesClient) Get(ctx context.Context, req *computepb.GetInstanceRequest) (*computepb.Instance, error) {
	m.getCalled = true
	m.lastGetReq = req
	return m.getInstance, m.getError
}

func (m *mockInstancesClient) Start(ctx context.Context, req *computepb.StartInstanceRequest) (operation, error) {
	m.startCalled = true
	m.lastStartReq = req
	if m.startError != nil {
		return nil, m.startError
	}
	return &mockOperation{waitError: m.waitError}, nil
}

func (m *mockInstancesClient) Stop(ctx context.Context, req *computepb.StopInstanceRequest) (operation, error) {
	m.stopCalled = true
	m.lastStopReq = req
	if m.stopError != nil {
		return nil, m.stopError
	}
	return &mockOperation{waitError: m.waitError}, nil
}

func (m *mockInstancesClient) Insert(ctx context.Context, req *computepb.InsertInstanceRequest) (operation, error) {
	m.insertCalled = true
	m.lastInsertReq = req
	if m.insertError != nil {
		return nil, m.insertError
	}
	return &mockOperation{waitError: m.waitError}, nil
}

func (m *mockInstancesClient) Delete(ctx context.Context, req *computepb.DeleteInstanceRequest) (operation, error) {
	m.deleteCalled = true
	m.lastDeleteReq = req
	if m.deleteError != nil {
		return nil, m.deleteError
	}
	return &mockOperation{waitError: m.waitError}, nil
}

func (m *mockInstancesClient) SetMetadata(ctx context.Context, req *computepb.SetMetadataInstanceRequest) (operation, error) {
	m.setMetadataCalled = true
	m.lastSetMetadataReq = req
	if m.setMetadataError != nil {
		return nil, m.setMetadataError
	}
	return &mockOperation{waitError: m.waitError}, nil
}

func (m *mockInstancesClient) Close() error {
	m.closeCalled = true
	return m.closeError
}

// mockOperation is a mock implementation of operation for testing.
type mockOperation struct {
	waitError error
}

func (m *mockOperation) Wait(ctx context.Context) error {
	return m.waitError
}

// Helper functions to create test instances

func newTestInstance(name, status, zone, machineType string) *computepb.Instance {
	return &computepb.Instance{
		Name:        proto.String(name),
		Status:      proto.String(status),
		Zone:        proto.String("projects/test-project/zones/" + zone),
		MachineType: proto.String("projects/test-project/zones/" + zone + "/machineTypes/" + machineType),
	}
}

func newTestInstanceWithIPs(name, status, zone, machineType, internalIP, externalIP string) *computepb.Instance {
	inst := newTestInstance(name, status, zone, machineType)
	inst.NetworkInterfaces = []*computepb.NetworkInterface{
		{
			NetworkIP: proto.String(internalIP),
			AccessConfigs: []*computepb.AccessConfig{
				{
					NatIP: proto.String(externalIP),
				},
			},
		},
	}
	return inst
}

// newNotFoundError creates a 404 googleapi.Error for testing.
func newNotFoundError() error {
	return &googleapi.Error{
		Code:    404,
		Message: "not found",
	}
}

// newForbiddenError creates a 403 googleapi.Error for testing.
func newForbiddenError() error {
	return &googleapi.Error{
		Code:    403,
		Message: "forbidden",
	}
}

// mockFirewallsClient is a mock implementation of firewallsClient for testing.
type mockFirewallsClient struct {
	rule          *computepb.Firewall
	getError      error
	insertError   error
	patchError    error
	waitError     error
	closeCalled   bool
	getCalled     bool
	insertCalled  bool
	patchCalled   bool
	lastGetReq    *computepb.GetFirewallRequest
	lastInsertReq *computepb.InsertFirewallRequest
	lastPatchReq  *computepb.PatchFirewallRequest
}

func (m *mockFirewallsClient) Get(ctx context.Context, req *computepb.GetFirewallRequest) (*computepb.Firewall, error) {
	m.getCalled = true
	m.lastGetReq = req
	if m.getError != nil {
		return nil, m.getError
	}
	if m.rule != nil {
		return m.rule, nil
	}
	return nil, newNotFoundError()
}

func (m *mockFirewallsClient) Insert(ctx context.Context, req *computepb.InsertFirewallRequest) (operation, error) {
	m.insertCalled = true
	m.lastInsertReq = req
	if m.insertError != nil {
		return nil, m.insertError
	}
	return &mockOperation{waitError: m.waitError}, nil
}

func (m *mockFirewallsClient) Patch(ctx context.Context, req *computepb.PatchFirewallRequest) (operation, error) {
	m.patchCalled = true
	m.lastPatchReq = req
	if m.patchError != nil {
		return nil, m.patchError
	}
	return &mockOperation{waitError: m.waitError}, nil
}

func (m *mockFirewallsClient) Close() error {
	m.closeCalled = true
	return nil
}

// mockNetworksClient is a mock implementation of networksClient for testing.
type mockNetworksClient struct {
	network  *computepb.Network
	getError error
}

func (m *mockNetworksClient) Get(_ context.Context, _ *computepb.GetNetworkRequest) (*computepb.Network, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	if m.network != nil {
		return m.network, nil
	}
	return &computepb.Network{}, nil
}

func (m *mockNetworksClient) Close() error { return nil }

// mockSubnetworksClient is a mock implementation of subnetworksClient for testing.
type mockSubnetworksClient struct {
	subnetwork *computepb.Subnetwork
	getError   error
}

func (m *mockSubnetworksClient) Get(_ context.Context, _ *computepb.GetSubnetworkRequest) (*computepb.Subnetwork, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	if m.subnetwork != nil {
		return m.subnetwork, nil
	}
	return &computepb.Subnetwork{}, nil
}

func (m *mockSubnetworksClient) Close() error { return nil }
