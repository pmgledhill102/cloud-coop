# ADR-0007: Infrastructure Management Approach

## Status

Accepted

## Context

The sandbox environment requires management of GCP resources:
- Compute Engine VM instance
- Service account and IAM bindings
- Firewall rules
- (Optional) Persistent disks, snapshots

A Terminal UI (TUI) will provide the primary interface for users to manage these resources. The TUI needs to:
- Check if resources exist
- Create resources if missing
- Start/stop VMs
- Resize VMs (change machine type)
- Query VM status in real-time

The question is whether to use Terraform or direct gcloud CLI commands for these operations.

## Decision

Use **direct gcloud CLI commands** invoked from the TUI, not Terraform.

Terraform may be provided as an optional reference for initial setup, but the TUI will use gcloud for all runtime operations.

## Options Considered

### Option 1: Terraform for All Operations

Use Terraform to manage all infrastructure, with the TUI generating and applying Terraform configurations.

**Pros:**
- Declarative infrastructure definition
- State tracking and drift detection
- Plan/apply workflow shows changes before execution
- Well-established IaC best practice
- Reproducible environments
- Can version control infrastructure

**Cons:**
- **State file management** - need to store and sync terraform.tfstate
- **Slow for simple operations** - `terraform apply` is slow for "just start the VM"
- **Overkill for single resources** - overhead not justified for one VM
- **Poor fit for dynamic operations** - resize/start/stop are imperative, not declarative
- **Complexity** - requires Terraform binary, state backend, locking
- **User friction** - plan/apply workflow adds steps to simple operations

### Option 2: gcloud CLI Commands (Chosen)

Use gcloud CLI directly for all operations. The TUI constructs and executes gcloud commands.

**Pros:**
- **Simple and direct** - one command = one action
- **Fast** - no state reconciliation, immediate execution
- **No state file** - GCP is the source of truth
- **Perfect fit for imperative operations** - start, stop, resize are naturally imperative
- **Minimal dependencies** - only requires gcloud CLI (already needed for auth)
- **Easy to debug** - can run same commands manually
- **Real-time status** - `gcloud compute instances describe` returns current state

**Cons:**
- No drift detection (but single VM rarely drifts)
- No plan preview (but operations are simple and reversible)
- Must handle idempotency manually (check before create)
- Less "infrastructure as code" rigour

### Option 3: Hybrid Approach

Use Terraform for initial setup, gcloud for runtime operations.

**Pros:**
- Initial setup is reproducible via Terraform
- Runtime operations remain simple

**Cons:**
- Two tools to understand and maintain
- Potential for Terraform state and actual state to diverge
- Confusing when to use which tool

## Analysis

### Operation Characteristics

| Operation | Frequency | Nature | Best Fit |
|-----------|-----------|--------|----------|
| Initial setup | Once | Declarative | Either |
| Start VM | Daily | Imperative | gcloud |
| Stop VM | Daily | Imperative | gcloud |
| Resize VM | Weekly | Imperative | gcloud |
| Check status | Frequent | Query | gcloud |
| Add agent | Frequent | Imperative | SSH/tmux |
| Delete/recreate VM | Rare | Declarative | Either |

The vast majority of operations are **imperative** (do this now) rather than **declarative** (ensure this state). gcloud is naturally imperative; Terraform is naturally declarative.

### State Management Complexity

Terraform requires state file management:

```
# With Terraform, you need:
- terraform.tfstate (local or remote)
- State locking (if remote)
- State backup
- Handling state corruption
- Importing existing resources
```

With gcloud, GCP itself is the source of truth:

```bash
# Check if VM exists
gcloud compute instances describe claude-sandbox --zone=europe-north2-a

# If not found, create it
gcloud compute instances create claude-sandbox ...

# No state file to manage
```

### Speed Comparison

```bash
# Terraform: Check state, plan, apply
$ time terraform apply -auto-approve
real    0m45.234s  # Even for a simple start

# gcloud: Direct API call
$ time gcloud compute instances start claude-sandbox --zone=europe-north2-a
real    0m8.127s
```

For a TUI that needs to feel responsive, 8 seconds is acceptable; 45 seconds is not.

### Idempotency

Terraform handles idempotency automatically. With gcloud, we handle it explicitly:

```bash
# Check if exists before create
if ! gcloud compute instances describe claude-sandbox --zone=europe-north2-a &>/dev/null; then
  gcloud compute instances create claude-sandbox ...
fi

# Start is already idempotent (starting a running VM is a no-op)
gcloud compute instances start claude-sandbox --zone=europe-north2-a
```

This is slightly more code but straightforward and predictable.

### When Terraform Makes Sense

Terraform excels when:
- Managing many interdependent resources
- Multiple environments (dev, staging, prod)
- Team collaboration on infrastructure
- Complex networking or IAM policies
- Need for plan/review workflow

None of these apply to a personal single-VM sandbox.

## Implementation

### TUI Command Mapping

| TUI Action | gcloud Command |
|------------|----------------|
| Check VM exists | `gcloud compute instances describe NAME --zone=ZONE` |
| Get VM status | `gcloud compute instances describe NAME --format='value(status)'` |
| Start VM | `gcloud compute instances start NAME --zone=ZONE` |
| Stop VM | `gcloud compute instances stop NAME --zone=ZONE` |
| Resize VM | `gcloud compute instances set-machine-type NAME --machine-type=TYPE` |
| Create VM | `gcloud compute instances create NAME [flags...]` |
| Delete VM | `gcloud compute instances delete NAME --zone=ZONE` |
| Check SA exists | `gcloud iam service-accounts describe EMAIL` |
| Create SA | `gcloud iam service-accounts create NAME` |

### Error Handling

```bash
# Example: Start VM with error handling
output=$(gcloud compute instances start claude-sandbox --zone=europe-north2-a 2>&1)
exit_code=$?

case $exit_code in
  0) echo "VM started successfully" ;;
  1)
    if [[ $output == *"not found"* ]]; then
      echo "VM does not exist - create it first"
    elif [[ $output == *"already running"* ]]; then
      echo "VM is already running"
    else
      echo "Failed to start VM: $output"
    fi
    ;;
esac
```

### Optional Terraform Reference

Provide Terraform configs as documentation/reference for users who prefer IaC:

```
sandbox-setup-template/
├── terraform/           # Reference configs (optional use)
│   ├── main.tf
│   ├── variables.tf
│   └── README.md       # "These are for reference; TUI uses gcloud"
└── scripts/            # TUI uses these
    └── gcloud commands
```

## Consequences

### Positive

- TUI operations are fast and responsive
- No state file to manage, backup, or corrupt
- Simple debugging - run gcloud commands manually
- Minimal dependencies (just gcloud CLI)
- Natural fit for imperative start/stop/resize operations
- GCP console always shows accurate state

### Negative

- No automatic drift detection
- Must handle idempotency checks manually
- Less "infrastructure as code" discipline
- Cannot easily replicate exact setup (mitigated by documenting gcloud commands)

### Neutral

- Users familiar with Terraform may initially expect it
- gcloud commands are well-documented and stable
- Can always add Terraform later if needs change
