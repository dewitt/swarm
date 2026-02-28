# Context Management: Deep Dive & Competitive Analysis

As the complexity of a task grows, the LLM's context window becomes the most
critical constraint. Managing what the agent "sees" is the primary challenge
in CLI agent design. This document compares how leading tools handle context
and proposes a unified, market-aligned design for the `swarm` CLI.

## Competitive Landscape

### 1. Claude Code

- **Implicit Loading:** Tends to rely heavily on the agent's ability to
  autonomously search and read files using its own internal tools (e.g.,
  `GlobTool`, `GrepTool`, `ViewTool`). It dynamically fetches context as
  needed to answer questions.
- **Explicit Injection (Per-Turn):** The user can type `@filename` to force a
  file into the context for that specific prompt.
- **Pinning:** Claude Code does *not* heavily emphasize a manual pinning
  system (like `/context add`), preferring the agent to manage its own working
  memory, though history retention achieves a similar effect over a session.

### 2. Cursor (Editor)

- **Explicit Injection (Per-Turn):** The `@` symbol is the primary UX
  paradigm. Users type `@` to bring up a menu of files, symbols, docs, and web
  pages, injecting them into the current chat turn.
- **Context Pinning (Chat Context):** Users can explicitly attach files to a
  chat session. These files stay "pinned" to the top of the context window for
  every subsequent turn in that chat thread until removed.

### 3. Gemini CLI

- **Workspace Context:** Can load entire directories of context upfront, but
  this can lead to massive token consumption on large repositories.
- **Explicit Management:** Emphasizes specific files via command-line
  arguments or explicit reads.
- **Plan vs Act:** Separates the context of "brainstorming" from "execution"
  to manage token use safely.

### 4. Codex (Early Implementations)

- **Heavy Bash Reliance:** Relied on `cat`, `ls`, and `grep` executed via bash
  to pipe context into the LLM. Extremely inefficient token usage and highly
  prone to context blowout.

## Core UX Paradigms Identified

The market has converged on two primary workflows for context management:

1. **Transient Injection (`@` Reference):** The user explicitly provides a
   file for a *single turn*. (e.g., "Refactor `@app.go` to use the new DB
   driver.") The file is read, injected, and then naturally ages out of the
   context window as the conversation progresses.
1. **Persistent Pinning ("The Pinned Context"):** The user declares that a
   file is foundational to the *entire session*. (e.g., "I'm working on a
   massive refactor of `manager.go`, never forget what's in this file.") This
   file is injected into the system prompt or prepended to every single turn.

## Proposed Design for `swarm` CLI

Following our "UX Familiarity" principle, we will adopt the market-standard
paradigms:

### Phase 1: Transient Injection (`@` Syntax)

- **Status:** **Implemented**.
- **UX:** The user types `@path/to/file.go`. The CLI automatically reads the
  file and prepends it to the user's prompt text before sending it to the SDK.
- **Benefit:** Instantly familiar to any Cursor or Claude user.

### Phase 2: Autonomous Context (Agentic Retrieval)

- **Status:** **Implemented**.
- **UX:** The user asks a vague question ("How does logging work?"). The agent
  uses the `read_local_file` and `grep_search` tools to find the answer.
- **Benefit:** Reduces the burden on the user to know exactly which files to
  load.

### Phase 3: Persistent Context Pinning (`/context` and `/drop`)

- **Status:** **Implemented**.
- **UX:**
  - `/context add <file_path>`: Pins a file to the active session.
  - `/context`: Lists all currently pinned files.
  - `/drop <file_path>`: Removes a file from the pinned context.
  - `/drop all`: Clears the pinned context.
- **Mechanics:** Pinned files are stored in a `map[string]string` in the
  `AgentManager`. Before *every* `Chat` execution, the contents of all pinned
  files are injected into the system prompt instructions (similar to how we
  inject `GEMINI.md` or global memory).
- **Why this deviates slightly from Claude Code:** While Claude relies almost
  entirely on agentic retrieval, explicit pinning is a highly requested
  power-user feature (popularized by Cursor) that provides deterministic
  control over token usage and prevents the agent from "forgetting" critical
  files during long, sprawling debugging sessions.
