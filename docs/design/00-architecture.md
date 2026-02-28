# Architecture Overview

## Introduction

The `swarm` project is a command-line interface (CLI) and embeddable SDK for
managing, building, and deploying AI agents. It embraces a highly modular,
lightweight, and GitOps-first philosophy, delegating complex orchestration
logic to large language models (LLMs) rather than hardcoded implementations.

## High-Level Goals

1. **User Experience First**: Provide an out-of-the-box experience similar to
   Gemini CLI or Claude Code, installable simply via `brew install swarm`.
1. **Framework Agnosticism**: While the tool's core logic is built on the
   Google Agent Development Kit (ADK), it orchestrates and manages agents
   written in any framework (ADK, LangChain, LangGraph, custom frameworks).
1. **Clean Separation of Concerns**: Decouple the Terminal UI from the core
   business logic. The core should serve as an embeddable Go SDK that can be
   wrapped in other languages via `cgo` or WebAssembly (`wasm`).
1. **Dynamic Extensibility**: Maintain a minimal core footprint by leveraging
   LLMs and dynamic "Skills" instead of hardcoded features. Capabilities like
   swarm orchestration or specialized deployment targets are handled as
   plugins.
1. **Multi-Agent Native**: Architect the system around the principle that "one
   agent alone is never enough." The platform natively assumes multi-agent
   collaboration, delegation, and routing.

## System Architecture

The architecture is divided into three primary layers:

### 1. Presentation Layer (Terminal UI & CLI)

- **Component**: The `swarm` CLI binary.
- **Responsibility**: Provides the interactive, conversational terminal
  interface. It handles prompt parsing, rich text rendering (Markdown, syntax
  highlighting), and local user configuration.
- **Technology**: Go (with terminal UI libraries like
  `charmbracelet/bubbletea`).

### 2. Core SDK Layer (Business Logic & Orchestration)

- **Component**: The embeddable `swarm` Go SDK.
- **Responsibility**: The brain of the CLI. Powered internally by the Go ADK,
  it handles:
  - Agent capability discovery (inspecting a project's agents).
  - Plugin and Skill loading.
  - Interaction with LLM providers.
  - Orchestration of multi-agent tasks (routing requests to the right local or
    remote agent).
  - GitOps workflow triggers.
- **Design Principle**: Strict separation from the UI. Everything the CLI does
  must be available via an exported SDK method.

### 3. Execution & Extensibility Layer (Plugins & Skills)

- **Component**: Dynamic capabilities and framework adapters.
- **Responsibility**: Interfaces with user-defined agents.
- **Adapters**: Adapters map concepts from LangChain, LangGraph, or ADK into a
  unified interface so the core SDK can run or deploy them uniformly.
- **Skills**: Lightweight, context-injected Markdown instructions (potentially
  backed by tools) that extend the CLI's knowledge (e.g., a "Deployment Skill"
  that knows how to deploy a LangGraph agent to AWS).

## Multi-Agent Foundation

The core platform is built around multi-agent patterns. When the CLI receives
a complex task, the primary internal ADK agent acts as a router. It assesses
the available "Skills" and local user-defined agents, delegating sub-tasks
rather than attempting to solve everything monolithically.

## Deployment & GitOps

The tool tightly integrates with standard version control operations.
"Deploying" an agent isn't necessarily a bespoke API call; by default, it
translates to scaffolding CI/CD pipelines (e.g., GitHub Actions) and pushing
code. The platform manages the entire lifecycle—from local iterative testing
to production releases—via Git commits and tags.
