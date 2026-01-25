# ADR-0013: SSH and Remote Execution

## Status

Accepted

## Context

The TUI runs on the user's workstation but needs to manage tmux sessions on a remote VM. This requires:
- Executing commands on the VM (list tmux windows, create sessions, etc.)
- Connecting the user's terminal to a specific tmux session
- Maintaining responsiveness in the TUI

We need to decide how the TUI communicates with the remote VM.

## Decision

Use the **Go SSH library** (`golang.org/x/crypto/ssh`) for programmatic commands, and **shell out to `ssh`** for interactive terminal sessions.

## Options Considered

### Option 1: Shell Out to SSH for Everything

```go
func (r *Remote) ListAgents() ([]Agent, error) {
    cmd := exec.Command("ssh", r.host, "tmux", "list-windows", "-t", "agents")
    output, err := cmd.Output()
    // parse output...
}
```

**Pros:**
- Simple implementation
- Uses user's existing SSH config (~/.ssh/config)
- Handles SSH agent forwarding automatically

**Cons:**
- Requires `ssh` binary on user's system
- Process spawn overhead for each command
- Parsing text output is fragile
- Error handling is harder

### Option 2: Go SSH Library for Everything

```go
import "golang.org/x/crypto/ssh"

func (r *Remote) ListAgents() ([]Agent, error) {
    session, err := r.client.NewSession()
    if err != nil {
        return nil, err
    }
    defer session.Close()

    output, err := session.CombinedOutput("tmux list-windows -t agents -F '#{window_index}:#{window_name}'")
    // parse output...
}
```

**Pros:**
- No external dependencies
- Type-safe connection handling
- Better error handling
- Single binary distribution maintained

**Cons:**
- Must handle SSH key loading explicitly
- SSH agent forwarding requires extra work
- Interactive terminal attachment is complex

### Option 3: Hybrid Approach (Chosen)

Use Go SSH library for programmatic commands, shell out to `ssh` for interactive sessions.

**Programmatic (Go SSH):**
- List tmux sessions
- Create new agent windows
- Kill agent windows
- Check agent status

**Interactive (shell out to ssh):**
- `[C]onnect` - attach to agent's tmux window

**Pros:**
- Best of both worlds
- Programmatic commands are fast and reliable
- Interactive sessions use familiar SSH behavior
- Single binary for TUI, ssh binary only for connect

**Cons:**
- Two different code paths
- Still requires `ssh` for connect feature

## Implementation

### SSH Client Setup

```go
// internal/ssh/client.go
package ssh

import (
    "os"
    "path/filepath"

    "golang.org/x/crypto/ssh"
    "golang.org/x/crypto/ssh/agent"
    "golang.org/x/crypto/ssh/knownhosts"
)

type Client struct {
    conn *ssh.Client
    host string
    user string
}

func NewClient(host, user string) (*Client, error) {
    // Try SSH agent first
    authMethods := []ssh.AuthMethod{}

    if agentConn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
        authMethods = append(authMethods, ssh.PublicKeysCallback(
            agent.NewClient(agentConn).Signers,
        ))
    }

    // Fall back to key file
    keyPath := filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519")
    if key, err := os.ReadFile(keyPath); err == nil {
        if signer, err := ssh.ParsePrivateKey(key); err == nil {
            authMethods = append(authMethods, ssh.PublicKeys(signer))
        }
    }

    // Known hosts verification
    hostKeyCallback, err := knownhosts.New(
        filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"),
    )
    if err != nil {
        hostKeyCallback = ssh.InsecureIgnoreHostKey() // For initial setup
    }

    config := &ssh.ClientConfig{
        User:            user,
        Auth:            authMethods,
        HostKeyCallback: hostKeyCallback,
    }

    conn, err := ssh.Dial("tcp", host+":22", config)
    if err != nil {
        return nil, fmt.Errorf("ssh dial: %w", err)
    }

    return &Client{conn: conn, host: host, user: user}, nil
}
```

### Remote Commands

```go
// internal/ssh/commands.go

func (c *Client) Run(cmd string) (string, error) {
    session, err := c.conn.NewSession()
    if err != nil {
        return "", err
    }
    defer session.Close()

    output, err := session.CombinedOutput(cmd)
    return string(output), err
}

func (c *Client) ListAgentSessions() ([]AgentSession, error) {
    output, err := c.Run(`tmux list-windows -t agents -F '#{window_index}|#{window_name}|#{pane_current_command}' 2>/dev/null || echo ''`)
    if err != nil {
        return nil, err
    }

    var sessions []AgentSession
    for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
        if line == "" {
            continue
        }
        parts := strings.Split(line, "|")
        if len(parts) >= 3 {
            sessions = append(sessions, AgentSession{
                Index:   parts[0],
                Name:    parts[1],
                Command: parts[2],
            })
        }
    }
    return sessions, nil
}

func (c *Client) CreateAgentSession(name string, agentCmd string) error {
    cmd := fmt.Sprintf(`tmux new-window -t agents -n %s '%s'`, name, agentCmd)
    _, err := c.Run(cmd)
    return err
}

func (c *Client) KillAgentSession(index string) error {
    _, err := c.Run(fmt.Sprintf(`tmux kill-window -t agents:%s`, index))
    return err
}
```

### Interactive Connect (Shell Out)

```go
// internal/ssh/connect.go

func (c *Client) ConnectInteractive(windowIndex string) error {
    // Shell out to ssh for interactive session
    // This gives proper terminal handling, colors, resize, etc.

    cmd := exec.Command("ssh", "-t",
        fmt.Sprintf("%s@%s", c.user, c.host),
        fmt.Sprintf("tmux select-window -t agents:%s && tmux attach -t agents", windowIndex),
    )

    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    return cmd.Run()
}
```

### SSH Key Setup

The TUI should guide users to set up SSH keys if not present:

```go
func (c *Client) EnsureSSHKey() error {
    keyPath := filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519")
    if _, err := os.Stat(keyPath); os.IsNotExist(err) {
        return fmt.Errorf("no SSH key found at %s - run: ssh-keygen -t ed25519", keyPath)
    }
    return nil
}
```

## Authentication Flow

1. TUI starts → checks for SSH key
2. Connects to VM using Go SSH library
3. If connection fails, prompts user to add key to VM:
   ```
   gcloud compute ssh claude-sandbox --zone=europe-north2-a
   ```
4. Once connected, maintains persistent connection for commands
5. For interactive connect, spawns `ssh` process

## Consequences

### Positive

- Fast programmatic commands via persistent SSH connection
- Interactive sessions work naturally with proper terminal handling
- SSH agent integration for key management
- Single binary distribution (ssh only needed for connect)

### Negative

- Interactive connect requires `ssh` binary
- Must handle SSH key setup in onboarding
- Two code paths to maintain

### Neutral

- Go SSH library is well-maintained and widely used
- SSH is universally available on macOS/Linux
- Known hosts handling adds security
