# 21. Smart Model Fallback and Dynamic Provider Rerouting

## Objective

As the Swarm CLI matures, the core architecture must gracefully handle the
inherent unreliability of external LLM APIs. Timeouts, quota exhaustion, 503
service unavailability, and unexpected model degradation are the reality of
working with modern API providers. Currently, Swarm relies heavily on a single
primary configuration (e.g. Gemini 2.5 Flash / Pro). If this provider goes
down, the entire agentic system halts.

This document outlines the architectural necessity of **Smart Model
Fallbacks**, moving Swarm from a static model consumer to a resilient,
self-healing routing mesh.

## The Problem

1. **Network Fragility:** Even with aggressive internal HTTP timeouts (e.g.
   45s TTFT bounds), a model timeout currently forces the orchestration engine
   to cleanly fail the span. While the system can _replan_, if the underlying
   endpoint is wholly unresponsive, replanning to the exact same model will
   result in an infinite failure loop.
1. **Quota and Rate Limits:** Agents operating autonomously (like the
   `codebase-investigator`) can burn through token quotas quickly.
1. **Task-Specific Degradation:** Certain models excel at logic but fail at
   formatting or specific tool use.

## Core Mechanisms to Implement

### 1. The Fallback Cascade

The configuration layer should support an array of models/providers rather
than a single string.

```yaml
models:
  primary: "gemini-3.1-pro-preview"
  fallback:
    - "gemini-2.5-flash"
    - "claude-3-5-sonnet-latest" # Assuming multi-provider support
```

If the primary model fails with a `429 Too Many Requests`,
`503 Service Unavailable`, or a hard `context deadline exceeded`, the
`genai.Client` multiplexer should automatically catch the error and seamlessly
retry the exact same prompt transparently against the next model in the
cascade.

### 2. Autonomous Provider Switch (The "Circuit Breaker")

Swarm's `Engine` must possess a "circuit breaker" logic.

- If a specific endpoint fails three times consecutively across the swarm, the
  state manager should trip the breaker and open the circuit.
- All subsequent spans are immediately routed to the secondary provider until
  a background health-check ping demonstrates the primary is back online.

### 3. Graceful Capability Downgrading

When falling back from a highly intelligent model (Pro) to a faster/cheaper
model (Flash), the Engine should be aware of context window discrepancies and
tool-use capabilities. If falling back means losing extensive context, the
Engine may need to automatically summarize the `session.Events` before
dispatching the request to the smaller model.

## User Experience

The user should be aware but not burdened:

- `[Swarm] ⚙️ Gemini Pro timed out. Automatically falling back to Claude 3.5 Sonnet...`

This enables true "fire and forget" reliability for long-running workflows
that the user kicks off and walks away from.
