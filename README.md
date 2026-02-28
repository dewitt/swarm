# Agents CLI

A clean CLI and embeddable SDK for managing, building, and deploying AI agents
into native ecosystems.

![Agents CLI Demo](docs/assets/demo.gif)

## Features

- **Terminal UI**: Persistent, interactive terminal sessions built on Bubble
  Tea, featuring rich native markdown rendering (`glamour`), async execution,
  client-side slash commands (`/help`, `/model list`, `/skills`), multi-pane
  layouts, real-time agent handoff visualization, and flattened multiline
  history for stable autocomplete.

- **Swarm Dashboard (Mission Control)**: The UI shifts from a basic chat REPL
  into an air-traffic control layout when swarms are deployed. It visualizes
  concurrent agents working in parallel via live status cards above the main
  chat. Try it by running `agents --demo-swarm`.

  ![Swarm Dashboard Demo](docs/assets/demo_swarm.gif)

- **Direct Shell Execution (`!`)**: Toggle into a dedicated shell mode or run
  single-shot bash commands instantly from within the REPL without breaking
  your flow.

- **Context Referencing (`@`)**: Seamlessly inject files directly into the
  LLM's context window by typing `@` to trigger an inline, fuzzy-filtered
  overlay of your workspace files.

- **Session Persistence & Resumption**: Sessions are automatically persisted
  to a local SQLite database. View past activity with `/sessions`, rewind the
  conversation history with `/rewind`, and easily pick up where you left off
  using the `--resume` flag.

- **Read-Only Plan Mode**: Use the `/plan` command or `--plan` flag to safely
  brainstorm architecture with the agent explicitly sandboxed from modifying
  your filesystem.

- **Global Memory & Configuration**: Teach the agent your preferences once
  using the `/remember` command, and it will persist across all your projects.
  Check your global settings with `agents config` or `/config`.

- **Web Fetch & Search**: Native capabilities to search the web and fetch
  up-to-date documentation during task execution using the `web_researcher`
  agent.

- **UNIX Piping**: Integrate agents directly into your workflows via
  single-shot prompts and standard input (e.g.,
  `cat error.log | agents -p "What went wrong?"`).

- **Dynamic Skills Architecture**: Completely decoupled capabilities adhering
  to the open `agentskills.io` standard (`SKILL.md`). Easily write, share, and
  dynamically load new skills without recompiling.

- **Framework Agnostic**: Natively supports Google ADK, LangGraph, and custom
  architectures via `agent.yaml` manifests.

- **Native CI/CD Integration**: Seamlessly scaffolds standard CI/CD pipelines
  (like GitHub Actions) and integrates directly with your native ecosystem.

- **Agent Swarms**: The core SDK is powered by the Google Agent Development
  Kit (ADK) and orchestrates a swarm of specialized internal agents (Router,
  Builder, Deployment) using a cascading model architecture (fast models for
  routing, reasoning models for execution). Agent responses are attributed
  with colorful badges in the chat log.

## Prerequisites

- Go 1.21 or higher.

## Building from Source

To build the `agents` binary from source:

```bash
# Clone the repository
git clone https://github.com/dewitt/agents.git
cd agents

# Build the binary
go build -o bin/agents ./cmd/agents

# Run the CLI
./bin/agents
```

## Running the CLI

Simply running the binary launches the full-screen interactive Terminal User
Interface (TUI):

```bash
./bin/agents
```

From here, you can start conversing with the internal Router Agent, scaffold
new projects, or deploy existing agents.

## Documentation & Philosophy

- **[Project Philosophy](PHILOSOPHY.md)**: Read our core beliefs about thin
  software, fat models, and Zero-HITL verification.
- **[Design Docs](docs/design/)**: Detailed implementation roadmaps and
  architectural overviews.
- **[Critical User Journeys](docs/cuj/)**: Example workflows illustrating how
  the CLI is intended to be used.

______________________________________________________________________

*The `demo.gif` above is generated autonomously using Charmbracelet's `vhs`
tool. Agents working on this project should re-run `vhs demo.tape` whenever
they significantly alter the UI.*
