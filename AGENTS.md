# Agent Collaboration Guide (AGENTS.md)

**Welcome, AI Agent.**

If you are reading this file, you have been assigned to work on the `swarm`
project. This document serves as your primary context, architectural
constraint guide, and communication protocol. Whether you are an instance of
the Gemini CLI, Claude Code, Cursor, or the `swarm` CLI itself, you must treat
this document as the highest-priority operational directive.

______________________________________________________________________

## 1. Project Vision & The Swarm Operator Paradigm

The `swarm` project is a framework-agnostic, Go-based CLI and embeddable SDK
designed to help developers natively manage, build, test, and deploy AI agents
from their terminal.

**Core Philosophy:** "One agent alone is never enough."

We are moving towards the **Swarm Operator Paradigm**. The user ceases to be a
simple pair-programmer and instead assumes the role of a Swarm Operator
overseeing a highly skilled, infinitely scalable virtual workforce. The system
must be intelligent enough to autonomously decompose tasks, dynamically
provision specialized agents, and coordinate their parallel efforts.

The CLI does not rely on a monolithic system prompt. Instead, it utilizes the
Google Agent Development Kit (ADK) to instantiate an internal swarm of
specialized agents (Router, GitOps, Builder, Observers, Planners, etc.).

______________________________________________________________________

## 2. Architectural Constraints & Philosophy

When writing or modifying code in this repository, you **must** adhere to the
following rules:

1. **Defer to the Frontier (Thin Software, Fat Models):** We write as little
   custom code as possible. When implementing a feature, you must evaluate
   solutions in this strict order:
   - 1. **Model:** Can the raw frontier LLM solve this natively?
   - 2. **Skills:** Can a dynamic Markdown Skill (e.g., `skills/`) do it?
   - 3. **Framework:** Does the Google ADK provide it natively?
   - 4. **Code:** *Only if all else fails*, write custom Go code for it.
1. **Strict Separation of Concerns:** The CLI (Presentation Layer) and the SDK
   (Business Logic) must be strictly decoupled. The TUI (`cmd/swarm/`) is
   merely a "dumb" client consuming standardized events from the embeddable
   `pkg/sdk/` backend.
1. **World-Class CLI UX & Familiarity:** The UI must be highly interactive and
   visually beautiful, mirroring established market leaders (Gemini CLI,
   Cursor, Claude Code). Innovate only with conviction.
1. **Ubiquitous Mediation:** Every interaction is mediated. Assume the
   presence of **Input Agents** (preprocessing) and **Output Agents**
   (synthesis).
1. **Dynamic Replanning & Checks:** "No plan survives first contact."
   Execution nodes (agents/tools) must dynamically replan and provide upward
   feedback on failure. **Observer Agents** monitor the byzantine swarm for
   deviations. When in doubt, fall back on the smartest models rather than
   rigid heuristics.
1. **Native Ecosystems:** Version control (Git) is the absolute source of
   truth. Deployments should be standard CI/CD pipelines (e.g., GitHub
   Actions), not proprietary APIs.
1. **Go Standards:** Use idiomatic Go conventions, ensure `cgo` and `wasm`
   compatibility, and provide comprehensive unit tests. Run `go vet` and
   `go fmt` consistently.

______________________________________________________________________

## 3. Hierarchical Memory System

Swarm utilizes a structured 4-Tier Memory model to prevent context window
blowout and ensure facts are preserved across sessions. You MUST utilize these
tools to shape the context window actively.

1. **Tier 1: Working Memory (Context Isolation):** Sub-agents (like
   `codebase_investigator`) run in isolated context bubbles. They do not see
   the global chat history. When you are a sub-agent, use your tools (like
   `bash_execute`) to find answers, and return a dense summary. Do NOT dump
   raw bash output into your final response unless explicitly asked, as this
   poisons the orchestrator's context.
1. **Tier 2: Episodic Memory (Session History):** The linear timeline of the
   current chat.
