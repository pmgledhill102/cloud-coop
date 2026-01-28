# Multi-Agent Workflow

This document describes the user journeys for running multiple AI coding agents on a cloud VM
using cloudcoop with git worktrees and terminal-native splits.

**Related decisions:**

- [ADR-0022: Worktree-Based Agent Workspaces](../decisions/0022-worktree-based-agent-workspaces.md)
- [ADR-0023: Repo-Scoped tmux Sessions](../decisions/0023-repo-scoped-tmux-sessions.md)
- [ADR-0024: Clone-on-Demand Remote Setup](../decisions/0024-clone-on-demand-remote-setup.md)
- [ADR-0025: Terminal-Native Split Workflow](../decisions/0025-terminal-native-split-workflow.md)
- [ADR-0026: VM Git Authentication](../decisions/0026-vm-git-authentication.md)
- [ADR-0027: Agent Startup Hooks](../decisions/0027-agent-startup-hooks.md)

## Concepts

### Repo Slug

A URL-safe identifier derived from the git remote URL. Used for directory names and tmux
session names.

```text
git@github.com:acme/acme-backend.git  →  acme-backend
https://github.com/acme/frontend.git  →  frontend
```

### Worktree-to-Window Mapping

Each local git worktree maps to one tmux window on the VM. One agent runs per window.

```text
Local machine                          VM
─────────────                          ──
~/dev/acme-backend/        →  tmux session: acme-backend
  main/                    →    window 0: main
  feature-auth/            →    window 1: feature-auth
  fix-payments/            →    window 2: fix-payments
```

### Grouped Sessions

tmux grouped sessions allow multiple terminal splits to attach to the same tmux session
while independently selecting which window each split displays. Each split creates a
grouped session that shares windows with the main session but has its own current-window
pointer. Grouped sessions auto-destroy on disconnect.

### Directory Layout on VM

```text
/repos/
  acme-backend.git              # Bare clone (shared object store)
  acme-frontend.git

/workspaces/
  acme-backend/
    main/                       # Worktree → agent workspace
    feature-auth/
    fix-payments/
  acme-frontend/
    main/
    redesign/
```

## Journey 1: First-Time Setup

**Scenario:** User has a local repo with git worktrees and a running cloudcoop VM. They want
to start agents on the VM, one per worktree.

### Prerequisites

- A cloudcoop VM is running (`cloudcoop vm start`)
- VM has git access to the repository (deploy key configured — see
  [ADR-0026](../decisions/0026-vm-git-authentication.md))
- Local repo has worktrees set up

### Steps

**1. Check local worktree layout:**

```console
$ cd ~/dev/acme-backend
$ git worktree list
/Users/paul/dev/acme-backend/main           abc1234 [main]
/Users/paul/dev/acme-backend/feature-auth   def5678 [feature-auth]
/Users/paul/dev/acme-backend/fix-payments   ghi9012 [fix-payments]
```

**2. Sync to VM:**

```console
$ cloudcoop agents sync
Detecting worktrees for acme-backend...
  Found 3 worktrees: main, feature-auth, fix-payments

Checking VM git access...
  ✓ Can access git@github.com:acme/acme-backend.git

Setting up VM...
  Cloning bare repo to /repos/acme-backend.git... done
  Creating worktree: /workspaces/acme-backend/main... done
  Creating worktree: /workspaces/acme-backend/feature-auth... done
  Creating worktree: /workspaces/acme-backend/fix-payments... done

Starting tmux session: acme-backend
  Window 0: main → claude
  Window 1: feature-auth → claude
  Window 2: fix-payments → claude

✓ 3 agents running in session "acme-backend"
```

**3. Open Ghostty and create splits:**

Open Ghostty. In the first pane:

```console
$ cloudcoop agents attach --next
Attaching to acme-backend window 0 (main)...
```

Press `⌘+D` to create a vertical split. In the new pane:

```console
$ cloudcoop agents attach --next
Attaching to acme-backend window 1 (feature-auth)...
```

Press `⌘+Shift+D` for a horizontal split. In the new pane:

```console
$ cloudcoop agents attach --next
Attaching to acme-backend window 2 (fix-payments)...
```

**Result:** Three Ghostty splits, each showing a different agent working on a different branch.

```text
┌─────────────────────┬─────────────────────┐
│ agent: main         │ agent: feature-auth │
│ claude running...   │ claude running...   │
│                     ├─────────────────────┤
│                     │ agent: fix-payments │
│                     │ claude running...   │
└─────────────────────┴─────────────────────┘
```

## Journey 2: Daily Reconnect

**Scenario:** User closed Ghostty yesterday. Agents are still running on the VM. They want
to reconnect.

### Steps

**1. Check running agents:**

```console
$ cd ~/dev/acme-backend
$ cloudcoop agents list
Session: acme-backend (3 agents)
  0: main           claude  running  12h 30m
  1: feature-auth   claude  running  12h 30m
  2: fix-payments   claude  running  12h 30m
```

**2. Open Ghostty and attach:**

Create splits and run `cloudcoop agents attach --next` in each, same as initial setup.
The `--next` command detects which windows already have a client attached and assigns
the next unattached window.

