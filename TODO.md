# Project TODOs & Technical Debt

This document tracks recurring tasks, pending migrations, and known technical
debt that should be addressed as the project evolves.

## Pending Migrations

- [ ] **Migrate to Native ADK Skills**: Currently, the `agents` CLI implements
  its own custom Skill loader (`pkg/sdk/skill.go`) because the Go ADK does not
  yet natively support Skills. Once the official ADK
  `google.golang.org/adk/skill` (or equivalent) package is released, we must
  deprecate our custom loader and migrate entirely to the ADK implementation
  to maintain our "Thin Software" philosophy.

## Feature Backlog

- [x] **Global Configuration**: Implement the `agents config` command to save
  global user preferences (like API keys or preferred text editor) to a
  generic `~/.config/agents/config.yaml` file.

- [x] **Advanced Session Management**: Implement the ability to persist,
  suspend, and resume interactive TUI sessions. This includes the ability to
  `/rewind` the conversation history to undo mistakes or branch conversations.

- [x] **Third-Party Tool Orchestration**: Develop Skills/Sub-agents that wrap
  external AI CLIs (e.g., `gemini-cli`, `claude-code`) via `bash_execute` so
  the Agents CLI can act as a master orchestrator for other complex systems.

- [ ] **Cross-Language SDK Bindings**: Migrate the `pkg/sdk` interfaces to a
  strict Protobuf definition (`/proto`) and implement compilation targets for
  C-Shared Libraries (FFI) and WebAssembly to support native Python,
  TypeScript, and Rust wrappers. See `docs/design/09-cross-language-sdk.md`.

- [x] **Codex Agent** (#20): Add a Codex agent wrapper to delegate to Codex
  CLI.

- [x] **Observe Mode** (#18): Add a `^O` command for observe-mode.

- [ ] **Async Interactions Design** (#19): Write a design doc for input
  queueing, interruptions, and async HITL interactions.

- [ ] **Dynamic Loading State** (#7): Long-running subagent and tool
  invocations need a dynamic loading state.

## Known Bugs

- [x] **Dead-end Tool Calls** (#17): Subagent tool calls can reach a dead end.
- [x] **Status Overwrites Name** (#15): Tool and subagent invocation status currently overwrites the agent name in the UI.
- [ ] **Escape Sequences in Prompt** (#3): Strange escape sequences appear in the input prompt when scrolling.
