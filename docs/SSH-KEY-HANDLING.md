# SSH Key Handling

This document covers how cloudcoop handles SSH keys: both host keys (verifying the VM is
who it claims to be) and identity keys (proving the user is who they claim to be). It
covers both the Go SSH library path and the native `ssh` shell-out path, explains the
security trade-offs in the context of this application, and identifies current
inconsistencies.

## Two SSH Channels

cloudcoop uses SSH in two fundamentally different ways (ADR-0013):

| Channel | Implementation | Used For |
|---------|---------------|----------|
| **Programmatic** | Go `x/crypto/ssh` library | Listing agents, creating/killing tmux windows, checking provisioning status, running commands |
| **Interactive** | Shell out to `ssh` binary | Connecting to agent tmux sessions, auth login, following provision logs |

Both channels need to solve the same two problems: authenticating the user to the VM
(identity keys) and verifying the VM's identity (host keys). They solve these problems
differently, and the differences matter.

## Identity Keys (User Authentication)

### How the user proves their identity to the VM

**Go library path** (`internal/ssh/client.go`):

1. Check `SSH_AUTH_SOCK` for an SSH agent. If the agent has keys, use it (preferred).
2. Fall back to key files in order: `id_ed25519`, `id_rsa`, `id_ecdsa`,
   `google_compute_engine` (GCP-specific).
3. RSA keys are automatically upgraded to `rsa-sha2-512`/`rsa-sha2-256` algorithms,
   because modern OpenSSH servers disable the legacy `ssh-rsa` (SHA-1) algorithm.
4. If no auth methods are found, the connection fails with a clear error.

**Shell-out path**:

The native `ssh` command handles authentication itself using the user's SSH config
(`~/.ssh/config`), SSH agent, and default key files. cloudcoop passes no identity-related
flags to `ssh` -- it relies entirely on the user's existing SSH setup.

### What this means in practice

For most users, both paths "just work" because either their SSH agent is running or they
have standard key files. The Go library path is more predictable (explicit key discovery
order) while the shell-out path inherits whatever the user's SSH config does (which may
include `IdentityFile` directives, `ProxyJump`, etc.).

The one subtle difference: the Go library path does not read `~/.ssh/config`. If a user has
configured `IdentityFile /path/to/custom/key` for their VM in SSH config, the shell-out
path will use it but the Go library path will not. In practice this rarely causes problems
because most users either use an SSH agent or standard key file names.

## Host Keys (VM Verification)

This is where things get interesting and where the architecture-specific trade-offs matter.

### The textbook TOFU model

Standard SSH Trust On First Use works like this:

1. First connection: SSH shows the host key fingerprint, asks the user to accept/reject.
2. If accepted, the key is stored in `~/.ssh/known_hosts`.
3. Subsequent connections: SSH verifies the key matches. If it doesn't, the connection is
   refused with a scary warning about a possible MITM attack.

This model assumes hosts are long-lived and their keys rarely change. When a key does
change, it's a strong signal that something is wrong.

### Why textbook TOFU is a poor fit for cloudcoop

cloudcoop manages ephemeral cloud VMs. These VMs are routinely:

- **Deleted and recreated** (new VM, new host key, possibly same IP)
- **Stopped and started** (same key, possibly new external IP)
- **Given recycled IPs** (different VM, different key, same IP as a previous VM)

In this environment, host key changes are the normal case, not the exceptional case. A
strict TOFU model would cause constant "HOST KEY HAS CHANGED" errors that train users to
blindly accept key changes -- the opposite of the security outcome TOFU is designed to
achieve.

Furthermore, the threat model is specific: the user just provisioned this VM themselves
through an authenticated GCP API call. The channel used to create the VM (GCP API over TLS
with ADC authentication) is separate from and arguably stronger than the channel used to
connect to it (SSH). A MITM on the SSH connection would need to intercept traffic between
the user's machine and a VM whose IP address was just returned by a GCP API call moments
earlier.

### What cloudcoop actually does

