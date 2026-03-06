# Bubble Tea 2.0 Migration Plan

## Overview

Charm has recently released Bubble Tea 2.0 (v2), marking a major architectural
update. The core highlight of this release is a new rendering algorithm
modeled after `ncurses`, which significantly optimizes output by only updating
the changed portions of the screen (the "cursed" renderer). This results in
vastly improved performance, particularly over SSH or lower-bandwidth
connections, and smoother complex UI updates.

This document serves as an implementation plan for migrating the Swarm CLI's
TUI from `bubbletea` v1 to v2.

## Scope and Impacted Files

Based on codebase analysis, the migration will primarily affect the following:

- **Dependencies:** `go.mod`, `go.sum`
- **Core TUI Implementation:** `cmd/swarm/interactive.go`
- **TUI Tests:** `cmd/swarm/interactive_test.go`

## Migration Steps

### 1. Update Dependencies

Bubble Tea v2 introduces a major version bump, changing the module path. We
must update the import path and ensure the broader Charm ecosystem is
compatible.

1. Run `go get github.com/charmbracelet/bubbletea/v2@latest`.
1. Update related Charm dependencies to their respective v2-compatible
   versions (if applicable), specifically:
   - `github.com/charmbracelet/bubbles`
   - `github.com/charmbracelet/lipgloss`
1. Run `go mod tidy` to clean up old dependencies.

### 2. Update Import Paths

Globally replace the old import path with the new v2 path across the Go source
files:

```go
// Old
import tea "github.com/charmbracelet/bubbletea"

// New
import tea "github.com/charmbracelet/bubbletea/v2"
```

Files to update:

- `cmd/swarm/interactive.go`
- `cmd/swarm/interactive_test.go`

### 3. Refactor Input Handling

Bubble Tea v2 introduces a declarative API and higher-fidelity input handling
(better keyboard modifiers, special keys, etc.). This may introduce breaking
changes to how `tea.KeyMsg` is structured.

1. **Audit `Update` functions:** Review the `Update(msg tea.Msg)` signature in
   `cmd/swarm/interactive.go` (specifically around lines 711-1026).
1. **Verify Key Types:** Check if constants like `tea.KeyCtrlC`, `tea.KeyEsc`,
   `tea.KeyEnter`, `tea.KeyUp`, `tea.KeyDown`, `tea.KeyPgUp` have changed
   behavior, casing, or struct properties in v2.
1. **Mouse Events:** We utilize `tea.MouseMsg` (around line 1026). Verify that
   the mouse event struct and coordinate handling remain identical or update
   them according to the v2 API.
1. **Test Fixtures:** Update `cmd/swarm/interactive_test.go` to ensure mock
   messages (e.g., `tea.KeyMsg{Type: tea.KeyEnter}`) conform to the new v2
   structures.

### 4. Review Rendering and Lip Gloss Output

The new renderer only repaints what has changed. While this is an
under-the-hood improvement, we must verify that our custom `View()` functions
and dynamic elements do not suffer from visual artifacts.

1. **Dynamic Loading States:** We use spinners (`m.spinner.Tick`) and async
   agent cards. Ensure that these frequent, localized updates render cleanly
   without flickering or leaving "ghost" characters on screen.
1. **Clear Screen Commands:** We heavily use `tea.ClearScreen`. We must
   evaluate if these explicit clears are still necessary or if the new
   renderer handles full-screen repaints more intelligently, potentially
   allowing us to remove them for better performance.

### 5. Testing and Validation

1. **Unit Tests:** Run `go test ./cmd/swarm/...` to ensure all structural
   changes are correct and logic holds.
1. **Interactive Smoke Tests:** Run `go run ./cmd/swarm` locally. Test:
   - Typing inputs and executing commands.
   - Scrolling views.
   - Asynchronous loading indicators (e.g., long-running LLM requests).
   - Resizing the terminal window (`tea.WindowSizeMsg`).
1. **SSH / Curses Edge Cases:** If possible, test the CLI over an SSH session
   to explicitly validate the performance improvements promised by the new
   "cursed" renderer.

## Risks and Mitigations

- **Upstream Ecosystem Lag:** Some third-party Bubbles or Lip Gloss components
  might not be fully v2 compatible yet.
  - *Mitigation:* Audit the specific components we use (List, Textarea,
    Spinner, Viewport). If any are broken in v2, we may need to temporarily
    fork or wait for upstream fixes before merging this migration.
- **Terminal Quirks:** The new renderer might expose bugs in niche terminal
  emulators.
  - *Mitigation:* Test across standard terminals (iTerm2, Terminal.app,
    Windows Terminal, Kitty, Alacritty) to ensure consistent behavior.
