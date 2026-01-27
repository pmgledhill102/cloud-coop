# Post-MVP Implementation Approach

**Version:** 1.0
**Date:** 2026-01-27
**Status:** Draft

---

## Executive Summary

This document outlines a phased approach for cloudcoop development beyond the MVP. The roadmap
addresses security gaps, code quality improvements, feature completion, and multi-cloud expansion
over four progressive phases.

**Strategic Direction:**

1. **Security First** - Address critical SSH host key verification gap before expanding features
2. **Complete GCP** - Finish documented but unimplemented GCP features (resize, firewall, metadata)
3. **Multi-Cloud** - Expand to AWS and Azure using existing provider interface design
4. **Advanced Features** - Add sophisticated capabilities (API key management, session recovery)

**Timeline Overview:**

| Phase | Focus | Complexity |
|-------|-------|------------|
| 1 | Security & Quality Foundation | 3-4 weeks |
| 2 | GCP Feature Completion | 4-6 weeks |
| 3 | Multi-Cloud Expansion | 8-12 weeks |
| 4 | Advanced Features | 6-8 weeks |

---

## Current State Assessment

### MVP Completion Status

Based on the comprehensive MVP review (2026-01-26):

| Category | Rating | Notes |
|----------|--------|-------|
| Documentation Currency | 7/10 | README.md and TROUBLESHOOTING.md updated |
| ADR Compliance | 71% | 5 fully implemented, 2 partial, 2 not implemented |
| Code Quality | 8/10 | SSH helpers extracted, deduplication complete |
| Security | 7/10 | Vulnerabilities fixed, CI scanning added |
| Test Coverage | 6.5/10 | CLI tests at 19%, full coverage needs mock infrastructure |
| CI/CD Maturity | 9/10 | govulncheck and gosec added |

### Key Gaps

1. **Security:** SSH host key verification falls back to `InsecureIgnoreHostKey()` (gosec G106)
2. **Features:** VM Resize, Dynamic IP Firewall, VM Metadata not implemented
3. **Multi-Cloud:** AWS and Azure interfaces designed but not implemented
4. **API Keys:** No OAuth, Secret Manager, or SSH forwarding for agent authentication
5. **Testing:** CLI 19%, SSH 5.5%, TUI 20.8% coverage

### Technical Debt

| Item | Location | Impact |
|------|----------|--------|
| Large `tui/app.go` | ~1000 LOC | Maintainability |
| `Update()` function | Lines 417-615 | 198 lines, ~30 branches |
| Low test coverage | CLI, SSH, TUI packages | Quality risk |

### Open Issues

| Issue ID | Title | Priority | Phase |
|----------|-------|----------|-------|
| cc-xzi | Improve SSH host key management UX | P2 | 1 |
| cc-s7h | Add cloudcoop metadata to created VMs | P3 | 2 |
| cc-l5v | Review ADR-0003: disk auto-delete behavior | P3 | 2 |
| cc-3.1 | Terminal config generator (Ghostty/iTerm2/Kitty) | P4 | 4 |
| cc-atw | Support terminals other than Ghostty | P4 | 4 |

---

## Phase 1: Security & Quality Foundation

**Objective:** Establish a secure, maintainable codebase before adding new features.

### 1.1 SSH Host Key Management (cc-xzi)

**Current Problem:**

```go
// internal/ssh/client.go:121-128
func loadKnownHostsOrInsecure() ssh.HostKeyCallback {
    if cb, err := knownhosts.New(...); err == nil {
        return cb
    }
    return ssh.InsecureIgnoreHostKey()  // ⚠️ MITM vulnerability
}
```

**Solution Approach:**

1. **Never fall back to InsecureIgnoreHostKey** - Require valid host key verification
2. **Interactive TOFU (Trust On First Use)** - Prompt user to accept new host keys
3. **Automatic key learning** - On first connection, learn and store the host key
4. **Clear error messaging** - Explain when host key changes (potential MITM)

**Implementation:**

