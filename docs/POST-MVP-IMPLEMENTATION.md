# Post-MVP Implementation Approach

**Version:** 2.0
**Date:** 2026-01-28
**Status:** Active

---

## Executive Summary

This document outlines a phased approach for cloudcoop development beyond the MVP. Version 2.0
reflects the architectural redesign captured in ADRs 0022-0027 (accepted 2026-01-28), which
replaced the original Phase 1 plan with a worktree-based multi-repo agent workspace model.

**Strategic Direction:**

1. **Multi-Agent Workspace Foundation** — Implement `agents sync` and `agents attach` (ADRs 0022-0027)
2. **Security Foundation** — Address SSH host key verification and code quality
3. **GCP Feature Complete** — Finish resize, firewall, metadata features
4. **Multi-Cloud & Advanced** — AWS/Azure providers, agent auth, session recovery

**Phase Overview:**

| Phase | Focus | Key ADRs |
|-------|-------|----------|
| 1 | Multi-Agent Workspace Foundation | 0022, 0023, 0024, 0025, 0026, 0027 |
| 2 | Security & Quality Foundation | — |
| 3 | GCP Feature Completion | 0012 |
| 4 | Multi-Cloud & Advanced Features | 0009, 0010 |

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

### The Pivot: ADRs 0022-0027

Six ADRs (accepted 2026-01-28) redesigned the agent workspace and session model:

| Aspect | Old Model | New Model (ADRs 0022-0027) |
|--------|-----------|---------------------------|
| Workspaces | Flat `/workspaces/agent-N` | Git worktrees: `/workspaces/<slug>/<branch>` |
| tmux sessions | Single `"agents"` session | Per-repo sessions: `acme-backend`, `acme-frontend` |
| Setup command | `provision-vm.sh` scripts | `cloudcoop agents sync` (clone-on-demand) |
| Agent start | `start-agents.sh [count]` | Auto-start per worktree during sync |
| Agent attach | `cloudcoop connect <index>` | `cloudcoop agents attach --next` (grouped sessions) |
| Git access | Not addressed | Deploy keys via `gh` CLI (ADR-0026) |
| Agent config | Hardcoded `claude` | Configurable per-repo commands + pre-hooks (ADR-0027) |
| Multi-repo | Not supported | First-class: one tmux session per repo |

### Technical Debt

