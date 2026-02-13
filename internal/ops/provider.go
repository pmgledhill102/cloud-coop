// Package ops provides shared operations used by both the TUI and CLI.
// It eliminates duplication by centralising provider creation, VM access
// management, SSH connection setup, and VM state validation.
package ops

import (
	"context"
	"fmt"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/cloud/gcp"
	"github.com/cloud-coop/cloudcoop/internal/config"
)

// NewProvider creates a cloud provider based on the config's cloud.provider setting.
func NewProvider(ctx context.Context, cfg *config.Config) (cloud.Provider, func(), error) {
	switch cfg.Cloud.Provider {
	case "gcp":
		p, err := gcp.New(ctx, cfg.Cloud.GCP.Project, cfg.Cloud.GCP.Zone)
		if err != nil {
			return nil, nil, fmt.Errorf("create GCP provider: %w", err)
		}
		return p, func() { _ = p.Close() }, nil
	default:
		return nil, nil, fmt.Errorf("unsupported provider: %s", cfg.Cloud.Provider)
	}
}
