---
plan_id: P11
title: Render Outputs
status: ready
depends_on:
  - P02
  - P10
consumes:
  - docs/plans/tech-plan-v1.1-runtime/02-app-server-client-lifecycle.md
  - docs/plans/tech-plan-v1.1-runtime/10-render-input-aggregation.md
produces:
  - internal/stage/render/render.go
  - runtime/skills/wv-render-stage/SKILL.md
  - schemas/panel_result_v1.json
  - runs/<run_id>/03_render/raw_panel.json
  - runs/<run_id>/03_render/raw_panel.md
  - runs/<run_id>/03_render/raw_cards.json
  - runs/<run_id>/03_render/certified_panel.json
  - runs/<run_id>/03_render/certified_panel.md
  - runs/<run_id>/03_render/certified_cards.json
  - runs/<run_id>/03_render/status.json
completion_evidence:
  - raw_branch_can_complete_without_certified_branch
  - certified_outputs_require_p10_certified_availability
  - status_records_rendered_skipped_failed_for_both_branches
---

# Goal

Execute the terminal Stage 3 render stage from the P10-owned render-input artifacts into final operator-facing outputs under `runs/<run_id>/03_render/`, without reopening Stage 2 state and without modifying any artifact under `runs/<run_id>/02_answer/`.

# Scope

- P11 owns only final render execution and final output writing in:
  - `internal/stage/render/render.go`
  - `runtime/skills/wv-render-stage/SKILL.md`
  - `schemas/panel_result_v1.json`
- P11 reads only current-run P10 artifacts from `runs/<run_id>/03_render/`:
  - `raw_render_input.json`
  - `certified_render_input.json`
- P11 may use the P02 app-server lifecycle to run up to two single-turn render workers, one per eligible branch.
- Branch execution is fixed and independent:
  - evaluate the raw branch first
  - evaluate the certified branch second
  - run the raw branch only when `raw_render_input.json.available == true`
  - skip the raw branch only when `raw_render_input.json.available == false`
  - run the certified branch only when `certified_render_input.json.available == true`
  - skip the certified branch only when `certified_render_input.json.available == false`
  - treat `certified_render_input.json.available` as the upstream P10 gate result and never recompute why it is false
  - one branch being skipped must not suppress the other branch
- Successful branches write exactly three final artifacts:
  - `raw_panel.json` or `certified_panel.json`
  - `raw_panel.md` or `certified_panel.md`
  - `raw_cards.json` or `certified_cards.json`
- `status.json` is always written and records branch state as exactly one of:
  - `rendered`
  - `skipped`
  - `failed`
- P11 never modifies:
  - `runs/<run_id>/02_answer/**`
  - `runs/<run_id>/03_render/raw_render_input.json`
  - `runs/<run_id>/03_render/certified_render_input.json`

# Out Of Scope

- Any read from or write to `runs/<run_id>/02_answer/**`
- Any recomputation of P10 availability, omissions, displayability, or certified gate math
- Any redesign of P10 raw/certified render-input contracts
- Any Stage 2 recovery, backfill, or placeholder generation
- Any output beyond the final `03_render` branch artifacts and `status.json`

# Required Inputs

- `docs/plans/tech-plan-v1.1-runtime/02-app-server-client-lifecycle.md`
- `docs/plans/tech-plan-v1.1-runtime/10-render-input-aggregation.md`
- current-run `runs/<run_id>/03_render/raw_render_input.json`
- current-run `runs/<run_id>/03_render/certified_render_input.json`

# Implementation Tasks

1. Define the authoritative rendered-panel schema in `schemas/panel_result_v1.json`.
   - Use one schema for both `raw_panel.json` and `certified_panel.json`.
   - Require these top-level keys:
     - `schema_version`, exact literal `worldview_panel_result_v1`
     - `stage`, exact literal `03_render`
     - `variant`, exact literal `raw` or `certified`
     - `run_id`
     - `title`
     - `summary`
     - `sections`
     - `cards`
   - Require `sections[*]` to contain:
     - `heading`
     - `body`
   - Require `cards[*]` to contain:
     - `persona_id`
     - `source_outcome`
     - `headline`
     - `body`
   - Permit `cards[*].source_outcome` values exactly as follows:
     - raw branch: `certified` or `rejected`
     - certified branch: `certified` only
   - Make the schema sufficient to derive `*_panel.md` and `*_cards.json` locally without another model call.
