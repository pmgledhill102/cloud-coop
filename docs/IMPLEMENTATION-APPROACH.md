# Implementation Approach

This document describes the tracer bullet approach for implementing cloudcoop.

## Philosophy

A **tracer bullet** approach builds thin vertical slices of functionality end-to-end, proving out
the architecture and integration points before filling in details. Each iteration delivers working
software that can be demonstrated and tested.

Benefits:

- Early validation of architectural decisions
- Continuous integration of all layers (CLI, TUI, GCP SDK, SSH)
- Reduced risk of late-stage integration failures
- Demonstrable progress at each stage
- Flexibility to adjust based on learnings

### Quality Gates

Between each iteration, we pause to:

1. **Test** - Manual verification that everything works as expected
2. **Refactor** - Clean up code before adding more complexity
3. **Document** - Update docs and comments while context is fresh
4. **Review** - Consider architectural improvements based on learnings

This prevents technical debt accumulation and ensures each layer is solid before building on top of it.

## Architecture Layers

Each tracer bullet passes through these layers:

```text
┌─────────────────────────────────────────┐
│  User Interface (CLI / TUI)             │  ← Cobra + Bubbletea
├─────────────────────────────────────────┤
│  Application Logic                      │  ← Core business logic
├─────────────────────────────────────────┤
│  Cloud Provider (GCP)                   │  ← Compute SDK
├─────────────────────────────────────────┤
│  Remote Execution (SSH)                 │  ← Go SSH library
├─────────────────────────────────────────┤
│  Agent Management (tmux)                │  ← tmux commands
└─────────────────────────────────────────┘
```

---

## Iteration 1: CLI/TUI Skeleton

**Goal:** Prove the hybrid CLI/TUI pattern works with build pipeline.

### Deliverables

- `cloudcoop` (no args) launches minimal Bubbletea TUI displaying static placeholder
- `cloudcoop version` prints version string
- `cloudcoop status` prints mock/hardcoded status
- Binary builds for darwin/linux on arm64/amd64
- CI pipeline runs tests and builds

### Proves

- Cobra + Bubbletea integration pattern
- Cross-platform build configuration
- CI/CD pipeline functionality

### Exit Criteria

User can run `cloudcoop` and see TUI, run `cloudcoop status` and see mock output.

---

## Gate 1: Foundation Review

### Manual Testing Checklist

- [ ] `cloudcoop` launches TUI, displays placeholder, exits cleanly with `q`
- [ ] `cloudcoop version` prints version and exits
- [ ] `cloudcoop status` prints mock status and exits
- [ ] `cloudcoop --help` shows available commands
- [ ] `cloudcoop status --help` shows command-specific help
- [ ] Binary runs on macOS ARM64 (if available)
- [ ] Binary runs on macOS AMD64 (if available)
- [ ] Binary runs on Linux ARM64 (if available)
- [ ] Binary runs on Linux AMD64 (if available)
- [ ] CI pipeline passes on push
- [ ] CI pipeline passes on PR

### Refactoring Considerations

- **Package structure:** Ensure clean separation established early:
  - `cmd/` - Cobra command definitions
  - `internal/tui/` - Bubbletea models and views
  - `internal/config/` - Configuration handling (stub)
  - `internal/cloud/` - Cloud provider interface (stub)
- **Error handling pattern:** Establish consistent error wrapping approach
- **Logging:** Decide on logging library (zerolog, slog) and integrate
- **Exit codes:** Define exit code constants (0=success, 1=error, 2=usage)

### Code Quality Checks

- [ ] `go vet ./...` passes
- [ ] `golangci-lint run` passes
- [ ] No TODO comments without associated issue/bead
- [ ] All exported functions have doc comments
- [ ] Test coverage >60% for new code

### Documentation Updates

- [ ] README updated with build/run instructions
- [ ] CLAUDE.md created with project context
- [ ] `--help` text is clear and complete

---

## Iteration 2: GCP Authentication & Read

**Goal:** Prove GCP SDK integration and credential flow.

### Deliverables

- Authenticate using Application Default Credentials (ADC)
- `cloudcoop status` queries real VM instance from Compute API
- TUI displays real VM status (name, state, zone, machine type)
- Graceful handling when VM not found (with setup hint)
- Configuration for project/zone/instance name

### Proves

- GCP Compute SDK integration
- ADC credential discovery
- Error handling patterns

### Exit Criteria

