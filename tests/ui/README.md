# UI Regression Testing Guide

Whenever code that impacts the text entry UI, viewport, or main terminal
layout is modified, you MUST run this regression test to verify that the UI
remains stable and functional.

## How to run the test:

1. Rebuild the application:
   ```bash
   go build -o bin/agents ./cmd/agents
   ```
1. Run the VHS tape to generate the visual output:
   ```bash
   vhs < tests/ui/text_entry.tape
   ```
1. Use your vision capabilities via the `read_file` tool to visually review
   the generated GIF (`tests/ui/text_entry.gif`).

## Acceptance Criteria (What to look for in the GIF):

When reviewing the output, ensure the following criteria are perfectly met:

1. **Layout Stability**: The input box must maintain a constant height. The
   overall UI layout (including the status bar and viewport) must **NOT jump
   or shift** when `Enter` is pressed or when the background "Thinking..."
   spinner replaces the text box.
1. **No Submission Leakage**: When hitting `Enter` to submit a message, the
   textarea must clear completely. When the next message is typed, the text
   must appear strictly on the **FIRST line** of the input box, right next to
   the `> ` prompt. The `Enter` keystroke must not "leak" and insert an empty
   newline before the text.
1. **Multiline Support**: Pressing `Alt+Enter` or `Ctrl+J` must insert a
   newline in the input box, allowing for multiline message composition, and
   should not trigger a submission.
1. **History Navigation**: Pressing `Up` and `Down` must properly cycle
   through previous input commands without causing rendering artifacts or
   prompt duplication.
