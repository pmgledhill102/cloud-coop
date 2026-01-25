# First-Run Setup Flow

This document describes the user experience when running cloudcoop for the first time.

## Overview

The TUI guides users through setup interactively, creating resources as needed. No manual pre-configuration required.

## Setup Stages

```
┌─────────────────────────────────────────────────────────────────┐
│  Stage 1: Prerequisites                                         │
│  • Check for gcloud CLI (for initial auth)                     │
│  • Check for SSH key                                            │
│  • Check for active GCP authentication                          │
├─────────────────────────────────────────────────────────────────┤
│  Stage 2: Project Configuration                                 │
│  • Select or create GCP project                                 │
│  • Enable required APIs                                         │
│  • Set default region/zone                                      │
├─────────────────────────────────────────────────────────────────┤
│  Stage 3: IAM Setup                                             │
│  • Create service account                                       │
│  • Grant minimal permissions                                    │
│  • Create custom firewall role (if IP allowlist enabled)        │
├─────────────────────────────────────────────────────────────────┤
│  Stage 4: VM Creation                                           │
│  • Create VM with chosen machine type                           │
│  • Configure spot instance settings                             │
│  • Wait for VM to be ready                                      │
├─────────────────────────────────────────────────────────────────┤
│  Stage 5: VM Provisioning                                       │
│  • SSH into VM                                                  │
│  • Install development tooling                                  │
│  • Install configured agents (Claude Code, etc.)                │
│  • Set up tmux                                                  │
├─────────────────────────────────────────────────────────────────┤
│  Stage 6: Agent Authentication                                  │
│  • Authenticate each configured agent                           │
│  • Claude: `claude auth login` (OAuth)                         │
│  • Store tokens securely                                        │
└─────────────────────────────────────────────────────────────────┘
```

## Detailed Flow

### Stage 1: Prerequisites

```
┌─────────────────────────────────────────────────────────────────┐
│  cloudcoop - First Run Setup                                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Checking prerequisites...                                      │
│                                                                 │
│  [✓] SSH key found (~/.ssh/id_ed25519)                         │
│  [✓] gcloud CLI installed                                       │
│  [✗] GCP authentication                                         │
│                                                                 │
│  You need to authenticate with GCP. Run:                        │
│                                                                 │
│    gcloud auth login                                            │
│    gcloud auth application-default login                        │
│                                                                 │
│  Press [Enter] after completing authentication...               │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Checks:**
- SSH key exists (`~/.ssh/id_ed25519` or `~/.ssh/id_rsa`)
- gcloud CLI installed and in PATH
- Active GCP authentication (for initial setup only - TUI uses SDK after)

### Stage 2: Project Configuration

```
┌─────────────────────────────────────────────────────────────────┐
│  cloudcoop - Project Setup                                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Select a GCP project:                                          │
│                                                                 │
│  > my-sandbox-project                                           │
│    other-project-123                                            │
│    [Create new project]                                         │
│                                                                 │
│  ↑/↓: Navigate  Enter: Select  q: Quit                         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

If creating new project:
```
┌─────────────────────────────────────────────────────────────────┐
│  Create New Project                                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Project ID: cloudcoop-sandbox-[random]                         │
│  Billing Account: My Billing Account                            │
│                                                                 │
│  This will:                                                     │
│  • Create project 'cloudcoop-sandbox-abc123'                    │
│  • Link to billing account                                      │
│  • Enable Compute Engine API                                    │
│  • Enable Secret Manager API                                    │
│                                                                 │
│  [Create] [Cancel]                                              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Stage 3: IAM Setup

```
┌─────────────────────────────────────────────────────────────────┐
│  cloudcoop - IAM Setup                                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Creating service account and permissions...                    │
│                                                                 │
│  [✓] Service account: cloudcoop@project.iam.gserviceaccount.com│
│  [✓] Role: Compute Admin (project)                             │
│  [✓] Role: Secret Manager Admin (project)                       │
│  [✓] Role: Service Account User (project)                       │
│  [✓] Custom role: Firewall Manager (for IP allowlist)          │
│                                                                 │
│  These permissions allow cloudcoop to manage VMs and secrets    │
│  within this project only.                                      │
│                                                                 │
│  [Continue]                                                     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Stage 4: VM Creation

