# ADR-0021: Provisioning Script Location

## Status

Accepted

## Context

cloudcoop needs to provision VMs with development tooling when they're created. The current
`scripts/provision-vm.sh` is a ~660-line bash script that installs:

- Multiple language runtimes (Node, Python, Go, Rust, Java, Ruby, PHP, .NET)
- Development tools and linters (golangci-lint, eslint, prettier, etc.)
- Cloud CLIs (gcloud, aws, az)
- Container tools (Docker, kubectl, helm)
- Agent management scripts (start-agents.sh, stop-agents.sh, etc.)

This script must be passed to the cloud provider's VM creation API (GCP startup-script metadata,
AWS user-data, Azure custom-data). We need to decide where this script should live and how it
gets to the VM.

Key considerations:

1. **Versioning** - Script changes should be traceable and match the binary version
2. **Customization** - Users may want to modify the script for their environment
3. **Offline use** - Should work without network access after installation
4. **Multi-cloud** - Must work across GCP, AWS, and Azure
5. **Updates** - How do users get script improvements?
6. **Size** - Script is currently ~25KB, could grow

## Decision

Use **remote URL fetch** with a **default URL pointing to the script in the cloudcoop
repository**. Allow users to override via configuration.

```toml
[provisioning]
# Default (if not specified):
# https://raw.githubusercontent.com/pmgledhill102/cloudcoop/main/scripts/provision-vm.sh
script_url = "https://example.com/my-custom-provision.sh"
```

**Behavior:**

1. If `script_url` is not set in config, use the default URL pointing to `scripts/provision-vm.sh` in the cloudcoop repo
2. If `script_url` is set, fetch from that URL instead
3. Script is fetched at VM creation time and passed to the cloud provider's startup-script mechanism

**Why this approach:**

- Simple implementation - just fetch a URL
- Default works out of the box (points to managed script in this repo)
- Users can override to point to forks, custom scripts, or internal URLs
- Future-proof: default URL can later point to a separate repo without changing the interface
- Keeps provisioning script in the codebase for now, reducing maintenance overhead

## Provisioning Script Contract

Any provisioning script (default or custom) **must** implement the following contract to ensure
cloudcoop can monitor progress and gate agent creation.

### Status Reporting

The script must write status to a well-known location that cloudcoop can query via SSH:

```bash
STATUS_FILE="/var/run/cloudcoop/provision-status"
LOG_FILE="/var/log/cloudcoop/provision.log"

# Status values: pending | running | completed | failed
echo "running" > "$STATUS_FILE"
```

**Required status transitions:**

