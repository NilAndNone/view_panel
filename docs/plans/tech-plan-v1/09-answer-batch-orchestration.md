---
plan_id: P09
title: Answer Batch Orchestration
status: proposed
depends_on:
  - P08
consumes:
  - docs/plans/tech-plan-v1/08-answer-single-worker-runner.md
  - docs/plans/tech-plan-v1/04-audit-log-pipeline.md
produces:
  - internal/stage/answer/batch.go
  - internal/orchestrator/run.go
completion_evidence:
  - persona_fanout_supported
  - forbidden_tool_detection_defined
  - aggregate_stage2_status_written
---

# Goal

Execute the full persona batch for Stage 2 with concurrency control, worker failure handling, and strict certification rules.

# Scope

- Fan out the single-worker runner across the persona set.
- Add concurrency limits and timeout handling.
- Interrupt and terminate workers that overrun.
- Define one persona outcome per worker in three mutually exclusive states: certified, failed, rejected.
- Derive Stage 2 per-persona outcomes directly from the single-worker artifacts and status from P08, without inventing a new aggregate schema.
- Map any forbidden-tool item to `failed` in the final Stage 2 outcome model.
- Keep `rejected` for invalid or missing final structured outputs produced by the P08 validation path.
- Aggregate per-persona outcomes into stage-level status for certified/failed/rejected reporting.

# Out Of Scope

- Stage 1 prompt generation.
- Render input aggregation.
- UI presentation of worker results.

# Required Inputs

- `docs/plans/tech-plan-v1/08-answer-single-worker-runner.md`
- `docs/plans/tech-plan-v1/04-audit-log-pipeline.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

- Add `internal/stage/answer/batch.go`.
- Extend `internal/orchestrator/run.go` to drive the full Stage 2 fan-out.
- Add forbidden-item detection for command, file, MCP, dynamic tool, collaboration, web, and image items and classify those outcomes as `failed`.
- Persist/compose only Stage 2 per-persona status artifacts in `02_answer` and classify them as certified, failed, or rejected.
- Write certified outcome summaries that later render stages can trust based on the existing artifacts/status from P08.

# Acceptance Checks

- Batch orchestration can run all personas with a configurable concurrency limit.
- Timeouts trigger interrupt-first shutdown before process kill.
- Forbidden tool items map to `failed` in the final per-persona outcome model.
- Invalid or missing final structured output from worker validation maps to `rejected`.
- Stage-level status is mutually exclusive per persona across certified, failed, and rejected.

# Handoff

P10 consumes only per-persona Stage 2 result/status artifacts under `02_answer` by filtering on certification outcome. No new aggregate file is introduced.
