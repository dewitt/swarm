# Swarm Dashboard & Control Panel Architecture

## The Problem: The Black Box of Agent Swarms

Current AI developer tools (including the foundational iterations of the
`swarm` CLI) treat interactions linearly: a user inputs a prompt, a spinner
appears (`Thinking...`), and eventually, an output is rendered.

As we scale to complex multi-agent topologies—where dozens or even hundreds of
specialized agents (Web Researchers, Architects, Debuggers, Code Generators,
Linters) operate concurrently, asynchronously, and over long periods—the
single-spinner paradigm collapses.

**If 50 agents are running, the user cannot tolerate a black box.** They must
know:

1. Who is working?
1. What are they doing right now?
1. Are they stuck?
1. How can I intervene?

This document outlines the design for the **Swarm Dashboard and Control
Panel**, the defining feature that transforms `swarm` from a basic chat
interface into a Supreme Orchestrator.

## The Vision: Air Traffic Control for Code

The UI must shift from a traditional "Chat REPL" to a "Mission Control"
paradigm.

When a swarm is deployed, the terminal layout will dynamically split. The main
chat/viewport remains for high-level synthesis, but a new, persistent **Swarm
Dashboard** pane emerges.

### 1. The Swarm Dashboard (Visualizing Concurrency)

The dashboard will utilize a grid or dynamic list of "Agent Cards."

**Each Agent Card displays:**

- **Identity & State:** Agent name, Avatar/Color code, and current state
  (e.g., 🟢 Active, 🟡 Waiting for Input, 🔴 Errored, 💤 Sleeping).
- **Current Intent (The "Observer" Pattern):** Instead of flashing raw tool
  calls (e.g., `read_local_file`), the SDK employs a lightweight "Observer
  Agent" that reads the stream of raw tool calls and synthesizes them into
  human-readable intents (e.g., *"Tracing database authentication flow..."* or
  *"Resolving merge conflicts..."*).
- **Live Telemetry:** If an agent is executing a shell command, the card
  streams the last 3 lines of `stdout` in real-time. If it's searching the
  web, it shows the active query.
- **Resource Constraints:** Current token expenditure or execution time limit
  warnings.

### 2. The Control Panel (Intervention & Steering)

Users must be able to interact with the swarm in real-time without halting the
entire system.

- **Granular Pausing:** The user can select a specific Agent Card and press
  `Space` to pause that individual agent's execution loop, leaving the rest of
  the swarm running.
- **Micro-Steering:** A user can "focus" into an agent's card (e.g., pressing
  `Enter`) to open a direct sub-chat with that specific agent, answering its
  questions or correcting its assumptions without polluting the global chat
  context.
- **Panic/Kill:** A dedicated hotkey (`K`) to instantly terminate a runaway
  agent.

### 3. Asynchronous & Long-Running Jobs

Not all tasks finish in seconds. A user might tell the swarm: *"Migrate this
entire 100k line Java codebase to Go."*

- **Background Swarms:** Agents can be dispatched into the "background." The
  TUI session can be closed safely while the agents continue running via an
  independent daemon or cloud service.
- **Notification Center:** When the user returns to the TUI (or via OS
  notifications), they are presented with a unified inbox of agent completion
  reports, requests for HITL (Human-In-The-Loop) permissions, and PR links.

## 4. The Separation of Concerns: SDK vs. UI

A foundational principle of the `swarm` project is the strict separation
between the core SDK (`pkg/sdk/`) and the Presentation Layer (`cmd/swarm/`).
This separation must be rigorously maintained as we build out the Swarm
Dashboard.

The "Engineering Manager" paradigm is a business logic concept, not just a UI
trick.

- **The SDK Emits Standardized Events:** The core SDK must handle all the
  complex multi-agent orchestration, dynamic provisioning, and observer
  summarization. It must expose this state purely through a standardized Event
  Bus or gRPC/REST interface (e.g., `AgentSpawnedEvent`,
  `AgentStatusUpdateEvent`, `TelemetryStreamEvent`).
- **The UI is Just a Consumer:** The TUI (Bubble Tea) is merely *one* possible
  consumer of this event stream. It listens to the bus and renders the Agent
  Cards accordingly.
