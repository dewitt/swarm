# Design Doc: Async HITL, Input Queueing, and Interruptions

**Status:** Implemented (Core Input Queueing and Interruptions). Soft
Interruptions (Course Correction) and explicit HITL blocking UI are still
pending future iteration.

## Context

As the `swarm` CLI evolves into an agent-centric Agent Panel for orchestrating
swarms of parallel, long-running agentic processes, the traditional
synchronous Chat REPL paradigm breaks down.

When a user types a command like _"Refactor the billing service"_, the swarm
might take 45 minutes to execute tests, read files, and write code. During
this time, the user cannot simply be locked out of the interface. They must be
able to:

1. Queue up additional thoughts or instructions.
1. Observe that work is ongoing.
1. Interrupt the swarm if it hallucinates or goes down the wrong path.
1. Respond to asynchronous Human-In-The-Loop (HITL) requests from sub-agents.

This document outlines the architectural and UX strategies for handling
concurrent input, interruptions, and async HITL interactions.

---

## 1. Input Queueing vs. Blocking

Currently, if the user presses `Enter` while `m.loading == true`, the UI
discards the input. This is a severe UX anti-pattern for a multi-agent system.

### The Solution: The "Thought Queue"

The CLI should adopt a non-blocking queue model.

- **Behavior:** The user can type into the textarea and press `Enter` at any
  time, even if 10 agents are actively running.
- **Queueing:** Instead of sending the input directly to a blocking
  `Manager.Chat` call, the UI appends the text to an internal `m.inputQueue`.
- **Visuals:** Queued messages appear in the chat log immediately but are
  styled distinctly (e.g., slightly dimmed or with an hourglass icon `⧖`) to
  indicate they are "pending ingestion" by the Swarm Agent.
- **Processing:** Once the Swarm Agent's current cognitive cycle frees up, or
  if an agent specifically requests input, it pops the oldest message from the
  queue.

---

## 2. Visual Cues in the Input Box

The user must intuitively understand the system's state just by glancing at
the input prompt. If they are expected to wait, the UI should communicate that
gracefully without restricting their freedom to type.

### The UX Treatment

- **Idle State:**
  - Prompt: `> ` (Google Blue)
  - Placeholder: `Type your message...`
- **Active Swarm State (Agents Working):**
  - Prompt: `⧖ ` (Google Yellow, animated subtly)
  - Placeholder: `Agents are working. Type to queue a message or interrupt...`
  - Border: The border of the input box could shift to a dashed line or a
    specific color (like Yellow) to emphasize the active background state.
- **HITL Blocking State (Agent Demands Input):**
  - Prompt: `[Skill Builder] > ` (Google Red/Urgent)
  - Placeholder: `Waiting for your confirmation... (Y/n)`
  - The input box becomes the focal point, perhaps dimming the rest of the UI
    slightly to emphasize that the swarm is entirely blocked until the human
    responds.

---

## 3. Interruption Mechanisms

When a user realizes an agent is hallucinating or deleting the wrong files,
they need immediate control. We must support two distinct types of
interruptions.

### A. Soft Interruption (Steering)

A user wants to correct an agent's assumptions without destroying its current
progress.

- **Trigger:** The user types a message and hits `Enter` while the swarm is
  working.
- **Mechanism:** The message goes into the Input Queue. An
  `InterruptionSignal` is sent to the SDK Event Bus. The Swarm Agent is
  "tapped on the shoulder." It pauses its current chain of thought, reads the
  queued message, and decides how to course-correct the swarm dynamically.

### B. Forceful Interruption (Emergency Stop)

A user wants to instantly halt all execution, clear the queue, and stop the
bleed.

- **Trigger:** The user presses a dedicated hotkey, e.g., `Ctrl+C` (currently
  mapped to quit) or `Esc`.
- **Mechanism:**
  1. The UI instantly clears the input queue.
  1. A context cancellation (`context.Cancel()`) is propagated down through
     the `Manager` to all running ADK `runner` instances.
  1. The `bash_execute` tools receive the cancellation and send SIGKILL to
     their underlying subprocesses.
  1. The UI renders: `[System] Swarm execution forcefully halted by user.`

---

## 4. Architectural Implementation (SDK Event Bus)

To support this non-linear flow, the core `pkg/sdk/` must migrate away from
the linear `Chat(prompt) (<-chan string)` signature.

### The Proposed `SessionContext`

The SDK should expose a long-lived `SessionContext`.

1. **The Ingress Channel:** The UI pushes strings (or `InputEvent` structs)
   into a continuous ingress channel. The Swarm Agent listens to this channel
   in a persistent background goroutine.
1. **The Egress Channel (Event Bus):** The SDK pushes structured events
   (`AgentSpawned`, `ToolStream`, `TextChunk`, `HITLRequest`) back to the UI.
1. **The HITL Loop:** If a tool like `ask_user` is invoked by an agent, the
   SDK pauses that specific agent's execution thread and emits a `HITLRequest`
   event. The UI captures this, changes the input box state, and routes the
   next user input directly back to that specific tool call's fulfillment
   channel.

### Summary of Changes

- Refactor `cmd/swarm/interactive.go` to handle queued inputs and visual state
  shifts.
- Refactor `pkg/sdk/manager.go` to support persistent background runners and
  bidirectional channels.
- Implement `context.Context` plumbing through all custom tools to support
  instant forceful interruptions.
