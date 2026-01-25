# Development Environment Setup

This document describes how to configure a safe development environment for building and testing the Sandbox Manager TUI. The goal is to provide Claude Code (or another coding agent) with sufficient access to develop and test against real cloud resources, while limiting blast radius.

## Principles

1. **Isolated project** - Dedicated GCP project for development, separate from personal/production
2. **Least privilege** - Service account with minimum permissions required
3. **Cost controls** - Budget alerts, small instances, auto-cleanup
4. **Short feedback loops** - Agent can create/destroy test resources freely within the sandbox

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Your GCP Organization / Personal Account                               │
│                                                                         │
│  ┌─────────────────────────┐    ┌─────────────────────────────────────┐│
│  │  Production Projects    │    │  sandbox-manager-dev (isolated)     ││
│  │  (no agent access)      │    │                                     ││
│  │                         │    │  ┌─────────────────────────────┐   ││
│  │  • Personal project     │    │  │  dev-agent@...iam.gsa.com   │   ││
│  │  • Work projects        │    │  │                             │   ││
│  │  • Billing accounts     │    │  │  Permissions (project-only):│   ││
│  │                         │    │  │  • Compute Admin            │   ││
│  └─────────────────────────┘    │  │  • Secret Manager Admin     │   ││
│           ╳                     │  │  • Service Account User     │   ││
│    No access                    │  │  • Logging Writer           │   ││
│                                 │  └─────────────────────────────┘   ││
│                                 │                                     ││
│                                 │  Resources (disposable):            ││
│                                 │  • Test VMs                         ││
│                                 │  • Test service accounts            ││
│                                 │  • Test secrets                     ││
│                                 └─────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────────────┘
```

## Setup Instructions

### 1. Create Isolated Development Project

```bash
# Create a new project specifically for development
export DEV_PROJECT_ID="sandbox-manager-dev-$(date +%Y%m)"
gcloud projects create $DEV_PROJECT_ID --name="Sandbox Manager Dev"

# Link to billing account (required for Compute Engine)
gcloud billing accounts list
export BILLING_ACCOUNT="XXXXXX-XXXXXX-XXXXXX"
gcloud billing projects link $DEV_PROJECT_ID --billing-account=$BILLING_ACCOUNT

# Enable required APIs
gcloud services enable compute.googleapis.com --project=$DEV_PROJECT_ID
gcloud services enable secretmanager.googleapis.com --project=$DEV_PROJECT_ID
gcloud services enable iam.googleapis.com --project=$DEV_PROJECT_ID
gcloud services enable cloudresourcemanager.googleapis.com --project=$DEV_PROJECT_ID
```

### 2. Create Development Service Account

```bash
# Create service account for the coding agent
gcloud iam service-accounts create dev-agent \
  --project=$DEV_PROJECT_ID \
  --display-name="Development Agent"

export DEV_SA="dev-agent@${DEV_PROJECT_ID}.iam.gserviceaccount.com"

# Grant permissions ONLY within this project
# Compute Admin - create/start/stop/delete VMs
gcloud projects add-iam-policy-binding $DEV_PROJECT_ID \
  --member="serviceAccount:$DEV_SA" \
  --role="roles/compute.admin"

# Secret Manager Admin - manage test secrets
gcloud projects add-iam-policy-binding $DEV_PROJECT_ID \
  --member="serviceAccount:$DEV_SA" \
  --role="roles/secretmanager.admin"

# Service Account User - use service accounts in the project
gcloud projects add-iam-policy-binding $DEV_PROJECT_ID \
  --member="serviceAccount:$DEV_SA" \
  --role="roles/iam.serviceAccountUser"

# Service Account Admin - create test service accounts
gcloud projects add-iam-policy-binding $DEV_PROJECT_ID \
  --member="serviceAccount:$DEV_SA" \
  --role="roles/iam.serviceAccountAdmin"

# Logging Writer - write logs for testing
gcloud projects add-iam-policy-binding $DEV_PROJECT_ID \
  --member="serviceAccount:$DEV_SA" \
  --role="roles/logging.logWriter"

# Logging Viewer - read logs for verification
gcloud projects add-iam-policy-binding $DEV_PROJECT_ID \
  --member="serviceAccount:$DEV_SA" \
  --role="roles/logging.viewer"
