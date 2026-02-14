# ADR-0027: Agent Startup Hooks

## Status

Accepted

## Context

When an agent starts in a tmux window, it needs more than just `cd <worktree> && claude`.
Different repositories and workflows require environment variables, tool configuration, or
setup commands before the agent begins. Examples:

- `export BEADS_NO_DAEMON=1` — prevents beads from spawning a background daemon in each
  agent session (beads is the expected issue tracker for repos using cloudcoop; this
  environment variable is harmless on repos without beads)
- `export ANTHROPIC_API_KEY=...` — API key injection
- `nvm use 18` — select Node.js version for the project
- `source .envrc` — load direnv-style project configuration

Currently, agent startup is hardcoded in `scripts/provision-vm.sh` within `start-agents.sh`:

```bash
tmux send-keys -t $SESSION_NAME:agent-$i \
  "cd /workspaces/agent-$i && claude --dangerously-skip-permissions" Enter
```

This approach:

- Cannot be customized per-repo or per-user
- Hardcodes the agent command (`claude`)
- Has no mechanism for pre-agent setup commands
- Contradicts the agent-agnostic design in [ADR-0008](0008-agent-agnostic-design.md)

## Decision

Support configurable pre-agent commands that run after `cd`'ing into the worktree but before
starting the coding agent. Configure these in `cloudcoop.toml`.

**Configuration:**

```toml
[agents]
default_command = "claude"
pre_commands = ["export BEADS_NO_DAEMON=1"]

# Per-repo overrides (optional)
[agents.repos.acme-backend]
command = "claude"
pre_commands = ["export BEADS_NO_DAEMON=1", "nvm use 18"]

[agents.repos.acme-frontend]
command = "aider"
pre_commands = ["export BEADS_NO_DAEMON=1", "nvm use 20"]
```

**Resulting tmux window command:**

```bash
cd /workspaces/acme-backend/feature-auth \
  && export BEADS_NO_DAEMON=1 \
  && nvm use 18 \
  && claude
```

The command chain is:

1. `cd <worktree-path>`
2. Execute each `pre_commands` entry in order (joined with `&&`)
3. Execute the agent command (repo-specific or default)

**Agent command resolution:**

1. Check `agents.repos.<slug>.command` — repo-specific override
2. Fall back to `agents.default_command`
3. Fall back to `"claude"` (hardcoded default)

**Pre-command resolution:**

1. Start with global `agents.pre_commands` (if defined)
2. Append repo-specific `agents.repos.<slug>.pre_commands` (if defined)
3. Both lists are concatenated (global first, then repo-specific)

**Default behaviour:**

With no configuration, the command is `cd <worktree> && claude` — backwards compatible
with the current behaviour, plus the implicit beads configuration.

## Options Considered

### Option 1: Hardcoded Environment Variables

Set known environment variables (like `BEADS_NO_DAEMON`) directly in the agent startup
code without user configuration.

**Pros:**

- Zero configuration needed
- Works out of the box for known tools
- Simple implementation

**Cons:**

- Cannot support user-specific or project-specific needs
- Adding new variables requires code changes and releases
- Not extensible — every new tool needs a code update
- Contradicts agent-agnostic design principle

### Option 2: Config-Based Pre-Commands (Chosen)

Configurable command list in `cloudcoop.toml`, executed before the agent command.

**Pros:**

- Fully extensible — any shell command can be a pre-command
- Per-repo customization supports diverse projects
- Global defaults reduce repetition
- Agent command is also configurable — swap `claude` for `aider` via config
- Transparent — user sees exactly what runs in their config file
- Enables agent-agnostic design ([ADR-0008](0008-agent-agnostic-design.md)) via command config

**Cons:**

- User must edit TOML config for customization
- Shell command injection risk (mitigated: user controls their own config)
- Command chain failures need clear error reporting (which command failed?)
- Pre-commands run in every new agent window (must be idempotent)

### Option 3: Per-Repo Shell Scripts

Each repository includes a `.cloudcoop/pre-agent.sh` script that runs before the agent.

**Pros:**

- Configuration lives with the project (checked into repo)
- Different repos have different setup naturally
- Familiar pattern (like `.envrc`, `.nvmrc`)

**Cons:**

- Requires modifying each repository
- Script must exist on the VM (chicken-and-egg: clone happens before script runs)
- Not all repos are controlled by the user (open source contributions)
- No global defaults
- Security concern — repo could contain malicious pre-agent script

## Consequences

### Positive

- Agent startup is fully configurable without code changes
- Per-repo agent selection enables mixed-agent workflows (Claude for one repo, Aider for another)
- Global pre-commands reduce boilerplate (e.g., beads config applies everywhere)
- Extends [ADR-0008](0008-agent-agnostic-design.md) — agent-agnostic design is
  achieved through configuration, not just code abstraction
- Transparent — the full command chain is visible in `cloudcoop.toml`

### Negative

- Users must learn TOML configuration format for customization
- Pre-commands that fail will prevent agent startup (by design — `&&` chaining)
- No validation of pre-commands at config time (only fails at runtime)
- Config file grows with number of repos

### Neutral

- Depends on [ADR-0022](0022-worktree-based-agent-workspaces.md) for worktree paths
- `BEADS_NO_DAEMON=1` is assumed as a sensible default for the global pre-commands;
  it has no effect on repos that don't use beads
- The per-repo `command` field replaces the hardcoded `claude` in `start-agents.sh`
- Future enhancement: `cloudcoop agents sync` could validate pre-commands by running
  them in a dry-run mode
