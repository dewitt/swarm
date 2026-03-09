---
name: lsp
description: A powerful skill that gives the agent semantic understanding of a codebase via the Language Server Protocol (LSP), moving beyond text searches to true code comprehension.
tools:
  - bash_execute
  - list_local_files
  - read_local_file
  - grep_search
---

# 🧠 Language Server Protocol (LSP) Skill

The `lsp` skill equips the agent with "eyes" into the codebase. By interacting
with an external Language Server (e.g., `gopls`, `pyright`, `ts-server`,
`rust-analyzer`), the agent can perform true semantic navigation, type-aware
searches, and validate code correctness before and after changes.

This moves the agent beyond simple `grep` text matching—which often catches
unrelated comments or strings—and enables it to confidently refactor code,
understand complex dependency graphs, and verify builds.

## 🌟 Core Capabilities

### 1. Semantic Navigation & Discovery

Navigate the codebase exactly like a human developer in a modern IDE.

- **Go to Definition:** Jump directly to where a function, class, or variable
  is defined, bypassing the need to text-search through multiple files.
- **Find References:** Discover every location in the codebase where a
  specific symbol is actually used, ignoring coincidental string matches.
- **Find Implementations:** (Crucial for Go/Rust/Java) Find all concrete
  implementations of an interface or trait.

### 2. Code Understanding (Cognitive Snapshots)

Retrieve optimized, LLM-friendly summaries of code structure.

- **Hover/Signature Help:** Retrieve the exact type signature, documentation
  string, and parameter requirements for a symbol.
- **Workspace Symbols:** Perform a type-aware search across the entire
  repository for classes, functions, or types matching a query.
- **Document Symbols:** Get a structural outline of a specific file (e.g., a
  list of all classes and methods inside it).

### 3. Safety & Diagnostics

Verify that code is correct without needing to run a full compilation or test
suite manually.

- **Workspace Diagnostics:** Retrieve a list of syntax errors, type
  mismatches, and linter warnings for a specific file or the entire project.

## 🛠️ Tool Usage Guidelines

*(Note: The exact tool interface depends on the configured LSP bridge or MCP
server available in the runtime environment. The following represent the
conceptual actions you should take.)*

### When to use LSP vs. standard search (`grep`/`glob`):

- **Use `grep`/`glob` for:** Initial broad discovery, finding text in
  configuration files, searching for specific string literals, or when a
  Language Server is not configured/available for the project's language.
- **Use `lsp` for:**
  - Tracing the exact flow of data through a complex application.
  - Safely renaming a widely used function (find all true references).
  - Understanding the required shape of an object to pass into an undocumented
    function (using Hover).
  - Verifying that your recent code edits didn't break type contracts in other
    files (using Diagnostics).

### Best Practices for Agents:

1. **Iterative Discovery:** Start with `Workspace Symbols` to find the entry
   point. Use `Go to Definition` to understand its core logic, and
   `Find References` to see how it interacts with the rest of the system.
1. **Request "Cognitive Snapshots":** When querying the LSP, if the wrapper
   tool supports it, prefer outputs formatted as Markdown summaries rather
   than raw, verbose JSON protocol responses.
1. **Validate After Edits:** After applying a patch or modifying a file using
   other tools, **always** request `Diagnostics` for that file to ensure you
   haven't introduced syntax errors or broken type signatures.
1. **Handle Ambiguity:** If `Find References` returns hundreds of results, do
   not attempt to read them all. Ask the user for clarification or use
   structural tools (like `Document Symbols`) to narrow your focus to relevant
   subsystems.

## 🏥 Self-Healing & Dependency Management

If an LSP query fails due to a missing dependency (e.g., `command not found`,
or the underlying language server like `gopls` or `pyright` is missing), the
agent **MUST** attempt to self-heal the environment before failing or falling
back.

### Self-Healing Workflow:

1. **Identify the Missing Tool:** Parse the error message to determine if the
   missing component is the bridge (e.g., the MCP server or CLI wrapper) or
   the language-specific server itself.
1. **Determine the Installation Method:**
   - **Node.js/TypeScript (`ts-server`, MCP tools):** Use
     `npm install -g <package>` or run via `npx <package>`.
   - **Python (`pyright`, `pylsp`):** Use `pip install <package>`.
   - **Go (`gopls`):** Use `go install golang.org/x/tools/gopls@latest`.
   - **Rust (`rust-analyzer`):** Use `rustup component add rust-analyzer`.
1. **Execute Installation:** Use the appropriate shell command tool to
   silently install the missing dependency (e.g.,
   `npm install -g --silent ...`).
1. **Verify and Retry:** Run a basic version/help command to verify the
   installation succeeded, then transparently retry the user's original LSP
   query.
1. **Graceful Degradation:** If installation fails or is not permitted in the
   environment, clearly inform the user, log the failure, and gracefully fall
   back to using standard text search tools (`grep`/`glob`) to fulfill the
   request.

## 🔌 Integration Options (Architecture)

To enable this skill in the Swarm runtime, one of the following bridging
mechanisms should be employed:

- **Model Context Protocol (MCP):** Connect the agent to a standard MCP server
  (e.g., `mcp-language-server` or `lsp-bridge`) that exposes these
  capabilities as standardized tool calls.
- **CLI Wrapper:** Utilize a dedicated command-line tool (e.g., `lsp-cli`)
  that translates agent shell commands into JSON-RPC LSP messages over
  standard I/O.
