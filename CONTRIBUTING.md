# Contributing to Swarm CLI

Thank you for your interest in contributing to Swarm CLI! We welcome
contributions to help improve the project.

## How to Contribute

To ensure a smooth process for everyone, we kindly ask that you follow these
guidelines:

### 1. Issue-First Workflow

Before starting work on a bug fix, enhancement, or new feature, please **file
an issue** first.

- This allows the community and maintainers to discuss the problem or feature,
  provide feedback, and agree on an approach before you invest time in coding.
- Feel free to assign the issue to yourself if you plan to work on it.

### 2. Pull Request Workflow

All changes should be proposed via Pull Requests (PRs).

- Ensure your code passes all unit tests and builds successfully before
  submitting the PR.
- Reference the issue number in the PR description (e.g., `Fixes #123`).

### 3. Testing

We maintain a strict separation between deterministic unit tests and
non-deterministic End-to-End (E2E) tests.

- **Unit Tests:** These run by default and use mocks for LLM interactions. Run
  them with: `go test ./...`.
- **E2E Tests:** These exercise live models (Gemini/Claude) and require a
  valid `GOOGLE_API_KEY`. They are skipped by default. To run them, set the
  `SWARM_RUN_E2E` environment variable:
  ```bash
  SWARM_RUN_E2E=1 go test ./...
  ```

### 4. Branch Management

- Create a **new, dedicated branch** for each PR you work on (e.g.,
  `git checkout -b fix-issue-123`).
- Please do not submit PRs directly from your `main` branch.
- Once your PR is reviewed and merged into the main repository, the branch
  will be deleted to keep the repository clean.

### 4. Markdown Formatting

- All markdown files should be formatted using `mdformat --wrap 78` to ensure
  consistency.
- **CRITICAL:** Do **NOT** run `mdformat` on `SKILL.md` files (located in the
  `skills/` directory). These files use YAML frontmatter for agent
  configuration that will be corrupted by standard markdown formatters.

## Code of Conduct

We expect all contributors to adhere to a respectful and welcoming code of
conduct. Be kind, collaborate constructively, and help us maintain a positive
environment.