1. **Tier 3: Semantic Memory (Project Facts):** This is a project-scoped
   embedded SQLite database (FTS5). If you discover a critical, non-obvious
   fact about the architecture (e.g., "The build command is X", or "This tool
   is failing due to Y"), you **MUST** use the `commit_fact` tool to
   permanently save it. Future Swarm planners will automatically query this
   database using `retrieve_fact` before they act, preventing repetitive
   mistakes.
1. **Tier 4: Global Memory:** User preferences across all projects (managed
   via the `/remember` command and `.gemini/GEMINI.md` overrides).

______________________________________________________________________

## 4. Repository Structure

- `/cmd/swarm/`: The entry point for the CLI binary (Cobra, Bubble Tea TUI).
- `/pkg/sdk/`: The embeddable Go SDK. Contains the Swarm, session management
  (SQLite), and the ADK Swarm logic.
- `/docs/design/`: High-level and detailed architectural documents. **You must
  read relevant documents here before making architectural changes.**
- `/docs/cuj/`: Critical User Journeys. If you change a workflow, you must
  update or add a CUJ here.
- `/skills/`: The dynamic Markdown-based skills that teach the core Swarm
  agent new capabilities (e.g., how to scaffold ADK projects, how to wrap
  other CLIs).
- `/eval/` and `/pkg/eval/`: Agentic testing and evaluation framework.

______________________________________________________________________

## 5. How to Find Work ## 4. How to Find Work & Contribute Contribute

If you have been summoned to this repository without a specific task, or if
you have completed your current assignment and are looking for what to do
next, follow this procedure:

### Step 1: Check the Roadmap and Backlog

1. Read `TODO.md` in the root directory. This tracks immediate technical debt
   and the active feature backlog.
1. Read `docs/design/06-implementation-roadmap.md`. Determine which phase is
   currently active.

### Step 2: Propose a Plan

Once you identify an actionable unit of work:

1. Synthesize a plan of action.
1. Share this plan with the human developer for approval *before* executing
   file changes.

### Step 3: Implement, Test, and Verify (Zero-HITL)

Agents must respect the human developer's time and attention.
Human-In-The-Loop (HITL) should *only* be required for permissions or creative
opinions.

1. **Mechanical Verification is Autonomous:** Never ask a human to run a
   binary just to verify if it compiled correctly.
1. **Execute Tests:** Utilize headless testing, `go test ./...`, and Bubble
   Tea state verification to verify your work autonomously. Every time a swarm
   eval test is run, ensure the resulting trajectories are persisted to a
   durable location for subsequent review.
1. **UI Regression Testing:** If modifying the text entry UI, viewport, or
   layout, you **MUST** run the UI regression tests (e.g.,
   `vhs tests/ui/text_entry.tape`) to verify visual stability. See
   `tests/ui/README.md`.
1. **Format & Styling Guidelines:**
   - All markdown files in this repository (except for `SKILL.md` files) must
     be formatted using `mdformat --wrap 78` before being committed.
   - **NEVER** run `mdformat` on `SKILL.md` files (found in subdirectories of
     `skills/`). These files contain YAML frontmatter that is critical for the
     Swarm SDK to parse agent metadata, and `mdformat` will corrupt this
     structure.
   - Quotation marks must be used exclusively for direct, verbatim quotes from
     sources or specific UI elements. Avoid using quotation marks for
     emphasis, technical terminology, neologisms, or novel phrases.
1. **File GitHub Issues:** Use the GitHub CLI (`gh issue create`) to file new
   issues for bugs, technical debt, or feature enhancements discovered.

### Step 4: Asynchronous Handoffs

Because this project is built by multiple agents asynchronously, leave a clean
trail for the next agent:

1. **Document Your Intent:** Ensure uncompleted decisions or roadblocks are
   documented in `TODO.md` or a design doc.
1. **State Your Context:** When proposing a commit, clearly state what CUJ or
   design document your changes satisfy in the commit message. Note that
   commit messages should not use a 'word:' prefix (e.g., 'feat:', 'docs:')
   and should focus on "why" rather than "what".

## 6. Specialized Advocate Agents

The Swarm ecosystem includes specialized sub-agents designed to perform
rigorous, persona-driven reviews of the project. These agents are defined in
the `skills/` directory and can be invoked directly to audit the codebase,
user experience, or agentic performance.

To utilize these advocates, you can explicitly instruct an agent (like Gemini
CLI, Claude Code, or the Swarm CLI itself) to adopt their persona:

- **`@code_review`**: Invokes the Code Quality Advocate
  (`skills/code_review/SKILL.md`). Use this for comprehensive
  codebase reviews, identifying architectural flaws, and enforcing idiomatic
  design.
  - *Example:* "As the @code_review, audit the `pkg/sdk/` package
    for race conditions and unhandled errors."
- **`@ux_review`**: Invokes the User Advocate
  (`skills/ux_review/SKILL.md`). Use this for user-centric UX evaluations,
  hunting friction points, and polishing terminal and web interfaces.
  - *Example:* "@ux_review review the splash screen discoverability and
    propose layout improvements."
- **`@quality_review`**: Invokes the Agentic Quality Advocate
  (`skills/quality_review/SKILL.md`). Use this to evaluate
  trajectory efficiency, tool-use rigor, orchestration handoffs, and
  LLM-as-a-Judge grading rubrics.
  - *Example:* "Run an agentic quality audit using the
    @quality_review skill on the latest trajectory logs in
    `scenario_3`."

When instructed to act as one of these advocates, you MUST strictly adhere to
the workflow and reporting formats defined in their respective `SKILL.md`
files, ensuring all findings are properly deduplicated and logged in their
corresponding `docs/*_ISSUES.md` backlogs.

______________________________________________________________________

By adhering to this guide, you ensure that the `swarm` codebase remains an
industry-standard example of clean, multi-agent software engineering. Good
luck!
