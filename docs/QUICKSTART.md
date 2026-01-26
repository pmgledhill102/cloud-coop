# Quickstart Guide

This guide covers manual setup for testing cloudcoop with an existing GCP VM.

## Prerequisites

1. **GCP Project** with Compute Engine API enabled
2. **An existing VM** in your project (cloudcoop doesn't create VMs yet)
3. **gcloud CLI** installed and authenticated

## Set Environment Variables

Set these variables once, then copy-paste all commands below:

```bash
export PROJECT_ID="your-gcp-project-id"
export ZONE="us-central1-a"
export VM_NAME="your-vm-name"
```

To find your values:

```bash
# List your projects
gcloud projects list

# List VMs in a project
gcloud compute instances list --project=$PROJECT_ID
```

## Step 1: Enable Required GCP APIs

For a new project, enable the required APIs:

```bash
gcloud config set project $PROJECT_ID

gcloud services enable \
  compute.googleapis.com \
  secretmanager.googleapis.com \
  iam.googleapis.com
```

| API | Purpose |
|-----|---------|
| `compute.googleapis.com` | VM lifecycle management |
| `secretmanager.googleapis.com` | Secure storage of agent API keys |
| `iam.googleapis.com` | Service account management |

Verify APIs are enabled:

```bash
gcloud services list --enabled | grep -E 'compute|secret|iam'
```

## Step 2: Set Up GCP Authentication

cloudcoop uses Application Default Credentials (ADC) to authenticate with GCP.

```bash
gcloud auth login
gcloud auth application-default login
```

Verify ADC is working:

```bash
gcloud auth application-default print-access-token
```

You should see a token printed. If you get an error, re-run the `application-default login` command.

## Step 3: Create Configuration File

### Option A: Use the setup wizard (recommended)

```bash
make build
./bin/cloudcoop config init
```

The wizard will prompt you for your GCP project, zone, and VM name.

### Option B: Create manually

```bash
mkdir -p ~/.config/cloudcoop

cat > ~/.config/cloudcoop/cloudcoop.toml << EOF
[cloud]
provider = "gcp"

[cloud.gcp]
project = "$PROJECT_ID"
zone = "$ZONE"

[vm]
name = "$VM_NAME"
EOF
```

**Verify the config:**

```bash
./bin/cloudcoop config show
```

## Step 4: Test the Status Command

```bash
make build
./bin/cloudcoop status
```

Expected output for a running VM:

```text
cloudcoop status

Cloud:    gcp
Project:  your-gcp-project-id

VM:
  Name:         your-vm-name
  Status:       ● running
  Zone:         us-central1-a
  Machine Type: e2-medium
  External IP:  34.123.45.67
  Internal IP:  10.128.0.2

Agents:
  (querying agents not yet implemented)
```

Expected output if VM doesn't exist:

```text
VM:
  your-vm-name: not found

  The configured VM does not exist.
  Create it in the GCP Console or use 'cloudcoop create'.
```

## Step 5: Test the TUI

```bash
./bin/cloudcoop
```

The TUI should display your VM's status. Press `r` to refresh, `q` to quit.

## Step 6: Configure SSH Access

cloudcoop uses SSH to execute commands on the VM. You need:

1. **An SSH key** - cloudcoop looks for keys in this order:
   - SSH agent (recommended) - keys loaded via `ssh-add`
   - `~/.ssh/id_ed25519`
   - `~/.ssh/id_rsa`
   - `~/.ssh/id_ecdsa`

2. **Firewall access** - The VM must allow SSH (port 22) from your IP.

### Generate an SSH Key (if needed)

```bash
ssh-keygen -t ed25519 -C "your-email@example.com"
```

### Add Key to SSH Agent (recommended)

```bash
eval "$(ssh-agent -s)"
ssh-add ~/.ssh/id_ed25519
```

### Configure GCP Firewall

For VMs with external IPs, ensure SSH access is allowed:

```bash
# Check existing firewall rules
gcloud compute firewall-rules list --filter="name~ssh" --project=$PROJECT_ID

# Create rule if needed (allows SSH from anywhere - restrict for production)
gcloud compute firewall-rules create allow-ssh \
  --project=$PROJECT_ID \
  --direction=INGRESS \
  --priority=1000 \
  --network=default \
  --action=ALLOW \
  --rules=tcp:22 \
  --source-ranges=0.0.0.0/0
```

For production, restrict `--source-ranges` to your IP or use IAP tunneling.

### Test SSH Connectivity

```bash
./bin/cloudcoop ssh hostname
```

Expected output: the VM's hostname.

## Troubleshooting

### "Configuration not found"

Run the setup wizard or create the config file manually:

```bash
./bin/cloudcoop config init
```

### "create compute client: google: could not find default credentials"

Run `gcloud auth application-default login` to set up ADC.

### "get instance: googleapi: Error 403: Required 'compute.instances.get' permission"

Your account needs the `Compute Viewer` role (or higher) on the project:

```bash
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="user:$(gcloud config get-value account)" \
  --role="roles/compute.viewer"
```

### "get instance: googleapi: Error 404: The resource was not found"

Either the VM name or zone is incorrect. Verify with:

```bash
gcloud compute instances list --project=$PROJECT_ID
```

### "No SSH authentication methods available"

cloudcoop couldn't find any SSH keys. Either:

1. Start SSH agent and add your key:

   ```bash
   eval "$(ssh-agent -s)"
   ssh-add ~/.ssh/id_ed25519
   ```

2. Or ensure you have an unencrypted key at `~/.ssh/id_ed25519`, `~/.ssh/id_rsa`, or `~/.ssh/id_ecdsa`.

### "Connection refused" or SSH timeout

1. Check the VM is running:

   ```bash
   ./bin/cloudcoop status
   ```

2. Check firewall allows SSH from your IP:

   ```bash
   gcloud compute firewall-rules list --filter="name~ssh" --project=$PROJECT_ID
   ```

3. Verify SSH works via gcloud:

   ```bash
   gcloud compute ssh $VM_NAME --zone=$ZONE --project=$PROJECT_ID
   ```

### "VM has no external IP address"

The VM doesn't have a public IP. Either:

- Add an external IP to the VM in GCP Console
- Use IAP tunneling (not yet supported by cloudcoop)

## Creating a Test VM (Optional)

If you don't have a VM to test with, create one:

```bash
gcloud compute instances create $VM_NAME \
  --project=$PROJECT_ID \
  --zone=$ZONE \
  --machine-type=e2-micro \
  --image-family=ubuntu-2404-lts-amd64 \
  --image-project=ubuntu-os-cloud
```

Remember to delete it when done to avoid charges:

```bash
gcloud compute instances delete $VM_NAME --project=$PROJECT_ID --zone=$ZONE
```