```

### 3. Generate Service Account Key (for local development)

```bash
# Create key for local development
gcloud iam service-accounts keys create ./dev-credentials.json \
  --iam-account=$DEV_SA \
  --project=$DEV_PROJECT_ID

# IMPORTANT: Add to .gitignore
echo "dev-credentials.json" >> .gitignore

# Set environment variable for SDK authentication
export GOOGLE_APPLICATION_CREDENTIALS="$(pwd)/dev-credentials.json"
export GOOGLE_CLOUD_PROJECT="$DEV_PROJECT_ID"
```

### 4. Set Up Budget Alerts

```bash
# Create a budget to prevent runaway costs
# This requires the Billing API and billing account admin access

# Via console: https://console.cloud.google.com/billing/budgets
# Or via gcloud (if you have billing admin):

gcloud billing budgets create \
  --billing-account=$BILLING_ACCOUNT \
  --display-name="Sandbox Manager Dev Budget" \
  --budget-amount=50 \
  --threshold-rule=percent=50 \
  --threshold-rule=percent=90 \
  --threshold-rule=percent=100 \
  --filter-projects="projects/$DEV_PROJECT_ID"
```

**Recommended budget: $50/month** - sufficient for development, alerts before damage.

### 5. Configure Quotas (Optional)

Limit resource creation to prevent accidents:

```bash
# Limit to small number of VMs
gcloud compute project-info set-usage-export-bucket $DEV_PROJECT_ID \
  --bucket=gs://your-usage-bucket  # Optional: export usage data

# Via console, set quotas:
# - CPUs: 32 (enough for 2x c4a-highcpu-16)
# - Persistent Disk SSD: 200GB
# - IP addresses: 5
```

## Testing Strategy

### Test Levels

```
┌─────────────────────────────────────────────────────────────────────┐
│  Level 1: Unit Tests (no cloud access)                              │
│  • Mock cloud SDK clients                                           │
│  • Test business logic, state machines, TUI rendering               │
│  • Fast: milliseconds                                               │
│  • Run: Every code change                                           │
├─────────────────────────────────────────────────────────────────────┤
│  Level 2: Integration Tests (real API calls)                        │
│  • Call real GCP APIs against dev project                           │
│  • Test SDK wrappers, auth, error handling                          │
│  • Medium: seconds to minutes                                       │
│  • Run: Before commit                                               │
├─────────────────────────────────────────────────────────────────────┤
│  Level 3: E2E Tests (full workflow)                                 │
│  • Create real VM, start/stop, resize, delete                       │
│  • Test complete user workflows                                     │
│  • Slow: minutes                                                    │
│  • Run: Before merge, nightly                                       │
└─────────────────────────────────────────────────────────────────────┘
```

### Unit Tests (Mocked)

```go
// internal/cloud/gcp/provider_test.go
package gcp

import (
    "context"
    "testing"

    "cloud.google.com/go/compute/apiv1/computepb"
    "github.com/stretchr/testify/mock"
)

type MockInstancesClient struct {
    mock.Mock
}

func (m *MockInstancesClient) Get(ctx context.Context, req *computepb.GetInstanceRequest) (*computepb.Instance, error) {
    args := m.Called(ctx, req)
    return args.Get(0).(*computepb.Instance), args.Error(1)
}

func TestGetVMStatus_Running(t *testing.T) {
    mockClient := new(MockInstancesClient)
    mockClient.On("Get", mock.Anything, mock.Anything).Return(
        &computepb.Instance{Status: proto.String("RUNNING")},
        nil,
    )

    provider := &GCPProvider{client: mockClient}
    status, err := provider.GetVMStatus(context.Background(), "test-vm")

    assert.NoError(t, err)
    assert.Equal(t, VMStatusRunning, status)
}
```

### Integration Tests (Real APIs)

```go
// internal/cloud/gcp/integration_test.go
//go:build integration

package gcp

import (
    "context"
    "os"
    "testing"
)

