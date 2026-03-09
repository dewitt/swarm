# Swarm v0.06 Release Notes

We are excited to announce the **v0.06** release of Swarm. This release
establishes the foundational pillars of our "Self-Healing Swarm" architecture,
introduces native semantic codebase mapping, drastically improves the core
stability of the SDK, and brings Swarm into complete compliance with native OS
directory standards.

## Core Features and Fixes Delivered in v0.06

### 1. Native Semantic Orchestration (LSP & Tree-sitter)

Swarm agents no longer rely on brittle `grep` searches for codebase discovery.
We have introduced a dual-engine semantic architecture:

- **Tree-sitter Integration:** Agents can now instantly extract high-level
  structural skeletons (structs, interfaces, methods) of large files without
  blowing out their context window with implementation details.
- **Model Context Protocol (MCP) Bridge:** We integrated the official `mcp-go`
  SDK, allowing Swarm to orchestrate real Language Servers (like `gopls` or
  `pyright`) as detached background daemons.
- **High-Yield Abstractions:** Agents are now equipped with abstracted,
  type-safe tools (`analyze_impact`, `get_api_signature`, `validate_code`,
  `rename_symbol`) to execute complex refactors deterministically.

### 2. The Self-Healing Ecosystem

We implemented several critical mechanisms from Epic #29 to allow the Swarm to
dynamically recover from errors:

- **Asynchronous HITL Interception:** The engine now intercepts and parses
  sub-agent output. If an agent explicitly asks the user a question or hits a
  blocking failure, the Swarm will forcibly halt any replanning loops and
  gracefully bubble the prompt up to the user.
- **Automated Post-Incident Artifacts:** When the Swarm hits a fatal recursive
  loop, it now automatically generates a localized `incident_report.md`
  containing the full JSON trajectory dump for post-mortem analysis.
- **Dynamic Skill Reloading:** The orchestrator now performs thread-safe
  hot-reloading of the `skills/` directory mid-session. When the
  `skill_builder_agent` creates a new skill, the Swarm instantly ingests and
  routes to it.
- **Sysadmin Agent:** A new specialized environment manager that safely
  auto-diagnoses the host OS and installs missing dependencies (via Homebrew,
  APT, npm) into localized environments.

### 3. OS Compliance and Configuration Overhaul

- **XDG Base Directory Support:** Swarm completely abandons hardcoded
  dotfiles. It now uses the OS-native user config directory (e.g.,
  `~/Library/Application Support/swarm`, `%AppData%`, or `~/.config/swarm`)
  for global preferences.
- **System-Wide Skills:** Skills can now be installed globally via system
  package managers into `/usr/share/swarm/skills`. The CLI dynamically
  discovers them while still respecting local project overrides.
- **Embedded Zero-Config Installs:** The core foundational skills
  (`swarm_agent`, `input_agent`) are now compiled directly into the binary
  using `//go:embed`. This guarantees that running
  `go install github.com/dewitt/swarm/cmd/swarm@latest` will yield a perfectly
  functioning CLI from any directory on your computer without manual
  configuration.
- **`.swarm` Migration:** The project-local config directory has been
  officially renamed from `.gemini` to `.swarm`.

### 4. Quality of Life & Refactoring

- **SDK Modularization:** Dismantled the massive 1,800-line `swarm.go` God
  Object into six tightly scoped, domain-specific behavioral files
  (`swarm_plan.go`, `swarm_reflect.go`, etc.), massively improving agentic
  navigation and reducing merge conflicts.
- **Semantic Forgetting (`/forget`):** Added a new CLI command enabling users
  to surgically purge hallucinated or poisoned facts from the persistent
  `state.db` database.
- **UI & Noise Reduction:** Autocomplete suggestion boxes now render as true
  visual overlays rather than disruptive layout shifts. Single-shot prompt
  mode is now completely silent by default, hiding intermediate routing steps
  unless `--verbose` is passed.
- **Naked Crash Prevention:** Gracefully intercepted missing `GOOGLE_API_KEY`
  panics, ensuring the terminal doesn't leak raw struct dumps or `lipgloss`
  ANSI escape artifacts upon early exit.
- **Asynchronous Dashboards:** The `/memory` and `/sessions` TUI commands have
  been rewritten to execute asynchronously, ensuring the UI never blocks while
  performing massive SQLite operations.

______________________________________________________________________

# Swarm v0.05 Release Notes

We are thrilled to announce the **v0.05** release of Swarm. This release
introduces a paradigm-shifting **4-Tier Hierarchical Memory** architecture
designed to permanently solve "context rot" and token exhaustion during
long-horizon, massively multi-agent workflows. It also includes comprehensive
stability, concurrency, and performance improvements across the entire
codebase.

