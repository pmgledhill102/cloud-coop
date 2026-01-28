# ADR-0022: Worktree-Based Agent Workspaces

## Status

Proposed

## Context

Currently, agents work in flat numbered directories (`/workspaces/agent-1`, `/workspaces/agent-2`, etc.)
as established in [ADR-0001](0001-agent-execution-model.md). This layout has no relationship to the
user's local repository structure — agents are assigned arbitrary workspace directories with no
connection to specific branches or worktrees.

In practice, users working with multiple AI coding agents follow a worktree-based workflow locally:
each agent works on a separate git worktree (and therefore a separate branch) within the same
repository. This maps naturally to how agents operate — each agent needs an isolated working
directory with its own branch to avoid conflicts with other agents.

The current flat directory layout creates friction:

- No correspondence between local worktrees and remote agent workspaces
- Users must manually track which agent is working on which branch
- No way to sync local repository structure to the VM
- Adding or removing agents requires manual directory management
- Multi-repo workflows (e.g., backend + frontend) have no natural organization

We need a workspace layout that mirrors the user's local git worktree structure and supports
multiple repositories on a single VM.

## Decision

Replace flat `/workspaces/agent-N` directories with git worktree-based workspaces organized by
repository.

**Directory layout on VM:**

```text
/repos/
  acme-backend.git          # Bare clone
  acme-frontend.git         # Bare clone

/workspaces/
  acme-backend/
    main/                   # Worktree: main branch
    feature-auth/           # Worktree: feature-auth branch
    fix-payments/           # Worktree: fix-payments branch
  acme-frontend/
    main/                   # Worktree: main branch
    redesign/               # Worktree: redesign branch
```

**Key concepts:**

- **Repo slug**: A URL-safe identifier derived from the repository name (e.g., `acme-backend`
  from `git@github.com:acme/acme-backend.git`). Used as directory names and tmux session names.
- **Bare clone**: Each repository is cloned as a bare repo at `/repos/<slug>.git`. Worktrees
  branch from this bare clone, sharing object storage and reducing disk usage.
- **1:1 mapping**: Each local worktree corresponds to exactly one remote worktree. One agent
  runs per worktree.

## Options Considered

### Option 1: Flat Directories with Metadata

Keep `/workspaces/agent-N` layout but add metadata files tracking which repo/branch each
workspace represents.

**Pros:**

- No change to existing directory layout
- Backwards compatible
- Simple implementation

**Cons:**

- Metadata can drift from reality
- No git-native relationship between workspaces
- Each workspace is a full clone (wastes disk)
- No natural multi-repo organization
- Branch switching requires full checkout operations

### Option 2: Git Worktree-Based Workspaces (Chosen)

Bare clone per repo, worktrees for each agent workspace.

**Pros:**

- Mirrors user's local development structure
- Git-native — worktrees share objects, save disk space
- Natural 1:1 mapping between local and remote worktrees
- Multi-repo support via repo slug directories
- Adding/removing worktrees is a lightweight git operation
- Each worktree has its own index and working directory (agent isolation)
- `git worktree list` provides authoritative workspace inventory

**Cons:**

- More complex initial setup than flat directories
- Requires git to be available on VM (already the case)
- Bare clone + worktree model is less familiar than regular clones
- Worktree pruning needed if local worktrees are deleted

### Option 3: Full Clone per Agent

Clone the entire repository independently for each agent workspace.

**Pros:**

- Complete isolation — each clone is independent
- Familiar mental model (just `git clone` per agent)
- No shared state between agent workspaces

**Cons:**

- Wastes disk space — full object store duplicated per clone
- Slow initial setup (N full clones vs 1 bare clone + N lightweight worktrees)
- No structural relationship between clones
- Harder to detect drift between agents

## Consequences

### Positive

- Workspace structure on VM mirrors local development environment
- Disk-efficient: shared object store via bare clone
- Adding a new agent workspace is a fast `git worktree add` operation
- Natural support for multiple repositories on one VM
- `git worktree list` serves as source of truth for active workspaces
- Clear naming: workspace paths encode both repository and branch context

### Negative

- More complex setup logic compared to `mkdir /workspaces/agent-N`
- Git worktree has constraints (e.g., same branch can't be checked out in two worktrees)
- Bare clone must be kept up-to-date for new remote branches
- Users unfamiliar with `git worktree` may find the layout confusing

### Neutral

- Extends [ADR-0001](0001-agent-execution-model.md) — agents still run in tmux windows with
  one agent per workspace, but workspaces are now git worktrees instead of flat directories
- Repo slug derivation must handle edge cases (repos with same name in different orgs,
  special characters)
- The `/repos/` and `/workspaces/` directories replace the previous flat `/workspaces/` layout