```go
type HostKeyAction int
const (
    HostKeyAccept HostKeyAction = iota  // First connection - learn key
    HostKeyReject                        // Key mismatch - abort
    HostKeyPrompt                        // Ask user to confirm
)

func verifyHostKey(hostname string, remote net.Addr, key ssh.PublicKey) error {
    known, err := loadKnownHosts()
    if err != nil {
        return fmt.Errorf("cannot load known_hosts: %w", err)
    }

    switch known.Check(hostname, remote, key) {
    case knownhosts.KeyOK:
        return nil
    case knownhosts.KeyChanged:
        return fmt.Errorf("HOST KEY CHANGED for %s - possible MITM attack", hostname)
    case knownhosts.KeyNotFound:
        return promptAndAddKey(hostname, key)
    }
}
```

**Deliverables:**

- [ ] Remove `InsecureIgnoreHostKey()` fallback
- [ ] Implement TOFU pattern with user confirmation
- [ ] Add `~/.ssh/known_hosts` management
- [ ] Display key fingerprint for verification
- [ ] Provide `--accept-host-key` flag for automation

**Success Criteria:**

- gosec G106 finding eliminated
- New VM connections prompt for key confirmation
- Changed host keys abort with clear error message
- Existing valid host keys work without prompts

### 1.2 Refactor tui/app.go

**Current State:** ~1000 LOC single file with large `Update()` function.

**Target Structure:**

```text
internal/tui/
├── app.go          # Main model, Init(), simplified Update()
├── commands.go     # Command generation (tea.Cmd functions)
├── handlers.go     # Message handlers extracted from Update()
├── view.go         # View() and rendering helpers
└── styles.go       # Lipgloss style definitions (if needed)
```

**Deliverables:**

- [ ] Extract keyboard handlers to `handlers.go`
- [ ] Extract command builders to `commands.go`
- [ ] Move view logic to `view.go`
- [ ] Reduce `Update()` to dispatcher pattern (~50 lines)
- [ ] Maintain full test coverage through refactor

**Success Criteria:**

- No file exceeds 300 LOC
- `Update()` function under 100 lines
- All existing TUI tests pass
- No functional changes to user experience

### 1.3 Improve Test Coverage

**Current Coverage Gaps:**

| Package | Current | Target | Gap |
|---------|---------|--------|-----|
| CLI | 19.2% | 60% | +41% |
| SSH | 5.5% | 60% | +55% |
| TUI | 20.8% | 50% | +30% |

**Approach:**

1. **CLI Testing** - Add mock cloud provider for command testing
2. **SSH Testing** - Extend MockSSHClient for additional scenarios
3. **TUI Testing** - Add tea.Model state machine tests

**Priority Test Scenarios:**

| Package | Scenario | Priority |
|---------|----------|----------|
| SSH | Connection failure handling | High |
| SSH | Host key verification | High |
| CLI | `cloudcoop start` happy path | High |
| CLI | `cloudcoop status` error handling | High |
| TUI | Keyboard navigation | Medium |
| TUI | Confirmation dialogs | Medium |

**Deliverables:**

- [ ] CLI package mock infrastructure
- [ ] SSH connection tests with mock server
- [ ] TUI state transition tests
- [ ] Integration test for full workflow

**Success Criteria:**

- CLI coverage ≥60%
- SSH coverage ≥60%
- TUI coverage ≥50%
- No reduction in existing coverage

---

## Phase 2: GCP Feature Completion

**Objective:** Implement documented features in TUI-REQUIREMENTS.md that are not yet built.

### 2.1 VM Resize

**Reference:** TUI-REQUIREMENTS.md Section 2.4

**Requirements:**

- Display current and available machine types
- Require VM stopped before resize
- Offer to stop running VM
- Update machine type via `SetMachineType` API

**UI Design:**

```text
Current: c4a-highcpu-16 (16 vCPU, 32GB) - ~$0.12/hr spot
Available sizes:
  [1] Small   c4a-highcpu-4   ( 4 vCPU,  8GB) - ~$0.03/hr spot
  [2] Medium  c4a-highcpu-8   ( 8 vCPU, 16GB) - ~$0.06/hr spot
  [3] Large   c4a-highcpu-16  (16 vCPU, 32GB) - ~$0.12/hr spot  ← current
  [4] XLarge  c4a-highcpu-32  (32 vCPU, 64GB) - ~$0.24/hr spot

Select size [1-4] or Esc to cancel:
```