User with valid GCP credentials sees real VM status.

---

## Gate 2: Cloud Integration Review

### Manual Testing Checklist

- [ ] With valid ADC: `cloudcoop status` shows real VM state
- [ ] With valid ADC: TUI shows real VM state, updates on refresh
- [ ] With no credentials: Clear error message with fix instructions
- [ ] With wrong project: Clear error message identifying the issue
- [ ] With non-existent VM: Helpful message suggesting setup
- [ ] Status shows: instance name, zone, state, machine type
- [ ] `cloudcoop status --json` outputs valid JSON (if implemented)
- [ ] TUI handles API latency gracefully (loading indicator)

### Refactoring Considerations

- **Cloud provider interface:** Abstract GCP behind interface for future AWS/Azure:

  ```go
  type CloudProvider interface {
      GetInstance(ctx context.Context) (*Instance, error)
      StartInstance(ctx context.Context) error
      StopInstance(ctx context.Context) error
  }
  ```

- **Context propagation:** Ensure context flows through for cancellation
- **Credential caching:** Consider caching authenticated client
- **Rate limiting:** Add basic rate limiting for API calls
- **Retry logic:** Implement exponential backoff for transient failures

### Code Quality Checks

- [ ] No hardcoded project IDs, zones, or instance names
- [ ] Secrets/credentials never logged
- [ ] API errors wrapped with context
- [ ] Unit tests use mock GCP client
- [ ] Integration test documented (manual steps)

### Documentation Updates

- [ ] Prerequisites documented (gcloud CLI, ADC setup)
- [ ] Configuration options documented
- [ ] Error messages include links to relevant docs
- [ ] Troubleshooting section started

---

## Iteration 3: VM Lifecycle Control

**Goal:** Prove write operations and async operation handling.

### Deliverables

- `cloudcoop start` starts a stopped VM, waits for RUNNING state
- `cloudcoop stop` stops a running VM, waits for STOPPED state
- TUI keybindings: `S` to start, `T` to stop
- Progress indicator during long-running operations
- Idempotent behaviour (start when running = no-op with message)

### Proves

- GCP mutation operations
- Long-running operation polling
- TUI state updates during async work

### Exit Criteria

User can start/stop VM from both CLI and TUI.

---

## Gate 3: Lifecycle Control Review

### Manual Testing Checklist

- [ ] `cloudcoop start` on stopped VM: starts and waits for RUNNING
- [ ] `cloudcoop start` on running VM: no-op with "already running" message
- [ ] `cloudcoop stop` on running VM: stops and waits for STOPPED
- [ ] `cloudcoop stop` on stopped VM: no-op with "already stopped" message
- [ ] TUI `S` key starts VM with progress indicator
- [ ] TUI `T` key stops VM with progress indicator
- [ ] Ctrl+C during start/stop: cancels gracefully, shows current state
- [ ] TUI updates status after operation completes
- [ ] Operation timeout handled gracefully (if VM takes too long)
- [ ] Concurrent start requests handled safely

### Refactoring Considerations

- **Operation abstraction:** Consider operation pattern for all async work:

  ```go
  type Operation interface {
      Wait(ctx context.Context) error
      Status() OperationStatus
  }
  ```

- **TUI state machine:** Formalize TUI states (idle, loading, error)
- **Command confirmation:** Consider `--yes` flag pattern for destructive ops
- **Async in TUI:** Ensure Bubbletea Cmd pattern used correctly for async

### Code Quality Checks

- [ ] Operations are idempotent
- [ ] Timeouts configurable
- [ ] Progress feedback provided for operations >2s
- [ ] No goroutine leaks (check with `go test -race`)
- [ ] Context cancellation respected throughout

### Documentation Updates

- [ ] CLI commands documented with examples
- [ ] TUI keybindings documented
- [ ] Common workflows documented (daily start/stop)
- [ ] Cost implications noted (running vs stopped)

---

## Iteration 4: SSH Connectivity

**Goal:** Prove remote command execution path.

### Deliverables

- Establish SSH connection to VM using Go SSH library
- `cloudcoop ssh <command>` runs arbitrary command, prints output
- SSH key discovery (standard locations, SSH agent)
- TUI shows connection status indicator
- Graceful handling of connection failures

### Proves

- SSH authentication and key handling
- Network connectivity to VM
- Remote command execution

### Exit Criteria

