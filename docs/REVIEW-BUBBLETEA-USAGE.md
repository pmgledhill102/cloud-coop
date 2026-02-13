# Review: Bubbletea TUI Framework Usage

> Date: 2026-02-13
> Scope: Evaluation against Bubbletea/Bubbles/Lipgloss best practices,
> identifying missed opportunities

## Executive Summary

The TUI implementation is **functional and well-tested**, with
excellent async command handling and well-typed messages. However, it
is built as a **monolithic single-model architecture** that doesn't
leverage the Bubbles component library, resulting in manual
reimplementations of standard components (help display, key handling,
list selection, scrolling).

**Key findings:**

- Only 1 of ~10 available Bubbles components is used (spinner)
- Manual key handling instead of the `key` + `help` bubbles pattern
- Monolithic Model with 24+ fields instead of composed sub-models
- Hardcoded ANSI colours that don't adapt to light/dark terminals
- Terminal width/height captured but never used in rendering

---

## 1. Component Architecture -- Monolithic (Anti-Pattern)

The entire TUI is a single `Model` struct with 24+ fields
(`app.go:18-68`):

```go
type Model struct {
    width, height        int
    ready, loading       bool
    refreshing           bool
    autoRefreshPaused    bool
    operation            string
    cfg                  *config.Config
    cfgErr               error
    vmInfo               *cloud.VMInfo
    vmErr                error
    agents               *agent.ListResult
    agentsErr            error
    agentsLoading        bool
    provisionStatus      *provisioning.StatusInfo
    provisionErr         error
    provisionLoading     bool
    selectedAgentIdx     int
    confirmingKill       bool
    killTargetIndex      int
    killTargetName       string
    selectingSize        bool
    sizeOptions          []string
    selectedSizeIdx      int
    confirmingDelete     bool
    workspaceInfo        *workspace.Info
    firewallChecked      bool
    sshKeyChecked        bool
    showHelp             bool
    spinner              spinner.Model
    cleanup              func()
}
```

**Impact:**

- High cognitive load when reading the Update function
- Test setup requires constructing many fields (`app_test.go:28-32`)
- Difficult to add new features without further bloating the model

**Recommended decomposition:**

| Sub-model | Fields | Responsibility |
|-----------|--------|----------------|
| `vm.Model` | cfg, vmInfo, vmErr, operation | VM state and lifecycle |
| `agents.Model` | agents, agentsErr, selectedIdx | Agent list and selection |
| `provision.Model` | provisionStatus, provisionErr | Provisioning display |
| `ui.Model` | showHelp, selectingSize, width | UI chrome and modals |

Each sub-model would implement `tea.Model` and handle its own
messages, with the parent routing messages to the appropriate child.

---

## 2. Message Handling -- Excellent (Strength)

Messages are well-typed custom structs (`commands.go:30-69`):

```go
type configLoadedMsg struct {
    cfg       *config.Config
    err       error
    workspace *workspace.Info
}
type vmInfoMsg struct {
    info    *cloud.VMInfo
    err     error
    cleanup func()
}
type agentsMsg struct {
    result *agent.ListResult
    err    error
}
type vmStartMsg struct{ err error }
type refreshTickMsg struct{}
```

The `Update` function uses clean type switches (`app.go:96-166`),
avoiding string-based dispatch. This is exemplary Bubbletea practice.

The handler functions are well-factored too: `handleVMInfo()`,
`handleAgents()`, `handleVMOpResult()` etc. (`handlers.go`).

No changes recommended.

---

## 3. Command Handling -- Excellent (Strength)

All async operations correctly return `tea.Cmd` closures:

- `loadConfig()`, `fetchVMInfo()`, `startVM()`, `stopVM()` etc.
  (`commands.go`)
- `tea.ExecProcess()` used correctly for SSH attachment
  (`commands.go:268-298`)
- `tea.Batch()` used to compose concurrent commands
  (`handlers.go:107, 112`)

**One concern:** `createVM()` includes a synchronous
`ssh.WaitForSSH()` call that blocks for up to 30 seconds inside the
`tea.Cmd` closure. While this runs on a background goroutine (correct
for Bubbletea), consider whether a progress indication should be shown
during this wait.

---

## 4. Bubbles Library Usage -- Severely Underutilised (Major Gap)