func TestRealVMOperations(t *testing.T) {
    if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
        t.Skip("Skipping integration test: no credentials")
    }

    project := os.Getenv("GOOGLE_CLOUD_PROJECT")
    if project == "" {
        t.Fatal("GOOGLE_CLOUD_PROJECT must be set")
    }

    ctx := context.Background()
    provider, err := NewGCPProvider(ctx, project, "europe-north2-a")
    require.NoError(t, err)

    // Test: Get status of non-existent VM
    status, err := provider.GetVMStatus(ctx, "nonexistent-vm-12345")
    assert.Equal(t, VMStatusNotFound, status)
}
```

Run integration tests:
```bash
# Set credentials
export GOOGLE_APPLICATION_CREDENTIALS="./dev-credentials.json"
export GOOGLE_CLOUD_PROJECT="sandbox-manager-dev-202601"

# Run integration tests only
go test -tags=integration ./internal/cloud/gcp/...
```

### E2E Tests (Full Workflow)

```go
// e2e/vm_lifecycle_test.go
//go:build e2e

package e2e

import (
    "context"
    "testing"
    "time"
)

func TestVMLifecycle(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    defer cancel()

    vmName := fmt.Sprintf("e2e-test-%d", time.Now().Unix())

    // Create VM
    err := provider.CreateVM(ctx, VMConfig{
        Name:        vmName,
        MachineType: "e2-micro",  // Smallest for testing
        DiskSizeGB:  10,
        Spot:        true,
    })
    require.NoError(t, err)

    // Cleanup on exit
    defer func() {
        provider.DeleteVM(context.Background(), vmName)
    }()

    // Wait for running
    require.Eventually(t, func() bool {
        status, _ := provider.GetVMStatus(ctx, vmName)
        return status == VMStatusRunning
    }, 2*time.Minute, 5*time.Second)

    // Stop VM
    err = provider.StopVM(ctx, vmName)
    require.NoError(t, err)

    // Wait for stopped
    require.Eventually(t, func() bool {
        status, _ := provider.GetVMStatus(ctx, vmName)
        return status == VMStatusStopped
    }, 2*time.Minute, 5*time.Second)

    // Resize VM
    err = provider.ResizeVM(ctx, vmName, "e2-small")
    require.NoError(t, err)

    // Start VM
    err = provider.StartVM(ctx, vmName)
    require.NoError(t, err)

    // Verify running with new size
    require.Eventually(t, func() bool {
        status, _ := provider.GetVMStatus(ctx, vmName)
        return status == VMStatusRunning
    }, 2*time.Minute, 5*time.Second)
}
```

Run E2E tests:
```bash
# Run E2E tests (slow, costs money)
go test -tags=e2e -timeout=15m ./e2e/...
```

## Resource Naming Convention

Use consistent naming to identify and clean up test resources:

```
Pattern: {type}-{purpose}-{timestamp}

Examples:
- e2e-test-1706184000        # E2E test VM
- integration-vm-1706184000   # Integration test VM
- dev-manual-testing          # Manual testing VM
```

## Cleanup Script

Create a script to remove orphaned test resources:

```bash
#!/bin/bash
# scripts/cleanup-dev-resources.sh

PROJECT="${GOOGLE_CLOUD_PROJECT:-sandbox-manager-dev}"
ZONE="europe-north2-a"
MAX_AGE_HOURS=4

echo "Cleaning up test resources in $PROJECT..."

# Delete VMs older than MAX_AGE_HOURS
gcloud compute instances list \
  --project=$PROJECT \
  --filter="name~'^(e2e-test|integration-vm)-' AND creationTimestamp<'-P${MAX_AGE_HOURS}H'" \
  --format="value(name)" | while read vm; do
    echo "Deleting old test VM: $vm"
    gcloud compute instances delete $vm --zone=$ZONE --quiet --project=$PROJECT
done

# Delete orphaned disks
gcloud compute disks list \
  --project=$PROJECT \
  --filter="name~'^(e2e-test|integration-vm)-' AND NOT users:*" \
  --format="value(name)" | while read disk; do
    echo "Deleting orphaned disk: $disk"
    gcloud compute disks delete $disk --zone=$ZONE --quiet --project=$PROJECT
done

