# CUJ: Building a Local Agent with ADK Python

## User Persona

Alex is a backend software engineer who wants to experiment with building a
custom AI assistant using Python. They have heard of the Google Agent
Development Kit (ADK) but aren't sure how to set up the boilerplate.

## Journey

### 1. Installation

Alex installs the CLI tool via Homebrew.

```bash
brew install agents
```

### 2. Initialization & Conversation

Alex creates a new directory and starts the interactive CLI.

```bash
mkdir my-python-agent && cd my-python-agent
agents
```

The CLI launches its interactive interface:

> **Agents CLI:** Welcome to Agents! How can I help you today? **Alex:** I
> want to build a simple agent that can fetch the current weather using Python
> and the Google ADK.

### 3. Scaffolding

The `agents` internal Router Agent understands the intent. It delegates the
task to the Builder Agent, equipped with the ADK Python Skill.

> **Agents CLI:** Excellent! I'll set up a Google ADK Python project for you.
> *[Spinner: Generating boilerplate...]* I have created the following files:
>
> - `requirements.txt` (with `google-adk` and `requests`)
> - `agent.py` (your ADK agent with a mock `get_weather` tool)
> - `agent.yaml` (the manifest)
>
> Would you like me to create a virtual environment and install the
> dependencies for you? (Y/n)

Alex presses `Enter` to accept.

### 4. Code Generation

The CLI executes the shell commands to set up the environment.

> **Agents CLI:** Dependencies installed. You can review the `agent.py` file.
> It defines a `root_agent` with a tool called `get_weather`. You can run this
> agent locally using the `agents run` command, or simply ask me to test it
> right here.

### 5. Local Testing

Alex wants to make sure it works.

> **Alex:** Let's test it. Ask the agent what the weather is in Tokyo.

The CLI, acting as an orchestrator, invokes the local Python process defined
in `agent.yaml`.

> **Agents CLI:** Running local agent... **Agent (Local):** The weather in
> Tokyo is currently clear and 15°C.
>
> **Agents CLI:** The local agent successfully executed the `get_weather`
> tool. How else would you like to modify it?
