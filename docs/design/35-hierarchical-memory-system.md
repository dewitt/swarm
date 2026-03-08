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
forgotten between sessions or require rigid, hardcoded database schemas.

**The Vision:** Guided by SOTA research in agentic memory architectures, we
are implementing a **Four-Tier Hierarchical Memory System**. This system will
shift Swarm from passive, linear context accumulation to dynamic, agent-driven
**Active Pruning** and highly localized semantic retrieval.

## 2. Architecture: The 4-Tier Memory Model

We propose categorizing all state into four temporal tiers. The Swarm Engine
will orchestrate how information flows between them.

### Tier 1: Working Memory (Execution State)

- **Scope:** The current active execution cycle
  (`Plan -> Execute -> Reflect`).
- **Content:** The immediate execution graph and stdout from the *current*
  tool invocations.
- **Management (Context Isolation):** To prevent "context poisoning,"
  sub-agents (like `codebase-investigator`) must operate in isolated context
  windows. They execute noisy `grep` or `ls` commands in their own sandbox and
  return *only* a dense, synthesized summary to the primary Swarm
  orchestrator's Working Memory.

### Tier 2: Episodic Memory (Chronological Logs)

- **Scope:** The comprehensive audit log of a session.
- **Content:** The raw sequence of executed commands, compilation errors, and
  conversational turns.
- **Management (Active Pruning):** Academic research shows recursive
  background summarization is "mathematically destructive" for engineering, as
  it strips precise syntactical details. Instead, we will implement the
  **"Focus" architecture**. Agents will be granted a `prune_context` tool to
  autonomously delete their own noisy interaction history (e.g., a massive
  bash output) from their immediate context window once they have extracted
  the necessary facts.

### Tier 3: Semantic Memory (Factual Substrates)

- **Scope:** The local repository/workspace (`.gemini/state/`).
- **Content:** Persistent, timeless facts distilled from episodic memory
  (e.g., "The build command is X", or "Claude-code is currently
  rate-limited").
- **Management (SQLite + FTS5):** We reject the "Polyglot" approach of
  external vector databases as too heavy for a CLI. Instead, we will use a
  unified, embedded **SQLite database**. We will leverage the `FTS5` extension
  for lightning-fast lexical search (exact keyword matching), which is often
  superior to fuzzy cosine similarity for strict syntactical coding tasks.
  Agents will use `commit_fact` and `retrieve_fact` tools to query this
  embedded pipeline.

### Tier 4: Global Memory (Foundational Parameters)

- **Scope:** Across all projects on the machine (`~/.config/swarm/`).
- **Content:** User identity, global API availability, cross-project
  preferences (e.g., "Always use tabs").
- **Management:** Managed by global config and the `/remember` command,
  feeding into the foundational system prompt via cascading
  `.gemini/GEMINI.md` file overrides.

## 3. Impact on Existing Designs

This architectural shift **supersedes and deprecates Design Doc 34 (Tool
Reliability Scoring)**.

By treating a "broken tool" simply as a semantic fact stored in Tier 3 memory,
we avoid building highly specific database schemas. An agent recording *"The
test command here is `npm run test:ci`"* utilizes the exact same retrieval
infrastructure as Overwatch recording *"The `claude-code` tool is currently
rate-limited."*

## 4. Implementation Strategy

1. **Phase 1 (Context Isolation):** Refactor the SDK's execution loop so that
   sub-agents do not leak their verbose tool execution history (stdout) back
   into the primary orchestrator's context window. Only their final
   `FinalContent` summary is passed up.
1. **Phase 2 (Semantic SQLite):** Implement the `state` table in SQLite with
   the FTS5 extension. Provide `commit_fact` and `retrieve_fact` tools to the
   core Swarm agents.
1. **Phase 3 (Active Pruning):** Grant agents the `prune_context` tool,
   allowing them to consciously shape their own Tier 1 and Tier 2 memory
   streams.
