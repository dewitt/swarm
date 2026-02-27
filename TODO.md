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

- [ ] **Global Configuration**: Implement the `agents config` command to save
  global user preferences (like API keys or preferred text editor) to a
  generic `~/.config/agents/config.yaml` file.

- [ ] **Advanced Session Management**: Implement the ability to persist,
  suspend, and resume interactive TUI sessions. This includes the ability to
  `/rewind` the conversation history to undo mistakes or branch conversations.

- [x] **Third-Party Tool Orchestration**: Develop Skills/Sub-agents that wrap
  external AI CLIs (e.g., `gemini-cli`, `claude-code`) via `bash_execute` so
  the Agents CLI can act as a master orchestrator for other complex systems.

- [ ] **Cross-Language SDK Bindings**: Migrate the `pkg/sdk` interfaces to a
  strict Protobuf definition (`/proto`) and implement compilation targets for
  C-Shared Libraries (FFI) and WebAssembly to support native Python,
  TypeScript, and Rust wrappers. See `docs/design/09-cross-language-sdk.md`.
