# ADR-0015: SSH Testing in Sandboxed Environments

## Status

Accepted

## Context

Claude Code (and similar AI coding assistants) run in sandboxed execution environments that restrict outbound network connections. Specifically, connections to port 22 (SSH) are blocked. This creates a significant testing gap:

1. **Manual testing burden**: Every SSH-related feature requires the user to manually test functionality, breaking the development flow
2. **Reduced confidence**: Without automated testing, regressions in SSH functionality may go undetected
3. **Iteration friction**: The majority of cloudcoop features from here forward require SSH connectivity (agent sessions, tmux management, etc.)

During Gate 4 (SSH Integration Review), we discovered that `cloudcoop ssh hostname` could not be tested from within Claude Code sessions due to these network restrictions. The user had to run all SSH tests manually.

### Constraints

- Claude Code sandbox blocks port 22 outbound
- We want to maintain sandboxed execution for security (avoid unrestricted mode)
- Test VMs are ephemeral (created/destroyed frequently)
- Solution should work for both development testing and CI/CD

## Decision

**Use a non-standard SSH port (2222) for development/test VMs, with network tags for firewall management.**

This allows SSH testing from sandboxed environments while maintaining security boundaries. Production VMs can continue using port 22.

## Options Considered

### Option 1: Run in Unrestricted Mode

Disable sandbox restrictions to allow port 22 connections.

**Pros:**
- No code or infrastructure changes required
- Works immediately with existing VMs

**Cons:**
- Defeats the purpose of sandboxing
- Reduces security posture for all operations, not just SSH
- User explicitly wants to avoid this approach
- Not a pattern we want to establish for future development

**Verdict:** Rejected. Security compromise is not acceptable.

### Option 2: Local Proxy

Run a local proxy that accepts connections on an allowed port, then forwards to port 22 on the remote host.

```
Claude Code → localhost:2222 → Proxy → VM:22
```

**Pros:**
- No changes to VM configuration
- Works with any existing VM

**Cons:**
- Requires additional software (socat, ssh tunnel, or custom proxy)
- Complex setup for each testing session
- Must configure proxy with target VM's IP for each test
- Introduces additional failure points
- Doesn't work in CI/CD without similar proxy setup
- State management (starting/stopping proxy) adds complexity

**Verdict:** Too complex and fragile for regular development workflow.

### Option 3: Non-Standard SSH Port on Test VMs (Recommended)

Configure test/development VMs to run SSH on port 2222 (or another non-blocked port) in addition to or instead of port 22.

```
Claude Code → VM:2222 → sshd
```

**Pros:**
- Simple, standard SSH configuration change
- Works within existing sandbox restrictions
- No additional software or proxies required
- Firewall can use network tags for automatic rule application
- Can be automated via VM startup script
- Works in CI/CD environments
- Provides minor security benefit (reduces automated scanning)

**Cons:**
- Requires VM image/startup script modification
- Non-standard port may confuse users initially
- Requires firewall rule for port 2222
- `gcloud compute ssh` uses port 22 by default (need `-p 2222` flag)
- cloudcoop needs configurable SSH port

**Verdict:** Best balance of simplicity, security, and practicality.

### Option 4: SSH over WebSocket (Port 443)

Use websocket-based SSH that tunnels over HTTPS.

**Pros:**
- Works through almost any firewall
- Uses standard HTTPS port

**Cons:**
- Requires additional software on VM (websockify, gotty, or similar)
- More complex setup and maintenance
- Performance overhead
- Non-standard approach
- Adds debugging complexity

**Verdict:** Overkill for this use case. Adds unnecessary complexity.

### Option 5: GCP IAP Tunnel Integration

Use GCP's Identity-Aware Proxy to tunnel SSH over HTTPS.

**Pros:**
- Secure by design (no external IP needed)
- Works through firewalls
- Google-supported solution

**Cons:**
- Requires `gcloud` CLI or IAP API integration
- May not work from Claude Code sandbox (IAP itself may be blocked)
- GCP-specific, doesn't generalize to AWS/Azure
- Adds significant complexity to SSH client code

**Verdict:** Good for production security but doesn't solve sandbox testing problem and adds cloud-specific complexity.

