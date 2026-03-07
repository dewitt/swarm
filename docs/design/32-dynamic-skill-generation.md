# Design: Dynamic Skill Generation and Execution

*Resolves a key initiative in Epic #29 (The Self-Healing Swarm).*

## 1. Context & Motivation

As outlined in Epic #29 (The Self-Healing Swarm), Swarm currently relies
heavily on pre-configured Markdown skills (e.g., `skills/git/SKILL.md`) to
define the capabilities of its agents. While effective, this creates a rigid
boundary: if a user asks Swarm to perform a task for which it has no skill
(e.g., "deploy this to Vercel" or "compile this Rust project" when those tools
aren't explicitly taught), Swarm must rely on generic tools like
`bash_execute` or `web_researcher`.

Often, this leads to thrashing. The model might fail, request a replan, try a
different bash command, fail again, and eventually require human-in-the-loop
(HITL) intervention.

**The Vision:** We want Swarm to be "self-healing" and continuously learning.
If Swarm encounters a complex, repeatable task it doesn't know how to do, it
should possess the meta-ability to:

1. Search for existing community skills.
1. Dynamically *write* a new `SKILL.md` file for itself.
1. Hot-reload the new skill into its current context.
1. Execute the task using the newly acquired capability.

## 2. Architecture & Philosophy

This capability leans heavily into the "Thin Software, Fat Models" philosophy
defined in `AGENTS.md`. We do not want to hardcode complex skill-fetching APIs
or registry logic in Go. Instead, we want to empower the agents to use their
existing tools (file writing, bash execution, web searching) to bootstrap
themselves.

### 2.1. The `skill-creator` Agent

We already have a nascent `skill-creator` agent defined in
`skills/skill-creator/SKILL.md`. Its current job is to "help users expand the
capabilities of Swarm."

We need to elevate this agent from a passive assistant to an active
participant in the automated routing loop.

### 2.2. The Trigger Mechanism (Routing)

Currently, the `routing_agent` (or `swarm` agent) classifies requests and
delegates them to known specialists. If no specialist exists, it falls back to
generic planning.

We will update the core routing instruction so that if the agent identifies a
missing capability that is likely to be a discrete, repeatable workflow (e.g.,
"interacting with AWS", "deploying to Fly.io", "managing a PostgreSQL
database"), it will proactively route the task to the `skill-creator` to
generate the skill first.

### 2.3. The Skill Generation Loop

When the `skill-creator` is invoked to fulfill a missing capability:

1. **Research:** It uses `web_researcher` to look up the best practices or CLI
   commands for the requested technology.
1. **Draft:** It uses `write_local_file` to create a new `SKILL.md` in the
   `.gemini/skills/` directory (or the project's local skills folder).
1. **Notify:** It informs the `swarm` orchestrator that the new skill is
   ready.
1. **Reload:** The `swarm` CLI automatically detects the new file (or we
   implement a `/reload` command trigger) and injects the new specialist into
   the active session.
1. **Execute:** The original task is then routed to the newly created agent.

## 3. Implementation Plan

### Phase 1: Empowering the Skill Creator

1. **Update `skills/skill-creator/SKILL.md`:** Refine the prompt to ensure it
   generates high-quality, production-ready skills that strictly adhere to the
   `agentskills.io` specification. It must know to place these skills in
   `.gemini/skills/` (or the appropriate configured directory).
1. **Add Web Research:** Give the `skill-creator` access to the
   `web_researcher` tool so it doesn't hallucinate API usages or CLI flags
   when writing the new skill.

### Phase 2: Updating the Orchestrator

1. **Update `skills/routing-agent/SKILL.md`:** Add an explicit instruction:
   *If the user requests a complex technical workflow that no existing agent
   can handle, route the request to the `skill-creator` agent to generate a
   new specialized skill for it first.*
1. **Update `pkg/sdk/swarm.go`:** Ensure the SDK watches the skills directory
   for changes and hot-reloads the agent registry, or implement a mechanism
   where the `skill-creator` can trigger a reload event upon successfully
   writing a file.

### Phase 3: Community Discovery (Future)

Eventually, the `skill-creator` will not just write skills from scratch. It
will query a public registry (e.g., a GitHub repository or `agentskills.io`
index) to download community-vetted skills before attempting to generate its
own.

## 4. Expected Outcome

By implementing this, we reduce the burden on the user to manually configure
the swarm. The swarm becomes an adaptive entity that expands its own
capabilities in real-time to meet the demands of the user's workflow,
significantly reducing HITL friction.
