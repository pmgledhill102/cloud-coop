# ADR-0012: Network Security for SSH Access

## Status

Accepted (Revised)

## Context

The sandbox VM requires SSH access for:

- Programmatic commands (list/create/kill tmux sessions)
- Interactive terminal attachment to agent sessions
- Potentially long-running tmux sessions (hours)

Security considerations:

- Exposing SSH to the internet increases attack surface
- Key-based authentication is strong but not infallible
- Most developers work from dynamic IPs (home ISP, mobile hotspot)
- Manual firewall management creates friction

We need to balance security with usability for this developer-focused tool.

## Decision

Support two primary approaches, with **IAP tunnel as the recommended default**:

1. **IAP Tunnel (Recommended)** - No public SSH exposure, Google identity authentication
2. **Dynamic IP Firewall (Alternative)** - For users who need direct SSH or have IAP constraints

## Options Considered

### Option 1: IAP Tunnel (Recommended)

Identity-Aware Proxy (IAP) TCP forwarding allows SSH access through Google's infrastructure without
exposing the VM to the public internet.

**How it works:**

```text
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ Workstation │────▶│ Google IAP  │────▶│  VM (no     │
│             │     │ (OAuth +    │     │  external   │
│             │     │  TCP proxy) │     │  IP)        │
└─────────────┘     └─────────────┘     └─────────────┘
```

**Usage:**

```bash
# Via gcloud (easiest)
gcloud compute ssh claude-sandbox --zone=europe-north2-a --tunnel-through-iap

# Via ssh with ProxyCommand
ssh -o ProxyCommand='gcloud compute start-iap-tunnel %h %p --zone=europe-north2-a --listen-on-stdin' claude-sandbox
```

**Pros:**

- No external IP needed on VM - zero public attack surface
- Two-factor authentication: Google identity + SSH keys
- No firewall rules to manage - access controlled via IAM
- Works from any network (no IP allowlisting needed)
- Audit logging via Cloud Audit Logs
- No additional setup for users already authenticated with gcloud

**Cons:**

- Requires `gcloud` CLI installed and authenticated
- Adds latency (~10-50ms per request due to proxy hop)
- Data transfer costs (see Cost Analysis below)
- Dependency on Google's IAP infrastructure availability

### Option 2: Dynamic IP Firewall

For users who need direct SSH access or have constraints with IAP.

**Modes:**

| Mode | Behaviour |
|------|----------|
| `auto` | Detect public IP on startup, update firewall rule |
| `manual` | User specifies IP/CIDR in config, TUI applies it |
| `disabled` | No firewall management, user handles manually |

**Pros:**

- Direct connection - no proxy latency
- No data transfer costs
- Works without gcloud CLI (standard SSH)
- Familiar model for users with static IPs

**Cons:**

- Requires external IP on VM
- Exposes SSH to (restricted) internet
- Dynamic IPs require ongoing firewall updates
- IP detection can fail (breaks access)

### Option 3: VPN / Tailscale

Use a VPN or mesh network like Tailscale.

**Pros:**

- Very secure - VM only accessible via VPN
- Works well for teams

**Cons:**

- Additional software to install and manage
- More complex setup
- Overkill for single-user sandbox

## Cost Analysis: IAP TCP Forwarding

IAP TCP forwarding is charged at **$0.01 per GB** of data transferred (both ingress and egress through the tunnel).

### Estimated Costs for SSH/tmux Usage

| Activity | Data Volume | Monthly Cost |
|----------|-------------|--------------|
| Light usage (few commands/day) | ~100 MB/month | ~$0.001 |
| Moderate usage (active development) | ~1 GB/month | ~$0.01 |
| Heavy usage (12 agents, constant interaction) | ~10 GB/month | ~$0.10 |
| File transfers via SCP (occasional) | ~50 GB/month | ~$0.50 |

**Conclusion:** For typical SSH/tmux usage, IAP costs are negligible (cents per month). Text-based
terminal sessions generate minimal data. Only significant file transfers would incur noticeable
costs.

### Cost Comparison

| Approach | Monthly Cost (typical usage) |
|----------|------------------------------|
| IAP Tunnel | ~$0.01 - $0.10 |
| External IP (for direct SSH) | ~$3.00 (static) or $0 (ephemeral) |
| VPN (e.g., Cloud VPN) | ~$36.00+ |

## Technical Considerations for tmux Sessions

