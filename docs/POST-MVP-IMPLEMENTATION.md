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

1. **Server Build Complete** - Integrate VM provisioning and agent setup (SETUP-FLOW Stage 5-6)
2. **Security Foundation** - Address SSH host key verification and code quality
3. **GCP Feature Complete** - Finish resize, firewall, metadata features
4. **Multi-Cloud & Advanced** - AWS/Azure providers, terminal config generator, session recovery

**Timeline Overview:**

| Phase | Focus | Complexity |
|-------|-------|------------|
| 1 | VM Provisioning & Agent Setup | 3-4 weeks |
| 2 | Security & Quality Foundation | 2-3 weeks |
| 3 | GCP Feature Completion | 4-6 weeks |
| 4 | Multi-Cloud & Advanced Features | 10-14 weeks |

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

1. **VM Provisioning:** SETUP-FLOW.md Stage 5 (install tools, agents) not integrated into TUI
2. **Agent Authentication:** SETUP-FLOW.md Stage 6 (OAuth for Claude Code) not implemented
3. **Agent Management Scripts:** provision-vm.sh has scripts but no TUI/CLI integration
4. **Terminal Config:** Multi-session Ghostty/iTerm2 views not implemented (cc-3.1)
5. **Security:** SSH host key verification falls back to `InsecureIgnoreHostKey()` (gosec G106)
6. **Features:** VM Resize, Dynamic IP Firewall, VM Metadata not implemented
7. **Multi-Cloud:** AWS and Azure interfaces designed but not implemented

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

## Phase 1: VM Provisioning & Agent Setup

**Objective:** Complete the end-to-end VM setup experience documented in SETUP-FLOW.md.

The current MVP can create VMs and manage their lifecycle, but the server-side setup is manual.
The vision in SETUP-FLOW.md describes an automated provisioning flow that installs all development
tooling and configures agents. The provision-vm.sh script already exists but isn't integrated.

### 1.1 Integrate VM Provisioning Script

**Current State:**

- `scripts/provision-vm.sh` is a comprehensive 650-line script that installs:
  - Development languages: Node.js, Go, Python, Rust, Java, Ruby, PHP, .NET
  - Claude Code CLI (`@anthropic-ai/claude-code`)
  - Linting tools: golangci-lint, eslint, prettier, ruff, etc.
  - Container tools: Docker, crane, dive, trivy
  - Cloud CLIs: gcloud, aws, az
  - Database clients: PostgreSQL, MySQL, Redis, MongoDB
  - Agent management scripts: `start-agents.sh`, `stop-agents.sh`, etc.

**Missing:** The script exists but isn't invoked by cloudcoop.

**Implementation:**

1. **Startup Script Metadata** - Pass provision-vm.sh via GCP startup-script metadata:

```go
// During VM creation
metadata := &computepb.Metadata{
    Items: []*computepb.Items{
        {Key: proto.String("startup-script"), Value: proto.String(provisionScript)},
    },
}
```

1. **Provisioning Status Display** - Show progress in TUI:

```text
┌─────────────────────────────────────────────────────────────────┐
│  Provisioning VM (first run takes 5-10 minutes)                 │
├─────────────────────────────────────────────────────────────────┤
│  [✓] System packages                                            │
│  [✓] Docker                                                     │
│  [✓] Node.js 24                                                 │
│  [░░░░░░░░░░] Claude Code CLI...                               │
│  [ ] tmux configuration                                         │
│  [ ] Workspace directories                                      │
└─────────────────────────────────────────────────────────────────┘
```

1. **Provisioning Verification** - SSH check for installed tools:

```go
func (p *Provisioner) CheckStatus(ctx context.Context) (ProvisionStatus, error) {
    // Check if Claude Code is installed
    _, err := p.ssh.Run("claude --version")
    if err != nil {
        return ProvisionStatusIncomplete, nil
    }
    return ProvisionStatusComplete, nil
}
```

**Deliverables:**

- [ ] Embed provision-vm.sh in binary or fetch from config
- [ ] Pass startup-script metadata during VM creation
- [ ] Add provisioning status check to TUI
- [ ] Show provisioning progress on first connection
- [ ] Add `cloudcoop provision` CLI command for re-provisioning

**Success Criteria:**

- New VM is fully provisioned automatically
- TUI shows provisioning status
- `claude --version` works on fresh VM

### 1.2 Agent Authentication (SETUP-FLOW Stage 6)

**Reference:** SETUP-FLOW.md Stage 6, ADR-0009

The vision shows interactive agent authentication after provisioning:

```text
┌─────────────────────────────────────────────────────────────────┐
│  Authenticate Claude Code:                                      │
│                                                                 │
│  Opening browser for authentication...                          │
│  If browser doesn't open, visit:                                │
│  https://console.anthropic.com/auth?callback=...               │
│                                                                 │
│  [Waiting for authentication...]                                │
└─────────────────────────────────────────────────────────────────┘
```