**Implementation:**

```go
// internal/cloud/gcp/provider.go
func (p *GCPProvider) ResizeVM(ctx context.Context, name, machineType string) error {
    op, err := p.instancesClient.SetMachineType(ctx, &computepb.SetMachineTypeInstanceRequest{
        Project:  p.project,
        Zone:     p.zone,
        Instance: name,
        InstancesSetMachineTypeRequestResource: &computepb.InstancesSetMachineTypeRequest{
            MachineType: proto.String(fmt.Sprintf("zones/%s/machineTypes/%s", p.zone, machineType)),
        },
    })
    if err != nil {
        return fmt.Errorf("set machine type: %w", err)
    }
    return op.Wait(ctx)
}
```

**Deliverables:**

- [ ] Add `ResizeVM` to cloud provider interface
- [ ] Implement GCP `SetMachineType` call
- [ ] Add resize TUI screen with size picker
- [ ] Add `cloudcoop resize` CLI command
- [ ] Add keyboard shortcut [R] for resize

**Success Criteria:**

- Resize works for stopped VMs
- Error message if VM is running
- User can select from configured sizes
- README.md updated with resize feature

### 2.2 Dynamic IP Firewall

**Reference:** ADR-0012 (Network Security for SSH Access)

**Modes:**

| Mode | Behavior |
|------|----------|
| `iap` | Use IAP tunnel (no firewall needed) |
| `auto` | Detect public IP, update firewall rule |
| `manual` | User specifies IP/CIDR in config |
| `disabled` | No firewall management |

**Implementation:**

```go
// internal/cloud/gcp/firewall.go
func (p *GCPProvider) UpdateSSHFirewall(ctx context.Context, sourceRanges []string) error {
    rule := &computepb.Firewall{
        Name:         proto.String("cloudcoop-ssh-access"),
        Network:      proto.String("global/networks/default"),
        Allowed:      []*computepb.Allowed{{IPProtocol: proto.String("tcp"), Ports: []string{"22"}}},
        SourceRanges: sourceRanges,
        TargetTags:   []string{"cloudcoop-ssh"},
    }
    // Update or create firewall rule
}
```

**Deliverables:**

- [ ] Add `[network]` configuration section
- [ ] Implement IP detection for `auto` mode
- [ ] Add firewall rule create/update logic
- [ ] Display current firewall status in TUI
- [ ] Add `cloudcoop firewall` CLI command

**Success Criteria:**

- `auto` mode updates firewall on startup
- IAP mode works without external IP
- `manual` mode applies configured ranges
- Status view shows firewall state

### 2.3 VM Metadata Tagging (cc-s7h)

**Purpose:** Tag VMs with cloudcoop metadata for identification and diagnostics.

**Metadata Fields:**

| Key | Value | Purpose |
|-----|-------|---------|
| `cloudcoop-version` | `0.2.0` | Track which version created VM |
| `cloudcoop-created` | ISO timestamp | Creation time |
| `cloudcoop-config-hash` | SHA256[:8] | Detect config drift |

**Implementation:**

```go
// During VM creation
metadata := &computepb.Metadata{
    Items: []*computepb.Items{
        {Key: proto.String("cloudcoop-version"), Value: proto.String(version.String())},
        {Key: proto.String("cloudcoop-created"), Value: proto.String(time.Now().Format(time.RFC3339))},
        {Key: proto.String("cloudcoop-config-hash"), Value: proto.String(configHash[:8])},
    },
}
```

**Deliverables:**

- [ ] Add metadata on VM creation
- [ ] Display metadata in status view
- [ ] Warn if VM version differs from CLI version
- [ ] Document metadata fields in CLAUDE.md

**Success Criteria:**

- New VMs have cloudcoop metadata
- Status shows cloudcoop version mismatch warnings
- Config hash helps detect drift

### 2.4 Disk Auto-Delete Review (cc-l5v)

**Context:** ADR-0003 specifies disk auto-delete behavior on VM deletion.

**Review Items:**

- [ ] Verify current `AutoDelete` setting matches ADR-0003
- [ ] Document implications in TUI delete confirmation
- [ ] Add warning about data loss in delete dialog
- [ ] Consider adding disk snapshot option before delete

