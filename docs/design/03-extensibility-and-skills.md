# Dynamic Extensibility & Skills

## Rationale

The agent ecosystem is evolving too rapidly to build a monolithic CLI that
natively understands every framework, model provider, and deployment target.
The `agents` project takes a different approach: **Dynamic Extensibility via
Skills**.

We defer the "heavy lifting" to LLMs. Instead of writing Go code to parse a
specific API or manage a proprietary framework, we provide the CLI's internal
agents with plain-text documentation and generic tools.

## What is a Skill?

A Skill is a collection of resources (usually Markdown files, OpenAPI specs,
or basic scripts) and a manifest that teaches the internal ADK-based core how
to perform a new type of task.

### Example: "AWS Deployment Skill"

Rather than hardcoding AWS CloudFormation logic, an AWS Deployment Skill might
consist of:

1. `instructions.md`: Guidelines on how to deploy an agent to AWS Lambda.
1. `tools.yaml`: A list of generic CLI tools (e.g., granting the agent
   permission to run `aws-cli` commands).
1. `context/`: Best-practice examples of `serverless.yml` files.

When a user asks to deploy to AWS, the Router Agent detects this intent, loads
the AWS Skill, and dynamically acquires the capability to generate and execute
the deployment.

## Pluggable Architecture

The core SDK provides a standard plugin interface for discovering and loading
Skills.

- **Local Skills**: Users can drop a `.agents/skills` folder into their
  project to teach the CLI project-specific rules.
- **Global Skills**: Users can install community skills
  (`agents skill install github-manager`).

## Advantage over Hardcoding

By using Skills:

1. The `agents` binary remains extremely small and light.
1. Capabilities can be updated independently of the core CLI releases.
1. Users can inspect, fork, and modify how the CLI behaves simply by editing
   Markdown files, democratizing extensibility.
