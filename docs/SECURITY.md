# Security Considerations

> **See also:** [SECURITY-MODEL.md](SECURITY-MODEL.md) for the architectural security model
> (trust boundaries, privilege asymmetry, and reasoning behind the security posture).

This document covers operational security for the cloudcoop sandbox environment: what to
configure, how to monitor, and how to respond to incidents.

## Threat Model

The sandbox is designed to:

1. **Contain agent actions** - Agents run in tmux sessions with shared VM access
2. **Limit GCP access** - Minimal IAM permissions via service account
3. **Prevent lateral movement** - Network isolation within the VM
4. **Audit activity** - Logging of all agent actions

> **Note:** Agents run as tmux windows on the VM, not in Docker containers. However, agents may
> use Docker as part of their work (building images, running tests, etc.). The Docker
> configuration below applies to containers the agents create, not to the agents themselves.

## Security Layers

### 1. GCP Service Account (Least Privilege)

cloudcoop **requires** a dedicated service account for VMs. The `cloud.gcp.service_account`
configuration is mandatory and VM creation will fail without it. This prevents VMs from inheriting
the overly-permissive default Compute Engine service account (which has Editor permissions).

The VM should run under a dedicated service account with minimal permissions:

```text
✅ Recommended Permissions:
- roles/logging.logWriter            # Write logs to Cloud Logging
- roles/monitoring.metricWriter      # Write metrics
- roles/artifactregistry.reader      # Pull container images (optional)
- roles/secretmanager.secretAccessor # Access specific secrets (optional)

❌ NOT Allowed (Never Grant These):
- roles/compute.*              # Cannot create/delete VMs
- roles/iam.*                  # Cannot modify IAM
- roles/storage.*              # Cannot access arbitrary buckets
- roles/billing.*              # Cannot affect billing
- roles/editor                 # Overly broad permissions
- roles/owner                  # Full project access
```

See [SETUP-FLOW.md](SETUP-FLOW.md#manual-service-account-setup-required) for setup instructions.

### 2. Network Isolation

- **Dedicated VPC** - Sandbox runs in isolated network
- **No ingress** - Default deny all inbound (except SSH via IAP)
- **Egress allowed** - Agents need internet for API calls
- **IAP-only SSH** - No public IP exposure required

### 3. Container Security (for agent workloads)

When agents create Docker containers as part of their work, recommend these security settings:

```yaml
security_opt:
  - no-new-privileges:true     # Prevent privilege escalation

deploy:
  resources:
    limits:
      cpus: "2"                # CPU limits
      memory: 6G               # Memory limits
```

### 4. Workload Identity (Recommended)

Instead of service account keys, use Workload Identity:

```bash
# The VM's service account automatically provides credentials
# No key files to manage or rotate
```

## What Agents CAN Do

- Read/write files within their workspace
- Execute arbitrary code (that's the point)
- Make API calls to external services
- Run Docker commands to build/test their work (Docker installed on VM)
- Access secrets via Secret Manager (configured ones only)

## What Agents CANNOT Do

- Access other agents' workspaces (directory conventions)
- Modify GCP IAM permissions
- Create/delete GCP resources
- Access production databases (not in network)

## Recommendations

### 1. Rotate API Keys Regularly

```bash
# Update the secret
echo "new-api-key" | gcloud secrets versions add anthropic-api-key --data-file=-

# Old versions remain accessible until disabled
gcloud secrets versions disable anthropic-api-key --version=1
```

### 2. Use Separate Projects

For higher isolation, run the sandbox in a dedicated GCP project:

```bash
# Sandbox project has no access to production resources
gcloud projects create claude-sandbox-project
```

### 3. Enable Audit Logging

```hcl
# In terraform/main.tf
resource "google_project_iam_audit_config" "all" {
  project = var.project_id
  service = "allServices"
  audit_log_config {
    log_type = "ADMIN_READ"
  }
  audit_log_config {
    log_type = "DATA_WRITE"
  }
}
```

### 4. Set Up Alerts

```bash
# Alert on suspicious activity
gcloud alpha monitoring policies create \
  --notification-channels=YOUR_CHANNEL \
  --condition="metric.type=\"compute.googleapis.com/instance/cpu/utilization\" > 0.9"
```

### 5. Regular Reviews

- Review Cloud Audit Logs weekly
- Check for unexpected API calls
- Monitor costs for anomalies
- Rotate credentials quarterly

## Emergency Response

### If an Agent is Compromised

1. **Stop all agents immediately:**

   ```bash
   ./scripts/stop-agents.sh
   ```

2. **Revoke API key:**

   ```bash
   gcloud secrets versions disable anthropic-api-key --version=latest
   ```

3. **Isolate the VM:**

   ```bash
   gcloud compute instances stop claude-sandbox --zone=ZONE
   ```

4. **Review logs:**

   ```bash
   gcloud logging read "resource.type=gce_instance"
   ```

5. **Rotate all credentials** before resuming

## Compliance Notes

- **Data residency**: Agents make API calls to Anthropic (US-based)
- **PII handling**: Do not process PII in sandbox environment
- **Audit trail**: Cloud Logging retains logs for 30 days by default

## Security Checklist

- [x] Service account has minimal permissions (enforced by config validation)
- [ ] IAP enabled for SSH access
- [ ] No external IP (or restricted firewall)
- [ ] API key stored in Secret Manager
- [ ] Audit logging enabled
- [ ] Alerts configured for anomalies
- [ ] Docker images scanned for vulnerabilities
- [ ] Regular credential rotation scheduled
