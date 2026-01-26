# First-Run Setup Flow

This document describes the user experience when running cloudcoop for the first time.

## Overview

The TUI guides users through setup interactively, creating resources as needed. No manual pre-configuration required.

## Setup Stages

```text
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

```text
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

```text
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

```text
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

```text
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

```text
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

```text
┌─────────────────────────────────────────────────────────────────┐
│  Creating VM...                                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  [✓] Created VM: claude-sandbox                                 │
│  [✓] Attached service account                                   │
│  [✓] Configured spot instance with STOP on preemption          │
│  [✓] Added cloudcoop metadata (version, timestamp)             │
│  [░░░░░░░░░░░░░░░░░░░] Waiting for VM to start...              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**VM Metadata:** cloudcoop adds metadata labels to VMs it creates:

- `cloudcoop-version`: Version that created the VM (for upgrade detection)
- `cloudcoop-created`: Creation timestamp
- `cloudcoop-config-hash`: Config hash (to detect when reconfiguration needed)

### Stage 5: VM Provisioning

```text
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

```text
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

```text
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
│  ~/.config/cloudcoop/cloudcoop.toml                             │
│                                                                 │
│  [Start using cloudcoop]                                        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Subsequent Runs

After initial setup, cloudcoop starts directly to the main TUI:

```text
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

After setup, configuration is stored in `~/.config/cloudcoop/cloudcoop.toml`:

```toml
[cloud]
provider = "gcp"

[cloud.gcp]
project = "cloudcoop-sandbox-abc123"
zone = "europe-north2-a"
service_account = "cloudcoop-vm@cloudcoop-sandbox-abc123.iam.gserviceaccount.com"

[vm]
name = "claude-sandbox"
machine_type = "arm-16cpu-32gb"
disk_size_gb = 50
spot = true

[network]
ip_allowlist_mode = "disabled"

[agents]
default = "claude"
installed = ["claude"]
```

## Manual Service Account Setup (Required)

Before creating a VM, you must create a dedicated service account with minimal permissions.
This is required for security (see [SECURITY.md](SECURITY.md)).

### 1. Create the Service Account

```bash
# Set your project ID
PROJECT_ID="your-gcp-project-id"

# Create the service account
gcloud iam service-accounts create cloudcoop-vm \
  --project="${PROJECT_ID}" \
  --display-name="cloudcoop VM service account"
```

### 2. Grant Minimal Permissions

Grant only the permissions the VM actually needs:

```bash
# Allow writing logs to Cloud Logging
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:cloudcoop-vm@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/logging.logWriter"

# Allow writing metrics to Cloud Monitoring
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:cloudcoop-vm@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/monitoring.metricWriter"

# Optional: Allow pulling container images from Artifact Registry
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:cloudcoop-vm@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/artifactregistry.reader"

# Optional: Allow accessing specific secrets (if needed)
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:cloudcoop-vm@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"
```

### 3. Update Configuration

Add the service account to your config file:

```toml
[cloud.gcp]
project = "your-gcp-project-id"
zone = "us-central1-a"
service_account = "cloudcoop-vm@your-gcp-project-id.iam.gserviceaccount.com"
```

### Important: What NOT to Grant

Do NOT grant these permissions to the VM service account:

- `roles/compute.*` - VM should not modify infrastructure
- `roles/iam.*` - VM should not modify IAM
- `roles/storage.*` - VM should not access arbitrary buckets
- `roles/editor` or `roles/owner` - Never grant broad permissions

The default Compute Engine service account has Editor permissions by default, which is why
cloudcoop requires a dedicated service account.

## Resetting Setup

To start fresh:

```bash
# Remove local config
rm -rf ~/.config/cloudcoop

# Optionally delete cloud resources
cloudcoop destroy --confirm
```
