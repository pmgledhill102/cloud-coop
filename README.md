# cloudcoop

A terminal UI for managing sandboxed AI coding agents on cloud VMs.

## Overview

cloudcoop provisions and manages cloud VMs configured as secure sandboxes for running multiple AI coding agents (Claude Code, Aider, Gemini CLI, etc.). Each agent runs in its own tmux session with full development tooling.

**Key features:**
- TUI for VM lifecycle management (start, stop, resize)
- Agent-agnostic: supports Claude Code, Aider, Gemini CLI, and others
- Cloud-agnostic: GCP first, AWS and Azure planned
- Automatic IP-based firewall rules (optional)
- Cost-optimized: spot instances, stop when idle

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  Your Workstation                                               │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  cloudcoop TUI                                            │  │
│  │  • VM status, start/stop/resize                           │  │
│  │  • Agent session management                               │  │
│  │  • IP-based firewall updates                              │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────┬───────────────────────────────┘
                                  │ SSH + Cloud SDK
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│  Cloud VM (GCP/AWS/Azure)                                       │
│  c4a-highcpu-16 (ARM) · 50GB SSD · Spot instance               │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  tmux: agents                                             │  │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐     ┌─────────┐     │  │
│  │  │ agent-1 │ │ agent-2 │ │ agent-3 │ ... │agent-12 │     │  │
│  │  │ claude  │ │ claude  │ │  aider  │     │ claude  │     │  │
│  │  └─────────┘ └─────────┘ └─────────┘     └─────────┘     │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
│  Shared: Docker · Dev tooling · Git credentials                 │
│  Service account: Minimal GCP permissions                       │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# Download latest release
curl -fsSL https://github.com/yourorg/cloudcoop/releases/latest/download/cloudcoop-$(uname -s)-$(uname -m) -o cloudcoop
chmod +x cloudcoop

# First run: setup wizard guides you through GCP project setup
./cloudcoop

# Or with explicit config
./cloudcoop --project=my-gcp-project --zone=europe-north2-a
```

## TUI Interface

```
┌─────────────────────────────────────────────────────────────────┐
│  cloudcoop                                           v0.1.0     │
├─────────────────────────────────────────────────────────────────┤
│  Cloud: GCP (europe-north2-a)                                   │
│  VM: claude-sandbox          ● Running                          │
│  Type: c4a-highcpu-16        Cost: ~$0.12/hr (spot)            │
│  IP Firewall: ✓ 203.0.113.42/32                                │
│                                                                 │
│  Agents (8 active)                                              │
│  [1] agent-1   claude   ● issue-142       2h 30m               │
│  [2] agent-2   claude   ● issue-143       2h 28m               │
│  [3] agent-3   aider    ● quick-fix       0h 45m               │
│  [4] agent-4   claude   ○ idle            -                    │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  [S]tart  s[T]op  [R]esize  [A]dd  [K]ill  [C]onnect  [Q]uit   │
└─────────────────────────────────────────────────────────────────┘
```

## Configuration

```yaml
# ~/.config/cloudcoop/config.yaml
cloud:
  provider: gcp
  gcp:
    project: my-project
    zone: europe-north2-a

vm:
  name: claude-sandbox
  machine_type: arm-16cpu-32gb  # Normalized, mapped per cloud
  disk_size_gb: 50
  spot: true

network:
  ip_allowlist:
    mode: auto  # auto | manual | disabled

agents:
  default: claude
```

## Agent Support

| Agent | Autonomous Mode | Session Persistence |
|-------|-----------------|---------------------|
| Claude Code | `--dangerously-skip-permissions` | `--continue`, `--resume` |
| Aider | `--yes-always` | Stateless |
| Gemini CLI | TBD | TBD |
| GitHub Copilot | Limited | OAuth-based |

## Cost

For c4a-highcpu-16 in europe-north2 (Stockholm):

| Usage | Monthly Cost |
|-------|-------------|
| Spot 24/7 | ~$85 |
| Spot 8h/day | ~$30 |
| Stopped (disk only) | ~$5 |

## Documentation

- [Architecture Decisions](decisions/README.md) - ADRs explaining design choices
- [TUI Requirements](docs/TUI-REQUIREMENTS.md) - Detailed TUI specification
- [Development Environment](docs/DEVELOPMENT-ENVIRONMENT.md) - Contributing guide
- [Setup Flow](docs/SETUP-FLOW.md) - First-run experience

## License

Apache 2.0
