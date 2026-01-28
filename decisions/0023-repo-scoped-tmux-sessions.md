# ADR-0023: Repo-Scoped tmux Sessions

## Status

Proposed

## Context

The codebase currently hardcodes a single tmux session name throughout:

- `internal/agent/agent.go` — uses `"agents"` in all tmux commands (`new-session -s agents`,
  `list-windows -t agents`, `new-window -t agents`, `kill-window -t agents:`)
- `internal/ssh/connect.go` — uses `"agents"` in `tmux attach -t agents`
- `internal/terminal/ghostty.go`, `kitty.go`, `iterm2.go` — use `"agents"` in attach commands
- `scripts/provision-vm.sh` — creates `start-agents.sh` with `SESSION_NAME="claude-agents"`
  (notably inconsistent with the Go code's `"agents"`)

This single-session design cannot support multiple repositories on one VM. With the introduction
of worktree-based workspaces ([ADR-0022](0022-worktree-based-agent-workspaces.md)), each
repository has its own set of worktrees and agents. A single tmux session conflates agents from
different repos, making it difficult to:

- List agents for a specific repository
- Attach to a repo's agents without seeing unrelated windows
- Manage agent lifecycle per repository
- Support the `cloudcoop agents list --all` vs per-repo listing distinction

## Decision

Use one tmux session per repository, named by repo slug (e.g., `acme-backend`, `acme-frontend`).

**tmux session structure:**

```text
tmux sessions:
  acme-backend          # Session for backend repo
    0: main             # Window: agent on main worktree
    1: feature-auth     # Window: agent on feature-auth worktree
    2: fix-payments     # Window: agent on fix-payments worktree
  acme-frontend         # Session for frontend repo
    0: main             # Window: agent on main worktree
    1: redesign         # Window: agent on redesign worktree
```

**CLI behavior:**

- `cloudcoop agents list` — lists agents for the current repo (detected from cwd git remote)
- `cloudcoop agents list --all` — lists agents across all repos/sessions
- `cloudcoop agents attach` — attaches to the current repo's session
- Repo detection: parse `git remote get-url origin` from cwd, derive slug

**Repo slug derivation:**

```text
git@github.com:acme/acme-backend.git  →  acme-backend
https://github.com/acme/frontend.git  →  frontend
```

The slug is the repository name portion of the remote URL, lowercased with special characters
replaced by hyphens.

## Options Considered

### Option 1: Single Session with Prefixed Window Names

Keep one tmux session but prefix window names with the repo slug
(e.g., `acme-backend:main`, `acme-frontend:redesign`).

**Pros:**

- Minimal change to existing code (session name stays the same)
- All agents visible in one `tmux list-windows`
- Simple implementation

**Cons:**

- Window names become long and harder to read
- No native tmux grouping — all windows in one flat list
- `tmux attach` always shows all repos (no per-repo focus)
- Window name parsing required to filter by repo
- Doesn't compose well with grouped sessions ([ADR-0025](0025-terminal-native-split-workflow.md))

### Option 2: Per-Repository tmux Session (Chosen)

One tmux session per repository, named by repo slug.

**Pros:**

- Clean separation between repos at the tmux level
- `tmux list-sessions` shows all active repos at a glance
- Per-repo attach (`tmux attach -t acme-backend`) for focused work
- Window names are short and meaningful (just the worktree/branch name)
- Natural fit for grouped sessions ([ADR-0025](0025-terminal-native-split-workflow.md))
- `tmux list-windows -t <slug>` lists only that repo's agents

**Cons:**

- Requires changing all hardcoded `"agents"` references in Go code
- CLI must detect current repo context (from cwd)
- Session lifecycle is per-repo (create when first agent starts, destroy when last stops)
- Users must specify repo when not in a repo directory

### Option 3: Per-Worktree tmux Session

One tmux session per worktree (e.g., `acme-backend-feature-auth`).

**Pros:**

- Maximum isolation — each agent in its own session
- No window management needed

**Cons:**

- Too many sessions for typical workflows (10+ sessions)
- Loses the grouping benefit of seeing all agents for a repo together
- `tmux list-sessions` becomes unwieldy
- Doesn't match the mental model of "a repo's agents"

## Consequences

### Positive

- Multi-repo support on a single VM
- Clean tmux session naming that maps to repositories
- CLI can auto-detect repo context from working directory
- Per-repo agent management (list, attach, sync) works naturally
- Composes well with terminal-native splits via grouped sessions

### Negative

- All hardcoded `"agents"` references must be replaced with dynamic session names
- CLI needs repo detection logic (parsing git remote URLs)
- Error handling needed for when user is not in a git repo directory
- Must handle session name collisions (two repos with same name in different orgs)

### Neutral

- Key files requiring changes: `internal/agent/agent.go`, `internal/ssh/connect.go`,
  `internal/terminal/ghostty.go`, `internal/terminal/kitty.go`,
  `internal/terminal/iterm2.go`, `scripts/provision-vm.sh`
- The inconsistency between Go code (`"agents"`) and `start-agents.sh` (`"claude-agents"`)
  is resolved — both will use the repo slug
- Depends on [ADR-0022](0022-worktree-based-agent-workspaces.md) for the repo slug concept
