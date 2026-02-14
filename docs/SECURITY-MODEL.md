# Security Model

This document describes cloudcoop's architectural security model: the trust boundaries between
components, the reasoning behind the privilege structure, and the known limitations.

For operational security guidance (configuration, incident response, compliance), see
[SECURITY.md](SECURITY.md).

## Component Trust Model

cloudcoop has three components with fundamentally different trust profiles:

| Component | Trust Level | Why |
|-----------|-------------|-----|
| cloudcoop binary (local) | High trust, high privilege | Deterministic, open-source, reviewable. Runs on the user's machine with their existing credentials. |
| Cloud VM | Medium trust, constrained | Ephemeral infrastructure managed by cloudcoop. Limited IAM permissions via dedicated service account. |
| AI agents (on VM) | Low trust, sandboxed | Non-deterministic. Execute arbitrary code. Constrained by VM boundary and scoped permissions. |

The system architecture exists to give the low-trust components (agents) enough access to do
useful work while preventing them from affecting anything beyond their designated scope.

## The Privilege Asymmetry

The local binary has broad access to the user's infrastructure. The agents have narrow,
scoped access. This asymmetry is intentional and central to the security model.

### What the local binary can access

- **GCP project**: Create, delete, stop, and resize VMs. Create and modify firewall rules for
  SSH access.
- **GitHub**: Register deploy keys on repositories via the `gh` CLI.
- **VM**: Full SSH access. Execute commands, copy files, manage tmux sessions.
- **Local filesystem**: Generate SSH key pairs in `~/.ssh/`. Read git worktree state and
  remote URLs.

### Why this is acceptable

The local binary is **deterministic and reviewable**. Its behaviour is defined by open-source
code that can be audited before execution. It does not make autonomous decisions or generate
novel actions.

Specific mitigations:

- **User-initiated actions only.** cloudcoop has no background daemon, no cron job, no
  auto-update mechanism. Every action is triggered by the user through the TUI or CLI.
