# ADR-0025: Terminal-Native Split Workflow

## Status

Accepted

## Context

Users run cloudcoop agents on a remote VM and need to view and interact with multiple agents
simultaneously. The current approach uses tmux's built-in window/pane navigation: the user
attaches to the tmux session and switches between windows using tmux key bindings.

This works but has a significant UX limitation: tmux's pane splitting and navigation uses
different key bindings than the user's terminal emulator. Users of Ghostty, iTerm2, or Kitty
already have muscle memory for their terminal's native splits (e.g., `⌘+D` in Ghostty/iTerm2).
Forcing them to use tmux's `Ctrl-B` prefix for splits and navigation creates cognitive overhead.

The goal is to let users use their terminal emulator's native split functionality, with each
split showing a different agent. The user creates splits in Ghostty (or their preferred terminal),
and each split independently connects to a different agent's tmux window.

**The challenge:** When multiple terminal splits each run `tmux attach -t <session>`, they all
share the same current window. Selecting a window in one split changes it in all splits, because
standard tmux attach shares the session's window state.

## Decision

Use tmux grouped sessions for terminal-native splits. `cloudcoop agents attach --next`
auto-assigns the next unattached tmux window to each terminal split.

**Grouped sessions:**

tmux supports "grouped sessions" via `tmux new-session -t <target-session>`. A grouped session:

- Shares all windows with the target session (windows are the same objects)
- Has its own independent current-window pointer
- Auto-destroys when the client detaches

This means each terminal split can independently navigate windows without affecting other splits.

**Attach workflow:**

```text
User in Ghostty:
┌──────────────────┬──────────────────┐
│ Split 1          │ Split 2          │
│ $ cloudcoop      │ $ cloudcoop      │
│   agents attach  │   agents attach  │
│   --next         │   --next         │
│                  │                  │
│ → Window 0:      │ → Window 1:      │
│   main           │   feature-auth   │
├──────────────────┼──────────────────┤
│ Split 3          │ Split 4          │
│ $ cloudcoop      │                  │
│   agents attach  │                  │
│   --next         │                  │
│                  │                  │
│ → Window 2:      │                  │
│   fix-payments   │                  │
└──────────────────┴──────────────────┘
```

**`--next` algorithm:**

1. Query tmux for all windows in the repo session: `tmux list-windows -t <slug>`
2. Query tmux for all clients attached to grouped sessions:
   `tmux list-clients -t <slug>` and their current windows
3. Find windows with no client currently viewing them
4. Select the first unattached window (by index)
5. Create a grouped session: `tmux new-session -t <slug> -s <slug>-<unique>`
6. Select the assigned window: `tmux select-window -t <slug>-<unique>:<window>`

**Grouped session naming:**

Each grouped session gets a unique name: `<slug>-<unique>` where `<unique>` is a short
random suffix (e.g., `acme-backend-a1b2`). These sessions auto-destroy when the client
disconnects.

**Explicit window selection:**

Users can also attach to a specific window:

```bash
cloudcoop agents attach --window feature-auth
cloudcoop agents attach --window 2
```

## Options Considered

### Option 1: tmux-Only Navigation (Current)

User attaches to tmux session and uses tmux key bindings to switch windows/panes.

**Pros:**

- Already implemented
- Works in any terminal
- Familiar to tmux power users
- No additional complexity

**Cons:**

- Conflicts with terminal emulator's native split key bindings
- Can only view one agent at a time (unless using tmux panes)
- tmux pane navigation differs from terminal split navigation
- Users must learn tmux bindings
- No way to see multiple agents simultaneously using native terminal splits

### Option 2: Detach-on-Select Hooks

Use tmux hooks to automatically detach and reattach when a window is selected,
assigning each client a "home" window.

**Pros:**

- Works with standard tmux attach
- No grouped sessions needed

**Cons:**

- Complex hook logic
- Fragile — hooks can fail or have timing issues
- Non-standard tmux behaviour confuses users
- Detach/reattach causes visible flicker
- Difficult to debug

### Option 3: Grouped Sessions (Chosen)

Use tmux grouped sessions with `--next` auto-assignment.

**Pros:**

- Native tmux feature — stable and well-tested
- Each split has independent window selection
- Auto-destroys on disconnect (no cleanup needed)
- `--next` makes setup trivial (no manual window assignment)
- Works with any terminal emulator that supports splits
- Users keep their terminal's native key bindings for splits

**Cons:**

- `tmux list-clients` output must be parsed to find unattached windows
- Grouped session names add noise to `tmux list-sessions`
- Race condition possible if two `--next` commands run simultaneously
  (mitigated by using tmux's atomic session creation)
- Users unfamiliar with grouped sessions may be confused by `tmux ls` output

### Option 4: Separate Sessions per Window

Create independent tmux sessions for each agent window (e.g., `acme-backend-main`,
`acme-backend-feature-auth`).

**Pros:**

- Simple mental model — one session per agent
- No grouped session complexity
- Easy to attach to specific agent

**Cons:**

- Loses the concept of a repo's agents as a group
- Many sessions clutter `tmux list-sessions`
- No shared window list — can't easily see all agents for a repo
- Adding/removing agents requires session lifecycle management
- Doesn't compose with the repo-scoped session model ([ADR-0023](0023-repo-scoped-tmux-sessions.md))

## Consequences

### Positive

- Users can use their terminal emulator's native splits (Ghostty ⌘+D, iTerm2 ⌘+D, etc.)
- Each split independently shows a different agent
- `--next` auto-assignment eliminates manual window selection during setup
- Grouped sessions auto-destroy on disconnect — no cleanup needed
- Works with any terminal emulator, not just Ghostty

### Negative

- Slightly more complex attach logic compared to simple `tmux attach`
- `tmux list-sessions` shows ephemeral grouped sessions alongside repo sessions
- Must handle the race condition where two `--next` calls compete for the same window
- Users may still need tmux bindings for operations within a single window

### Neutral

- Depends on [ADR-0023](0023-repo-scoped-tmux-sessions.md) for repo-scoped session names
- The `--next` algorithm is deterministic (lowest-index unattached window) so repeated
  calls produce predictable assignments
- Terminal emulator-specific split commands (Ghostty, iTerm2, Kitty) remain the user's
  responsibility — cloudcoop provides the attach command, not the split
