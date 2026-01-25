//go:build integration

// Package integration contains integration tests that interact with external
// systems. These tests are skipped by default and only run when explicitly
// requested using the -tags=integration flag.
//
// Run integration tests:
//
//	go test -tags=integration ./integration/...
//
// Prerequisites:
//   - Set GOOGLE_APPLICATION_CREDENTIALS to service account key file
//   - Set GOOGLE_CLOUD_PROJECT to the development project ID
//
// See docs/DEVELOPMENT-ENVIRONMENT.md for full setup instructions.
package integration

import (
	"context"
	"os"
	"testing"
	"time"
)

// testConfig holds configuration for integration tests.
type testConfig struct {
	ProjectID string
	Zone      string
}

// setupTestConfig loads configuration from environment variables.
// Tests will be skipped if required environment variables are not set.
func setupTestConfig(t *testing.T) *testConfig {
	t.Helper()

	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("Skipping integration test: GOOGLE_APPLICATION_CREDENTIALS not set")
	}

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		t.Skip("Skipping integration test: GOOGLE_CLOUD_PROJECT not set")
	}

	zone := os.Getenv("GOOGLE_CLOUD_ZONE")
	if zone == "" {
		zone = "europe-north2-a" // Default zone
	}

	return &testConfig{
		ProjectID: projectID,
		Zone:      zone,
	}
}

// TestIntegrationExample demonstrates the structure of an integration test.
// Replace this with actual integration tests once cloud providers are implemented.
func TestIntegrationExample(t *testing.T) {
	cfg := setupTestConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("verify project access", func(t *testing.T) {
		// This test verifies we have valid credentials for the project.
		// In a real test, this would make an API call to verify access.

		if cfg.ProjectID == "" {
			t.Fatal("project ID is empty")
		}

		// Placeholder: Replace with actual API verification
		// Example:
		//   client, err := compute.NewInstancesRESTClient(ctx)
		//   if err != nil {
		//       t.Fatalf("failed to create client: %v", err)
		//   }
		//   defer client.Close()
		//
		//   _, err = client.List(ctx, &computepb.ListInstancesRequest{
		//       Project: cfg.ProjectID,
		//       Zone:    cfg.Zone,
		//   })
		//   if err != nil {
		//       t.Fatalf("failed to list instances: %v", err)
		//   }

		t.Logf("Would verify access to project %s in zone %s", cfg.ProjectID, cfg.Zone)

		// Simulate successful verification
		_ = ctx // Use context to avoid lint error
	})
}

// TestIntegrationCleanup demonstrates how to structure tests with cleanup.
// Resource cleanup should always use defer to ensure execution.
func TestIntegrationCleanup(t *testing.T) {
	_ = setupTestConfig(t)

	// Track resources created during the test
	var createdResources []string

	// Always clean up resources, even if test fails
	defer func() {
		for _, resource := range createdResources {
			t.Logf("Would clean up resource: %s", resource)
			// In a real test:
			// if err := provider.Delete(ctx, resource); err != nil {
			//     t.Logf("warning: failed to clean up %s: %v", resource, err)
			// }
		}
	}()

	t.Run("create and cleanup resource", func(t *testing.T) {
		// Simulate resource creation
		resourceName := "test-resource-12345"
		createdResources = append(createdResources, resourceName)

		t.Logf("Would create resource: %s", resourceName)
		// In a real test:
		// err := provider.Create(ctx, resourceName)
		// if err != nil {
		//     t.Fatalf("failed to create: %v", err)
		// }

		// Verify the resource exists and is configured correctly
		t.Logf("Would verify resource: %s", resourceName)
	})
}
