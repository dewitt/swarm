# Swarm Code Review Guide

This document contains specialized instructions for an AI agent (or human
contributor) to perform a comprehensive, top-to-bottom codebase review. The
objective of this process is to iteratively improve the quality of the Swarm
repository over time by systematically rooting out issues.

## When to Use This Guide

You should execute this workflow whenever a user requests a "thorough code
review," an "audit of the codebase," or a "daily quality check."

## Core Objectives

1. **Bug Hunting**: Identify obvious logic errors, memory leaks, unhandled
   errors, and subtle concurrency or race conditions. For the TUI
   (`cmd/swarm`), focus heavily on Bubble Tea lifecycle bugs, out-of-bounds
   rendering, and state management.
1. **Dead Code Elimination**: Hunt down unused variables, functions,
   unexported structs, abandoned prototypes, or legacy features that clutter
   the project.
1. **Idiomatic Practices**: Ensure the code adheres strictly to Go and Bubble
   Tea best practices (e.g., proper error wrapping, channel closing, avoiding
   blocking I/O in TUI `Update` loops).
1. **Documentation Alignment**: Verify that `README.md`, `PHILOSOPHY.md`, and
   the `docs/` folder accurately reflect the current implementation. Flag any
   drift between what the code *does* and what the documentation *says*.

______________________________________________________________________

## The Code Review Workflow

When instructed to perform a review, follow these phases methodically. Do not
attempt to fix all issues in a single massive commit; instead, document your
findings and propose a targeted strategy for the user to approve.

### Phase 1: Static Analysis & Health Checks

Before reading the logic deeply, ensure the fundamental health of the project:

1. Run `go mod tidy` to check for unused or unaligned dependencies.
1. Run `go vet ./...` to catch standard Go compilation and shadow errors.
1. Run `go test ./...` to verify that existing tests pass.
1. Execute any project-specific linting if available.

### Phase 2: Systematic Codebase Audit

Use a combination of `glob`, `grep_search`, and `read_file` to review the
application layer by layer. Look specifically for:

#### A. Core SDK & Architecture (`pkg/sdk/`)

- **Concurrency**: Are goroutines spawned without a clear lifecycle? Are
  channels closed properly? Are mutexes used correctly to prevent race
  conditions on shared state (e.g., `sync.RWMutex` usage)?
- **Error Handling**: Are errors swallowed? Are they properly wrapped using
  `fmt.Errorf("...: %w", err)`?
- **Interfaces**: Are interfaces small and focused? Is there overly coupled
  logic that should be abstracted?

#### B. Terminal User Interface (`cmd/swarm/`)

- **Bubble Tea Antipatterns**: Ensure `Update()` methods never perform
  blocking I/O (like HTTP requests or disk reads) directly; they must return a
  `tea.Cmd`.
- **Rendering Bounds**: Check `lipgloss` layouts and `View()` functions. Are
  widths and heights properly calculated and passed down? Are styles
  word-wrapping unexpectedly?
- **State Leaks**: Are UI models holding onto massive amounts of history or
  telemetry logs without caps or truncation?

#### C. Evaluator & Tests (`pkg/eval/`, `tests/`)

- **Test Coverage**: Identify critical paths in the SDK or CLI that lack test
  coverage.
- **Brittle Tests**: Are tests relying on hardcoded timing, sleep statements,
  or unstable network calls?
- **Eval Parity**: Do the LLM-as-a-judge evaluation scenarios accurately
  reflect the features of the current Swarm CLI?

### Phase 3: Documentation Sync

1. Read `README.md` and check the "Features" list. Does the code actually
   implement everything listed? Are the commands accurate?
1. Check inline godoc comments on exported SDK methods. Do they clearly
   explain the parameters and return values?
1. Are there `TODO.md` items that have actually been completed but not checked
   off?

### Phase 4: Reporting and Execution

Synthesize your findings into a structured report.

1. Group the findings by priority (Critical Bugs, Idiomatic Refactors, Dead
   Code, Doc Updates).
1. Do NOT immediately start rewriting massive chunks of code.
1. Present the report to the user and ask: *"Which of these areas would you
   like me to tackle first?"*
1. Proceed iteratively with the user's approval, employing the standard
   `Plan -> Act -> Validate` loop for each fix.