echo "Cleanup complete."
```

## CI/CD Integration

### GitHub Actions Example

```yaml
# .github/workflows/test.yml
name: Tests

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test ./...

  integration-tests:
    runs-on: ubuntu-latest
    # Only run on main branch or with label
    if: github.ref == 'refs/heads/main' || contains(github.event.pull_request.labels.*.name, 'run-integration')
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - uses: google-github-actions/auth@v2
        with:
          credentials_json: ${{ secrets.GCP_DEV_CREDENTIALS }}
      - run: |
          export GOOGLE_CLOUD_PROJECT="sandbox-manager-dev"
          go test -tags=integration ./internal/cloud/gcp/...

  e2e-tests:
    runs-on: ubuntu-latest
    # Only run nightly or with explicit label
    if: github.event_name == 'schedule' || contains(github.event.pull_request.labels.*.name, 'run-e2e')
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - uses: google-github-actions/auth@v2
        with:
          credentials_json: ${{ secrets.GCP_DEV_CREDENTIALS }}
      - run: |
          export GOOGLE_CLOUD_PROJECT="sandbox-manager-dev"
          go test -tags=e2e -timeout=15m ./e2e/...
      - name: Cleanup
        if: always()
        run: ./scripts/cleanup-dev-resources.sh
```

## What the Agent Can and Cannot Do

### Within Dev Project (Allowed)

| Action | Allowed | Notes |
|--------|---------|-------|
| Create VMs | ✓ | Any size, but budget alerts will fire |
| Delete VMs | ✓ | Including accidental deletion of test VMs |
| Start/Stop VMs | ✓ | Normal operations |
| Create service accounts | ✓ | For testing SA workflows |
| Create secrets | ✓ | For testing secret management |
| View logs | ✓ | For debugging |
| Modify IAM within project | ✓ | Can grant roles within the project |

### Outside Dev Project (Blocked)

| Action | Blocked | Why |
|--------|---------|-----|
| Access other projects | ✓ | No IAM bindings |
| Modify billing | ✓ | No billing admin role |
| Create projects | ✓ | No org-level permissions |
| Access production data | ✓ | Completely isolated |
| Modify org policies | ✓ | No org admin role |

## Cost Estimation

| Resource | Usage | Est. Monthly Cost |
|----------|-------|-------------------|
| E2E test VMs | ~10 runs × 10 min | ~$1 |
| Integration tests | ~100 API calls/day | ~$0.10 |
| Dev VM (manual testing) | ~20 hrs/month spot | ~$5 |
| Persistent disks | ~50GB retained | ~$5 |
| **Total** | | **~$12/month** |

Budget alert at $50 provides 4x headroom for unexpected usage.

## Quick Start Checklist

```bash
# 1. Create project (one-time)
□ gcloud projects create sandbox-manager-dev-YYYYMM
□ Link billing account
□ Enable APIs (compute, secretmanager, iam)

# 2. Create service account (one-time)
□ Create dev-agent service account
□ Grant Compute Admin, Secret Manager Admin, SA Admin roles
□ Generate and download key JSON

# 3. Configure local environment
□ Add dev-credentials.json to .gitignore
□ Set GOOGLE_APPLICATION_CREDENTIALS
□ Set GOOGLE_CLOUD_PROJECT

# 4. Set up safety nets
□ Create budget alert ($50)
□ Set resource quotas (optional)

# 5. Verify access
□ Run: gcloud compute instances list --project=$DEV_PROJECT_ID
□ Should return empty list (no error)

# 6. Run tests
□ go test ./...                           # Unit tests
□ go test -tags=integration ./...         # Integration tests
□ go test -tags=e2e -timeout=15m ./e2e/... # E2E tests
```

## Troubleshooting

### "Permission denied" errors

```bash
# Check current authentication
gcloud auth list

# Verify service account permissions
gcloud projects get-iam-policy $DEV_PROJECT_ID \
  --filter="bindings.members:$DEV_SA" \
  --format="table(bindings.role)"
```

### Tests creating resources but not cleaning up

```bash
# Run cleanup script
./scripts/cleanup-dev-resources.sh

# Or manually list and delete
gcloud compute instances list --project=$DEV_PROJECT_ID
gcloud compute instances delete <name> --zone=europe-north2-a --project=$DEV_PROJECT_ID
```

### Budget alerts firing unexpectedly

1. Check for orphaned resources: `gcloud compute instances list`
2. Check for large disks: `gcloud compute disks list`
3. Run cleanup script
4. Review test code for missing cleanup in `defer` statements