```console
# First split
$ cloudcoop agents attach --next
Attaching to acme-backend window 0 (main)...

# Second split (⌘+D)
$ cloudcoop agents attach --next
Attaching to acme-backend window 1 (feature-auth)...

# Third split (⌘+Shift+D)
$ cloudcoop agents attach --next
Attaching to acme-backend window 2 (fix-payments)...
```

**3. Or attach to a specific window:**

```console
$ cloudcoop agents attach --window feature-auth
Attaching to acme-backend window 1 (feature-auth)...
```

## Journey 3: Multi-Repo

**Scenario:** User works on two repos (backend + frontend) that share one VM. Each repo has
its own worktrees and agents.

### Steps

**1. Sync both repos:**

```console
$ cd ~/dev/acme-backend
$ cloudcoop agents sync
...
✓ 3 agents running in session "acme-backend"

$ cd ~/dev/acme-frontend
$ cloudcoop agents sync
...
✓ 2 agents running in session "acme-frontend"
```

**2. List all agents across repos:**

```console
$ cloudcoop agents list --all
Session: acme-backend (3 agents)
  0: main           claude  running  2h 10m
  1: feature-auth   claude  running  2h 10m
  2: fix-payments   claude  running  2h 10m

Session: acme-frontend (2 agents)
  0: main           claude  running  1h 45m
  1: redesign       aider   running  1h 45m
```

**3. List agents for current repo only:**

```console
$ cd ~/dev/acme-frontend
$ cloudcoop agents list
Session: acme-frontend (2 agents)
  0: main       claude  running  1h 45m
  1: redesign   aider   running  1h 45m
```

**4. Attach to a specific repo's agents:**

Open one Ghostty window for backend, another for frontend. In each, use
`cloudcoop agents attach --next` (repo auto-detected from cwd).

## Journey 4: Mid-Session Changes

**Scenario:** User needs to add or remove worktrees while agents are already running.

### Adding a Worktree

**1. Create worktree locally:**

```console
cd ~/dev/acme-backend
git worktree add ../acme-backend/hotfix-login hotfix-login
```

**2. Sync to VM:**

```console
$ cloudcoop agents sync
Detecting worktrees for acme-backend...
  Found 4 worktrees: main, feature-auth, fix-payments, hotfix-login

Syncing with VM...
  ✓ main — already exists
  ✓ feature-auth — already exists
  ✓ fix-payments — already exists
  Creating worktree: /workspaces/acme-backend/hotfix-login... done
  Starting agent in new window: hotfix-login → claude

✓ 4 agents running in session "acme-backend" (1 new)
```

**3. Attach to the new agent:**

```console
$ cloudcoop agents attach --window hotfix-login
Attaching to acme-backend window 3 (hotfix-login)...
```

### Removing a Worktree

**1. Remove worktree locally:**

```console
cd ~/dev/acme-backend
git worktree remove ../acme-backend/fix-payments
```

**2. Sync detects the removal:**

```console
$ cloudcoop agents sync
Detecting worktrees for acme-backend...
  Found 3 worktrees: main, feature-auth, hotfix-login

Syncing with VM...
  ✓ main — already exists
  ✓ feature-auth — already exists
  ✓ hotfix-login — already exists
  ⚠ fix-payments — exists on VM but not locally

Remove fix-payments from VM? This will:
  - Stop the agent running in this worktree
  - Remove /workspaces/acme-backend/fix-payments
  - Remove the tmux window

Proceed? [y/N] y
  Removing fix-payments... done

✓ 3 agents running in session "acme-backend" (1 removed)
```

## Technical Summary

### Architecture

```text
Workstation                              Cloud VM
───────────                              ────────
~/dev/acme-backend/                      /repos/acme-backend.git (bare)
  main/              ──sync──→           /workspaces/acme-backend/
  feature-auth/                            main/         → tmux window 0
  fix-payments/                            feature-auth/ → tmux window 1
                                           fix-payments/ → tmux window 2

Ghostty splits       ──SSH──→            tmux grouped sessions
  Split 1 ─────────────────→             acme-backend-a1b2 → window 0
  Split 2 ─────────────────→             acme-backend-c3d4 → window 1
  Split 3 ─────────────────→             acme-backend-e5f6 → window 2
```

### Command Summary

| Command | Description |
|---------|-------------|
| `cloudcoop agents sync` | Clone repo, create worktrees, start agents on VM |
| `cloudcoop agents list` | List agents for current repo |
| `cloudcoop agents list --all` | List agents across all repos |
| `cloudcoop agents attach --next` | Attach to next unattached agent window |
| `cloudcoop agents attach --window <name>` | Attach to specific agent window |

### Configuration

```toml
# ~/.config/cloudcoop/cloudcoop.toml

[agents]
default_command = "claude"
pre_commands = ["export BEADS_NO_DAEMON=1"]

# Per-repo overrides
[agents.repos.acme-backend]
command = "claude"
pre_commands = ["nvm use 18"]

[agents.repos.acme-frontend]
command = "aider"
pre_commands = ["nvm use 20"]
```

Global `pre_commands` run first, then repo-specific `pre_commands`. The full command for
each tmux window is:

```bash
cd /workspaces/<slug>/<worktree> \
  && export BEADS_NO_DAEMON=1 \
  && nvm use 18 \
  && claude
```

### VM Git Access

Repositories are cloned on the VM using deploy keys scoped to each repository. See
[ADR-0026](../decisions/0026-vm-git-authentication.md) for setup instructions.
