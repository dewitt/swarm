# Design Doc: Native Semantic Orchestration (LSP & Tree-sitter)

**Status:** Proposed **Author:** Gemini CLI Swarm **Date:** March 2026

## Objective

To implement a robust, non-blocking semantic understanding layer within the
`swarm` SDK, enabling agents to navigate codebases using Language Server
Protocol (LSP) and Tree-sitter. This will replace brittle `grep`-based
discovery with deterministic, type-aware analysis, solving the "infinite hang"
deadlock experienced during raw shell execution of language servers.

## 1. Problem Statement

Current Swarm agents rely heavily on `grep_search` and `read_local_file`. When
tasked with complex refactoring or understanding large cross-file
dependencies, these tools are insufficient.

Our recent attempt to use `gopls` directly via `bash_execute` resulted in a
system-level deadlock. This is a well-documented failure mode where the
agent's execution loop blocks on the interactive JSON-RPC stdin/stdout stream
of the language server daemon.

## 2. Proposed Dual-Engine Architecture

Following the industry standard identified in recent research, we will
implement a hybrid architecture:

### A. Tree-sitter (Fast, Local Structural Parsing)

- **Use Case:** Instant codebase mapping, semantic chunking for RAG, and
  extracting function skeletons without needing a functional build
  environment.
- **Implementation:** Integrate `tree-sitter` bindings directly into the Go
  SDK.
- **New Tool:** `get_code_skeleton` – Returns a structural map of a file or
  directory (classes, methods, signatures) without implementation bodies.

### B. LSP via MCP (Deep, Dynamic Semantic Analysis)

- **Use Case:** Deterministic "Go to Definition," "Find References," "Symbol
  Rename," and real-time type diagnostics.
- **Implementation:** Abandon custom JSON-RPC wrappers. Integrate the official
  **Model Context Protocol (MCP)** SDK as a managed intermediary.
- **Bridging:** Swarm will spawn language servers (like `gopls` or `pyright`)
  in a detached daemon mode with network listening flags (HTTP/SSE) to
  completely sidestep pipe-based deadlocks.
- **Lifecycle:** The SDK will manage the server lifecycle (handshake,
  initialization, and shutdown) independently of the agent's reasoning loop.

## 3. High-Yield Function Abstractions

We will not expose raw LSP JSON-RPC to the LLM. Instead, we will register
abstracted, intent-driven tools:

| Abstracted LLM Tool | Underlying Action | Purpose | | :--- | :--- | :--- | |
`analyze_impact` | `textDocument/references` | Determine the "blast radius" of
a change. | | `get_api_signature` | `hover` / `signatureHelp` | Understand how
to call a function/class. | | `validate_code` | `publishDiagnostics` |
Real-time grounding and recursive self-correction. | | `rename_symbol` |
`textDocument/rename` | Safe, project-wide refactoring. |

## 4. Context Window & Large Payload Management

To prevent "context rot" from voluminous LSP responses (e.g., finding 500
references):

1. **Indirect Discovery:** If a payload exceeds 2,000 tokens, the orchestrator
   will write the raw result to a hidden temporary file (`.swarm/tmp/...`) and
   return a synthesized summary to the agent with instructions on how to
   paginate through the full file.
1. **Graph-Based Ranking:** Implement a basic PageRank-style algorithm to
   prioritize the most "architecturally significant" files in the repository
   map.

## 5. Implementation Roadmap

1. **Phase 1: MCP Client Integration:** Add the Go MCP SDK dependency and
   implement a `ManagedLSP` struct to handle server lifecycles.
1. **Phase 2: The "Overwatch" Safety:** Implement hard caps on sequential LSP
   calls and timeouts to prevent recursive hallucination loops.
1. **Phase 3: Autonomous Provisioning:** Enable the SDK to detect missing
   binaries and autonomously run `go install` or `npm install` to provision
   the required environment.
1. **Phase 4: Agentic Eval:** Update `scenario_7_lsp` to verify that agents
   actively prefer these new tools over primitive `grep`.

## 6. Philosophy Alignment

- **Defer to the Frontier:** By providing "eyes," we allow the model to use
  its inherent reasoning rather than our custom logic.
- **Zero-HITL Verification:** Native diagnostics (`validate_code`) allow
  agents to verify their own fixes autonomously.
- **World-Class CLI UX:** Faster, more accurate refactoring makes the tool
  feel "magical."
