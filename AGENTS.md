# Agent Collaboration Guide (AGENTS.md)

**Welcome, AI Agent.**

If you are reading this file, you have been assigned to work on the `agents`
project. This document serves as your primary context, architectural
constraint guide, and communication protocol.

## Project Vision

The `agents` project is a Go-based CLI and embeddable SDK for managing,
building, and deploying AI agents. It prioritizes native ecosystem integrations, dynamic
extensibility (Skills), and is built on the philosophy that "one agent alone
is never enough."

## Architectural Constraints

When writing or modifying code in this repository, you **must** adhere to the
following rules:

1. **Separation of Concerns:** The CLI (Presentation Layer) and the SDK
   (Business Logic) must be strictly decoupled. The CLI is merely a consumer
   of the SDK. Do not leak terminal UI logic (e.g., Bubbletea components, ANSI
   codes) into the core SDK.
1. **Go Standards:** The project is written in Go. You must use idiomatic Go
   conventions, ensure the SDK is compatible with `cgo` and WebAssembly
   (`wasm`), and provide comprehensive Go unit tests for any new logic.
1. **Google ADK First:** The internal business logic of the CLI is powered by
   the Google Agent Development Kit (ADK) for Go. Ensure you are familiar with
   its interfaces (`LlmAgent`, `Tool`, `Launcher`).
1. **Native CI/CD Integration:** Deployments and environment changes should be
   implemented via standard CI/CD files (e.g., committing GitHub Actions) rather than
   proprietary API calls whenever possible.
1. **No Hardcoding:** If a feature involves a specific cloud provider or an
   external framework (like LangChain), it should be built as a dynamically
   loaded "Skill", not hardcoded into the Go binary.

## Project Structure

- `/cmd/agents`: The entry point for the CLI binary.
- `/pkg/`: The embeddable Go SDK (core business logic, ADK integrations,
  GitOps tools).
- `/docs/design/`: High-level and detailed architectural documents. **Read
  these before implementing new features.**
- `/docs/cuj/`: Critical User Journeys. If you change a workflow, you must
  update or add a CUJ here.
- `/skills/` (Future): Default skills shipped with the CLI.

## Agent Protocol & Handoffs

Because this project will be built by multiple agents asynchronously, you must
leave a clean trail for the next agent:

1. **Document Your Intent:** Before closing your session, ensure any
   uncompleted architectural decisions or roadblocks are documented in an
   issue or a design doc.
1. **Markdown Formatting:** All markdown files must be formatted using
   `mdformat --wrap 78` before being committed.
1. **State Your Context:** When reviewing PRs or committing code, clearly
   state what CUJ or design document your changes satisfy.
1. **Testing is Mandatory:** Never commit a feature without an accompanying
   test. The next agent relies on your tests to ensure they don't break your
   work.

## Next Steps

Check the `docs/design/06-implementation-roadmap.md` file to see the current
sequence of work and determine which phase is currently active.
