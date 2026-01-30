// Package ssh provides SSH client functionality for connecting to VMs.
package ssh

import (
	"fmt"
	"os/user"
)

// ErrNoIPAvailable is returned when no IP address is available for connection.
var ErrNoIPAvailable = fmt.Errorf("no IP address available")

// ResolveVMIP returns the best IP address for SSH connection.
// It prefers external IP, falling back to internal IP.
// Returns ErrNoIPAvailable if neither is available.
func ResolveVMIP(externalIP, internalIP string) (string, error) {
	if externalIP != "" {
		return externalIP, nil
	}
	if internalIP != "" {
		return internalIP, nil
	}
	return "", ErrNoIPAvailable
}

// ResolveSSHUser returns the SSH username to use.
// If configUser is set, it is used. Otherwise, the current OS user is returned.
func ResolveSSHUser(configUser string) string {
	if configUser != "" {
		return configUser
	}
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
}

// ResolvePort returns the SSH port to use.
// If port is 0, returns the default SSH port (22).
func ResolvePort(port int) int {
	if port == 0 {
		return 22
	}
	return port
}

// SetupClientConfig creates an SSH Config from resolved parameters.
// This is a convenience wrapper that applies defaults.
func SetupClientConfig(host, user string, port int) Config {
	return Config{
		Host:    host,
		User:    user,
		Port:    ResolvePort(port),
		Timeout: DefaultTimeout,
	}
}
