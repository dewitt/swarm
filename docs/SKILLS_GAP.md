# Swarm Skills Gap Analysis

This document tracks identified gaps in Swarm's current tooling and skill
ecosystem, specifically focusing on capabilities that prevent Swarm from
acting as a fully autonomous software engineer. The recommendations below are
based on comparative analysis with other leading agentic CLIs (like Gemini
CLI, Claude Code, and Codex).

## 1. Background Process Management (High Priority)

**The Gap:** Swarm's current `bash_execute` tool is strictly synchronous; it
calls `cmd.Start()` followed immediately by `cmd.Wait()`. If an agent attempts
to start a local development server (e.g., `npm run dev` or `go run main.go`),
the tool hangs indefinitely, eventually causing a context timeout and breaking
the execution loop.

**How Others Do It:** Competitors like Gemini CLI solve this by providing an
`is_background: true` boolean parameter on their shell execution tools. This
allows the CLI to detach the process, return the PID to the agent, and
silently capture stdout/stderr without blocking the agent's event loop.

**Recommendation:**

- Update `pkg/sdk/tools.go` to support an `is_background` boolean flag in
  `BashExecuteArgs`.
- Implement a background process registry so agents can subsequently tail logs
  or kill the detached processes.

______________________________________________________________________

## 2. Database Explorer (`db-explorer`)

**The Gap:** Swarm has excellent tools for filesystem manipulation
(`read_file`, `write_file`) but zero native abstractions for inspecting state
inside a database. Agents currently have to write raw shell scripts (e.g.,
piping strings to `sqlite3`), which frequently leads to context-window
blowouts if an agent accidentally dumps a massive table to standard output.

**How Others Do It:** Claude Code and Codex rely almost entirely on shell
delegation, expecting the user to have `psql` or `sqlite3` installed, and
hoping the LLM is smart enough to append `LIMIT 10`. This is brittle and
error-prone.

**Recommendation:**

- Develop a dedicated `db-explorer` skill.
- Implement specific, parameterized tools (e.g., `list_tables`, `get_schema`,
  `execute_query_paginated`).
- The tool layer must explicitly enforce pagination and truncate large cell
  payloads to protect the agent's context window.

______________________________________________________________________

## 3. Headless Browser / E2E Tester (`browser-agent`)

**The Gap:** Swarm currently relies on `web-researcher` and `web_fetch` tools,
which are strictly for pulling static HTML/Markdown. While Swarm can
successfully scaffold a React application, it is completely blind to the
visual layer and cannot actually *verify* that the app renders correctly or
that interactive elements work.

**How Others Do It:**

- *Claude Code:* Relies on writing temporary, ad-hoc Playwright or Puppeteer
  scripts to disk and executing them via the shell. This is slow and highly
  prone to syntax errors.
- *Gemini CLI:* Features a native `browser_agent` tool that interacts directly
  with the browser's accessibility (A11y) tree, allowing it to navigate,
  click, and "see" the page reliably without writing intermediate test
  scripts.

**Recommendation:**

- Implement a native `browser-agent` skill in Swarm.
- Expose specific semantic tools (`goto_url`, `get_dom_tree`, `click_element`)
  backed by a headless browser driver.
- This bridges the gap between writing code and empirical, end-to-end user
  acceptance testing.
