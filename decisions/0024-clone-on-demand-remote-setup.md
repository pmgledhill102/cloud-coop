# ADR-0024: Clone-on-Demand Remote Setup

## Status

Accepted

## Context

With worktree-based workspaces ([ADR-0022](0022-worktree-based-agent-workspaces.md)) and
repo-scoped tmux sessions ([ADR-0023](0023-repo-scoped-tmux-sessions.md)), the VM needs a
bare clone and worktrees for each repository the user wants agents to work on. Someone or
something must set up this structure on the VM.

The question is: how does the repository and worktree structure get from the user's local
machine to the VM?

**Requirements:**

- Mirror the user's local worktree structure on the VM
- Support incremental updates (add new worktrees, detect removed ones)
- Handle first-time setup and reconnect scenarios
- Minimise manual steps — users shouldn't SSH in to set things up
- Work across repositories (user may have multiple repos)

**Inputs available locally:**

- `git worktree list --porcelain` — lists all local worktrees and their branches
- `git remote get-url origin` — the clone URL for the repository

## Decision

`cloudcoop agents sync` auto-clones repositories and creates worktrees on the VM by reading
local git state and executing remote commands via SSH.

**Sync algorithm:**

1. **Read local state**: Run `git worktree list --porcelain` and `git remote get-url origin`
   in the user's current directory
2. **Derive repo slug**: Extract repository name from remote URL
3. **Check remote state**: SSH to VM, check if `/repos/<slug>.git` exists
4. **Clone if missing**: `git clone --bare <remote-url> /repos/<slug>.git`
5. **Fetch latest**: `git -C /repos/<slug>.git fetch --all --prune`
6. **Create worktrees**: For each local worktree, create corresponding remote worktree
   at `/workspaces/<slug>/<worktree-name>` if it doesn't exist
7. **Start tmux session**: Create tmux session named `<slug>` with one window per worktree,
   each running the configured agent command
8. **Detect removals**: If remote worktrees exist that don't match any local worktree,
   prompt the user before cleanup

**Incremental behaviour:**

- Running `sync` again only creates what's missing — existing worktrees and running agents
  are left untouched
- New local worktrees are added as new tmux windows
- Removed local worktrees trigger a confirmation prompt before cleanup
- `git fetch` ensures new remote branches are available for worktree creation

**Pre-flight check:**

Before cloning, `sync` verifies git authentication by running
`git ls-remote <remote-url>` on the VM. If this fails, it provides an actionable error
message directing the user to set up VM git access (see
[ADR-0026](0026-vm-git-authentication.md)).

## Options Considered

### Option 1: Manual Setup

User SSHs into the VM and manually clones repos, creates worktrees, and starts tmux sessions.

**Pros:**

- No cloudcoop logic needed
- User has full control
- Works with any git workflow

**Cons:**

- Tedious and error-prone for 10+ worktrees
- Must be repeated for each VM or after preemption
- No automation benefit from cloudcoop
- Defeats the purpose of a management tool

### Option 2: rsync from Local

Sync local worktree contents to the VM using rsync.

**Pros:**

- Exact mirror of local state
- Works without VM git access
- Familiar tool

**Cons:**

- Transfers entire working directory contents (potentially gigabytes)
- Slow over network for large repos
- Doesn't transfer git history correctly (worktrees share `.git`)
- Ongoing sync needed as agents make changes
- Two-way sync is complex and fragile
- Loses git worktree structure on the remote

### Option 3: Clone-on-Demand (Chosen)

`cloudcoop agents sync` reads local git metadata and executes clone/worktree operations
on the VM via SSH.

**Pros:**

- Automated — one command sets up everything
- Incremental — only creates what's missing
- Uses git natively — bare clone + worktrees on VM
- Minimal data transfer — only sends commands, not file contents
- Repo is fully functional on VM (can push, pull, branch)
- Handles multi-repo naturally (run sync from each repo's directory)
- Composable with agent startup hooks ([ADR-0027](0027-agent-startup-hooks.md))

**Cons:**

- VM needs git access to the remote (authentication required)
- Initial clone can be slow for large repositories
- Local uncommitted changes are not synced (only branches/worktrees)
- Relies on SSH access to VM for remote commands

### Option 4: Git Bundle Transfer

Create git bundles locally and transfer them to the VM.

**Pros:**

- Works without VM git access to the remote
- Full git history transferred
- No authentication setup needed on VM

**Cons:**

- Bundles can be large for repos with long history
- Must re-bundle and transfer for updates
- Doesn't support incremental updates well
- Unusual workflow — most developers aren't familiar with git bundles
- Push from VM back to remote still needs authentication

## Consequences

### Positive

- One command (`cloudcoop agents sync`) handles complete VM setup for a repository
- Incremental updates make daily workflow fast (only creates new worktrees)
- Git-native approach means full repo functionality on VM
- Pre-flight auth check prevents confusing failures mid-clone
- Multi-repo support — run sync from each repo's directory

### Negative

- VM must have git access to remote repositories (see [ADR-0026](0026-vm-git-authentication.md))
- Initial clone of large repos may be slow
- Local uncommitted work is not synced (by design — agents work on committed branches)
- Sync must handle edge cases (bare clone corruption, worktree conflicts, disk space)

### Neutral

- Depends on [ADR-0022](0022-worktree-based-agent-workspaces.md) for directory layout
  and [ADR-0023](0023-repo-scoped-tmux-sessions.md) for tmux session naming
- Security companion: [ADR-0026](0026-vm-git-authentication.md) addresses VM git authentication
- Agent startup is handled by [ADR-0027](0027-agent-startup-hooks.md) — sync creates worktrees
  and starts agents with configured commands
