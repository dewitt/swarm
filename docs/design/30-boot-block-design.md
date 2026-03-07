# Design Document: Initial Context Message ("Boot Block")

**Author:** Gemini CLI Swarm
**Date:** March 2026
**Status:** Brainstorming

## Objective
To design a "Boot Block"—the very first message rendered in the Swarm CLI's chat viewport—that serves as a comprehensive, highly readable dashboard. It must instantly orient the user to their exact context (environment, active session, loaded files, and system state) without overwhelming them, replacing the legacy splash screen.

## Core Principles
1. **Density vs. Readability:** We want to pack a lot of information (paths, commits, states) into a small vertical footprint using Markdown formatting (tables, bolding, bullet points) so it is scannable at a glance.
2. **Contextual Awareness:** If the user is resuming an old session, that should be immediately obvious. If they have pinned a specific `AGENTS.md` file, that should be visible.
3. **Visual Hierarchy:** Use headers and color (via Glamour markdown styles) to separate "Environment" (Git/Files) from "System" (Swarm version, Model) and "Session" (History/Resume state).

## Proposed Information Architecture

### 1. Header (The "Logo" replacement)
A subtle, single-line header indicating the tool and version.
*Example:* `🤖 Swarm CLI (v0.0.3) - Ready.`

### 2. Environment & Git State
Where am I, and what is the state of the codebase?
*   **Workspace:** The current directory (ideally shortened if it's the user's home dir, e.g., `~/git/swarm`).
*   **Branch:** The active git branch.
*   **Status:** Clean, or number of modified/untracked files.
*   **Recent Commit:** Just the *single* most recent commit (hash and message) to provide immediate context without taking up 3 lines like the previous iteration.

### 3. Session & Swarm State
What am I doing right now?
*   **Session State:** "New Session" vs. "Resuming Session: [Session ID/Summary]".
*   **Active Model:** The default LLM provider (e.g., `gemini-2.5-pro`).
*   **Context Files:** A list of globally loaded or pinned files (e.g., `AGENTS.md`, `.gemini/GEMINI.md`). If the list is empty, omit this section.

## Design Iterations (Markdown Mockups)

### Iteration 1: The List View (Current approach, but expanded)
```markdown
**Swarm CLI** `v0.0.3`

**Environment:**
- **Workspace:** `~/git/swarm`
- **Branch:** `main` (3 modified files)
- **HEAD:** `a4e5f60` Refactor TUI startup experience

**Session:**
- **Status:** Resuming previous session
- **Model:** `gemini-2.5-pro`
- **Context:** `AGENTS.md`, `docs/design/29-tui-startup-experience.md`
```
*Critique 1:* Clean, but slightly vertically greedy.

### Iteration 2: The Table View
```markdown
### Swarm CLI (v0.0.3)

| Environment | | Session | |
| :--- | :--- | :--- | :--- |
| **Dir** | `~/git/swarm` | **State** | Resuming |
| **Branch** | `main` (Modified) | **Model** | `gemini-2.5-pro` |
| **HEAD** | `a4e5f60` | **Context** | `AGENTS.md` |
```
*Critique 2:* Very dense, saves vertical space. Glamour renders tables reasonably well, but it might feel a bit too rigid or "spreadsheet-like" for a chat interface.

### Iteration 3: The "K9s / Lazygit" Header Style (Dense key-value pairs)
```markdown
**Swarm CLI** `v0.0.3` | **Model:** `gemini-2.5-pro` | **State:** `Resuming session`

**Dir:** `~/git/swarm`  | **Branch:** `main` *(modified)* | **HEAD:** `a4e5f60`
**Context:** `AGENTS.md`, `docs/design/29-tui-startup-experience.md`
```
*Critique 3:* Extremely space-efficient. It uses the horizontal width of the terminal effectively. This requires careful markdown formatting to ensure it doesn't wrap awkwardly on very narrow terminals, but it looks the most "pro".

## Technical Requirements for Implementation
To achieve Iteration 3 (or similar), we need to update the `buildBootMessage` function in `cmd/swarm/interactive.go` to gather more data:
1.  **Session Info:** We need to know if the session was resumed, and ideally, its ID or summary.
2.  **Context Files:** We need to access the Swarm engine's `ListContext()` or pinned files list.
3.  **Path Shortening:** A helper to replace `/Users/name/` with `~/` for cleaner display.
4.  **Single Commit:** Modify the git helper to just grab the `HEAD` commit hash and short message.