User can run `cloudcoop ssh hostname` and see VM hostname.

---

## Gate 4: SSH Integration Review

### Manual Testing Checklist

- [ ] `cloudcoop ssh hostname` returns VM hostname
- [ ] `cloudcoop ssh "echo hello"` returns "hello"
- [ ] `cloudcoop ssh "cat /etc/os-release"` returns OS info
- [ ] SSH with key in `~/.ssh/id_rsa` works
- [ ] SSH with key in `~/.ssh/id_ed25519` works
- [ ] SSH via SSH agent works
- [ ] SSH with custom key path works (if configurable)
- [ ] Connection to stopped VM: clear error message
- [ ] Connection timeout: clear error after reasonable wait
- [ ] Invalid key: clear error message
- [ ] Network unreachable: clear error message
- [ ] TUI shows "Connected" indicator when SSH verified
- [ ] TUI shows "Disconnected" when SSH fails

### Refactoring Considerations

- **SSH client abstraction:** Interface for testing:

  ```go
  type SSHClient interface {
      Run(ctx context.Context, cmd string) (string, error)
      Close() error
  }
  ```

- **Connection pooling:** Consider reusing SSH connections
- **Known hosts:** Decide on host key verification strategy
- **Username handling:** Make SSH username configurable
- **Timeout configuration:** Connection and command timeouts

### Code Quality Checks

- [ ] SSH keys never logged or exposed
- [ ] Connection errors don't leak internal details
- [ ] SSH client properly closed on all paths
- [ ] Unit tests use mock SSH client
- [ ] No shell injection vulnerabilities in command execution

### Documentation Updates

- [ ] SSH key setup documented
- [ ] Firewall requirements documented
- [ ] Troubleshooting SSH issues section
- [ ] IAP tunnel option documented (if supported)

---

## Iteration 5: Agent Sessions (Read)

**Goal:** Prove tmux integration and output parsing.

### Deliverables

- `cloudcoop agents list` executes `tmux list-windows`, parses output
- Display: window index, name, current command, duration
- TUI displays agent list in main view
- Handle "no tmux session" case gracefully
- Handle "tmux not installed" case

### Proves

- Remote command output parsing
- tmux protocol understanding
- Session state representation

### Exit Criteria

User sees list of tmux windows (or empty state message).

---

## Gate 5: Agent Read Review

### Manual Testing Checklist

- [ ] `cloudcoop agents list` with active sessions shows list
- [ ] `cloudcoop agents list` with no sessions shows empty message
- [ ] `cloudcoop agents list` with tmux not installed shows helpful error
- [ ] List shows: index, window name, current process
- [ ] `cloudcoop agents list --json` outputs valid JSON (if implemented)
- [ ] TUI displays agent list in scrollable view
- [ ] TUI handles 0 agents gracefully
- [ ] TUI handles 20+ agents (scrolling works)
- [ ] TUI refreshes agent list periodically or on keypress
- [ ] Agent status indicators (active/idle) are accurate

### Refactoring Considerations

- **Agent model:** Define clear struct for agent representation:

  ```go
  type Agent struct {
      Index     int
      Name      string
      Command   string
      Status    AgentStatus  // Active, Idle, Unknown
      StartTime time.Time
  }
  ```

- **tmux parser:** Dedicated parser for tmux output formats
- **Polling vs events:** Consider approach for real-time updates
- **Session naming:** Establish convention (e.g., "agents" session name)

### Code Quality Checks

- [ ] tmux output parsing handles edge cases
- [ ] Malformed tmux output doesn't crash
- [ ] Parser has unit tests with real tmux output samples
- [ ] No assumptions about tmux version

### Documentation Updates

- [ ] tmux session naming convention documented
- [ ] Expected VM setup documented (tmux installed, session created)
- [ ] Agent list fields explained

---

## Iteration 6: Agent Sessions (Write)

**Goal:** Prove full agent lifecycle control.

### Deliverables

- `cloudcoop agents add [--name=NAME] [--command=CMD]` creates tmux window
- `cloudcoop agents kill <index>` kills tmux window by index
- TUI keybindings: `A` to add, `K` to kill (with confirmation)
- Default command configurable (e.g., `claude --dangerously-skip-permissions`)
- Prevent killing window with active process (warning + force flag)

### Proves

- Bidirectional tmux control
- Agent creation with custom commands
- Safe deletion with confirmation

### Exit Criteria

