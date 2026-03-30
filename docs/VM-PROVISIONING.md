# VM Provisioning Prerequisites

This guide covers how to provision and configure a GCP VM for use with cloudcoop. Follow these
steps before running cloudcoop for the first time.

## Overview

The cloudcoop TUI automates most setup, but you need:

1. A GCP account with billing enabled
2. The gcloud CLI installed and authenticated
3. An SSH key for VM access
4. Basic understanding of GCP concepts

## Prerequisites

### 1. Install gcloud CLI

Install the Google Cloud CLI for your platform:

```bash
# macOS (using Homebrew)
brew install --cask google-cloud-sdk

# Linux (Debian/Ubuntu)
curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg
echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | sudo tee /etc/apt/sources.list.d/google-cloud-sdk.list
sudo apt-get update && sudo apt-get install google-cloud-cli

# Other platforms
# See: https://cloud.google.com/sdk/docs/install
```

### 2. Authenticate with GCP

```bash
# Authenticate your user account
gcloud auth login

# Set up application default credentials (for SDK access)
gcloud auth application-default login
```

### 3. Create or Select a GCP Project

```bash
# List existing projects
gcloud projects list

# Use an existing project
gcloud config set project YOUR_PROJECT_ID

# Or create a new project
gcloud projects create cloudcoop-sandbox --name="Cloudcoop Sandbox"
gcloud config set project cloudcoop-sandbox

# Link billing account (required for Compute Engine)
gcloud billing accounts list
gcloud billing projects link cloudcoop-sandbox --billing-account=XXXXXX-XXXXXX-XXXXXX
```

### 4. Enable Required APIs

```bash
gcloud services enable compute.googleapis.com
gcloud services enable iap.googleapis.com
gcloud services enable secretmanager.googleapis.com
```

### 5. Generate SSH Key

If you don't have an SSH key:

```bash
# Generate ED25519 key (recommended)
ssh-keygen -t ed25519 -C "your-email@example.com"

# Or RSA key
ssh-keygen -t rsa -b 4096 -C "your-email@example.com"
```

The key should be at `~/.ssh/id_ed25519` or `~/.ssh/id_rsa`.

## Creating the VM

### Recommended Configuration

Create a VM optimised for cloudcoop with spot pricing and persistent boot disk:

```bash
gcloud compute instances create claude-sandbox \
  --zone=europe-north2-a \
  --machine-type=c4a-highcpu-16 \
  --boot-disk-size=50GB \
  --boot-disk-type=pd-ssd \
  --boot-disk-auto-delete=no \
  --image-family=ubuntu-2404-lts-arm64 \
  --image-project=ubuntu-os-cloud \
  --provisioning-model=SPOT \
  --instance-termination-action=STOP \
  --tags=claude-sandbox \
  --metadata=enable-oslogin=TRUE
```

Key flags explained:

