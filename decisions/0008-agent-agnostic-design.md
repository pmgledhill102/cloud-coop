# ADR-0008: Agent-Agnostic Design

## Status

Accepted

## Context

The sandbox infrastructure was initially designed for Claude Code, but the core components are generic:

| Component | Claude-Specific? | Notes |
|-----------|------------------|-------|
| GCP VM (Compute Engine) | No | Any agent can run here |
| Spot instance + persistent disk | No | Cost optimization is universal |
| Service account + IAM | No | Least-privilege applies to any agent |
| Docker installation | No | Any agent may need to build containers |
| Development tooling | No | Go, Python, Node, etc. are universal |
| tmux session management | No | Any CLI agent can run in tmux |
| TUI for VM management | No | Start/stop/resize is agent-agnostic |
| Agent invocation command | **Yes** | `claude --dangerously-skip-permissions` |
| Session persistence | **Yes** | `--continue`, `--resume` flags |
| Session naming | **Yes** | `/rename` command syntax |

Only ~5% of the system is agent-specific. The infrastructure should be designed to support multiple AI coding agents.

## Decision

Design the sandbox as an **agent-agnostic platform** with pluggable agent configurations.

Agent-specific behavior is isolated to a configuration file that defines:
- Agent command and flags
- Session resume mechanism
- Environment variables required

## Options Considered

### Option 1: Claude Code Only

Hardcode Claude Code throughout the implementation.

**Pros:**
- Simpler initial implementation
- No abstraction overhead
- Optimized for one tool

**Cons:**
- Cannot use with other agents (Gemini CLI, Copilot, Codex, etc.)
- Must fork/rewrite to support alternatives
- Vendor lock-in to Anthropic tooling

### Option 2: Agent-Agnostic with Configuration (Chosen)

Abstract agent-specific details into a configuration layer.

**Pros:**
- Support multiple agents with minimal changes
- Easy to add new agents as they emerge
- Community can contribute agent configs
- Future-proof as AI coding tools evolve rapidly
- Can run different agents in different tmux windows simultaneously

**Cons:**
- Slightly more complex initial design
- Must define abstraction boundaries carefully
- Some agent-specific features may not map cleanly

### Option 3: Plugin Architecture

Full plugin system with runtime-loadable agent modules.

**Pros:**
- Maximum flexibility
- Agents can define custom UI elements
- Rich extensibility

**Cons:**
- Over-engineered for the problem
- Significant implementation complexity
- Plugin API maintenance burden

## Design

### Agent Configuration File

```yaml
# ~/.config/sandbox-manager/agents.yaml

agents:
  claude:
    name: "Claude Code"
    command: "claude"
    autonomous_flags: ["--dangerously-skip-permissions"]
    resume_flag: "--continue"
    resume_session_flag: "--resume"
    session_rename: "/rename {name}"  # In-session command
    env: {}

  gemini:
    name: "Gemini CLI"
    command: "gemini"
    autonomous_flags: ["--sandbox"]  # Hypothetical
    resume_flag: "--resume"
    resume_session_flag: "--session"
    session_rename: null  # May not support
    env:
      GOOGLE_AI_API_KEY: "${GOOGLE_AI_API_KEY}"

  copilot:
    name: "GitHub Copilot CLI"
    command: "gh copilot"
    autonomous_flags: []
    resume_flag: null  # May not support session persistence
    resume_session_flag: null
    session_rename: null
    env:
      GITHUB_TOKEN: "${GITHUB_TOKEN}"

  aider:
    name: "Aider"
    command: "aider"
    autonomous_flags: ["--yes-always", "--no-suggest-shell-commands"]
    resume_flag: null  # Stateless
    resume_session_flag: null
    session_rename: null
    env:
      ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
      # Or: OPENAI_API_KEY, etc.

  codex:
    name: "OpenAI Codex CLI"
    command: "codex"
    autonomous_flags: ["--auto-approve"]  # Hypothetical
    resume_flag: null
    resume_session_flag: null
    session_rename: null
    env:
      OPENAI_API_KEY: "${OPENAI_API_KEY}"

default_agent: claude
```