cloudcoop maintains its own known_hosts file at `~/.config/cloudcoop/known_hosts`, separate
from `~/.ssh/known_hosts`. This separation is important: it prevents ephemeral VM keys from
polluting the user's personal known_hosts file.

The host key flow in the **Go library path** (`internal/ssh/client.go:NewClient`):

1. Call `EnsureHostKey(host, port)` which:
   a. Removes any existing entry for this host from cloudcoop's known_hosts
   b. Runs `ssh-keyscan -t ed25519,rsa,ecdsa <host>` to fetch the current key
   c. Appends the fetched key to cloudcoop's known_hosts
2. Create a `HostKeyCallback` from the just-updated known_hosts file
3. Connect with that callback

The host key flow in the **shell-out path** (varies by call site):

- `ConnectInteractive` (`internal/ssh/connect.go`): Calls `EnsureHostKey`, then passes
  `-o UserKnownHostsFile=<cloudcoop-known-hosts>` to `ssh`
- `auth login` (`internal/cli/auth.go`): Same -- `EnsureHostKey` + `UserKnownHostsFile`
- `provision logs --follow` (`internal/cli/provision_logs.go`): **Does not call
  `EnsureHostKey`**, does not pass `UserKnownHostsFile` -- uses the user's default SSH
  known_hosts
- TUI `connectToAgent` (`internal/tui/commands.go`): **Does not call `EnsureHostKey`**,
  does not pass `UserKnownHostsFile` -- uses the user's default SSH known_hosts

### The auto-accept pattern

The current `EnsureHostKey` implementation always removes the old key and fetches the new
one. Every connection starts fresh. This is functionally equivalent to auto-accepting
whatever key the host presents -- there is no persistence across connections and no
detection of key changes.

This means:

- **No MITM detection**: If the host key changes between connections (because of an
  attacker, not because of VM recreation), cloudcoop will silently accept the new key.
- **No user prompt**: The user is never asked to verify a fingerprint.
- **Same security as `InsecureIgnoreHostKey`**: In terms of MITM protection, the current
  approach provides the same guarantees. The only difference is that the key is briefly
  verified between the `ssh-keyscan` and the `ssh.Dial` call (preventing a key-swap in
  that small window).

### Is this actually a problem?

For this application, probably not, for several reasons:

1. **The provisioning channel is stronger than the SSH channel.** The user creates the VM
   through an authenticated GCP API call. The VM's IP is returned by that API. An attacker
   who could MITM the SSH connection would also need to either compromise DNS/routing to
   redirect traffic from that IP, or compromise the GCP API response itself.

2. **The threat window is narrow.** The user provisions a VM and connects to it. The
   connection target is a fresh IP returned by the cloud provider moments earlier. This is
   not a "connect to a hostname that might resolve differently" scenario.

3. **User frustration from false positives outweighs MITM risk.** Strict host key checking
   for ephemeral VMs produces constant warnings that teach users to ignore them. A security
   control that users routinely bypass is worse than no control, because it creates a false
   sense of security.

4. **The VM is a sandbox, not a secrets vault.** The VM runs AI coding agents with scoped
   deploy keys and API tokens. A MITM could observe agent API keys and repository access.
   This is a real risk but a limited one -- the keys are scoped, rotatable, and the
   attacker gains access to a sandbox, not to production infrastructure.

5. **The attack requires network-level access.** SSH MITM requires the attacker to be on
   the network path between the user and the VM. For GCP VMs with IP-scoped firewall rules,
   this means compromising the user's network or the cloud provider's network -- both of
   which imply much larger problems than SSH key verification.

