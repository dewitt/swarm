# Design: Global Trajectory Persistence & Telemetry

## 1. Context

Currently, the `swarm` CLI successfully captures and persists execution
trajectories during normal interactive operations (stored in
`~/.config/swarm/trajectories/`). However, during end-to-end evaluations
(`swarm eval`), the Swarm Engine is instantiated inside a temporary, ephemeral
sandbox environment (`/tmp/swarm-eval-*`). This sandbox is deleted immediately
after the evaluation scenario completes, causing those specific execution
trajectories to be lost.

Furthermore, we lack a unified mechanism to categorize and "donate" *any* of
these valuable trajectories (interactive or evaluation) back to the community.
These artifacts are highly valuable for:

- **Debugging:** Understanding *why* an agent failed a specific rubric.
- **Training Data (Epic #29):** Mining these trajectories for dynamic prompt
  optimization and automated skill generation.
- **Regression Analysis:** Comparing how trajectory lengths or tool usage
  changes across CLI versions.

## 2. Objective

1. Build a mechanism into the `swarm eval` command to automatically persist
   the resulting evaluation trajectories to a durable location before the
   temporary sandbox is destroyed.
1. Establish a global foundation for voluntarily "donating" **any** trajectory
   (interactive or evaluation) to a centralized dataset to improve the Swarm
   ecosystem.

## 3. Proposed Solution

### 3.1. Durable Storage Location

Evaluation trajectories should be stored alongside standard interactive
trajectories, but distinctly categorized so they do not pollute the user's
personal session history.

- **Proposed Path:** `~/.config/swarm/eval_trajectories/`
- Standard interactive sessions will remain in
  `~/.config/swarm/trajectories/`.

### 3.2. The `--donate` Flag (Global Telemetry Opt-In)

To support the vision of collective improvement, we will introduce a global
telemetry opt-in mechanism.

- Users can run `swarm eval --donate` to explicitly mark a batch of evaluation
  runs for donation.
- Users can set `swarm config set telemetry true` to implicitly mark **all**
  trajectories (both interactive CLI sessions and evaluations) for donation.
- **Short-Term:** The CLI will explicitly log that the trajectory was saved
  and "marked for donation" (e.g., by adding a `"donate": true` field to the
  JSON payload or saving it to a dedicated `~/.config/swarm/donations/`
  directory).
- **Long-Term:** A background worker or subsequent command (e.g.,
  `swarm telemetry push`) will securely scrub these annotated trajectories for
  personal secrets and transmit them to a centralized Swarm dataset (e.g., a
  HuggingFace dataset or dedicated API).

### 3.3. Architecture Changes

**A. `pkg/sdk/swarm.go` (Centralized Persistence)**

1. The SDK currently handles trajectory persistence for interactive sessions
   via `saveTrajectory()`.
1. We will expand the SDK's initialization options (or configuration) to
   accept a custom `TrajectoryDir`.
1. The SDK will be responsible for injecting the `"donate": true` marker into
   the JSON payload if the global `Telemetry` config is true or if overridden
   by a runtime context flag.

**B. `pkg/eval/eval.go` (The Evaluator Engine)**

1. The `Evaluator` will configure the embedded Swarm Engine to use the
   `eval_trajectories` subdirectory.
1. Crucially, the evaluator must use `defer` to ensure the trajectory is
   extracted and flushed to disk *before* the temporary sandbox is destroyed,
   guaranteeing we capture data even if the scenario panics or fails
   catastrophically.

**C. `cmd/swarm/eval.go` (The CLI Handler)**

1. The CLI handler remains a "dumb client". It does not handle file I/O or
   JSON marshaling.
1. It only parses the `--donate` flag and passes it down into the `pkg/eval`
   evaluator configuration.

### 3.4. File Structure Example

```text
~/.config/swarm/
├── config.yaml
├── history.json
├── trajectories/           # Standard interactive sessions
│   └── tr-12345.json
└── eval_trajectories/      # New: Evaluation artifacts
    ├── scenario_1_20260307T120000_pass.json
    └── scenario_2_20260307T120500_fail.json
```

## 4. Implementation Steps

1. **Update SDK Configuration (`pkg/sdk/config.go` & `pkg/sdk/swarm.go`):**

   - Add `Telemetry bool` to the global `Config` struct.
   - Update `SwarmConfig` to accept `TrajectoryDir` and `ForceDonate`.
   - Update the internal `saveTrajectory` method to utilize these new fields.

1. **Update `pkg/eval/eval.go`:**

   - Configure the internal `SwarmConfig` to use `eval_trajectories`.
   - Implement `defer` blocks to explicitly flush and save the trajectory
     before the `os.RemoveAll(sandbox)` trigger executes.

1. **Update CLI Flags (`cmd/swarm/eval.go`):**

   - Add the `--donate` flag to the `eval` command.
   - Pass this flag to the `evaluator.Run` options.

1. **Verify:**

   - Run `swarm eval scenario_1_linter --donate`.
   - Verify the `~/.config/swarm/eval_trajectories/` directory is created and
     populated with the valid JSON file containing the donation marker.
