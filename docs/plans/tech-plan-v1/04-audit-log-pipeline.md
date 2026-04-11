---
plan_id: P04
title: Audit Log Pipeline
status: proposed
depends_on:
  - P02
  - P03
consumes:
  - docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md
  - docs/plans/tech-plan-v1/03-run-layout-and-artifact-writer.md
produces:
  - internal/audit/logger.go
  - internal/audit/summary.go
completion_evidence:
  - protocol_in_out_logs_defined
  - normalized_events_defined
  - run_summary_and_global_index_defined
---

# Goal

Implement the audit pipeline that makes `run_root` the primary evidence source for every run and every worker, with `run_summary.json` generated as a derived view from authoritative run artifacts and normalized events under `run_root`.

# Scope

- Persist `protocol.out.jsonl` and `protocol.in.jsonl` under each stage/worker path beneath `run_root`.
- Persist normalized `events.jsonl` under each stage/worker path beneath `run_root`.
- Persist run-level `run_summary.json`.
- Persist a locator-only `run_index.jsonl` append-only file.

# Out Of Scope

- Generating Stage 1 or Stage 2 content.
- Worker prompt rendering.
- Render-stage aggregation.

# Required Inputs

- `docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md`
- `docs/plans/tech-plan-v1/03-run-layout-and-artifact-writer.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

- Add `internal/audit/logger.go` for raw protocol and normalized event recording.
- Add `internal/audit/summary.go` for stage summary and global index writing.
- Define the event fields required by the incident-driven audit chain.
- Add tests for append-only JSONL output and summary completeness.

# Acceptance Checks

- Every worker-capable stage can emit both raw protocol logs and normalized events.
- The global index is documented as a locator-only index, append-only, and is not blocked by missing or delayed `run_summary.json` generation.
- Missing audit artifacts can be surfaced as run failure conditions.

- `run_summary.json` is treated as a derived summary and never overrides the canonical evidence in `run_root` artifacts/events.

# Handoff

P06 and P09 use this audit layer to record stage decisions and worker outcomes.
