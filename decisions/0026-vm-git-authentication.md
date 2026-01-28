# ADR-0026: VM Git Authentication

## Status

Proposed

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

**Minimum permissions agents need for autonomous work:**

- Read/write repository contents (push commits, create branches)
- Read issues/PRs (if using GitHub issues for context)
- Create pull requests (if agent workflow includes PR creation)

**Git hosting platforms considered:** GitHub (primary), GitLab, Bitbucket.

## Decision

Use GitHub deploy keys (read-write, per-repo) as the recommended authentication method for
VM git access.

**Setup flow:**

1. User generates an SSH key pair on the VM (or cloudcoop generates it during provisioning)
2. User adds the public key as a deploy key on each repository that agents will access
3. Configure deploy key with write access enabled
4. VM's SSH config maps each repo's host to the correct key

**SSH config on VM (`~/.ssh/config`):**

```text
# Default GitHub access
Host github.com
  IdentityFile ~/.ssh/cloudcoop_deploy_key
  IdentitiesOnly yes
```

For multiple repos needing different keys:

```text
Host github-backend
  HostName github.com
  IdentityFile ~/.ssh/deploy_acme_backend
  IdentitiesOnly yes

Host github-frontend
  HostName github.com
  IdentityFile ~/.ssh/deploy_acme_frontend
  IdentitiesOnly yes
```

With corresponding git remote URLs using the host alias:

```text
git remote set-url origin git@github-backend:acme/acme-backend.git
```

**Pre-flight check:**

`cloudcoop agents sync` runs `git ls-remote <remote-url>` on the VM before attempting to
clone. If authentication fails, it displays:

```text
Error: VM cannot access git@github.com:acme/acme-backend.git

To fix this:
  1. SSH to your VM: cloudcoop ssh
  2. Generate a key: ssh-keygen -t ed25519 -f ~/.ssh/cloudcoop_deploy_key -N ""
  3. Copy the public key: cat ~/.ssh/cloudcoop_deploy_key.pub
  4. Add it as a deploy key at: https://github.com/acme/acme-backend/settings/keys
  5. Enable "Allow write access"
  6. Run sync again: cloudcoop agents sync
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

### Option 3: GitHub Deploy Keys (Chosen)

Per-repository SSH keys added as deploy keys on GitHub.

**Pros:**

- **Scoped to a single repository** — key only grants access to one repo
- No expiration — deploy keys remain valid until removed
- Configurable read-only or read-write access per key
- SSH-based — standard git authentication, no credential helpers needed
- Compromise of one key doesn't affect other repositories
- Easy to audit — visible in repository settings
- Easy to revoke — remove the key from repository settings

**Cons:**

- One key per repository — setup scales linearly with repo count
- Requires repository admin access to add deploy keys
- SSH config complexity for multiple repos with different keys
- Must be set up manually (GitHub API could automate but adds complexity)
- Not available on all git hosting platforms with identical semantics

### Option 4: GitHub App Installation Token

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

- Least-privilege access — each deploy key only grants access to one repository
- No expiration — set up once, works indefinitely
- Clear security boundary — VM can only access explicitly configured repos
- Easy to audit and revoke via GitHub repository settings
- Standard SSH-based git authentication — no special tooling needed

### Negative

- Manual setup per repository (generate key, add to GitHub)
- Scales linearly — N repos require N deploy keys
- Repository admin access required to add deploy keys
- SSH config management for multi-repo setups
- GitLab and Bitbucket have different deploy key semantics

### Neutral

- Security companion to [ADR-0024](0024-clone-on-demand-remote-setup.md) — sync checks
  auth before cloning
- Future improvement: `cloudcoop` could automate deploy key creation via GitHub API
  (with user's permission)
- Users with existing VM git access (e.g., pre-configured SSH keys) can skip this setup
- The pre-flight check in `cloudcoop agents sync` provides clear guidance when auth fails
