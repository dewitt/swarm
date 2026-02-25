# CLI User Experience (UX)

## "Out-of-the-box" Magic

The `agents` CLI is designed to be frictionless. It should require zero
configuration to get started and feel instantly familiar to users of tools
like Gemini CLI or Claude Code.

### Installation

Installation must be trivial:

```bash
brew install agents
```

### First Run

Upon first run, the CLI detects if an API key is present. If not, it politely
guides the user to authenticate or provide a key:

```
$ agents

Welcome to Agents!
It looks like you don't have an LLM provider configured.
Would you like to authenticate with Google, Anthropic, or OpenAI?
> ...
```

## Conversational Interface

The primary interface is a persistent, interactive terminal session.

```bash
$ agents
> I want to build a new customer support agent.
```

The CLI responds using rich text (Markdown formatting, syntax highlighting)
and asks clarifying questions.

## Multi-Agent Interaction

Because "one agent alone is never enough," the CLI UI should clearly
communicate when it is delegating tasks or when user-defined agents are
interacting.

- **Streaming Indicators**: When the CLI is thinking, a subtle spinner
  indicates progress.
- **Agent Avatars/Tags**: When the internal Router Agent delegates to the
  Builder Agent, the UI can briefly indicate this handoff, reinforcing the
  multi-agent nature of the platform.
- **Swarm Visualization**: If the user is running a local swarm of agents, the
  CLI should provide a multiplexed view of their logs and interactions.

## Commands vs. Conversation

While the CLI is highly conversational, it also supports standard command-line
arguments for CI/CD or scripting contexts:

- `agents build`: Compiles or prepares the current agent project.
- `agents deploy`: Triggers the deployment workflow.
- `agents test`: Runs the test suite for the local agent.

When run interactively (`agents`), these commands can be invoked via natural
language:

> "Please run the tests and then deploy if they pass."
