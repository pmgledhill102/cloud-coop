# Troubleshooting Guide

Common issues and solutions for the Claude Code sandbox environment.

## Connection Issues

### Cannot SSH into VM

**Symptom:** `gcloud compute ssh` hangs or fails

**Solutions:**

1. **Check IAP permissions:**
   ```bash
   gcloud projects get-iam-policy PROJECT_ID \
     --filter="bindings.role:roles/iap.tunnelResourceAccessor"
   ```

2. **Verify VM is running:**
   ```bash
   gcloud compute instances describe claude-sandbox --zone=ZONE
   ```

3. **Check firewall rules:**
   ```bash
   gcloud compute firewall-rules list --filter="network:claude-sandbox-network"
   ```

4. **Try direct SSH (if external IP enabled):**
   ```bash
   ssh -i ~/.ssh/google_compute_engine EXTERNAL_IP
   ```

### SSH multiplexing issues

**Symptom:** Stale connections, "Connection refused"

**Solution:** Clear socket directory
```bash
rm -rf ~/.ssh/sockets/*
```

## Agent Issues

### Agents won't start

**Symptom:** `docker-compose up` fails

**Check 1: Docker is running**
```bash
sudo systemctl status docker
sudo systemctl start docker
```

**Check 2: Image exists**
```bash
docker images | grep claude-sandbox
```

**Check 3: Pull image if missing**
```bash
docker pull us-central1-docker.pkg.dev/PROJECT/claude-sandbox/agent:latest
```

### API key not found

**Symptom:** "ANTHROPIC_API_KEY is not set"

**Solution 1: Set directly in .env**
```bash
echo "ANTHROPIC_API_KEY=sk-ant-..." >> /workspaces/.env
```

**Solution 2: Add to Secret Manager**
```bash
echo "sk-ant-..." | gcloud secrets versions add anthropic-api-key --data-file=-
```

**Solution 3: Check Secret Manager access**
```bash
gcloud secrets versions access latest --secret=anthropic-api-key
```

### Agent container crashes

**Symptom:** Container exits immediately

**Check logs:**
```bash
docker logs claude-agent-1
```

**Common causes:**
- Missing API key
- Invalid settings.json
- Out of memory (increase limits)

## Resource Issues

### Out of memory

**Symptom:** Agents killed by OOM killer

**Solution 1: Reduce agent count**
```bash
AGENT_COUNT=8 ./scripts/start-agents.sh
```

**Solution 2: Increase VM size**
```hcl
# In terraform/terraform.tfvars
machine_type = "e2-standard-32"  # 128 GB RAM
```

**Solution 3: Reduce per-agent limits**
```yaml
# In docker-compose.yml
deploy:
  resources:
    limits:
      memory: 4G  # Reduce from 6G
```

### Disk full

**Symptom:** "No space left on device"

**Check usage:**
```bash
df -h
du -sh /var/lib/docker/*
```

**Clean up:**
```bash
# Remove unused Docker resources
docker system prune -a

# Remove old agent workspaces
docker volume prune
```

## Network Issues

### Cannot reach external APIs

**Symptom:** API calls timeout

**Check 1: NAT is configured**
```bash
gcloud compute routers nats list --router=claude-sandbox-router --region=REGION
```

**Check 2: DNS resolution**
```bash
docker exec claude-agent-1 nslookup api.anthropic.com
```

**Check 3: Test connectivity**
```bash
docker exec claude-agent-1 curl -v https://api.anthropic.com
```

### GitHub authentication fails

**Symptom:** `gh` commands fail

**Solution: Re-authenticate**
```bash
docker exec -it claude-agent-1 gh auth login
```

Or set token in .env:
```bash
GITHUB_TOKEN=ghp_xxxxx
```

## Terraform Issues

### State lock error

**Symptom:** "Error acquiring state lock"

**Solution:**
```bash
terraform force-unlock LOCK_ID
```

### API not enabled

**Symptom:** "API not enabled" errors

**Solution:**
```bash
gcloud services enable compute.googleapis.com
gcloud services enable artifactregistry.googleapis.com
gcloud services enable secretmanager.googleapis.com
```

### Quota exceeded

**Symptom:** Cannot create VM

**Check quotas:**
```bash
gcloud compute regions describe REGION --format="table(quotas)"
```

**Request increase:**
Visit: https://console.cloud.google.com/iam-admin/quotas

## Performance Issues

### Slow agent responses

**Possible causes:**
1. API rate limiting - spread requests across agents
2. Network latency - use region closer to Anthropic
3. Resource contention - reduce agent count

### High CPU usage

**Check:**
```bash
docker stats
```

**Solutions:**
- Reduce concurrent agents
- Add CPU limits to containers
- Scale up VM size

## Logging & Debugging

### View all agent logs

```bash
# Follow all logs
docker-compose logs -f

# Last 100 lines from specific agent
docker logs --tail 100 claude-agent-1
```

### Enable debug logging

```bash
# In container
export CLAUDE_CODE_DEBUG=1
```

### Export logs to Cloud Logging

Logs are automatically sent to Cloud Logging. Query with:
```bash
gcloud logging read "resource.type=gce_instance AND resource.labels.instance_id=INSTANCE_ID"
```

## Recovery Procedures

### Reset agent workspace

```bash
# Stop agent
docker stop claude-agent-1

# Remove workspace volume
docker volume rm workspaces_agent-1-workspace

# Restart agent
docker start claude-agent-1
```

### Full environment reset

```bash
# Stop everything
./scripts/stop-agents.sh

# Remove all containers and volumes
docker-compose down -v

# Rebuild images
./docker/build-and-push.sh

# Start fresh
./scripts/start-agents.sh 12
```

### Recreate VM from scratch

```bash
cd terraform
terraform destroy
terraform apply
```