Currently only `spinner` is used. The following standard Bubbles
components would significantly improve the TUI:

### 4a. `key` + `help` Bubbles (HIGH PRIORITY)

**Current approach** -- manual string-based key handling
(`handlers.go:91-163`):

```go
key := strings.ToLower(msg.String())
switch key {
case "q", "ctrl+c": // quit
case "r":            // refresh
case "s":            // start
// ...20+ more cases...
```

**And manual help rendering** (`view.go:225-259`):

```go
actions := []string{"?: help", "q: quit", "r: refresh"}
if m.autoRefreshPaused {
    actions = append(actions, "a: resume auto")
}
// ...20+ more conditional lines...
return helpStyle.Render(strings.Join(actions, " . "))
```

**Better approach** using `key.Binding` + `help.Model`:

```go
type KeyMap struct {
    Quit    key.Binding
    Refresh key.Binding
    Start   key.Binding
    Stop    key.Binding
    // etc.
}

var keys = KeyMap{
    Quit:    key.NewBinding(
        key.WithKeys("q", "ctrl+c"),
        key.WithHelp("q", "quit"),
    ),
    Refresh: key.NewBinding(
        key.WithKeys("r"),
        key.WithHelp("r", "refresh"),
    ),
    Start:   key.NewBinding(
        key.WithKeys("s"),
        key.WithHelp("s", "start"),
    ),
}

// In Update:
if key.Matches(msg, m.keys.Quit) { ... }

// Help is auto-generated from bindings
helpView := m.help.View(m.keys)
```

**Benefits:**

- Centralised key definition (single source of truth)
- Help text auto-generated from bindings
- Context-sensitive help: enable/disable bindings based on state
- Foundation for user-configurable key bindings

### 4b. `list` Bubble (MEDIUM PRIORITY)

**Current approach** -- manual cursor tracking for agent selection
(`handlers.go:154-161`):

```go
case "up", "k":
    if m.hasAgents() && m.selectedAgentIdx > 0 {
        m.selectedAgentIdx--
    }
case "down", "j":
    if m.hasAgents() && m.selectedAgentIdx < len(m.agents.Sessions)-1 {
        m.selectedAgentIdx++
    }
```

And manual rendering (`view.go:164-199`):

```go
for i, s := range m.agents.Sessions {
    sel := "  "
    if i == m.selectedAgentIdx {
        sel = "> "
    }
    // ...format and append...
}
```

The `list` bubble provides all of this plus filtering, pagination,
mouse support, and auto-generated help. It would be particularly
valuable as the number of agents grows.

### 4c. `viewport` Bubble (MEDIUM PRIORITY)

When the terminal is small or the agent list is long, content
overflows with no scrolling. The `viewport` bubble provides scrollable
content areas with keyboard navigation.

### 4d. `progress` Bubble (LOW PRIORITY)

During long operations like VM creation (300s timeout), only a spinner
is shown. A progress bar would provide better feedback, especially if
provisioning status progress can be mapped to a percentage.

---

## 5. Window Sizing -- Captured but Unused

Terminal resize is handled (`app.go:102-106`):

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    m.ready = true
```

However, **`m.width` and `m.height` are never referenced in any
rendering code**. The `view.go` functions don't adapt to terminal
size. This means:

- Content can overflow on narrow terminals
- The box border doesn't resize
- Agent lists can exceed terminal height

**Recommendation:** Use `m.width` in Lipgloss rendering:

```go
boxStyle.Width(m.width - 4).Render(content)
```

And use `m.height` to determine how many agent rows to show, or adopt
the `viewport` bubble.

---

## 6. Styling -- Centralised but Not Adaptive

**Strength:** All styles are centralised in `styles.go`:

```go
titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
runningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
stoppedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
```

**Problem:** Hardcoded ANSI colour numbers don't adapt to light/dark
terminal backgrounds. Colour 46 (bright green) is invisible on some
light themes; colour 241 (grey) may be invisible on certain dark
themes.

**Recommendation:** Use `lipgloss.AdaptiveColor`:

```go
runningStyle = lipgloss.NewStyle().
    Foreground(lipgloss.AdaptiveColor{Light: "#00875F", Dark: "#00FF87"})
