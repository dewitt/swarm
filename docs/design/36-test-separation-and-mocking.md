# Test Separation & Mocking Strategy

**Status:** Proposed
**Author:** Gemini CLI Swarm
**Date:** March 2026

## Objective

The current test suite for `swarm` suffers from flakiness and high latency because core logic tests are implicitly reliant on live LLM calls (e.g., `TestSemanticMemoryE2E`). As the framework grows, these live API calls introduce non-determinism, monetary cost, and network delays.

This document proposes a formalized strategy to cleanly separate deterministic **Unit Tests** (which run locally in milliseconds) from non-deterministic **End-to-End (E2E) Tests** (which exercise live Frontier Models), ensuring that `go test ./...` remains lightning-fast and entirely reliable. Crucially, we must not simply punt difficult tests to the E2E suite; core logic must be aggressively mocked.

## 1. The Separation Mechanism

We will use Go's standard environment variable approach combined with file naming conventions to enforce a strict boundary between test suites.

*   **Unit Tests (`*_test.go`):** Run by default when executing `go test ./...`. These tests MUST NOT make external network calls to live LLMs.
*   **Live E2E Tests (`*_e2e_test.go`):** Require explicit opt-in. They will only run if a specific environment variable is set (e.g., `SWARM_RUN_E2E=1`).

If an E2E test is executed without the explicit environment variable, it must `t.Skip("Skipping live E2E test; set SWARM_RUN_E2E=1 to run")`. This prevents accidental triggers during rapid local development.

## 2. Core Logic Must Be Mocked

The easiest (and worst) solution to a flaky live test is to simply label it `e2e` and ignore it. This degrades our test coverage over time.

Instead, tests that evaluate **internal orchestration logic** (e.g., parsing JSON plans, routing logic, memory extraction, tool selection) must be rewritten to use Mock LLMs.

### The `MockModel` Interface

We will expand our existing `MockModel` (partially implemented in `pkg/sdk/swarm_test.go`) into a fully robust testing primitive.

The `MockModel` must be capable of:
1.  **Asserting Prompts:** Verifying that the internal SDK logic constructed the correct prompt string (e.g., checking if semantic memories were injected properly).
2.  **Simulating Responses:** Returning predetermined strings, JSON blocks, or structured function calls depending on the incoming prompt's content.
3.  **Simulating Failures:** Returning network errors or malformed JSON (like the markdown wrap we just fixed) to test the Swarm's error-handling and replanning resilience.

**Example: Rewriting `TestSemanticMemory`**
Instead of hitting Gemini to extract a fact, the unit test will:
1. Initialize the Swarm with `MockModel`.
2. Feed it a mock trajectory.
3. Assert that the SDK constructed the correct `Reflect` prompt.
4. Provide a mock JSON response from `MockModel` containing the extracted fact.
5. Assert that the SDK correctly saved that fact to the SQLite database.

## 3. The Role of Live E2E Tests

If we mock the logic, what are E2E tests for?

Live tests (`*_e2e_test.go` and the `eval/` suite) are strictly reserved for testing **Model Capabilities and Prompt Efficacy**.

*   *Are the models still smart enough to solve Scenario 6?*
*   *Did a new prompt tweak cause Claude to start looping?*
*   *Is the Gemini API currently experiencing latency?*

These tests validate the "fat models" part of our philosophy. They are expensive, slow, and non-deterministic by nature, and should be run on a dedicated CI cron schedule or explicitly before a release.

## 4. Implementation Plan

1.  **Refactor Mocks:** Extract and enhance `MockModel` from `swarm_test.go` into a shared testing utility package (e.g., `pkg/sdk/testutils`).
2.  **Audit Existing Tests:** Review every test in `pkg/sdk/`. Identify tests making live API calls.
3.  **Convert to Unit Tests:** Rewrite logic-focused live tests (like memory extraction or routing) to use the new `MockModel`.
4.  **Isolate E2E Tests:** For tests that truly require live reasoning, ensure they are in files ending in `_e2e_test.go` and guarded by `if os.Getenv("SWARM_RUN_E2E") == "" { t.Skip(...) }`.