**Implementation:**

1. **SSH Tunnel for OAuth** - Forward localhost for OAuth callbacks:

```bash
ssh -L 8080:localhost:8080 claude-sandbox -t "claude auth login"
```

1. **Auth Status Check** - Verify authentication on connection:

```go
func (a *AuthChecker) CheckClaudeAuth(ctx context.Context) (AuthStatus, error) {
    output, err := a.ssh.Run("claude auth status")
    // Parse output for authentication state
}
```

**Deliverables:**

- [ ] Add SSH tunnel helper for OAuth flows
- [ ] Check agent auth status on VM start
- [ ] Show auth status in TUI infrastructure section
- [ ] Add `cloudcoop auth` CLI command
- [ ] Support re-authentication flow

### 1.3 Agent Management Script Integration

**Current State:**

provision-vm.sh creates these scripts on the VM:

- `start-agents.sh [count]` - Start N Claude agents in tmux
- `stop-agents.sh` - Kill all agent sessions
- `attach-agent.sh [number]` - Attach to specific agent
- `list-agents.sh` - List running agents

**Missing:** TUI doesn't invoke these scripts.

**Integration:**

```go
// internal/agent/manager.go
func (m *Manager) StartAgents(ctx context.Context, count int) error {
    return m.ssh.Run(fmt.Sprintf("start-agents.sh %d", count))
}

func (m *Manager) StopAllAgents(ctx context.Context) error {
    return m.ssh.Run("stop-agents.sh")
}
```

**TUI Integration:**

```text
┌─────────────────────────────────────────────────────────────────┐
│  Start Agents                                                   │
├─────────────────────────────────────────────────────────────────┤
│  How many agents? [12]                                          │
│                                                                 │
│  Agent type: ● Claude Code  ○ Aider  ○ Gemini CLI              │
│                                                                 │
│  [Start]  [Cancel]                                              │
└─────────────────────────────────────────────────────────────────┘
```

**Deliverables:**

- [ ] Invoke start-agents.sh from TUI [A]dd action
- [ ] Invoke stop-agents.sh from TUI bulk stop
- [ ] Show agent count in status view
- [ ] Add `cloudcoop agents start --count=N` CLI

### 1.4 Terminal Config Generator (cc-3.1)

**Reference:** TUI-REQUIREMENTS.md Section 7

Generate terminal emulator configs for viewing multiple agent sessions simultaneously.

**Ghostty Config:**

```toml
# ~/.config/ghostty/cloudcoop-12-agents.toml
# Generated by: cloudcoop terminal-config --agents=12

[window]
title = "cloudcoop agents"

# 3x4 grid of agent views
[[window.split]]
command = "ssh claude-sandbox -t 'tmux attach -t agents:agent-1'"

[[window.split]]
command = "ssh claude-sandbox -t 'tmux attach -t agents:agent-2'"
# ... repeat for all agents
```

**iTerm2 AppleScript:**

```applescript
tell application "iTerm2"
    tell current session of current tab of current window
        split horizontally with default profile command "ssh claude-sandbox -t 'tmux attach -t agents:agent-1'"
    end tell
end tell
```

**Deliverables:**

- [ ] Add `cloudcoop terminal-config` command
- [ ] Generate Ghostty config (TOML)
- [ ] Generate iTerm2 profile/AppleScript
- [ ] Generate Kitty config
- [ ] Support configurable grid layouts (2x2, 3x4, 4x4)

**Success Criteria:**

- `cloudcoop terminal-config --terminal=ghostty --agents=12` generates working config
- User can open terminal with multi-pane agent view

---

## Phase 2: Security & Quality Foundation

**Objective:** Establish a secure, maintainable codebase before adding new features.

### 2.1 SSH Host Key Management (cc-xzi)

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

### 2.2 Refactor tui/app.go

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

### 2.3 Improve Test Coverage

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

## Phase 3: GCP Feature Completion

**Objective:** Implement documented features in TUI-REQUIREMENTS.md that are not yet built.

### 3.1 VM Resize

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

### 3.2 Dynamic IP Firewall

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

### 3.3 VM Metadata Tagging (cc-s7h)

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

### 3.4 Disk Auto-Delete Review (cc-l5v)

**Context:** ADR-0003 specifies disk auto-delete behavior on VM deletion.

**Review Items:**

- [ ] Verify current `AutoDelete` setting matches ADR-0003
- [ ] Document implications in TUI delete confirmation
- [ ] Add warning about data loss in delete dialog
- [ ] Consider adding disk snapshot option before delete

---

## Phase 4: Multi-Cloud & Advanced Features

**Objective:** Implement AWS and Azure providers per ADR-0010, plus advanced capabilities.

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

