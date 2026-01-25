# ADR-0006: Cloud vs Local Execution Environment

## Status

Accepted

## Context

Claude Code includes safety guardrails that prompt for user approval before executing potentially impactful operations: running shell commands, modifying files, making network requests, etc. While these guardrails provide safety, they create significant friction when running multiple agents autonomously.

**The core tension:** Running agents with guardrails enabled means constant interruptions—approve this command, approve that file edit, approve this Docker build. For a single agent this is manageable. For 10-12 agents working in parallel, it becomes untenable. The friction destroys the productivity benefit of using agents in the first place.

The `--dangerously-skip-permissions` flag removes these prompts, allowing agents to work autonomously. However, the security risk of running this on a personal machine is unacceptable:

- Agents can execute arbitrary commands without approval
- Agents can modify or delete any accessible files
- Agents can access credentials and sensitive data
- Mistakes or unexpected behavior have no safety net

The agents also need Docker access to:
- Build container images for services under development
- Run containers locally for testing
- Push images to container registries
- Deploy to Cloud Run for integration testing

**The problem:** On a local machine, I cannot accept the risk of disabling guardrails. But keeping guardrails enabled makes multi-agent workflows impractical due to constant approval prompts. This significantly degrades the development experience and defeats the purpose of autonomous agents.

This creates a security challenge: how do we provide autonomous execution capability while limiting blast radius if something goes wrong?

The development machine is a MacBook Air used for personal and professional work.

## Decision

Run Claude Code agents on a dedicated GCP Compute Engine VM rather than locally on the development machine.

Use GCP IAM service accounts with least-privilege permissions to control what cloud resources agents can access.

## Options Considered

### Option 1: Local Execution on MacBook (with guardrails enabled)

Run agents directly on the development machine with safety guardrails enabled.

**Pros:**
- No cloud costs
- Lower latency (no network hop)
- Simpler setup (no VM management)
- Works offline
- Safe - all operations require approval

**Cons:**
- **Constant interruptions** - every command, file edit, and Docker operation requires manual approval
- **Destroys multi-agent productivity** - 12 agents means 12x the approval prompts
- **Cannot leave agents unattended** - defeats the purpose of autonomous operation
- **Significant UX degradation** - the friction makes agents feel like a hindrance rather than a help
- Docker daemon runs as root with system-wide privileges
- Even with restricted user accounts, Docker socket access grants effective root
- Risk to personal data and credentials on the machine
- Difficult to truly isolate agents from sensitive files (~/.ssh, ~/.aws, browser data)
- No cloud IAM integration - agents would need personal credentials
- Machine resources shared with other work

### Option 1b: Local Execution on MacBook (with guardrails disabled)

Run agents directly on the development machine with `--dangerously-skip-permissions`.

**Pros:**
- No cloud costs
- Lower latency (no network hop)
- Full agent autonomy
- Works offline

**Cons:**
- **Unacceptable security risk** - agents can do anything on the machine
- Docker daemon runs as root with system-wide privileges
- Even with restricted user accounts, Docker socket access grants effective root
- Risk to personal data and credentials on the machine
- Difficult to truly isolate agents from sensitive files (~/.ssh, ~/.aws, browser data)
- No cloud IAM integration - agents would need personal credentials
- A single agent mistake could compromise the entire machine

### Option 2: Local Virtual Machine (Parallels/UTM)

Run agents inside a VM on the MacBook.

**Pros:**
- Better isolation than bare metal
- No cloud costs
- Works offline
- Full control over VM environment

**Cons:**
- Significant resource overhead on laptop (RAM, CPU, battery)
- Still no cloud IAM integration - need to manage credentials in VM
- VM has broad network access unless manually restricted
- Docker-in-VM performance penalty
- Managing VM snapshots and state adds complexity
- MacBook Air thermal constraints for sustained workloads

### Option 3: Cloud VM with Service Account (Chosen)

Run agents on a GCP Compute Engine VM with a dedicated service account.

**Pros:**
- **Complete isolation from personal machine** - no risk to local data
- **Docker runs in isolated environment** - elevated privileges contained to VM
- **Native IAM integration** - service account with least-privilege permissions
- **No credentials on VM** - uses instance metadata for authentication
- **Auditable** - all API calls logged with service account identity
- **Scalable resources** - 16 vCPUs vs laptop's limited cores
- **Disposable** - can delete and recreate VM without risk
- **Network isolation** - firewall rules control egress