- **Portability:** By keeping the orchestration and telemetry strictly within
  the SDK, we ensure that other developers can effortlessly build completely
  different clients on top of the same swarm logic. A team could build a
  Next.js web dashboard, a native iOS app, a VS Code extension, or a Slack
  bot, and all of them would be able to monitor and steer the swarm
  identically without rewriting any of the core orchestration logic.

## Architectural Requirements (SDK & UI)

To achieve this, the underlying Go SDK (`pkg/sdk/`) and the Bubble Tea UI
(`cmd/swarm/`) require significant upgrades.

### SDK Upgrades: Event-Driven Architecture

Currently, `Manager.Chat` returns a single channel of strings. This must be
upgraded to an **Event Bus**.

- **Agent Lifecycle Events:** The ADK runner must emit structured events:
  `AgentSpawned`, `AgentPaused`, `AgentResumed`, `AgentTerminated`.
- **Tool Telemetry Events:** Tool execution must be non-blocking with respect
  to telemetry. E.g., `bash_execute` must yield a channel of `stdout` chunks
  back to the event bus *while* the tool is running, rather than returning a
  single monolithic string when it finishes.
- **The Observer Runtime:** A mechanism to spin up cheap, low-latency models
  (e.g., `gemini-2.5-flash-8b`) purely for synthesizing telemetry into
  human-readable status updates without impacting the main agent's reasoning
  loop.

### UI Upgrades: Bubble Tea Multiplexing

- **Component Architecture:** The UI must migrate to a rigid component
  architecture using libraries like `bubbles/list` or custom grid layouts.
- **Focus Management:** The UI must support complex focus states (e.g.,
  focusing the global chat input vs. focusing the dashboard grid to select an
  agent).
- **Responsive Layout:** The dashboard must collapse or paginate elegantly if
  the user's terminal window is too small to render 50 distinct Agent Cards.

## Implementation Roadmap (Swarm Dashboard)

1. **Phase 1: Structured Telemetry & Tool Context.** Refactor `Manager.Chat`
   to return a structured `Event` struct rather than a formatted string.
   Update the current TUI to display the specific arguments of a tool call
   (e.g., `Running bash: 'npm test'`).
1. **Phase 2: The UI Grid.** Build the basic Bubble Tea grid component for the
   Swarm Dashboard. Hardcode mock agents into the grid to perfect the layout,
   responsive collapsing, and focus navigation.
1. **Phase 3: Live Output Streaming.** Refactor the `bash_execute` tool in the
   SDK to stream its output to the event bus. Connect this stream to the Agent
   Cards in the UI.
1. **Phase 4: The Observer Agent.** Implement the background summarization
   loop that translates raw tool execution events into high-level semantic
   status updates for the UI.
1. **Phase 5: Granular Interventions.** Implement the Control Panel logic
   allowing the user to pause, terminate, or converse with specific agents in
   the dashboard.

## The Ultimate Vision: "Do This Complicated Thing"

The north star of the `swarm` project is an interface that effortlessly
scales from a trivial, single-turn query to a massive, asynchronous
engineering effort—all from the exact same entry point.

A user should be able to type `$ agents` to open the console and simply say:

> *"Refactor the entire billing microservice from Python to Go, ensure 100%
> test coverage, and update the Kubernetes deployment manifests."*

The CLI must be smart enough to understand that this is not a task for a
single LLM call. It must autonomously:

1. **Plan & Decompose:** The Router agent immediately spins up a "Planning
   Swarm" to break the massive request down into a dependency graph of
   sub-tasks.
1. **Dynamic Provisioning:** The system dynamically provisions as many agents
   as necessary to complete the work in parallel. It might launch three
   "Translation Agents" to convert Python files to Go concurrently, one "Test
   Synthesis Agent" to write the Go tests, and a "DevOps Agent" to handle the
   manifests.
1. **Continuous Coordination:** A "Supervisor Agent" sits above them, managing
   dependencies (e.g., telling the Test Agent to wait until the Translation
   Agent finishes a specific package).
1. **Transparent Execution:** Throughout this process (which may take hours),
   the Swarm Dashboard gives the user perfect, real-time visibility into the
   chaotic concurrency, synthesizing the activity via Observer Agents so the
   user feels completely in control.

In this paradigm, the user is no longer a pair programmer; they are an
Engineering Manager. The CLI is their entire engineering department,
dynamically scaling its workforce to meet the exact complexity of the request.
