# Telemetry, Logging, and Evaluation Architecture

For the `swarm` CLI to be a viable, production-grade application, it must
have robust support for observability. This document outlines the strategy for
capturing application logs, handling errors gracefully within the TUI
environment, and recording complete session trajectories for downstream agent
evaluations.

## 1. Objectives

- **Non-Destructive Logging:** Ensure that debug and error logs do not pollute
  `stdout` or `stderr`, which would corrupt the Bubble Tea Terminal User
  Interface (TUI).
- **Trajectory Capture:** Record the complete, step-by-step history of user
  prompts, agent reasoning, tool calls, and tool outputs to facilitate prompt
  optimization ("hill-climbing") and regression testing.
- **Graceful Error Handling:** Capture panics and deep stack errors, reporting
  them cleanly within the UI or a dedicated log file rather than crashing the
  terminal state.

## 2. Session Trajectories & Evals

To systematically improve the agent's intelligence, we need data on how it
behaves in the wild.

### Utilizing ADK Session Storage

We currently use the Google ADK's native `session/database` package backed by
SQLite (`~/.config/swarm/sessions.db`). This database naturally stores every
`session.Event`.

- **Tool Traces:** Because the ADK captures the execution of function tools
  (like `bash_execute` or `read_local_file`), the SQLite database contains the
  exact sequence of actions the agent took to solve a problem.
- **Exporting for Evals:** We will build a CLI utility (e.g.,
  `agents export-sessions --format=jsonl`) that dumps these SQLite traces into
  standard formats suitable for evaluation pipelines (like LangSmith,
  Braintrust, or custom evaluation scripts).

## 3. Application Logging Strategy

Because the TUI owns the terminal screen, standard `fmt.Println` or
`log.Printf` calls will break the visual layout.

### File-Based Logging

All internal application telemetry (e.g., "Initializing SDK", "Loading skill
X", "GORM queries") must be routed to a rotating log file, not the terminal.

- **Log Location:** `~/.config/swarm/logs/agents.log`
- **Structured Logging:** Use Go's `log/slog` package to write structured JSON
  logs to this file. This makes it easier to parse and filter errors later.

### Log Levels and Debugging

- Implement standard log levels (`DEBUG`, `INFO`, `WARN`, `ERROR`).
- By default, the application runs at the `INFO` level.
- Introduce an `AGENTS_DEBUG=true` environment variable or a `--debug` CLI
  flag. When active, `DEBUG` level statements (and verbose outputs like GORM
  SQL queries) are written to the log file.
- **In-App Viewing:** Implement a `/logs` slash command or a dedicated UI pane
  that allows the user to `tail` the log file directly within the application.

## 4. Crash Reporting and Terminal Safety

If the Go application panics while Bubble Tea is running in the "Alternate
Screen" mode, the user's terminal can be left in a broken, unusable state
(hidden cursors, broken line wrapping).

- **Panic Recovery:** Implement a global `defer recover()` at the entry point
  of `launchInteractiveShell()`. If a panic occurs, the recovery handler must
  cleanly shut down the Bubble Tea program, restore the terminal state, and
  then write the stack trace to `stderr` and the log file.
- **Sub-Agent Isolation:** If a sub-agent or tool execution fails or panics,
  the error should be caught by the Swarm Agent's execution loop and returned
  to the user as a standard chat message (e.g., "✦ The gitops_agent
  encountered a fatal error: ..."), rather than crashing the host process.

## 5. Implementation Roadmap

1. **Phase 1 (Logging Foundation):** Replace the standard `log.Fatal` and
   `log.Printf` calls in `pkg/sdk/manager.go` with a configured `slog`
   instance pointing to the local `agents.log` file.
1. **Phase 2 (Terminal Safety):** Implement the global panic recovery wrapper
   around the TUI entry point.
1. **Phase 3 (Trajectory Export):** Build the CLI command to export the SQLite
   event history for offline evaluation.
