# Critical User Journeys (CUJs)

This directory contains detailed narratives of how developers interact with
the `agents` CLI in various scenarios. These journeys guide the product
design, ensuring the tool remains user-centric, frictionless, and powerful.

## Existing Journeys

1. **[Building a Local Agent with ADK Python](./01-build-local-adk-python-agent.md)**
   - **Focus:** Out-of-the-box experience, project scaffolding, and local
     testing.
1. **[Deploying an Agent to Google Agent Engine](./02-deploy-to-google-agent-engine.md)**
   - **Focus:** Dynamic Skills, GitOps workflows, and CI/CD generation.
1. **[Swarm Collaboration on System Design](./03-swarm-design-collaboration.md)**
   - **Focus:** Native multi-agent orchestration, specialized roles, and
     transparent collaboration.

______________________________________________________________________

## Brainstorming: Future CUJs

To ensure the CLI remains competitive with advanced tools like Gemini
CLI, Claude Code, or Codex, we should explore the following user
journeys:

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

### 9. Natural Language "Review & Audit"

- **Concept:** Before submitting a Pull Request, a developer asks the CLI to
  perform a security and performance audit. An "Auditor Agent" reviews the
  diff, flags potential vulnerabilities (e.g., SQL injection risks), and
  generates inline code suggestions to fix them.
- **Key Features:** Specialized knowledge domains, pre-commit hooks, diff
  analysis.

### 10. Agentic Test Generation in CI

- **Concept:** While the primary UI is the terminal, the `agents` binary also
  runs in CI/CD. When a user pushes a feature without tests, a headless
  instance of the CLI detects the omission, generates missing unit and
  integration tests, and automatically commits them to the branch if they
  pass.
- **Key Features:** Headless execution, test synthesis, CI/CD integration.
