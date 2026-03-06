# Design Document 27: Stateful Routing and Reflection

## 1. Goal

Upgrade the Swarm Engine from a single-shot DAG execution model to a stateful
"Plan -> Execute -> Reflect" architecture. This will enable the Swarm Agent to
autonomously chain multi-step workflows (like diagnosing a codebase *and then*
applying the fix) and turn our remaining Agentic E2E tests green.

## 2. The Problem: "Fire and Forget" Routing

Analysis of recent E2E evaluation failures (specifically Scenarios 1, 3, and
5\) revealed a structural cognitive limitation in our `Execute` loop:

1. The user asks Swarm to "fix a bug in main.go".
1. The Swarm Agent generates an initial `ExecutionGraph` containing a single
   span: `[{"agent": "codebase-investigator"}]`.
1. The engine executes the graph. The investigator successfully finds the bug
   and outputs a correct textual report.
1. Because the sub-agent didn't error (it completed its scope), it doesn't
   trigger the `request_replan` exception logic.
1. The original execution graph is now empty. The Swarm Engine considers the
   task complete and yields control back to the user, **never actually
   applying the fix.**

This is brittle. The Swarm Planner is acting as a "fire and forget" router. It
predicts the *first* step but lacks a mechanism to verify if the *overall user
intent* was solved after that step concludes.

## 3. The Solution: The Reflection Loop

We must introduce a stateful reflection phase at the end of every Execution
Graph traversal. Instead of yielding to the user when `Execute()` finishes,
the `Chat()` method will pass the aggregated execution results back to the
`Swarm Agent` (acting as a "Judge") to ask if the original request string is
fully resolved.

If not resolved, the Swarm Agent generates a *new* execution graph (or calls
the `write_local_file` tool directly) to take the next required steps,
bridging the gap between diagnosis and implementation.

### 3.1 Proposed Changes to `pkg/sdk/swarm.go`

Inside the `Chat(ctx, prompt)` function, we will wrap the planning and
execution phases in a `for` loop:

```go
maxCycles := 5 // Bounded to prevent infinite loops
o := NewEngine(nil) // Persistent engine across cycles to maintain context

for cycle := 0; cycle < maxCycles; cycle++ {
    // 1. Plan
    // Plan receives the structured context of prior cycles, not just a string concatenation.
    graph, err := m.Plan(ctx, prompt, o.GetTrajectory())
    if err != nil {
        return handleErr(err)
    }
    
    // 2. Execute
    events, updatedEngine, err := m.Execute(ctx, graph, o)
    if err != nil {
        return handleErr(err)
    }
    
    // Drain events sequentially to prevent goroutine deadlock before reflection
    for ev := range events {
        out <- ev 
    }
    o = updatedEngine // Carry-over engine state
    
    // 3. Reflect
    // Reflect is a strict evaluation against the original request, using a dedicated LLM call
    // that outputs structured JSON: { "is_resolved": boolean, "reasoning": string, "next_steps": string }
    reflection, err := m.Reflect(ctx, prompt, o.GetTrajectory())
    if err != nil {
         return handleErr(err)
    }
    
    out <- ObservableEvent{Timestamp: time.Now(), AgentName: "Swarm", State: AgentStateThinking, Thought: "Reflecting: " + reflection.Reasoning}
    
    if reflection.IsResolved {
        break // Return to user
    }
    
    // Inject structured reflection into the prompt for the next cycle
    prompt = fmt.Sprintf("Original Goal: %s\n\nReflection from last cycle: %s\nNext Steps: %s", originalPrompt, reflection.Reasoning, reflection.NextSteps)
}
```

### 3.2 Impact on Terminal UI

Because the engine already emits `ObservableEvent` streams, this background
logic requires zero changes to the interactive Bubble Tea client. The user
will simply see the Swarm Agent enter an `AgentStateThinking` phase after a
worker finishes, followed by a new `AgentStateExecuting` phase as it spawns
the follow-up task.

## 4. Verification Plan

1. Implement the `Reflect` loop in `pkg/sdk/swarm.go`.
1. Re-run `swarm eval`. The `swarm_agent` should read the output of the
   investigators in Scenarios 1, 3, and 5, realize the bug hasn't actually
   been physically patched yet, and autonomously launch a follow-up span to
   `write_local_file`.
1. Achieve 100% pass rate on the E2E suite.

## 5. Peer Critique & Revisions

This design was peer-reviewed by an independent model (`claude-code`) acting
as a Principal Engineer. The critique identified several critical flaws in the
naive `for` loop, namely deadlocking the `events` channel by not draining it
before `Reflect`, losing engine state across cycles, and unbounded loops
intersecting with inner `request_replan` mechanics.

**Revisions Made:**

- **Explicit Event Draining:** The outer loop now explicitly drains the
  `events` channel yielded by `Execute` to prevent blocking the execution
  goroutines.
- **Engine Carry-over:** The `Engine` instance `o` is initialized *outside*
  the loop and passed continuously to `Execute` and `Plan` to ensure context,
  tool results, and trajectory history are strictly maintained across
  iterations.
- **Structured Reflection Signature:** `Reflect` is explicitly defined as
  returning a strict
  `{"is_resolved": bool, "reasoning": string, "next_steps": string}` JSON
  struct, abandoning opaque string concatenation.
- **Bounded Execution Safety:** `maxCycles` acts as a hard safety limit. If
  breached, the engine emits a visible user-facing final error rather than
  silently failing.
