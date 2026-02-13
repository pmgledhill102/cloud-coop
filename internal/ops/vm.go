package ops

import (
	"errors"
	"fmt"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
)

// VM state validation errors. Callers can use errors.Is to distinguish
// between no-op states (e.g. already running) and blocking states
// (e.g. currently stopping).
var (
	ErrVMNotFound       = errors.New("VM not found")
	ErrVMAlreadyRunning = errors.New("VM is already running")
	ErrVMAlreadyStopped = errors.New("VM is already stopped")
	ErrVMStarting       = errors.New("VM is currently starting")
	ErrVMStopping       = errors.New("VM is currently stopping")
	ErrVMExists         = errors.New("VM already exists")
	ErrVMRunning        = errors.New("VM is running, stop it first")
)

// ValidateForStart checks whether the VM can be started.
// Returns nil if the VM is in a valid state to start.
func ValidateForStart(info *cloud.VMInfo) error {
	switch info.Status {
	case cloud.VMStatusStopped:
		return nil
	case cloud.VMStatusRunning:
		return ErrVMAlreadyRunning
	case cloud.VMStatusStarting:
		return ErrVMStarting
	case cloud.VMStatusStopping:
		return fmt.Errorf("%w, please wait and try again", ErrVMStopping)
	case cloud.VMStatusNotFound:
		return ErrVMNotFound
	default:
		return fmt.Errorf("unexpected state: %s", info.Status)
	}
}

// ValidateForStop checks whether the VM can be stopped.
// Returns nil if the VM is in a valid state to stop.
func ValidateForStop(info *cloud.VMInfo) error {
	switch info.Status {
	case cloud.VMStatusRunning:
		return nil
	case cloud.VMStatusStopped:
		return ErrVMAlreadyStopped
	case cloud.VMStatusStopping:
		return ErrVMStopping
	case cloud.VMStatusStarting:
		return fmt.Errorf("%w, please wait and try again", ErrVMStarting)
	case cloud.VMStatusNotFound:
		return ErrVMNotFound
	default:
		return fmt.Errorf("unexpected state: %s", info.Status)
	}
}

// ValidateForCreate checks whether a new VM can be created.
// Returns nil if no VM exists (status not_found).
func ValidateForCreate(info *cloud.VMInfo) error {
	switch info.Status {
	case cloud.VMStatusNotFound:
		return nil
	case cloud.VMStatusRunning, cloud.VMStatusStopped:
		return ErrVMExists
	default:
		return fmt.Errorf("VM is in state %s, cannot create", info.Status)
	}
}

// ValidateForDelete checks whether the VM can be deleted.
// Returns nil if the VM is stopped and can be deleted.
func ValidateForDelete(info *cloud.VMInfo) error {
	switch info.Status {
	case cloud.VMStatusStopped:
		return nil
	case cloud.VMStatusNotFound:
		return ErrVMNotFound
	case cloud.VMStatusRunning:
		return ErrVMRunning
	default:
		return fmt.Errorf("VM is in state %s, cannot delete", info.Status)
	}
}
