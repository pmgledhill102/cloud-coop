package ops

import "time"

// Shared timeout constants for VM operations. Both TUI and CLI should
// reference these to keep timeouts consistent across execution paths.
const (
	// TimeoutVMCreate is the timeout for VM creation (can take several minutes).
	TimeoutVMCreate = 300 * time.Second

	// TimeoutVMLifecycle is the timeout for start, stop, and delete operations.
	TimeoutVMLifecycle = 180 * time.Second

	// TimeoutVMStatus is the timeout for VM status queries.
	TimeoutVMStatus = 10 * time.Second

	// TimeoutAccess is the timeout for firewall and SSH key operations.
	TimeoutAccess = 15 * time.Second

	// TimeoutProvision is the timeout for provisioning status checks.
	TimeoutProvision = 30 * time.Second
)
