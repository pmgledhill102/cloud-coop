# ADR-0009: API Key Management

## Status

Accepted

## Context

AI coding agents require API keys or authentication tokens to communicate with their backend services:

| Agent | Authentication Method |
|-------|----------------------|
| Claude Code | Anthropic API key or OAuth (browser) |
| Aider | ANTHROPIC_API_KEY, OPENAI_API_KEY, etc. |
| Gemini CLI | Google Cloud auth or API key |
| GitHub Copilot | GitHub OAuth |
| OpenAI Codex | OPENAI_API_KEY |

These keys are sensitive:
- They grant access to paid API services
- They may have usage limits/quotas
- Compromise could lead to unexpected bills or abuse
- Some keys grant access beyond just the AI API

The sandbox VM runs with `--dangerously-skip-permissions`, meaning agents can read any file on the VM. We need to balance security with usability.

## Decision

Use a **tiered approach** based on agent capabilities:

1. **OAuth browser flow** (preferred) - for agents that support it
2. **SSH agent forwarding** (fallback) - keys stay on local machine
3. **GCP Secret Manager** (alternative) - for headless/automated scenarios

Never store raw API keys directly on the VM filesystem.

## Options Considered

### Option 1: Keys Stored on VM

Store API keys in files on the VM (e.g., `~/.anthropic/api_key`, environment in `.bashrc`).

**Pros:**
- Simple setup
- Works immediately after VM start
- No external dependencies

**Cons:**
- **Keys exposed to all processes on VM** - agents with guardrails off can read them
- Keys persist on disk - survive VM stop/start
- If VM is compromised, keys are compromised
- Must manually rotate keys on VM
- Keys in bash history if set via command line

### Option 2: SSH Environment Forwarding

Keys stored on local machine, passed to VM via SSH `SendEnv`/`AcceptEnv`.

```bash
# Local: ~/.ssh/config
Host claude-sandbox
  SendEnv ANTHROPIC_API_KEY
  SendEnv OPENAI_API_KEY

# VM: /etc/ssh/sshd_config
AcceptEnv ANTHROPIC_API_KEY OPENAI_API_KEY
```

**Pros:**
- Keys never written to VM disk
- Keys only present during active SSH session
- Revoke locally = revoked everywhere
- Works with any agent using environment variables

**Cons:**
- Keys in memory on VM during session (still readable by processes)
- Must have keys on local machine
- SSH session must remain active
- Doesn't work for background/unattended agents

### Option 3: OAuth Browser Flow (Preferred for supported agents)

Agents authenticate via browser, tokens stored in agent-specific secure storage.

```bash
# Claude Code - first run opens browser
claude auth login

# Stores token in ~/.claude/ with appropriate permissions
# Token is scoped and revocable from Anthropic dashboard
```

**Pros:**
- **No raw API keys anywhere** - uses delegated tokens
- Tokens scoped to specific permissions
- Revocable from provider dashboard without changing keys
- Natural UX - same as logging into a website
- Provider handles token refresh

**Cons:**
- Requires browser access from VM (can use SSH tunnel)
- Not all agents support it
- Token still stored on VM (but more limited than raw API key)
- Initial setup requires interactive session

### Option 4: GCP Secret Manager

Store keys in GCP Secret Manager, fetch at runtime.

```bash
# Store secret (one-time, from local machine)
echo -n "sk-ant-..." | gcloud secrets create anthropic-api-key --data-file=-

# Grant VM service account access
gcloud secrets add-iam-policy-binding anthropic-api-key \
  --member="serviceAccount:claude-sandbox@project.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

# On VM: fetch at session start
export ANTHROPIC_API_KEY=$(gcloud secrets versions access latest --secret=anthropic-api-key)
```

**Pros:**
- Keys never on local machine or VM disk
- Centralized management and rotation
- Audit log of all access
- Can revoke VM access without changing key
- Works for headless/automated scenarios

**Cons:**
- Additional GCP cost (~$0.03/10,000 access operations)
- VM needs secretmanager IAM permissions
- Key in memory once fetched
- More complex setup
- Requires GCP project

### Option 5: Workload Identity (GCP-native services only)

For Gemini CLI or other Google services, use the VM's service account directly.

```bash
# No API key needed - uses instance metadata
gcloud auth application-default login --no-browser
# Or: automatic with service account attached to VM
```

**Pros:**
- No keys at all
- Uses existing IAM permissions
- Automatic token refresh
- Fully audited

