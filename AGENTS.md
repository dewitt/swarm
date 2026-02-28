# Agent Collaboration Guide (AGENTS.md)

**Welcome, AI Agent.**

If you are reading this file, you have been assigned to work on the `swarm`
project. This document serves as your primary context, architectural
constraint guide, and communication protocol. Whether you are an instance of
the Gemini CLI, Claude Code, Cursor, or the `swarm` CLI itself, you must
treat this document as the highest-priority operational directive.

______________________________________________________________________

## 1. Project Vision & What This Repository Is

The `swarm` project is a framework-agnostic, Go-based CLI and embeddable SDK
designed to help developers natively manage, build, test, and deploy AI agents
from their terminal.

**Core Philosophy:** "One agent alone is never enough."

The CLI does not rely on a monolithic system prompt. Instead, it utilizes the
Google Agent Development Kit (ADK) to instantiate an internal swarm of
specialized agents (a Router, a GitOps agent, a Builder, etc.). This makes the
architecture highly modular, testable, and capable of adapting to complex user
requests.

______________________________________________________________________

## 2. Architectural Constraints

When writing or modifying code in this repository, you **must** adhere to the
following rules:

1. **The Delegation Hierarchy (Thin Software, Fat Models):** We write as
   little custom code as possible. When implementing a feature, you must
   evaluate solutions in this strict order:
   - 1. Can the raw model do it?
   - 2. Can a dynamic Markdown Skill (e.g., `skills/`) do it?
   - 3. Does the Google ADK provide it natively?
   - 4. *Only if all else fails*, write custom Go code for it.
1. **Separation of Concerns:** The CLI (Presentation Layer) and the SDK
   (Business Logic) must be strictly decoupled. The CLI (`cmd/swarm/`) is
   merely a consumer of the SDK. Do not leak terminal UI logic (e.g.,
   Bubbletea components, ANSI codes) into the core SDK (`pkg/sdk/`).
1. **Go Standards:** The project is written in Go. You must use idiomatic Go
   conventions, ensure the SDK is compatible with `cgo` and WebAssembly
   (`wasm`), and provide comprehensive Go unit tests for any new logic.
1. **Google ADK First:** The internal business logic of the CLI is powered by
   the Google Agent Development Kit (ADK) for Go.
1. **Native CI/CD Integration:** Deployments and environment changes should be
   implemented via standard CI/CD files (e.g., committing GitHub Actions)
   rather than proprietary API calls whenever possible.
1. **Dynamic Skills Architecture:** If a feature involves a specific cloud
   provider or an external framework (like LangGraph or Claude Code), it
   should be built as a dynamically loaded Markdown "Skill" in the `skills/`
   directory, not hardcoded into the Go binary.

______________________________________________________________________

## 3. Repository Structure

- `/cmd/swarm/`: The entry point for the CLI binary (Cobra, Bubble Tea TUI).
- `/pkg/sdk/`: The embeddable Go SDK. Contains the AgentManager, session
  management (SQLite), and the ADK Router logic.
- `/docs/design/`: High-level and detailed architectural documents. **You must
  read relevant documents here before making architectural changes.**
- `/docs/cuj/`: Critical User Journeys. If you change a workflow, you must
  update or add a CUJ here.
- `/skills/`: The dynamic Markdown-based skills that teach the core Router
  agent new capabilities (e.g., how to scaffold ADK projects, how to wrap
  other CLIs like `gemini-cli` or `claude-code`).

______________________________________________________________________

## 4. How to Find Work & Contribute

If you have been summoned to this repository without a specific task, or if
you have completed your current assignment and are looking for what to do
next, follow this procedure:

### Step 1: Check the Roadmap and Backlog

1. Read `TODO.md` in the root directory. This file tracks immediate technical
   debt and the active feature backlog.
1. Read `docs/design/06-implementation-roadmap.md`. This document outlines the
   broader phases of the project. Determine which phase is currently active.

### Step 2: Propose a Plan

Once you identify an actionable unit of work from the `TODO.md` or the
Roadmap:

1. Synthesize a plan of action.
1. Share this plan with the human developer for approval *before* you begin
   executing file changes.

### Step 3: Implement, Test, and Verify (Zero-HITL)

Agents must respect the human developer's time and attention.
Human-In-The-Loop (HITL) should *only* be required for permissions or creative
opinions.

1. **Mechanical Verification is Autonomous:** You must never ask a human to
   run a binary just to verify if it compiled correctly.
3. **Execute Tests:** You must utilize headless testing, `go test ./...`, and
   Bubble Tea state verification to verify your own work autonomously.
4. **UI Regression Testing:** Whenever you modify code that impacts the text
   entry UI, viewport, or main terminal layout, you **MUST** run the UI
   regression tests (e.g., `vhs tests/ui/text_entry.tape`) to verify visual
   stability. See `tests/ui/README.md` for instructions.
5. **Format:** All markdown files must be formatted using `mdformat --wrap 78`
   before being committed.

### Step 4: Asynchronous Handoffs

Because this project is built by multiple agents asynchronously, you must
leave a clean trail for the next agent:

1. **Document Your Intent:** Before closing your session, ensure any
   uncompleted architectural decisions or roadblocks are documented in
   `TODO.md` or a design doc.
1. **State Your Context:** When proposing a commit, clearly state what CUJ or
   design document your changes satisfy in the commit message.

______________________________________________________________________

By adhering to this guide, you ensure that the `swarm` codebase remains an
industry-standard example of clean, multi-agent software engineering. Good
luck!
