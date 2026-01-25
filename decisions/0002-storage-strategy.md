# ADR-0002: Storage Strategy

## Status

Accepted

## Context

Claude Code agents need persistent storage that survives:
1. Manual VM stops (end of work session)
2. Spot instance preemption by GCP
3. VM reboots

Key data to persist:
- `~/.claude/` - Claude Code session database (SQLite)
- `/workspaces/` - Git repositories and work in progress
- Installed tools and configurations

## Decision

Use the boot disk as the sole persistent storage. Configure it with `--boot-disk-auto-delete=no` to survive instance deletion. No separate persistent disk needed.

Disk size: 50GB SSD (pd-ssd)

## Options Considered

### Option 1: Separate Persistent Disk

Attach a secondary persistent disk mounted at `/mnt/persist` with symlinks from `~/.claude/` and `/workspaces/`.

**Pros:**
- Clear separation between OS and data
- Can resize data disk independently
- Can snapshot just the data disk
- Can detach and attach to different VMs

**Cons:**
- Additional complexity (mount scripts, fstab entries, symlinks)
- Extra cost for second disk
- More failure modes (mount failures, symlink issues)
- Boot disk already survives stop/preemption by default

### Option 2: Boot Disk Only (Chosen)

Use a single boot disk with `--boot-disk-auto-delete=no`. All data lives on the boot disk.

**Pros:**
- Simplest possible setup
- No symlinks or mount management needed
- Boot disk survives VM stop and spot preemption by default
- Single disk to manage and snapshot
- Lower cost (one disk instead of two)

**Cons:**
- OS and data on same disk (less separation)
- Resizing requires stopping the VM
- Can't detach data separately

### Option 3: Cloud Storage (GCS) Sync

Periodically sync state to Google Cloud Storage buckets.

**Pros:**
- Survives complete VM deletion
- Cross-region redundancy
- Cheaper for cold storage

**Cons:**
- Sync delays could lose recent work
- More complex restore process
- Ongoing egress costs
- Not suitable for active working state

## Consequences

### Positive

- Zero additional setup beyond VM creation
- No mount failures or symlink issues
- Everything "just works" after VM start
- Cost efficient: 50GB SSD at ~$5/month when stopped

### Negative

- Must remember `--boot-disk-auto-delete=no` when creating VM
- Cannot easily move data to a different VM (would need disk snapshot/clone)

### Neutral

- 50GB sufficient for Claude sessions (~200MB) plus typical git repos
- Can resize disk later if needed: `gcloud compute disks resize`
