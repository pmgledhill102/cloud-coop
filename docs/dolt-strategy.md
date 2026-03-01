# Dolt Sync Strategy: Git Remote for Beads

## Problem

The beads migration to Dolt created a sync problem. The database is local-only, cloud-coop VMs are
ephemeral, and there is no way to create or manage issues from mobile/web Claude Code sessions.

Every environment that runs agents needs access to the same issue tracker.

## Solution

Use Dolt's native Git remote support (added in v1.81.10, built specifically for beads). The
existing GitHub repo serves as the Dolt remote via `refs/dolt/data` — no new infrastructure needed.

Every environment uses `bd` natively. The GitHub repo is the shared remote.

```text
Mac (bd CLI) ──────────────────┐
                               │
cloud-coop VM (bd CLI) ────────┤── GitHub repo (refs/dolt/data)
                               │      dolt push / dolt pull
Claude Code web/mobile (bd) ───┘
                               ▲
                          bootstrap script
                        installs dolt + beads
```

## How Git Remote Works

- Dolt stores database state on `refs/dolt/data` in the Git repo
- This ref is **not cloned** by normal `git clone` — it does not affect working copies
- Managed entirely by `dolt push` / `dolt pull`
- Uses existing Git/SSH authentication
- Built specifically for beads by the Dolt team

## Implementation Phases

### Phase 1: Configure Dolt Git Remote (Mac)

One-time setup on the local Mac to establish the remote and push the initial database state.

```bash
cd /opt/homebrew/var/dolt/beads_cc/
dolt remote add origin git@github.com:pmgledhill102/cloud-coop.git
dolt push origin main
```

Verify with `dolt remote -v` and confirm that `refs/dolt/data` appears on GitHub.

### Phase 2: Bootstrap Script

Write `scripts/beads-bootstrap.sh` — installs Dolt + beads, clones the database from the Git
remote, starts the Dolt server. Used by:

- cloud-coop VM provisioning (automatic)
- Claude Code web/mobile sessions (manual or via project instructions)

Script responsibilities:

1. Install Dolt (`curl -sSL https://install.dolthub.com/install.sh | bash`)
2. Install beads (`go install` or binary download)
3. Clone Dolt database: `dolt clone git@github.com:pmgledhill102/cloud-coop.git beads_cc`
4. Start Dolt server: `dolt sql-server --database beads_cc &`
5. Configure beads: `bd init --prefix cc --server-port 3306`

### Phase 3: Integrate with cloud-coop Provisioning

Add `dolt pull` (or full bootstrap) to the VM provisioning script so beads is available immediately
after VM startup.

- Modify `scripts/provision-vm.sh` to include Dolt database clone and server setup
- Add periodic `bd dolt push` (cron or agent hook) so changes sync back

### Phase 4: Makefile Targets

```makefile
beads-push:     ## Push beads database to GitHub
 cd /opt/homebrew/var/dolt/beads_cc/ && dolt push origin main

beads-pull:     ## Pull latest beads database from GitHub
 cd /opt/homebrew/var/dolt/beads_cc/ && dolt pull origin main
```

### Phase 5: Documentation

- ADR-0030 capturing the decision (see `decisions/0030-beads-dolt-git-remote-sync.md`)
- Update project instructions for running bootstrap on web/mobile sessions

## When Push/Pull Happens

### Push

Happens as part of the existing session completion protocol:

```bash
git pull --rebase
bd dolt push          # replaces deprecated `bd sync`
git push
```

Every agent session that modifies issues pushes Dolt alongside code. This also applies to PR
merges — when work lands, the issue state (closed, updated) goes with it.

### Pull

Happens at session start:

- VM provisioning script runs `dolt pull origin main`
- Claude Code web/mobile bootstrap runs `dolt clone` or `dolt pull`
- Local dev: `make beads-pull` before starting work

### Optional Automation

- Pre-push git hook: auto-trigger `dolt push` on every `git push`
- Startup hook: auto-trigger `dolt pull` when `bd` first runs in a session

### Sync Diagram

```text
Mac (local dev)                 GitHub repo                   cloud-coop VM / Claude Code
     │                       (refs/dolt/data)                      │
     │─── dolt push ────────────►│                                 │
     │                           │◄──────────── dolt pull ─────────│  (startup)
     │                           │                                 │
     │  bd create / bd close     │     bd create / bd close        │
     │                           │                                 │
     │                           │◄──────────── dolt push ─────────│  (session end)
     │◄── dolt pull ─────────────│                                 │
     │                           │            VM terminated ──────►│  (data safe)
```

## Multi-Agent Concurrency

### Why Dolt Server Mode Is Required

Dolt has two modes:

- **CLI mode** (`dolt sql`) — single process, file-level locking, one writer at a time
- **Server mode** (`dolt sql-server`) — MySQL-compatible TCP server, handles concurrent reads/writes
  via SQL transactions

Multiple agents (on Mac or VM) all connect to the same Dolt server on `localhost:3306`. The server
serialises writes internally. This is the primary reason beads migrated from SQLite to Dolt.

### Server Configuration

**On Mac** (already running via Homebrew):

```bash
# Already configured:
brew services start dolt   # Starts dolt sql-server on port 3306
# Database: beads_cc at /opt/homebrew/var/dolt/beads_cc/
```

**On cloud-coop VM** (needs setup in provisioning):

```bash
# In provisioning script:
dolt clone git@github.com:pmgledhill102/cloud-coop.git /var/lib/dolt/beads_cc
cd /var/lib/dolt/beads_cc
dolt sql-server --host 127.0.0.1 --port 3306 &

# Or as a systemd service:
# [Unit]
# Description=Dolt SQL Server
# After=network.target
# [Service]
# ExecStart=/usr/local/bin/dolt sql-server --host 127.0.0.1 --port 3306
# WorkingDirectory=/var/lib/dolt/beads_cc
# Restart=always
```

**Beads connects** via the server (already configured in `.beads/metadata.json`):

```json
{
  "backend": "dolt",
  "dolt_mode": "server",
  "dolt_server_port": 3306,
  "dolt_database": "beads_cc"
}
```

### Multi-Agent Scenario

```text
Agent 1 (Claude Code) ──┐
Agent 2 (Aider) ─────────┤── bd create / bd close ──► dolt sql-server (port 3306)
Agent 3 (Gemini CLI) ────┘                                    │
                                                         beads_cc database
                                                               │
                                                    Session completion:
                                                      dolt push origin main
```

All agents share the same Dolt server. Each `bd` command is a SQL transaction — no conflicts
between concurrent writers on different issues. The server handles this.

### Push/Pull Coordination

- Only one process should push at a time (like `git push`)
- If two pushes race, the second fails — pull, merge, retry
- In practice: only the agent completing its session pushes (as part of session close protocol)
- Pull happens once at startup, not per-agent

## Conflict Handling

Conflicts are rare in issue tracking (different issues, append-mostly). When they occur:

- Dolt stores conflicts in `dolt_conflicts_issues` with base/ours/theirs columns
- Resolution: `CALL DOLT_CONFLICTS_RESOLVE('issues', 'ours')` or per-cell
- For beads, "last writer wins" is usually safe
