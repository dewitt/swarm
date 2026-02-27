# Competitive Analysis: CLI Agent Feature Superset

This document tracks the capabilities of leading AI CLI agents (Gemini CLI, Claude Code, and Codex) to ensure the `agents` project targets a comprehensive superset of modern development features.

## 1. Core Interactions & Interface
- [x] **REPL & Print Modes:** Interactive chat sessions vs. single-shot command line executions (e.g., `claude -p "..."`).
- [x] **Input Piping:** Accepting standard input from other UNIX commands (e.g., `cat logs.txt | agents -p "explain"`).
- [x] **Slash Commands (`/`):** Built-in shortcuts for session management (e.g., `/help`, `/clear`, `/model`, `/plan`, `/act`).
- [ ] **Custom Slash Commands:** Allowing teams to define repository-specific commands (e.g., `/review-pr`) via configuration files.
- [x] **Context Referencing (`@`):** Explicitly injecting files, folders, or web URLs into the context window using the `@` symbol.
- [x] **Direct Shell Execution (`!`):** Allowing the user to quickly run a shell command from within the REPL without switching contexts.

## 2. Context & Memory Management
- [x] **Hierarchical Instructions:** Supporting persistent project-specific guidelines (e.g., `AGENTS.md`, `GEMINI.md`, `CLAUDE.md`) at the workspace or directory level.
- [ ] **Lazy Context Loading:** Conservatively reading files only when necessary rather than ingesting the entire codebase upfront to save tokens (Codex approach).
- [x] **Session State:** Ability to `/clear` history to drop context, or export/share session state with teammates.
- [x] **Global Memory:** Persisting user preferences globally across all workspaces (e.g., "I prefer tabs over spaces") via `/remember`.

## 3. Tool Execution & Code Modification
- [x] **Structured File Operations:** Built-in, optimized tools for `read_local_file`, `list_local_files`, and `grep_search` pattern matching.
- [ ] **Surgical Editing:** Applying minimal diffs or patches rather than rewriting entire files (e.g., Codex's `apply_patch` or Gemini's `replace` tool).
- [x] **Shell-Centric Fallbacks:** Empowering the agent to use standard UNIX tools (`ls`, `cat`, `git`) when specific internal tools don't exist.
- [ ] **Web Fetch & Search:** Ability to search the live web for up-to-date documentation or fetch raw content from URLs.

## 4. Multi-Agent & Extensibility Features
- [x] **Agent Skills:** Procedural, markdown-based guides that teach the agent how to perform specific, complex tasks (`skill_creator_agent`).
- [x] **Subagent Orchestration:** Delegating specific domains (like a Security Audit or Build process) to specialized sub-agents with distinct system prompts.
- [ ] **Model Context Protocol (MCP):** Standardized integration with external tool servers (e.g., connecting to Slack, Jira, or internal APIs).
- [ ] **Automated Hooks:** Running scripts automatically before or after agent actions (e.g., running a linter after every code edit).

## 5. Security & Safety
- [ ] **Granular Permissions:** Prompting the user for approval before executing destructive shell commands or modifying sensitive files.
- [ ] **Checkpointing / Undo:** Automatically snapshotting the git working tree before an agent modifies files, allowing for an instant `/restore` if the agent hallucinates.
- [x] **Read-Only "Plan Mode":** A mode dedicated strictly to architectural design and exploration, explicitly sandboxed from modifying the filesystem.

## Summary of Competitor Strengths
*   **Gemini CLI:** Strong focus on checkpointing/undo, structured high-performance tools (like fast ripgrep), and explicit Plan vs. Act modes.
*   **Claude Code:** Excellent UX with `@` referencing, `!` shell commands, input piping, and seamless MCP integration.
*   **Codex CLI:** Highly shell-centric (relies on bash rather than custom tools), deeply integrates with `AGENTS.md`, and focuses heavily on surgical diffing.
