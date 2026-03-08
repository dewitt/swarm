# Design: Tool Reliability Scoring & Flake Memory

*Resolves an initiative in Epic #29 (The Self-Healing Swarm).*

## 1. Context & Motivation

As Swarm evolves into a massively multi-agent orchestrator, it relies heavily
on delegating sophisticated tasks to specialized external tools and agent
wrappers (e.g., `codex`, `claude-code`, `gemini-cli`).

However, these external tools are inherently volatile:

- They require active API subscriptions that run out of credits.
- They suffer from vendor rate limits (429s).
- The underlying service may experience downtime.
- A local binary might not be installed or configured correctly.

Currently, Swarm treats every new turn as a blank slate. If the `codex` skill
fails because it's out of credits today, Swarm will stubbornly attempt to use
it again on the very next prompt, failing recursively until Overwatch or a
human intervenes.

**The Vision:** Swarm needs a persistent "Flake Memory." It should keep track
of tool reliability across sessions. If an external skill is fundamentally
broken or consistently failing, Swarm should dynamically deprecate it, alert
the user, and automatically fall back to the next best alternative (e.g.,
using `bash_execute` or a different model) without wasting time and tokens on
guaranteed failures.

## 2. Architecture & Philosophy

This aligns with the **Dynamic Replanning & Checks** philosophy. We must rely
on stateful telemetry to guide future routing decisions.

### 2.1. The Persistence Layer (SQLite)

We already have a persistent `session.Service` backed by SQLite
(`~/.config/swarm/swarm.db`). We will add a new table: `tool_reliability`.

Schema Concept:

- `tool_name` (e.g., `codex`)
- `consecutive_failures` (int)
- `last_failed_at` (timestamp)
- `last_error_signature` (string snippet of the error, e.g., "credit balance
  too low")
- `status` (enum: `healthy`, `degraded`, `offline`)

### 2.2. The Feedback Loop

1. **Execution Phase:** When `pkg/sdk/engine.go` marks a span as failed, it
   will update the `tool_reliability` table, incrementing the failure count
   for that tool. If a tool succeeds, the counter resets.
1. **Deprecation Phase:** If a tool hits a specific threshold (e.g., 3
   consecutive failures), its status is marked as `offline`.
1. **Routing Phase:** When the SDK initializes the `routing_agent` (and
   `planning_agent`), it queries the database for `offline` tools. It injects
   a dense system prompt modifier into the Router's context: *"WARNING: The
   `codex` specialist is currently OFFLINE due to repeated errors ('credit
   balance too low'). DO NOT delegate tasks to `codex`. You must find an
   alternative path."*

### 2.3. The Healing Phase

Tools should not stay deprecated forever. A "cooldown" period should apply. If
a tool has been offline for >24 hours (or if the user explicitly types
`/reset-tools`), Swarm should lift the embargo and allow the router to
cautiously attempt delegation again.

## 3. Implementation Plan

### Phase 1: Database Schema & Tracking

1. Create a lightweight `ReliabilityTracker` struct within the SDK that
   interfaces with the SQLite DB.
1. Hook into `Engine.MarkFailed()` and `Engine.MarkComplete()` to update the
   tracking metrics.
1. Establish the threshold logic (e.g., 3 consecutive failures = `offline` for
   12 hours).

### Phase 2: Dynamic Router Injection

1. Update `pkg/sdk/swarm.go`'s `Reload()` method (which constructs the agent
   contexts).
1. Before building the `AVAILABLE SPECIALISTS` list, query the
   `ReliabilityTracker`.
1. Append any generated warning strings (the embargo context) directly into
   the `routingInstruction`.

### Phase 3: TUI Visibility

1. Update the TUI (the "Skills" count in the boot block or the `/skills`
   modal) to visibly indicate if a skill has been temporarily disabled by the
   system, ensuring the user understands why Swarm is choosing alternative
   paths.

## 4. Expected Outcome

Swarm becomes "street smart." If a user's OpenAI API key expires, Swarm will
try it, realize the failure is systemic, remember it, and immediately pivot to
using Claude or Gemini for all subsequent coding tasks across all sessions
until the cooldown period expires or the user intervenes.
