# Project TODOs & Technical Debt

This document tracks recurring tasks, pending migrations, and known technical
debt that should be addressed as the project evolves.

## Pending Migrations

- [ ] **Migrate to Native ADK Skills**: Currently, the `swarm` CLI implements
  its own custom Skill loader (`pkg/sdk/skill.go`) because the Go ADK does not
  yet natively support Skills. Once the official ADK
  `google.golang.org/adk/skill` (or equivalent) package is released, we must
  deprecate our custom loader and migrate entirely to the ADK implementation
  to maintain our "Thin Software" philosophy.

## Feature Backlog

- [x] **Global Configuration**: Implement the `swarm config` command to save
  global user preferences (like API keys or preferred text editor) to a
  generic `~/.config/swarm/config.yaml` file.

- [x] **Advanced Session Management**: Implement the ability to persist,
  suspend, and resume interactive TUI sessions. This includes the ability to
  `/rewind` the conversation history to undo mistakes or branch conversations.

- [x] **Third-Party Tool Orchestration**: Develop Skills/Sub-agents that wrap
  external AI CLIs (e.g., `gemini-cli`, `claude-code`) via `bash_execute` so
  the Swarm CLI can act as a master engine for other complex systems.

- [x] **Input Agent Oversight** (#25): Implemented as a failsafe for the human-in-the-loop.
  to detect intent shifts and handle transitions.
  See `docs/design/16-chat-input-agent.md`.

- [x] **Agent Panel & Cards**: Implement a dynamic, event-driven visualization of
  the agent swarm with ephemeral lifecycles and responsive fidelity levels.
  See `docs/design/17-agent-cards-and-panel.md`.

- [ ] **Agent Panel Interactivity**: Implement mouse support for Agent Cards,
  allowing users to click cards to "drill down" into logs or micro-steer
  individual agents.

- [ ] **Dynamic Relationship Mapping**: Visualize the execution graph by
  drawing dependency lines between parent and child Agent Cards in the panel.

- [ ] **Cross-Language SDK Bindings**: Migrate the `pkg/sdk` interfaces to a
  strict Protobuf definition (`/proto`) and implement compilation targets for
  C-Shared Libraries (FFI) and WebAssembly to support native Python,
  TypeScript, and Rust wrappers. See `docs/design/09-cross-language-sdk.md`.

- [x] **Codex Agent** (#20): Add a Codex agent wrapper to delegate to Codex
  CLI.

- [x] **Observe Mode** (#18): Add a `^O` command for observe-mode.

- [x] **Async Interactions** (#19): Implement input queueing, graceful
  interruptions (`Ctrl+C` / `Esc`), and async HITL interactions.

- [x] **Dynamic Loading State** (#7): Long-running subagent and tool
  invocations need a dynamic loading state.

## Known Bugs

- [x] **Dead-end Tool Calls** (#17): Subagent tool calls can reach a dead end.
- [x] **Status Overwrites Name** (#15): Tool and subagent invocation status
  currently overwrites the agent name in the UI.
- [ ] **Escape Sequences in Prompt** (#3): Strange escape sequences appear in
  the input prompt when scrolling.
