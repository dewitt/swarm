# Swarm Code Review Guide

This document contains principled instructions for an AI agent (or human
contributor) to perform a comprehensive codebase review. The objective is to
iteratively improve the quality, maintainability, and reliability of the Swarm
repository.

## When to Use This Guide

Execute this workflow whenever a user requests a "thorough code review," an
"audit of the codebase," or a "quality check."

## Core Principles

1. **Robustness & Reliability**: Identify logic errors, unhandled exceptions,
   memory leaks, and concurrency issues (e.g., race conditions, unclosed
   channels, orphaned goroutines).
1. **Cleanliness & Clarity**: Hunt down dead code, unused variables, abandoned
   prototypes, and legacy features. Ensure the codebase remains lean.
1. **Idiomatic Design**: Ensure the code adheres to ecosystem best practices
   (e.g., standard Go idioms, proper error wrapping) and framework-specific
   patterns (e.g., non-blocking I/O in Bubble Tea `Update` loops).
1. **Documentation Alignment**: Verify that documentation (`README.md`,
   `docs/`, inline comments) accurately reflects the current implementation.
   Code and documentation must evolve together.

## The Code Review Workflow

Follow these phases methodically. Do not attempt to fix all issues in a single
massive commit; instead, document findings and propose a targeted strategy.

### Phase 1: Static Analysis & Health Checks

Ensure the fundamental health of the project using standard tooling:

- Verify dependency alignment and neatness.
- Run static analysis and vetting tools to catch compilation and shadow
  errors.
- Execute the test suite to ensure baseline functionality remains intact
  (preferring fast/unit tests for quick feedback).
- Apply project-specific linting or formatting. Note: Ensure `golangci-lint`
  is up to date with the current Go version. If it hangs or behaves
  unexpectedly, update it by running
  `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` and
  `golangci-lint cache clean`.

### Phase 2: Systematic Codebase Audit

Review the application layer by layer, focusing on architectural principles
rather than just syntax:

#### A. Core Architecture & SDK

- **Concurrency & Lifecycles**: Ensure all goroutines have a clear lifecycle
  and termination path. Verify graceful shutdown mechanisms for long-running
  processes.
- **Resource Management**: Check for proper synchronization around shared
  resources. Ensure external processes (e.g., shell commands) are
  context-bound to prevent hanging.
- **Error Handling**: Verify that errors are handled thoughtfully, providing
  necessary context (e.g., wrapping) without being swallowed or over-logged.
- **Coupling & Abstraction**: Evaluate interfaces and package boundaries.
  Ensure concerns are properly separated.

#### B. User Interface (TUI)

- **Asynchronous Execution**: Verify that the main event loop (e.g., Bubble
  Tea `Update`) never blocks on I/O. All long-running operations must be
  asynchronous.
- **Rendering Stability**: Look for layout instability, out-of-bounds
  rendering, or unconstrained memory growth in UI state components (e.g.,
  unbounded history arrays).

#### C. Testing & Evaluation

- **Reliability**: Identify brittle tests relying on hardcoded timing
  (`sleep`), race conditions, or unstable external dependencies.
- **Coverage & Scope**: Pinpoint critical paths lacking adequate test coverage
  or evaluation scenarios that no longer match CLI capabilities.

### Phase 3: Documentation Sync

- Check if user-facing documentation (e.g., `README.md` features) matches the
  actual implemented behavior.
- Review inline documentation for exported APIs to ensure accuracy and
  usefulness.
- Reconcile `TODO.md` items with actual codebase state.

### Phase 4: Reporting and Execution

Synthesize findings into a structured report:

1. **Deduplicate**: Review existing issues in `docs/CODE_ISSUES.md` to avoid
   duplicating known problems.
1. **Update Backlog**: Append new findings or significant context to
   `docs/CODE_ISSUES.md`.
1. **Categorize**: Group findings by priority (e.g., Critical Bugs, Idiomatic
   Refactors, Dead Code, Documentation).
1. **Propose**: Present the report to the user and ask: *"Which of these areas
   would you like me to tackle first?"*
1. **Execute**: Proceed iteratively with the user's approval, employing the
   `Plan -> Act -> Validate` loop for each targeted fix. Do not rewrite
   massive chunks of code unprompted.