### IAP with Long-Running tmux Sessions

| Consideration | Impact | Mitigation |
|---------------|--------|------------|
| **Connection stability** | IAP connections may timeout after extended idle periods | tmux handles disconnection gracefully; reconnect and reattach |
| **Latency** | 10-50ms additional latency | Imperceptible for interactive terminal use |
| **Throughput** | Limited vs direct connection | Sufficient for terminal I/O; avoid large file transfers via SSH |
| **Reconnection** | Must re-establish IAP tunnel on disconnect | Use `--tunnel-through-iap` flag; connection multiplexing helps |

### Tested Scenarios

| Scenario | IAP Performance |
|----------|-----------------|
| Interactive Claude Code session | ✓ Works well - latency not noticeable |
| Multiple tmux windows (12 agents) | ✓ Works well - single tunnel, multiple sessions |
| Detach/reattach after hours | ✓ Works well - tmux preserves state |
| Large log file viewing | ⚠️ Slower than direct - use `less` with paging |
| SCP file transfers (>100MB) | ⚠️ Noticeably slower - consider gsutil instead |

### Recommended Configuration

For optimal IAP experience with tmux:

```bash
# ~/.ssh/config
Host claude-sandbox
    HostName claude-sandbox
    ProxyCommand gcloud compute start-iap-tunnel %h %p --zone=europe-north2-a --listen-on-stdin
    User sandbox
    # Keep connection alive
    ServerAliveInterval 30
    ServerAliveCountMax 3
    # Connection multiplexing (faster subsequent connections)
    ControlMaster auto
    ControlPath ~/.ssh/sockets/%r@%h-%p
    ControlPersist 600
```

## Configuration

```toml
# ~/.config/cloudcoop/cloudcoop.toml

[network]
# Primary access method: iap | direct
access_method = "iap"

[network.iap]
# Automatically use IAP for all SSH operations
enabled = true

[network.direct]
# Dynamic IP firewall management: auto | manual | disabled
ip_allowlist_mode = "auto"
# For mode: manual
allowed_ranges = [
    "203.0.113.0/24",    # Home ISP range
    "198.51.100.50/32",  # Office static IP
]
```

## Required Permissions

### For IAP Access

Users need the IAP-secured Tunnel User role:

```bash
# Grant IAP tunnel access to a user
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="user:developer@example.com" \
  --role="roles/iap.tunnelResourceAccessor"

# Or for a group
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="group:developers@example.com" \
  --role="roles/iap.tunnelResourceAccessor"
```

The VM's firewall must allow IAP's IP range for SSH:

```bash
gcloud compute firewall-rules create allow-ssh-iap \
  --direction=INGRESS \
  --action=ALLOW \
  --rules=tcp:22 \
  --source-ranges=35.235.240.0/20 \
  --target-tags=allow-iap-ssh
```

### For Dynamic IP Firewall (if using direct SSH)

Minimal custom role (not full `compute.securityAdmin`):

```bash
gcloud iam roles create sandboxFirewallManager \
  --project=$PROJECT_ID \
  --title="Sandbox Firewall Manager" \
  --permissions="\
compute.firewalls.get,\
compute.firewalls.create,\
compute.firewalls.update"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:$SA_EMAIL" \
  --role="projects/$PROJECT_ID/roles/sandboxFirewallManager"
```

This grants permission to manage firewall rules only - not security policies, SSL certificates,
or other security resources that `compute.securityAdmin` includes.

## Consequences

### Positive

- **IAP as default** provides zero public attack surface out of the box
- No external IP required - reduced cost and complexity
- Two-factor security (Google identity + SSH keys) without extra setup
- Works from any network - no IP management for mobile developers
- Dynamic IP firewall available for users with specific requirements
- Minimal ongoing costs for typical usage patterns

### Negative

- IAP requires `gcloud` CLI - adds dependency
- Slight latency increase with IAP (~10-50ms)
- Large file transfers slower via IAP tunnel
- IAP availability dependent on Google infrastructure

### Neutral

- Both approaches supported - users can choose based on needs
- Direct SSH remains available for advanced users or specific use cases
- Cost model shifts from external IP ($3/month static) to data transfer (typically <$0.10/month)

## Related Decisions

- **ADR-0013**: SSH and Remote Execution - covers implementation details for SSH operations
- **ADR-0003**: Instance Provisioning Model - VM configuration including network setup