- **No credential storage.** cloudcoop does not store secrets. It uses credentials managed by
  existing tools: gcloud ADC (user's GCP identity), gh auth (user's GitHub identity), and
  ssh-agent (user's SSH keys).
- **No privilege elevation.** The binary runs with the user's existing permissions. It does
  not request sudo, setuid, or additional IAM roles beyond what the user already has.
- **Deterministic execution.** Given the same inputs and configuration, the binary produces
  the same API calls and SSH commands. There is no LLM or generative component in the local
  binary itself.

### How agents differ

Agents are **non-deterministic and autonomous**. An AI coding agent generates novel code,
runs arbitrary commands, and makes decisions that cannot be predicted from its inputs. An agent
may run for hours with no human oversight.

The entire system architecture (VM isolation, scoped IAM, deploy keys, network rules) exists
to constrain what agents can do when they act unexpectedly.

## Security Boundaries

```text
                   Local Machine
                  +-----------------------------------------+
                  |  cloudcoop binary                       |
                  |    |                                    |
                  |    +-- gcloud ADC (GCP project access)  |
                  |    +-- gh auth (GitHub API access)      |
                  |    +-- ssh-agent (SSH key access)       |
                  +----|----|-------------------------------+
                       |    |
          GCP API -----+    +---- SSH (host key verified)
                       |              |
                  +----|--------------|---------+
                  |  Cloud VM                   |
                  |    Service account:          |
                  |      logging + monitoring    |
                  |      secret access (opt.)    |
                  |      NO compute/iam/storage  |
                  |                              |
                  |    +-- Agent 1 (tmux window) |
                  |    +-- Agent 2 (tmux window) |
                  |    +-- Agent 3 (tmux window) |
                  |                              |
                  |    Deploy keys:              |
                  |      repo-a (read-write)     |
                  |      repo-b (read-write)     |
                  +------------------------------+
                       |               |
          GitHub SSH --+    Outbound --+-- Agent APIs
          (per-repo)                   +-- Package registries
                                       +-- etc.
```

### Boundary details

**Local machine to GCP.** Authentication via gcloud Application Default Credentials (ADC).
The binary operates within a single GCP project configured by the user. Actions are scoped
to Compute Engine (VMs, firewall rules) and optionally Secret Manager.

**Local machine to VM.** SSH with host key verification. cloudcoop maintains its own
known_hosts file at `~/.config/cloudcoop/known_hosts`, separate from `~/.ssh/known_hosts`,
to avoid polluting the user's file with ephemeral VM keys. Host keys are verified on first
connection and checked on subsequent connections.

**Local machine to GitHub.** The `gh` CLI is used to register deploy keys on repositories.
This requires the user's `gh auth` session, which is managed by `gh` itself. cloudcoop uses
it transiently during `agents sync` and does not cache or store the token.

**VM to GitHub.** Deploy keys, generated locally and copied to the VM, grant read-write
access to a single repository each ([ADR-0026](../decisions/0026-vm-git-authentication.md)).
A deploy key for repo-a cannot access repo-b. Keys are stored at
`~/.ssh/cloudcoop-deploy-<slug>` on the VM with corresponding SSH config entries.

**VM to GCP.** The VM runs under a dedicated service account with minimal IAM roles:
logging, monitoring, and optionally Secret Manager access. The service account explicitly
lacks compute, IAM, and storage roles — it cannot create VMs, modify permissions, or access
arbitrary buckets. This is enforced in code: VM creation fails without a configured service
account ([ADR-0020](../decisions/0020-vm-service-account-requirement.md)).

**VM to internet.** Outbound (egress) traffic is allowed because agents need access to APIs,
package registries, and other development services. Inbound (ingress) traffic is denied by
default. SSH access is provided through IAP tunneling (no public IP required) or through a
dynamically managed firewall rule scoped to the user's IP
([ADR-0012](../decisions/0012-dynamic-ip-firewall.md)).

**Agent to agent.** Agents run as tmux windows under the same user (`sandbox`) on a shared
filesystem. Isolation between agents is by directory convention, not by OS-level enforcement.
See [Known Limitations](#known-limitations) for the implications.

## What Agents Can Do

Within the VM boundary, agents have these capabilities:

- Read and write files within their worktree (convention, not enforced)
- Execute arbitrary code and shell commands on the VM
- Push to a single git repository via the deploy key configured for their worktree
- Make outbound API calls (egress is allowed)
- Run Docker containers (Docker is installed for development work)
- Access API keys from environment variables (sourced from Secret Manager or SSH forwarding)
- Use passwordless sudo on the VM

## What Agents Cannot Do

The VM boundary and IAM configuration prevent agents from:

- Accessing other GCP projects or resources (service account has no compute, IAM, or storage
  roles)
- Creating or modifying cloud infrastructure (no `compute.admin` or equivalent)
- Accessing repositories other than the one their deploy key permits
- Accepting inbound network connections (firewall blocks ingress; IAP or IP-scoped rules
  control SSH)
- Modifying IAM permissions or impersonating other service accounts
- Accessing GCS buckets or other storage beyond what the scoped service account permits

## Known Limitations

The current security model is designed for a single developer running their own agents on
their own VM. Several constraints are acceptable for this use case but would need hardening
for multi-tenant or shared-infrastructure scenarios.

**Shared filesystem between agents.** All agents run as the `sandbox` user on the same VM.
One agent can read or modify another agent's worktree. There is no OS-level isolation (no
separate users, no namespaces, no containers)
([ADR-0001](../decisions/0001-agent-execution-model.md)).

**Passwordless sudo.** Agents run as `sandbox` with passwordless sudo, meaning they
effectively have root on the VM. This is acceptable because the VM itself is the trust
boundary, not the user account.

**No resource limits between agents.** There are no per-agent CPU, memory, or disk quotas.
A runaway agent can starve others. For the current use case (developer's own agents), this
is a tolerable operational risk, not a security risk.

**API key exposure.** API keys (e.g., ANTHROPIC_API_KEY) are available in environment
variables on the VM. Any process on the VM can read them. Mitigation: use scoped tokens
where possible, rotate keys regularly, and use separate keys for sandbox vs. production
([ADR-0009](../decisions/0009-api-key-management.md)).

**Deploy key scope is per-repo, not per-agent.** All agents on the same VM share the same
deploy keys. An agent working on repo-a could theoretically push to repo-b if both keys are
present on the VM. Mitigation: this is scoped to repositories the user has explicitly
configured, not the user's entire GitHub account.

## Open-Source as a Security Control

The open-source nature of cloudcoop is a meaningful security property, not just a licensing
choice. It addresses the core question: why should a user trust a binary with access to their
GCP project, GitHub account, and SSH keys?

- **Auditability.** Users can read the source code and verify exactly what API calls the
  binary makes before running it.
- **Community review.** Vulnerabilities are found faster when anyone can inspect the code.
- **No hidden behaviour.** There are no obfuscated network calls, telemetry, or data
  exfiltration paths. The binary's network interactions are limited to GCP APIs, SSH
  connections, and `gh` CLI invocations.
- **Reproducible builds.** Go produces deterministic binaries from source. Users can build
  from source and verify the result matches distributed binaries.
- **Readable provisioning.** The VM provisioning script is a readable bash script, not a
  binary blob. Users can inspect what gets installed and configured on the VM.

Compare with the alternative: a closed-source VM management tool with identical permissions
would require the user to blindly trust that it doesn't misuse GCP credentials, SSH keys,
or GitHub access. Open source eliminates that requirement.

## Credential Inventory

Every credential cloudcoop touches, where it lives, and what it grants:

| Credential | Stored At | Managed By | Scope | Used By |
|------------|-----------|------------|-------|---------|
| GCP ADC | gcloud config dir | `gcloud auth` | GCP project (user's identity) | Local binary |
| SSH key pair | `~/.ssh/` | User / ssh-agent | VM access | Local binary |
| Deploy keys | `~/.ssh/cloudcoop-deploy-*` | cloudcoop (generated), GitHub (registered) | Single repo read-write | VM (copied from local) |
| gh token | gh config dir | `gh auth` | GitHub account (deploy key registration) | Local binary (transient) |
| VM service account | GCP IAM | GCP | Minimal roles (logging, monitoring) | VM |
| API key (e.g., Anthropic) | Secret Manager or env var | User | Specific API service | Agents on VM |

cloudcoop itself stores no secrets. It generates deploy key pairs (stored in `~/.ssh/`) and
copies them to VMs. All other credentials are managed by their respective tools and accessed
transiently.

## ADR Cross-References

Security-relevant architecture decisions and their key security implications:

| ADR | Decision | Security Implication |
|-----|----------|---------------------|
| [ADR-0001](../decisions/0001-agent-execution-model.md) | Agents in tmux, not Docker | Simplicity over per-agent isolation; VM is the trust boundary |
| [ADR-0009](../decisions/0009-api-key-management.md) | Tiered API key management | OAuth preferred, Secret Manager for headless; raw keys never on VM disk |
| [ADR-0012](../decisions/0012-dynamic-ip-firewall.md) | IAP tunnel preferred over public SSH | Zero public attack surface by default; direct SSH available as alternative |
| [ADR-0013](../decisions/0013-ssh-remote-execution.md) | Go SSH library + native ssh hybrid | Host key verification enforced; separate known_hosts for ephemeral VMs |
| [ADR-0020](../decisions/0020-vm-service-account-requirement.md) | Dedicated service account required | VM creation fails without explicit service account; prevents default Editor permissions |
| [ADR-0026](../decisions/0026-vm-git-authentication.md) | Per-repo deploy keys, local storage | Scoped to single repository; automated registration via gh; VM replacement is seamless |
