# Design: Heuristic Deadlock Detection ("Overwatch")

*Resolves a key initiative in Epic #29 (The Self-Healing Swarm).*

## 1. Context & Motivation

As observed in evaluation trajectories (e.g., Scenario 5 where `claude-code`
repeatedly failed due to low credits), Swarm agents can easily fall into
"logic loops." A common failure mode occurs when:

1. An agent executes a tool (e.g., `bash_execute`).
1. The tool returns a terminal error.
1. The agent uses `request_replan` or simply retries the exact same span.
1. The system blindly routes the span back to the same agent.
1. The agent tries the exact same command, failing again.

This creates a deadlock. The agent burns through tokens, frustrates the user,
and eventually hits a hard iteration limit.

**The Vision:** We need a proactive "Overwatch" mechanism—a lightweight,
heuristic observer built directly into the SDK's execution engine. This
observer will monitor the live trajectory stream, detect cyclical patterns or
repeated failures, and violently interrupt the execution graph to force a
structural replan or escalate to human intervention before tokens are wasted.

## 2. Architecture & Philosophy

This capability adheres to the "Ubiquitous Mediation" philosophy. We are
introducing a mediating layer within the `Engine` that watches state
transitions.

### 2.1. The Overwatch Routine

The `pkg/sdk/engine.go` currently executes nodes in a graph. We will introduce
an `Overwatch()` method that is invoked after every span completion.

### 2.2. Heuristic Rules for Deadlock

The Overwatch routine will analyze the `Trajectory` for the following
heuristics:

1. **Consecutive Tool Failures:** If the same agent uses the same tool with
   the exact same arguments and receives an error `N` times in a row (e.g.,
   `N=3`), it is deadlocked.
1. **Cyclic Delegation:** If Agent A delegates to Agent B, who delegates to
   Agent A, who delegates to Agent B without any meaningful state change or
   successful tool execution in between, it is deadlocked.
1. **Repeated Replanning:** If `request_replan` is invoked consecutively `N`
   times without any intervening successful tool execution that mutates the
   environment (e.g., writing a file), the planning mechanism is deadlocked.

### 2.3. The Interruption Mechanism

When a deadlock is detected, the Overwatch routine will:

1. **Halt Execution:** Instantly cancel the current executing span and prevent
   further scheduled nodes from running.
1. **Synthesize Context:** Generate a strict, system-level error event:
   `DEADLOCK_DETECTED: Agent X repeatedly failed to execute Tool Y. Forcing structural replan.`
1. **Force Re-Routing:** Pass this dense error context back to the
   `routing_agent` (or `planning_agent`) with an explicit instruction: *Do NOT
   use Agent X or Tool Y for this immediate next step. You must find an
   alternative path.*

## 3. Implementation Plan

### Phase 1: State Tracking in the Engine

1. Modify `pkg/sdk/engine.go` (or `swarm.go`) to maintain a rolling window of
   recent tool executions, including their arguments and success/fail status.
1. Implement the `detectDeadlock()` function to evaluate the heuristic rules
   against this rolling window.

### Phase 2: The Violent Interruption

1. If `detectDeadlock()` returns true, the Engine must emit an
   `AgentStateError` event.
1. The main orchestration loop in `pkg/sdk/swarm.go` (inside the `Chat` loop)
   must catch this specific deadlock error.
1. Instead of simply returning the error to the user, the orchestrator will
   automatically trigger a new `Reflection` or `Replan` cycle, forcefully
   injecting the deadlock context into the prompt so the model is aware of its
   past failures.

## 4. Expected Outcome

By implementing Overwatch, we eliminate the "infinite looping" failure mode.
Swarm becomes self-aware of its own failures. If a specific tool or agent is
broken (e.g., a missing API key), Swarm will try it, realize it's failing,
immediately stop trying it, and either try a different approach or cleanly
escalate to the user with a precise error report.
