# ADR-0010: Cloud-Agnostic Design

## Status

Accepted

## Context

The current design targets GCP exclusively. Users may prefer or require AWS or Azure due to:
- Existing cloud credits or enterprise agreements
- Organizational cloud standards
- Regional availability or compliance requirements
- Familiarity with a specific cloud's tooling

The question is whether to abstract the cloud layer to support multiple providers, and if so, how.

## Decision

Design for **cloud abstraction from the start**, but implement **GCP first** as the reference provider.

Use a provider interface pattern that isolates cloud-specific code, making it straightforward to add AWS and Azure support later.

## Complexity Assessment

### Service Mapping

| Capability | GCP | AWS | Azure |
|------------|-----|-----|-------|
| **VM Service** | Compute Engine | EC2 | Virtual Machines |
| **CLI** | gcloud | aws | az |
| **Spot Instances** | Spot VMs | Spot Instances | Spot VMs |
| **ARM Instances** | C4A (Axion) | Graviton (c7g/c8g) | Cobalt (Dpsv6) |
| **IAM** | Service Accounts | IAM Roles + Instance Profiles | Managed Identities |
| **Secrets** | Secret Manager | Secrets Manager | Key Vault |
| **Regions** | europe-north2 | eu-north-1 | swedencentral |
| **Zones** | europe-north2-a | eu-north-1a | swedencentral (no zones) |

### Conceptual Differences

| Concept | Complexity | Notes |
|---------|------------|-------|
| Start/Stop VM | Low | All clouds support this similarly |
| Resize VM | Low | Stop → change type → start (universal) |
| Create VM | Medium | Different flags, but mappable |
| Machine types | Medium | Naming differs, need mapping table |
| Spot behavior | Medium | Different preemption semantics |
| IAM/Permissions | High | Fundamentally different models |
| Secrets | Medium | Similar concepts, different APIs |
| Networking/Firewall | High | Very different models |

### Effort Estimate

| Component | GCP (done) | AWS (additional) | Azure (additional) |
|-----------|------------|------------------|-------------------|
| VM lifecycle | ✓ | ~2 days | ~2 days |
| Machine type mapping | ✓ | ~1 day | ~1 day |
| IAM setup docs | ✓ | ~2 days | ~2 days |
| Secret management | ✓ | ~1 day | ~1 day |
| Provisioning script | ✓ | ~3 days | ~3 days |
| Testing | ✓ | ~2 days | ~2 days |
| **Total** | Baseline | ~11 days | ~11 days |

## Options Considered

### Option 1: GCP Only

Keep the current GCP-only implementation.

**Pros:**
- Simplest implementation
- No abstraction overhead
- Optimized for one platform
- Faster initial delivery

**Cons:**
- Excludes AWS/Azure users
- Cannot leverage existing cloud credits elsewhere
- Vendor lock-in

### Option 2: Full Abstraction Layer (Chosen)

Define interfaces for cloud operations, implement per provider.

**Pros:**
- Users can choose their preferred cloud
- Leverage existing cloud relationships/credits
- Future-proof for new clouds
- Clean separation of concerns
- Community can contribute providers

**Cons:**
- More upfront design work
- Must maintain multiple implementations
- Lowest common denominator features
- Testing matrix grows

### Option 3: Pulumi/Terraform for Abstraction

Use infrastructure-as-code tools that support multiple clouds.

**Pros:**
- Existing multi-cloud abstraction
- Declarative infrastructure

**Cons:**
- Already rejected Terraform for runtime ops (ADR-0007)
- Adds heavyweight dependency
- Slow for simple operations
- State file management

### Option 4: Container-Based Abstraction

Run everything in containers, abstract at container orchestration level.

**Pros:**
- Cloud-agnostic by nature
- Could use Kubernetes anywhere

**Cons:**
- Over-engineered for single VM use case
- Adds significant complexity
- Higher cost (K8s control plane)

## Design

### Provider Interface

