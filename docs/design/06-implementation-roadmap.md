# Implementation Roadmap

This document sequences the detailed design and implementation phases for the
`swarm` project. Future AI agents should consult this roadmap to determine
the next actionable unit of work.

## Phase 1: Foundational CLI & SDK Shell

**Goal:** Establish the Go module, the basic command structure, and the
separation between the CLI and the SDK.

- **Task 1.1:** Initialize the Go module (`go.mod`) and configure the project
  directory structure (`cmd/swarm`, `pkg/sdk`).
- **Task 1.2:** Implement the basic CLI routing using a modern Go CLI
  framework (e.g., `cobra` or `bubbletea` for the interactive TUI shell).
- **Task 1.3:** Define the core SDK interfaces in `pkg/sdk/` (e.g.,
  `AgentManager`, `SkillLoader`). These should be empty stubs with thorough
  godoc comments explaining their intent.
- **Task 1.4:** Setup the testing harness. Ensure `go test ./...` passes.

## Phase 2: ADK Integration & The Internal Router

**Goal:** Hook up the Google Agent Development Kit (ADK) for Go to power the
CLI's internal "Router Agent".

- **Task 2.1:** Implement the `LlmAgent` from the Go ADK within the SDK's
  core.
- **Task 2.2:** Build the interactive chat loop in the CLI that sends user
  input to the internal Router Agent and streams responses back to the
  terminal.
- **Task 2.3:** Implement basic Tool execution for the Router Agent (e.g., a
  simple `list_local_files` tool) to prove the ADK tool-calling loop works.
- **Task 2.4:** Write integration tests mocking the LLM endpoint to verify the
  Router Agent parses user intents correctly.

## Phase 3: Manifests & The Builder Agent

**Goal:** Enable the CLI to understand and scaffold user-defined agents.

- **Task 3.0:** Implement the Client-Side Slash Command Router (`/help`,
  `/clear`) to intercept local commands before they are sent to the LLM.
- **Task 3.1:** Define the schema for `agent.yaml`.
- **Task 3.2:** Implement the SDK logic to parse and validate `agent.yaml`
  files.
- **Task 3.3:** Instantiate the internal "Builder Agent". Give it tools to
  read the `agent.yaml` and scaffold boilerplates for different frameworks
  (starting with ADK Python).
- **Task 3.4:** Ensure this workflow satisfies
  `docs/cuj/01-build-local-adk-python-agent.md`.

## Phase 4: GitOps & Deployment Scaffolding

**Goal:** Implement the "GitOps Agent" that translates deployment intents into
CI/CD configurations.

- **Task 4.1:** Implement generic Git execution tools within the SDK (e.g.,
  wrapping `git commit`, `git push`, or using `go-git`).
- **Task 4.2:** Instantiate the internal "GitOps Agent".
- **Task 4.3:** Provide the GitOps Agent with tools to detect the local git
  state and write GitHub Actions workflow files.
- **Task 4.4:** Ensure this workflow satisfies
  `docs/cuj/02-deploy-to-google-agent-engine.md`.

## Phase 5: Dynamic Skills Architecture & Configuration

**Goal:** Move away from hardcoded tools, allow dynamic Skill loading, and
enable user configuration (like selecting the active LLM).

- **Task 5.1:** Design the physical structure of a Skill (e.g., a `.skills/`
  folder containing `instructions.md` and `tools.yaml`).
- **Task 5.2:** Implement the `SkillLoader` in the Go SDK that reads these
  folders and dynamically registers ADK tools and context injections for the
  internal agents.
- **Task 5.3:** Refactor Phase 3 and Phase 4 to use the new Skills
  architecture instead of hardcoded Go logic.
- [x] **Task 5.4:** Implement configuration management. Introduce an
  `agents config` command and a `/model` slash command to allow the user to
  override the default `gemini-2.5-flash` model (including an "auto" routing
  mode).

## Phase 6: Multi-Agent Orchestration & Swarms

**Goal:** Enable complex topologies where multiple internal agents collaborate
transparently on a single user request.

- [x] **Task 6.1:** Implement the multiplexed UI in the terminal to display
  multiple agent streams concurrently.
- [ ] **Task 6.2:** Create the "Swarm Skill" that teaches the Router Agent how
  to instantiate and delegate to specialized sub-agents (Architect, Security
  Expert, Data Engineer).
- [ ] **Task 6.3:** Ensure this workflow satisfies
  `docs/cuj/03-swarm-design-collaboration.md`.

## Phase 7: Dynamic Swarm Provisioning & Mission Control

**Goal:** Realize the "Engineering Manager Paradigm" where the CLI
autonomously decomposes complex tasks, provisions $N$ agents to execute them
in parallel, and provides a dashboard to monitor them.

- [x] **Task 7.1:** Implement the "Swarm Dashboard" Bubble Tea layout
  component, creating a visual grid of active "Agent Cards" above the main
  chat viewport.
- [ ] **Task 7.2:** Refactor the SDK Event Bus to stream live tool telemetry
  (e.g., streaming `stdout` from `bash_execute`) directly to the Agent Cards.
- [x] **Task 7.3:** Implement the "Observer Agent" pattern: A lightweight,
  parallel model execution loop that synthesizes raw telemetry into semantic
  status updates (implemented as "Observe Mode" `^O`).
- **Task 7.4:** Implement Swarm "Control Panel" interactions: pausing,
  resuming, or killing specific agents from the dashboard.
- **Task 7.5:** Implement dynamic auto-provisioning: Teach the Router Agent
  how to write a dependency graph and spin up arbitrary, parallel Worker
  Agents based on the graph's complexity.
