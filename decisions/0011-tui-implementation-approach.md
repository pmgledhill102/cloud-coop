# ADR-0011: TUI Implementation Approach

## Status

Accepted

## Context

The sandbox manager needs a terminal-based user interface. Key requirements:
- Interactive dashboard showing VM and agent status
- Keyboard-driven navigation
- Real-time updates
- Cross-platform (macOS, Linux)
- Easy distribution

Several implementation decisions need to be made:
1. Language: Shell scripts vs Go vs other
2. TUI framework (if Go): Cobra, Bubbletea, Tview, etc.
3. Cloud interaction: CLI tools (gcloud/aws/az) vs native SDKs
4. Repository structure: Monorepo vs plugins

## Decision

1. **Language**: Go
2. **Framework**: Bubbletea (TUI) + Cobra (CLI subcommands)
3. **Cloud interaction**: Native Go SDKs
4. **Structure**: Single repository with internal packages

## Options Considered

### Language Choice

#### Option A: Shell Scripts (Bash)

```bash
#!/bin/bash
# Simple but limited

show_status() {
    status=$(gcloud compute instances describe claude-sandbox --format='value(status)')
    echo "VM Status: $status"
}
```

**Pros:**
- Quick to prototype
- No compilation needed
- Direct CLI tool integration
- Universal on Unix systems

**Cons:**
- Poor error handling
- Difficult to build complex TUI
- Parsing CLI output is fragile
- Hard to test
- No type safety
- Cross-platform issues (macOS vs Linux bash differences)

#### Option B: Python + Textual

```python
from textual.app import App
from textual.widgets import Static

class SandboxManager(App):
    def compose(self):
        yield Static("VM Status: Running")
```

**Pros:**
- Textual is a powerful TUI framework
- Familiar to many developers
- Good async support

**Cons:**
- Requires Python runtime
- Virtual environment management
- Slower startup than compiled language
- Distribution complexity (pip, poetry, etc.)

#### Option C: Go (Chosen)

```go
package main

import tea "github.com/charmbracelet/bubbletea"

func main() {
    p := tea.NewProgram(initialModel())
    p.Run()
}
```

**Pros:**
- Single static binary - trivial distribution
- Fast startup and execution
- Excellent cloud SDKs for all major providers
- Strong typing and error handling
- Great concurrency primitives (goroutines for parallel status checks)
- Cross-compilation for macOS/Linux
- Mature TUI ecosystem (Bubbletea, Lipgloss)

**Cons:**
- Longer initial development vs shell scripts
- Must compile for distribution
- Steeper learning curve than Python

#### Option D: Rust + Ratatui

**Pros:**
- Performance and safety
- Single binary
- Growing TUI ecosystem

**Cons:**
- Steeper learning curve
- Slower development velocity
- Smaller ecosystem for cloud SDKs

### TUI Framework (Go)

#### Cobra

Cobra is a **CLI framework**, not a TUI framework. It handles:
- Command structure (`sandbox start`, `sandbox stop`)
- Flag parsing (`--zone`, `--machine-type`)
- Help text generation
- Shell completions

**Use for:** Subcommand structure and flags.

#### Bubbletea (Chosen for TUI)

Bubbletea is an **interactive TUI framework** based on The Elm Architecture:
- Model: Application state
- Update: Handle events, update state
- View: Render state to terminal

```go
type model struct {
    vmStatus   string
    agents     []Agent
    cursor     int
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "s":
            return m, startVM
        case "q":
            return m, tea.Quit
        }
    }
    return m, nil
}

func (m model) View() string {
    return lipgloss.NewStyle().Render(
        fmt.Sprintf("VM: %s\n[S]tart [T]op [Q]uit", m.vmStatus),
    )
}
```

**Use for:** Interactive dashboard, real-time updates, keyboard navigation.

#### Tview

Alternative TUI framework with more traditional widget model.