```
┌─────────────────────────────────────────────────────────────────┐
│  cloudcoop - VM Configuration                                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Region: europe-north2 (Stockholm)                              │
│                                                                 │
│  Machine Type:                                                  │
│    [ ] arm-8cpu-16gb   (~$0.06/hr spot)   Light workloads      │
│    [●] arm-16cpu-32gb  (~$0.12/hr spot)   Recommended          │
│    [ ] arm-32cpu-64gb  (~$0.24/hr spot)   Heavy builds         │
│                                                                 │
│  Disk Size: 50 GB (SSD)                                        │
│                                                                 │
│  [●] Use spot instance (70% cheaper, may be preempted)         │
│  [ ] Use on-demand instance (guaranteed availability)           │
│                                                                 │
│  Estimated cost: ~$5/month stopped, ~$85/month running 24/7    │
│                                                                 │
│  [Create VM]                                                    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

```
┌─────────────────────────────────────────────────────────────────┐
│  Creating VM...                                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  [✓] Created VM: claude-sandbox                                 │
│  [✓] Attached service account                                   │
│  [✓] Configured spot instance with STOP on preemption          │
│  [░░░░░░░░░░░░░░░░░░░] Waiting for VM to start...              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Stage 5: VM Provisioning

```
┌─────────────────────────────────────────────────────────────────┐
│  cloudcoop - Installing Tools                                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Provisioning VM with development tools...                      │
│  (This takes 5-10 minutes on first run)                        │
│                                                                 │
│  [✓] System packages                                            │
│  [✓] Docker                                                     │
│  [✓] Node.js 24                                                 │
│  [✓] Go 1.25                                                    │
│  [✓] Python 3.11                                                │
│  [░░░░░░░░░░] Rust...                                          │
│  [ ] Java 21                                                    │
│  [ ] Claude Code CLI                                            │
│  [ ] tmux configuration                                         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Stage 6: Agent Authentication

```
┌─────────────────────────────────────────────────────────────────┐
│  cloudcoop - Agent Setup                                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Authenticate Claude Code:                                      │
│                                                                 │
│  Opening browser for authentication...                          │
│  If browser doesn't open, visit:                                │
│  https://console.anthropic.com/auth?callback=...               │
│                                                                 │
│  [Waiting for authentication...]                                │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

After authentication:
```
┌─────────────────────────────────────────────────────────────────┐
│  cloudcoop - Setup Complete!                                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ✓ Everything is ready                                          │
│                                                                 │
│  Your sandbox:                                                  │
│  • Project: cloudcoop-sandbox-abc123                            │
│  • VM: claude-sandbox (europe-north2-a)                        │
│  • Type: c4a-highcpu-16 (spot)                                 │
│                                                                 │
│  Configuration saved to:                                        │
│  ~/.config/cloudcoop/config.yaml                                │
│                                                                 │
│  [Start using cloudcoop]                                        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Subsequent Runs

After initial setup, cloudcoop starts directly to the main TUI:

```
┌─────────────────────────────────────────────────────────────────┐
│  cloudcoop                                           v0.1.0     │
├─────────────────────────────────────────────────────────────────┤
│  Cloud: GCP (europe-north2-a)                                   │
│  VM: claude-sandbox          ○ Stopped                          │
│                                                                 │
│  [S]tart VM to begin working                                    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Configuration File

After setup, configuration is stored in `~/.config/cloudcoop/config.yaml`:

```yaml
cloud:
  provider: gcp
  gcp:
    project: cloudcoop-sandbox-abc123
    zone: europe-north2-a
    service_account: cloudcoop@cloudcoop-sandbox-abc123.iam.gserviceaccount.com

vm:
  name: claude-sandbox
  machine_type: arm-16cpu-32gb
  disk_size_gb: 50
  spot: true

network:
  ip_allowlist:
    mode: disabled

agents:
  default: claude
  installed:
    - claude
```

## Resetting Setup

To start fresh:

```bash
# Remove local config
rm -rf ~/.config/cloudcoop

# Optionally delete cloud resources
cloudcoop destroy --confirm
```