---

## Phase 3: Multi-Cloud Expansion

**Objective:** Implement AWS and Azure providers per ADR-0010.

### Effort Estimate (from ADR-0010)

| Component | AWS | Azure |
|-----------|-----|-------|
| VM lifecycle | ~2 days | ~2 days |
| Machine type mapping | ~1 day | ~1 day |
| IAM setup docs | ~2 days | ~2 days |
| Secret management | ~1 day | ~1 day |
| Provisioning script | ~3 days | ~3 days |
| Testing | ~2 days | ~2 days |
| **Total** | ~11 days | ~11 days |

### 3.1 AWS Provider

**Service Mapping:**

| Capability | GCP | AWS |
|------------|-----|-----|
| VM Service | Compute Engine | EC2 |
| Spot Instances | Spot VMs | Spot Instances |
| ARM Instances | C4A (Axion) | Graviton (c7g/c8g) |
| IAM | Service Accounts | IAM Roles + Instance Profiles |
| Secrets | Secret Manager | Secrets Manager |

**Implementation Files:**

```text
internal/cloud/aws/
├── provider.go      # AWSProvider implementation
├── ec2.go           # VM lifecycle operations
├── types.go         # Machine type mapping
└── secrets.go       # Secrets Manager integration
```

**Key Implementation:**

```go
type AWSProvider struct {
    region     string
    client     *ec2.Client
    instanceID string  // Cached after lookup by Name tag
}

func (p *AWSProvider) StartVM(ctx context.Context, name string) error {
    instanceID, err := p.getInstanceIDByName(ctx, name)
    if err != nil {
        return err
    }
    _, err = p.client.StartInstances(ctx, &ec2.StartInstancesInput{
        InstanceIds: []string{instanceID},
    })
    return err
}
```

**Deliverables:**

- [ ] Implement full provider interface for AWS
- [ ] Add AWS configuration section to TOML
- [ ] Create AWS provisioning documentation
- [ ] Add AWS-specific machine type mappings
- [ ] Integration tests with localstack

### 3.2 Azure Provider

**Service Mapping:**

| Capability | GCP | Azure |
|------------|-----|-------|
| VM Service | Compute Engine | Virtual Machines |
| Spot Instances | Spot VMs | Spot VMs |
| ARM Instances | C4A (Axion) | Cobalt (Dpsv6) |
| IAM | Service Accounts | Managed Identities |
| Secrets | Secret Manager | Key Vault |

**Implementation Files:**

```text
internal/cloud/azure/
├── provider.go      # AzureProvider implementation
├── vm.go            # VM lifecycle operations
├── types.go         # Machine type mapping
└── keyvault.go      # Key Vault integration
```

**Key Implementation:**

```go
type AzureProvider struct {
    subscription  string
    resourceGroup string
    client        *armcompute.VirtualMachinesClient
}

func (p *AzureProvider) StartVM(ctx context.Context, name string) error {
    poller, err := p.client.BeginStart(ctx, p.resourceGroup, name, nil)
    if err != nil {
        return fmt.Errorf("begin start: %w", err)
    }
    _, err = poller.PollUntilDone(ctx, nil)
    return err
}
```

**Deliverables:**

- [ ] Implement full provider interface for Azure
- [ ] Add Azure configuration section to TOML
- [ ] Create Azure provisioning documentation
- [ ] Add Azure-specific machine type mappings
- [ ] Integration tests with Azurite (where applicable)

### 3.3 Machine Type Normalization

**Standard Types:**

| Normalized | GCP | AWS | Azure |
|------------|-----|-----|-------|
| arm-4cpu-8gb | c4a-highcpu-4 | c7g.xlarge | Standard_D4pds_v6 |
| arm-8cpu-16gb | c4a-highcpu-8 | c7g.2xlarge | Standard_D8pds_v6 |
| arm-16cpu-32gb | c4a-highcpu-16 | c7g.4xlarge | Standard_D16pds_v6 |
| arm-32cpu-64gb | c4a-highcpu-32 | c7g.8xlarge | Standard_D32pds_v6 |

**Configuration:**

