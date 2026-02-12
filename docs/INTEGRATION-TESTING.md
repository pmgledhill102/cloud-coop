# Integration Testing

End-to-end integration tests that exercise cloudcoop against a real GCP environment. These tests
run infrequently (weekly CI + manual trigger) and validate the full VM lifecycle, SSH connectivity,
provisioning, agent operations, and firewall management against live cloud infrastructure.

## Why Integration Tests

Unit tests cover ~45% of the codebase with mocks for cloud providers, SSH, and config. Integration
tests catch what mocks cannot:

- GCP API behavior changes and deprecations
- SSH connectivity and host key handling against real VMs
- Provisioning script reliability (all 16 steps)
- Firewall rule creation and idempotency
- Spot instance scheduling and metadata
- Agent tmux operations on a fully provisioned VM

## Prerequisites

### Dedicated GCP Project

Integration tests require an isolated GCP project to avoid interference with production resources.

**Required APIs:**

- `compute.googleapis.com`
- `iam.googleapis.com`
- `logging.googleapis.com`

**Service account:**

- Name: `cc-integration-test`
- Roles: `roles/compute.admin`, `roles/iam.serviceAccountUser`
- Key: JSON key file (stored as GitHub Actions secret for CI)

**Budget alert:** $50/month with alerts at 50%, 90%, 100%

See [Terraform Configuration](#terraform-configuration) below for automated provisioning
of the project resources.

### Environment Variables

```bash
# Required
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/sa-key.json
export GOOGLE_CLOUD_PROJECT=cloudcoop-integration-test

# Optional (defaults shown)
export GOOGLE_CLOUD_ZONE=europe-north2-a
```

Tests skip automatically when `GOOGLE_APPLICATION_CREDENTIALS` or `GOOGLE_CLOUD_PROJECT`
are not set.

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
        Network:            "default",
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

The integration test workflow runs weekly and on manual trigger:

```yaml
name: Integration Tests
on:
  schedule:
    - cron: '0 6 * * 1'    # Weekly Monday 6am UTC
  workflow_dispatch:         # Manual trigger

jobs:
  integration:
    runs-on: ubuntu-latest
    timeout-minutes: 45
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: google-github-actions/auth@v2
        with:
          credentials_json: '${{ secrets.GCP_INTEGRATION_SA_KEY }}'
      - name: Run integration tests
        env:
          GOOGLE_CLOUD_PROJECT: ${{ vars.GCP_INTEGRATION_PROJECT }}
          GOOGLE_CLOUD_ZONE: us-central1-a
        run: go test -v -tags=integration -timeout 30m ./integration/...
      - name: Cleanup orphaned VMs
        if: always()
        run: |
          gcloud compute instances list \
            --project=${{ vars.GCP_INTEGRATION_PROJECT }} \
            --filter="name~cc-inttest-" \
            --format="value(name,zone)" | \
          while read name zone; do
            gcloud compute instances delete "$name" --zone="$zone" --quiet || true
          done
```

### Required GitHub Configuration

- **Secret:** `GCP_INTEGRATION_SA_KEY` -- Service account JSON key
- **Variable:** `GCP_INTEGRATION_PROJECT` -- GCP project ID

## Terraform Configuration

Terraform files for provisioning the dedicated GCP project live in `integration/terraform/`.
These are reference configurations intended to be integrated into your own Terraform ecosystem.

**Resources provisioned:**

- Required API enablement (`compute`, `iam`, `logging`)
- Service account (`cc-integration-test`) with `compute.admin` and `iam.serviceAccountUser`
- Billing budget ($50/month with percentage alerts)

**Usage:**

```bash
cd integration/terraform
terraform init
terraform plan -var="project_id=cloudcoop-integration-test" \
               -var="billing_account=XXXXXX-XXXXXX-XXXXXX"
terraform apply
```

## TUI Testing Strategy

### Current Approach (Unit-Level)

The TUI has 50+ unit tests in `internal/tui/app_test.go` using a custom `testutil.TestModel`
helper that synchronously drives the bubbletea model. These test key handlers, message handlers,
and view rendering without a real terminal or cloud connection.

This approach is recommended for TUI logic because:

- Tests run in milliseconds
- No terminal or pty required
- Mock providers isolate cloud behavior
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
    --project=cloudcoop-integration-test \
    --filter="name~cc-inttest-"

# Delete them
gcloud compute instances delete <name> --zone=<zone> --quiet
```

The `max_uptime_minutes=60` safety net ensures orphaned VMs auto-stop within an hour.

### Quota Issues

Integration tests use the smallest available machine type (c4a-highcpu-4). If you hit quota
limits, check:

```bash
gcloud compute regions describe us-central1 \
    --project=cloudcoop-integration-test \
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
