# Dynamic Swarm Provisioning & Reactive Orchestration

## The Vision: The Reactive "Engineering Manager"

The `swarm` project rejects the idea of a static execution plan. In complex agentic systems, **"no plan survives first contact with the enemy."** 

We are moving from a "Waterfall Scheduler" (Plan -> Execute) to a **Reactive Controller** (Seed -> Execute -> Observe -> Mutate). The system operates as a **Byzantine Swarm**, where multiple agents operate concurrently, constantly re-evaluating the path forward based on real-world feedback.

## Architectural Components (The Reactive Model)

### 1. The Input Agent (Invisible Intermediary)

Every user prompt is processed by the **Input Agent**.
- **Role:** Invisible preprocessing. It handles social greetings, meta-questions, and intent classification.
- **Invisibility:** While its actions are recorded in trajectories and the Agent Panel, it does not appear as a speaking participant in the user chat.

### 2. The Planning Agent (On-Demand Architect)

The **Planning Agent** (formerly the Architect) is an on-demand service for task decomposition.
- **Node Autonomy:** Nodes are trusted to execute on their own. They only call upon the Planning Agent if they require a structured execution graph to fulfill a complex request.
- **Triviality Handling:** Trivial tasks skip the Planning Agent entirely, with nodes responding directly to the requester.

### 3. The Reactive Task Pool (The Living Graph)

The core of the system is a thread-safe **Task Pool**.
- **Dynamic Mutability:** Tasks can be added, canceled, or branched mid-execution.
- **Result Sharing (Blackboard):** Every completed node posts its results to a shared Blackboard, making the data available to all concurrent and future nodes.

### 4. Continuous Observer Agents (Checks & Balances)

To manage a byzantine swarm, we need independent oversight.
- **Sidecar Observers:** Lightweight agents that monitor the live telemetry of workers.
- **Trust but Verify:** Observers ensure that autonomous nodes stay on track, correcting hallucination loops or intent deviations asynchronously.

### 5. The Output Agent (Invisible Shield)

The **Output Agent** provides the final layer of sanity checking.
- **Role:** Damage control. It reviews every response destined for the user. If it detects a critical failure, it blocks the output and triggers a replan.
- **Invisibility:** Like the Input Agent, it is a silent mediator that does not appear in the chat log.

### 6. The Swarm Agent (The Core Persona)

The **Swarm Agent** (formerly the Swarm Agent) is the primary agent.
- **Role:** It holds the instructions that define the overall persona, characteristics, and rules of the swarm application. It is the agent that "represents" the system to the user.

## UI Visualization (The Execution Graph)

The Agent Panel must evolve to visualize these dependencies:
- **Status Mapping:** 
    - ⚪ **Pending:** Task discovered but dependencies not met.
    - 🔵 **Active:** Task currently executing.
    - 🟢 **Complete:** Task finished successfully.
    - 🔴 **Failed:** Task hit an unrecoverable error.
    - 🟡 **Blocked:** Waiting for user input or resource availability.
    - 🟣 **Invalidated:** Task pruned due to a pivot in the plan.

## Implementation Phases

1. **Naming Consolidation:** Rename all internal symbols to match Input/Output/Planning/Swarm agent conventions.
2. **Invisible Mediation:** Refactor the UI to hide mediation agents from the chat history while keeping them in the panel.
3. **Node Autonomy:** Trust worker nodes to handle their own next steps by default.
4. **Observer Sidecars:** Refactor background monitoring to be truly asynchronous.
5. **UI Graph Fidelity:** Update the Agent Panel to visualize a **living, shifting graph**.