User can create and destroy agent sessions from CLI and TUI.

---

## Gate 6: Agent Write Review

### Manual Testing Checklist

- [ ] `cloudcoop agents add` creates new tmux window with default command
- [ ] `cloudcoop agents add --name=test-agent` creates named window
- [ ] `cloudcoop agents add --command="aider"` creates window with custom command
- [ ] `cloudcoop agents kill 1` kills window at index 1
- [ ] `cloudcoop agents kill 1` with active process shows warning
- [ ] `cloudcoop agents kill 1 --force` kills despite active process
- [ ] TUI `A` key prompts for agent options (or uses defaults)
- [ ] TUI `K` key shows confirmation before killing
- [ ] TUI updates list after add/kill operations
- [ ] Adding agent when VM stopped shows clear error
- [ ] Killing non-existent index shows clear error
- [ ] Adding 12+ agents works correctly

### Refactoring Considerations

- **Command builder:** Centralize agent command construction
- **Confirmation pattern:** Reusable confirmation dialog for TUI
- **Agent templates:** Consider preset agent configurations
- **Bulk operations:** Prepare for future bulk add/kill

### Code Quality Checks

- [ ] Shell escaping correct for agent commands
- [ ] No command injection vulnerabilities
- [ ] Agent name sanitization (no special chars)
- [ ] Concurrent add/kill operations handled safely

### Documentation Updates

- [ ] Agent commands documented with examples
- [ ] Default agent command explained
- [ ] Custom agent configurations documented
- [ ] Safety warnings for kill operations

---

## Iteration 7: Interactive Connect

**Goal:** Prove interactive terminal handoff.

### Deliverables

- `cloudcoop connect <index>` shells out to SSH, attaches to tmux window
- TUI keybinding: `C` to connect to selected agent
- Clean terminal state on exit (restore after SSH disconnect)
- Option to re-launch TUI after disconnect or exit completely

### Proves

- PTY handling for interactive sessions
- Terminal state management
- TUI suspend/resume or exit flow

### Exit Criteria

User can connect to agent, interact, detach, and return.

---

## Gate 7: Interactive Connect Review

### Manual Testing Checklist

- [ ] `cloudcoop connect 1` attaches to tmux window 1
- [ ] Can interact with agent (type commands, see output)
- [ ] Detach with `Ctrl+B D` returns to shell
- [ ] Exit agent with `exit` returns to shell
- [ ] Terminal state restored (no garbled output)
- [ ] TUI `C` key connects to selected agent
- [ ] After disconnect, option to return to TUI or exit
- [ ] Resize terminal during session works correctly
- [ ] Special characters render correctly
- [ ] Colors/formatting preserved
- [ ] Connecting to non-existent index shows clear error
- [ ] Connecting when VM stopped shows clear error

### Refactoring Considerations

- **Terminal state:** Save/restore terminal state properly
- **Signal handling:** Handle SIGWINCH for resize, SIGINT gracefully
- **Alternative connection:** Consider native Go SSH PTY vs shelling out
- **Connection options:** tmux attach flags (-d for detach others, etc.)

### Code Quality Checks

- [ ] No terminal state leaks
- [ ] Signals handled correctly
- [ ] PTY allocation works on all platforms
- [ ] Child process cleanup on all exit paths

### Documentation Updates

- [ ] Connection workflow documented
- [ ] tmux detach/navigation keys documented
- [ ] Troubleshooting terminal issues section

---

## Iteration 8: Configuration & Polish

**Goal:** Production-ready user experience.

### Deliverables

- Config file support: `~/.config/cloudcoop/cloudcoop.toml`
- `cloudcoop config show` displays current configuration
- `cloudcoop config set <key> <value>` updates configuration
- First-run detection with setup wizard prompt
- Comprehensive error messages with actionable hints
- `--help` text polished for all commands

### Proves

- Real-world usability
- Configuration persistence
- User onboarding flow

### Exit Criteria

New user can install, configure, and use cloudcoop without reading docs.

---

## Gate 8: Polish Review

### Manual Testing Checklist

- [ ] First run with no config triggers setup wizard
- [ ] Setup wizard collects: project, zone, instance name
- [ ] `cloudcoop config show` displays all settings
- [ ] `cloudcoop config set cloud.project my-project` works
- [ ] Config file created at correct location
- [ ] Config file permissions are secure (not world-readable)
- [ ] Invalid config values rejected with clear message
- [ ] All commands work with config file (no CLI flags needed)
- [ ] CLI flags override config file values
- [ ] Environment variables override config (if supported)
- [ ] Every error message has actionable guidance
- [ ] `--help` is clear and complete for all commands
- [ ] Version includes git commit hash

