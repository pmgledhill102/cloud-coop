# Troubleshooting Guide

Common issues and solutions for cloudcoop.

## Connection Issues

### Cannot SSH into VM

**Symptom:** `cloudcoop` shows "connecting..." indefinitely or SSH fails

**Solutions:**

1. **Check VM is running:**

   ```bash
   cloudcoop status
   # Or directly via gcloud:
   gcloud compute instances describe VM_NAME --zone=ZONE --format='value(status)'
   ```

2. **Verify SSH key is configured:**

   ```bash
   # Check your SSH key exists
   ls -la ~/.ssh/id_rsa ~/.ssh/id_ed25519

   # Test SSH directly
   ssh -v USER@VM_IP
   ```

3. **Check firewall allows SSH:**

   ```bash
   gcloud compute firewall-rules list --filter="allowed.ports:22"
   ```

4. **For IAP tunnel issues:**

   ```bash
   # Verify IAP permissions
   gcloud projects get-iam-policy PROJECT_ID \
     --filter="bindings.role:roles/iap.tunnelResourceAccessor"
   ```

### SSH connection drops or hangs

**Symptom:** Connection works initially then freezes

**Solutions:**

1. **Enable SSH keepalive in ~/.ssh/config:**

   ```text
   Host *
       ServerAliveInterval 60
       ServerAliveCountMax 3
   ```

2. **Clear stale SSH control sockets:**

   ```bash
   rm -rf ~/.ssh/sockets/*
   ```

3. **Check network stability:**

   ```bash
   ping VM_IP
   ```

## Agent Issues

### tmux not installed on VM

**Symptom:** cloudcoop shows "tmux not installed"

**Solution:** Install tmux on the VM:

```bash
# SSH into VM first
ssh USER@VM_IP

# Then install tmux
sudo apt-get update && sudo apt-get install -y tmux
```

### No agents session exists

**Symptom:** Agent list is empty or shows "no session"

This is normal if no agents have been started yet. Use the **A** key in the TUI or:

```bash
# Start an agent via CLI
cloudcoop agents add --name agent-1 --command "claude --dangerously-skip-permissions"
```

### Cannot connect to agent

**Symptom:** Pressing **C** to connect fails

**Solutions:**

1. **Check the agent is running:**

   ```bash
   # SSH to VM and list tmux windows
   ssh USER@VM_IP "tmux list-windows -t agents"
   ```

2. **Check tmux session exists:**

   ```bash
   ssh USER@VM_IP "tmux has-session -t agents && echo 'Session exists'"
   ```

3. **Manually attach to agent:**

   ```bash
   # Connect directly via SSH + tmux
   ssh -t USER@VM_IP "tmux attach -t agents"
   ```

### Agent shows wrong status

**Symptom:** Agent appears idle but is actually working

The TUI shows the current process in the tmux pane. If an agent is running a subprocess, it may
show that subprocess name instead of "claude" or "aider".

**Refresh the view:** Press **r** to refresh the agent list.

## VM Issues

### VM won't start

**Symptom:** Pressing **S** (Start) shows an error

**Solutions:**

1. **Check GCP quota:**

   ```bash
   gcloud compute regions describe REGION --format="table(quotas)"
   ```

2. **Check spot instance availability:**
   Spot instances may be unavailable in your zone. Try a different zone or use standard instances.

3. **Verify your permissions:**

   ```bash
   gcloud projects get-iam-policy PROJECT_ID \
     --filter="bindings.members:user:YOUR_EMAIL"
   ```

### VM won't stop

**Symptom:** Pressing **T** (sTop) shows an error

**Solution:** Check if VM is already stopped:

```bash
gcloud compute instances describe VM_NAME --zone=ZONE --format='value(status)'
```

If stuck in STOPPING state, wait a few minutes. GCP VMs can take time to stop gracefully.

## Configuration Issues

### Configuration file not found

**Symptom:** cloudcoop shows "config error"

**Solution:** Create the config file:

```bash
# Run the setup wizard
cloudcoop config init

# Or create manually
mkdir -p ~/.config/cloudcoop
cat > ~/.config/cloudcoop/cloudcoop.toml << 'EOF'
[cloud]
provider = "gcp"

[cloud.gcp]
project = "your-project-id"
zone = "us-central1-a"

[vm]
name = "claude-sandbox"
EOF
```

### Invalid configuration

**Symptom:** cloudcoop shows specific config field errors

**Solutions:**

1. **View current configuration:**

   ```bash
   cloudcoop config show
   ```

2. **Update specific values:**

   ```bash
   cloudcoop config set cloud.gcp.project my-project
   cloudcoop config set cloud.gcp.zone us-central1-a
   cloudcoop config set vm.name claude-sandbox
   ```

3. **Validate TOML syntax:**

   ```bash
   cat ~/.config/cloudcoop/cloudcoop.toml
   # Check for syntax errors like missing quotes or brackets
   ```

## API Key Issues

### Anthropic API key not available to agents

**Symptom:** Claude Code agents fail with API key errors

cloudcoop does not manage API keys directly. You need to configure keys on the VM.

**Solutions:**

1. **Set environment variable in VM shell profile:**

   ```bash
   ssh USER@VM_IP
   echo 'export ANTHROPIC_API_KEY="sk-ant-..."' >> ~/.bashrc
   ```

2. **Use SSH agent forwarding for GitHub/other credentials:**

   ```bash
   # In your ~/.ssh/config
   Host VM_IP
       ForwardAgent yes
   ```

## Network Issues

### Cannot reach external APIs from VM

**Symptom:** Agents cannot reach api.anthropic.com or api.github.com

**Solutions:**

1. **Check NAT gateway is configured (for private VMs):**

   ```bash
   gcloud compute routers nats list --router=ROUTER_NAME --region=REGION
   ```

2. **Test connectivity from VM:**

   ```bash
   ssh USER@VM_IP "curl -s https://api.anthropic.com/v1/messages -I | head -1"
   ```

3. **Check DNS resolution:**

   ```bash
   ssh USER@VM_IP "nslookup api.anthropic.com"
   ```

## Debugging

### Enable verbose output

Set the LOG_LEVEL environment variable:

```bash
LOG_LEVEL=debug cloudcoop
```

### Check cloudcoop version

```bash
cloudcoop version
```

### View raw VM information

```bash
cloudcoop status --json
```

## Recovery Procedures

### Kill a stuck agent

If an agent is unresponsive:

1. **Use the TUI:** Select the agent and press **K** to kill it

2. **Or manually via SSH:**

   ```bash
   ssh USER@VM_IP "tmux kill-window -t agents:WINDOW_INDEX"
   ```

### Reset all agents

To kill all agents and start fresh:

```bash
ssh USER@VM_IP "tmux kill-session -t agents"
```

### Full VM reset

If the VM is in a bad state:

```bash
# Stop the VM
cloudcoop stop
# or: gcloud compute instances stop VM_NAME --zone=ZONE

# Start it fresh
cloudcoop start
# or: gcloud compute instances start VM_NAME --zone=ZONE
```
