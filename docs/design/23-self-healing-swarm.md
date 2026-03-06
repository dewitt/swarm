# Design Doc 23: The Self-Healing Swarm

**Status**: Proposed **Author**: Antigravity **Date**: March 2026

## Objective

To establish a vision and architectural roadmap for evolving Swarm from a
reactive command-line tool into a proactive, **self-healing ecosystem**. By
treating our `.config/swarm/trajectories/` local database not just as an audit
log, but as a rich training dataset, Swarm can autonomously learn, adapt, and
optimize its own behavior over time.

## Background & Philosophy

Currently, Swarm agents operate in isolated sessions. While they are resilient
*within* a session (capable of `request_replan` or fixing a broken
`go build`), they suffer from amnesia *between* sessions. A failure mode
encountered today will likely be repeated tomorrow.

Aligning with our core ethos of **Zero-HITL** and **Fat Models, Thin
Software**, we must leverage the immense reasoning capabilities of models to
build a self-repairing telemetry loop. If developers can manually parse
trajectory JSONs to find bugs, the Swarm itself should be able to do the same
asynchronously.

## Analysis of Recent Trajectories

An automated analysis of the last 20 execution trajectories revealed:

1. Massive reliance on raw `bash_execute`, indicating missing specialized
   tools.
1. Environmental roadblocks causing hard replans (e.g., missing API keys or
   credit balances for secondary agents).
1. Complex multi-agent parallel workflows that succeed but take significant
   time (upwards of 100 seconds) due to repeated execution of boilerplate
   scaffolding.
1. Heavy utilization of the `output_agent` for sanity checks, which
   effectively catches errors but at the cost of latency.

______________________________________________________________________

## 10 Ideas for a Self-Healing Swarm

Based on trajectory data and Swarm's philosophy, here are 10 concrete
mechanisms to implement self-healing:

### 1. Trajectory Mining & Pattern Recognition

Run a background "Swarm Analyst" agent that periodically reviews the SQLite
trajectory DB during idle time. It will identify systemic failure modes (e.g.,
repeatedly failing `bash_execute` syntaxes) and globally inject avoidance
heuristics into the `System Prompt` of the routing agent.

### 2. Automated Skill Generation (`SKILL.md` Synthesis)

Trajectories show high reliance on generic bash commands for specific tasks
(e.g., JSON parsing, Git rebasing). When the Analyst detects repeated
programmatic sequences, Swarm should autonomously draft and propose a
specialized `SKILL.md` to formalize that capability, reducing token usage and
execution fragility in future sessions.

### 3. Dynamic Prompt Optimization (DPO)

Analyze the fastest, most successful trajectories to extract the optimal
conversational paths. Use these highlighted trajectories as automatic few-shot
examples or dynamic context injections to implicitly fine-tune the
`input_agent` to recognize intent faster.

### 4. Proactive Environment Remediation

When a tool fails due to local environment issues (e.g., "Claude Code CLI
unavailable", "go fmt command not found"), Swarm currently throws an error or
aborts. A self-healing Swarm will detect the missing dependency, pause the
overarching goal, execute a sub-plan to *fix the environment* (e.g.,
`apt-get install`, or open a prompt asking for an API key), and seamlessly
resume.

### 5. Self-Monitoring Load Balancing

Trajectories track token usage, durations, and API latency. If Swarm detects
systemic timeouts or 429 errors from Google Gemini, it should autonomously
trigger the smart fallback logic (Design Doc 21) based on historical
degradation thresholds, shifting to Claude or local models without human
intervention.

### 6. Automated "Post-Incident" Artifacts

For any session that required multiple `request_replan` spans or exceeded
latency thresholds, Swarm will generate a localized `incident_report.md` in
the workspace. This acts as a feedback loop, summarizing *why* the agents got
stuck and politely asking the user for architectural guidance for next time.

### 7. Memory Compaction & Context Decay Prevention

Long trajectories lead to context window saturation and hallucinations. By
analyzing at what point agents typically lose focus, we can introduce an
asynchronous "Memory Compactor" agent that watches the live stream and
replaces verbose history with dense, factual summaries the moment the context
reaches 80% capacity.

### 8. Semantic Test Generation (Workflow Regression)

Convert successful, complex user workflows directly into executable
integration tests. If an agent successfully scaffolds a new UI module after 5
iterations, it should synthesize those steps into a test script. If a future
change breaks this learned workflow, the swarm detects it via local regression
and self-corrects the codebase.

### 9. Tool Reliability Scoring

Maintain a moving average reliability score for every tool registered in the
`Adk`. If `bash_execute` degrades below an 80% success rate on a specific
machine architecture, the planner will dynamically deprecate it for that
session and favor alternative tools, warning the user of the system
instability.

### 10. Heuristic Deadlock Detection ("Overwatch")

Trajectories occasionally reveal logic loops (e.g., Agent A asks Agent B,
Agent B fails and asks Agent A). A lightweight `Overwatch` routine will
monitor the live event stream, detect cyclic or repetitive spans using
trajectory embedding similarity, forcefully interrupt the live span, and
mandate a hard structural replan.

## Next Steps

1. Implement the `Swarm Analyst` background chron job within the CLI.
1. Augment the `Memory` functionality (`/remember`) to accept automated
   updates from the Analyst, not just manual user inputs.
1. Build the reliability scoring matrix into the `pkg/sdk/tools.go`
   interceptors.
