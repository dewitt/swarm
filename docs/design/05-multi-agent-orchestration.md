# Multi-Agent Orchestration

## Core Principle: One Agent is Never Enough

The `swarm` project is designed around the belief that single-agent
architectures are fragile. Real-world tasks require delegation, specialized
context, and verification loops.

This principle applies at two levels:

1. **Internal Architecture**: How the CLI works under the hood.
1. **Target Architecture**: How the CLI helps users build their own agents.

## Internal Architecture: The Core Swarm

The CLI does not rely on a single massive system prompt. Instead, the core Go
SDK instantiates a swarm of specialized ADK agents using a cascading model
architecture:

- **Swarm Agent**: The front-door, powered by `gemini-2.5-flash` for snappy
  coordination. It converses with the user, maintains session context, and
  routes specific technical tasks to specialized sub-agents.
- **Web Researcher Agent**: Specialized in deep web research using the Google
  Search grounding tool to pull down the latest documentation.
- **Skill Planning Agent**: Maintains and optimizes dynamic `SKILL.md` files,
  enforcing human-in-the-loop validation for major changes.
- **GitOps Agent**: Specialized in crafting CI/CD pipelines, writing GitHub
  Actions, and executing Git operations.

Sub-agents (Skills) default to `gemini-3.1-pro-preview` for thorough execution
but can request specific models via their SkillManifest.

When the Swarm Agent delegates to a sub-agent, the terminal UI visualizes
this handoff in real-time (e.g., "Handoff to builder_agent..."). All agent
responses are identified by their author in the chat log using colorful agent
badges for clear multi-agent swarm attribution, making the multi-agent
execution transparent to the user.

## Target Architecture: Managing User Swarms

When users use the CLI to manage their own projects, the CLI treats
multi-agent topologies as first-class citizens.

If a user is building a LangGraph multi-agent system, the CLI provides tools
to:

- Visualize the routing graph locally.
- Mock interactions between sub-agents during local testing.
- Deploy the swarm either as a single monolithic service or as distributed
  microservices (if the deployment Skill supports it).

## Swarm Extensibility

Support for new multi-agent patterns (e.g., a "Debate" pattern, or a
"Supervisor-Worker" pattern) can be added to the CLI via Skills. A Skill can
provide templates and prompts that teach the internal Planning Agent how to
scaffold these complex patterns for the user.

## 4. Third-Party Agent Orchestration

As the ecosystem evolves, monolithic AI tools (like Claude Code, Codex, or
Gemini CLI) will become highly capable specialists. The `swarm` CLI should
not attempt to rewrite these massive, proprietary systems from scratch.

Instead, the `swarm` CLI will act as the **Supreme Orchestrator**.

### The Implementation

We have built specialized "Wrapper Agents" (as Skills) that wrap the
command-line interfaces of *other* AI coding agents, specifically `gemini-cli`
and `claude-code`.

For example, if a user asks for a massive refactoring of a legacy codebase:

1. The primary **Swarm Agent** analyzes the request.
1. It delegates the task to the **Gemini CLI Sub-agent**.
1. The Sub-agent constructs the appropriate bash command (e.g.,
   `gemini -p "refactor everything" --apply`) and executes it using the
   `bash_execute` tool.
1. The Swarm Agent reviews the resulting diff and reports back to the user.

By treating other AI CLI tools as executable sub-agents, the `swarm` CLI
becomes the ultimate unified control plane for software development.