2. Create the render worker contract in `runtime/skills/wv-render-stage/SKILL.md`.
   - The skill accepts exactly one P10 render-input path and emits exactly one JSON object conforming to `schemas/panel_result_v1.json`.
   - Instruct the worker to use only the provided render-input artifact from `03_render/`.
   - Explicitly forbid:
     - reopening `02_answer`
     - reading `answer_batch.json`
     - rerunning certified-gate logic
     - reclassifying rejected versus certified entries
   - Preserve the input branch variant exactly:
     - raw input produces `variant == "raw"`
     - certified input produces `variant == "certified"`
   - Keep render execution to one P02 single-turn run per attempted branch with no retries.
3. Implement branch orchestration in `internal/stage/render/render.go`.
   - Define a stage request that includes at least:
     - `run_id`
     - `render_root`
     - `raw_render_input_path`
     - `certified_render_input_path`
     - P02 launch/runtime options
     - per-branch timeout
   - Read and validate only the P10 input artifacts from `03_render/`.
   - Raw branch behavior is exact:
     - if `raw_render_input.json` cannot be opened or parsed, mark raw as `failed`
     - if `raw_render_input.json.available == false`, mark raw as `skipped` with `skip_reason == "input_unavailable"` and write no raw branch outputs
     - if `raw_render_input.json.available == true`, run one render worker turn via P02, validate the returned JSON against `panel_result_v1`, then write:
       - `runs/<run_id>/03_render/raw_panel.json`
       - `runs/<run_id>/03_render/raw_panel.md`
       - `runs/<run_id>/03_render/raw_cards.json`
   - Certified branch behavior is exact:
     - if `certified_render_input.json` cannot be opened or parsed, mark certified as `failed`
     - if `certified_render_input.json.available == false`, mark certified as `skipped` with `skip_reason == "upstream_gate_not_passed"` and write no certified branch outputs
     - if `certified_render_input.json.available == true`, run one render worker turn via P02, validate the returned JSON against `panel_result_v1`, then write:
       - `runs/<run_id>/03_render/certified_panel.json`
       - `runs/<run_id>/03_render/certified_panel.md`
       - `runs/<run_id>/03_render/certified_cards.json`
   - Raw execution must still be possible when the certified branch is skipped.
   - P11 must not open:
     - `runs/<run_id>/02_answer/answer_batch.json`
     - `runs/<run_id>/02_answer/personas/**`
     - `result.raw.txt`
4. Implement deterministic final-output writers in `internal/stage/render/render.go`.
   - `raw_panel.json` and `certified_panel.json` are the exact validated JSON objects returned by the render worker.
   - `raw_cards.json` and `certified_cards.json` are the exact `cards` arrays copied from the corresponding validated panel JSON, preserving array order.
   - `raw_panel.md` and `certified_panel.md` are rendered locally from the validated panel JSON with fixed formatting:
     - line 1 is `# {title}`
     - then one summary paragraph
     - then one `## {heading}` block per `sections[*]` in order
     - then a final `## Cards` block rendering `cards[*]` in order
     - the `## Cards` block shape is exact and repeated once per card in array order:
       ```markdown
       ## Cards

       ### {headline}
       - persona_id: {persona_id}
       - source_outcome: {source_outcome}

       {body}
       ```
     - `headline` renders only in the `### {headline}` line
     - `persona_id` renders only in the `- persona_id: {persona_id}` line immediately below the headline
     - `source_outcome` renders only in the `- source_outcome: {source_outcome}` line immediately below the `persona_id` line
     - `body` renders only after one blank line below `source_outcome`
     - consecutive cards are separated by exactly one blank line between the prior card body and the next card headline
   - Each branch writes outputs only after schema validation succeeds.
   - A failed branch must not leave newly written partial outputs for that branch.
   - A later branch failure must not delete outputs from an earlier successful branch.