| Flag | Purpose |
|------|---------|
| `--boot-disk-auto-delete=no` | Disk survives instance deletion for data persistence |
| `--provisioning-model=SPOT` | Use spot pricing (~70% discount) |
| `--instance-termination-action=STOP` | Stop (don't delete) on preemption |
| `--tags=claude-sandbox` | For firewall rules targeting |
| `--metadata=enable-oslogin=TRUE` | Use OS Login for SSH key management |

### VM Metadata (cloudcoop-managed VMs)

When cloudcoop creates VMs, it adds metadata labels for identification and upgrade detection:

| Metadata Key | Example Value | Purpose |
|--------------|---------------|---------|
| `cloudcoop-version` | `v0.1.0` | Version of cloudcoop that created the VM |
| `cloudcoop-created` | `2024-01-15T10:30:00Z` | ISO timestamp of creation |
| `cloudcoop-config-hash` | `a1b2c3d4` | Hash of config for upgrade detection |

This metadata enables:

- **Identification**: Detect if a VM was created by cloudcoop vs manually
- **Upgrade workflows**: Detect missing tools/config when cloudcoop version advances
- **Multi-VM support**: Identify which VMs belong to cloudcoop
- **Diagnostics**: `cloudcoop doctor` can verify VM state matches expectations

To manually add cloudcoop metadata to an existing VM:

```bash
gcloud compute instances add-metadata claude-sandbox \
  --zone=europe-north2-a \
  --metadata=cloudcoop-version=v0.1.0,cloudcoop-created=$(date -u +%Y-%m-%dT%H:%M:%SZ)
```

### Alternative: On-Demand Instance

For guaranteed availability (no preemption):

```bash
gcloud compute instances create claude-sandbox \
  --zone=europe-north2-a \
  --machine-type=c4a-highcpu-16 \
  --boot-disk-size=50GB \
  --boot-disk-type=pd-ssd \
  --boot-disk-auto-delete=no \
  --image-family=ubuntu-2404-lts-arm64 \
  --image-project=ubuntu-os-cloud \
  --tags=claude-sandbox \
  --metadata=enable-oslogin=TRUE
```

### Machine Type Options

| Machine Type | vCPUs | Memory | Spot Price | Use Case |
|-------------|-------|--------|------------|----------|
| c4a-highcpu-8 | 8 | 16GB | ~$0.06/hr | Light workloads |
| c4a-highcpu-16 | 16 | 32GB | ~$0.12/hr | Recommended for 4-8 agents |
| c4a-highcpu-32 | 32 | 64GB | ~$0.24/hr | Heavy builds, many agents |

## IAP (Identity-Aware Proxy) Setup

IAP provides secure SSH access without exposing the VM to the public internet.

### Enable IAP Firewall Rule

```bash
# Create firewall rule for IAP SSH access
gcloud compute firewall-rules create allow-ssh-iap \
  --direction=INGRESS \
  --priority=1000 \
  --network=default \
  --action=ALLOW \
  --rules=tcp:22 \
  --source-ranges=35.235.240.0/20 \
  --target-tags=claude-sandbox
```

The source range `35.235.240.0/20` is Google's IAP IP range.

### Grant IAP Access

```bash
# Grant yourself IAP tunnel access
gcloud projects add-iam-policy-binding $(gcloud config get-value project) \
  --member="user:YOUR_EMAIL@example.com" \
  --role="roles/iap.tunnelResourceAccessor"
```

### SSH via IAP

```bash
# SSH using IAP tunnel (recommended)
gcloud compute ssh claude-sandbox --zone=europe-north2-a --tunnel-through-iap

# Or set up SSH config for easier access
gcloud compute config-ssh
```

## Basic VM Setup

After creating the VM, SSH in and set up the environment.

### Update System Packages

```bash
sudo apt-get update && sudo apt-get upgrade -y
```

### Install tmux

tmux is essential for persistent sessions that survive SSH disconnections:

```bash
sudo apt-get install -y tmux

# Optional: Install a nicer tmux configuration
cat > ~/.tmux.conf << 'EOF'
# Enable mouse support
set -g mouse on

# Start windows and panes at 1, not 0
set -g base-index 1
setw -g pane-base-index 1

# Increase scrollback buffer
set -g history-limit 50000

# Status bar
set -g status-bg colour235
set -g status-fg colour136
EOF
```

### Create Non-Root User (Optional)

If using OS Login, user creation is handled automatically. Otherwise:

```bash
# Create user with sudo access
sudo useradd -m -s /bin/bash -G sudo developer
sudo passwd developer

# Add SSH key for the new user
sudo mkdir -p /home/developer/.ssh
sudo cp ~/.ssh/authorized_keys /home/developer/.ssh/
sudo chown -R developer:developer /home/developer/.ssh
sudo chmod 700 /home/developer/.ssh
sudo chmod 600 /home/developer/.ssh/authorized_keys
```

### Install Development Tools

Basic tools for development:

```bash
# Essential tools
sudo apt-get install -y \
  git \
  curl \
  wget \
  unzip \
  build-essential \
  jq \
  htop

# Docker (for containerized builds)
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER
```

## Daily Workflow

### Starting Work

```bash
# Start the VM
gcloud compute instances start claude-sandbox --zone=europe-north2-a

# SSH in via IAP
gcloud compute ssh claude-sandbox --zone=europe-north2-a --tunnel-through-iap

# Start or attach to tmux session
tmux new-session -s work || tmux attach-session -t work
```

### Stopping Work

```bash
# Detach from tmux: Ctrl+b, then d
# Exit SSH: exit or Ctrl+d

# Stop the VM to save costs
gcloud compute instances stop claude-sandbox --zone=europe-north2-a
```

### What Happens on Stop/Preemption

| Event | Result | Cost |
|-------|--------|------|
| Running (spot) | ~70% discount | Varies by machine type |
| GCP preempts | VM stops, disk persists | $0 compute |
| You stop manually | VM stops, disk persists | $0 compute |
| Stopped | Just disk storage | ~$5/month for 50GB SSD |

Everything on the boot disk persists:

- `~/.claude/` - All Claude sessions intact
- `/workspaces/` - All git repos and work
- Installed tools, configs, everything

## Cost Estimates

For c4a-highcpu-16 in europe-north2 (Stockholm):

| Usage Pattern | Monthly Cost |
|---------------|-------------|
| On-demand 24/7 | ~$280 |
| Spot 24/7 | ~$85 |
| Spot 8h/day (workdays) | ~$30 |
| Stopped (disk only) | ~$5 |

See [COST-OPTIMISATION.md](./COST-OPTIMISATION.md) for more strategies.

## Troubleshooting

### Cannot SSH to VM

```bash
# Check VM status
gcloud compute instances describe claude-sandbox --zone=europe-north2-a --format="get(status)"

# Check firewall rules
gcloud compute firewall-rules list --filter="name~ssh"

# Check IAP permissions
gcloud projects get-iam-policy $(gcloud config get-value project) \
  --filter="bindings.role:roles/iap.tunnelResourceAccessor"
```

### VM Preempted Frequently

Spot instances can be preempted based on demand. Options:

- Switch to on-demand for critical work
- Choose regions with lower demand
- Use during off-peak hours

### Disk Full

```bash
# Check disk usage
df -h

# Clean up Docker resources
docker system prune -a --volumes

# Find large files
sudo du -h / --max-depth=3 2>/dev/null | sort -rh | head -20
```

## Next Steps

After VM setup:

1. cloudcoop TUI will handle agent installation
2. See [SPOT-RESILIENCE.md](./SPOT-RESILIENCE.md) for session persistence
3. See [TOOLING.md](./TOOLING.md) for installed development tools
