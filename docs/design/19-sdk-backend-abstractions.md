# SDK Backend Abstractions & Interfaces

## Overview

Just as the Terminal UI is strictly divided into four principled areas, the core `swarm` SDK is designed around a set of rigorous, decoupled abstractions. This document defines the primary interfaces and data structures that power the framework-agnostic Swarm backend.

By adhering to these abstractions, we ensure that the "Engineering Manager" paradigm remains a property of the *core SDK*, allowing the same orchestration logic to be seamlessly embedded into web panels, VS Code extensions, or Slack bots.

## 1. The Swarm Interface

The `Swarm` is the primary facade and entry point for any consumer (like the CLI) interacting with the underlying swarm. It encapsulates context management, session persistence, and the execution lifecycle.

### Core Responsibilities
- **Context Management:** Handles pinning and dropping files (`AddContext`, `DropContext`) from the active session's memory window.
- **Planning:** Takes a raw user prompt and invokes the internal Planning Agent to generate a deterministic `ExecutionGraph` of spans (`Plan()`).
- **Execution:** Takes an `ExecutionGraph` and coordinates its resolution via the `Engine`, returning a stream of `ChatEvent`s (`Execute()`).
- **Fast-Path Interaction:** Provides a simpler `Chat()` method that internally pipelines intent classification, planning, and execution for conversational use cases.
- **Skill Discovery:** Dynamically loads and resolves `.skills/` directories, compiling them into executable ADK `Agent` instances.

## 2. The Engine and Node Autonomy

The `Engine` (`pkg/sdk/engine.go`) is the reactive runtime engine of the SDK. Complex goals require dynamic replanning ("no plan survives first contact").

### Core Responsibilities
- **Dependency Resolution:** Manages a pool of `Span`s, determining which spans are ready to run based on their defined `Dependencies`.
- **Parallel Dispatch:** Dispatches unblocked spans concurrently to the correct internal agents.
- **Dynamic Subgraph Expansion:** If an agent encounters an obstacle or realizes it needs to break a span down further, the Engine can capture that new subgraph and stitch it into the existing execution flow dynamically.
- **Context Aggregation:** As spans complete, the Engine aggregates their results (`GetContext()`) so that downstream dependent spans receive the exact outputs of their predecessors.

## 3. ExecutionGraph and Spans (Spans)

Work in the system is represented as a directed acyclic graph (DAG), designed to be highly observable and compatible with OpenTelemetry (OTel) conventions.

### `ExecutionGraph`
Represents a snapshot of the spans to be executed. It can also contain an `ImmediateResponse` to short-circuit the execution engine for trivial conversational queries.

### `Span` (Span)
A single unit of work. Every `Span` is uniquely identifiable and tracks its lifecycle (`Pending`, `Active`, `Complete`, `Failed`, `Invalidated`).
- **Node Autonomy:** Each span is assigned to a specific `Agent`. The SDK trusts that agent to do its best to fulfill the span instructions independently.
- **Telemetry Ready:** Spans store `StartTime`, `EndTime`, `Duration`, and `Attributes` (e.g., `gen_ai.prompt`, `gen_ai.completion`), allowing trivial export to tracing systems.

## 4. The Skill Abstraction

Instead of hardcoding support for every new agent framework or external tool, the SDK relies on dynamic `Skill`s.

### `SkillManifest`
A skill is represented by a directory containing a `SKILL.md` file. The YAML frontmatter of this file (`SkillManifest`) defines the capability's `Name`, `Description`, the target `Model` (e.g., `flash` vs. `pro`), and the specific native `Tools` it requires access to.

### Dynamic Compilation
At runtime, the `Swarm` scans the skills directories, reads the markdown instructions, binds the requested tools from the `toolRegistry`, and dynamically compiles them into native ADK `Agent` instances. This enforces the "Thin Software, Fat Models" philosophy—behavior is defined in plain text models rather than Go code.

## 5. Event Stream (`ChatEvent`)

Communication between the `Swarm` and the consumer interface is completely decoupled via an asynchronous channel of `ChatEvent`s. 

### Granular Visibility
The CLI UI consumes these events to render the multiplexed Agent Panel. Event types include:
- `ChatEventThought`: The agent's internal reasoning.
- `ChatEventToolCall` / `ChatEventToolResult`: Real-time lifecycle hooks for ephemeral UI spinners.
- `ChatEventTelemetry`: Streaming stdout/stderr from underlying bash processes or Git operations.
- `ChatEventObserver`: Interventions from parallel monitoring agents checking for infinite loops.
- `ChatEventFinalResponse`: The synthesized, Markdown-formatted final answer intended for the user.