```go
// pkg/cloud/provider.go

type Provider interface {
    // Identity
    Name() string  // "gcp", "aws", "azure"

    // VM Lifecycle
    GetVMStatus(ctx context.Context, name string) (VMStatus, error)
    StartVM(ctx context.Context, name string) error
    StopVM(ctx context.Context, name string) error
    CreateVM(ctx context.Context, config VMConfig) error
    DeleteVM(ctx context.Context, name string) error
    ResizeVM(ctx context.Context, name string, machineType string) error

    // VM Info
    GetVMIP(ctx context.Context, name string) (string, error)
    ListMachineTypes(ctx context.Context) ([]MachineType, error)

    // Authentication
    CheckAuth(ctx context.Context) (AuthStatus, error)

    // Secrets (optional capability)
    SupportsSecrets() bool
    GetSecret(ctx context.Context, name string) (string, error)
    SetSecret(ctx context.Context, name, value string) error
}

type VMStatus string
const (
    VMStatusRunning  VMStatus = "running"
    VMStatusStopped  VMStatus = "stopped"
    VMStatusNotFound VMStatus = "not_found"
    VMStatusUnknown  VMStatus = "unknown"
)

type VMConfig struct {
    Name        string
    MachineType string  // Normalized: "arm-16cpu-32gb"
    DiskSizeGB  int
    Region      string  // Normalized, mapped to cloud-specific
    Spot        bool
    Image       string  // Normalized: "ubuntu-24.04-arm64"
}

type MachineType struct {
    ID       string  // Cloud-specific: "c4a-highcpu-16"
    Normalized string // Our standard: "arm-16cpu-32gb"
    VCPUs    int
    MemoryGB int
    Arch     string  // "arm64", "x86_64"
    SpotCost float64 // $/hour
}
```

### Provider Implementations (Native SDKs)

Per ADR-0011, we use native Go SDKs rather than shelling out to CLI tools.

```go
// internal/cloud/gcp/provider.go
import (
    compute "cloud.google.com/go/compute/apiv1"
    computepb "cloud.google.com/go/compute/apiv1/computepb"
)

type GCPProvider struct {
    project string
    zone    string
    client  *compute.InstancesClient
}

func (p *GCPProvider) StartVM(ctx context.Context, name string) error {
    op, err := p.client.Start(ctx, &computepb.StartInstanceRequest{
        Project:  p.project,
        Zone:     p.zone,
        Instance: name,
    })
    if err != nil {
        return fmt.Errorf("start instance: %w", err)
    }
    return op.Wait(ctx)
}

// internal/cloud/aws/provider.go
import (
    "github.com/aws/aws-sdk-go-v2/service/ec2"
)

type AWSProvider struct {
    region string
    client *ec2.Client
}

func (p *AWSProvider) StartVM(ctx context.Context, name string) error {
    // Look up instance ID by Name tag
    instanceID, err := p.getInstanceIDByName(ctx, name)
    if err != nil {
        return err
    }
    _, err = p.client.StartInstances(ctx, &ec2.StartInstancesInput{
        InstanceIds: []string{instanceID},
    })
    return err
}

// internal/cloud/azure/provider.go
import (
    "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

type AzureProvider struct {
    resourceGroup string
    subscription  string
    client        *armcompute.VirtualMachinesClient
}

func (p *AzureProvider) StartVM(ctx context.Context, name string) error {
    poller, err := p.client.BeginStart(ctx, p.resourceGroup, name, nil)
    if err != nil {
        return fmt.Errorf("begin start: %w", err)
    }
    _, err = poller.PollUntilDone(ctx, nil)
    return err
}
```

### Configuration

