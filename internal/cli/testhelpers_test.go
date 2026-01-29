package cli

import (
	"context"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
	"github.com/cloud-coop/cloudcoop/internal/testutil"
)

// testConfig returns a valid test configuration.
func testConfig() *config.Config {
	return &config.Config{
		Cloud: config.CloudConfig{
			Provider: "gcp",
			GCP: config.GCPConfig{
				Project:        "test-project",
				Zone:           "us-central1-a",
				ServiceAccount: "cloudcoop-vm@test-project.iam.gserviceaccount.com",
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

// newNoopSSHMock creates a mock SSH client that accepts any command.
func newNoopSSHMock() *testutil.MockSSHClient {
	return testutil.NewMockSSHClient().ExpectAnyCommand("", nil)
}

// withMockSSHClient sets up a mock SSH client factory and returns a cleanup function.
func withMockSSHClient(mock *testutil.MockSSHClient) func() {
	original := sshClientFactory
	sshClientFactory = func(cfg ssh.Config) (ssh.Runner, error) {
		return mock, nil
	}
	return func() {
		sshClientFactory = original
	}
}

// withFullMocks sets up mock provider, config, and SSH client, returning a cleanup function.
func withFullMocks(cfg *config.Config, provider *cloud.MockProvider, sshClient *testutil.MockSSHClient) func() {
	cleanupConfig := withMockConfig(cfg, nil)
	cleanupProvider := withMockProvider(provider)
	cleanupSSH := withMockSSHClient(sshClient)
	return func() {
		cleanupSSH()
		cleanupProvider()
		cleanupConfig()
	}
}
