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

**A. `pkg/eval/eval.go` (The Evaluator Engine)**

1. The `Run` method currently returns a `*Result` which includes a
   `Trajectory string`. However, this is just a stringified console dump, not
   the structured JSON `Trajectory` object produced by the SDK.
1. We need to modify `eval.go` to capture the structured JSON `Trajectory`
   object from the sandbox instance before the instance is destroyed.
1. The `Evaluator.Run` method should be updated to return the raw
   `sdk.Trajectory` alongside the evaluation `Result`.

**B. `cmd/swarm/eval.go` (The CLI Handler)**

1. After executing a scenario and receiving the result (and the structured
   `Trajectory` object), the CLI handler will marshal the trajectory to JSON.
1. The handler will write this JSON file to the durable `eval_trajectories`
   directory.
1. The filename should include the scenario ID, a timestamp, and whether it
   passed or failed. Format: `{scenario_id}_{timestamp}_{status}.json` (e.g.,
   `scenario_1_20260307T120000_pass.json`).

### 3.3. File Structure Example

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

1. **Update `pkg/eval/eval.go`:**

   - Modify the `Run` method to extract the `Trajectory` from the `sdk.Swarm`
     instance after `instance.Chat()` completes. (Note: The `sdk.Swarm`
     interface might need a new method like `GetLastTrajectory()` if it
     doesn't expose it natively).
   - Add a `RawTrajectory` field of type `sdk.Trajectory` to the `eval.Result`
     struct.

1. **Update `cmd/swarm/eval.go`:**

   - In the execution loop, after `evaluator.Run()` returns, extract the
     `RawTrajectory`.
   - Determine the global configuration directory using `sdk.GetConfigDir()`.
   - Ensure the `eval_trajectories` subdirectory exists (`os.MkdirAll` with
     `0o755`).
   - Write the marshaled JSON to the structured filename using `os.WriteFile`
     with `0o600` permissions.
   - Print a small log line in verbose mode indicating where the trajectory
     was saved.

1. **Update CLI Flags and Configuration:**

   - Add the `--donate` flag to the `eval` command to explicitly mark
     evaluation runs for donation.
   - Update the global `config.yaml` to include a `telemetry: true/false`
     field.
   - Update `pkg/sdk/swarm.go` to check the `telemetry` config flag or
     `--donate` flag and add a `"donate": true` marker to the JSON payload
     before saving the trajectory.

1. **Verify:**

   - Run `swarm eval scenario_1_linter --donate`.
   - Verify the `~/.config/swarm/eval_trajectories/` directory is created and
     populated with the valid JSON file containing the donation marker.