That said, the current approach could be improved for defense-in-depth without adding user
friction. See [Recommendations](#recommendations) below.

## Current Inconsistencies

There are two shell-out paths that bypass cloudcoop's managed known_hosts entirely:

### 1. TUI `connectToAgent` (`internal/tui/commands.go:238-250`)

```go
c := exec.Command("ssh", "-t", "-p", fmt.Sprintf("%d", port),
    fmt.Sprintf("%s@%s", sshUser, ip), tmuxCmd)
```

This does not call `EnsureHostKey` and does not pass `-o UserKnownHostsFile`. It uses the
user's default `~/.ssh/known_hosts`. If the VM's key is not in the user's personal
known_hosts, this will trigger OpenSSH's interactive "Are you sure you want to continue
connecting?" prompt, or fail with a host key mismatch error.

This is inconsistent with the CLI `connect` path (`internal/cli/connect.go`), which calls
`ConnectInteractive` and properly uses cloudcoop's managed known_hosts.

### 2. Provision logs follow mode (`internal/cli/provision_logs.go:103-121`)

```go
sshArgs := []string{
    "-t",
    "-p", fmt.Sprintf("%d", port),
    fmt.Sprintf("%s@%s", sshUser, ip),
    remoteCmd,
}
```

Same problem -- no `EnsureHostKey`, no `UserKnownHostsFile`. The non-follow mode of the
same command uses the Go SSH library path (which does handle host keys correctly).

### Impact

These inconsistencies mean:

- Users who only use the CLI (`cloudcoop agents connect`) will have a consistent experience
  -- cloudcoop manages host keys silently.
- Users who use the TUI connect feature or `provision logs --follow` may see unexpected
  "The authenticity of host ... can't be established" prompts, or "REMOTE HOST
  IDENTIFICATION HAS CHANGED" errors after VM recreation.
- The user's `~/.ssh/known_hosts` gets polluted with ephemeral VM keys, which is exactly
  what the separate cloudcoop known_hosts was designed to prevent.

## Recommendations

### Fix the inconsistencies (straightforward)

Both `tui/commands.go:connectToAgent` and `cli/provision_logs.go` (follow mode) should call
`EnsureHostKey` and pass `-o UserKnownHostsFile` to `ssh`, matching what
`ConnectInteractive` and `auth login` already do.

This is a clear bug fix -- two paths are missing host key handling that the other paths
already implement correctly.

### Consider pinning host keys per VM (optional improvement)

The current `EnsureHostKey` could be enhanced to pin the host key for the lifetime of a VM:

1. On first connection to a VM, fetch and store the key (keyed by VM name, not just IP).
2. On subsequent connections, verify the stored key matches -- but only if the VM hasn't
   been recreated since the key was stored.
3. On VM deletion, clear the stored key.

This would detect MITM during the life of a single VM without triggering false positives on
VM recreation. The VM name and creation timestamp (available from the cloud API) provide the
signal for when a "new" VM justifies accepting a new key.

This is a defense-in-depth measure. It adds meaningful protection against a specific attack
(MITM during the life of a running VM) without the user friction that strict TOFU causes
for ephemeral infrastructure. But it's not urgent given the threat model discussed above.

### Do not add interactive host key prompts

Prompting the user to verify a host key fingerprint for a VM they just created through an
authenticated API call adds friction without meaningful security. The user has no
independent channel to verify the fingerprint (unlike, say, a server administered by a
colleague who can read the fingerprint over the phone). The only source of truth is the VM
itself, which is the thing being verified -- a circular dependency.

## Summary

| Aspect | Go Library Path | Shell-Out Path (correct) | Shell-Out Path (inconsistent) |
|--------|----------------|-------------------------|------------------------------|
| Identity keys | Explicit discovery: agent, then key files | Inherited from user's SSH config | Same |
| Host key source | cloudcoop known_hosts | cloudcoop known_hosts via `-o UserKnownHostsFile` | User's `~/.ssh/known_hosts` (bug) |
| `EnsureHostKey` called | Yes | Yes | No (bug) |
| MITM protection | Auto-accept (re-scans each time) | Auto-accept (re-scans each time) | Whatever OpenSSH does by default |
| User experience | Silent, no prompts | Silent, no prompts | May prompt or error on key mismatch |

The core design -- auto-accepting host keys for cloudcoop-managed VMs, using a separate
known_hosts file -- is sound for this application's threat model. The two inconsistent
shell-out paths should be fixed to match the existing correct paths. Interactive host key
prompts would add user friction without meaningful security benefit.