```yaml
# ~/.config/sandbox-manager/config.yaml

cloud:
  provider: gcp  # gcp, aws, azure

  gcp:
    project: my-project-id
    zone: europe-north2-a
    service_account: claude-sandbox@my-project.iam.gserviceaccount.com

  aws:
    region: eu-north-1
    instance_id: i-0abc123def456  # Or looked up by tag
    profile: sandbox  # AWS CLI profile

  azure:
    subscription: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    resource_group: claude-sandbox-rg

vm:
  name: claude-sandbox
  machine_type: arm-16cpu-32gb  # Normalized, mapped per cloud
  disk_size_gb: 50
  spot: true

# Machine type mapping (built-in, can override)
machine_types:
  arm-8cpu-16gb:
    gcp: c4a-highcpu-8
    aws: c7g.2xlarge
    azure: Standard_D8pds_v6
  arm-16cpu-32gb:
    gcp: c4a-highcpu-16
    aws: c7g.4xlarge
    azure: Standard_D16pds_v6
  arm-32cpu-64gb:
    gcp: c4a-highcpu-32
    aws: c7g.8xlarge
    azure: Standard_D32pds_v6
```

### Region Mapping

```yaml
# Built-in region normalization
regions:
  europe-nordic:
    gcp: europe-north1      # Finland
    aws: eu-north-1         # Stockholm
    azure: swedencentral    # Gävle
  europe-west:
    gcp: europe-west1       # Belgium
    aws: eu-west-1          # Ireland
    azure: westeurope       # Netherlands
  us-east:
    gcp: us-east1           # South Carolina
    aws: us-east-1          # Virginia
    azure: eastus           # Virginia
```

### TUI Changes

```
┌─────────────────────────────────────────────────────────────────┐
│  Sandbox Manager                                     v0.2.0     │
├─────────────────────────────────────────────────────────────────┤
│  Cloud: GCP (europe-north2)  [Tab to switch]                   │
│                                                                 │
│  Infrastructure Status                                          │
│  VM: claude-sandbox          ● Running                         │
│  Type: c4a-highcpu-16        (arm-16cpu-32gb)                  │
│  ...                                                            │
└─────────────────────────────────────────────────────────────────┘
```

### API Parity Check (Reference)

While we use native SDKs (not CLI), this table shows equivalent operations exist across all clouds:

| Operation | GCP SDK | AWS SDK | Azure SDK |
|-----------|---------|---------|-----------|
| Check auth | Default credentials | Default credentials | Default credentials |
| VM status | `InstancesClient.Get` | `DescribeInstances` | `VirtualMachinesClient.Get` |
| Start VM | `InstancesClient.Start` | `StartInstances` | `VirtualMachinesClient.BeginStart` |
| Stop VM | `InstancesClient.Stop` | `StopInstances` | `VirtualMachinesClient.BeginDeallocate` |
| Resize | `InstancesClient.SetMachineType` | `ModifyInstanceAttribute` | `VirtualMachinesClient.BeginUpdate` |
| Get IP | Instance.NetworkInterfaces | DescribeInstances | NetworkInterfacesClient.Get |

All three cloud SDKs provide equivalent capabilities. The abstraction is feasible.

## Implementation Phases

### Phase 1: GCP Reference (Current)
- Complete GCP implementation
- Define provider interface based on GCP experience
- Document patterns for other providers

### Phase 2: Abstraction Layer
- Refactor GCP code behind provider interface
- Move GCP-specific code to `pkg/cloud/gcp/`
- Update TUI to use provider interface
- Test that GCP still works identically

### Phase 3: AWS Provider
- Implement AWS provider
- Add EC2, IAM, Secrets Manager support
- Document AWS-specific setup
- Test full workflow on AWS

### Phase 4: Azure Provider
- Implement Azure provider
- Add VM, Managed Identity, Key Vault support
- Document Azure-specific setup
- Test full workflow on Azure

## Consequences

### Positive

- Users can choose their preferred/required cloud
- Leverage existing cloud credits and relationships
- Community can contribute and maintain providers
- Clean architecture with good separation of concerns
- Future clouds (Oracle, DigitalOcean, etc.) can be added

### Negative

- More code to maintain
- Must test across multiple clouds
- Some features may not map perfectly across clouds
- Documentation must cover all providers
- IAM setup differs significantly per cloud

### Neutral

- GCP remains the reference implementation
- ~95% of TUI code is cloud-agnostic
- Provisioning scripts need per-cloud versions
- Configuration grows but remains manageable
