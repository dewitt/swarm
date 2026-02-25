# Agents CLI

A highly polished, world-class CLI and embeddable SDK for managing, building,
and deploying AI agents via GitOps workflows.

![Agents CLI Demo](docs/assets/demo.gif)

## Features

- **World-Class TUI**: Persistent, interactive terminal sessions built on
  Bubble Tea, featuring rich text, async execution, client-side slash commands
  (`/help`, `/clear`), and multi-pane layouts.
- **Framework Agnostic**: Natively supports Google ADK, LangGraph, and custom
  architectures via `agent.yaml` manifests.
- **GitOps First**: Includes a native GitOps Agent that scaffolds GitHub
  Actions CI/CD pipelines and pushes code directly from the CLI.
- **Agent Swarms**: The core SDK is powered by the Google Agent Development
  Kit (ADK) and orchestrates a swarm of specialized internal agents (Router,
  Builder, GitOps) to fulfill your requests.

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
