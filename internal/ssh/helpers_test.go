package ssh

import (
	"errors"
	"testing"
)

func TestResolveVMIP(t *testing.T) {
	tests := []struct {
		name       string
		externalIP string
		internalIP string
		want       string
		wantErr    error
	}{
		{
			name:       "external IP preferred",
			externalIP: "203.0.113.1",
			internalIP: "10.0.0.1",
			want:       "203.0.113.1",
			wantErr:    nil,
		},
		{
			name:       "fallback to internal when no external",
			externalIP: "",
			internalIP: "10.0.0.1",
			want:       "10.0.0.1",
			wantErr:    nil,
		},
		{
			name:       "error when no IP available",
			externalIP: "",
			internalIP: "",
			want:       "",
			wantErr:    ErrNoIPAvailable,
		},
		{
			name:       "external only",
			externalIP: "203.0.113.1",
			internalIP: "",
			want:       "203.0.113.1",
			wantErr:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveVMIP(tt.externalIP, tt.internalIP)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ResolveVMIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ResolveVMIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveSSHUser(t *testing.T) {
	tests := []struct {
		name       string
		configUser string
		wantEmpty  bool
	}{
		{
			name:       "config user is used when set",
			configUser: "ubuntu",
			wantEmpty:  false,
		},
		{
			name:       "falls back to current user when config empty",
			configUser: "",
			wantEmpty:  false, // Current user should exist in test env
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSSHUser(tt.configUser)
			if tt.wantEmpty && got != "" {
				t.Errorf("ResolveSSHUser() = %v, want empty", got)
			}
			if !tt.wantEmpty && got == "" {
				t.Errorf("ResolveSSHUser() = empty, want non-empty")
			}
			if tt.configUser != "" && got != tt.configUser {
				t.Errorf("ResolveSSHUser() = %v, want %v", got, tt.configUser)
			}
		})
	}
}

func TestResolvePort(t *testing.T) {
	tests := []struct {
		name string
		port int
		want int
	}{
		{
			name: "zero returns default 22",
			port: 0,
			want: 22,
		},
		{
			name: "custom port is preserved",
			port: 2222,
			want: 2222,
		},
		{
			name: "standard port is preserved",
			port: 22,
			want: 22,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolvePort(tt.port); got != tt.want {
				t.Errorf("ResolvePort() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetupClientConfig(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		user     string
		port     int
		wantHost string
		wantUser string
		wantPort int
	}{
		{
			name:     "all values set",
			host:     "10.0.0.1",
			user:     "ubuntu",
			port:     2222,
			wantHost: "10.0.0.1",
			wantUser: "ubuntu",
			wantPort: 2222,
		},
		{
			name:     "zero port gets default",
			host:     "10.0.0.1",
			user:     "ubuntu",
			port:     0,
			wantHost: "10.0.0.1",
			wantUser: "ubuntu",
			wantPort: 22,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := SetupClientConfig(tt.host, tt.user, tt.port)
			if cfg.Host != tt.wantHost {
				t.Errorf("SetupClientConfig().Host = %v, want %v", cfg.Host, tt.wantHost)
			}
			if cfg.User != tt.wantUser {
				t.Errorf("SetupClientConfig().User = %v, want %v", cfg.User, tt.wantUser)
			}
			if cfg.Port != tt.wantPort {
				t.Errorf("SetupClientConfig().Port = %v, want %v", cfg.Port, tt.wantPort)
			}
			if cfg.Timeout != DefaultTimeout {
				t.Errorf("SetupClientConfig().Timeout = %v, want %v", cfg.Timeout, DefaultTimeout)
			}
		})
	}
}
