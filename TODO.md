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

- [ ] **Agent Panel Interactivity**: Implement mouse support for Agent Cards,
  allowing users to click cards to "drill down" into logs or micro-steer
  individual agents.

- [ ] **Cross-Language SDK Bindings**: Migrate the `pkg/sdk` interfaces to a
  strict Protobuf definition (`/proto`) and implement compilation targets for
  C-Shared Libraries (FFI) and WebAssembly to support native Python,
  TypeScript, and Rust wrappers. See `docs/design/09-cross-language-sdk.md`.

## Known Bugs

- None currently identified.