### TUI Agent Selection

```
┌─────────────────────────────────────────────────────────────────┐
│  Sandbox Manager                                     v0.1.0     │
├─────────────────────────────────────────────────────────────────┤
│  Agent: Claude Code  [↑↓ to change]                            │
│                                                                 │
│  Agent Sessions (3 active)                                      │
│  [1] claude   ● issue-142  go-gin service      2h 30m          │
│  [2] claude   ● issue-143  python-flask        1h 15m          │
│  [3] aider    ● issue-144  quick fix           0h 20m          │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  [A]dd Agent: Claude Code ▼                                     │
└─────────────────────────────────────────────────────────────────┘
```

### Start Agent Script

The `start-agents-resilient.sh` script becomes agent-aware:

```bash
#!/bin/bash
# start-agent.sh

AGENT=${1:-claude}
MODE=${2:-fresh}  # fresh, continue, resume
WORKSPACE=${3:-/workspaces/default}

# Load agent config
CONFIG=$(yq e ".agents.$AGENT" ~/.config/sandbox-manager/agents.yaml)
COMMAND=$(echo "$CONFIG" | yq e '.command')
AUTO_FLAGS=$(echo "$CONFIG" | yq e '.autonomous_flags | join(" ")')

case $MODE in
  fresh)
    RESUME_FLAGS=""
    ;;
  continue)
    RESUME_FLAG=$(echo "$CONFIG" | yq e '.resume_flag // ""')
    RESUME_FLAGS="$RESUME_FLAG"
    ;;
  resume)
    RESUME_FLAG=$(echo "$CONFIG" | yq e '.resume_session_flag // ""')
    RESUME_FLAGS="$RESUME_FLAG $SESSION_NAME"
    ;;
esac

# Launch in tmux
cd "$WORKSPACE"
tmux new-window -t agents -n "$AGENT-$WINDOW_NUM" \
  "$COMMAND $AUTO_FLAGS $RESUME_FLAGS"
```

### Capability Matrix

Document which features each agent supports:

| Feature | Claude | Gemini | Copilot | Aider | Codex |
|---------|--------|--------|---------|-------|-------|
| Autonomous mode | ✓ | ? | ✗ | ✓ | ? |
| Session persistence | ✓ | ? | ✗ | ✗ | ? |
| Session resume | ✓ | ? | ✗ | ✗ | ? |
| In-session rename | ✓ | ? | ✗ | ✗ | ? |
| Docker support | ✓ | ✓ | ✓ | ✓ | ✓ |
| Git integration | ✓ | ✓ | ✓ | ✓ | ✓ |

(? = not yet documented/tested)

### What Remains Generic

| Component | Implementation |
|-----------|---------------|
| VM provisioning | gcloud commands, unchanged |
| VM lifecycle | start/stop/resize, unchanged |
| Service account | IAM permissions, unchanged |
| Tooling installation | provision-vm.sh, unchanged |
| tmux management | Session/window commands, unchanged |
| TUI framework | Agent selector added, core unchanged |
| Cost tracking | Per-VM, not per-agent |

### What Becomes Pluggable

| Component | Abstraction |
|-----------|-------------|
| Agent invocation | `agents.yaml` command + flags |
| Session resume | `agents.yaml` resume_flag |
| API keys | `agents.yaml` env mapping |
| Agent display name | `agents.yaml` name field |

## Consequences

### Positive

- Can use Claude, Gemini, Aider, or future agents with same infrastructure
- Easy to experiment with different agents on same codebase
- Community can contribute agent configurations
- Not locked into single vendor
- Can run mixed agent sessions (Claude for complex, Aider for quick fixes)

### Negative

- Abstraction may not capture all agent-specific features
- Must update configs as agents evolve
- Some agents may lack features (no session persistence = manual resume)
- Testing matrix grows with each agent

### Neutral

- Default to Claude Code (current primary use case)
- Agent configs are optional - works with hardcoded Claude if no config exists
- Future agents can be added without code changes
- May need agent-specific documentation sections
