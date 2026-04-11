---
plan_id: P10
title: Render Input Aggregation
status: proposed
depends_on:
  - P09
consumes:
  - docs/plans/tech-plan-v1/09-answer-batch-orchestration.md
produces:
  - internal/stage/render/aggregate.go
  - runs/<run_id>/03_render/render_input.json
completion_evidence:
  - certified_results_only
  - minimum_success_rate_enforced
  - missing_personas_not_backfilled
---

# Goal

Build the render-stage input only from certified Stage 2 outputs and fail fast when the minimum success threshold is not met.

# Scope

- Read Stage 2 persona statuses.
- Select only certified worker results.
- Enforce the configured minimum success rate.
- Write `render_input.json`.

# Out Of Scope

- Producing final panel output.
- Rewriting Stage 2 answers.
- Guessing missing worker results.

# Required Inputs

- `docs/plans/tech-plan-v1/09-answer-batch-orchestration.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

- Add `internal/stage/render/aggregate.go`.
- Read only current-run Stage 2 per-persona artifacts under `runs/<run_id>/02_answer/`.
- Filter Stage 2 results to include only outcomes that are:
  - from the current run,
  - schema-valid,
  - explicitly `certified`, and
  - supported by valid Stage 2 artifacts for that same persona in this run.
- Compute minimum success-rate gating before render begins using only current-run Stage 2 artifact count.
- Write `render_input.json` for the final stage.

# Acceptance Checks

- The aggregator excludes failed, uncertified, or tool-contaminated worker outputs.
- The minimum success-rate gate is enforced before render execution.
  - Let `T` be the number of Stage 2 personas considered in the current run, and `C` the count of those with certified, valid outputs.
  - Configure `min_success_rate` (default `0.8` from source design).
  - Fail the gate immediately if `T = 0` before any division or threshold comparison.
  - Pass gate if and only if `T > 0`, `C > 0`, and `C / T >= min_success_rate`.
  - If `C` is zero, the gate fails regardless of `T`.
  - Equality at threshold (for example `C / T == 0.8` when `min_success_rate == 0.8`) passes.
- Missing personas are never backfilled or invented.

# Handoff

P11 consumes only `render_input.json` and the certified result set behind it.
