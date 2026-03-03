# Design Doc 25: Universal Agent Observability 

**Status**: Proposed
**Author**: Antigravity
**Date**: March 2026

## Objective

To define Swarm's positioning not as a competitor to specialized agentic CLIs (like Claude Code, Gemini CLI, or Codex), but as the **Orchestration and Observability Plane** that sits *above* them. 

This document outlines how Swarm will pragmatically capture telemetry from third-party ecosystems to unify their execution contexts, providing human operators with a single, highly-observable dashboard (The Agent Panel) and a centralized SQLite trajectory ledger.

## The Core Value Proposition: Swarm as the "DataDog for Agents"

A massive engineering organization will not standardize on a single LLM or a single coding agent. Different teams will use different tools optimized for their specific tech stacks.

If a developer uses Swarm to orchestrate a complex system migration involving three different agent architectures, the hardest problem isn't triggering the remote CLIs; the hardest problem is piercing the "black box" of those tools to understand:
1. What are they doing right now?
2. Are they stuck in a loop?
3. What context did they gather that directly led to their final artifact?

Swarm's primary value unlocks when it bridges these silos.

## Architectural Approach: Pragmatic Adaptation, Not Standardization

We initially considered proposing a universal "Agentic Telemetry Protocol." However, forcing massive, fast-moving ecosystems to adopt a new inter-process communication standard is a losing battle. 

Instead, **Swarm will aggressively adapt to whatever telemetry we can get our hands on today.**

We will rely on Swarm's dynamic `SKILL.md` architecture to act as the adapter layer. Each third-party CLI will have an associated Swarm Skill that defines not only how to *invoke* the tool, but how to *parse* its native, idiosyncratic output stream into Swarm's standardized `ObservableEvent`.

### Mechanism 1: Native Debug Flags & Structured Logs

Most CLI tools support a level of verbosity for their own developers.
- **Example:** If `gemini-cli` supports a `--json-log=/tmp/gemini.log` flag, the `gemini-cli-skill` will aggressively append this flag to the `bash_execute` command.
- A secondary, lightweight Swarm Observer goroutine will `tail -f /tmp/gemini.log`, parsing the native JSON lines and translating them into Swarm `Span` updates (e.g., marking tools as started/finished in the Agent Panel).

### Mechanism 2: ANSI Stdout Scraping & The Semantic Observer

When structured logs are unavailable (or undocumented), Swarm must rely on scraping the raw terminal multiplexer output.

As we established in the Agent Panel UI, Swarm already captures the `stdout` stream of its sub-processes.
- The `claude-code-skill` might include instructions for Swarm's internal "Semantic Observer" (a lightweight Gemini model running in parallel).
- The Observer watches the raw ANSI string stream and synthesizes human-readable statuses (e.g., *"Claude is reading `utils.go`"*).
- This approach is entirely decoupled from the third-party tool's internals; it relies only on what the tool presents to the human.

### Mechanism 3: Post-Mortem SQLite/State Extraction

If an agent (like ADK Python) uses its own local SQLite database for session history, Swarm can treat that database as a target.
- The Skill definition instructs Swarm to wait for the sub-agent to terminate. 
- Swarm then opens the sub-agent's isolated `.db` file, extracts the newly created trajectories, and merges them into Swarm's global unified SQLite ledger.
- This ensures that if the system fails, Swarm's "Self-Healing" trajectory analysis has the complete context of *why* the sub-agent made the decisions it did, completely fulfilling `docs/design/20-trajectory-collection.md`.

## Implementation Strategy (v0.05 Target)

1. **Skill Telemetry Declarations:** Extend the `SkillLoader` to optionally parse a `telemetry_adapter` block in a Skill's manifest. This block defines the strategy (e.g., `type: stdout_scrape`, `type: log_tail`).
2. **The Output Normalizer:** Build an intermediate SDK interface (`TelemetrySink`) that accepts raw byte streams or JSON blobs from these adapters and maps them to Swarm's internal `Span` and `Trajectory` schemas.
3. **Reference Implementations:** Create official Swarm Skills for `claude-code` and `gemini-cli` to prove the viability of scraping and adapting their native observability footprints.

## Adversarial Review & Mitigations

An adversarial review of this design (simulating Claude 3.5 Sonnet) highlighted critical systemic flaws with the "Pragmatic Adaptation" approach:

1. **The Brittleness of Stdout Scraping:** UI text is not an API contract. If a tool changes its status string from "Reading file..." to "Analyzing context...", our Semantic Observer parsers break immediately.
   * *Mitigation:* We accept this brittleness as a known cost of the adapter pattern. To mitigate, Skills must define `version_constraints`. A `claude-code-skill` that scrapes `stdout` will strictly pin to `claude-code v0.1.X`. When the upstream tool updates, the Skill must be explicitly re-tested and version-bumped within Swarm.
2. **The Cost and Latency of the Semantic Observer:** Running a secondary LLM to watch a primary LLM doubles inference costs and introduces lag into the UI.
   * *Mitigation:* The Semantic Observer (`fastModel`) will be heavily debounced and rate-limited. It will only sample the stream every 3-5 seconds, rather than processing every emitted token, drastically reducing token burn.
3. **Violating SQLite Encapsulation:** Reading a third-party tool's internal database is a dangerous vector for schema drift, file locks, and data corruption.
   * *Mitigation:* We will abandon direct SQLite file extraction for third-party CLIs. If a tool does not provide a native export command (e.g., `gemini export-session`), Swarm will not attempt to manually scrape its private database files. We will rely purely on the observable execution graph (standard I/O).

## Conclusion

By treating third-party agents as highly capable, "black-box" primitives and using version-pinned Skills to pry those boxes open via pragmatism (scraping and logging), Swarm circumvents the need for global standards. It positions itself as the essential management layer for the multi-model future.
