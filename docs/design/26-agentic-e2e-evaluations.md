# Design Doc 26: Agentic E2E Evaluations (LLM-as-a-Judge)

**Status**: Proposed **Author**: Antigravity **Date**: March 2026

## Objective

To define a robust, repeatable methodology for end-to-end (E2E) testing of
Swarm's autonomous behaviors. Because agentic workflows are non-deterministic,
traditional unit testing (asserting exact string matches) is insufficient. We
will build a dedicated E2E test suite that runs "real world" tasks in
sandboxed environments and uses frontier LLMs as the evaluator
("LLM-as-a-Judge") to score the final artifacts and trajectories.

## The Philosophy: Evaluation over Assertion

When testing traditional software, `assert(output == "Hello World")` is the
gold standard. When testing multi-agent swarms, the output is often a deeply
nuanced markdown document, a refactored codebase, or an architectural debate.

We must shift from **Assertion** to **Evaluation**:

1. **The Sandbox:** Every test must run in a perfectly isolated, reproducible
   environment (e.g., an ephemeral `t.TempDir()` initialized with a specific
   git commit or boilerplate).
1. **The Execution:** The `Swarm` engine runs the user prompt against the
   sandbox.
1. **The Judge:** A separate, explicitly configured LLM (e.g.,
   `gemini-2.5-pro` parameterized with temperature 0.0) reads the resulting
   sandbox state and the Swarm's SQLite trajectory.
1. **The Rubric:** The Judge is given a strict grading rubric (e.g., "Pass the
   test ONLY if the `Cache` struct contains `sync.Mutex` AND the `TestCache`
   file exists").

## Technical Architecture

We will implement this not as standard `_test.go` files that run on every
commit, but as a dedicated `swarm eval` command or a separate suite
(`go test -tags=eval ./...`) executed nightly or pre-release.

### 1. Test Definition Format (`.yaml` or `.json` fixtures)

Each E2E test is defined by a fixture:

- **Name:** "Bugfix: Concurrent Map Writes"
- **Fixture Path:** `./eval/fixtures/bug_concurrent_map/` (contains broken
  code)
- **Prompt:** "There is a concurrent map write panic in this generic cache.
  Fix it and write a test proving it works."
- **Judge Rubric:** "Did the agent use `sync.RWMutex` or `sync.Map`? Does
  `go test` pass in the workspace? If both are true, output PASS. Otherwise,
  output FAIL with reasoning."

### 2. The Evaluator Runtime (`pkg/eval/`)

A new Go package `pkg/eval` will handle the orchestration:

1. Copy the fixture array to a temp directory.
1. Instantiate `swarm.New()` pointing its `ConfigDir` to that temp directory
   (isolating skills and SQLite).
1. Execute the `Prompt`.
1. Capture the final SQLite Trajectory and the full file diff of the temp
   directory.
1. Send the Diff + Trajectory + Rubric to the `JudgeModel`.

## Proposed Initial Scenarios (The "Half-Dozen")

To prove the efficacy of this Evaluation system, we should implement these 6
real-world scenarios immediately:

### 1. The Linter Fix (Low Complexity)

- **Sandbox:** A Go project with 15 `golangci-lint` violations (unused vars,
  shadowed vars, missing comments).
- **Prompt:** "Run the linter and fix all issues."
- **Rubric/Judge:** The suite natively runs `golangci-lint run`. Only passes
  if exit code is 0. (No LLM judge technically needed, proves tool execution
  baseline).

### 2. The Unfamiliar API Migration (Medium Complexity)

- **Sandbox:** A Python script using the deprecated `requests` library.
- **Prompt:** "Migrate this script to use `httpx` with async/await. You must
  read the web documentation for `httpx` if you don't know the syntax."
- **Rubric/Judge:** The LLM Judge inspects the diff. It fails the test if
  `requests` is still imported, or if `async/await` is not used properly.

### 3. The Security Audit (High Complexity/Multi-Agent)

- **Sandbox:** A Node.js Express server with a blatant SQL injection
  vulnerability and a hardcoded JWT secret.
- **Prompt:** "I am the CTO. Have a Security Expert agent audit `server.js`
  and fix any critical vulnerabilities."
- **Rubric/Judge:** The LLM Judge evaluates the git diff. Did the agent
  parameterize the SQL queries? Did it move the JWT secret to `process.env`?

### 4. Git-Native PR Review (The Coordination Eval)

- **Sandbox:** A local git repository with a branch containing a buggy
  refactor.
- **Prompt:** "Act as a strict code reviewer. Review the diff on this branch
  against main. Leave a markdown file `REVIEW.md` containing your critique."
- **Rubric/Judge:** The LLM Judge reads `REVIEW.md`. Did the agent catch the
  specific bug seeded in the fixture? Is the tone appropriately professional?

### 5. The "It Compiles But Is Wrong" Logic Bug

- **Sandbox:** A Go function computing the Fibonacci sequence, but initialized
  with `[0, 0]` instead of `[0, 1]`. The tests pass because someone wrote bad
  tests.
- **Prompt:** "Users are reporting the Fibonacci endpoint returns 0 for
  everything. Figure out why and fix both the code and the tests."
- **Rubric/Judge:** The LLM Judge checks if the agent successfully identified
  that the *tests* were the source of the false positive, modified the tests,
  and fixed the initialization array.

### 6. The "Self-Healing" Recovery Trace (The Meta Eval)

- **Sandbox:** A project that requires `protoc` to build, but the `protoc`
  binary is intentionally missing from the `$PATH` in the execution sandbox.
- **Prompt:** "Build this project."
- **Rubric/Judge:** The LLM Judge reads the *Trajectory*. It expects the agent
  to fail the first `bash_execute("make build")`, see the "protoc not found"
  error, deduce it needs to install `protobuf-compiler` (or download the
  binary), install it, and retry. The test passes if the trajectory contains
  this specific error-recovery loop and the final build succeeds.

## Conclusion

By treating E2E tests as semantic grading rather than strict binary
assertions, we can measure the *reasoning* and *resilience* of the Swarm. This
test suite will become the primary benchmark for deciding when it is safe to
tag a new major release.