### Option 6: Mock SSH Server for Unit Tests Only

Use a mock SSH server for unit tests, accept that integration tests require manual verification.

**Pros:**
- Clean separation of unit and integration tests
- Mock server can run on any port locally

**Cons:**
- Doesn't solve integration testing problem
- SSH-related features remain difficult to test end-to-end
- Mocks may drift from real behavior

**Verdict:** Useful complement but doesn't solve the core problem.

## Extended Analysis: Non-Standard Port for All VMs?

Should cloudcoop standardize on a non-standard SSH port (e.g., 2222) for ALL VMs, not just test VMs?

### Arguments For

1. **Security through obscurity**: Automated scanners and bots target port 22. Using a non-standard port reduces noise and opportunistic attacks.

2. **Consistent behavior**: Same configuration for dev, test, and production simplifies operations.

3. **Testing parity**: Production VMs can be tested the same way as development VMs.

### Arguments Against

1. **Limited security benefit**: Sophisticated attackers will port scan. This only stops the lowest-effort attacks.

2. **Operational friction**: Every SSH command needs `-p 2222`. Easy to forget.

3. **Tool compatibility**: Many tools assume port 22:
   - `gcloud compute ssh` defaults to 22
   - SSH config complexity increases
   - Third-party integrations may break

4. **Documentation burden**: Must document non-standard port everywhere.

5. **User confusion**: "Why doesn't `ssh vm-name` work?"

### Recommendation

**Use non-standard ports for test/development VMs only. Keep production VMs on port 22.**

Rationale:
- Test VMs are ephemeral and used in controlled environments
- Production VMs benefit from standard tooling
- The security benefit is minimal for production (proper firewall rules are more important)
- Users expect port 22 for SSH

Make the SSH port configurable in cloudcoop config so users can choose their preference.

## Implementation

### 1. VM Startup Script

Add to VM metadata startup script:

```bash
#!/bin/bash
# Add port 2222 to SSH (in addition to 22)
if ! grep -q "Port 2222" /etc/ssh/sshd_config; then
  echo "Port 2222" >> /etc/ssh/sshd_config
  systemctl restart sshd
fi
```

### 2. Firewall Rule with Network Tags

```bash
gcloud compute firewall-rules create allow-ssh-2222 \
  --project=cloud-coop-dev \
  --direction=INGRESS \
  --priority=1000 \
  --network=default \
  --action=ALLOW \
  --rules=tcp:2222 \
  --target-tags=cloudcoop-dev \
  --description="Allow SSH on port 2222 for cloudcoop development VMs"
```

Tag test VMs with `cloudcoop-dev`:

```bash
gcloud compute instances add-tags $VM_NAME \
  --tags=cloudcoop-dev \
  --zone=$ZONE
```

### 3. cloudcoop Configuration

Add SSH port to config file (`~/.config/cloudcoop/cloudcoop.toml`):

```toml
[ssh]
port = 2222  # Default: 22
user = ""    # Default: current user
```

### 4. Code Changes

Update `internal/ssh/client.go`:

```go
type Config struct {
    Host    string
    User    string
    Port    int  // Now configurable, not hardcoded to 22
    Timeout time.Duration
}
```

Update CLI to read port from config.

## Consequences

### Positive

- SSH features can be tested from within Claude Code sessions
- Automated testing in CI/CD becomes possible
- Development iteration speed increases significantly
- Maintains security sandbox for all other operations
- Network tags simplify firewall management across ephemeral VMs

### Negative

- Adds configuration complexity (SSH port setting)
- Test VMs differ slightly from production configuration
- Must document port 2222 requirement for development

### Neutral

- Minor refactoring to make SSH port configurable
- VM provisioning documentation needs update
- Users can choose to use non-standard ports in production if they prefer

## Related Decisions

- ADR-0013: SSH and Remote Execution (establishes Go SSH library usage)
- ADR-0012: Dynamic IP Firewall (related firewall management patterns)

## References

- [Claude Code sandbox restrictions](https://docs.anthropic.com/claude-code/sandbox)
- [GCP IAP TCP forwarding](https://cloud.google.com/iap/docs/using-tcp-forwarding)
- [SSH hardening best practices](https://www.ssh.com/academy/ssh/port)
