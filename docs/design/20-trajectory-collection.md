# Trajectory Collection and Offline Analysis

## Core Principle: Every Execution is an Opportunity to Improve

In a complex, multi-agent system like Swarm, reasoning and execution paths
(trajectories) are incredibly valuable. When the system succeeds, the
trajectory is a golden example of autonomous problem-solving. When the system
fails, gets stuck in a loop, or misinterprets an intent, the trajectory is the
exact debugging trace needed to correct the behavior.

**The Principle:** Every single user request and subsequent swarm execution
should be saved as a comprehensive trajectory, providing a continuous feedback
loop for improving the system.

## What is a Trajectory?

A trajectory is the complete, serialized execution history of a single session
or task. It includes:

- **The initial user prompt and context.**
- **The generated `ExecutionGraph` (spans and dependencies).**
- **Every agent's internal reasoning (`ChatEventThought`).**
- **Every tool call made, its arguments, and the raw results
  (`ChatEventToolCall`/`ChatEventToolResult`).**
- **Any observer interventions or replan events.**
- **The final synthesized response.**

_(Note: The `Span` abstraction inherently supports this via OTel conventions,
capturing traces of the entire graph.)_

## Storage Strategy

### Phase 1: Local-Only (Current Focus)

Trajectories are highly sensitive. They contain user source code, raw terminal
output, and potentially environment variables.

- **Default Behavior:** All trajectories are saved locally by default to a
  designated directory, such as `~/.swarm/trajectories/` or within the
  `.git/.swarm/` directory of the active project.
- **Format:** Stored as structured JSON Lines (JSONL) or standard JSON files,
  timestamped and tagged with the session ID.
- **Privacy:** Local storage guarantees no code or terminal output leaves the
  user's machine without explicit action.

### Phase 2: Opt-In Telemetry (Future)

As the system matures, we can introduce a mechanism for users to _volunteer_
their trajectories to help improve the core Swarm product.

- **Anonymization:** A pipeline to strip PII, secrets, or proprietary code
  paths before upload.
- **Explicit Consent:** Users must explicitly opt-in (e.g.,
  `swarm config set share_trajectories true` or an interactive prompt after a
  massive, successful task).

## The Immediate Value: Local Agentic Debugging

The primary, immediate reason to store these trajectories locally is **Agentic
Self-Improvement**.

Because Swarm is built heavily on dynamic `SKILL.md` files and prompt
engineering, the best way to debug a swarm failure isn't always stepping
through Go code—it's reading the agent's thoughts.

By saving trajectories locally, a developer (or a coding assistant like
Gemini/Claude) can:

1. **Review the Trace:** Ask the coding agent to read
   `~/.swarm/trajectories/failed_run_123.json`.
1. **Identify the Flaw:** The coding agent can pinpoint exactly where a
   sub-agent hallucinated a bash command or where the `planning_agent`
   misunderstood the dependencies.
1. **Implement the Fix:** The coding agent can then directly propose edits to
   `SKILL.md` files or the underlying SDK tools (like `tools.go`) to prevent
   that specific failure mode from ever happening again.

In essence, the Swarm produces the exact diagnostic data needed for another AI
to improve the Swarm. This creates a powerful, localized flywheel of
continuous improvement.
