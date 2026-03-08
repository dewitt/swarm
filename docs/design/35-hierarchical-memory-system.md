# Design: Hierarchical Memory & Context Compression

*Resolves multiple initiatives in Epic #29 (The Self-Healing Swarm).*

## 1. Context & Motivation

As Swarm scales into a massively multi-agent orchestrator, our context
management is becoming a bottleneck. Currently, we rely on disparate systems:

- **Pinned Context:** Explicitly forcing files into the prompt
  (`/context add`).
- **Session History:** A linear, uncompressed log of user messages and agent
  responses stored in SQLite.
- **Working Context:** The ephemeral execution trajectory passed between
  routing and reflection phases.
- **Global Preferences:** The `~/.gemini/GEMINI.md` and `.gemini/GEMINI.md`
  files.

**The Problem:** Context grows linearly until it hits a token limit (context
blowout), driving up latency and cost, while degrading the LLM's reasoning
ability. Furthermore, "lessons learned" (such as discovering a tool is broken,
or figuring out the undocumented build command for a repo) are either
forgotten between sessions or require rigid, hardcoded database schemas (like
the SQLite table proposed in *Design Doc 34: Tool Reliability Scoring*).

**The Vision:** We need a unified **Hierarchical Memory System**. This system
will dynamically compress old context, explicitly manage facts, and seamlessly
merge tool reliability tracking into a generalized semantic knowledge base.

## 2. Architecture: The 4-Tier Memory Model

We propose categorizing all state into four temporal tiers. The Swarm Engine
will orchestrate how information flows between them.

### Tier 1: Working Memory (Immediate Context)

- **Scope:** The current active execution cycle
  (`Plan -> Execute -> Reflect`).
- **Content:** The immediate execution graph, stdout from the *current* tool
  invocations, and the active reflection reasoning.
- **Management:** Highly volatile. Passed directly in the prompt. Once a cycle
  resolves, the outcome is committed to Tier 2, and the verbose execution logs
  are flushed from the prompt window.

### Tier 2: Episodic Memory (The Session & Compaction)

- **Scope:** The current interactive chat session.
- **Content:** The history of user requests and the final synthesized answers
  from agents.
- **Management (Memory Compaction):** This directly addresses an Epic #29
  initiative. When Tier 2 exceeds a token saturation threshold (e.g., 50k
  tokens), an asynchronous background `Compactor Agent` processes the oldest
  80% of the history. It synthesizes the sprawling conversation into dense,
  chronological bullet points (e.g., *"User asked for X. Swarm built Y and
  encountered bug Z, which was fixed."*). The raw text is dropped, drastically
  reducing token usage while preserving the conversational narrative.

### Tier 3: Semantic Memory (The Project)

- **Scope:** The local repository/workspace (`.gemini/state/`).
- **Content:** Explicit architectural constraints, pinned context files, and
  **learned operational facts**.
- **Management (The State Tools):** We will implement a robust key-value store
  accessible via the `read_state` and `write_state` tools.
- **Replacing Design 34 (Tool Reliability):** Instead of a hardcoded SQLite
  table for tool failures, Overwatch or specialized agents will simply use
  `write_state`. If `codex` fails 3 times due to credit limits, Overwatch
  writes to Tier 3:
  `{"key": "tool_health:codex", "value": "OFFLINE: out of credits"}`. The
  routing agent automatically queries `tool_health:*` before planning,
  allowing it to dynamically route around failures using generalized memory
  rather than hardcoded logic.

### Tier 4: Global Memory (The User)

- **Scope:** Across all projects on the machine (`~/.config/swarm/`).
- **Content:** User identity, global API availability, cross-project
  preferences (e.g., "Always use tabs").
- **Management:** Managed by global config and the `/remember` command,
  feeding into the foundational system prompt.

## 3. Impact on Existing Designs

This architectural shift **supersedes and deprecates Design Doc 34 (Tool
Reliability Scoring)**.

By treating a "broken tool" simply as a semantic fact stored in Tier 3/4
memory, we avoid building highly specific database schemas. An agent recording
*"The test command here is `npm run test:ci`"* utilizes the exact same
retrieval infrastructure as Overwatch recording *"The `claude-code` tool is
currently rate-limited."*

## 4. Implementation Strategy

1. **Phase 1 (Semantic KV Store):** Fully implement the `read_state` and
   `write_state` tools in the SDK (`pkg/sdk/swarm.go`) backed by a lightweight
   JSON or SQLite key-value store scoped to the project workspace.
1. **Phase 2 (Overwatch Integration):** Update the Heuristic Deadlock detector
   to automatically commit a `write_state` payload when a tool loop is
   detected, and update the router prompt to read tool health states before
   delegating.
1. **Phase 3 (Memory Compaction):** Implement the asynchronous
   `Compactor Agent` to monitor the SQLite session history size and replace
   old events with synthesized summary events.
