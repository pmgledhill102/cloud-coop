# ADR-0018: TUI State Machine

## Status

Accepted

## Context

The cloudcoop TUI needs to manage complex asynchronous operations (VM start/stop, agent add/kill,
SSH connections) while maintaining a responsive user interface. We need a clear state management
pattern that handles operation states, prevents conflicting operations, and provides confirmation
flows for destructive actions.

The question is how to structure the state machine to handle these requirements while keeping the
code maintainable and predictable.

## Decision

Implement the TUI using the Elm Architecture pattern via the Bubbletea framework, with explicit
operation states, typed messages for all async operations, and confirmation flows for destructive
actions.

### State Model

The Model struct maintains several state categories:

1. **UI State**: width, height, ready, loading flags
2. **Operation State**: string field tracking current operation ("", "starting", "stopping",
   "adding", "killing")
3. **Data State**: config, VM info, agents list with associated errors
4. **Selection State**: selectedAgentIdx for agent list navigation
5. **Confirmation State**: confirmingKill flag with target index/name

### Operation States

The `operation` field acts as a mutex for long-running operations:

- **""** (empty): Idle state, user can initiate operations
- **"starting"**: VM start in progress
- **"stopping"**: VM stop in progress
- **"adding"**: Agent creation in progress
- **"killing"**: Agent termination in progress

When `operation != ""`, most user inputs are blocked to prevent conflicting actions.

### Message Types

All async operations communicate via typed messages:

```go
type configLoadedMsg struct { cfg *config.Config; err error }
type vmInfoMsg struct { info *cloud.VMInfo; err error; cleanup func() }
type vmStartMsg struct { err error }
type vmStopMsg struct { err error }
type agentsMsg struct { result *agent.ListResult; err error }
type agentAddedMsg struct { session *agent.Session; err error }
type agentKilledMsg struct { index int; err error }
type connectFinishedMsg struct { err error }
```

### Confirmation Flow

Destructive operations (killing agents) use a two-step confirmation:

1. User presses K to initiate kill
2. Model sets `confirmingKill = true` and stores target info
3. View renders confirmation dialog
4. User presses Y to confirm or N/Esc to cancel
5. On confirm: sets `operation = "killing"` and dispatches kill command
6. On cancel: clears `confirmingKill` flag

## Options Considered

### Option 1: Single State Field with Enum

Use a single state field with all possible states (idle, loading, starting, stopping, etc.).

**Pros:**

- Simple state representation
- Clear state transitions

**Cons:**

- Combinatorial explosion when combining states (loading + confirming, etc.)
- Hard to track multiple concurrent concerns

### Option 2: Multiple Boolean Flags (Chosen)

Use separate fields for different concerns: loading, operation, confirmingKill.

**Pros:**

- Each concern tracked independently
- Easy to add new concerns without refactoring
- Natural fit for Elm Architecture's Update function
- Clear separation between UI state and operation state

**Cons:**

- Possible invalid state combinations (mitigated by canModifyAgents() helper)
- State spread across multiple fields

### Option 3: Explicit State Machine Library

Use a formal state machine library with defined transitions.

**Pros:**

- Compile-time guarantees about valid transitions
- Formal state machine semantics

**Cons:**

- Additional dependency
- Overhead for relatively simple state model
- Less idiomatic for Bubbletea applications

## Consequences

### Positive

- Predictable state management following Elm Architecture
- Operation mutex prevents conflicting async operations
- Confirmation flow protects against accidental destructive actions
- Typed messages provide compile-time safety for async communication
- Helper methods (canModifyAgents) centralize state validation logic
- View function can render appropriate UI based on current state combination

### Negative

- State validation logic must be maintained in multiple places (key handlers + canModifyAgents)
- Operation field uses string type rather than enum constants
- No formal state transition validation

### Neutral

- Each async operation follows the pattern: set operation -> dispatch command -> handle message ->
  clear operation -> refresh state
- Error handling stores errors in model for display, doesn't interrupt operation flow
- Window size and ready state tracked separately from business logic state
