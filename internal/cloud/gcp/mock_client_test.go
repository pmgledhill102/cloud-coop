package gcp

import (
	"context"

	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/googleapi"
	"google.golang.org/protobuf/proto"
)

// mockInstancesClient is a mock implementation of instancesClient for testing.
type mockInstancesClient struct {
	getInstance  *computepb.Instance
	getError     error
	startError   error
	stopError    error
	closeError   error
	waitError    error
	getCalled    bool
	startCalled  bool
	stopCalled   bool
	closeCalled  bool
	lastGetReq   *computepb.GetInstanceRequest
	lastStartReq *computepb.StartInstanceRequest
	lastStopReq  *computepb.StopInstanceRequest
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