**Cons:**
- Cloud costs (~$5/month stopped, ~$85/month spot when running)
- Requires internet connectivity
- Slight latency for SSH/remote access
- Need to manage VM lifecycle

### Option 4: Cloud-based Development Environments (Codespaces/Cloud Workstations)

Use managed cloud development environments.

**Pros:**
- Managed infrastructure
- Good IDE integration
- Built-in security controls

**Cons:**
- Higher cost than raw VM
- Less control over environment
- May have restrictions on long-running processes
- Not optimized for running multiple autonomous agents

## Security Analysis

### Docker Daemon Risk

The Docker daemon (`dockerd`) runs as root and provides:
- Ability to mount any host filesystem path into containers
- Ability to run privileged containers with full host access
- Access to host network namespace
- Ability to load kernel modules (with --privileged)

**On local machine:** Any process with Docker socket access can effectively become root. Restricting user accounts is insufficient - Docker socket access bypasses these restrictions.

**On cloud VM:** Docker's elevated privileges are contained within the VM. The VM itself has limited permissions via its service account. Even if an agent compromises the VM completely, it cannot:
- Access personal files (they're not on the VM)
- Use personal cloud credentials (VM uses service account)
- Exceed service account IAM permissions
- Access other cloud resources not explicitly granted

### Service Account Least-Privilege

The VM's service account can be scoped to only:
- Push/pull from specific Artifact Registry repositories
- Deploy to specific Cloud Run services
- Read/write specific GCS buckets
- Access specific Pub/Sub topics

Example minimal permissions for this project:
```
roles/artifactregistry.writer  (scoped to project)
roles/run.developer            (scoped to specific services)
roles/pubsub.publisher         (scoped to specific topics)
roles/logging.logWriter        (for observability)
```

### Audit Trail

All GCP API calls made by the service account are logged in Cloud Audit Logs, providing:
- What action was attempted
- When it occurred
- Whether it succeeded or was denied
- Which service account made the request

This audit trail doesn't exist for local Docker/CLI operations.

## The Autonomy Trade-off

The fundamental issue is that Claude Code's guardrails create a binary choice on a local machine:

| Mode | Security | Autonomy | Multi-Agent Viability |
|------|----------|----------|----------------------|
| Guardrails ON | Safe | None - constant prompts | Impractical |
| Guardrails OFF | Unacceptable risk | Full | Dangerous |

**With guardrails enabled**, every potentially impactful operation requires approval:
- "Allow execution of `git status`?" → Approve
- "Allow modification of `src/main.go`?" → Approve
- "Allow execution of `docker build`?" → Approve
- "Allow execution of `go test`?" → Approve

For a single agent on a focused task, this is tolerable. For 12 agents working on different issues simultaneously, the constant context-switching between approval prompts makes the workflow unusable. The cognitive overhead exceeds the productivity benefit.

**With guardrails disabled locally**, the agents work smoothly but with full access to:
- Personal SSH keys and cloud credentials
- Browser sessions and saved passwords
- Personal documents and photos
- System configuration and installed software

This is not a risk worth taking on a personal machine.

**The cloud VM resolves this trade-off:**

| Mode | Security | Autonomy | Multi-Agent Viability |
|------|----------|----------|----------------------|
| Cloud VM + guardrails OFF | Contained blast radius | Full | Practical |

The VM provides the isolation boundary that makes `--dangerously-skip-permissions` acceptable. Agents operate autonomously within the VM, but the VM itself is:
- Disposable (can delete and recreate)
- Isolated (no personal data present)
- Constrained (service account limits cloud access)
- Audited (all API calls logged)

## Consequences

### Positive

- Personal machine remains clean and secure
- Docker's elevated privileges are sandboxed to disposable VM
- Fine-grained IAM controls what cloud resources agents can touch
- Complete audit trail of all cloud API operations
- Can run 12 agents with 16 vCPUs without laptop thermal throttling
- Easy to add/remove permissions as project needs change
- Can delete entire VM if compromised, recreate in minutes

### Negative

- Monthly cloud costs (mitigated by spot instances and stop when idle)
- Requires internet connectivity to work
- Additional complexity of VM management
- SSH latency for interactive work (mitigated by tmux persistence)

### Neutral

- Development workflow shifts to remote-first
- Need to sync code via git (good practice anyway)
- Local machine becomes thin client for SSH access
