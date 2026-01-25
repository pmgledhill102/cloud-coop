# ADR-0001: Agent Execution Model

## Status

Accepted

## Context

We need to run multiple Claude Code agents (10-12) concurrently on a single GCP Compute Engine VM. Each agent works on different tasks and needs access to development tools, git repositories, and the ability to build and run containers.

The question is how to isolate and manage these agent sessions.

## Decision

Run Claude Code agents directly in shell sessions managed by tmux, not inside Docker containers. Docker is installed on the VM for agents to use as part of their development work (building images, running services), but agent execution itself happens in native shell sessions.

## Options Considered

### Option 1: Docker Containers per Agent

Run each Claude Code agent inside its own Docker container with mounted volumes.

**Pros:**
- Strong isolation between agents
- Easy to reset/recreate individual agent environments
- Consistent environment via Dockerfile
- Resource limits per container

**Cons:**
- Added complexity (Docker-in-Docker or sibling containers)
- Performance overhead for file I/O through volume mounts
- Claude Code session persistence more complex (need to persist ~/.claude across container restarts)
- Nested virtualization complications on some instance types

### Option 2: Direct Shell Sessions with tmux (Chosen)

Run agents directly in shell sessions within a tmux session, one window per agent.

**Pros:**
- Simple architecture - no container orchestration needed
- Native file system performance
- Claude Code session persistence works naturally (~/.claude on disk)
- Agents can use Docker directly for their own container builds
- Easy to attach/detach and monitor via tmux

**Cons:**
- Less isolation between agents (shared file system, processes)
- All agents share same installed tools/versions
- One agent could potentially affect others (resource contention)

### Option 3: Separate VMs per Agent

Run each agent on its own small VM instance.

**Pros:**
- Complete isolation
- Independent scaling and lifecycle
- Can use different machine types per agent

**Cons:**
- Significantly higher cost (VM overhead per instance)
- More complex orchestration
- Higher management overhead

## Consequences

### Positive

- Simple setup and operation
- Native performance for all file operations
- Claude Code's built-in session persistence (`--continue`, `--resume`) works seamlessly
- Agents can build and run Docker containers for their work without nesting issues
- Easy debugging via tmux attach

### Negative

- Agents share resources; one runaway process could affect others
- Need to manage tool version consistency manually
- No per-agent resource limits (though rarely needed for Claude Code workloads)

### Neutral

- Workspaces separated by directory (`/workspaces/agent-1`, `/workspaces/agent-2`, etc.)
- All agents use same installed toolchain versions
