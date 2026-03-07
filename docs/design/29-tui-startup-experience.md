# Design Document: TUI Startup Experience Redesign

**Author:** Gemini CLI Swarm
**Date:** March 2026
**Status:** Proposed

## Objective
To refine the Swarm CLI's initial startup experience by moving from a large, disruptive "splash screen" to a sleek, context-rich environment that feels professional, informative, and immediately ready for action.

## Motivation
Currently, the Swarm CLI TUI injects a massive ASCII logo and a "Recent Activity" box into the primary chat viewport (`m.messages`) as a fake message `"SPLASH_SCREEN"`.

This design has several drawbacks:
1. **Vertical Real Estate:** The large logo takes up valuable chat history space, forcing the user to scroll past it or immediately clearing it away with new commands.
2. **Context Blindness:** Aside from the recent activity, the user lacks immediate clarity on the environment they just booted into (e.g., current git branch, loaded files, Swarm version, active models).
3. **Empty States:** When no tasks are actively running in the Agent Panel, the panel just displays a basic placeholder text ("No active tasks"). This is a missed opportunity for subtle branding.

## Proposed Changes

### 1. Subtle Branding in the Agent Panel
We will repurpose the empty state of the top Agent Panel (Task Panel). Instead of rendering a large logo in the chat box, we will render a much smaller, stylized Swarm logo inside the Agent Panel *only* when `len(m.spans) == 0`.
- **Aesthetic:** A simple 1-to-3 line high-fidelity ASCII/ANSI art logo or badge that fits perfectly within the borders of the Agent panel.
- **Behavior:** As soon as a Swarm task begins (i.e., `len(m.spans) > 0`), the logo smoothly transitions out and is replaced by the dynamic task tree. When the Swarm goes idle, the logo fades back in.

### 2. Context-Rich Boot Message
Instead of the `SPLASH_SCREEN` string that renders a large layout box, the very first real message injected into the `m.messages` array upon boot will be a highly structured, markdown-formatted "System Info" block.

This message will act as a concise terminal prompt or MOTD (Message of the Day), providing:
- **Environment:** Current working directory (`pwd`), active Git branch, and number of modified files.
- **System Info:** Swarm CLI version, currently active default LLM (e.g., `gemini-2.5-pro`), and OS details.
- **Memory/Context:** A brief note on any loaded global context files or pinned memory (`AGENTS.md`, etc.).
- **Recent Activity:** A condensed 1-2 line summary of the last 3 local Git commits or the last Swarm CLI session, avoiding the need for a secondary split-pane box.

### 3. Implementation Plan
- **Step 1:** Modify `cmd/swarm/interactive.go` to remove the `"SPLASH_SCREEN"` message injection logic from `updateViewport()`.
- **Step 2:** Update `m.renderAgentPanel()` to return a stylized ASCII mini-logo when `len(m.spans) == 0`.
- **Step 3:** Introduce a new initialization function (`buildBootMessage()`) that queries the environment, Git state, and Swarm configuration to generate a markdown string. Append this string to `m.messages` as the first entry during the TUI model `Init()` phase.
- **Step 4:** Remove the redundant `fetchRecentActivity()` background tea command and the `m.cachedActivity` state tracking, as this information will now be statically captured at boot inside the boot message.

## Conclusion
This redesign aligns the Swarm CLI with modern, professional terminal tools (like `k9s` or `lazygit`). It trades flashy, disruptive startup graphics for dense, immediately useful contextual information, while maintaining brand identity through a subtle Agent Panel watermark.
