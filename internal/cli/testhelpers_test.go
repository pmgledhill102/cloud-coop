package cli

import (
	"context"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
)

// testConfig returns a valid test configuration.
func testConfig() *config.Config {
	return &config.Config{
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
}

// withMockProvider sets up a mock provider for testing and returns a cleanup function.
func withMockProvider(mock *cloud.MockProvider) func() {
	original := providerFactory
	providerFactory = func(ctx context.Context, cfg *config.Config) (cloud.Provider, func(), error) {
		return mock, func() {}, nil
	}
	return func() {
		providerFactory = original
	}
}

// withMockConfig sets up a mock config loader and returns a cleanup function.
func withMockConfig(cfg *config.Config, err error) func() {
	original := configLoader
	configLoader = func() (*config.Config, error) {
		return cfg, err
	}
	return func() {
		configLoader = original
	}
}

// withMocks sets up both mock provider and config, returning a cleanup function.
func withMocks(cfg *config.Config, mock *cloud.MockProvider) func() {
	cleanupConfig := withMockConfig(cfg, nil)
	cleanupProvider := withMockProvider(mock)
	return func() {
		cleanupProvider()
		cleanupConfig()
	}
}
