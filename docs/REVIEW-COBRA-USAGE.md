# Review: Cobra CLI Framework Usage

> Date: 2026-02-13
> Scope: Evaluation of Cobra feature usage against best practices,
> identifying missed opportunities

## Executive Summary

The project demonstrates **solid, pragmatic use of Cobra** with good
command structure, consistent `RunE` error handling, and comprehensive
help text. However, three significant Cobra features are not used that
would meaningfully improve both the user experience and code quality:

1. **Command Groups** -- would dramatically improve help output
   readability (low effort)
2. **Shell Completions** -- would improve usability for frequent
   users (medium effort)
3. **PersistentPreRun** -- would eliminate ~50 lines of duplicated
   config-loading boilerplate (medium effort)

**Overall Cobra Usage Score: 6.5/10** -- strong foundation with
significant untapped features.

---

## 1. Command Groups (NOT USED -- High Impact, Low Effort)

With 15+ subcommands, the flat help output is hard to scan:

```text
Available Commands:
  agents      Manage agent sessions
  auth        Manage agent authentication
  config      Manage cloudcoop configuration
  connect     Connect to an agent session
  create      Create a new VM
  delete      Delete a VM
  ...13 more commands...
```

Cobra's `AddGroup()` + `GroupID` (available since v1.6, project uses
v1.10.2) would produce:

```text
VM Lifecycle:
  create      Create a new VM
  start       Start a stopped VM
  stop        Stop a running VM
  delete      Delete a VM
  status      Show status of VMs and agents

Agent Management:
  agents      Manage agent sessions
  connect     Connect to an agent session

Configuration & Setup:
  config      Manage cloudcoop configuration
  setup       Set up a cloud project for cloudcoop

Additional Commands:
  auth        Manage agent authentication
  provision   VM provisioning management
  ssh         Execute a command on the VM via SSH
  terminal    Terminal emulator utilities
  version     Print version information
```

**Implementation** (~40 lines in `root.go`):

```go
rootCmd.AddGroup(
    &cobra.Group{ID: "vm", Title: "VM Lifecycle:"},
    &cobra.Group{ID: "agents", Title: "Agent Management:"},
    &cobra.Group{ID: "config", Title: "Configuration & Setup:"},
)
// Then: createCmd.GroupID = "vm", etc.
```

---

## 2. Shell Completions (NOT USED -- High Impact, Medium Effort)

No `ValidArgsFunction` or `RegisterFlagCompletionFunc` is used
anywhere. This is the single largest UX gap.

### Opportunities

| Command | What to Complete | How |
|---------|-----------------|-----|
| `agents kill <idx>` | Agent indices | `ValidArgsFunction` |
| `agents attach --window` | Window names/indices | Flag completion |
| `config get <key>` | Config key names | `ValidArgsFunction` |
| `config set <key> <val>` | Config keys, then values | `ValidArgsFunction` |
| `create --size` | small, medium, large, xlarge | Flag completion |
| `terminal generate --format` | ghostty, iterm2, kitty | Flag completion |
| `auth login <agent>` | claude (and future agents) | `ValidArgs` |

**Example** for config key completion:

```go
configShowCmd.ValidArgsFunction = func(
    cmd *cobra.Command,
    args []string,
    toComplete string,
) ([]string, cobra.ShellCompDirective) {
    return config.AllKeys(), cobra.ShellCompDirectiveNoFileComp
}
```

A `completion` subcommand should also be added so users can install
completions for their shell.

---

## 3. PersistentPreRun for Config Loading (NOT USED)

The same config-loading boilerplate is repeated in **15+ command
functions**:

```go
cfg, err := configLoader()
if err != nil {
    return handleConfigError(err)
}
if err := cfg.Validate(); err != nil {
    return handleConfigError(fmt.Errorf("invalid configuration: %w", err))
}
```

Found in: `status.go`, `start.go`, `stop.go`, `delete.go`,
`create.go`, `ssh.go`, `auth.go`, `provision_status.go`,
`provision_logs.go`, `agents_sync.go`, `agents_list.go`,
`agents_add.go`, `agents_kill.go`, `agents_attach.go`, `connect.go`,
and `vm_conn.go`.

**Recommendation:** Use `PersistentPreRunE` on the root command:

```go
var globalCfg *config.Config

rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
    // Skip for commands that don't need config
    if cmd.Name() == "version" || cmd.Name() == "setup" || cmd.Name() == "help" {
        return nil
    }
    var err error
    globalCfg, err = configLoader()
    if err != nil {
        return handleConfigError(err)
    }
    return globalCfg.Validate()
}
```

This eliminates ~50 lines of duplicated boilerplate and ensures
config validation is consistent.

---

## 4. Argument Validation -- Good, Minor Inconsistencies

