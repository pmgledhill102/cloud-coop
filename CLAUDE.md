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
- Cost-optimized with spot instances

## Architecture Summary

```text
Workstation (cloudcoop TUI)
    |
    | SSH + Cloud SDK
    v
Cloud VM (GCP/AWS/Azure)
    |
    +-- tmux: agents session
         |
         +-- agent-1 (claude)
         +-- agent-2 (claude)
         +-- agent-3 (aider)
         ...
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

- Agents run in tmux windows within an "agents" session
- Support multiple agent types with different CLI flags
- Session modes: Fresh, Continue (resume most recent), Pick (select from available)

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

## Issue Tracking

This project uses beads (bd) for git-backed issue tracking. See AGENTS.md for workflow.

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```
