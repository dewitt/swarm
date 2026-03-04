# Design Document 27: Stateful Routing and Reflection

## 1. Goal
Upgrade the Swarm Engine from a single-shot DAG execution model to a stateful "Plan -> Execute -> Reflect" architecture. This will enable the Swarm Agent to autonomously chain multi-step workflows (like diagnosing a codebase *and then* applying the fix) and turn our remaining Agentic E2E tests green.

## 2. The Problem: "Fire and Forget" Routing
Analysis of recent E2E evaluation failures (specifically Scenarios 1, 3, and 5) revealed a structural cognitive limitation in our `Execute` loop:
1. The user asks Swarm to "fix a bug in main.go".
2. The Swarm Agent generates an initial `ExecutionGraph` containing a single span: `[{"agent": "codebase-investigator"}]`.
3. The engine executes the graph. The investigator successfully finds the bug and outputs a correct textual report.
4. Because the sub-agent didn't error (it completed its scope), it doesn't trigger the `request_replan` exception logic.
5. The original execution graph is now empty. The Swarm Engine considers the task complete and yields control back to the user, **never actually applying the fix.**

This is brittle. The Swarm Planner is acting as a "fire and forget" router. It predicts the *first* step but lacks a mechanism to verify if the *overall user intent* was solved after that step concludes.

## 3. The Solution: The Reflection Loop
We must introduce a stateful reflection phase at the end of every Execution Graph traversal. Instead of yielding to the user when `Execute()` finishes, the `Chat()` method will pass the aggregated execution results back to the `Swarm Agent` (acting as a "Judge") to ask if the original request string is fully resolved.

If not resolved, the Swarm Agent generates a *new* execution graph (or calls the `write_local_file` tool directly) to take the next required steps, bridging the gap between diagnosis and implementation.

### 3.1 Proposed Changes to `pkg/sdk/swarm.go`
Inside the `Chat(ctx, prompt)` function, we will wrap the planning and execution phases in a `for` loop:
```go
maxCycles := 3
for cycle := 0; cycle < maxCycles; cycle++ {
    // 1. Plan
    graph, err := m.Plan(ctx, prompt)
    
    // 2. Execute
    events, _, err := m.Execute(ctx, graph, o)
    
    // 3. Reflect
    // After execution finishes, we query the Swarm Agent (or a dedicated Reflection Agent)
    // with the full session history and ask: "Is the user request '%s' fully satisfied? If not, what must be done next?"
    reflection, isDone := m.Reflect(ctx, prompt)
    if isDone {
        break // Return to user
    }
    
    // Inject the reflection into the prompt for the next cycle
    prompt = fmt.Sprintf("Previous step completed. Reflection: %s. Continue pursuing the original goal.", reflection)
}
```

### 3.2 Impact on Terminal UI
Because the engine already emits `ObservableEvent` streams, this background logic requires zero changes to the interactive Bubble Tea client. The user will simply see the Swarm Agent enter an `AgentStateThinking` phase after a worker finishes, followed by a new `AgentStateExecuting` phase as it spawns the follow-up task.

## 4. Verification Plan
1. Implement the `Reflect` loop in `pkg/sdk/swarm.go`.
2. Re-run `swarm eval`. The `swarm_agent` should read the output of the investigators in Scenarios 1, 3, and 5, realize the bug hasn't actually been physically patched yet, and autonomously launch a follow-up span to `write_local_file`.
3. Achieve 100% pass rate on the E2E suite.