## Core Features and Fixes Delivered in v0.05

### 1. The 4-Tier Hierarchical Memory System

We have completely redesigned the Swarm memory engine to mimic the paging
logic of modern operating systems, separating state into four distinct,
rigorously typed tiers:

- **Working Memory (Tier 1):** The ephemeral execution state. We introduced a
  new **Passive Episodic Pruning** algorithm. Massive tool outputs (like
  15,000-line `grep` logs) are now automatically truncated from the context
  window once their semantic value has been extracted, keeping the agent's
  prompt fast, cheap, and highly focused.
- **Episodic Memory (Tier 2):** A high-fidelity, chronological audit log of
  all interactions and LLM responses, stored safely in the underlying session
  database.
- **Semantic Memory (Tier 3):** An embedded SQLite database utilizing the
  `FTS5` extension. We implemented **Passive Reflective Extraction**: the
  orchestrator now monitors the execution graph in the background and
  automatically extracts "timeless facts" (e.g., project-specific build
  commands, API keys, hidden paths) without requiring agents to explicitly
  call a `commit_fact` tool. These facts are seamlessly injected into the
  active prompt of all agents during the routing phase.
- **Global Memory (Tier 4):** Centralized tracking of foundational
  instructions, combining pinned context files (`@`), loaded `SKILL.md`
  documents, and user preferences (`/remember`).

### 2. Memory Observability and State Interception

- **Enhanced `/memory` Command:** The `/memory` TUI command now renders a
  dynamic, right-aligned table providing real-time token footprint estimates
  and entity counts for all four memory tiers.
- **Input Agent as a Memory Interceptor:** The routing logic has been upgraded
  so that the `input_agent` actively reads Semantic Memory. If a user asks a
  question that is already stored as a known fact, the Input Agent bypasses
  specialized search/codebase tools entirely, allowing the orchestrator to
  instantly answer from cache.

### 3. Comprehensive Concurrency & Reliability Audit

We executed a rigorous, top-to-bottom architectural audit against our
`CODE_REVIEW_GUIDE.md` standards, remediating several critical systemic
issues:

- **TUI Async Initialization:** Eliminated synchronous SQLite locks that were
  blocking the main thread during startup. Swarm now boots asynchronously via
  Bubble Tea `tea.Cmd`, rendering instantly.
- **Orphaned Goroutine Leaks:** Fixed a silent file-descriptor and goroutine
  leak in the `bashExecuteTool` telemetry stream where background processes
  would outlive their observer channels.
- **Sub-process Reaping:** Shell commands executed via the CLI now strictly
  enforce UNIX Process Group ID (PGID) cancellation. If a user interrupts
  (`^C`) a long-running test watcher, Swarm securely tears down the entire
  process tree, not just the parent bash shell.
- **Robust Telemetry Backoff:** The background LLM Observer loop now enforces
  mutex-locked interval throttling and actively broadcasts transient LLM
  provider failures to the UI, preventing silent background crashes or API
  rate-limit exhaustion.

______________________________________________________________________

# Swarm v0.04 Release Notes

We are excited to announce the **v0.04** release of Swarm. This release
represents a massive leap forward in both the internal architecture and the
user experience, focusing heavily on TUI performance, background process
management, and the establishment of rigorous, persona-driven review
frameworks.

## Core Features and Fixes Delivered in v0.04

### 1. Advanced UI Rendering (Bubble Tea v2 Migration)

- **Architectural Upgrade:** Fully migrated the Terminal User Interface to
  Charm's `Bubble Tea 2.0` and `Lip Gloss v2`.