| Item | Location | Impact |
|------|----------|--------|
| Hardcoded `"agents"` session | agent.go, connect.go, terminal/*.go | Blocks multi-repo |
| `InsecureIgnoreHostKey()` fallback | ssh/client.go | Security risk (gosec G106) |
| Low test coverage | CLI, SSH, TUI packages | Quality risk |

---

## Phase 1: Multi-Agent Workspace Foundation

**Objective:** Implement the core `agents sync` and `agents attach` workflow from ADRs 0022-0027.
This is the primary user-facing feature that makes cloudcoop useful for real multi-agent development.

**Acceptance Criteria:** User can run `cloudcoop agents sync` from a repo with worktrees, have
agents automatically started on the VM, and attach to them via terminal splits.

**Beads Epic:** cc-m9b

**Prerequisites:** Phase 1 assumes VM provisioning (Stages 1-5 of SETUP-FLOW.md) is
already complete — the VM exists, has development tools installed, and SSH access works.
Agent authentication (Stage 6) is deferred to Phase 4 (cc-cz1). Provisioning is handled
by the existing `provision-vm.sh` startup script and `cloudcoop provision` commands.

### 1.1 Repo Slug & Worktree Detection (cc-b47)

**ADRs:** 0022, 0023
**New package:** `internal/workspace/`

- Detect git remote URL from cwd (`git remote get-url origin`)
- Derive repo slug from URL (handle SSH and HTTPS formats)
- List local worktrees via `git worktree list --porcelain`
- Parse worktree output into structured data (path, branch, commit)
- Handle edge cases: not a git repo, no remote, bare repo

### 1.2 Repo-Scoped tmux Sessions (cc-xmd)

**ADR:** 0023
**Depends on:** 1.1

Files that currently hardcode `"agents"` session name:

- `internal/agent/agent.go` — all tmux commands
- `internal/ssh/connect.go` — tmux attach
- `internal/terminal/ghostty.go`, `kitty.go`, `iterm2.go` — attach commands

Changes:

- Replace all hardcoded `"agents"` with dynamic session name parameter
- Session name = repo slug (e.g., `acme-backend`)
- Update `ListSessions()`, `CreateSession()`, `KillSession()` to accept session name
- Add `ListAllSessions()` for `--all` flag
- Update all existing tests

### 1.3 Deploy Key Management (cc-5bo)

**ADR:** 0026
**Depends on:** 1.1
**New package:** `internal/deploykey/`

- Check for existing key at `~/.ssh/cloudcoop-deploy-<slug>`
- Generate ed25519 key pair if missing
- Register on GitHub via `gh api repos/{owner}/{repo}/keys`
- Detect `gh` availability; fallback to manual instructions
- Copy private key to VM via SCP
- Write SSH config entry on VM (handle multi-repo host aliases)
- Pre-flight check: `git ls-remote` on VM

### 1.4 Clone-on-Demand Sync (cc-gov)

**ADR:** 0024
**Depends on:** 1.1, 1.2, 1.3

- Implement `cloudcoop agents sync` command
- Read local worktree state
- SSH to VM, check for existing bare clone at `/repos/<slug>.git`
- Clone bare repo if missing: `git clone --bare <url> /repos/<slug>.git`
- Fetch latest: `git -C /repos/<slug>.git fetch --all --prune`
- Create worktrees: `git worktree add /workspaces/<slug>/<name> <branch>`
- Skip existing worktrees (idempotent)
- Detect removed worktrees (exist on VM but not locally), prompt for cleanup
- Start tmux session with one window per worktree

### 1.5 Agent Startup Hooks (cc-mya)

**ADR:** 0027
**Depends on:** 1.4

Extend config schema with `[agents]` section:

- `default_command` (default: `"claude"`)
- `pre_commands` (list of strings)
- `[agents.repos.<slug>]` overrides

Build tmux window command: `cd <worktree> && <pre_commands...> && <agent_command>`

Resolution order:

- Agent command: repo-specific > default > `"claude"`
- Pre-commands: global + repo-specific (concatenated)

### 1.6 Terminal-Native Attach (cc-f4g)

**ADR:** 0025
**Depends on:** 1.2 (can run in parallel with 1.5)

- `cloudcoop agents attach --next`:
  - List windows in repo session
  - List clients/grouped sessions to find attached windows
  - Select first unattached window
  - Create grouped session: `tmux new-session -t <slug> -s <slug>-<unique>`
  - Select assigned window
- `cloudcoop agents attach --window <name|index>`
- `cloudcoop agents list` (current repo)
- `cloudcoop agents list --all` (all repos)

### 1.7 Phase 1 Verification (cc-n5j)

**Depends on:** all above

Manual test gate:

- [ ] `cloudcoop agents sync` from repo with 3 worktrees creates VM setup
- [ ] Deploy key generated and registered on GitHub automatically
- [ ] Bare clone + worktrees created on VM
- [ ] 3 tmux windows running configured agent command
- [ ] `cloudcoop agents list` shows 3 agents for current repo
- [ ] `cloudcoop agents attach --next` attaches to next unattached window
- [ ] Second `--next` attaches to a different window
- [ ] Re-running sync is idempotent (no duplicate windows)
- [ ] Adding a local worktree + sync creates new window
- [ ] Multi-repo: sync from second repo creates separate session

### Implementation Order

```text
1.1 (slug/worktree detection)
  → 1.2 (repo-scoped tmux) — can parallel with 1.3
  → 1.3 (deploy keys) — can parallel with 1.2
    → 1.4 (clone-on-demand sync) — depends on 1.1, 1.2, 1.3
      → 1.5 (startup hooks) — depends on 1.4
      → 1.6 (terminal-native attach) — depends on 1.2, can parallel with 1.5
        → 1.7 (verification) — depends on all above
```

### New Packages & Files

| Package | Files | Purpose |
|---------|-------|---------|
| `internal/workspace/` | `workspace.go`, `sync.go`, `workspace_test.go` | Worktree detection, sync logic |
| `internal/deploykey/` | `deploykey.go`, `deploykey_test.go` | Deploy key lifecycle |

### Files to Modify

| File | Change |
|------|--------|
| `internal/agent/agent.go` | Parameterize session name, add grouped session support |
| `internal/ssh/connect.go` | Parameterize session name for attach |
| `internal/terminal/ghostty.go` | Use dynamic session name |
| `internal/terminal/kitty.go` | Use dynamic session name |
| `internal/terminal/iterm2.go` | Use dynamic session name |
| `internal/config/config.go` | Add `[agents]` section with repos/hooks |
| `internal/cli/agents.go` | Add `sync`, `attach --next`, `list --all` subcommands |

---

## Phase 2: Security & Quality Foundation

**Objective:** Establish a secure, maintainable codebase before adding GCP features.

**Beads Epic:** cc-w5l (blocked by Phase 1)

### 2.1 SSH Host Key Management

**Current Problem:**

```go
// internal/ssh/client.go — falls back to InsecureIgnoreHostKey() (gosec G106)
```

**Solution:**

1. Remove `InsecureIgnoreHostKey()` fallback entirely
2. Implement TOFU (Trust On First Use) with user confirmation
3. Cloudcoop-managed known_hosts file
4. `--accept-host-key` flag for automation
5. Clear error on host key change (potential MITM warning)

**Files:** `internal/ssh/client.go`, new `internal/ssh/hostkey.go`

### 2.2 Refactor tui/app.go

**Note:** tui/app.go was already refactored to 117 LOC during MVP. This task verifies the
refactor accounts for repo-scoped sessions from Phase 1.

- Ensure handlers.go, commands.go, view.go work with dynamic session names
- No file exceeds 300 LOC
- All TUI tests pass with repo-scoped session changes

### 2.3 Improve Test Coverage

| Package | Current | Target | Gap |
|---------|---------|--------|-----|
| CLI | 19.2% | 60% | +41% |
| SSH | 5.5% | 60% | +55% |
| TUI | 20.8% | 50% | +30% |

- Add tests for new Phase 1 packages (workspace, deploykey)
- CLI mock provider infrastructure
- SSH mock server tests
- TUI state transition tests

---

## Phase 3: GCP Feature Completion

**Beads Epic:** cc-md9 (blocked by Phase 2)

### 3.1 VM Resize (cc-md9.1)

- Add `ResizeVM` to cloud provider interface
- GCP `SetMachineType` implementation
- TUI resize screen + CLI command + [R] shortcut
- Require VM stopped before resize

### 3.2 Dynamic IP Firewall (cc-md9.2)

- `[network]` config section
- Modes: `iap`, `auto`, `manual`, `disabled`
- IP detection for `auto` mode
- Firewall rule CRUD via GCP SDK
- TUI status display

### 3.3 VM Metadata Tagging

- Add cloudcoop metadata on VM creation (`cloudcoop-version`, `cloudcoop-created`, `cloudcoop-config-hash`)
- Display in status view
- Version mismatch warnings

### 3.4 Disk Auto-Delete Review

- Verify `AutoDelete` setting matches ADR-0003
- Document implications in TUI delete confirmation
- Data loss warning in delete dialog

---

## Phase 4: Multi-Cloud & Advanced Features

**Beads Epic:** cc-af8 (blocked by Phase 3)

### 4.1 AWS Provider (cc-af8.1)

- Implement full provider interface for AWS (EC2 lifecycle, Spot Instances, Graviton)
- AWS configuration section in TOML
- Integration tests with localstack

### 4.2 Azure Provider (cc-af8.2)

- Implement full provider interface for Azure (VM lifecycle, Spot VMs, Cobalt)
- Azure configuration section in TOML
- Integration tests where applicable

### 4.3 Machine Type Normalization (cc-af8.3)

Standard sizes across clouds:

| Normalized | GCP | AWS | Azure |
|------------|-----|-----|-------|
| arm-4cpu-8gb | c4a-highcpu-4 | c7g.xlarge | Standard_D4pds_v6 |
| arm-8cpu-16gb | c4a-highcpu-8 | c7g.2xlarge | Standard_D8pds_v6 |
| arm-16cpu-32gb | c4a-highcpu-16 | c7g.4xlarge | Standard_D16pds_v6 |
| arm-32cpu-64gb | c4a-highcpu-32 | c7g.8xlarge | Standard_D32pds_v6 |

### 4.4 Agent Authentication (cc-cz1)

**Moved from old Phase 1.** Agent auth is now a post-sync concern, not a setup gate.

- OAuth tunnel for Claude Code auth (SSH port forwarding)
- GCP Secret Manager integration for headless API keys
- SSH environment forwarding as fallback
- Per-agent auth status in TUI
- Support multiple agent types (Claude Code, Aider, Gemini CLI)

**References:** ADR-0009, previously cc-nat

### 4.5 Session Recovery After Preemption (cc-af8.5)

- Periodic state snapshots (every 5 minutes)
- Detect previous session on VM restart
- Offer to restore agent configuration
- Start agents in "continue" mode

### 4.6 Bulk Agent Operations (cc-af8.6)

- Bulk start command
- Bulk stop (kill session) command
- [B] keyboard shortcut
- `cloudcoop agents start --count=N` CLI

### 4.7 Agent Logs Viewing (cc-af8.7)

- Log viewing TUI screen
- Follow mode with streaming
- [L] keyboard shortcut
- `cloudcoop logs <agent>` CLI command

**Note:** The old terminal config generator (cc-3.1) is effectively replaced by ADR-0025's
terminal-native splits. Users create splits in their terminal and run `attach --next` in each.
The existing `internal/terminal/` package may still be useful for convenience scripts but is
no longer a Phase 1 deliverable.

---

## Verification Plan

### Phase 1 End-to-End Test

```bash
# Prerequisites: VM running, gh authenticated, local repo with worktrees
cd ~/dev/my-project
git worktree list  # shows 3 worktrees

# Test sync
cloudcoop agents sync
# Expected: deploy key created, bare clone on VM, 3 worktrees, 3 tmux windows

# Test list
cloudcoop agents list
# Expected: 3 agents shown with worktree names

# Test attach
cloudcoop agents attach --next  # attaches to window 0
# In new terminal split:
cloudcoop agents attach --next  # attaches to window 1

# Test idempotency
cloudcoop agents sync  # no changes, existing agents untouched

# Test incremental
git worktree add feature-new
cloudcoop agents sync  # creates 4th worktree + window
```

### Build Verification

```bash
make all  # fmt, lint, test, build — must pass at each phase
```

---

## Success Metrics

### Phase 1 Success (Multi-Agent Workspace Foundation)

- [ ] `cloudcoop agents sync` creates worktree-based workspace on VM
- [ ] Deploy keys auto-generated and registered
- [ ] `cloudcoop agents attach --next` provides terminal-native split workflow
- [ ] Multi-repo support with per-repo tmux sessions
- [ ] Idempotent re-sync

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
- [ ] Bulk operations start/stop agents efficiently

---

## Appendix: Critical Files by Phase

### Phase 1 (Multi-Agent Workspace Foundation)

| File | Purpose |
|------|---------|
| `internal/workspace/workspace.go` | Repo slug detection, worktree parsing |
| `internal/workspace/sync.go` | Clone-on-demand, worktree creation on VM |
| `internal/deploykey/deploykey.go` | Deploy key generation, GitHub registration |
| `internal/agent/agent.go` | Repo-scoped tmux sessions, grouped sessions |
| `internal/ssh/connect.go` | Parameterized session attach |
| `internal/config/config.go` | Agent startup hooks configuration |
| `internal/cli/agents.go` | sync, attach --next, list --all commands |
| `internal/terminal/*.go` | Dynamic session name support |

### Phase 2 (Security & Quality)

| File | Purpose |
|------|---------|
| `internal/ssh/client.go` | Host key verification fix |
| `internal/ssh/hostkey.go` | New TOFU logic |
| `internal/tui/*.go` | Verify repo-scoped session support |
| `internal/cli/*_test.go` | New CLI tests |

### Phase 3 (GCP Feature Completion)

| File | Purpose |
|------|---------|
| `internal/cloud/provider.go` | Add ResizeVM, Firewall methods |
| `internal/cloud/gcp/resize.go` | VM resize implementation |
| `internal/cloud/gcp/firewall.go` | Firewall management |
| `internal/tui/resize.go` | Resize screen |
| `internal/cli/resize_cmd.go` | Resize CLI command |

### Phase 4 (Multi-Cloud & Advanced)

| File | Purpose |
|------|---------|
| `internal/cloud/aws/provider.go` | AWS provider |
| `internal/cloud/azure/provider.go` | Azure provider |
| `internal/cloud/types.go` | Machine type mappings |
| `internal/auth/oauth.go` | OAuth browser flow |
| `internal/auth/secrets.go` | Secret Manager integration |
| `internal/session/snapshot.go` | State capture |
| `internal/session/recovery.go` | Session restore |

---

## References

- [ADR-0022](../decisions/0022-worktree-based-agent-workspaces.md) — Worktree-Based Agent Workspaces
- [ADR-0023](../decisions/0023-repo-scoped-tmux-sessions.md) — Repo-Scoped tmux Sessions
- [ADR-0024](../decisions/0024-clone-on-demand-remote-setup.md) — Clone-on-Demand Remote Setup
- [ADR-0025](../decisions/0025-terminal-native-split-workflow.md) — Terminal-Native Split Workflow
- [ADR-0026](../decisions/0026-vm-git-authentication.md) — VM Git Authentication
- [ADR-0027](../decisions/0027-agent-startup-hooks.md) — Agent Startup Hooks
- [ADR-0009](../decisions/0009-api-key-management.md) — API Key Management
- [ADR-0010](../decisions/0010-cloud-agnostic-design.md) — Cloud-Agnostic Design
- [ADR-0012](../decisions/0012-dynamic-ip-firewall.md) — Dynamic IP Firewall
- [SETUP-FLOW.md](SETUP-FLOW.md) — First-run setup wizard
- [TUI-REQUIREMENTS.md](TUI-REQUIREMENTS.md) — Full TUI specification
- [Beads Issues](../.beads/) — cc-m9b (Phase 1), cc-w5l (Phase 2), cc-md9 (Phase 3), cc-af8 (Phase 4)
