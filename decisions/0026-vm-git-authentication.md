# ADR-0026: VM Git Authentication

## Status

Accepted

## Context

With clone-on-demand remote setup ([ADR-0024](0024-clone-on-demand-remote-setup.md)), the VM
must be able to `git clone` and `git push` to the user's repositories. The VM needs
authenticated access to the git remote.

**Security constraints:**

- The VM should only access named repositories, not have broad access to the user's
  GitHub/GitLab account
- Agents running on the VM will push commits and create branches
- The principle of least privilege applies — grant only the permissions agents need
- Credentials on the VM are exposed to all agents (shared filesystem per
  [ADR-0001](0001-agent-execution-model.md))

**Operational constraints:**

- VMs are ephemeral — spot preemption, region changes, and resizing can destroy a VM at any time
- Re-creating git credentials on every new VM is a significant friction point
- Users may work from multiple machines (laptop, desktop)

**Minimum permissions agents need for autonomous work:**

- Read/write repository contents (push commits, create branches)
- Read issues/PRs (if using GitHub issues for context)
- Create pull requests (if agent workflow includes PR creation)

**Git hosting platforms considered:** GitHub (primary), GitLab, Bitbucket.

## Decision

Use GitHub deploy keys (read-write, per-repo) stored locally in `~/.ssh/` and copied to the
VM automatically by `cloudcoop agents sync`. Deploy key registration on GitHub is automated
via the `gh` CLI, with a manual fallback for users without `gh`.

**Key storage:**

Deploy keys are generated and stored on the user's local machine at
`~/.ssh/cloudcoop-deploy-<repo-slug>`. This is the canonical location — VMs receive copies.

**Setup flow (first time per repo):**

1. User runs `cloudcoop agents sync` from a repo directory
2. cloudcoop checks for `~/.ssh/cloudcoop-deploy-<slug>`
3. If missing, generates the key pair:
   `ssh-keygen -t ed25519 -f ~/.ssh/cloudcoop-deploy-<slug> -N ""`
4. Registers the public key as a deploy key on GitHub via `gh api`:

   ```bash
   gh api repos/{owner}/{repo}/keys \
     -f title="cloudcoop-deploy-<slug>" \
     -f key="$(cat ~/.ssh/cloudcoop-deploy-<slug>.pub)" \
     -F read_only=false
   ```

5. Copies the private key to the VM via SCP
6. Writes the SSH config entry on the VM
7. Runs pre-flight check (`git ls-remote`) to verify access
8. Proceeds with clone

The entire flow — key generation, GitHub registration, VM copy, and clone — happens in a
single `cloudcoop agents sync` invocation with no manual steps.

**Manual fallback:**

If `gh` is not installed or not authenticated, cloudcoop falls back to displaying the public
key and prompting the user to add it manually:

```text
gh CLI not available — manual setup required.

Add this public key as a deploy key at:
  https://github.com/acme/acme-backend/settings/keys

Public key:
  ssh-ed25519 AAAA... cloudcoop-deploy-acme-backend

Enable "Allow write access", then run:
  cloudcoop agents sync
```

**gh authentication requirements:**

The user's `gh` token must have the `admin:public_key` scope (classic tokens) or
repository administration permission (fine-grained tokens) to create deploy keys. Most
developers who have run `gh auth login` already have sufficient permissions.

**Subsequent syncs and new VMs:**

1. `cloudcoop agents sync` finds existing local key at `~/.ssh/cloudcoop-deploy-<slug>`
2. Copies private key to VM via SCP (idempotent — overwrites if already present)
3. Ensures SSH config entry exists on VM
4. Pre-flight check passes — no GitHub or `gh` interaction needed

**SSH config on VM (`~/.ssh/config`):**

```text
# Default GitHub access
Host github.com
  IdentityFile ~/.ssh/cloudcoop-deploy-acme-backend
  IdentitiesOnly yes
```

For multiple repos needing different keys:

```text
Host github-acme-backend
  HostName github.com
  IdentityFile ~/.ssh/cloudcoop-deploy-acme-backend
  IdentitiesOnly yes

Host github-acme-frontend
  HostName github.com
  IdentityFile ~/.ssh/cloudcoop-deploy-acme-frontend
  IdentitiesOnly yes
```

With corresponding git remote URLs using the host alias:

```text
git remote set-url origin git@github-acme-backend:acme/acme-backend.git
```

**Pre-flight check:**

`cloudcoop agents sync` runs `git ls-remote <remote-url>` on the VM before attempting to
clone. If authentication fails, it displays:

```text
Error: VM cannot access git@github.com:acme/acme-backend.git

Local deploy key exists at ~/.ssh/cloudcoop-deploy-acme-backend
but the VM was denied access.

To fix this:
  1. Verify the deploy key is added at:
     https://github.com/acme/acme-backend/settings/keys
  2. Ensure "Allow write access" is enabled
  3. Run sync again: cloudcoop agents sync
```

## Options Considered

### Option 1: Forward User's Personal SSH Key

Copy the user's `~/.ssh/id_ed25519` to the VM or use SSH agent forwarding.

**Pros:**

