# Design Doc: Agent Cards and the Agent Panel

## The Vision: The Defining Feature of Swarm CLI

The "Agent Card" and its enclosing "Agent Panel" are the defining visual and
interactive features of the Swarm CLI. They transition the user experience from
a traditional linear chat log into a dynamic, multi-dimensional Agent Panel for
observing and managing autonomous intelligence. 

By visually representing each agent as a tangible, stateful entity, users can
instantly comprehend the scale, concurrency, and health of their execution
environment. We believe this pattern will become the industry standard for
interacting with multi-agent systems.

## The Agent Card

The Agent Card is the atomic unit of the Agent Panel. Each card represents a
single instantiated agent in the swarm.

### 1. Dynamic Sizing and Responsive Fidelity
Cards must be highly elastic, automatically adjusting their size and fidelity based on the available terminal real estate and the number of currently active agents.

- **Maximum Fidelity (1-3 agents):** Displays the agent's icon, full name, current status message (e.g., "Reading `auth.go`"), live telemetry (e.g., last tail of `stdout`), and a progress indicator or spinner.
- **Medium Fidelity (4-10 agents):** Displays the icon, name, and a truncated status string. (This is our current baseline).
- **Minimum Fidelity (10+ agents):** Compresses down to a minimalist square containing *only* the agent's stable emoji/icon and a color-coded border. This ensures that even massive swarms (50+ agents) remain completely legible within a standard terminal window.

### 2. Stable Identity and Color Coding
- **Stable Emoji:** Every agent must have a stable, recognizable emoji (e.g., 🧠 for Swarm, 🌐 for Web Researcher, 🐙 for GitOps). This anchors the agent's identity, especially at minimum fidelity.
- **Border States:** The card's border acts as a quick-glance status indicator:
  - 🟢 **Green/Blue (Pulsing):** Active and thinking/executing.
  - 🔵 **Cyan:** Success / Task complete.
  - 🟡 **Yellow:** Waiting for user input (HITL) or blocked on a dependency.
  - 🔴 **Red:** Error state requiring intervention.
  - ⚪ **Gray (Dimmed):** Idle or sleeping.

### 3. Interactivity (Drill-Down)
Agent cards are not static telemetry. They are interactive targets.
- **Clickable/Selectable:** Users can use their mouse or keyboard to select a specific card.
- **Micro-Steering:** Clicking a card opens a modal, sub-pane, or shifts the main chat focus to that specific agent. This allows the user to inspect its scratchpad, view its full tool execution history, or chat with it directly to correct its course without polluting the global routing context.

## The Agent Panel

The Agent Panel is the container for the Agent Cards.

### 1. Layout and Visibility
- **Open by Default:** Because observing the swarm is the core value proposition of the CLI, the Agent Panel is open and visible by default.
- **Resizable & Collapsible:** The user can dynamically resize the panel
  (allocating more room for the chat or more room for the Agent Panel) or hide
  it entirely via a hotkey for deep focus work.

### 2. Ephemeral Lifecycles (The Fade-Out)
To prevent the Agent Panel from becoming cluttered with stale information,
agent cards follow an ephemeral lifecycle based on activity.

- **Pop-In:** A card instantly materializes in the Agent Panel the moment an
  agent is provisioned or activated.
- **The Fade-Out:** Once an agent completes its task, its card shifts to a
  "Success" or "Idle" state. After a configured timeout (e.g., 30 seconds),
  the card gracefully fades out and is removed from the Agent Panel.
- **Resident Agents:** "Always-on" agents like the **Swarm Agent** or the
  **Input Agent (Input Agent)** effectively never fade out because they are
  constantly evaluating inputs. They will remain visible but may drop to a
  dimmed "Idle" state.
- **Rarely Used Agents:** Specialized agents (e.g., a "DB Migration Expert")
  will only appear momentarily when called upon, do their work, and vanish,
  keeping the Agent Panel clean and highly relevant to the *current*
  moment.

### 3. Dynamic Relationship Mapping (Future Innovation)
As an advanced feature, the Agent Panel could utilize terminal drawing characters (e.g., `│`, `└`, `├`, `─`) to draw dynamic dependency lines between cards.
- If the Swarm Agent spawns the Investigator, a line connects them.
- If the Investigator spawns a Web Researcher, the hierarchy deepens.
- This creates a real-time, living execution graph, making complex multi-agent delegation instantly understandable.

---
**Status:** Accepted Design
**Next Steps:** Implementation of Ephemeral Lifecycles, Dynamic Resizing, and Mouse Click Support.
