# Design Doc: The Input Agent

## Problem Statement

In a multi-agent system, the conversation often gets "stuck" in the context of
a specialized sub-agent. When a user changes the subject or provides a
general-purpose command, the sub-agent might try to handle it within its
narrow domain, leading to poor performance or confusion. Humans frequently
digress or shift topics, and the system should adapt seamlessly to these
changes.

## Proposed Solution: The "Input Agent" (Input Agent)

The Input Agent is a high-speed, lightweight agent that sits between the
user's raw input and the execution engine. Its sole responsibility is to
classify the input and determine which agent in the swarm is best suited to
handle it _before_ the message is processed by the current agent.

### Key Responsibilities

1. **Digression Detection**: Recognizing when the user's input is no longer
   related to the current agent's task.
1. **Contextual Routing**: Determining if the input should go to:
   - The current active agent (e.g., continuing a Git operation).
   - The primary Swarm Agent (for a new general-purpose task).
   - A different specialized agent (e.g., switching from Git to System
     Design).
1. **Implicit vs. Explicit Handoff**: Handling cases where the user doesn't
   explicitly ask for a handoff but their intent clearly shifts.

### Mechanism

The Input Agent would run as a pre-processor in the `sdk.Chat` method.

```go
// Conceptual pseudo-code in pkg/sdk/manager.go
func (m *defaultManager) Chat(ctx context.Context, prompt string) (<-chan string, error) {
    // 1. Run the Input Agent (Input Agent) to determine the target agent
    targetAgent := m.inputRouter.Classify(ctx, prompt, m.currentAgent)

    // 2. If targetAgent != m.currentAgent, perform a silent or explicit handoff
    if targetAgent != m.currentAgent {
        m.run.Transfer(targetAgent)
    }

    // 3. Proceed with normal execution
    return m.run.Run(...)
}
```

## Agent Opinion & Analysis

This approach is superior to the "Quiet Observer" pattern in several ways:

1. **Proactivity**: It catches digressions _before_ the wrong agent tries to
   answer, preventing confusing or incorrect responses.
1. **Latency/Efficiency**: Because the Input Agent has a very narrow scope
   (just routing), it can use a much smaller, faster model (like
   `gemini-1.5-flash-8b`) than the primary Swarm Agent or sub-agents.
1. **Simplicity**: It centralizes the "routing logic" into a single,
   specialized agent rather than spreading "oversight" logic across the SDK.

### Implementation Considerations

- **Statefulness**: The Input Agent needs to know who the "current" agent is
  and what the recent history looks like.
- **Ambiguity**: If the input is ambiguous, it should default to the current
  agent to avoid unnecessary context switching.
- **Latency Budget**: This pre-processing step must be extremely fast (\<
  500ms) to ensure the UI remains responsive.

______________________________________________________________________

**Status:** Implemented **Reference:** Issue #25 (Swarm Agent Oversight)
