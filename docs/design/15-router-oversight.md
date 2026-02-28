# Issue: Router Agent Oversight and Task Handoff

## Problem Statement

When a user is interacting with a specialized subagent (e.g., `github_agent`, `architect_agent`), the session context often becomes locked to that agent's specific domain. If the user then provides a general followup request that is outside that domain, the specialized agent may struggle to handle it, or provide a suboptimal response, because it lacks the "big picture" capabilities of the primary Router Agent.

Currently, the transition back to the Router Agent is often manual or requires the user to explicitly "break" the subagent's loop.

## Proposed Solution: The "Quiet Observer" Pattern

The Router Agent should continuously observe the conversation between the user and any active subagents. It should be empowered to "jump in" and take over the conversation when it detects a shift in intent that is better suited for a different agent or for the Router's own general-purpose orchestration logic.

### Key Capabilities

1. **Passive Intent Detection**: The Router Agent analyzes every user message, even when a subagent is active.
2. **Contextual Handoff**: If the Router detects a domain shift, it interrupts the subagent and provides a transition message (e.g., "I'll take it from here to handle your broader request...").
3. **Seamless State Transfer**: The Router should maintain the context of what the subagent just accomplished so it can incorporate that into the next phase of the task.

## Agent Opinion & Analysis

As an AI agent, I believe this "Oversight" pattern is critical for a truly fluid multi-agent experience. Here are my specific thoughts on the implementation:

### 1. The "Interrupt" UX
The transition needs to be graceful. A blunt "I'm taking over" might feel jarring. Instead, the Router should use a transition that acknowledges the subagent's work: *"Now that the GitHub PR is created, I can help you with the broader system design update you mentioned."*

### 2. Efficiency vs. Over-Analysis
Running the Router Agent as a "quiet observer" on every turn adds token cost and latency. We should explore a "Triggered Observation" approach:
- The subagent itself could have a "Handoff Trigger" tool it calls when it detects it's out of its depth.
- Alternatively, a lightweight classifier (or a cheaper model) could act as the observer, only waking up the full Router Agent when a transition is likely needed.

### 3. Maintaining "Agency"
Specialized agents are effective because they have a narrow focus. If the Router interrupts too frequently, it may prevent the subagent from completing complex, multi-step tasks. The "Quiet Observer" needs a high threshold for interruption—only jumping in when the user's intent clearly diverges from the current subagent's manifest.

### 4. Implementation Strategy
I recommend implementing this as an "Oversight Skill" in the Go SDK. This skill would allow the Router to register a `Post-User-Input` hook that runs in parallel with (or just before) the active agent's turn.

---
**Status:** Open for Discussion
**Reference:** Task 6.2 (Swarm Skill)
