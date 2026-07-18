# CLAUDE.md

Project context for AI assistants working on cloudcoop.

## Project Overview

cloudcoop is a terminal UI (TUI) for managing sandboxed AI coding agents on cloud VMs. It provisions
and manages cloud VMs configured as secure sandboxes for running multiple AI coding agents (Claude
Code, Aider, Gemini CLI, etc.) in tmux sessions.

**Key capabilities:**

- VM lifecycle management (start, stop, resize)
- Agent session management via tmux
- Cloud-agnostic: GCP first, AWS and Azure planned
- Agent-agnostic: supports Claude Code, Aider, Gemini CLI
- Automatic IP-based firewall rules
- Cost-optimised with spot instances

## Architecture Summary

```text
Workstation (cloudcoop TUI/CLI)
    |
    | SSH + Cloud SDK
    v
Cloud VM (GCP/AWS/Azure)
    |
    +-- tmux: acme-backend       (per-repo session)
    |    +-- main (claude)
    |    +-- feature-auth (claude)
    |    +-- fix-bug-42 (aider)
    |
    +-- tmux: acme-frontend      (per-repo session)
         +-- main (claude)
         +-- redesign (gemini)
```

**Technology stack:**

- Language: Go 1.25+
- TUI Framework: Bubbletea + Lipgloss (styling)
- CLI Framework: Cobra
- Cloud: Native Go SDKs (not CLI wrappers)
- Remote execution: Go SSH library for commands, shell out to ssh for interactive
- Configuration: TOML at ~/.config/cloudcoop/cloudcoop.toml (see ADR-0014)

**Project structure:**

```text
cmd/              # Main binary entry point
internal/
  tui/            # Bubbletea TUI components
  cloud/          # Provider interface + implementations (gcp/, aws/, azure/)
  setup/          # GCP project setup automation (setup command)
  agent/          # Agent configuration
  ssh/            # SSH/tmux operations
config/           # Default configurations
decisions/        # Architecture Decision Records (ADRs)
docs/             # Documentation
```

## Key Commands

```bash
# Build and development
make build        # Compile the binary to bin/cloudcoop
make test         # Run all tests
make lint         # Run golangci-lint
make fmt          # Format Go source files
make all          # Format, lint, test, and build
make clean        # Remove build artifacts
make help         # Show all available targets

# First-time setup
cloudcoop setup   # Automated GCP project provisioning + config
```

## Important Patterns and Constraints

### Cloud Interaction

- Use native Go SDKs, not CLI wrappers (ADR-0011)
- SDKs auto-detect credentials (gcloud ADC, AWS credentials file, etc.)
- All three major cloud SDKs are mature and production-ready

### SSH and Remote Execution

- Use Go SSH library for programmatic commands (listing sessions, etc.)
- Shell out to native `ssh` for interactive terminal attachment (ADR-0013)
- Use IAP tunnel for secure access when external IP not available

### TUI Design

- Follow The Elm Architecture: Model, Update, View
- Keyboard-driven navigation
- Single-letter shortcuts for quick actions (S=Start, T=Stop, R=Resize, etc.)
- Status checks should complete in <2 seconds
- SSH operations timeout at 5s default

### Agent Sessions

- Agents run in tmux windows within per-repo sessions (ADR-0023)
- Each repo gets its own tmux session named by slug (e.g., `acme-backend`)
- `cloudcoop agents sync` sets up worktree-based workspaces on the VM (ADR-0024)
- `cloudcoop agents attach --next` connects via grouped tmux sessions (ADR-0025)
- Support multiple agent types with configurable startup hooks (ADR-0027)

### Code Style

- Use golangci-lint with project .golangci.yml config
- Run gofmt and goimports for formatting
- Pre-commit hooks enforce quality gates

## Key Documentation

- [TUI-REQUIREMENTS.md](docs/TUI-REQUIREMENTS.md) - Detailed TUI specification
- [decisions/](decisions/README.md) - All Architecture Decision Records
- [SETUP-FLOW.md](docs/SETUP-FLOW.md) - First-run experience and setup wizard
- [DEVELOPMENT-ENVIRONMENT.md](docs/DEVELOPMENT-ENVIRONMENT.md) - Contributing guide
- [SECURITY-MODEL.md](docs/SECURITY-MODEL.md) - Trust boundaries and privilege model
- [SECURITY.md](docs/SECURITY.md) - Operational security and incident response
- [MULTI-AGENT-WORKFLOW.md](docs/MULTI-AGENT-WORKFLOW.md) - Multi-repo agent workflow

## Issue Tracking

This project uses GitHub Issues. See AGENTS.md for workflow and
`agentic-coding-config` `docs/github-issues-workflow.md` for conventions.

```bash
gh issue list --search "is:open -is:blocked"   # Find available work
gh issue view <n>                              # View issue details
gh issue edit <n> --add-assignee @me           # Claim work
gh issue close <n> --comment "Shipped in #<pr>"  # Complete work
```
