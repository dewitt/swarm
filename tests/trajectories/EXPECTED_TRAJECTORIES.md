# Expected Trajectories (Swarm Execution Paths)

This document establishes the theoretical baseline for how the `swarm`
architecture should execute specific classes of user prompts. We use these
expectations to evaluate whether our implementation is adhering to our core
philosophy (Node Autonomy, Invisible Mediation, and Fast Paths).

## Core Principles to Observe in Trajectories

1. **Fast Path Mediation:** Every trajectory begins with the `input_agent`
   (fast model) and every response to the user ends with the `output_agent`
   (fast model).
1. **Node Autonomy:** Agents should attempt to answer or execute tools
   themselves before defaulting to a complex execution graph. The
   `planning_agent` is expensive and should only be used for genuinely complex
   tasks.
1. **No Ghost Agents:** Every agent invocation must be recorded as a span in
   the OTel JSON trajectory.

______________________________________________________________________

### 1. The Trivial Greeting

- **Prompt:** `"Hello"` or `"What can you do?"`
- **Expected Spans:**
  1. `input_agent`: Reads prompt, outputs `ROUTE TO: swarm_agent`.
  1. `swarm_agent`: Generates a direct, helpful response.
  1. `output_agent`: Sanity checks the `swarm_agent`'s response, outputs `OK`.
- **Why:** The `input_agent` recognizes a social/meta query. It routes
  directly to the primary persona (`swarm_agent`). The `planning_agent` is
  completely bypassed. This journey should be extremely fast (< 3 seconds).

### 2. Single-Tool / Narrow Domain Task

- **Prompt:** `"How many Go files are there in this repo?"`
- **Expected Spans:**
  1. `input_agent`: Outputs `ROUTE TO: codebase-investigator` (or
     `swarm_agent`).
  1. `worker_agent`: Analyzes the request, executes a tool (e.g.,
     `bash_execute` running `find . -name "*.go" | wc -l`), and synthesizes
     the result.
  1. `output_agent`: Sanity checks the response, outputs `OK`.
- **Why:** This is a specific task that can be solved by one agent using one
  tool. Node autonomy dictates that the assigned agent simply executes the
  required tools. Generating a DAG via the `planning_agent` for this is a
  failure of efficiency.

### 3. Complex Multi-Step Task (Requiring a DAG)

- **Prompt:**
  `"Figure out the best way to write a mobile app for both iOS and Android sharing as much business logic in a single implementation as we can, then create a new git repo with a skeletal template that builds for both platforms"`
- **Expected Spans:**
  1. `input_agent`: Outputs `ROUTE TO: swarm_agent`.
  1. `swarm_agent`: Analyzes the request, recognizes it requires multi-domain
     coordination, and delegates to the `planning_agent` (or returns a
     `DEEP_PLAN_REQUIRED` signal).
  1. `planning_agent`: Decomposes the task into a JSON DAG (e.g., `t1`:
     Research with `web_researcher`, `t2`: Scaffold with `builder_agent`,
     `t3`: Initialize repo with `git_agent`).
  1. `t1 (web_researcher)`: Executes research. Checked by `output_agent`.
  1. `t2 (builder_agent)`: Executes scaffolding using `t1` context. Checked by
     `output_agent`.
  1. `t3 (git_agent)`: Executes git init using `t2` context. Checked by
     `output_agent`.
  1. `swarm_agent`: Synthesizes the final outcome to the user.
  1. `output_agent`: Final sanity check.
- **Why:** The task requires distinct phases (research, generation, git ops).
  The swarm correctly identifies the need for an execution graph and
  coordinates multiple sub-agents.

### 4. Error Recovery / Dynamic Replanning

- **Prompt:**
  `"Write a Python script using the 'foobar123_fake_lib' library."`
- **Expected Spans:**
  1. `input_agent`: Routes to a coding agent (`claude_code_agent` or
     `swarm_agent`).
  1. `worker_agent`: Attempts to install or use the library via
     `bash_execute`.
  1. `worker_agent`: Tool returns a `ModuleNotFoundError`. The agent
     recognizes the failure and emits a `request_replan` tool call or yields a
     specific error state.
  1. `planning_agent` (or `swarm_agent`): Consumes the failure context and
     mutates the graph (e.g., "Research alternative to foobar123").
- **Why:** Demonstrates Byzantine Swarm resilience. The agent tries, fails,
  and provides upward feedback to mutate the execution graph dynamically.
