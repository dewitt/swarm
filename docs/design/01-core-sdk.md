# Core SDK & Extensibility

## Embeddable Core

The core of `swarm` is fundamentally an SDK, written entirely in Go. By
decoupling the business logic from the presentation layer, we ensure that:

1. The terminal UI is just one of many possible consumers.
1. The core can be wrapped in other languages (Python, TypeScript) using `cgo`
   or compiled to WebAssembly (`wasm`) for use in browsers or Node.js.

## Powered by Google ADK

The internal business logic is powered by the Google Agent Development Kit
(ADK) for Go. The core SDK utilizes ADK's `LlmAgent` and tool interfaces to
power the CLI's interactive assistant.

### Internal Agent Topology

Internally, the SDK uses a multi-agent topology to resolve user commands:

- **Router Agent**: Analyzes user input and determines whether the request is
  a local file operation, an agent-building task, or a deployment command.
- **Builder Agent**: Specialized in scaffolding new agent projects across
  different frameworks (ADK, LangGraph).
- **GitOps Agent**: Translates deployment requests into CI/CD configurations
  and Git operations.

## Framework Agnosticism

While our internal agents use ADK, the *target* agents (the swarm the user is
building) can be in any framework. The SDK achieves this via an **Adapter
Pattern**.

### The Agent Manifest

To manage diverse frameworks, the SDK looks for (or generates) an `agent.yaml`
manifest in the user's project:

```yaml
name: support-bot
framework: langgraph
language: python
entrypoint: src/graph.py
```

The SDK uses framework-specific adapters to understand how to build, test, and
deploy based on this manifest.

## Dynamic Extensibility (Skills)

Instead of hardcoding support for every new agent framework or deployment
target, `swarm` uses **Skills**.

- A Skill is a lightweight, dynamically loaded configuration (often just
  Markdown + a tool manifest).
- When a user wants to add swarm capabilities or deploy to a new cloud
  provider, the CLI fetches a Skill that teaches the internal ADK agents how
  to perform that task.
- This defers the "heavy lifting" to the LLM. Rather than writing thousands of
  lines of Go for AWS deployment, a Skill provides the LLM with the necessary
  context and generic CLI tools to achieve it.

## Testing Strategy

The SDK must be rigorously testable.

- **Unit Tests**: Pure Go tests for all internal logic, adapters, and manifest
  parsing.
- **Integration Tests**: Tests that spin up mock LLM endpoints to verify that
  the internal ADK agents correctly interpret tools and user prompts.
- **Cross-Compilation Tests**: Automated CI checks ensuring the SDK
  successfully compiles via `cgo` and `wasm` without breaking dependencies.