**Cons:**
- Only works for Google Cloud services
- Not applicable to Anthropic, OpenAI, GitHub, etc.

## Recommended Approach

### Tier 1: OAuth Browser Flow (when available)

```bash
# First time setup - opens browser via SSH tunnel
ssh -L 8080:localhost:8080 claude-sandbox
claude auth login  # Opens localhost:8080, redirects to Anthropic

# Subsequent sessions - already authenticated
claude --dangerously-skip-permissions
```

**Supported by:**
- Claude Code ✓
- GitHub Copilot ✓
- Gemini CLI ✓ (via gcloud auth)

### Tier 2: Secret Manager (for env-var-based agents)

```bash
# Setup (once)
gcloud secrets create anthropic-api-key --data-file=- <<< "$ANTHROPIC_API_KEY"

# On VM startup script or .bashrc
export ANTHROPIC_API_KEY=$(gcloud secrets versions access latest --secret=anthropic-api-key 2>/dev/null)
```

**Use for:**
- Aider
- Raw API scripts
- Headless automation

### Tier 3: SSH Forwarding (temporary/testing)

```bash
# Quick testing without Secret Manager setup
ssh -o SendEnv=ANTHROPIC_API_KEY claude-sandbox
```

**Use for:**
- Initial testing
- Temporary sessions
- When Secret Manager not yet configured

## Implementation

### TUI Integration

The TUI should handle authentication status:

```
┌─────────────────────────────────────────────────────────────────┐
│  Authentication Status                                          │
│  ────────────────────                                           │
│  Claude Code:    ✓ Authenticated (OAuth, expires in 29 days)   │
│  Aider:          ✓ Secret Manager (anthropic-api-key)          │
│  Gemini CLI:     ✓ Service Account (workload identity)         │
│  GitHub Copilot: ✗ Not authenticated [A]uthenticate            │
└─────────────────────────────────────────────────────────────────┘
```

### Agent Configuration Extension

```yaml
# ~/.config/sandbox-manager/agents.yaml
agents:
  claude:
    auth:
      method: oauth
      command: "claude auth login"
      check: "claude auth status"

  aider:
    auth:
      method: secret_manager
      secrets:
        ANTHROPIC_API_KEY: "anthropic-api-key"
        # Or for OpenAI backend:
        # OPENAI_API_KEY: "openai-api-key"

  gemini:
    auth:
      method: workload_identity
      # Uses VM service account automatically

  copilot:
    auth:
      method: oauth
      command: "gh auth login"
      check: "gh auth status"
```

### SSH Tunnel Helper

For OAuth flows that need browser access:

```bash
#!/bin/bash
# scripts/auth-tunnel.sh

echo "Opening SSH tunnel for authentication..."
echo "A browser window will open. Complete authentication there."

# Forward localhost:8080 for OAuth callbacks
ssh -L 8080:localhost:8080 -L 8000:localhost:8000 claude-sandbox \
  -t "claude auth login"
```

## Security Considerations

### What's Protected

| Threat | OAuth | Secret Manager | SSH Forward | VM Storage |
|--------|-------|----------------|-------------|------------|
| Key on VM disk | ✗ (token only) | ✗ | ✗ | ✓ EXPOSED |
| Key in VM memory | Token only | ✓ Present | ✓ Present | ✓ Present |
| Survives VM stop | Token (encrypted) | ✗ | ✗ | ✓ PERSISTS |
| Remote revocation | ✓ Yes | ✓ Yes | ✗ No | ✗ No |
| Audit trail | ✓ Provider logs | ✓ GCP logs | ✗ No | ✗ No |

### Residual Risk

Even with best practices, keys/tokens are in memory during active sessions. An agent running with `--dangerously-skip-permissions` could theoretically:
- Read environment variables
- Access agent config files
- Exfiltrate tokens

**Mitigations:**
- Use scoped tokens with minimal permissions
- Monitor for unusual API usage patterns
- Rotate tokens regularly
- Use separate API keys for sandbox vs production

## Consequences

### Positive

- Raw API keys never stored on VM filesystem
- OAuth tokens are scoped and revocable
- Centralized secret management with audit trail
- Clear hierarchy: OAuth → Secret Manager → SSH forward
- TUI shows authentication status

### Negative

- OAuth requires interactive browser session for initial setup
- Secret Manager adds GCP cost and complexity
- Must set up authentication before agents can work
- Some agents may not support preferred auth methods

### Neutral

- Initial setup is more involved than just pasting a key
- Different agents use different auth methods
- Need to document auth setup per agent