```toml
[machine_types.arm-16cpu-32gb]
gcp = "c4a-highcpu-16"
aws = "c7g.4xlarge"
azure = "Standard_D16pds_v6"
```

---

## Phase 4: Advanced Features

**Objective:** Add sophisticated capabilities for power users.

### 4.1 API Key Management (ADR-0009)

**Tiered Approach:**

1. **OAuth browser flow** (preferred) - for agents that support it
2. **GCP Secret Manager** (alternative) - for headless/automated scenarios
3. **SSH environment forwarding** (fallback) - keys stay on local machine

**TUI Integration:**

```text
┌─────────────────────────────────────────────────────────────────┐
│  Authentication Status                                          │
│  ────────────────────                                           │
│  Claude Code:    ✓ Authenticated (OAuth, expires in 29 days)   │
│  Aider:          ✓ Secret Manager (anthropic-api-key)          │
│  Gemini CLI:     ✓ Service Account (workload identity)         │
│  GitHub Copilot: ✗ Not authenticated [A]uthenticate            │
└─────────────────────────────────────────────────────────────────┘
```

**Configuration Extension:**

```toml
[agents.claude.auth]
method = "oauth"
command = "claude auth login"
check = "claude auth status"

[agents.aider.auth]
method = "secret_manager"
[agents.aider.auth.secrets]
ANTHROPIC_API_KEY = "anthropic-api-key"
```

**Deliverables:**

- [ ] OAuth flow with SSH tunnel helper
- [ ] Secret Manager integration
- [ ] Auth status display in TUI
- [ ] Per-agent auth configuration

### 4.2 Session Recovery After Preemption

**Reference:** TUI-REQUIREMENTS.md Section 6.2

**Approach:**

1. Periodic state snapshots (every 5 minutes)
2. Detect previous session on VM restart
3. Offer to restore agent configuration
4. Start agents in "continue" mode

**State Captured:**

- Active tmux windows and names
- Agent process status (active/idle)
- Working directory per agent
- Last activity timestamp

**Recovery UI:**

```text
Previous session detected (stopped 15 minutes ago):
  - 12 agents were running
  - Last snapshot: 2026-01-27 10:45:00

  [Restore Previous Session]  [Start Fresh]
```

**Deliverables:**

- [ ] Implement periodic state capture
- [ ] Store state on VM boot disk
- [ ] Add recovery detection on startup
- [ ] Implement restore workflow

### 4.3 Bulk Agent Operations

**Reference:** TUI-REQUIREMENTS.md Section 3.3

**UI Design:**

```text
Start agents:
  Agent count: [12]
  Agent type:  ● Claude  ○ Aider  ○ Gemini
  Session mode: ○ Fresh  ● Continue  ○ Pick

  [Start All]  [Cancel]
```

**Deliverables:**

- [ ] Add bulk start command
- [ ] Add bulk stop (kill session) command
- [ ] Add keyboard shortcut [B] for bulk operations
- [ ] Add `cloudcoop agents start --count=N` CLI

### 4.4 Agent Logs Viewing

**Reference:** TUI-REQUIREMENTS.md Section 3.6

**Modes:**

- **Follow** - Live streaming of agent output
- **Historical** - Last N lines from tmux capture

**Implementation:**

```go
// Follow mode
sshClient.Run(fmt.Sprintf(`tmux capture-pane -t agents:%s -p -S -100`, index))
```

**Deliverables:**

- [ ] Add log viewing TUI screen
- [ ] Add follow mode with streaming
- [ ] Add keyboard shortcut [L] for logs
- [ ] Add `cloudcoop logs <agent>` CLI command

### 4.5 Terminal Config Generator (cc-3.1, cc-atw)

**Reference:** TUI-REQUIREMENTS.md Section 7

**Supported Terminals:**

| Terminal | Config Format | Status |
|----------|---------------|--------|
| Ghostty | TOML | Primary |
| iTerm2 | AppleScript/Profiles | Planned |
| Kitty | kitty.conf | Planned |

**Generated Config Example (Ghostty):**

```toml
# ~/.config/ghostty/cloudcoop-agents.toml
# Auto-generated by cloudcoop

[[window.split]]
command = "ssh claude-sandbox -t 'tmux attach -t agents:1'"

[[window.split]]
command = "ssh claude-sandbox -t 'tmux attach -t agents:2'"
```

