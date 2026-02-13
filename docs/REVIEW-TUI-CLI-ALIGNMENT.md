# Review: TUI and CLI Execution Path Alignment

> Date: 2026-02-13
> Scope: Code duplication and divergence between `internal/tui/` and
> `internal/cli/` execution paths

## Executive Summary

The TUI and CLI both use the same underlying service packages (`cloud`,
`ssh`, `provisioning`, `workspace`, `agent`), which is good. However,
there is **significant duplication at the orchestration layer** -- both
paths independently implement provider creation, SSH connection setup,
firewall/key management, and workspace sync. This creates maintenance
burden and has already led to subtle divergences in behaviour.

The recommended fix is to introduce a thin **service/operations layer**
between the UI code and the low-level packages, so both TUI and CLI
delegate to the same orchestration logic.

---

## 1. Provider Factory Duplication (HIGH)

Two near-identical provider creation functions exist:

| Location | Function |
|----------|----------|
| `tui/commands.go:84-95` | `newProvider()` |
| `cli/status.go:75-91` | `createProvider()` / `createProviderImpl()` |

Both contain the same `switch cfg.Cloud.Provider` logic for GCP
instantiation. When a new provider (AWS, Azure) is added, both must
be updated.

**Recommendation:** Extract to `internal/cloud/factory.go`:

```go
func NewFromConfig(ctx context.Context, cfg *config.Config) (Provider, func(), error)
```

---

## 2. Workspace Sync Duplication (HIGH)

Both `tui/commands.go:320-378` and `cli/agents_sync.go:37-131`
implement the same sync sequence:

1. Connect SSH
2. Set up deploy key
3. Set up git identity
4. Resolve agent command
5. Call `workspace.Sync()`

The TUI and CLI versions are ~85% identical. The only real differences
are:

- TUI receives a pre-detected `workspace.Info`; CLI detects it inline
- TUI returns a message struct; CLI prints results

**Recommendation:** Extract core logic to
`internal/workspace/sync_service.go`:

```go
func RunSync(client ssh.Runner, cfg *config.Config, ws *workspace.Info) (*SyncResult, error)
```

---

## 3. Firewall and SSH Key Setup Duplication (HIGH)

Three separate implementations exist:

| Location | What |
|----------|------|
| `tui/commands.go:402-439` | `ensureFirewall()` (async Cmd) |
| `cli/firewall.go:16-44` | `ensureFirewallAccess()` (inline) |
| `tui/commands.go:380-400` | `ensureSSHKey()` (async Cmd) |
| `cli/ssh_key.go:14-30` | `ensureSSHKeyAccess()` (inline) |

The core logic (resolve IP, call `EnsureFirewallAllowsSSH`, call
`EnsureSSHKeyOnVM`) is identical. Both default to `"default"` network.

**Recommendation:** Extract to a shared `internal/access/ensure.go`:

```go
func EnsureVMAccess(
    ctx context.Context,
    cfg *config.Config,
    provider cloud.Provider,
    vmInfo *cloud.VMInfo,
) error
```

The TUI wraps this in a `tea.Cmd`; the CLI calls it directly.

---

## 4. SSH Connection Setup Duplication (MEDIUM)

Both paths independently resolve IP, user, port and create an SSH
client:

| Location | Function |
|----------|----------|
| `tui/commands.go:230-239` | `connectSSH()` -- minimal |
| `cli/vm_conn.go:42-109` | `connectToVM()` -- comprehensive |

The core of both is identical:

```go
ip, _ := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
sshUser := ssh.ResolveSSHUser(cfg.SSH.User)
sshCfg := ssh.SetupClientConfig(ip, sshUser, cfg.SSH.Port)
sshCfg.VM = ssh.NewVMIdentity(vmInfo.Name, vmInfo.CloudcoopCreated)
client, _ := ssh.NewClient(sshCfg)
```

**Recommendation:** Extract the pure connection logic to
`internal/ssh/` (e.g. `ConnectToVM`), letting each caller handle its
own validation and error wrapping.

---

## 5. VM State Validation Divergence (MEDIUM)

The CLI performs explicit state validation before operations:

- `start.go`: Checks VM is stopped, rejects if already running
- `stop.go`: Checks VM is running, rejects if already stopped
- `create.go`: Checks VM doesn't already exist

The TUI relies on **handler guard methods** (`canVMOp()`,
`canModifyAgents()`) to disable buttons, but the underlying commands
(`startVM`, `stopVM`, etc.) perform no validation. If the TUI state is
stale, invalid operations could be attempted.

**Recommendation:** Move state validation into a shared
`internal/vm/operations.go` layer, so both paths benefit:

```go
func StartVM(ctx context.Context, provider cloud.Provider, name string) error {
    info, _ := provider.GetVMInfo(ctx, name)
    if info.Status == cloud.VMStatusRunning { return ErrAlreadyRunning }
    return provider.StartVM(ctx, name)
}
```

---

## 6. Timeout Value Consistency (MEDIUM)

Timeouts are chosen independently in each path but currently happen
to match:

| Operation | TUI | CLI | Match? |
|-----------|-----|-----|--------|
| Create VM | 300s | 300s | Yes |
| Start/Stop/Delete | 180s | 180s | Yes |
| Status check | 10s | 10s | Yes |
| Firewall check | 15s | N/A | No CLI timeout |
| Provision status | implicit | 30s | Diverged |

**Recommendation:** Define timeout constants in
`internal/config/timeouts.go` and reference from both paths.

---

## 7. Error Handling Pattern Divergence (LOW)

This is largely architectural and acceptable:

- **TUI:** Returns errors in typed messages
  (e.g. `vmStartMsg{err: err}`), displayed in the view
- **CLI:** Returns errors via `RunE`, printed by Cobra

However, some error message inconsistencies exist:

- TUI wraps errors as `"get VM info: %w"` while CLI uses
  `"get VM status: %w"` for the same operation

**Recommendation:** Use consistent error messages via the shared
service layer.

---

## 8. Feature Parity Gap

Operations available in only one path:

| Operation | TUI | CLI |
|-----------|-----|-----|
| Auth login/status | No | Yes |
| Provision logs | No | Yes |
| SSH command execution | No | Yes |
| Config management | No | Yes |
| Setup wizard | No | Yes |
| Terminal config gen | No | Yes |
| Agents attach (--next/--window) | No | Yes |
| Agents list (--all sessions) | No | Yes |

Some of these are CLI-only by design (setup, config, SSH). Others
could be added to the TUI in future (auth status display, provision
log viewing).

---

## Recommended Refactoring Plan

### Phase 1: Shared Factories and Helpers

1. `internal/cloud/factory.go` -- single provider creation function
2. `internal/access/ensure.go` -- combined firewall + SSH key setup
3. `internal/config/timeouts.go` -- shared timeout constants

### Phase 2: Operation Layer

1. `internal/vm/operations.go` -- StartVM, StopVM, CreateVM, DeleteVM
   with validation
2. `internal/workspace/sync_service.go` -- core sync orchestration
3. `internal/agent/operations.go` -- agent add/kill/list wrappers

### Phase 3: Thin UI Adapters

- TUI commands become thin wrappers: create `tea.Cmd` -> call shared
  operation -> return message
- CLI commands become thin wrappers: parse flags -> call shared
  operation -> print result

This would reduce the combined TUI+CLI orchestration code by an
estimated 40-50%, and ensure both paths always execute identical logic.
