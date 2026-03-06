# Design Doc 24: Git-Native Agent Collaboration

**Status**: Proposed **Author**: Antigravity **Date**: March 2026

## Objective

To map out Swarm's fundamental architecture for orchestrating long-running,
multi-turn, multi-model collaboration. This document proposes moving away from
centralized, synchronous state management (the "monolithic orchestrator") and
adopting a **decentralized, Git-native coordination protocol** inspired by
open standards like Agent-Roles, OpenClaw, and Gastown.

## The Holy Grail Workflow

The goal is to flawlessly execute complex, asynchronous pipelines such as:

> *“Write a design doc for a multiplayer, old-school ascii roguelike like
> Nethack. Coordinate a long-running conversation across the most capable AIs
> to write, review, critique, improve, and polish the doc.”*

Currently, if Swarm attempts this, it funnels the entire effort through a
single, ephemeral session loop. It relies entirely on the LLM's spontaneous
ability to `request_replan` or pass massive string payloads back and forth in
an immediate fashion. This leads to context window bloat, loss of reasoning
track, and high-latency deadlocks.

## Architectural Paradigm Shift: The GitHub Message Bus

The core constraint of Swarm is **"Fat Models, Thin Software."** We must avoid
reinventing deterministic state-machine workflow engines (like LangGraph)
natively in Go until the Google ADK formally supports them.

Instead of forcing agents to communicate via proprietary RPCs or JSON payloads
over SQLite, they will communicate identically to human developers: **via Git
and GitHub**.

### 1. Text-as-Truth & Sandbox Isolation

Agents (whether Swarm, Claude Code, or an external tool like GitHub Copilot
Workspace) must never assume a shared, perfectly synchronized local runtime
state. Instead, the repository itself is the blackboard.

- A Writer agent (`claude-3.5-sonnet`) creates a separate git branch and
  drafts `nethack_multiplayer_design.md`.
- It commits the file and opens a Pull Request.
- It is now done.

### 2. Peer-to-Peer Polling via standard tools

A Reviewer agent (`gemini-2.5-pro`) is running asynchronously. It polls GitHub
(via the standard `gh pr list` CLI). It sees an open PR requesting review.

- It checks out the branch.
- It reads the delta.
- It uses the `gh pr review` tool to leave formal, granular inline comments
  outlining architectural flaws in the networking stack.

### 3. Asymmetric Role Injection

We will adopt the **Agent Roles standard** natively. Swarm will no longer
hardcode agent personas in the binary. Instead, we rely on repository-level
configuration files (e.g., `AGENTS.md`, `ROLES.md`, or OpenClaw's `SOUL.md`).

- A project dictates the existence of an `Architect`, `Reviewer`, and
  `Tester`.
- When Swarm spins up a generic agent to fulfill a task, it dynamically reads
  these markdown definitions and injects them into the root System Prompt.
  This guarantees the model adopts the correct posture (e.g., a
  hyper-critical, adversarial mindset for the `Reviewer` role) before looking
  at the repository.

## The Work Loop (The "Gastown" Factory Floor)

By treating Swarm as the Execution Engine rather than the Identity Manager,
the workflow becomes radically decoupled:

1. **The Concierge (Router):** Parses the user's initial prompt. Recognizing
   the task is massive, it breaks it down into "Beads" (lightweight abstract
   tasks). It creates a GitHub Issue: "Draft Multiplayer Nethack Design" and
   assigns the `Architect` label.
1. **The Architect (Sub-agent):** Claims the Issue, works in a local branch,
   pushes a PR, and tags the `Reviewer` role.
1. **The Reviewer (Sub-agent):** Reads `PROCESS.md` to understand the review
   criteria. Critiques the PR.
1. **The Resolution:** This cycle repeats entirely asynchronously across
   GitHub Actions, native CLI polling, or explicit webhooks, safely
   constrained by native Git DAG histories.

## Phased Implementation (v0.05 Target)

1. **Role Standard Adoption:** Teach Swarm's `input_agent` to always locate
   and read `AGENTS.md` and `ROLES.md` on startup, merging that text into the
   dynamic context of all spawned child agents.
1. **Git-Centric Tool Bias:** Modify `tools.go` to aggressively favor `gh` CLI
   commands (`issue create`, `pr create`, `pr review`) for inter-agent
   handoffs over writing raw data back to the prompt window.
1. **The "Check Status" Loop:** Introduce an autonomous "daemon" mode for
   Swarm where it wakes up at an interval, runs `gh pr status` or checks
   GitHub notifications, reads any pending requests tagged to its configured
   role, and silently begins execution in the background.

## Adversarial Review & Mitigations

An automated peer review of this architecture identified several critical
flaws that must be addressed before v0.05 implementation:

1. **The Merge Conflict Catastrophe:** Git does not magically resolve semantic
   conflicts. If two agents edit `design.md` simultaneously, the system will
   halt helplessly.
   - *Mitigation:* We must reintroduce a lightweight, distributed locking
     mechanism (akin to Gastown's "beads") maintained in the local SQLite
     engine. Agents must "checkout" a conceptual lock before beginning file
     writes avoiding concurrent sandbox drift.
1. **GitHub API is not a High-Throughput Message Bus:** Polling `gh pr list`
   across a massive multi-agent swarm will instantly exhaust rate limits and
   introduce multi-minute latencies per turn.
   - *Mitigation:* The local Swarm SQLite `SessionService` remains the primary
     low-latency event bus for *local* coordination. The GitHub API is
     strictly reserved for *external* integrations, human-in-the-loop
     handoffs, or asynchronous macro-tasks triggered via native Webhooks.
1. **`ROLES.md` is a Security Landmine:** Dynamically executing plaintext role
   definitions from the repository introduces a trivial prompt-injection
   vector for malicious code.
   - *Mitigation:* All `.md` configuration files (`ROLES.md`, `AGENTS.md`)
     must be cryptographically signed or explicitly whitelisted by the human
     *Mayor* (the executing user) within the local `.config/swarm` directory
     before the Engine trusts their directives.
1. **The "Hidden" Orchestrator State:** Relying purely on PRs to loosely track
   a project loses the "macro-task" state machine, making debugging impossible
   when a task stalls.
   - *Mitigation:* The Concierge must retain a durable, centralized State
     Machine (DAG) locally. It maps an abstract task ("Write the design doc")
     to its concrete manifestations (PR #4, Issue #12), preserving the overall
     narrative graph.

## Conclusion

By outsourcing final artifact delivery to GitHub, Swarm speaks the universal
protocol of version control. However, by retaining the critical lessons of
security, latency, and state concurrency locally, we bridge the gap between
deterministic local tinkering and a massive, global, multi-model agent
workforce safely.
