package ssh

import (
	"errors"
	"testing"
)

func TestFormatHostPort(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{"standard port 22", "192.168.1.1", 22, "192.168.1.1"},
		{"port 0 (default)", "192.168.1.1", 0, "192.168.1.1"},
		{"custom port", "192.168.1.1", 2222, "192.168.1.1:2222"},
		{"hostname with custom port", "example.com", 8022, "example.com:8022"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatHostPort(tt.host, tt.port)
			if got != tt.want {
				t.Errorf("formatHostPort(%q, %d) = %q, want %q", tt.host, tt.port, got, tt.want)
			}
		})
	}
}

func TestFormatKnownHostsEntry(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{"standard port 22", "192.168.1.1", 22, "192.168.1.1"},
		{"port 0 (default)", "192.168.1.1", 0, "192.168.1.1"},
		{"custom port", "192.168.1.1", 2222, "[192.168.1.1]:2222"},
		{"hostname with custom port", "example.com", 8022, "[example.com]:8022"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatKnownHostsEntry(tt.host, tt.port)
			if got != tt.want {
				t.Errorf("formatKnownHostsEntry(%q, %d) = %q, want %q", tt.host, tt.port, got, tt.want)
			}
		})
	}
}

func TestIsHostKeyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "key is unknown error",
			err:  errors.New("ssh: handshake failed: knownhosts: key is unknown"),
			want: true,
		},
		{
			name: "key mismatch error",
			err:  errors.New("ssh: handshake failed: knownhosts: key mismatch"),
			want: true,
		},
		{
			name: "host key verification failed",
			err:  errors.New("host key verification failed for 192.168.1.1: something"),
			want: true,
		},
		{
			name: "unrelated error",
			err:  errors.New("connection refused"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHostKeyError(tt.err)
			if got != tt.want {
				t.Errorf("IsHostKeyError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
