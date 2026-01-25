# Cost Optimization Guide

Strategies to reduce costs while maintaining sandbox effectiveness.

## Cost Breakdown

Typical monthly costs (us-central1):

| Component | On-Demand | With Optimization |
|-----------|-----------|-------------------|
| e2-standard-16 VM | ~$400 | ~$120 (spot) |
| 200GB SSD disk | ~$34 | ~$34 |
| Network egress | ~$20 | ~$20 |
| **Total** | **~$454** | **~$174** |

## Optimization Strategies

### 1. Use Spot/Preemptible Instances (60-70% savings)

Spot instances can be preempted but cost significantly less.

```hcl
# terraform/terraform.tfvars
use_spot = true
```

**Trade-off:** VM may be stopped with 30 seconds notice. Suitable for:
- Development and testing
- Batch processing
- Non-time-critical work

**Mitigation:**
- Use persistent volumes (data survives preemption)
- Implement auto-restart scripts
- Run critical tasks on on-demand instances

### 2. Schedule Shutdown During Off-Hours (40-60% savings)

Stop the VM when not in use.

**Create shutdown schedule:**
```bash
# Create instance schedule
gcloud compute resource-policies create instance-schedule sandbox-schedule \
    --description="Shutdown nights and weekends" \
    --region=us-central1 \
    --vm-start-schedule="0 8 * * 1-5" \
    --vm-stop-schedule="0 20 * * *" \
    --timezone="America/New_York"

# Attach to VM
gcloud compute instances add-resource-policies claude-sandbox \
    --resource-policies=sandbox-schedule \
    --zone=us-central1-a
```

**Manual scripts:**
```bash
# scripts/schedule-stop.sh
gcloud compute instances stop claude-sandbox --zone=us-central1-a

# scripts/schedule-start.sh
gcloud compute instances start claude-sandbox --zone=us-central1-a
```

### 3. Right-Size the VM

Match VM size to actual usage.

| Agent Count | Recommended Machine | Monthly Cost |
|-------------|---------------------|--------------|
| 2-4 | e2-standard-4 | ~$100 |
| 4-6 | e2-standard-8 | ~$200 |
| 8-10 | e2-standard-16 | ~$400 |
| 10-16 | e2-standard-32 | ~$800 |

**Monitor and adjust:**
```bash
# Check CPU/memory utilization
gcloud monitoring metrics list --filter="metric.type=compute.googleapis.com/instance"

# Resize if underutilized
gcloud compute instances set-machine-type claude-sandbox \
    --machine-type=e2-standard-8 \
    --zone=us-central1-a
```

### 4. Committed Use Discounts (20-40% savings)

For sustained usage, commit to 1 or 3 years.

| Commitment | Discount |
|------------|----------|
| 1 year | ~20% |
| 3 years | ~40% |

```bash
# Check current usage
gcloud compute commitments list

# Create commitment (CLI)
gcloud compute commitments create sandbox-commitment \
    --region=us-central1 \
    --resources=vcpu=16,memory=64GB \
    --plan=twelve-month
```

### 5. Use Cheaper Disk Types

| Disk Type | Cost/GB/month | Use Case |
|-----------|---------------|----------|
| pd-standard (HDD) | $0.04 | Archives, logs |
| pd-balanced | $0.10 | General purpose |
| pd-ssd | $0.17 | High performance |

**For most sandbox use cases, pd-balanced is sufficient:**
```hcl
# terraform/variables.tf
boot_disk_type = "pd-balanced"  # Instead of pd-ssd
```

### 6. Clean Up Unused Resources

```bash
# Find orphaned disks
gcloud compute disks list --filter="-users:*"

# Delete unused snapshots
gcloud compute snapshots list --filter="creationTimestamp<'2024-01-01'"

# Prune Docker images on VM
docker system prune -a --volumes
```

### 7. Use Smaller Regions

Some regions are 10-20% cheaper:

| Region | Relative Cost |
|--------|---------------|
| us-central1 | Baseline |
| us-east1 | Similar |
| us-west1 | ~5% higher |
| europe-west1 | ~10% higher |
| asia-east1 | ~15% higher |

### 8. Network Optimization

**Reduce egress costs:**
- Keep API responses small
- Cache frequently accessed data
- Use internal IPs when possible

```bash
# Check egress usage
gcloud compute instances describe claude-sandbox \
    --format="get(networkInterfaces[0].accessConfigs[0].networkTier)"
```

## Cost Monitoring

### Set Up Budget Alerts

```bash
# Create budget
gcloud billing budgets create \
    --billing-account=BILLING_ACCOUNT_ID \
    --display-name="Claude Sandbox Budget" \
    --budget-amount=500 \
    --threshold-rule=percent=50 \
    --threshold-rule=percent=90 \
    --threshold-rule=percent=100
```

### View Current Costs

```bash
# Export billing to BigQuery for analysis
bq query --use_legacy_sql=false '
SELECT
  service.description,
  SUM(cost) as total_cost
FROM `PROJECT.billing_export.gcp_billing_export_v1_XXXXXX`
WHERE invoice.month = "202401"
GROUP BY 1
ORDER BY 2 DESC
'
```

## Comparison: Spot vs On-Demand vs Committed

For 12-agent sandbox (e2-standard-16):

| Strategy | Monthly Cost | Annual Cost | Savings |
|----------|-------------|-------------|---------|
| On-demand 24/7 | $400 | $4,800 | - |
| Spot 24/7 | $120 | $1,440 | 70% |
| On-demand business hours | $170 | $2,040 | 57% |
| Spot business hours | $50 | $600 | 88% |
| 1-year committed | $320 | $3,840 | 20% |

**Recommendation:** For development/testing, use spot instances with scheduled shutdown = 85%+ savings.

## Quick Start: Maximum Savings

```hcl
# terraform/terraform.tfvars
machine_type = "e2-standard-8"   # Right-sized for 6-8 agents
boot_disk_type = "pd-balanced"   # Balanced disk
use_spot = true                  # Spot pricing
```

Then add a shutdown schedule for off-hours.

Expected monthly cost: **~$50-80** (vs $454 baseline)
