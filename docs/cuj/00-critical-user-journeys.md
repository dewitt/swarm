# Critical User Journeys (CUJs)

This directory contains detailed narratives of how developers interact with
the `swarm` CLI in various scenarios. These journeys guide the product design,
ensuring the tool remains user-centric, frictionless, and powerful.

## Existing Journeys

1. **[Building a Local Agent with ADK Python](./01-build-local-adk-python-agent.md)**
   - **Focus:** Out-of-the-box experience, project scaffolding, and local
     testing.
1. **[Deploying an Agent to Google Agent Engine](./02-deploy-to-google-agent-engine.md)**
   - **Focus:** Dynamic Skills, GitOps workflows, and CI/CD generation.
1. **[Swarm Collaboration on System Design](./03-swarm-design-collaboration.md)**
   - **Focus:** Git-native decentralized multi-agent orchestration, specialized roles (`ROLES.md`), and asynchronous peer review.
1. **[Autonomous Swarm-based Skill Refinement](./04-swarm-skill-refinement.md)**
   - **Focus:** Self-updating skills, deep web research, human-in-the-loop
     validation, and GitOps branching strategies.
1. **[Agentic Test Generation in CI](./05-agentic-test-generation.md)**
   - **Focus:** Headless execution, native GitHub Actions integration, and asynchronous distributed PR creation.
1. **[Multi-Model Code Review and Consensus](./06-multi-model-code-review.md)**
   - **Focus:** Diverse model architectures (Codex, Claude, Gemini) collaborating natively on GitHub PRs to debate implementations and reach actionable consensus.

---

## Brainstorming: Future CUJs

To ensure the CLI remains competitive with advanced tools like Gemini CLI,
Claude Code, or Codex, we should explore the following user journeys:

### 4. Cross-Repository API Refactoring

- **Concept:** A user needs to update an API contract in a core service. They
  ask the CLI to not only update the service but also search across multiple
  dependent repositories, generate the necessary changes, and open
  cross-linked Pull Requests.
- **Key Features:** Multi-repo context management, large-scale code
  generation, "Architect" and "Worker" agent swarms.

### 5. Contextual Onboarding (The "Explain This System" Journey)

- **Concept:** A new developer joins a complex legacy project. They ask the
  CLI, "Where is the payment processing logic, and how does it connect to the
  database?" The CLI maps the architecture, traces the code paths, and
  generates a dynamic, interactive explanation.
- **Key Features:** Codebase indexing, architectural mapping, knowledge
  synthesis.

### 6. Automated Incident Response & Deep Debugging

- **Concept:** An alert fires in production. A developer pastes the stack
  trace into the CLI. The CLI instantiates a "Debugger Agent" that reads the
  logs, analyzes recent commits, reproduces the state locally, and proposes a
  surgical fix with a test case.
- **Key Features:** Log analysis, empirical reproduction, targeted patching.

### 7. Team Context Sharing & Handoffs

- **Concept:** A junior developer gets stuck trying to fix a bug after an hour
  of working with the CLI. They type `agents share session`. The CLI generates
  a secure context token. A senior engineer imports that token, and their
  local CLI resumes the exact conversational state, complete with the junior
  developer's history and the agent's current working memory.
- **Key Features:** State serialization, collaborative AI sessions, context
  portability.

### 8. Migrating Legacy Codebases via Swarm

- **Concept:** A team wants to migrate a large application from an old
  framework to a new one. They instruct the CLI to perform the migration. A
  "Supervisor Agent" divides the application into chunks and assigns them to
  "Worker Agents," aggregating and verifying the results before presenting the
  final diff.
- **Key Features:** Distributed task execution, supervisor-worker multi-agent
  patterns, complex state management.




