# Integration Testing

End-to-end integration tests that exercise cloudcoop against a real GCP environment. These tests
run infrequently (weekly CI + manual trigger) and validate the full VM lifecycle, SSH connectivity,
provisioning, agent operations, and firewall management against live cloud infrastructure.

## Why Integration Tests

Unit tests cover ~45% of the codebase with mocks for cloud providers, SSH, and config. Integration
tests catch what mocks cannot:

- GCP API behaviour changes and deprecations
- SSH connectivity and host key handling against real VMs
- Provisioning script reliability (all 16 steps)
- Firewall rule creation and idempotency
- Spot instance scheduling and metadata
- Agent tmux operations on a fully provisioned VM

## Prerequisites

### Dedicated GCP Project

Integration tests require an isolated GCP project to avoid interference with production resources.
The project is provisioned via Terraform in the
[gcp-org-management](https://github.com/pmgledhill102/gcp-org-management) repo
(`layers/3-projects/staging/cloud-coop.tf`).

**Project:** `cloud-coop` (in the `staging` folder)

**Required APIs:** `compute`, `iam`, `logging`

**Service account:** `cc-integration-test@cloud-coop.iam.gserviceaccount.com`

- Roles: `roles/compute.admin`, `roles/iam.serviceAccountUser`
- Authentication: SA impersonation (no long-lived keys per org policy)

**Network:** Custom VPC `inttest` with subnet `inttest-europe-north2` (`10.0.0.0/24`)

**Budget alert:** £25/month with kill switch at 100%

### Local Authentication

Authenticate via SA impersonation (no JSON key files needed):

```bash
gcloud auth application-default login \
  --impersonate-service-account=cc-integration-test@cloud-coop.iam.gserviceaccount.com
```

### Environment Variables

```bash
# Required
export GOOGLE_CLOUD_PROJECT=cloud-coop

# Optional (defaults shown)
export GOOGLE_CLOUD_ZONE=europe-north2-a
export GOOGLE_CLOUD_NETWORK=inttest
export GOOGLE_CLOUD_SUBNET=inttest-europe-north2
```

Tests skip automatically when `GOOGLE_CLOUD_PROJECT` is not set.

## Running Tests

```bash
# Full integration suite (20-25 minutes)
make test-integration-cloud

# Or directly with Go
go test -v -tags=integration -timeout 30m ./integration/...

# Unit tests are unaffected (no build tag)
make test
```

## Test Architecture

### Design Principles

- **Single shared VM per run** -- Create once, reuse across all test phases, delete at end.
  Avoids the cost of multiple VMs and the 10+ minute provisioning time per VM.
- **Full provisioning** -- Waits for all 16 provisioning steps to complete (~10-15 min).
  This validates the provisioning script and enables agent/tmux testing.
- **Ordered phases** -- `TestMain` orchestrates sequential test phases that build on each other.
- **Cleanup guarantees** -- `defer` cleanup in `TestMain` deletes the VM even on panic.
  The `max_uptime_minutes=60` config provides a GCP-level safety net.
- **Unique naming** -- VM names use `cc-inttest-{unix-timestamp}` to avoid collisions
  between concurrent runs and make orphan detection trivial.

### File Structure

```text
integration/
    integration_test.go      # TestMain, env setup, Phase 0 (credential validation)
    helpers_test.go           # testEnv struct, shared helpers, cleanup
    firewall_test.go          # Phase 1: Firewall rule CRUD
    lifecycle_test.go         # Phase 2: VM create; Phase 5: stop/start; Phase 6: delete
    ssh_test.go               # Phase 3a: SSH connectivity and key management
    provisioning_test.go      # Phase 3b: Wait for provisioning, check status/logs
    agents_test.go            # Phase 4: Agent CRUD operations
    testdata/
        integration.toml      # Test config template
```

All files use the `//go:build integration` build tag.

### Test Orchestration

A shared `testEnv` struct holds state across the entire test run:

```go
type testEnv struct {
    cfg          *config.Config
    provider     *gcp.Provider
    vmName       string
    vmInfo       *cloud.VMInfo
    sshPublicKey string
    projectID    string
    zone         string
}

var env *testEnv

func TestMain(m *testing.M) {
    env = setupEnv()    // Read env vars, create provider, generate unique VM name
    code := m.Run()
    env.cleanup()       // Always delete VM + firewall rules, even on panic
    os.Exit(code)
}
```

### Test Phases

Tests execute sequentially in phases. Each phase builds on resources created by earlier phases.

| Phase | File | What It Tests | Duration |
|-------|------|---------------|----------|
| 0 | `integration_test.go` | Credential validation, project access | ~5s |
| 1 | `firewall_test.go` | Create rule, update with new IP, idempotent no-op | ~15s |
| 2 | `lifecycle_test.go` | Create spot VM (max_uptime=60), verify status/metadata | ~3 min |
| 3a | `ssh_test.go` | SSH connect, run command, key provisioning idempotency | ~30s |
| 3b | `provisioning_test.go` | Poll status until completed, verify logs exist | ~10-15 min |
| 4 | `agents_test.go` | List (empty), add agent, add second, kill, verify list | ~1 min |
| 5 | `lifecycle_test.go` | Stop VM, verify stopped, start, verify running + SSH | ~5 min |
| 6 | `lifecycle_test.go` | Delete VM, verify not_found, clean up firewall rule | ~2 min |

Total: ~20-25 minutes.

### Timeout Strategy

| Operation | Timeout |
|-----------|---------|
| VM creation | 5 min |
| Provisioning poll | 15 min |
| SSH operations | 30 sec |
| VM start/stop | 3 min |
| Overall test suite | 30 min |

### Example Test Code

**VM creation (Phase 2):**

```go
func TestVM_Create(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    createCfg := cloud.VMCreateConfig{
        Name:               env.vmName,
        MachineType:        "c4a-highcpu-4",  // Smallest/cheapest
        DiskSizeGB:         30,                // Minimal for tests
        Image:              "projects/ubuntu-os-cloud/global/images/family/ubuntu-2510-arm64",
        Spot:               true,
        MaxUptimeMinutes:   60,                // Safety net: GCP auto-stops after 1 hour
        Network:            "inttest",
        Subnet:             "inttest-europe-north2",
        SSHPort:            22,
        SSHUser:            "integration",
        SSHPublicKey:       env.sshPublicKey,
        ServiceAccount:     env.cfg.Cloud.GCP.ServiceAccount,
        ProvisionScriptURL: env.cfg.Provisioning.ScriptURL,
    }
    err := env.provider.CreateVM(ctx, createCfg)
    require.NoError(t, err)
}
```

**Provisioning wait (Phase 3b):**

```go
func TestProvisioning_WaitForCompletion(t *testing.T) {
    deadline := time.Now().Add(15 * time.Minute)
    for time.Now().Before(deadline) {
        status, err := provisioning.CheckStatus(sshClient)
        if err == nil && status.State == "completed" {
            return
        }
        if err == nil && status.State == "failed" {
            t.Fatalf("Provisioning failed: %s", status.Message)
        }
        time.Sleep(15 * time.Second)
    }
    t.Fatal("Provisioning did not complete within 15 minutes")
}
```

**Agent operations (Phase 4):**

```go
func TestAgents_AddAndList(t *testing.T) {
    client := env.connectSSH(t)
    defer client.Close()

    session, err := agent.CreateSession(client, "agents", agent.CreateSessionOptions{
        Command: "echo 'test agent running'; sleep 3600",
    })
    require.NoError(t, err)
    assert.NotEmpty(t, session.Name)

    result, err := agent.ListSessions(client, "agents")
    require.NoError(t, err)
    assert.Len(t, result.Sessions, 1)
}
```

## New Feature: `max_uptime_minutes`

Integration tests depend on a new `max_uptime_minutes` config option that sets GCP's native
[`maxRunDuration`](https://cloud.google.com/compute/docs/instances/limit-vm-runtime) on the VM.
When the duration expires, GCP automatically stops the VM. This protects both test environments
and production from runaway costs.

### Configuration

```toml
[vm]
max_uptime_minutes = 60   # Auto-stop VM after 60 minutes (0 = disabled)
```

### CLI

```bash
cloudcoop create -s small --max-uptime 60
```

The `--max-uptime` flag overrides the config value. When set to 0 (default), no time limit
is applied.

### Implementation

Uses the `MaxRunDuration` field on `computepb.Scheduling`, which is already available in the
GCP Go SDK. Composes correctly with spot instance scheduling (both fields live on the same
`Scheduling` object). The termination action is set to `STOP` to preserve the boot disk.

## Cost Controls

Integration tests use five layers of cost protection:

| Control | Mechanism |
|---------|-----------|
| **Auto-stop** | `max_uptime_minutes=60` -- GCP stops VM after 1 hour |
| **Spot instances** | ~70% cheaper than on-demand |
| **Smallest machine** | c4a-highcpu-4 (~$0.03/hr spot) |
| **Budget alert** | $50/month on the dedicated project |
| **CI cleanup** | `always()` step force-deletes orphaned `cc-inttest-*` VMs |
| **Weekly schedule** | CI runs once per week, not on every push |
| **Naming convention** | `cc-inttest-*` prefix makes orphan detection trivial |

**Estimated cost per run:** ~$0.02

**Estimated monthly cost (weekly CI):** ~$0.08

## CI Configuration

### GitHub Actions Workflow

The integration test workflow runs weekly and on manual trigger. Authentication uses
Workload Identity Federation (no long-lived SA keys).

See `.github/workflows/integration.yml` for the full workflow.

### Required GitHub Configuration

- **Variable:** `GCP_INTEGRATION_PROJECT` -- GCP project ID (`cloud-coop`)
- **Variable:** `GCP_WIF_PROVIDER` -- Workload Identity Federation provider resource name

WIF must be configured in the `gcp-org-management` layer 0-bootstrap to allow the
`pmgledhill102/cloud-coop` repo to impersonate the `cc-integration-test` service account.

## Terraform Configuration

The GCP project and resources are managed in the
[gcp-org-management](https://github.com/pmgledhill102/gcp-org-management) repo.

**File:** `layers/3-projects/staging/cloud-coop.tf`

**Resources provisioned:**

- GCP project `cloud-coop` (in staging folder, £25/month budget with kill switch)
- Service account `cc-integration-test` with `compute.admin` and `iam.serviceAccountUser`
- SA impersonation binding for `paul@pmgledhill.com`
- Custom VPC `inttest` with subnet `inttest-europe-north2` (`10.0.0.0/24`)
- Firewall rules: SSH from anywhere, internal traffic within subnet

A reference copy of the Terraform config lives in `integration/terraform/cloud-coop.tf`.

## TUI Testing Strategy

### Current Approach (Unit-Level)

The TUI has 50+ unit tests in `internal/tui/app_test.go` using a custom `testutil.TestModel`
helper that synchronously drives the bubbletea model. These test key handlers, message handlers,
and view rendering without a real terminal or cloud connection.

This approach is recommended for TUI logic because:

- Tests run in milliseconds
- No terminal or pty required
- Mock providers isolate cloud behaviour
- The same code paths tested by integration tests (provider, SSH, agent) are called by the TUI

### Golden File Testing with `teatest`

[`teatest`](https://charm.land/blog/teatest/) from Charmbracelet enables snapshot testing of
TUI output. Golden files capture the exact rendered output for known states and flag visual
regressions automatically.

```go
func init() {
    lipgloss.SetColorProfile(termenv.Ascii) // Deterministic output across environments
}

func TestGolden_VMRunning(t *testing.T) {
    // Build model in known state, capture output, compare to golden file
    // Golden files stored in internal/tui/testdata/TestGolden_VMRunning.golden
}
```

Update golden files with:

```bash
go test ./internal/tui/ -update
```

Add `*.golden -text` to `.gitattributes` to prevent Git from modifying line endings.

### VHS for Documentation

[VHS](https://github.com/charmbracelet/vhs) records terminal sessions as GIFs from `.tape`
scripts. Useful for README demos and release documentation but too heavyweight for automated
CI testing (requires ttyd + ffmpeg).

### TUI Integration Testing (Not Recommended)

Testing the full TUI against a real cloud backend would require:

- Starting the bubbletea program with a pty
- Sending keystrokes and parsing ANSI output
- Waiting for async cloud operations to complete

This is fragile and slow. The integration tests exercise the same provider/SSH/agent code
paths that the TUI calls. Keep TUI tests at the unit level; integration tests validate the
underlying operations directly.

## Troubleshooting

### Orphaned VMs

If a test run is interrupted before cleanup:

```bash
# Find orphaned test VMs
gcloud compute instances list \
    --project=cloud-coop \
    --filter="name~cc-inttest-"

# Delete them
gcloud compute instances delete <name> --zone=<zone> --quiet
```

The `max_uptime_minutes=60` safety net ensures orphaned VMs auto-stop within an hour.

### Quota Issues

Integration tests use the smallest available machine type (c4a-highcpu-4). If you hit quota
limits, check:

```bash
gcloud compute regions describe europe-north2 \
    --project=cloud-coop \
    --format="table(quotas.metric,quotas.limit,quotas.usage)"
```

### Provisioning Timeout

If provisioning consistently times out (>15 min), check:

- VM serial console output in GCP Console
- Provisioning logs via SSH: `cat /var/run/cloudcoop/provision-progress`
- Network connectivity (apt mirrors, GitHub raw content)

## Implementation Status

| Phase | Scope | Status |
|-------|-------|--------|
| A | `max_uptime_minutes` feature (config, provider, GCP, CLI, TUI) | Done |
| B | Integration test files (helpers, lifecycle, SSH, provisioning, agents) | Done |
| C | CI workflow, Terraform, Makefile target | Done |
| D | TUI golden file tests with `teatest` (lower priority) | Future |