**Pros:**
- Widget-based (forms, tables, lists)
- Familiar to GUI developers

**Cons:**
- Less flexible than Bubbletea
- Harder to customize styling

#### Recommended Combination

```
┌─────────────────────────────────────────────────┐
│  Cobra                                          │
│  (subcommands: start, stop, status, agents)     │
│                                                 │
│  ┌───────────────────────────────────────────┐  │
│  │  Bubbletea                                │  │
│  │  (interactive TUI for 'sandbox' command)  │  │
│  │                                           │  │
│  │  ┌─────────────────────────────────────┐  │  │
│  │  │  Lipgloss                           │  │  │
│  │  │  (styling, colors, borders)         │  │  │
│  │  └─────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

### Cloud Interaction: CLI vs SDK

#### Option A: Shell Out to CLI Tools

```go
func getVMStatus() (string, error) {
    cmd := exec.Command("gcloud", "compute", "instances", "describe",
        "claude-sandbox", "--zone=europe-north2-a", "--format=value(status)")
    output, err := cmd.Output()
    return strings.TrimSpace(string(output)), err
}
```

**Pros:**
- Simple to implement
- Matches what users do manually
- Auth handled by CLI tool
- Less code

**Cons:**
- Requires CLI tools installed (`gcloud`, `aws`, `az`)
- Parsing text output is fragile
- Slower (process spawn overhead)
- Error messages vary by CLI version
- Harder to test (need CLI tools in CI)

#### Option B: Native Go SDKs (Chosen)

```go
import compute "cloud.google.com/go/compute/apiv1"

func getVMStatus(ctx context.Context) (string, error) {
    client, err := compute.NewInstancesRESTClient(ctx)
    if err != nil {
        return "", err
    }
    defer client.Close()

    instance, err := client.Get(ctx, &computepb.GetInstanceRequest{
        Project:  "my-project",
        Zone:     "europe-north2-a",
        Instance: "claude-sandbox",
    })
    return instance.GetStatus(), err
}
```

**Pros:**
- Single binary - no external dependencies
- Type-safe API responses
- Proper error types
- Faster (no process spawn)
- Testable (can mock clients)
- Consistent across platforms

**Cons:**
- More code initially
- Must handle auth explicitly
- SDK updates needed for new features

#### SDK Maturity Assessment

| Provider | SDK | Maturity | Notes |
|----------|-----|----------|-------|
| GCP | google-cloud-go | Excellent | First-class Go support, auto-generated from APIs |
| AWS | aws-sdk-go-v2 | Excellent | Very mature, comprehensive coverage |
| Azure | azure-sdk-for-go | Good | Significantly improved, now comprehensive |

All three providers have production-quality Go SDKs. Azure has invested heavily in their Go SDK over the past few years - my initial assumption about limited support was outdated.

**Verification:**
```go
// GCP - Compute Engine
import compute "cloud.google.com/go/compute/apiv1"

// AWS - EC2
import "github.com/aws/aws-sdk-go-v2/service/ec2"

// Azure - Compute
import "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
```

### Repository Structure

#### Option A: Multi-Repo with Plugins

```
github.com/org/sandbox-manager        # Core
github.com/org/sandbox-manager-gcp    # GCP provider
github.com/org/sandbox-manager-aws    # AWS provider
github.com/org/sandbox-manager-azure  # Azure provider
```

**Pros:**
- Independent release cycles
- Community can own providers
- Smaller core binary

**Cons:**
- Complex dependency management
- Version compatibility issues
- Harder to ensure consistency
- Overhead not justified for 3 providers

#### Option B: Monorepo with Internal Packages (Chosen)

```
sandbox-manager/
├── cmd/
│   └── sandbox/           # Main binary
│       └── main.go
├── internal/
│   ├── tui/              # Bubbletea TUI
│   │   ├── model.go
│   │   ├── update.go
│   │   └── view.go
│   ├── cloud/            # Provider interface
│   │   ├── provider.go   # Interface definition
│   │   ├── gcp/          # GCP implementation
│   │   ├── aws/          # AWS implementation
│   │   └── azure/        # Azure implementation
│   ├── agent/            # Agent configuration
│   │   └── config.go
│   └── ssh/              # SSH/tmux operations
│       └── session.go
├── pkg/                  # Public APIs (if any)
├── configs/              # Default configurations
└── docs/                 # Documentation
```

**Pros:**
- Single repository to manage
- Atomic changes across providers
- Shared testing infrastructure
- Simple dependency management
- Consistent versioning

**Cons:**
- All providers compiled into binary (larger size)
- Can't release providers independently

**Size consideration:** With all three SDKs, binary might be ~50MB. Acceptable for a development tool.

## Implementation Details

### Build Tags for Optional Providers

If binary size becomes a concern, use build tags:

```go
// internal/cloud/gcp/provider.go
//go:build gcp || all

