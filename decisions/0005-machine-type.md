# ADR-0005: Machine Type

## Status

Accepted

## Context

We need to select a GCP machine type to run 10-12 concurrent Claude Code agents. Each agent:
- Runs Claude Code CLI (Node.js based)
- May build Docker containers
- May run tests and compilation tasks
- Needs reasonable responsiveness

The workload is CPU-bound rather than memory-intensive for most tasks.

## Decision

Use **c4a-highcpu-16** (16 vCPU, 32GB RAM) with Axion ARM processors.

## Options Considered

### Option 1: N2/N2D (Intel/AMD x86)

General-purpose x86 instances.

**Pros:**
- Widest software compatibility
- Available in all regions
- Familiar architecture

**Cons:**
- Higher cost than ARM equivalents
- Less power-efficient

### Option 2: T2A (Ampere ARM)

First-generation GCP ARM instances.

**Pros:**
- ARM architecture (cost-effective)
- Good compatibility with modern software

**Cons:**
- Being superseded by C4A
- Less performant than Axion

### Option 3: C4A (Axion ARM) (Chosen)

Latest ARM instances using Google's custom Axion processors.

**Pros:**
- Best price/performance for CPU workloads
- ~30-40% better performance than T2A
- Excellent power efficiency
- Good availability in Tier 1 European regions
- highcpu variants available (more vCPUs per GB RAM)

**Cons:**
- ARM architecture (some x86 software won't work)
- Newer, less battle-tested than N2
- Not available in all regions

### Option 4: C3 (Intel Sapphire Rapids)

Latest Intel-based compute-optimized instances.

**Pros:**
- Highest single-thread performance
- Full x86 compatibility
- Latest Intel features

**Cons:**
- Most expensive option
- Overkill for this workload

## Machine Size Comparison

For C4A in europe-north2:

| Type | vCPU | RAM | Spot $/month | Notes |
|------|------|-----|--------------|-------|
| c4a-highcpu-8 | 8 | 16GB | ~$42 | Tight for 12 agents |
| c4a-highcpu-16 | 16 | 32GB | ~$85 | Good balance |
| c4a-highcpu-32 | 32 | 64GB | ~$170 | Headroom for builds |

**Selected: c4a-highcpu-16** - Provides ~1.3 vCPUs per agent with 32GB shared RAM.

## Consequences

### Positive

- Cost-effective: ~$85/month spot for 16 vCPUs
- Sufficient CPU for 12 concurrent agents
- 32GB RAM handles typical development workloads
- ARM ecosystem mature for Node.js, Go, Python, Rust, Java

### Negative

- Some x86-only software won't run (rare for modern dev tools)
- Docker images must be ARM or multi-arch
- Can't run Windows containers

### Neutral

- Can scale up to c4a-highcpu-32 if more headroom needed
- Most development tools have native ARM builds
- Claude Code runs well on ARM (Node.js has excellent ARM support)