```

---

## 7. View Rendering -- Functional but Could Use Lipgloss Layout

The view uses string concatenation and `strings.Join()`:

```go
// view.go:46
return fmt.Sprintf(
    "\n%s\n%s\n\n%s\n\n%s\n",
    title, subtitle, content, m.renderHelp(),
)
```

```go
// view.go:111
return boxStyle.Render(strings.Join(lines, "\n"))
```

**Recommendation:** Use Lipgloss layout functions for cleaner
composition:

```go
return lipgloss.JoinVertical(lipgloss.Top,
    titleSection,
    vmStatusSection,
    agentListSection,
    helpSection,
)
```

This makes layout changes easier and integrates better with responsive
sizing.

---

## 8. Error Display -- Consistent but No Auto-Clear

Errors are displayed consistently via `errorStyle` rendering. The
error handling flow is well-structured:

```go
func (m Model) handleVMOpResult(err error) (Model, tea.Cmd) {
    m.operation = ""
    if err != nil {
        m.vmErr = err
        return m, nil
    }
    m.loading = true
    return m, fetchVMInfo(m.cfg)
}
```

**Issue:** Errors persist until the user manually refreshes (`r` key).
Consider auto-clearing errors after a timeout:

```go
return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg {
    return clearErrorMsg{}
})
```

---

## 9. Testing -- Good but Not Using TestModel Helper

The test suite is comprehensive with 70+ test cases covering key
handlers, message handlers, and view rendering (`app_test.go`).

**Strength:** Clear table-driven tests with good coverage:

```go
func TestCanModifyAgents(t *testing.T) {
    tests := []struct {
        name string
        m    Model
        want bool
    }{...}
}
```

**Issue:** A well-designed `TestModel` helper exists in
`internal/testutil/tui.go` with `SendKey()`, `SendRune()`,
`AssertViewContains()` methods, but it is **not used in
`app_test.go`**. Tests manually construct models and call `Update()`
directly.

**Recommendation:** Migrate tests to use the existing
`testutil.TestModel` for more readable and maintainable tests.

---

## 10. Help Overlay -- Manual Implementation

The full help overlay (`view.go:261-290`) is a hardcoded string:

```go
func (m Model) renderHelpOverlay() string {
    help := `Keyboard Shortcuts

  Navigation
    k/up        Move up
    j/down      Move down
  ...`
    return boxStyle.Render(help)
}
```

This must be manually kept in sync with the actual key handlers in
`handlers.go`. If a key binding changes, the help string must also be
updated manually.

**With the `key` + `help` bubbles, this would be auto-generated from
the binding definitions, eliminating the maintenance burden.**

---

## Summary Scorecard

| Criterion | Rating | Notes |
|-----------|--------|-------|
| Component Architecture | Poor | Monolithic model, 24+ fields |
| Message Handling | Excellent | Well-typed custom structs |
| Command Handling | Excellent | Proper async tea.Cmd usage |
| Bubbles Library Usage | Poor | Only spinner; missing help, key, list |
| Key Bindings | Poor | Manual string-based handling |
| Window Sizing | Adequate | Captured but never used in rendering |
| View Rendering | Adequate | Works but uses string concatenation |
| Styling | Fair | Centralised but hardcoded colours |
| Error Handling | Good | Consistent pattern, no auto-clear |
| Testing | Good | Comprehensive; testutil helper unused |

---

## Top 5 Recommendations (Priority Order)

1. **Adopt `key` + `help` bubbles** (Medium effort) -- Replace manual
   key handling with `key.KeyMap` + `help.Model`. Auto-generates help,
   enables context-sensitive bindings. Eliminates ~100 lines of manual
   help rendering and makes key bindings a single source of truth.

2. **Use adaptive colours in Lipgloss** (Low effort) -- Replace
   hardcoded ANSI colour numbers with `AdaptiveColor` for light/dark
   terminal support. ~10 lines changed in `styles.go`.

3. **Use terminal dimensions in rendering** (Low effort) -- Reference
   `m.width` and `m.height` in view rendering for responsive layout.
   Prevents overflow on small terminals.

4. **Add `viewport` for agent list** (Medium effort) -- Prevents
   content loss when agent list exceeds terminal height. Becomes
   essential as users run more agents.

5. **Decompose into sub-models** (High effort) -- Split Model into
   vm, agents, provision, and ui sub-models. Improves maintainability
   but is a larger refactor best done when adding significant new TUI
   features.