### 4.1 AWS Provider

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

### 4.2 Azure Provider

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

### 4.3 Machine Type Normalization

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

### 4.4 API Key Management (ADR-0009)

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

### 4.5 Session Recovery After Preemption

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

### 4.6 Bulk Agent Operations

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

### 4.7 Agent Logs Viewing

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

### Phase 1 Success (VM Provisioning & Agent Setup)

- [ ] New VM is automatically provisioned with all dev tools
- [ ] Claude Code authenticates via OAuth tunnel
- [ ] `start-agents.sh 12` invocable from TUI
- [ ] Terminal configs generate for Ghostty, iTerm2, Kitty

### Phase 2 Success (Security & Quality)

- [ ] Zero gosec G106 findings (InsecureIgnoreHostKey)
- [ ] tui/app.go under 300 LOC
- [ ] CLI coverage ≥60%, SSH coverage ≥60%, TUI coverage ≥50%
- [ ] All existing tests pass

### Phase 3 Success (GCP Feature Completion)

- [ ] VM resize works from TUI and CLI
- [ ] Dynamic IP firewall auto-updates on startup
- [ ] New VMs have cloudcoop metadata
- [ ] All TUI-REQUIREMENTS.md GCP features implemented

### Phase 4 Success (Multi-Cloud & Advanced)

- [ ] `cloudcoop --cloud aws` works end-to-end
- [ ] `cloudcoop --cloud azure` works end-to-end
- [ ] OAuth flow works for Claude Code authentication
- [ ] Session recovery works after spot preemption
- [ ] Bulk operations start/stop 12 agents efficiently

---

## Appendix: Critical Files by Phase

### Phase 1 (VM Provisioning & Agent Setup)

| File | Purpose |
|------|---------|
| `scripts/provision-vm.sh` | Existing provisioning script (650 LOC) |
| `config/versions.env` | Tool version configuration |
| `internal/provisioner/provisioner.go` | New provisioning coordinator |
| `internal/provisioner/status.go` | Provisioning status checks |
| `internal/auth/tunnel.go` | SSH tunnel for OAuth |
| `internal/agent/manager.go` | Agent start/stop/list operations |
| `internal/terminal/generator.go` | Terminal config generators |

### Phase 2 (Security & Quality)

| File | Purpose |
|------|---------|
| `internal/ssh/client.go` | Host key verification fix |
| `internal/ssh/host_keys.go` | New file for TOFU logic |
| `internal/tui/app.go` | Refactor into smaller files |
| `internal/tui/handlers.go` | Extracted message handlers |
| `internal/tui/commands.go` | Extracted command builders |
| `internal/cli/*_test.go` | New CLI tests |

### Phase 3 (GCP Feature Completion)

| File | Purpose |
|------|---------|
| `internal/cloud/interface.go` | Add ResizeVM, Firewall methods |
| `internal/cloud/gcp/resize.go` | VM resize implementation |
| `internal/cloud/gcp/firewall.go` | Firewall management |
| `internal/tui/resize.go` | Resize screen |
| `internal/cli/resize_cmd.go` | Resize CLI command |

### Phase 4 (Multi-Cloud & Advanced)

| File | Purpose |
|------|---------|
| `internal/cloud/aws/provider.go` | AWS provider |
| `internal/cloud/aws/ec2.go` | EC2 operations |
| `internal/cloud/azure/provider.go` | Azure provider |
| `internal/cloud/azure/vm.go` | Azure VM operations |
| `internal/cloud/types.go` | Machine type mappings |
| `internal/auth/oauth.go` | OAuth browser flow |
| `internal/auth/secrets.go` | Secret Manager integration |
| `internal/session/snapshot.go` | State capture |
| `internal/session/recovery.go` | Session restore |
| `internal/tui/bulk.go` | Bulk operations screen |

---

## References

- [SETUP-FLOW.md](SETUP-FLOW.md) - First-run setup wizard (Stages 1-6)
- [TUI-REQUIREMENTS.md](TUI-REQUIREMENTS.md) - Full TUI specification
- [MVP-REVIEW-REPORT.md](MVP-REVIEW-REPORT.md) - Current state assessment
- [provision-vm.sh](../scripts/provision-vm.sh) - Existing VM provisioning script (650 LOC)
- [ADR-0003](../decisions/0003-instance-provisioning-model.md) - Instance Provisioning Model
- [ADR-0009](../decisions/0009-api-key-management.md) - API Key Management
- [ADR-0010](../decisions/0010-cloud-agnostic-design.md) - Cloud-Agnostic Design
- [ADR-0012](../decisions/0012-dynamic-ip-firewall.md) - Dynamic IP Firewall
- [Beads Issues](../.beads/) - cc-xzi, cc-s7h, cc-l5v, cc-3.1, cc-atw
