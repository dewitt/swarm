# Multi-Agent Orchestration

## Core Principle: One Agent is Never Enough

The `agents` project is designed around the belief that single-agent
architectures are fragile. Real-world tasks require delegation, specialized
context, and verification loops.

This principle applies at two levels:

1. **Internal Architecture**: How the CLI works under the hood.
1. **Target Architecture**: How the CLI helps users build their own agents.

## Internal Architecture: The Core Swarm

The CLI does not rely on a single massive system prompt. Instead, the core Go
SDK instantiates a swarm of specialized ADK agents:

- **Router Agent**: The front-door. It converses with the user, maintains
  session context, and routes specific technical tasks to specialized
  sub-agents.
- **Architect Agent**: Responsible for scaffolding project structures and
  writing foundational code based on user requirements.
- **Debugger Agent**: Invoked when a test fails or a deployment errors out. It
  analyzes stack traces and proposes fixes.
- **GitOps Agent**: Specialized in crafting CI/CD pipelines, writing GitHub
  Actions, and executing Git operations.

When the Router Agent delegates to the Debugger Agent, the terminal UI
visualizes this handoff, making the multi-agent execution transparent to the
user.

## Target Architecture: Managing User Swarms

When users use the CLI to manage their own projects, the CLI treats
multi-agent topologies as first-class citizens.

If a user is building a LangGraph multi-agent system, the CLI provides tools
to:

- Visualize the routing graph locally.
- Mock interactions between sub-agents during local testing.
- Deploy the agents either as a single monolithic service or as distributed
  microservices (if the deployment Skill supports it).

## Swarm Extensibility

Support for new multi-agent patterns (e.g., a "Debate" pattern, or a
"Supervisor-Worker" pattern) can be added to the CLI via Skills. A Skill can
provide templates and prompts that teach the internal Architect Agent how to
scaffold these complex patterns for the user.