- Simple — works immediately if user has GitHub SSH access
- No additional setup for the user
- Access to all repos the user can access

**Cons:**

- **Too broad** — grants VM access to all repos the user can access, not just the target repos
- Personal key compromise affects all repositories
- SSH agent forwarding has security implications (agent can use key while connected)
- Violates principle of least privilege
- If VM is compromised, attacker has user's full GitHub access

### Option 2: Fine-Grained Personal Access Token (PAT)

Use a GitHub fine-grained PAT scoped to specific repositories.

**Pros:**

- Can be scoped to specific repos and permissions
- Works over HTTPS (no SSH key management)
- GitHub's newer fine-grained tokens have good permission granularity
- Can be further scoped to read-only or specific operations

**Cons:**

- Tokens expire (maximum 1 year for fine-grained PATs)
- Must be renewed periodically
- Stored as plaintext on VM (in git credential store or environment variable)
- Token rotation requires updating VM configuration
- HTTPS credential management adds complexity

### Option 3: Deploy Keys Generated on VM

Per-repository SSH keys generated on the VM and added as deploy keys on GitHub.

**Pros:**

- Private key never leaves the VM
- Scoped to a single repository
- No expiration

**Cons:**

- **VM replacement requires full re-setup** — new VM means regenerating keys and re-adding
  to GitHub for every repo
- Spot preemption, region changes, or resizing destroy the keys
- Manual GitHub interaction on every VM replacement
- For N repos on spot instances, this friction compounds quickly

### Option 4: Local Deploy Keys with Automated GitHub Registration (Chosen)

Per-repository SSH keys stored locally in `~/.ssh/` and copied to VMs by
`cloudcoop agents sync`. Deploy key registration on GitHub automated via `gh api`.

**Pros:**

- **Zero manual steps** — key generation, GitHub registration, VM copy, and clone all
  happen in one `cloudcoop agents sync` invocation
- **Scoped to a single repository** — key only grants access to one repo
- **VM-ephemeral** — new VMs get keys automatically, no interaction needed
- No expiration — deploy keys remain valid until removed
- Configurable read-only or read-write access per key
- Keys live in `~/.ssh/` — familiar location, covered by existing backup and sync workflows
- Compromise of one key doesn't affect other repositories
- Easy to audit — visible in GitHub repository settings
- Easy to revoke — remove the key from repository settings
- Multi-machine users already have strategies for syncing `~/.ssh/` (dotfiles, 1Password
  SSH agent, manual copy)
- Graceful degradation — falls back to manual instructions if `gh` is unavailable

**Cons:**

- Depends on `gh` CLI for fully automated flow (reasonable assumption for target audience;
  manual fallback available)
- `gh` token must have `admin:public_key` scope or repository administration permission
- Private key transits the network (encrypted via SSH/SCP, but leaves local machine)
- Local machine holds deploy keys for all configured repos (incremental risk is small —
  local machine already holds personal SSH keys and cloud credentials with broader access)
- One key per repository — setup scales linearly with repo count
- Not available on all git hosting platforms with identical semantics

### Option 5: GitHub App Installation Token

Create a GitHub App with minimal permissions, install it on target repos,
generate installation tokens for the VM.

**Pros:**

- Fine-grained permissions via GitHub App manifest
- Can be installed across an organization
- Tokens are short-lived (1 hour) — limited blast radius
- Audit log integration
- Can be automated via API

**Cons:**

- Significant setup complexity (create app, install, manage tokens)
- Tokens expire hourly — need refresh mechanism
- Over-engineered for most use cases
- Requires maintaining a GitHub App
- Additional infrastructure for token refresh service

## Consequences

### Positive

- Fully automated first-time setup — `cloudcoop agents sync` handles everything when `gh`
  is available
- Least-privilege access — each deploy key only grants access to one repository
- No expiration — set up once per repo, works indefinitely
- VM replacement is seamless — `cloudcoop agents sync` copies existing keys automatically
- Spot preemption recovery requires no interaction at all
- Keys stored in `~/.ssh/` — users' existing backup, sync, and permission workflows apply
- Clear security boundary — VM can only access explicitly configured repos
- Easy to audit and revoke via GitHub repository settings

### Negative

- `gh` CLI is a soft dependency for the automated path (manual fallback exists)
- Private keys are copied to VMs (mitigated: transfer is over SSH, and VM compromise
  exposes the key regardless of where it was generated)
- Local machine holds all deploy keys (mitigated: incremental risk is small given what
  else lives on the local machine)
- Scales linearly — N repos require N deploy key pairs
- GitLab and Bitbucket have different deploy key APIs (GitLab supports a similar REST
  endpoint; Bitbucket uses a different model)

### Neutral

- Security companion to [ADR-0024](0024-clone-on-demand-remote-setup.md) — sync checks
  auth before cloning and automates key distribution
- `gh` is already commonly installed by developers using GitHub — cloudcoop could check
  for it during setup and suggest installation if missing
- Users with existing VM git access (e.g., pre-configured SSH keys) can skip this setup
- The pre-flight check in `cloudcoop agents sync` provides clear guidance when auth fails
- Future: could support `glab` for GitLab automation using the same pattern
