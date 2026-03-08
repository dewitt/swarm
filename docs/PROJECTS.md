# Agent Task Selection Process

This document defines the standard operating procedure for agents to
autonomously select, prioritize, and assign themselves meaningful work at the
start of a session.

When instructed to "find something to work on" or "self-assign a task," an
agent must follow this exact sequence to ensure their work aligns with current
project priorities, resolves known debt, and adheres to the `PHILOSOPHY.md`.

## The Selection Sequence

Agents should evaluate potential tasks in the following strict order of
priority. If a high-priority task is found, the agent should propose it to the
human-in-the-loop (HITL) before moving down the list.

### 1. Active Epics & GitHub Issues (Highest Priority)

**Goal:** Contribute to the current overarching milestone. **Action:**

- Run `gh issue list` to view open bugs and enhancement requests.
- Prioritize bugs over enhancements.
- If an Epic is active (e.g., "Epic: The Self-Healing Swarm"), read the issue
  body (`gh issue view <id>`) and look for unchecked markdown task boxes
  (`[ ]`). Propose tackling one of those specific sub-tasks.

### 2. The Debt & Quality Backlogs

**Goal:** Resolve known friction points and architectural flaws before adding
new features. **Action:** Review the following documents in order:

1. **`docs/AGENTIC_QUALITY_ISSUES.md`**: Look for critical reasoning failures,
   infinite loops, or Swarm orchestration crashes. Agentic stability is
   paramount.
1. **`docs/UX_ISSUES.md`**: Look for "High Friction" items. The CLI must
   maintain a world-class, polished user experience.
1. **`docs/CODE_ISSUES.md`**: Look for critical bugs, dead code, or
   refactoring needs.
1. **`TODO.md`**: Look for pending migrations (e.g., SDK API changes) or known
   bugs.

### 3. Unfinished Designs & Critical User Journeys (CUJs)

**Goal:** Implement approved features and ensure core workflows are supported.
**Action:**

- Scan `docs/cuj/` for scenarios that are not yet fully supported by the CLI
  or SDK.
- Search `docs/design/` for files containing status markers like
  `**Status:** Proposed`, `Draft`, or `Brainstorming`.
- Propose implementing a specific mechanism from a recently accepted design
  document.

### 4. Proactive Innovation & Brainstorming (Lowest Priority)

**Goal:** Advance the Swarm vision when the backlogs are clean or a request is
made for "something big." **Action:**

- Read `PHILOSOPHY.md`.
- Brainstorm 2-3 novel ideas that strictly adhere to the project's core tenets
  (e.g., "Zero-HITL Verification", "Dynamic Replanning", "The Agent as the
  Interface").
- Propose drafting a new design document (`docs/design/XX-new-idea.md`) to
  flesh out the concept before writing any code.

## Proposal Protocol

Once an agent identifies a candidate task, they MUST NOT begin implementation
immediately. They must check in with their HITL or however their work
assignments are being dispatched before proceeding:

1. **The Task:** What is the specific issue, bug, or feature? (Cite the source
   document or Issue #).
1. **The "Why":** Why is this important right now? (e.g., "It blocks Epic #29"
   or "It resolves a high-friction UX issue").
1. **The Approach:** A 1-2 sentence high-level summary of how the agent
   intends to solve it.
1. **Confirmation:** End with a prompt asking for permission to proceed (e.g.,
   *"Shall I begin working on this?"*).
