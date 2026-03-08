# Design Doc: SDK Refactoring & Modularization

**Status:** Proposed **Author:** Code Quality Advocate
(@code_quality_advocate) **Date:** March 2026

## 1. Problem Statement

The `pkg/sdk/swarm.go` file has grown into a "God Object," currently sitting
at over 1,800 lines of code. It contains initialization logic, tool registry,
the main execution loop, planning phases, reflection logic, and internal
structs.

From an **Agentic Coding Perspective**, massive files present distinct
challenges:

- **Context Window Exhaustion:** When an agent reads `swarm.go` to understand
  how `Plan()` works, it is forced to ingest 1,800 lines of irrelevant routing
  and UI-facing observable event logic, consuming a large portion of its token
  budget.
- **Search Noise:** Grep searches within the file return dozens of false
  positives.
- **Merge Conflicts:** As multiple agents (or parallel sub-agents) attempt to
  modify `swarm.go` simultaneously, the likelihood of Git merge conflicts
  increases exponentially.

## 2. Go Idiomatic Best Practices

In Go, it is highly idiomatic to split a single logical package (like `sdk`)
across multiple smaller `.go` files based on behavior or domain, rather than
grouping by object. Because all files in the same directory share the same
`package sdk` namespace, we do not need to export internal methods or structs
(capitalizing them) just to split them across files.

## 3. Proposed File Structure

We will surgically split `swarm.go` into the following tightly scoped files,
leaving all types and methods attached to `defaultSwarm` but separated by
domain:

1. **`swarm.go` (The API Contract & Init):**

   - Keeps the `Swarm` interface definition.
   - Keeps the `defaultSwarm` struct definition.
   - Keeps `NewSwarm()`, `Close()`, and `Reload()`.
   - Keeps the tool registry initialization.

1. **`swarm_chat.go` (The Entrypoint):**

   - Contains the `Chat()` method.
   - Contains the Input Agent / routing fast-path logic.
   - Contains the outer `maxCycles` orchestrator loop.

1. **`swarm_plan.go` (The Strategist):**

   - Contains the `Plan()` method.
   - Contains the `extractJSON` string manipulation helper.

1. **`swarm_execute.go` (The Actor):**

   - Contains the `Execute()` method (which manages the `Engine` loop).
   - Contains `executeSpan()`.
   - Contains the `runOutputAgent()` sanity-checker.

1. **`swarm_reflect.go` (The Critic):**

   - Contains the `Reflect()` method.
   - Contains the `Reflection` struct definition.
   - Contains `SummarizeState()`.

1. **`swarm_context.go` (Memory & State):**

   - Contains `AddContext()`, `DropContext()`, `ListContext()`.
   - Contains global tool wrappers like `readState`, `writeState`,
     `retrieveFact`.
   - Contains trajectory persistence (`saveTrajectory`, `GetSessionDir`).

## 4. Why this is GOOD for AI Agents

This refactoring is not just an aesthetic human preference; it drastically
improves agentic workflows:

- **Targeted Reading:** If an agent needs to fix a bug in the JSON
  unmarshaling, it only needs to read `swarm_plan.go` (approx. 150 lines)
  instead of the entire 1,800-line monolith.
- **Clearer Responsibility:** File names act as semantic signposts. An agent
  explicitly knows that `swarm_reflect.go` is where it should look to modify
  how the Swarm evaluates its own success.
- **Parallel Modification:** An agent can safely inject a new method into
  `swarm_context.go` while another agent (or human) modifies the `Execute()`
  loop in `swarm_execute.go` without creating a Git conflict.