1. `pending` - initial state (or file doesn't exist yet)
2. `running` - script has started execution
3. `completed` - script finished successfully
4. `failed` - script encountered an error

**Status file format:**

```text
<status>
[optional: error message if failed]
```

Example success:

```text
completed
```

Example failure:

```text
failed
apt-get install failed: Unable to locate package nodejs
```

### Progress Reporting (Optional)

Scripts may optionally report progress for TUI display:

```bash
PROGRESS_FILE="/var/run/cloudcoop/provision-progress"

# Format: <current_step>/<total_steps> <description>
echo "3/12 Installing Node.js" > "$PROGRESS_FILE"
```

### Idempotency Requirement

The script **must** be idempotent - safe to run multiple times without side effects. This enables:

- **Retry on failure** - re-run after transient errors (network, package mirrors)
- **Refresh/update** - re-run to update tool versions
- **Resume after preemption** - spot instance stopped mid-provision

**Implementation patterns for idempotency:**

```bash
# Check before install
command -v node &> /dev/null || apt-get install -y nodejs

# Use package manager's idempotent behaviour
apt-get install -y nodejs  # Already installed = no-op

# Guard file creation
[ -f /etc/cloudcoop/setup-complete ] || run_one_time_setup

# Conditional download
[ -f /usr/local/bin/tool ] || wget -O /usr/local/bin/tool https://...
```

### Exit Code Requirements

- Exit `0` on success (write `completed` to status file)
- Exit non-zero on failure (write `failed` + error message to status file)
- Trap errors to ensure status file is updated on unexpected failures:

```bash
set -e
trap 'echo -e "failed\n$BASH_COMMAND failed with exit code $?" > "$STATUS_FILE"' ERR

# ... provisioning steps ...

echo "completed" > "$STATUS_FILE"
exit 0
```

### Agent Creation Gate

cloudcoop **must not** allow agent creation until:

1. Status file exists
2. Status is `completed`

The TUI and CLI will:

- Poll status file via SSH during/after VM creation
- Display progress in status bar
- Block `cloudcoop agents start` if status != `completed`
- Offer `cloudcoop provision retry` to re-run on failure

### Well-Known Paths

| Path | Purpose |
|------|---------|
| `/var/run/cloudcoop/provision-status` | Current status (pending/running/completed/failed) |
| `/var/run/cloudcoop/provision-progress` | Optional progress (e.g., "3/12 Installing Node.js") |
| `/var/log/cloudcoop/provision.log` | Full provisioning output |
| `/etc/cloudcoop/` | Persistent configuration created by provisioning |

### Minimum Contract Example

A minimal compliant provisioning script:

```bash
#!/bin/bash
set -e

STATUS_DIR="/var/run/cloudcoop"
STATUS_FILE="$STATUS_DIR/provision-status"
LOG_FILE="/var/log/cloudcoop/provision.log"

mkdir -p "$STATUS_DIR" /var/log/cloudcoop
exec > >(tee -a "$LOG_FILE") 2>&1

trap 'echo -e "failed\n$BASH_COMMAND failed" > "$STATUS_FILE"' ERR

echo "running" > "$STATUS_FILE"

# ... idempotent provisioning steps ...

echo "completed" > "$STATUS_FILE"
exit 0
```

## Options Considered

### Option 1: Embed in Binary (go:embed)

Use Go's `embed` package to compile the script directly into the cloudcoop binary.

```go
//go:embed scripts/provision-vm.sh
var provisionScript string
```

**Pros:**

- Single binary distribution - no additional files to manage
- Script version always matches binary version
- Works offline after installation
- No network calls during VM creation
- Atomic updates - new binary includes new script
- Cross-platform consistent (script is identical everywhere)

**Cons:**

- No customization without recompiling
- Binary size increases (~25KB, negligible)
- Must rebuild/release for script-only changes
- Users can't easily inspect the script before running

### Option 2: External File with Config Path

Store script on disk, reference path in `cloudcoop.toml`:

```toml
[provisioning]
script_path = "~/.config/cloudcoop/provision-vm.sh"
```

**Pros:**

- Users can inspect and modify the script
- Updates possible without binary rebuild
- Familiar pattern (like SSH config files)
- Easy to version control separately

**Cons:**

- Additional file to distribute and manage
- Script/binary version mismatch possible
- Must handle missing file gracefully
- Platform-specific path handling
- Users must manually update script for improvements

### Option 3: Remote URL Fetch

Fetch script from URL at runtime:

```toml
[provisioning]
script_url = "https://raw.githubusercontent.com/owner/cloudcoop/main/scripts/provision-vm.sh"
# or
script_url = "gs://cloudcoop-assets/provision-vm.sh"
```

**Pros:**

- Always gets latest script (if using `main` branch)
- No local file management
- Can point to custom/fork
- Easy to update centrally

**Cons:**

- Network dependency during VM creation
- Security risk - remote content could change unexpectedly
- GitHub rate limits could cause failures
- Harder to audit what's running
- Breaks air-gapped/offline environments
- Version pinning requires managing URLs manually

### Option 4: Inline in Configuration

Store entire script in TOML config (or reference multiline string):

```toml
[provisioning]
script = """
#!/bin/bash
set -e
apt-get update
# ... 600+ more lines
"""
```

**Pros:**

- All configuration in one place
- Easy to customize

**Cons:**

- Config file becomes unwieldy (~700+ lines)
- Difficult to edit (no syntax highlighting)
- TOML escaping issues with complex bash
- Not practical for scripts of this size

### Option 5: Hybrid - Embedded Default with Override

Embed script in binary as the default, but allow override via config:

```toml
[provisioning]
# Optional - if set, uses this instead of embedded default
script_path = "/path/to/custom-provision.sh"
# Or fetch from URL
script_url = "https://example.com/my-provision.sh"
```

**Pros:**

- Works out of the box (embedded default)
- Power users can customize
- Offline-capable by default
- Version-matched default, custom override when needed
- Best of both worlds

**Cons:**

- More complex implementation
- Two code paths to maintain
- Potential confusion about which script is running
- Custom scripts may diverge from improvements in default

### Option 6: Cloud-Native Script Storage

Store script in cloud provider's native mechanism:

- GCP: Cloud Storage bucket or Secret Manager
- AWS: S3 bucket or Systems Manager Parameter Store
- Azure: Blob Storage or Key Vault

```toml
[provisioning]
gcp_bucket = "gs://my-project-cloudcoop/provision.sh"
aws_s3 = "s3://my-bucket/provision.sh"
```

**Pros:**

- Leverages cloud IAM for access control
- Fast access from within cloud (same region)
- Can use cloud versioning/audit features

**Cons:**

- Cloud-specific implementation for each provider
- Requires bucket setup before first use
- Adds cloud dependencies during provisioning
- More complex for users to configure
- Breaks local development/testing

### Option 7: Modular Script with User Extensions

Embed a core script, but support user extension scripts:

```toml
[provisioning]
# These run after the core provisioning
extra_scripts = [
    "~/.config/cloudcoop/post-provision.sh",
    "./project-setup.sh"
]
```

**Pros:**

- Core stays stable and tested
- Users extend rather than replace
- Easier to merge upstream improvements
- Lower risk of breaking base functionality

**Cons:**

- Users can't remove/change core behaviour
- Multiple scripts increase complexity
- Ordering and error handling between scripts
- Still need to decide where core script lives

### Option 8: Separate Repository

Extract provisioning into a standalone project (e.g., `cloudcoop-provision` or
`agent-sandbox-images`). Users configure which repo/release to use:

```toml
[provisioning]
# Default: official cloudcoop provisioning repo
repo = "github.com/cloudcoop/provision"
version = "v1.2.0"  # or "latest"

# Or point to a fork/custom repo
repo = "github.com/myorg/custom-provision"
version = "main"
```

The separate repo could contain:

- `provision.sh` - main provisioning script
- `versions.env` - tool versions
- `extras/` - optional add-on scripts (ML tools, specific frameworks, etc.)
- Release tags for version pinning
- CI/CD to test provisioning on actual VMs

**Pros:**

- **Clean separation of concerns** - cloudcoop is the TUI/CLI, provisioning is VM image definition
- **Independent release cycles** - fix provisioning bugs without cloudcoop release
- **Community contributions** - lower barrier to contribute provisioning improvements
- **Fork-friendly** - organizations fork the repo, maintain their stack, pull upstream updates
- **Reusable** - other tools could use the same provisioning (not tied to cloudcoop)
- **Testable in isolation** - CI can spin up VMs and test provisioning independently
- **Discoverable** - users browse repo to see exactly what gets installed
- **Version flexibility** - pin to stable release or track latest
- **Multiple profiles** - repo could offer `minimal.sh`, `full.sh`, `ml.sh`, etc.

**Cons:**

- **Two repos to maintain** - more release coordination
- **Version compatibility** - must document which provision versions work with which cloudcoop versions
- **Network dependency** - must fetch from GitHub/registry during VM creation
- **First-run complexity** - users need network access on first provision
- **Fork maintenance burden** - forked repos may fall behind upstream

**Mitigations:**

- Embed a "bootstrap" version in cloudcoop binary as fallback
- Cache fetched scripts locally for offline re-use
- Compatibility matrix in documentation
- Dependabot/Renovate to keep forks updated

## Consequences

### Positive

- **Simple implementation** - URL fetch is straightforward
- **Works out of the box** - default URL requires no user configuration
- **Customizable** - users override with their own URL when needed
- **Inspectable** - users can view the script at the URL before running
- **Future-proof** - can migrate to separate repo by changing default URL
- **No binary rebuilds** - script updates don't require cloudcoop release

### Negative

- **Network dependency** - requires internet access during VM creation
- **GitHub dependency** - default relies on GitHub raw content availability
- **No offline mode** - can't provision VMs without network
- **Caching complexity** - may want to cache fetched scripts locally

### Neutral

- Script remains in cloudcoop repo for now, reducing maintenance overhead
- Migration path to separate repo is clear if needed later
- Version pinning possible via URL (e.g., point to specific commit or tag)

### Future Migration Path

If provisioning complexity grows or community contributions increase, the default URL can be
changed to point to a separate `cloudcoop-provision` repository. This requires no changes to
user configurations - only the hardcoded default changes.