- **Dynamic Responsive Layouts:** Rewrote the TUI rendering engine to leverage
  declarative views and native terminal sizing. Eliminated several jitter bugs
  (e.g., the Agent Panel's 1-line layout jump) by explicitly disabling
  implicit word wraps and properly padding components via
  `lipgloss.PlaceHorizontal`.
- **Adaptive Theme Support:** The TUI now actively polls the terminal
  background color on launch and dynamically adjusts its entire color palette,
  ensuring the `glamour` markdown renderer and custom widgets look incredible
  on both Light and Dark terminal themes.
- **Viewport Polish:** Added a dynamic, non-intrusive scrollbar to the primary
  chat interface that correctly reflows content when active, without causing
  horizontal layout explosions.

### 2. The "Web Agent Panel" Enhancements

- **Live Web Dashboard:** Officially shipped the `http://localhost:5050` Web
  Agent Panel, providing a rich, graphical, Server-Sent Events (SSE) driven
  dashboard of your swarm's execution tree.
- **Graceful Teardown:** Fixed an architectural deadlock where active SSE
  browser connections would prevent the Swarm CLI from shutting down when the
  user hit `Ctrl+C`.
- **Status Accuracy:** Resolved an issue where agents would indefinitely
  display "Processing..." on the web after task completion by plumbing
  `FinalContent` payloads through the `ObservableEvent` stream.

### 3. Native Background Process Management

- **Detached Server Support:** Upgraded the core `bash_execute` tool with a
  new `is_background: true` boolean flag. Agents can now boot local
  development servers (e.g., `npm run dev`) natively.
- **Process Group Sandboxing:** Swarm now isolates these background tasks into
  standalone UNIX Process Groups (PGIDs) and dynamically returns the PGID to
  the agent.
- **Automatic Orphan Cleanup:** The Swarm engine tracks all detached PGIDs and
  automatically broadcasts a `SIGKILL` to tear down stray development servers
  when the interactive CLI session ends.

### 4. Persona-Driven AI Review Frameworks

- We've introduced a trio of specialized guides in the `docs/` folder,
  engineered to allow AI agents to independently perform hyper-focused audits
  of the repository, logging their findings into tracked `ISSUES.md` backlogs:
  - `CODE_REVIEW_GUIDE.md`: The Architect's lens for rooting out synchronous
    UI-blocking I/O, dead code, and unhandled errors. (This directly led to
    the elimination of critical SQLite file locks during testing!).
  - `UX_REVIEW_GUIDE.md`: The UX Reviewer's lens for identifying onboarding
    friction, confusing error dumps, and aesthetic inconsistencies.
  - `AGENTIC_QUALITY_REVIEW_GUIDE.md`: The ML Expert's lens for evaluating
    prompt efficacy, agent cognitive loops, and LLM-as-a-judge rubric
    strictness.

______________________________________________________________________

# Swarm v0.03 Release Notes

We are thrilled to announce the **v0.03** release of Swarm. This release
focuses on upgrading the runtime into an "Air Traffic Control" experience for
managing autonomous agents, and heavily reinforces the engine routing
architecture to guarantee reliable, autonomous code modifications.

## Core Features Delivered in v0.03

### 1. The Agent Panel & Observability API

- **Agent Panel UI**: Shipped a real-time Bubble Tea TUI dashboard displaying
  all executing agents, their tool usage, and granular state transitions
  (Thinking, Executing, Waiting, Complete, Error).
- **Observability API**: Deprecated legacy `ChatEvent` primitive strings in
  favor of a strongly-typed `ObservableEvent` struct, plumbing fully parsed
  `genai.FunctionCall` metadata and payloads directly to the interactive
  clients.
- **Semantic Observer**: Added an asynchronous `gemini-2.5-flash-8b`
  background loop inside `executeSpan` that digests raw stdout telemetry and
  tool payloads, outputting concise, human-readable semantic intents (e.g.,
  `💡 Running unit tests...`) dynamically to the Agent Panel UI.

### 2. Autonomous Remediation (Stateful Routing Engine)

- **Bounded Reflection Loop**: Solved the "fire and forget" limitations of the
  previous execution graph. The core `Chat()` execution block was refactored
  into a bounded 5-cycle loop.
- **Reflect Phase**: Integrated a new `Reflect()` method that heavily
  evaluates the trajectory progression post-execution, preventing the agent
  from concluding a task until a physical verification (like
  `write_local_file`) has successfully resolved the user's initial prompt.

### 3. Agentic E2E Test Suite

- Reached an **80% passing rate** on a rigorous 6-scenario LLM-as-a-judge test
  suite evaluating fully autonomous filesystem modification capabilities.
- Swarm can now reliably investigate bugs, read source code, and dynamically
  patch logic and tests without human intervention.

______________________________________________________________________

# Swarm v0.02 Release Notes

We are excited to announce the **v0.02** release of Swarm. This release
focuses on resolving critical stability bottlenecks under high-concurrency
loads, refining the CLI's interactivity and UX, and completing our vision for
dynamic execution graph rendering.

## Core Features and Fixes Delivered in v0.02

### 1. Massive Concurrency & Stability Improvements

- **SQLite Contention Eliminated:** Resolved `database is locked` panics
  during large-scale fan-outs (e.g. 10+ agents executing in parallel) by
  isolating sub-span session runners into their own lightweight, in-memory
  state.
- **Robust HTTP Scaling:** Patched `GOAWAY` stream concurrency disconnects and
  premature deadline drops when calling Google GenAI APIs by falling back to
  robust HTTP/1.1 connection pools and intelligently expanding
  `ResponseHeaderTimeout` and context timeout intervals.

### 2. The Living Graph Interface

- **Dynamic Relationship Mapping:** The Agent Panel now visually wires
  hierarchical dependencies between agents, rendering real-time parent-child
  tree layouts for multi-agent workflows.

### 3. Polish and Quality of Life

- **Autocomplete Enter-Key Bypass:** Fixed a UX friction point where
  `/<command>` inputs trapped users in autocomplete suggestions, requiring a
  double `Enter` press to execute.
- **Terminal Escaped Byte Scrubbing:** Introduced buffer sanitizers to prevent
  stray SGR mouse sequence bytes (`[<65...`) from bleeding into the chat
  prompt during aggressive scrolling.
- **Enriched Session History:** The `/sessions` command now extracts and
  previews up to 80 characters of the user's primary intent, rendering a much
  clearer ledger of past work.
  - **Error Line Wrapping:** Fatal application errors now natively respect the
    active terminal's layout width and cleanly wrap text instead of truncating
    into the void.

### 4. Code Hygiene and Structural Cleanup

- **SDK Path Resolution:** Deprecated manual un-guarded `~/.config/swarm`
  string concatenations in favor of a unified `sdk.GetConfigDir()` helper,
  standardizing SQLite and trajectory storage safely.
- **Terminology Alignment:** Executed a global terminology pass to solidify
  the "Swarm Operator" paradigm over legacy "Engineering Manager" metaphors in
  all markdown and system prompts. Stripped unnecessary "scare quotes" from
  core concepts (Zero-HITL, Agent Cards).
- **Static Analysis & Linters:** Performed a deep codebase audit using strict
  `staticcheck` and `golangci-lint` passes. Resolved leaking regex literals,
  dead styles, and unused local UI state variables.
- **Test Sandbox Safety:** Replaced dirty `os.Setenv` test configurations with
  `t.Setenv(k, v)` to guarantee parallel execution safety across the core SDK
  layers.

______________________________________________________________________

# Swarm v0.01 Release Notes

We are thrilled to announce the inaugural **v0.01** release of Swarm.

This release marks the completion of the foundational architecture. Swarm
provides a hyper-extensible, framework-agnostic "Swarm Operator" control plane
for managing an army of autonomous AI agents.

## Core Features Delivered in v0.01

### 1. The Core SDK and TUI Separation

- A clean, embeddable `pkg/sdk` module completely decoupled from the
  presentation layer.
- An asynchronous, robust `ObservableEvent` pipeline stream that guarantees
  the CLI interface never blocks while agents are executing or planning.
- Persistent local SQLite session management with `/sessions` and `/rewind`
  capabilities.

### 2. High-Fidelity Agent Observability (The Agent Panel)

- **Live Agent Cards:** A real-time, multiplexed Bubble Tea interface
  (`cmd/swarm/`) that organically visually scales to display concurrent agent
  processes working in parallel.
- **The Semantic Observer (Observe Mode):** The UI intelligently intercepts
  raw `stdout` and bash telemetry execution streams, utilizing background
  flash models (`gemini-2.5-flash-8b`) to automatically translate granular
  technical actions into concise, human-readable intents (e.g. replacing
  `bash_execute: find . -name "*.go"` with "Scanning the filesystem for Go
  files...").

### 3. Dynamic Skills Architecture

- "Thin Software, Fat Models": Replaced rigid, hardcoded Go implementations
  with dynamic, natively compiled Markdown behavior instructions.
- Support for open `SKILL.md` configurations inside custom `.skills/`
  directories, empowering the `Swarm Agent` and the user to continuously
  redefine, scaffold, and share the very agents that make up the system.

### 4. Robust UX Workflows

- **Seamless Shell Mode (`!`):** Drop natively into the system shell, execute
  bash workflows seamlessly, and bounce back to your AI operator console
  without ever leaving the TUI.
- **Context Pinning (`@`):** Fuzzy-filter your entire local file system to
  aggressively pin explicit codebase context into the Swarm's active memory
  pool.
- **Read-Only Plan Mode (`--plan`):** Securely brainstorm architecture
  decisions where your agents are physically sandboxed from destructive
  filesystem tools.

## Known Limitations and Future Work (v0.03)

- **Advanced GitOps:** Native CLI-driven scaffoldings for complex GitHub
  Actions pipelines are rudimentary.
- **Multi-Agent Supervision:** The core `Swarm Agent` currently fulfills most
  tasks iteratively. Formal recursive delegation (Architect -> Coder ->
  Tester) paths via deterministic execution graphs remain under development.
- **Cross-Repository Execution:** The CLI is currently bound to the immediate
  `.git` repository it is spawned within.

______________________________________________________________________

*The Swarm Authors (2026)*
