# Swarm CLI

A clean CLI and embeddable SDK for massively multi-agent orchestration and
observability into native ecosystems.

![Swarm CLI Demo](docs/assets/demo.gif)

## Features

- **Terminal UI**: Persistent, interactive terminal sessions built on Bubble
  Tea, featuring rich native markdown rendering (`glamour`), async execution,
  client-side slash commands (`/help`, `/model list`, `/skills`), multi-pane
  layouts, real-time agent handoff visualization, and flattened multiline
  history for stable autocomplete.

- **Agent Panel**: The UI shifts from a basic chat REPL into an agent-centric
  layout when swarms are deployed. It visualizes concurrent agents working in
  parallel via live "Agent Cards" featuring **Live Telemetry** (e.g.,
  real-time scrolling build logs and test outputs) and dynamic status updates.

- **Direct Shell Execution (`!`)**: Toggle into a dedicated shell mode or run
  single-shot bash commands instantly from within the REPL without breaking
  your flow.

- **Context Referencing (`@`)**: Seamlessly inject files directly into the
  LLM's context window by typing `@` to trigger an inline, fuzzy-filtered
  overlay of your workspace files.

- **Input Agent**: A high-speed, lightweight routing layer that pre-processes
  every user input. It proactively detects digressions and context shifts,
  rerouting the conversation to the Swarm Agent _before_ the message is
  processed, ensuring the system stays aligned with human topic-switching.

- **Session Persistence & Resumption**: Sessions are automatically persisted
  to a local SQLite database. View past activity with `/sessions`, rewind the
  conversation history with `/rewind`, and easily pick up where you left off
  using the `--resume` flag.

- **Async Execution & Input Queueing**: The CLI operates asynchronously. You
  are never locked out while agents are working. You can queue up multiple
  instructions or seamlessly interrupt (`Ctrl+C` or `Esc`) a runaway agent
  mid-thought without crashing the session.

- **Observe Mode**: Toggle deep ADK telemetry via `^O` to view thoughts, tool
  args, and tool results in a dedicated UI pane, giving you full visibility
  into the agent's internal reasoning.

- **Read-Only Plan Mode**: Use the `/plan` command or `--plan` flag to safely
  brainstorm architecture with the agent explicitly sandboxed from modifying
  your filesystem.

- **4-Tier Hierarchical Memory**: Swarm implements a robust, OS-inspired
  memory architecture to prevent context rot during long-horizon tasks.

  - *Working Memory*: The immediate token window, automatically pruned of
    massive tool outputs to preserve inference speed.
  - *Episodic Memory*: The chronological audit log of the active session.
  - *Semantic Memory*: An embedded SQLite/FTS5 database that passively
    extracts "timeless facts" (e.g., project build commands, API keys) during
    execution and automatically injects them into future agent prompts.
  - *Global Memory*: Cross-project preferences set via `/remember` or
    `.gemini/GEMINI.md` files. View real-time token stats for all tiers using
    the `/memory` command.

- **Web Fetch & Search**: Native capabilities to search the web and fetch
  up-to-date documentation during task execution using the `web_researcher`
  agent.

- **UNIX Piping**: Integrate agents directly into your workflows via
  single-shot prompts and standard input (e.g.,
  `cat error.log | swarm -p "What went wrong?"`). You can output raw JSON
  execution traces using `--trajectory` or a human-readable execution summary
  using `--explain`.

- **Dynamic Skills Architecture**: Completely decoupled capabilities adhering
  to the open `agentskills.io` standard (`SKILL.md`). Easily write, share, and
  dynamically load new skills without recompiling.

- **Framework Agnostic**: Natively supports Google ADK, LangGraph, and custom
  architectures via `agent.yaml` manifests.

- **Native CI/CD Integration**: Seamlessly scaffolds standard CI/CD pipelines
  (like GitHub Actions) and integrates directly with your native ecosystem.

- **Agent Swarms**: The core SDK is powered by the Google Agent Development
  Kit (ADK) and orchestrates a swarm of specialized internal agents (Swarm
  Agent, Builder, Deployment) using a cascading model architecture (fast
  models for routing, reasoning models for execution). Agent responses are
  attributed with colorful badges in the chat log.

## Prerequisites

- Go 1.21 or higher.

## Building from Source

To build the `swarm` binary from source:

```bash
# Clone the repository
git clone https://github.com/dewitt/swarm.git
cd swarm

# Build the binary (Standard)
go build -o bin/swarm ./cmd/swarm

# Build the binary with FTS5 Semantic Search (Recommended)
go build -tags fts5 -o bin/swarm ./cmd/swarm

# Run the CLI
./bin/swarm
```

## Running the CLI

Simply running the binary launches the full-screen interactive Terminal User
Interface (TUI):

```bash
./bin/swarm
```

From here, you can start conversing with the internal Swarm Agent, scaffold
new projects, or deploy existing agents.

## Web Agent Panel

Swarm includes a built-in web server that broadcasts live agent telemetry via
Server-Sent Events (SSE). While running the CLI, you can open your browser to
**[http://localhost:5050](http://localhost:5050)** to view a rich, graphical
dashboard of all active agents in your swarm, complete with live status
indicators and real-time execution logs.

![Swarm Web Agent Panel](docs/assets/web-demo.gif)

## E2E Evaluations (LLM-as-a-Judge)

![Swarm Eval Demo](docs/assets/eval_demo.gif)

### Evaluation Philosophy

We believe that autonomous agents cannot be effectively validated using
traditional, brittle unit testing due to the non-deterministic nature of LLMs.
Instead, Swarm embraces a **Zero-HITL, LLM-as-a-Judge** philosophy for
continuous integration.

Our native end-to-end evaluation harness spins up sandboxed, ephemeral
workspaces, issues high-level natural language directives to the Swarm, and
collects the entire execution trajectory (actions, tools, shell commands, and
final code state). A designated "Judge LLM" then evaluates this trajectory
against a strict, scenario-specific rubric to determine if the agent
successfully achieved the intended outcome without breaking the workspace.
This approach mirrors actual human workflows and ensures our agents remain
robust and capable as underlying foundation models evolve.

To run the full evaluation suite:

```bash
# Requires an active AI API key
export GOOGLE_API_KEY="..."

# Run all scenarios
swarm eval
```

To run a single, specific scenario (useful for debugging):

```bash
# Run a specific scenario by ID
swarm eval scenario_1
```

If you wish to add a new scenario, define its metadata (Name, Prompt, Rubric,
and Fixture Path) in `pkg/eval/scenarios.go` and add the sandbox code fixture
to the `eval/fixtures/` directory. Be sure to use the `--debug` flag natively
if you need to debug the AST parsing logic behind the harness.

## Documentation & Philosophy

- **[Project Philosophy](PHILOSOPHY.md)**: Read our core beliefs about thin
  software, fat models, and Zero-HITL verification.
- **[Design Docs](docs/design/)**: Detailed implementation roadmaps and
  architectural overviews.
- **[Critical User Journeys](docs/cuj/)**: Example workflows illustrating how
  the CLI is intended to be used.

______________________________________________________________________

_The `demo.gif` above is generated autonomously using Charmbracelet's `vhs`
tool. Agents working on this project should re-run `vhs demo.tape` whenever
they significantly alter the UI._
