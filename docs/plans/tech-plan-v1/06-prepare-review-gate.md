---
plan_id: P06
title: Prepare Review Gate
status: proposed
depends_on:
  - P04
  - P05
consumes:
  - docs/plans/tech-plan-v1/04-audit-log-pipeline.md
  - docs/plans/tech-plan-v1/05-prepare-input-builder.md
produces:
  - runtime/skills/wv-prepare-stage/SKILL.md
  - schemas/prepare_review_v1.json
completion_evidence:
  - prepare_review_written
  - prepare_status_written
  - stage2_gate_enforced
---

# Goal

Insert a review-only gate between canonical input preparation and Stage 2 execution.

# Scope

- Add the `wv-prepare-stage` skill.
- Define the `prepare_review_v1` schema.
- Run one run-level review session over the full prepared persona set after preparation (exactly one session total, not one per persona).
- Persist `prepare_review.json` and `prepare_status.json`.

# Out Of Scope

- Rewriting the canonical packet.
- Starting any Stage 2 worker.
- Rendering final panel output.

# Required Inputs

- `docs/plans/tech-plan-v1/04-audit-log-pipeline.md`
- `docs/plans/tech-plan-v1/05-prepare-input-builder.md`

# Implementation Tasks

- Add `runtime/skills/wv-prepare-stage/SKILL.md`.
- Add `schemas/prepare_review_v1.json`.
- Extend the Stage 1 pipeline to run the review session after packet generation.
- Fail the stage if review or completeness checks do not pass.

# Acceptance Checks

- The review step cannot mutate any canonical Stage 1 artifact.
- `prepare_status.json` is a run-level gate checked before Stage 2 fan-out begins.
- Stage 2 starts only when the single run-level `prepare_status.json` marks the prepared set as ready; per-persona prepared artifacts remain under the P05 output layout and are not individually used as Stage 2 gates.
- Review decisions are captured in the audit stream.

# Handoff

P07 can start only when this gate marks the run ready for Stage 2.
