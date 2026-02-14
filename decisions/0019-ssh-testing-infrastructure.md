# ADR-0019: SSH Testing Infrastructure

## Status

Accepted

## Context

Testing SSH-related functionality presents several challenges:

1. **Network restrictions**: Claude Code and similar AI coding assistants run in sandboxed environments
   that block SSH connections (see ADR-0015)
2. **External dependencies**: Real SSH tests require running VMs, which are slow, expensive, and
   introduce flakiness
3. **Test isolation**: SSH operations involve network I/O and remote state, making tests hard to
   reproduce reliably
4. **Interface verification**: Code that depends on SSH operations needs to verify correct command
   sequences without executing them

The `internal/ssh` package defines a `Runner` interface for executing remote commands:

```go
type Runner interface {
    Run(cmd string) (string, error)
    Close() error
}
```

We need a testing infrastructure that allows unit testing code that depends on this interface without
requiring actual SSH connections.

## Decision

Implement a **mock SSH client with expectations-based testing** in the `internal/testutil` package.
The mock supports:

1. **Command expectations**: Define expected commands with their responses
2. **Wildcard pattern matching**: Match commands using `*` wildcards for flexible assertions
3. **Thread-safe implementation**: Safe for concurrent test execution
4. **Verification helpers**: Assert all expectations were met and no unexpected calls occurred

## Options Considered

### Option 1: Interface-Based Test Doubles (Manual Stubs)

Create ad-hoc stub implementations for each test.

```go
type stubSSHClient struct {
    output string
    err    error
}

func (s *stubSSHClient) Run(cmd string) (string, error) {
    return s.output, s.err
}
```

**Pros:**

- Simple to understand
- No additional dependencies
- Full control over behaviour

**Cons:**

- Repetitive boilerplate in each test
- No built-in verification of expected calls
- Cannot verify command sequences
- Easy to miss testing edge cases

### Option 2: Third-Party Mocking Framework (gomock, testify/mock)

Use an established mocking library.

**Pros:**

- Feature-rich (argument matchers, call counts, ordering)
- Well-documented patterns
- Code generation support

**Cons:**

- Additional dependency
- Learning curve for contributors
- Generated code can be harder to debug
- May be overkill for simple interface

### Option 3: Custom Mock with Expectations Pattern (Chosen)

Build a purpose-built mock that supports command expectations with pattern matching.

**Pros:**

- Tailored to SSH command testing needs
- Simple API without external dependencies
- Supports wildcard patterns for command matching
- Thread-safe for parallel tests
- Built-in verification helpers

**Cons:**

- Custom code to maintain
- Less feature-rich than full mocking frameworks
- May need enhancement as testing needs grow

## Implementation

### MockSSHClient Structure

```go
// internal/testutil/ssh_mock.go

type MockSSHClient struct {
    mu           sync.Mutex
    expectations []*CommandExpectation
    calls        []string
    closed       bool
    anyCommand   bool
    defaultOut   string
    defaultErr   error
}

type CommandExpectation struct {
    Command string
    Output  string
    Err     error
    called  bool
}
```

### Usage Patterns

#### Basic Command Expectation

```go
func TestListAgents(t *testing.T) {
    mock := testutil.NewMockSSHClient()
    mock.ExpectCommand("tmux list-windows -t agents").Return("0:agent-1\n1:agent-2", nil)

    // Code under test
    agents, err := listAgents(mock)

    require.NoError(t, err)
    assert.Len(t, agents, 2)
    mock.AssertExpectations(t)
}
```

#### Wildcard Pattern Matching

For commands with dynamic arguments:

```go
func TestCreateAgent(t *testing.T) {
    mock := testutil.NewMockSSHClient()
    // Match any tmux new-window command
    mock.ExpectCommand("tmux new-window *").Return("", nil)

    // Code under test
    err := createAgent(mock, "task-123")

    require.NoError(t, err)
    mock.AssertExpectations(t)
}
```

Pattern matching rules:

- `*` matches any substring
- `prefix*` matches commands starting with "prefix"
- `*suffix` matches commands ending with "suffix"
- `pre*mid*suf` matches with wildcards between literal parts

#### Accept Any Command

For tests that don't care about specific commands:

```go
func TestErrorHandling(t *testing.T) {
    mock := testutil.NewMockSSHClient()
    mock.ExpectAnyCommand("", errors.New("connection refused"))

    // Test error handling
    err := doSSHOperation(mock)
    assert.Error(t, err)
}
```

#### Verify Call History

```go
func TestCommandSequence(t *testing.T) {
    mock := testutil.NewMockSSHClient()
    mock.ExpectAnyCommand("", nil)

    // Code under test
    setupAgent(mock)

    calls := mock.Calls()
    assert.Contains(t, calls[0], "tmux has-session")
    assert.Contains(t, calls[1], "tmux new-session")
}
```

### Thread Safety

The mock uses `sync.Mutex` to protect all state access:

```go
func (m *MockSSHClient) Run(cmd string) (string, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.calls = append(m.calls, cmd)
    // ... expectation matching
}
```

This enables safe use with `t.Parallel()`:

```go
func TestConcurrent(t *testing.T) {
    t.Parallel()

    mock := testutil.NewMockSSHClient()
    mock.ExpectAnyCommand("output", nil)

    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            mock.Run("cmd")
        }()
    }
    wg.Wait()

    assert.Len(t, mock.Calls(), 10)
}
```

### Verification Methods

| Method | Purpose |
|--------|---------|
| `AssertExpectations(t)` | Fail if any expected command was not called |
| `AssertNoUnexpectedCalls(t)` | Fail if commands were called without expectations |
| `Calls()` | Return list of all executed commands |
| `IsClosed()` | Check if `Close()` was called |
| `Reset()` | Clear all state for test reuse |

### Integration with Runner Interface

The mock implements the same interface as the real SSH client:

```go
// internal/ssh/client.go
type Runner interface {
    Run(cmd string) (string, error)
    Close() error
}

// internal/testutil/ssh_mock.go
var _ ssh.Runner = (*MockSSHClient)(nil)  // Compile-time check
```

This allows dependency injection in code under test:

```go
type AgentManager struct {
    ssh ssh.Runner
}

func NewAgentManager(runner ssh.Runner) *AgentManager {
    return &AgentManager{ssh: runner}
}
```

## Consequences

### Positive

- SSH-dependent code can be unit tested without network access
- Tests run fast (no actual connections)
- Tests are deterministic and reproducible
- Wildcard patterns reduce test brittleness
- Thread-safe design supports parallel test execution
- No external mocking dependencies

### Negative

- Custom mock requires maintenance
- Mock may not perfectly simulate real SSH behavior
- Integration tests still needed for end-to-end validation
- Pattern matching is simple (no regex support)

### Neutral

- Mock lives in `internal/testutil`, separate from production code
- Tests must use dependency injection pattern
- Documentation and examples guide correct usage

## Related Decisions

- ADR-0013: SSH and Remote Execution (defines the `Runner` interface)
- ADR-0015: SSH Testing in Sandboxed Environments (addresses network restrictions)