package gcp
```

```bash
# Build with all providers
go build -tags all ./cmd/sandbox

# Build with only GCP
go build -tags gcp ./cmd/sandbox
```

### Authentication Flow

```go
// internal/cloud/gcp/auth.go
func NewClient(ctx context.Context) (*compute.InstancesClient, error) {
    // SDK auto-detects credentials:
    // 1. GOOGLE_APPLICATION_CREDENTIALS env var
    // 2. gcloud auth application-default credentials
    // 3. Compute Engine metadata (if on GCP)
    return compute.NewInstancesRESTClient(ctx)
}

// internal/cloud/aws/auth.go
func NewClient(ctx context.Context) (*ec2.Client, error) {
    // SDK auto-detects credentials:
    // 1. Environment variables
    // 2. Shared credentials file (~/.aws/credentials)
    // 3. EC2 instance metadata
    cfg, err := config.LoadDefaultConfig(ctx)
    if err != nil {
        return nil, err
    }
    return ec2.NewFromConfig(cfg), nil
}
```

### Example TUI Structure

```go
// internal/tui/model.go
type Model struct {
    provider    cloud.Provider
    vmStatus    VMStatus
    agents      []agent.Session
    cursor      int
    width       int
    height      int
    err         error
}

// internal/tui/commands.go
func refreshStatus(p cloud.Provider) tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        status, err := p.GetVMStatus(ctx, "claude-sandbox")
        return statusMsg{status: status, err: err}
    }
}

// internal/tui/view.go
func (m Model) View() string {
    var b strings.Builder

    // Header
    b.WriteString(headerStyle.Render("Sandbox Manager"))
    b.WriteString("\n\n")

    // VM Status
    statusColor := lipgloss.Color("#00ff00")
    if m.vmStatus != VMStatusRunning {
        statusColor = lipgloss.Color("#ff0000")
    }
    b.WriteString(fmt.Sprintf("VM: %s\n",
        lipgloss.NewStyle().Foreground(statusColor).Render(string(m.vmStatus))))

    // Agents list
    for i, agent := range m.agents {
        cursor := " "
        if i == m.cursor {
            cursor = ">"
        }
        b.WriteString(fmt.Sprintf("%s %s\n", cursor, agent.Name))
    }

    // Help
    b.WriteString("\n[S]tart [T]op [R]esize [A]dd [Q]uit")

    return b.String()
}
```

## Consequences

### Positive

- Single static binary - download and run
- Fast startup and responsive UI
- Type-safe cloud interactions
- Testable with mocked cloud clients
- Professional TUI with Bubbletea/Lipgloss
- Cross-platform from single codebase

### Negative

- Larger binary (~50MB with all SDKs)
- Go knowledge required for contributions
- Initial development slower than shell scripts
- Must update SDK dependencies periodically

### Neutral

- Cobra provides familiar CLI patterns (`sandbox start --help`)
- Bubbletea learning curve (Elm Architecture)
- Can always shell out to CLI tools as fallback for missing SDK features
