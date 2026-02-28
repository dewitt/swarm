# Project Philosophy (PHILOSOPHY.md)

This document outlines the core beliefs and guiding principles of the `swarm`
project. When faced with an architectural decision, always favor the path that
aligns closest to these ideals.

## 1. Defer to the Frontier (Thin Software, Fat Models)

Foundational, frontier LLMs are improving at a rate faster than traditional
software built to orchestrate them possibly can. Therefore, our absolute
highest priority is to write as little custom code as possible.

When designing a new feature, you **must** attempt to resolve it in this
strict order of precedence:

1. **Delegate to the Model:** Can the raw frontier LLM solve this inherently
   just by asking it nicely? If yes, stop here.
1. **Delegate to Dynamic Skills:** If the model needs specialized context or a
   workflow, can it be provided via a plain-text, dynamically loaded Markdown
   Skill? If yes, build a Skill.
1. **Delegate to the Framework (ADK):** If programmatic logic or orchestration
   is truly required, does the Google Agent Development Kit (ADK) provide a
   native primitive for it? If yes, use the ADK.
1. **Write Custom Code (Last Resort):** Only write custom Go code in this CLI
   if it is strictly for terminal UI presentation or if all the above options
   have been entirely exhausted.

## 2. One Agent Alone is Never Enough

Single-agent architectures are fragile. Real-world, complex tasks require
specialized context, debate, and iterative verification loops.

- The system must natively assume and support multi-agent collaboration
  (Swarms, Supervisor-Worker patterns, Debate teams).
- Every problem should be approached by asking: *"Can we split this task among
  specialized sub-agents?"*

## 3. Zero-HITL (Human-In-The-Loop) for Verification

Agents must respect the human developer's time and attention.
Human-In-The-Loop should *only* be required for permissions (e.g., "Can I push
to `main`?") or creative opinions (e.g., "Do you like this UI layout?").

- **Mechanical Verification is Autonomous:** An agent must never ask a human
  to run a binary just to verify if it compiled correctly or to describe what
  the UI looks like.
- Agents must utilize headless testing, unit tests, and snapshot testing to
  verify their own work autonomously.

## 4. Native Ecosystems are the Source of Truth

We believe in playing well with the customer's existing ecosystem rather than
imposing proprietary deployment engines.

- Version control (Git) is the absolute source of truth for both code and
  infrastructure.
- "Deploying" an agent means scaffolding standard CI/CD pipelines (e.g.,
  GitHub Actions) and committing them to the repository, ensuring every change
  is versioned, auditable, and easily reversible.

## 5. World-Class CLI UX

A command-line tool for orchestrating AI should feel as magical and polished
as the AI it commands.

- We hold ourselves to the standard of tools like Gemini CLI, Claude Code, and
  Codex.
- The UI must be highly interactive, visually beautiful (rich text, colors,
  ephemeral spinners), and completely hide the mechanical complexity of the
  underlying LLM calls.

## 6. Architectural Separation of Concerns

We strictly enforce the boundary between the presentation layer and the
underlying intelligence.

- **UI vs. SDK:** The terminal UI (`cmd/swarm/`) must remain a "dumb" client.
  It handles rendering, input capture, and local configuration logic, but it
  MUST NOT contain LLM prompting logic, system instructions, or tool
  implementations. All intelligence, session management, and orchestration
  belong exclusively in the embeddable `pkg/sdk/` backend.
- **Modular Feature Partitioning:** New capabilities should be as
  self-contained as possible. Instead of creating monolithic "god classes,"
  favor registering new, scoped tools, or building independent sub-agents that
  communicate via standard interfaces.

## 7. UX Familiarity and Innovation

We believe in minimizing friction for developers transitioning between
different tools in the ecosystem.

- **Follow Market Leaders:** When implementing standard features (like slash
  commands, file referencing, or context management), we default to the UX
  patterns established by market leaders (e.g., Cursor, Claude Code, Gemini
  CLI, Codex). If a user knows how to use Claude Code, they should
  instinctively know how to use the `swarm` CLI.
- **Innovate with Conviction:** We only break from established UX norms when
  no standard exists, or when we have deep conviction that our novel approach
  represents a significant leap forward and is poised to become the new
  industry standard. We do not invent new paradigms just to be different.

## 8. Ubiquitous Mediation

We believe that, at the limit, every interaction—whether human-to-computer,
computer-to-computer, or computer-to-human—will be mediated by an
intelligent, autonomous agent acting on our behalf.

- **The Agent as the Interface:** The user no longer interacts directly with
  raw tools or rigid UIs. Instead, they interact with a coordinating
  intelligence that translates intent into action.
- **Proactive Conversation Management:** Our "Chat Input Agent" (CIA) is a
  first-class realization of this principle. By using an agent to proactively
  classify and route inputs, we ensure that the system remains fluid and
  aligned with human thought patterns, even when they digress.
- **Intelligent Output Synthesis:** Just as inputs are mediated, computer-to-human
  outputs should be synthesized by an agent to ensure they are communicated in
  the most effective format, tone, and language for the specific user and
  context (e.g., real-time translation or level-of-detail adjustment).
- **Orchestration by Default:** We assume that every component of a system
  will eventually be accessible through a mediating agentic layer, reducing
  complex technical workflows to high-level goal-setting.

## The "Engineering Manager" Paradigm

The ultimate goal of `swarm` is to abstract away the mechanics of agent
orchestration. A user should be able to type `$ swarm` and give an
arbitrarily complex directive (e.g., *"Migrate our billing service from Python
to Go"*).

The system must be intelligent enough to autonomously decompose the task,
dynamically provision the exact number of specialized agents required (one or
one hundred), and coordinate their parallel efforts to completion. In this
paradigm, the user ceases to be a pair-programmer and instead assumes the role
of an Engineering Manager overseeing a highly skilled, infinitely scalable
virtual workforce. The CLI's UI must reflect this shift by providing
high-level observability and steering mechanisms (Agent Panel), rather
than just a linear chat log.

### UI is Just a Consumer

Crucially, the "Engineering Manager" paradigm is a property of the *core SDK*,
not the Terminal UI. The complex logic of provisioning sub-agents, routing
tasks, and generating observer telemetry must be strictly encapsulated within
the embeddable Go library. The TUI is simply a thin presentation layer that
consumes these standardized events. This ensures that the same powerful swarm
orchestration can be seamlessly embedded into web-based Agent Panels, VS Code
extensions, or Slack bots without rewriting any business logic.
