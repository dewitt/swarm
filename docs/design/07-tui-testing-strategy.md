# TUI Testing Strategy & Agentic Loop

## The Philosophy: Zero HITL for Verification

A core tenet of the `agents` project is that **Human-In-The-Loop (HITL) should
only be required for permissions or creative opinions, never for mechanical
verification.**

Agents working on this project must *never* ask the human user to run the
binary and describe what the UI looks like. The agent must verify the visual
correctness, layout, and behavior of the Terminal UI autonomously.

## How to Test Bubble Tea UIs Autonomously

Because the CLI is built on `charmbracelet/bubbletea`, the UI is simply a
function of state (`model`) mapped to a string (`View()`). Agents must use the
following techniques to test the UI without a real terminal:

### 1. Model State Verification (Unit Testing)

Instead of running `tea.NewProgram`, agents can directly instantiate the
model, pass it `tea.Msg` events (like keystrokes or window resizes), and
assert that the internal state updates correctly.

```go
func TestUpdate(t *testing.T) {
    m := initialModel()
    
    // Simulate typing 'hello'
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
    
    // Simulate pressing Enter
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
    
    // Verify the state updated without needing a human
    if len(m.messages) != 2 {
        t.Fatalf("Expected 2 messages, got %d", len(m.messages))
    }
}
```

### 2. View Rendering Verification (Snapshot Testing)

To verify what the user actually sees, agents can call `m.View()` and inspect
the resulting string. This string contains the exact ANSI escape codes and
text that would be drawn to the screen.

Agents can use `lipgloss.StripTags(m.View())` to strip the ANSI colors and
verify the raw layout, or compare the exact output against a known-good
"snapshot" string saved in the test files.

### 3. Headless PTY Execution (Integration Testing)

For full end-to-end testing of the compiled binary, agents must not run
`bin/agents` directly and hang the shell. Instead, they should:

1. Write a test script that attaches a pseudo-terminal (PTY) to the
   `bin/agents` process.
1. Send byte streams (simulating typing).
1. Capture the stdout buffer.
1. Analyze the buffer to ensure the UI rendered correctly.

If an agent needs to verify a complex visual layout that is difficult to parse
via text, they should explore generating test output via tools like `vhs` (by
Charmbracelet) which can script TUI interactions and generate frame buffers.