**Deliverables:**

- [ ] Ghostty config generator
- [ ] iTerm2 config generator
- [ ] Kitty config generator
- [ ] CLI command `cloudcoop terminal-config`

---

## Risks and Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| SSH host key changes break automation | Medium | High | Provide `--accept-host-key` flag for CI/CD |
| Multi-cloud testing infrastructure | Medium | Medium | Use localstack (AWS) and mocks for initial testing |
| Azure ARM instance availability | Low | Medium | Fall back to x86 instances if needed |
| OAuth flow browser access from VM | Medium | Low | SSH tunnel helper script provided |
| Spot preemption during state save | Low | Medium | Atomic writes, keep 3 snapshots |

---

## Success Metrics

### Phase 1 Success

- [ ] Zero gosec G106 findings (InsecureIgnoreHostKey)
- [ ] tui/app.go under 300 LOC
- [ ] CLI coverage ≥60%, SSH coverage ≥60%, TUI coverage ≥50%
- [ ] All existing tests pass

### Phase 2 Success

- [ ] VM resize works from TUI and CLI
- [ ] Dynamic IP firewall auto-updates on startup
- [ ] New VMs have cloudcoop metadata
- [ ] All TUI-REQUIREMENTS.md GCP features implemented

### Phase 3 Success

- [ ] `cloudcoop --cloud aws` works end-to-end
- [ ] `cloudcoop --cloud azure` works end-to-end
- [ ] Machine type normalization works across clouds
- [ ] Documentation covers all three clouds

### Phase 4 Success

- [ ] OAuth flow works for Claude Code authentication
- [ ] Session recovery works after spot preemption
- [ ] Bulk operations start/stop 12 agents efficiently
- [ ] Terminal configs generate for Ghostty, iTerm2, Kitty

---

## Appendix: Critical Files by Phase

### Phase 1

| File | Purpose |
|------|---------|
| `internal/ssh/client.go` | Host key verification fix |
| `internal/ssh/host_keys.go` | New file for TOFU logic |
| `internal/tui/app.go` | Refactor into smaller files |
| `internal/tui/handlers.go` | Extracted message handlers |
| `internal/tui/commands.go` | Extracted command builders |
| `internal/cli/*_test.go` | New CLI tests |

### Phase 2

| File | Purpose |
|------|---------|
| `internal/cloud/interface.go` | Add ResizeVM, Firewall methods |
| `internal/cloud/gcp/resize.go` | VM resize implementation |
| `internal/cloud/gcp/firewall.go` | Firewall management |
| `internal/tui/resize.go` | Resize screen |
| `internal/cli/resize_cmd.go` | Resize CLI command |

### Phase 3

| File | Purpose |
|------|---------|
| `internal/cloud/aws/provider.go` | AWS provider |
| `internal/cloud/aws/ec2.go` | EC2 operations |
| `internal/cloud/azure/provider.go` | Azure provider |
| `internal/cloud/azure/vm.go` | Azure VM operations |
| `internal/cloud/types.go` | Machine type mappings |

### Phase 4

| File | Purpose |
|------|---------|
| `internal/auth/oauth.go` | OAuth browser flow |
| `internal/auth/secrets.go` | Secret Manager integration |
| `internal/session/snapshot.go` | State capture |
| `internal/session/recovery.go` | Session restore |
| `internal/tui/bulk.go` | Bulk operations screen |
| `internal/terminal/generator.go` | Config generators |

---

## References

- [TUI-REQUIREMENTS.md](TUI-REQUIREMENTS.md) - Full TUI specification
- [MVP-REVIEW-REPORT.md](MVP-REVIEW-REPORT.md) - Current state assessment
- [ADR-0009](../decisions/0009-api-key-management.md) - API Key Management
- [ADR-0010](../decisions/0010-cloud-agnostic-design.md) - Cloud-Agnostic Design
- [ADR-0012](../decisions/0012-dynamic-ip-firewall.md) - Dynamic IP Firewall
- [Beads Issues](../.beads/) - cc-xzi, cc-s7h, cc-l5v, cc-3.1, cc-atw
