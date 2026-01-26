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
	Close() error
}

// disksClient defines the interface for GCP Compute disks operations.
type disksClient interface {
	Delete(ctx context.Context, req *computepb.DeleteDiskRequest) (operation, error)
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

func (r *realInstancesClient) Close() error {
	return r.client.Close()
}

// realDisksClient wraps the actual GCP DisksClient to implement our interface.
type realDisksClient struct {
	client *compute.DisksClient
}

func (r *realDisksClient) Delete(ctx context.Context, req *computepb.DeleteDiskRequest) (operation, error) {
	op, err := r.client.Delete(ctx, req)
	if err != nil {
		return nil, err
	}
	return &realOperation{op: op}, nil
}

func (r *realDisksClient) Close() error {
	return r.client.Close()
}

// realOperation wraps the actual GCP Operation to implement our interface.
type realOperation struct {
	op *compute.Operation
}

func (r *realOperation) Wait(ctx context.Context) error {
	return r.op.Wait(ctx)
}
