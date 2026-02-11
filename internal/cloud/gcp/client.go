package gcp

import (
	"context"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
)

// instancesClient defines the interface for GCP Compute instances operations.
// This allows for dependency injection and testing with mocks.
type instancesClient interface {
	Get(ctx context.Context, req *computepb.GetInstanceRequest) (*computepb.Instance, error)
	Start(ctx context.Context, req *computepb.StartInstanceRequest) (operation, error)
	Stop(ctx context.Context, req *computepb.StopInstanceRequest) (operation, error)
	Insert(ctx context.Context, req *computepb.InsertInstanceRequest) (operation, error)
	Delete(ctx context.Context, req *computepb.DeleteInstanceRequest) (operation, error)
	SetMetadata(ctx context.Context, req *computepb.SetMetadataInstanceRequest) (operation, error)
	Close() error
}

// operation defines the interface for GCP long-running operations.
type operation interface {
	Wait(ctx context.Context) error
}

// realInstancesClient wraps the actual GCP InstancesClient to implement our interface.
type realInstancesClient struct {
	client *compute.InstancesClient
}

func (r *realInstancesClient) Get(ctx context.Context, req *computepb.GetInstanceRequest) (*computepb.Instance, error) {
	return r.client.Get(ctx, req)
}

func (r *realInstancesClient) Start(ctx context.Context, req *computepb.StartInstanceRequest) (operation, error) {
	op, err := r.client.Start(ctx, req)
	if err != nil {
		return nil, err
	}
	return &realOperation{op: op}, nil
}

func (r *realInstancesClient) Stop(ctx context.Context, req *computepb.StopInstanceRequest) (operation, error) {
	op, err := r.client.Stop(ctx, req)
	if err != nil {
		return nil, err
	}
	return &realOperation{op: op}, nil
}

func (r *realInstancesClient) Insert(ctx context.Context, req *computepb.InsertInstanceRequest) (operation, error) {
	op, err := r.client.Insert(ctx, req)
	if err != nil {
		return nil, err
	}
	return &realOperation{op: op}, nil
}

func (r *realInstancesClient) Delete(ctx context.Context, req *computepb.DeleteInstanceRequest) (operation, error) {
	op, err := r.client.Delete(ctx, req)
	if err != nil {
		return nil, err
	}
	return &realOperation{op: op}, nil
}

func (r *realInstancesClient) SetMetadata(ctx context.Context, req *computepb.SetMetadataInstanceRequest) (operation, error) {
	op, err := r.client.SetMetadata(ctx, req)
	if err != nil {
		return nil, err
	}
	return &realOperation{op: op}, nil
}

func (r *realInstancesClient) Close() error {
	return r.client.Close()
}

// firewallsClient defines the interface for GCP Compute firewalls operations.
// This allows for dependency injection and testing with mocks.
type firewallsClient interface {
	Get(ctx context.Context, req *computepb.GetFirewallRequest) (*computepb.Firewall, error)
	Insert(ctx context.Context, req *computepb.InsertFirewallRequest) (operation, error)
	Patch(ctx context.Context, req *computepb.PatchFirewallRequest) (operation, error)
	Close() error
}

// realFirewallsClient wraps the actual GCP FirewallsClient to implement our interface.
type realFirewallsClient struct {
	client *compute.FirewallsClient
}

func (r *realFirewallsClient) Get(ctx context.Context, req *computepb.GetFirewallRequest) (*computepb.Firewall, error) {
	return r.client.Get(ctx, req)
}

func (r *realFirewallsClient) Insert(ctx context.Context, req *computepb.InsertFirewallRequest) (operation, error) {
	op, err := r.client.Insert(ctx, req)
	if err != nil {
		return nil, err
	}
	return &realOperation{op: op}, nil
}

func (r *realFirewallsClient) Patch(ctx context.Context, req *computepb.PatchFirewallRequest) (operation, error) {
	op, err := r.client.Patch(ctx, req)
	if err != nil {
		return nil, err
	}
	return &realOperation{op: op}, nil
}

func (r *realFirewallsClient) Close() error {
	return r.client.Close()
}

// realOperation wraps the actual GCP Operation to implement our interface.
type realOperation struct {
	op *compute.Operation
}

func (r *realOperation) Wait(ctx context.Context) error {
	return r.op.Wait(ctx)
}
