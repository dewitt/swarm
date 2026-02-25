# Project TODOs & Technical Debt

This document tracks recurring tasks, pending migrations, and known technical debt that should be addressed as the project evolves.

## Pending Migrations

- [ ] **Migrate to Native ADK Skills**: Currently, the `agents` CLI implements its own custom Skill loader (`pkg/sdk/skill.go`) because the Go ADK does not yet natively support Skills. Once the official ADK `google.golang.org/adk/skill` (or equivalent) package is released, we must deprecate our custom loader and migrate entirely to the ADK implementation to maintain our "Thin Software" philosophy.

## Feature Backlog

- [ ] **Context Management**: Implement the backend logic for the `/context` and `/drop` slash commands to allow users to manually load and evict files from the agent's memory window.
- [ ] **Global Configuration**: Implement the `agents config` command to save global user preferences (like API keys or preferred text editor) to a generic `~/.config/agents/config.yaml` file.