5. Write final stage status to `runs/<run_id>/03_render/status.json`.
   - `status.json` must contain these top-level keys:
     - `schema_version`, exact literal `worldview_render_status_v1`
     - `stage`, exact literal `03_render`
     - `run_id`
     - `raw`
     - `certified`
     - `overall`
   - Each branch object must contain:
     - `state`, exact literal `rendered`, `skipped`, or `failed`
     - `input_path`
     - `input_available`
     - `skip_reason`
     - `panel_json_path`
     - `panel_md_path`
     - `cards_json_path`
     - `error`
   - `input_available` copies the parsed P10 `available` value when the input artifact was parsed successfully; otherwise it is `null`.
   - `skip_reason` is a closed set:
     - raw branch may use only `input_unavailable`
     - certified branch may use only `upstream_gate_not_passed`
   - Branch-object field values are state-literalized and deterministic:
     - when `state == "rendered"`:
       - `skip_reason == null`
       - `panel_json_path` is the exact branch output path:
         - raw: `runs/<run_id>/03_render/raw_panel.json`
         - certified: `runs/<run_id>/03_render/certified_panel.json`
       - `panel_md_path` is the exact branch output path:
         - raw: `runs/<run_id>/03_render/raw_panel.md`
         - certified: `runs/<run_id>/03_render/certified_panel.md`
       - `cards_json_path` is the exact branch output path:
         - raw: `runs/<run_id>/03_render/raw_cards.json`
         - certified: `runs/<run_id>/03_render/certified_cards.json`
       - `error == null`
     - when `state == "skipped"`:
       - `skip_reason` is branch-specific and exact:
         - raw: `input_unavailable`
         - certified: `upstream_gate_not_passed`
       - `panel_json_path == null`
       - `panel_md_path == null`
       - `cards_json_path == null`
       - `error == null`
     - when `state == "failed"`:
       - `skip_reason == null`
       - `panel_json_path == null`
       - `panel_md_path == null`
       - `cards_json_path == null`
       - `error` is the required non-null failure object for that branch, including for `write_output` failures after cleanup
   - `error` is `null` unless the branch state is `failed`; failed branches must include an object with:
     - `phase`, exact literal `input_load`, `render_turn`, `schema_validate`, or `write_output`
     - `message`
   - `overall` must contain:
     - `stage_outcome`, exact literal `completed` or `failed`
     - `rendered_variants`, an array of `raw` and/or `certified` in execution order
   - `overall.stage_outcome == "completed"` only when neither branch is `failed`; skipped branches are non-failing terminal states.
   - `status.json` must be written for every terminal P11 outcome, including:
     - both branches skipped
     - raw rendered and certified skipped
     - raw rendered and certified failed
     - raw failed and certified rendered
6. Lock the terminal handoff boundary in exported contracts and code comments.
   - P11 is the terminal runtime plan and only writes final `03_render` outputs plus `status.json`.
   - No P11 code may modify `runs/<run_id>/02_answer/**`.
   - No P11 code may modify either P10 render-input artifact.
   - No P11 code may recompute certified eligibility, omission logic, or any other Stage 2 or P10 decision.

# Acceptance Checks

- P11 reads only current-run `runs/<run_id>/03_render/raw_render_input.json` and `runs/<run_id>/03_render/certified_render_input.json` as render inputs.
- P11 never reads or writes anything under `runs/<run_id>/02_answer/`.
- Raw render runs exactly when `raw_render_input.json.available == true`; otherwise raw is skipped and only `status.json` records that branch outcome.
- Certified render runs exactly when `certified_render_input.json.available == true`; otherwise certified is skipped and no certified branch output files are written.
- Raw outputs can exist while all certified outputs are absent.
- Certified outputs exist only when P10 already marked the certified input `available == true`.
- Successful branches always write all three branch outputs.
- Skipped branches write none of their branch-specific outputs.
- `status.json` always exists and records the `rendered`, `skipped`, or `failed` outcome for both raw and certified branches plus the overall stage outcome.
- P11 never reopens `answer_batch.json`, rescans persona workspaces, recomputes displayability, or recomputes success-rate gate math.
- Rendered panel JSON validates against `schemas/panel_result_v1.json`.
- `*_cards.json` remains a direct projection of `cards` from validated panel JSON, preserving order.
- `*_panel.md` is derived locally from the validated panel JSON, so render execution has one authoritative structured output per branch.
- Each `status.json` branch object uses one exact field pattern per `state`:
  - rendered branches set exact branch output paths, `skip_reason == null`, and `error == null`
  - skipped branches set the exact branch-specific `skip_reason`, all three branch output paths to `null`, and `error == null`
  - failed branches set all three branch output paths to `null`, `skip_reason == null`, and a non-null `error` object
- The final `## Cards` block in each `*_panel.md` uses the exact per-card markdown shape:
  - `### {headline}`
  - `- persona_id: {persona_id}`
  - `- source_outcome: {source_outcome}`
  - blank line
  - `{body}`
  - one blank line between adjacent cards

# Handoff

- P11 is the terminal runtime stage.
- Downstream operators or tools should read only:
  - `runs/<run_id>/03_render/raw_panel.json`
  - `runs/<run_id>/03_render/raw_panel.md`
  - `runs/<run_id>/03_render/raw_cards.json`
  - `runs/<run_id>/03_render/certified_panel.json`
  - `runs/<run_id>/03_render/certified_panel.md`
  - `runs/<run_id>/03_render/certified_cards.json`
  - `runs/<run_id>/03_render/status.json`
- No downstream consumer should need to reopen Stage 2 artifacts after P11 completes.
