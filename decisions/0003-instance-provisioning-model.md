# ADR-0003: Instance Provisioning Model

## Status

Accepted

## Context

We need to choose how to provision the GCP Compute Engine VM that hosts Claude Code agents. The VM will be used intermittently (during active development sessions) rather than 24/7.

Cost efficiency is important, but so is preserving state across sessions.

## Decision

Use spot instances with `--instance-termination-action=STOP` combined with a persistent boot disk (`--boot-disk-auto-delete=no`).

This provides ~70% cost savings while running, with automatic state preservation when preempted or manually stopped.

## Options Considered

### Option 1: On-Demand Instances

Standard VM provisioning with guaranteed availability.

**Pros:**
- No preemption risk
- Guaranteed availability
- Simpler mental model

**Cons:**
- Full price (~$280/month for c4a-highcpu-16 if running 24/7)
- Paying premium for availability that isn't needed for dev workloads

### Option 2: Spot Instances with DELETE on Preemption

Spot pricing with instance deletion on preemption.

**Pros:**
- ~70% cost savings
- Lowest compute cost

**Cons:**
- Instance deleted on preemption (need to recreate)
- Boot disk deleted by default (data loss without separate persistent disk)
- More complex recovery process

### Option 3: Spot Instances with STOP on Preemption (Chosen)

Spot pricing with instance stop (not delete) on preemption, plus persistent boot disk.

**Pros:**
- ~70% cost savings while running
- Instance stops on preemption (not deleted)
- Boot disk survives (all state preserved)
- Same behavior as manual stop
- Simple restart: `gcloud compute instances start`

**Cons:**
- Still subject to preemption during work (30-second warning)
- May need to restart and resume if preempted mid-task

### Option 4: Preemptible Instances (Legacy)

Older preemptible VM model (max 24-hour lifetime).

**Pros:**
- Similar cost savings to spot

**Cons:**
- Legacy model, spot is the replacement
- Forced termination after 24 hours
- Less flexible than spot

## Consequences

### Positive

- ~70% cost reduction: ~$85/month spot vs ~$280/month on-demand (if running 24/7)
- When stopped (preempted or manual), only pay for disk: ~$5/month
- Restart is simple: `gcloud compute instances start`
- Claude Code `--continue` resumes previous session seamlessly

### Negative

- Work may be interrupted by preemption (30-second warning)
- Need to restart and resume agents after preemption
- Spot availability not guaranteed (rare in practice for standard regions)

### Neutral

- Preemption behavior identical to manual stop from state perspective
- Shutdown hook can cleanly save git state within 25-second window
- Typical preemption frequency is low (hours to days between events)
