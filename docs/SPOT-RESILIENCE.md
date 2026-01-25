# Spot Instance Resilience for Claude Code

Use spot instances for ~70% cost savings with persistent boot disk that survives both preemption and manual stops.

## Recommended Setup

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
  --instance-termination-action=STOP
```

Key flags:
- `--boot-disk-auto-delete=no` - Disk survives instance deletion
- `--provisioning-model=SPOT` - Use spot pricing (~70% discount)
- `--instance-termination-action=STOP` - Stop (don't delete) on preemption

## Daily Workflow

```bash
# Start working
gcloud compute instances start claude-sandbox --zone=europe-north2-a
# SSH in, run: ./start-agents-resilient.sh 12 continue

# Stop when done (or GCP preempts automatically)
gcloud compute instances stop claude-sandbox --zone=europe-north2-a
```

**What happens:**

| Event | Result | Cost |
|-------|--------|------|
| Running (spot) | ~70% discount | ~$85/month if 24/7 |
| GCP preempts | VM stops, disk persists | $0 compute |
| You stop manually | VM stops, disk persists | $0 compute |
| Stopped | Just disk storage | ~$5/month |

**Everything persists:**
- `~/.claude/` - All Claude sessions intact
- `/workspaces/` - All git repos and work
- Installed tools, configs, everything

---

## Shutdown Hook (Optional)

Install to cleanly save git state before preemption:

```bash
sudo cp scripts/save-claude-state.sh /usr/local/bin/
sudo cp scripts/restore-claude-state.sh /usr/local/bin/
sudo chmod +x /usr/local/bin/*.sh

# Install systemd service
sudo tee /etc/systemd/system/claude-shutdown.service << 'EOF'
[Unit]
Description=Save Claude state on shutdown/preemption
DefaultDependencies=no
Before=shutdown.target reboot.target halt.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/save-claude-state.sh
TimeoutStartSec=25

[Install]
WantedBy=shutdown.target reboot.target halt.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable claude-shutdown.service
```

---

## Session Resumption

Claude Code has built-in session persistence:

```bash
# Continue most recent session in current directory
claude --continue

# Resume specific named session
claude --resume my-task-name

# Interactive session picker
claude --resume
```

**Tip:** Name your sessions for easy resumption:
```bash
# Inside Claude Code:
/rename agent-1-issue-123

# Later:
claude --resume agent-1-issue-123
```

---

## Cost Comparison

For c4a-highcpu-16 in europe-north2 (Stockholm):

| Usage Pattern | Monthly Cost |
|---------------|-------------|
| On-demand 24/7 | ~$280 |
| Spot 24/7 | ~$85 |
| Spot 8h/day | ~$30 |
| Stopped (disk only) | ~$5 |

---

## Testing

```bash
# Test shutdown hook without shutting down
sudo /usr/local/bin/save-claude-state.sh
cat /var/log/claude-state-save.log

# Test full cycle
sudo reboot
# After reboot:
./start-agents-resilient.sh 12 continue
```
