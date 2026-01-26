# ADR-0020: VM Service Account Requirement

## Status

Accepted

## Context

VMs created by cloudcoop run AI coding agents that execute arbitrary code. Without explicit service
account configuration, GCP assigns the default Compute Engine service account which has Editor-level
permissions at the project level. This includes:

- `compute.admin` - create/delete VMs, modify firewall rules
- `storage.admin` - read/write all buckets
- `iam.serviceAccountUser` - impersonate other service accounts

If an AI agent is compromised or behaves unexpectedly, it could escalate privileges and access
sensitive resources across the entire GCP project. This violates the principle of least privilege.

## Decision

Require a dedicated service account for all VMs:

1. **GCP**: The `cloud.gcp.service_account` configuration field is required. VM creation fails without it.

2. **AWS (future)**: Will require an `InstanceProfile` configuration for IAM instance profiles.

3. **Azure (future)**: Will require a `ManagedIdentity` configuration for managed identities.

The cloud-agnostic `VMCreateConfig` struct includes a `ServiceAccount` field that each provider
implementation validates and uses appropriately.

## Options Considered

### Option 1: Required Service Account (Chosen)

Fail VM creation if service account not configured.

**Pros:**

- Enforces security by default
- Clear error message guides users to setup docs
- Prevents accidental use of overly-permissive default

**Cons:**

- Additional setup step for users
- Requires creating service account before using cloudcoop

### Option 2: Optional with Warning

Allow creation with default service account but log a warning.

**Pros:**

- Easier initial setup
- Non-blocking for quick testing

**Cons:**

- Users may ignore warnings and run insecure in production
- Security is opt-in rather than enforced

### Option 3: Auto-create Service Account

cloudcoop creates and manages its own service account.

**Pros:**

- Zero-friction setup
- Guaranteed correct permissions

**Cons:**

- Requires cloudcoop to have IAM permissions
- Violates separation of concerns
- Harder to audit/customize permissions

## Consequences

### Positive

- VMs run with least-privilege by default
- Users explicitly choose what permissions to grant
- Aligns implementation with security documentation
- Defense in depth for AI agent containment

### Negative

- Adds setup complexity (one-time gcloud commands)
- Existing users must update config before upgrading

### Neutral

- Service account setup documented in SETUP-FLOW.md
- Error messages direct users to documentation
