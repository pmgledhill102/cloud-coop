package cli

import (
	"context"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/config"
)

func TestRunConfigShow_AllKeys(t *testing.T) {
	cfg := &config.Config{
		Cloud: config.CloudConfig{
			Provider: "gcp",
			GCP: config.GCPConfig{
				Project: "test-project",
				Zone:    "us-central1-a",
			},
		},
		VM: config.VMConfig{
			Name: "test-vm",
		},
		SSH: config.SSHConfig{
			Port: 22,
			User: "testuser",
		},
	}
	cleanup := withMockConfig(cfg, nil)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runConfigShow(cmd, []string{})
	if err != nil {
		t.Errorf("runConfigShow() unexpected error: %v", err)
	}
}

func TestRunConfigShow_SpecificValidKey(t *testing.T) {
	cfg := &config.Config{
		Cloud: config.CloudConfig{
			Provider: "gcp",
			GCP: config.GCPConfig{
				Project: "my-project",
				Zone:    "us-central1-a",
			},
		},
		VM: config.VMConfig{
			Name: "my-vm",
		},
		SSH: config.SSHConfig{
			Port: 22,
		},
	}
	cleanup := withMockConfig(cfg, nil)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runConfigShow(cmd, []string{"cloud.gcp.project"})
	if err != nil {
		t.Errorf("runConfigShow() unexpected error: %v", err)
	}
}

func TestRunConfigShow_UnknownKey(t *testing.T) {
	cfg := &config.Config{
		Cloud: config.CloudConfig{
			Provider: "gcp",
			GCP: config.GCPConfig{
				Project: "test-project",
				Zone:    "us-central1-a",
			},
		},
		VM: config.VMConfig{
			Name: "test-vm",
		},
		SSH: config.SSHConfig{
			Port: 22,
		},
	}
	cleanup := withMockConfig(cfg, nil)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	// Should not error - prints available keys to stderr and returns nil
	err := runConfigShow(cmd, []string{"nonexistent.key"})
	if err != nil {
		t.Errorf("runConfigShow() unexpected error: %v", err)
	}
}

func TestRunConfigShow_EmptyValue(t *testing.T) {
	cfg := &config.Config{
		Cloud: config.CloudConfig{
			Provider: "gcp",
			GCP: config.GCPConfig{
				Project: "", // Empty
				Zone:    "us-central1-a",
			},
		},
		VM: config.VMConfig{
			Name: "test-vm",
		},
		SSH: config.SSHConfig{
			Port: 22,
		},
	}
	cleanup := withMockConfig(cfg, nil)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runConfigShow(cmd, []string{"cloud.gcp.project"})
	if err != nil {
		t.Errorf("runConfigShow() unexpected error: %v", err)
	}
}
