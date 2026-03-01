# Dynamic Swarm Provisioning & Parallel Execution

## The Vision: High-Scale Agentic Orchestration

The ultimate goal of the `swarm` project is to transition from a serial, single-agent conversation into a multi-agent "Engineering Manager" paradigm. In this model, a user provides a high-level goal, and the CLI autonomously decomposes it into a dependency graph of sub-tasks executed by parallel worker agents.

## Architectural Components

To support complex dynamic graphs with parallel branches, the following architectural components are required in the SDK (`pkg/sdk/`):

### 1. The Graph Planner (The Architect)

A specialized agent responsible for task decomposition. 
- **Input:** High-level user goal + Workspace context.
- **Output:** A Directed Acyclic Graph (DAG) of tasks. 
- **Schema:** 
  ```json
  {
    "tasks": [
      { "id": "task_1", "name": "Research API", "agent": "web_researcher", "dependencies": [] },
      { "id": "task_2", "name": "Scaffold Backend", "agent": "builder", "dependencies": ["task_1"] },
      { "id": "task_3", "name": "Design UI Mockups", "agent": "designer", "dependencies": ["task_1"] },
      { "id": "task_4", "name": "Integrate Frontend", "agent": "builder", "dependencies": ["task_2", "task_3"] }
    ]
  }
  ```

### 2. The Swarm Orchestrator (The Scheduler)

A new component within the `AgentManager` that manages the execution of the DAG.
- **Dependency Tracking:** Monitors the status of each task.
- **Parallel Dispatch:** Identifies "ready" tasks (those whose dependencies are met) and dispatches them to available workers.
- **Concurrency Control:** Limits the number of simultaneous active agents to manage token costs and local resource usage.

### 3. Parallel Execution Loop

The `Runner` must be upgraded to support multiple concurrent agent sessions.
- **Context Isolation:** Each worker agent must have its own `session.Session` or a scoped sub-session to prevent context contamination.
- **Shared State (The Blackboard):** Agents need a shared mechanism to pass results (e.g., Task 1's research findings must be available to Task 2 and Task 3). This can be achieved via:
    - **Local Filesystem:** Standard GitOps workflow.
    - **Session Service:** Appending events to a shared session with specific metadata.

### 4. Multiplexed Event Stream

The `ChatEvent` and the `Chat` return channel must be upgraded to support multiple agents streaming events simultaneously.
- **AgentID Requirement:** Every `ChatEvent` must include a unique `AgentID` (or `TaskID`) so the UI can route the event to the correct Agent Card in the panel.
- **Event Aggregation:** The SDK must multiplex streams from $N$ workers into a single outbound channel for the UI.

## UI Visualization (The Execution Graph)

The Agent Panel must evolve to visualize these dependencies:
- **Graph View:** Optional transition from a grid of cards to a node-link diagram (using ASCII/Unicode drawing characters).
- **Indentation/Nesting:** Use indentation in the grid to show which agents are blocked or branched from others.
- **Status Mapping:** 
    - ⚪ **Pending:** Task discovered but dependencies not met.
    - 🔵 **Active:** Task currently executing.
    - 🟢 **Complete:** Task finished successfully; results available to dependents.
    - 🟡 **Blocked:** Dependencies met, but waiting for resources or HITL.

## Implementation Phases

1. **Protocol Definition:** Define the JSON schema for the DAG and update `ChatEvent` struct.
2. **DAG Planner Agent:** Create a new "Architect" skill that can reliably output the DAG.
3. **Serial DAG Execution:** Implement the Scheduler but execute tasks one-by-one to verify logic.
4. **Parallel Dispatch:** Enable concurrent execution of independent branches using Go routines.
5. **UI Graph Rendering:** Update the Bubble Tea Agent Panel to render dependency lines.
