# Swarm v0.01 Release Notes

We are thrilled to announce the inaugural **v0.01** release of Swarm. 

This release marks the completion of the foundational architecture. Swarm provides a hyper-extensible, framework-agnostic "Swarm Operator" control plane for managing an army of autonomous AI agents.

## Core Features Delivered in v0.01

### 1. The Core SDK and TUI Separation
- A clean, embeddable `pkg/sdk` module completely decoupled from the presentation layer.
- An asynchronous, robust `ObservableEvent` pipeline stream that guarantees the CLI interface never blocks while agents are executing or planning.
- Persistent local SQLite session management with `/sessions` and `/rewind` capabilities.

### 2. High-Fidelity Agent Observability (The Agent Panel)
- **Live Agent Cards:** A real-time, multiplexed Bubble Tea interface (`cmd/swarm/`) that organically visually scales to display concurrent agent processes working in parallel.
- **The Semantic Observer (Observe Mode):** The UI intelligently intercepts raw `stdout` and bash telemetry execution streams, utilizing background flash models (`gemini-2.5-flash-8b`) to automatically translate granular technical actions into concise, human-readable intents (e.g. replacing `bash_execute: find . -name "*.go"` with "Scanning the filesystem for Go files...").

### 3. Dynamic Skills Architecture
- "Thin Software, Fat Models": Replaced rigid, hardcoded Go implementations with dynamic, natively compiled Markdown behavior instructions.
- Support for open `SKILL.md` configurations inside custom `.skills/` directories, empowering the `Swarm Agent` and the user to continuously redefine, scaffold, and share the very agents that make up the system.

### 4. Robust UX Workflows
- **Seamless Shell Mode (`!`):** Drop natively into the system shell, execute bash workflows seamlessly, and bounce back to your AI operator console without ever leaving the TUI.
- **Context Pinning (`@`):** Fuzzy-filter your entire local file system to aggressively pin explicit codebase context into the Swarm's active memory pool.
- **Read-Only Plan Mode (`--plan`):** Securely brainstorm architecture decisions where your agents are physically sandboxed from destructive filesystem tools.

## Known Limitations and Future Work (v0.02)
- **Advanced GitOps:** Native CLI-driven scaffoldings for complex GitHub Actions pipelines are rudimentary.
- **Multi-Agent Supervision:** The core `Swarm Agent` currently fulfills most tasks iteratively. Formal recursive delegation (Architect -> Coder -> Tester) paths via deterministic execution graphs remain under development.
- **Cross-Repository Execution:** The CLI is currently bound to the immediate `.git` repository it is spawned within.

---
*The Swarm Authors (2024)*