The project makes good use of Cobra's `Args` validators:

| Command | Validator | Correct? |
|---------|-----------|----------|
| `connect` | `cobra.ExactArgs(1)` | Yes |
| `agents kill` | `cobra.ExactArgs(1)` | Yes |
| `config get` | `cobra.MaximumNArgs(1)` | Yes |
| `config set` | `cobra.ExactArgs(2)` | Yes |
| `auth login` | `cobra.MaximumNArgs(1)` | Yes |
| `ssh` | `cobra.MinimumNArgs(1)` | Yes |

**Minor issue:** `agents attach` uses manual flag validation instead
of Cobra's built-in:

```go
// agents_attach.go:42-44 -- manual
if !attachNext && attachWindow == "" {
    return fmt.Errorf("either --next or --window is required")
}
```

This is acceptable since it's validating flags (not args), and Cobra's
`MarkFlagsMutuallyExclusive` is already correctly used on this command.

---

## 5. Flag Handling -- Good, with Scattered SSH Flags

**Strengths:**

- Persistent flags (`--config`, `--verbose`) correctly at root level
  (`root.go:35-36`)
- `MarkFlagsMutuallyExclusive` used properly (`agents_attach.go:38`)
- `MarkFlagRequired` used where needed (`terminal_generate.go:49`)

**Issue:** SSH-related parameters (user, port) are resolved
independently in many commands via `ssh.ResolveSSHUser()` and
`ssh.ResolvePort()`. These aren't exposed as flags except on the
`ssh` command itself (`ssh.go:35-36`).

**Recommendation:** Consider adding `--ssh-user` and `--ssh-port` as
persistent flags on root, or accept the current approach where these
are config-driven.

---

## 6. Error Handling -- Excellent

All commands consistently use `RunE` (not `Run`), enabling proper
error propagation. The single exception is `version.go` which uses
`Run` since it cannot fail -- an appropriate choice.

**Strengths:**

- `SilenceErrors: false` on root (`root.go:28`) shows errors to users
- `SuggestionsMinimumDistance: 4` (`root.go:29`) helps with typos
- Domain-specific error handlers: `handleConfigError()`,
  `handleProviderError()`, `handleSSHError()`
- Custom usage error detection (`root.go:61-71`)

No changes recommended.

---

## 7. Help Text and Documentation -- Excellent

All commands have descriptive `Short` and `Long` descriptions. Many
include examples:

- `auth.go:79-82` -- shows agent options
- `config_cmd.go:22-25, 58-62` -- multiple examples
- `agents_attach.go:22-27` -- shows flag usage
- `setup_cmd.go:46-49` -- shows flag combinations

**Minor gap:** A few commands have minimal help text (`provision.go`,
`terminal.go`), but these are parent commands where subcommand help
matters more.

---

## 8. Version Command -- Correct

Uses a custom `version` subcommand (`version.go:16-25`) with `Run`
(not `RunE`). Version info set via ldflags in `main.go:13-16`. This
is the standard Cobra pattern and works well.

Cobra's built-in `--version` flag (`rootCmd.Version`) is an
alternative but would only show a single version string, not the
multi-line format currently used. The current approach is fine.

---

## 9. Missing Features Not Yet Discussed

### Command Aliases

A few commands use aliases (`config get` has alias `show` at
`config_cmd.go:30`). Consider aliases for common operations:

- `cloudcoop ls` -> `cloudcoop agents list`
- `cloudcoop up` -> `cloudcoop start`
- `cloudcoop down` -> `cloudcoop stop`

### Custom Help Template

Not used. The default is adequate, but if command groups are adopted,
the default template handles group rendering automatically.

---

## Summary Scorecard

| Criterion | Score | Notes |
|-----------|-------|-------|
| Command Structure | 8/10 | Well-organised, missing command groups |
| Flag Handling | 8/10 | Consistent, SSH flags could be persistent |
| Shell Completions | 2/10 | Not implemented -- significant gap |
| Argument Validation | 8/10 | Good use of Args validators |
| PreRun/PostRun Hooks | 3/10 | Not used; config loading duplicated 15+ times |
| Command Groups | 0/10 | Not implemented, would improve help |
| Error Handling | 9/10 | Excellent consistency |
| Help Text | 9/10 | Comprehensive with good examples |
| Version Command | 9/10 | Properly implemented |
| Bespoke vs Framework | 8/10 | Minimal unnecessary reimplementation |

---

## Top 3 Recommendations (Priority Order)

1. **Add Command Groups** -- ~40 lines, dramatic help output
   improvement
2. **Add Shell Completions** -- ~150-200 lines, major UX improvement
   for power users
3. **Use PersistentPreRun for config** -- ~100 lines refactoring,
   eliminates duplication