### Refactoring Considerations

- **Config validation:** Comprehensive validation on load
- **Config migration:** Version config for future migrations
- **Secrets handling:** Consider keychain integration for sensitive values
- **Profiles:** Consider multiple config profiles for different projects

### Code Quality Checks

- [ ] No default credentials or project IDs in code
- [ ] Config file format documented
- [ ] All config options have defaults
- [ ] Config loading errors don't expose file paths to logs

### Documentation Updates

- [ ] Complete README with quick start
- [ ] All configuration options documented
- [ ] FAQ section added
- [ ] Troubleshooting guide complete

---

## Final Review: MVP Readiness

### End-to-End Scenarios

Test complete user journeys:

1. **New user setup:**
   - [ ] Install binary
   - [ ] Run `cloudcoop` for first time
   - [ ] Complete setup wizard
   - [ ] See VM status

2. **Daily workflow:**
   - [ ] Start VM
   - [ ] Add 4 agents
   - [ ] Connect to agent, do work
   - [ ] Disconnect
   - [ ] Stop VM at end of day

3. **Monitoring workflow:**
   - [ ] Launch TUI
   - [ ] View all agent statuses
   - [ ] Kill idle agent
   - [ ] Add replacement agent

4. **Scripting workflow:**
   - [ ] `cloudcoop start && cloudcoop agents add --count=8`
   - [ ] `cloudcoop status --json | jq .state`
   - [ ] `cloudcoop stop`

### Performance Checks

- [ ] TUI startup time <500ms
- [ ] Status refresh time <2s
- [ ] Agent list refresh time <1s
- [ ] No memory leaks during extended TUI use
- [ ] Binary size reasonable (<20MB)

### Security Review

- [ ] No credentials in logs
- [ ] No credentials in error messages
- [ ] Config file has appropriate permissions
- [ ] SSH keys handled securely
- [ ] No shell injection vulnerabilities

### Release Checklist

- [ ] Version number updated
- [ ] Changelog complete
- [ ] All tests passing
- [ ] All platforms build successfully
- [ ] README installation instructions tested
- [ ] GitHub release created with binaries

---

## Iteration Dependencies

```text
Iteration 1 (Skeleton)
    │
    ▼
  Gate 1
    │
    ▼
Iteration 2 (GCP Read)
    │
    ▼
  Gate 2
    │
    ▼
Iteration 3 (VM Lifecycle)
    │
    ▼
  Gate 3
    │
    ├───────────────────────┐
    ▼                       ▼
Iteration 4 (SSH)      Iteration 8 (Config)*
    │                       │
    ▼                       ▼
  Gate 4                  Gate 8
    │
    ▼
Iteration 5 (Agents Read)
    │
    ▼
  Gate 5
    │
    ▼
Iteration 6 (Agents Write)
    │
    ▼
  Gate 6
    │
    ▼
Iteration 7 (Connect)
    │
    ▼
  Gate 7
    │
    ▼
Final Review

* Iteration 8 can be done in parallel after Gate 3
```

---

## Definition of Done (per iteration)

- [ ] All deliverables implemented
- [ ] Unit tests for new functionality
- [ ] Integration test (manual or automated) passing
- [ ] CLI `--help` updated for new commands
- [ ] TUI keybindings documented in UI footer
- [ ] No regressions in previous iteration functionality
- [ ] Gate checklist completed before next iteration

---

## Future Iterations (Post-MVP)

These are out of scope for initial tracer bullets but inform architecture:

- **VM Resize:** Change machine type (requires stop/start)
- **Bulk Operations:** Start/stop multiple agents at once
- **Cost Display:** Show hourly/monthly cost estimates
- **Multi-VM:** Support multiple VMs per project
- **Multi-Cloud:** AWS and Azure providers
- **Session Recovery:** Restore agents after spot preemption

---

## Related Documents

- [TUI Requirements](./TUI-REQUIREMENTS.md) - Detailed UI specification
- [Setup Flow](./SETUP-FLOW.md) - First-run experience
- [Development Environment](./DEVELOPMENT-ENVIRONMENT.md) - Contributing guide
