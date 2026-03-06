# CUJ: Agentic Test Generation in CI

## User Persona

Alex is a platform infrastructure engineer. Their team frequently merges
fast-moving feature branches without adequate test coverage. Alex wants to use
Swarm not as an interactive terminal CLI, but as a headless, asynchronous
background worker in CI/CD to automatically fix this behavior without halting
developer velocity.

## Journey

### 1. The Trigger

A developer pushes a commit to a feature branch that modifies a complex
business logic file, `payment_gateway.go`. The webhook fires on GitHub.

### 2. The Concierge

A headless instance of Swarm runs as a GitHub Action responding to the `push`
event. The Swarm Concierge analyzes the commit diff.

> **[Swarm Concierge] (Internal Log):** Detected modification in
> `payment_gateway.go` with 0 new tests covering the added conditionals.
> Creating internal workflow: Spawn Worker to generate missing tests.

### 3. Asymmetric Work Dispatch

The Concierge uses the GitHub CLI to create an internal project tracking issue
or simply dispatches a sub-agent with the "Test Engineer" persona defined in
`ROLES.md`.

The `Test Engineer` clones the branch, runs the existing test suite locally
(`go test ./...`), and uses context from the changes to write extensive
table-driven tests for the new edge cases in `payment_gateway_test.go`.

### 4. Verification and Pull Request

After writing the tests, the `Test Engineer` runs `go test ./...` again to
verify the new tests pass locally. Once verified, it commits the file and
opens a formal Pull Request against the developer's original feature branch.

> **[Test Engineer] via GitHub PR Body:** "I noticed the structural changes to
> `Charge()` lacked test coverage for the null-currency edge case. I have
> generated passing table tests. Please review and merge these into your
> feature branch before shipping."

### 5. Seamless Developer Handoff

The original developer receives the review request, reads the cleanly
generated tests, and clicks "Merge". The feature ships safely, entirely
orchestrated by native Git integrations without ever requiring a human
developer to manually run local generation scripts.
