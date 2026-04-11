---
plan_id: P11
title: Render Stage Output
status: proposed
depends_on:
  - P10
consumes:
  - docs/plans/tech-plan-v1/10-render-input-aggregation.md
  - docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md
produces:
  - internal/stage/render/render.go
  - runtime/skills/wv-render-stage/SKILL.md
  - schemas/panel_result_v1.json
completion_evidence:
  - render_session_runs_once
  - panel_outputs_written
  - run_completion_status_finalized
---

# Goal

Run the render stage once over the certified result set and write the final panel artifacts and end-of-run status under the current run's `03_render` output root.

# Scope

- Add the `wv-render-stage` skill.
- Add the `panel_result_v1` schema.
- Run one render session over `render_input.json`.
- Write `panel.json`, `panel.md`, `cards.json`, and final render status as `status.json` under the run's `03_render` output area.
- Own only render output paths under `03_render`; do not write to or overwrite any `02_answer` Stage 2 paths.

# Out Of Scope

- Editing Stage 2 answers.
- Filling in missing worker responses.
- Changing the Stage 1 packet.

# Required Inputs

- `docs/plans/tech-plan-v1/10-render-input-aggregation.md`
- `docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md`

# Implementation Tasks

- Add `runtime/skills/wv-render-stage/SKILL.md`.
- Add `schemas/panel_result_v1.json`.
- Add `internal/stage/render/render.go` to execute the render session and persist outputs.
- Finalize end-of-run render status by writing `03_render/status.json` to record terminal render outcome at the plan level.
- Finalize audit summary links.

# Acceptance Checks

- Render reads only `render_input.json` and certified worker results.
- `03_render` is the canonical render output root; writes are limited to `panel.json`, `panel.md`, `cards.json`, and `status.json` within that area.
- Stage 2 artifacts under `02_answer` are read-only inputs.
- Render does not modify, replace, or overwrite any file in `02_answer`.
- `panel.json`, `panel.md`, `cards.json`, and `status.json` are written together.

# Handoff

This is the terminal plan in the chain. Its outputs feed user-facing inspection and final run reporting.
