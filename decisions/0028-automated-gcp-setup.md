# ADR-0028: Automated GCP Project Setup

## Status

Accepted

## Context

Users must manually run approximately 6 gcloud commands to set up a GCP project before they can use
cloudcoop. This creates significant friction between installing the binary and actually using it.
The steps include:

- Selecting or creating a GCP project
- Enabling required APIs (Compute Engine, IAM, Cloud Logging, Cloud Monitoring)
- Creating a dedicated service account (cloudcoop-vm)
- Granting IAM roles (logging.logWriter, monitoring.metricWriter)
- Creating an IAP firewall rule for secure SSH access

The existing `config init` command only writes TOML configuration files and doesn't provision any
GCP resources. This manual setup process is error-prone, difficult to maintain as requirements
change, and presents a poor first-run experience. New users must read documentation, execute
multiple commands, and manually verify each step before they can provision their first VM.

Additionally, the single-level configuration at `~/.config/cloudcoop/cloudcoop.toml` makes it
difficult to share project-specific settings across team members.

## Decision

Implement a `cloudcoop setup` command that automates GCP project provisioning and introduces
layered configuration:

1. **Global configuration**: `~/.config/cloudcoop/cloudcoop.toml` contains user-specific settings
   (default VM size, SSH keys, etc.)

2. **Project configuration**: `.cloudcoop/config.toml` contains project-specific settings (GCP
   project ID, region, service account, etc.) and is checked into git for team sharing

The `cloudcoop setup` command performs the following idempotent operations:

- Verifies prerequisites (SSH key exists, Application Default Credentials configured)
- Lists available GCP projects and prompts user to select one
- Enables required GCP APIs (compute.googleapis.com, iam.googleapis.com, logging.googleapis.com,
  monitoring.googleapis.com, serviceusage.googleapis.com, cloudresourcemanager.googleapis.com)
- Creates a dedicated service account (`cloudcoop-vm@PROJECT.iam.gserviceaccount.com`) if it
  doesn't exist
- Grants required IAM roles to the service account (logging.logWriter, monitoring.metricWriter)
- Creates an IAP firewall rule to allow SSH access from Identity-Aware Proxy
- Writes project configuration to `.cloudcoop/config.toml`

All operations are idempotent and safe to re-run. If resources already exist, the command verifies
their configuration and skips creation.

## Options Considered

### Option 1: Manual gcloud Commands (Status Quo)

Continue requiring users to manually run gcloud commands following documentation.

**Pros:**

- No additional code to write or maintain
- Users have full visibility and control
- Familiar to experienced GCP users

**Cons:**

- High friction for new users
- Error-prone (typos, wrong project, forgotten steps)
- Hard to maintain as requirements evolve
- No verification that setup was done correctly
- Different users may set up projects inconsistently

### Option 2: Automated Setup via cloudcoop CLI (Chosen)

Implement `cloudcoop setup` command to automate project provisioning.

**Pros:**

- Dramatically reduces friction for new users
- Idempotent - safe to re-run to fix gaps or verify setup
- Discoverable via `cloudcoop --help`
- Consistent setup across all users
- Easy to update as requirements change
- Layered configuration enables team sharing via git
- Validates prerequisites and provides clear error messages

**Cons:**

- Adds GCP SDK dependencies (Resource Manager, Service Usage, IAM Admin APIs)
- Requires user to have project-level IAM permissions to run setup
- Additional code to write and maintain
- May be opaque to users who prefer to understand each step

### Option 3: Terraform/Infrastructure-as-Code

Provide Terraform modules for provisioning GCP resources.

**Pros:**

- Industry-standard infrastructure tooling
- Declarative and easy to audit
- Version-controlled infrastructure state
- Can be integrated with existing Terraform workflows

**Cons:**

- Requires users to install and learn Terraform
- Separate toolchain outside cloudcoop CLI
- Too heavy for this use case
- Doesn't integrate with cloudcoop's interactive setup flow
- Harder to make idempotent across different execution environments

## Consequences

### Positive

- Dramatically reduces friction for new users - from 6+ manual commands to a single `cloudcoop setup`
- Idempotent operations allow safe re-runs to fix configuration gaps
- Layered configuration (global + project) enables team sharing via git
- Consistent project setup across all users
- Setup requirements automatically stay in sync as cloudcoop evolves
- Better error messages and prerequisite validation
- Easier to onboard new team members (just clone repo and run setup)

### Negative

- Adds GCP SDK dependencies for Resource Manager, Service Usage, and IAM Admin APIs (increases
  binary size and complexity)
- Requires user to have project-level IAM permissions (roles/owner or equivalent) to run setup
- Users with restricted permissions may not be able to run setup and will need admin assistance
- Additional maintenance burden for setup code
- May obscure what's happening under the hood for users who want full visibility

### Neutral

- Setup command coexists with manual setup - users can still run gcloud commands if preferred
- Project configuration in `.cloudcoop/config.toml` should be added to `.gitignore` by teams that
  don't want to share project settings (though sharing is recommended)
- The setup command documents the exact requirements, serving as executable documentation
