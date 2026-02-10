# SSH Key Handling

This document covers how cloudcoop handles SSH keys: both host keys (verifying the VM is
who it claims to be) and identity keys (proving the user is who they claim to be). It
covers both the Go SSH library path and the native `ssh` shell-out path, and explains the
security trade-offs in the context of this application.

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

The host key flow uses `EnsureHostKeyPinned` (`internal/ssh/hostkey.go`), which wraps
the base `EnsureHostKey` with per-VM pin tracking:

1. If a pin exists for this VM (matching name, created timestamp, host, and port) and the
   known_hosts entry is present: **skip** (no ssh-keyscan needed).
2. If the pin is stale (different created timestamp or IP): remove the old known_hosts
   entry, re-scan, and update the pin.
3. If no pin exists (first connection): scan, create pin.

The base `EnsureHostKey` does the actual key fetch:

1. Removes any existing entry for this host from cloudcoop's known_hosts
2. Runs `ssh-keyscan -T 5 -t ed25519,rsa,ecdsa <host>` to fetch the current key
   (with retries for VMs that are still booting)
3. Appends the fetched key to cloudcoop's known_hosts

**Go library path** (`internal/ssh/client.go:NewClient`):

1. Call `EnsureHostKeyPinned(host, port, vm)`
2. Create a `HostKeyCallback` from the just-updated known_hosts file
3. Connect with that callback

**Shell-out path** (all call sites are consistent):

- `ConnectInteractive` (`internal/ssh/connect.go`): Calls `EnsureHostKeyPinned`, then
  passes `-o UserKnownHostsFile=<cloudcoop-known-hosts>` to `ssh`
- `auth login` (`internal/cli/auth.go`): Same pattern
- `provision logs --follow` (`internal/cli/provision_logs.go`): Same pattern
- TUI `connectToAgent` (`internal/tui/commands.go`): Same pattern

### The pinned-key model

The `EnsureHostKeyPinned` implementation pins the host key for the lifetime of a VM.
It uses a TOML pin file (`~/.config/cloudcoop/pinned_keys.toml`) that maps VM names to
their host, port, and creation timestamp. This provides MITM detection during a VM's
lifetime without false positives on VM recreation:

- **MITM detection within a VM lifetime**: Once a key is pinned, subsequent connections
  skip `ssh-keyscan` and verify against the stored key. An attacker would need to intercept
  the very first connection to a newly created VM.
- **No false positives on VM recreation**: When a VM is deleted and recreated, the creation
  timestamp changes, triggering a fresh scan and new pin.
- **No user prompt**: The user is never asked to verify a fingerprint.

The unpinned `EnsureHostKey` (used as fallback when VM identity is not available) always
re-scans, which is functionally equivalent to auto-accepting whatever key the host
presents.

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

The pinned-key model provides meaningful defense-in-depth without adding user friction.

## Recommendations

### Do not add interactive host key prompts

Prompting the user to verify a host key fingerprint for a VM they just created through an
authenticated API call adds friction without meaningful security. The user has no
independent channel to verify the fingerprint (unlike, say, a server administered by a
colleague who can read the fingerprint over the phone). The only source of truth is the VM
itself, which is the thing being verified -- a circular dependency.

## Summary

| Aspect | Go Library Path | Shell-Out Path |
|--------|----------------|----------------|
| Identity keys | Explicit discovery: agent, then key files | Inherited from user's SSH config |
| Host key source | cloudcoop known_hosts | cloudcoop known_hosts via `-o UserKnownHostsFile` |
| `EnsureHostKeyPinned` called | Yes | Yes |
| MITM protection | Pinned per VM lifetime (re-scans on VM recreation) | Same |
| User experience | Silent, no prompts | Silent, no prompts |

The core design -- pinning host keys per VM lifetime, using a separate known_hosts file --
provides MITM detection during a VM's life while avoiding false positives from VM
recreation. Interactive host key prompts would add user friction without meaningful
security benefit.
