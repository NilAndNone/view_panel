---
plan_id: P08
title: Answer Single Worker Runner
status: proposed
depends_on:
  - P07
consumes:
  - docs/plans/tech-plan-v1/07-answer-worker-workspace.md
  - docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md
produces:
  - internal/stage/answer/runner.go
  - schemas/worldview_worker_result_v1.json
completion_evidence:
  - single_worker_turn_executes
  - final_agent_message_captured
  - result_artifacts_written
---

# Goal

Run one Stage 2 worker from start to finish and persist only the final structured result and attestation artifacts.

# Scope

- Start one app-server process for one persona.
- Run one thread and one turn.
- Parse only `item/completed.agentMessage` as final output.
- Validate the worker result against `worldview_worker_result_v1`.
- On success, persist authoritative structured `result.json` and the supporting artifacts.
- Persist `result.raw.txt` only as diagnostic/raw output capture; it is not the canonical structured artifact.
- `attestation.json` is the certification/validation record for the final run.
- When parsing, missing final completed message, or schema validation fails, record failure in `status.json` with a rejection reason and do not claim success in `result.json`.

# Out Of Scope

- Batch scheduling across personas.
- Retry policy across workers.
- Render-stage synthesis.

# Required Inputs

- `docs/plans/tech-plan-v1/07-answer-worker-workspace.md`
- `docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md`

# Implementation Tasks

- Add `schemas/worldview_worker_result_v1.json`.
- Add `internal/stage/answer/runner.go`.
- Wire the runner to read workspace inputs and write worker outputs.
- Ensure only `result.json` is authoritative for downstream consumers after successful schema validation.
- Define explicit failure behavior: write raw and status artifacts with rejection reason on parse/final-completed/schema validation failures; keep `result.json` absent when no valid result is available.
- Use `attestation.json` as the explicit validation/certification record for the run.

# Acceptance Checks

- The runner reads only pre-rendered inputs from the workspace contract.
- Final output comes from `item/completed.agentMessage` only.
- `result.raw.txt` is always written as diagnostic capture, while `result.json` is only written after successful completion + schema validation.
- `status.json` always records success/failure and includes rejection reason on failure.
- `attestation.json` records pass/fail certification status tied to schema validation and completed-message checks.

# Handoff

P09 scales this single-worker runner across the persona set.
