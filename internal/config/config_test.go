package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile(t *testing.T) {
	// Create temp config file
	dir := t.TempDir()
	path := filepath.Join(dir, "cloudcoop.toml")

	content := `
[cloud]
provider = "gcp"

[cloud.gcp]
project = "test-project"
zone = "us-central1-a"

[vm]
name = "test-vm"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Cloud.Provider != "gcp" {
		t.Errorf("provider = %q, want %q", cfg.Cloud.Provider, "gcp")
	}
	if cfg.Cloud.GCP.Project != "test-project" {
		t.Errorf("project = %q, want %q", cfg.Cloud.GCP.Project, "test-project")
	}
	if cfg.Cloud.GCP.Zone != "us-central1-a" {
		t.Errorf("zone = %q, want %q", cfg.Cloud.GCP.Zone, "us-central1-a")
	}
	if cfg.VM.Name != "test-vm" {
		t.Errorf("vm.name = %q, want %q", cfg.VM.Name, "test-vm")
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/config.toml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid gcp config",
			cfg: Config{
				Cloud: CloudConfig{
					Provider: "gcp",
					GCP:      GCPConfig{Project: "proj", Zone: "zone-a"},
				},
				VM: VMConfig{Name: "vm-1"},
			},
			wantErr: false,
		},
		{
			name: "missing project",
			cfg: Config{
				Cloud: CloudConfig{
					Provider: "gcp",
					GCP:      GCPConfig{Zone: "zone-a"},
				},
				VM: VMConfig{Name: "vm-1"},
			},
			wantErr: true,
		},
		{
			name: "missing zone",
			cfg: Config{
				Cloud: CloudConfig{
					Provider: "gcp",
					GCP:      GCPConfig{Project: "proj"},
				},
				VM: VMConfig{Name: "vm-1"},
			},
			wantErr: true,
		},
		{
			name: "missing vm name",
			cfg: Config{
				Cloud: CloudConfig{
					Provider: "gcp",
					GCP:      GCPConfig{Project: "proj", Zone: "zone-a"},
				},
			},
			wantErr: true,
		},
		{
			name: "unsupported provider",
			cfg: Config{
				Cloud: CloudConfig{Provider: "unknown"},
				VM:    VMConfig{Name: "vm-1"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultProviderIsGCP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cloudcoop.toml")

	// Config without explicit provider
	content := `
[cloud.gcp]
project = "test-project"
zone = "us-central1-a"

[vm]
name = "test-vm"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Cloud.Provider != "gcp" {
		t.Errorf("default provider = %q, want %q", cfg.Cloud.Provider, "gcp")
	}
}
