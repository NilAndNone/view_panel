---
plan_id: P10
title: Render Input Aggregation
status: ready
depends_on:
  - P03
  - P09
consumes:
  - docs/plans/tech-plan-runtime/03-run-layout-and-artifact-writer.md
  - docs/plans/tech-plan-runtime/09-answer-batch-orchestration.md
produces:
  - internal/stage/render/aggregate.go
  - runs/<run_id>/03_render/raw_render_input.json
  - runs/<run_id>/03_render/certified_render_input.json
completion_evidence:
  - raw_and_certified_render_inputs_split
  - certified_gate_math_frozen_in_p10
  - p11_handoff_avoids_stage2_rederivation
---

# Goal

Aggregate the current-run Stage 2 answer batch into two explicit Stage 3 render-input artifacts, one `raw` and one `certified`, so downstream rendering consumes stable Stage 3 inputs instead of reopening Stage 2 state or recomputing Stage 2 verdicts. The raw path deterministically considers both `certified` and `rejected` Stage 2 candidates; the certified path remains driven only by `certified`.

# Scope

- P10 owns render-input aggregation in `internal/stage/render/aggregate.go`.
- P10 reads only current-run Stage 2 artifacts:
  - `runs/<run_id>/02_answer/answer_batch.json`
  - the `result.json` files named by `answer_batch.json.certified[*].result_json_path`
  - the `result.json` files named by `answer_batch.json.rejected[*].result_json_path` for raw-path candidate loading
- Every entry in `answer_batch.json.rejected` participates exactly once in raw aggregation:
  - it becomes a raw displayable entry when it passes P10 displayability checks
  - otherwise it becomes a raw omitted entry with explicit `omission_reason`
- Every Stage 2 path consumed by P10 must be resolved relative to the directory containing `answer_batch.json`; P10 must not rescan persona directories to discover additional files and must not read from any prior run.
- P10 writes two Stage 3 contracts under `runs/<run_id>/03_render/`:
  - `raw_render_input.json`
  - `certified_render_input.json`
- The raw/certified split is permanent:
  - raw input exists to preserve visibility into displayable-but-not-certified output
  - certified input exists to preserve the trusted branch only
  - no later plan may collapse these contracts back into a single render-input path
- Both render-input files must carry enough data for P11 to render without reopening Stage 2:
  - each displayable entry carries the parsed authoritative `result.json` payload under `result`
  - each displayable entry also carries provenance fields copied from P09 (`persona_id`, `source_outcome`, `result_json_path`, `result_json_sha256`, `authoritative_text_sha256`, plus rejection metadata when applicable)
- `result_json_sha256` is shared with P09 and means the SHA-256 of the exact bytes of the referenced `result.json` file as written on disk; P10 must not canonicalize or re-serialize JSON before hashing.
- P10 owns displayability and certified-path gate math only. P10 does not redefine P09 `certified` versus `failed` versus `rejected` outcomes.
- Rejected-entry processing affects only the raw path; certified-path `T`, `C`, and `min_success_rate` remain defined solely from `answer_batch.json.certified`.
- `result.raw.txt` is never an authoritative render source in P10.
- No backfilling is allowed:
  - do not invent missing personas
  - do not synthesize placeholder answers
  - do not fabricate `result` payloads when a referenced Stage 2 artifact cannot be trusted
- Displayability is defined exactly here:
  - a Stage 2 entry is `raw-displayable` only when all of the following are true:
    - the entry comes from `answer_batch.json.certified` or `answer_batch.json.rejected`
    - `result_json_path` is non-null
    - the referenced `result.json` resolves under the current-run `02_answer` root
    - the referenced `result.json` exists
    - the referenced `result.json` parses as a JSON object
    - the SHA-256 of the exact bytes of that `result.json` file matches the entry's `result_json_sha256`
  - a Stage 2 entry is `certified-displayable` only when it is `raw-displayable` and its source array is `answer_batch.json.certified`
  - entries from `answer_batch.json.failed` are never displayable in P10
  - every raw-source candidate from `answer_batch.json.certified` and `answer_batch.json.rejected` must appear exactly once in `raw_render_input.json`:
    - displayable candidates go to `entries`
    - non-displayable candidates go to `omitted`
- Ordering is fixed:
  - `certified_render_input.json.entries` preserves the order of `answer_batch.json.certified`
  - `certified_render_input.json.omitted` preserves the order of non-displayable `answer_batch.json.certified` candidates
  - `raw_render_input.json.entries` contains all raw-displayable certified entries first in P09 certified order, followed by any raw-displayable rejected entries in P09 rejected order
  - `raw_render_input.json.omitted` contains all non-displayable certified candidates first in P09 certified order, followed by all non-displayable rejected candidates in P09 rejected order
  - P10 must not invent a cross-class interleaving that P09 did not persist

# Out Of Scope

- Recomputing or rewriting the Stage 2 batch verdict model owned by P09
- Any change to `runs/<run_id>/02_answer/answer_batch.json` or P09 ownership of that artifact
- Any use of `result.raw.txt` as authoritative input
- Render presentation, panel formatting, or final output generation owned by P11
- Cross-run recovery, backfill, or placeholder answer generation

# Required Inputs

- `docs/plans/tech-plan-runtime/03-run-layout-and-artifact-writer.md`
- `docs/plans/tech-plan-runtime/09-answer-batch-orchestration.md`
- current-run `runs/<run_id>/02_answer/answer_batch.json`

# Implementation Tasks

1. Create the render aggregation contract in `internal/stage/render/aggregate.go`.
   - Define an aggregate request that accepts only:
     - `run_id`
     - current-run `answer_batch.json` path
     - current-run `03_render` root or output paths
     - `min_success_rate`
   - Define an aggregate result that returns at least:
     - `raw_render_input_path`
     - `certified_render_input_path`
     - `raw_available`
     - `certified_available`
     - certified gate summary with `T`, `C`, `min_success_rate`, `observed_success_rate`, and `passed`
   - The aggregate entrypoint must reject any attempt to source render data from:
     - `result.raw.txt`
     - absolute Stage 2 paths outside the current run
     - any Stage 2 artifact not explicitly referenced by `answer_batch.json`
2. Implement candidate loading and displayability classification in `internal/stage/render/aggregate.go`.
   - Load `answer_batch.json` once as the only Stage 2 batch source of truth.
   - Build raw-source candidates from:
     - `answer_batch.json.certified`
     - `answer_batch.json.rejected`
   - Build certified-source candidates from:
     - `answer_batch.json.certified`
   - Do not inspect `answer_batch.json.failed` for renderable content.
   - For every candidate entry, resolve `result_json_path` relative to the directory containing `answer_batch.json`.
   - A candidate becomes displayable only when the resolved `result.json`:
     - stays under the current-run `02_answer` root
     - exists
     - parses as a JSON object
     - hashes, using the exact on-disk `result.json` bytes, to the exact `result_json_sha256` persisted by P09
   - Copy the parsed `result.json` object into the render entry as `result` without adding, removing, or inventing result fields.
   - When a candidate cannot be promoted into a displayable entry, record an omission with exact `omission_reason` from this closed set:
     - `result_json_path_missing`
     - `result_json_path_invalid`
     - `result_json_artifact_missing`
     - `result_json_invalid`
     - `result_json_sha256_mismatch`
   - For the raw artifact, every raw-source candidate must land in exactly one of `entries` or `omitted`; never drop a rejected candidate silently.
   - Preserve source-candidate ordering while classifying omissions:
     - certified-source omissions follow `answer_batch.json.certified` order
     - rejected-source omissions follow `answer_batch.json.rejected` order
3. Persist `runs/<run_id>/03_render/raw_render_input.json` as the raw Stage 3 contract.
   - Write `raw_render_input.json` for every completed P10 aggregation pass, even when no displayable entries are present.
   - `raw_render_input.json` must contain at least these top-level keys:
     - `schema_version`, exact literal `worldview_render_input_v1`
     - `stage`, exact literal `03_render`
     - `variant`, exact literal `raw`
     - `run_id`
     - `available`
     - `counts`
     - `entries`
     - `omitted`
   - `available` is exact `len(entries) > 0`.
   - every raw-source candidate from `answer_batch.json.certified` and `answer_batch.json.rejected` must appear exactly once in either `entries` or `omitted`
   - `counts` must contain at least:
     - `displayable_total`
     - `certified_displayable`
     - `rejected_displayable`
     - `omitted_total`
   - Each `entries[*]` must contain at least:
     - `persona_id`
     - `source_outcome`, exact literal `certified` or `rejected`
     - `result_json_path`
     - `result_json_sha256`
     - `authoritative_text_sha256`
     - `result`
     - `rejection_reason`
     - `forbidden_tool_hits`
   - Raw certified entries must set:
     - `source_outcome == "certified"`
     - `rejection_reason == null`
     - `forbidden_tool_hits == []`
   - Raw rejected entries must carry through P09 rejection metadata unchanged:
     - `source_outcome == "rejected"`
     - `rejection_reason`
     - `forbidden_tool_hits`
   - Each `omitted[*]` entry must contain at least:
     - `persona_id`
     - `source_outcome`, exact literal `certified` or `rejected`
     - `result_json_path`
     - `omission_reason`
   - Raw `omitted` ordering is fixed:
     - certified-source omissions first in `answer_batch.json.certified` order
     - rejected-source omissions next in `answer_batch.json.rejected` order
4. Persist `runs/<run_id>/03_render/certified_render_input.json` as the certified Stage 3 contract.
   - Write `certified_render_input.json` for every completed P10 aggregation pass, even when the certified path is unavailable, so P11 and operators can inspect one stable gate artifact without touching Stage 2.
   - `certified_render_input.json` must contain at least these top-level keys:
     - `schema_version`, exact literal `worldview_render_input_v1`
     - `stage`, exact literal `03_render`
     - `variant`, exact literal `certified`
     - `run_id`
     - `available`
     - `min_success_rate`
     - `gate`
     - `entries`
     - `omitted`
   - `entries` must contain only certified-displayable entries, preserving the order of `answer_batch.json.certified`.
   - `entries[*]` uses the same shape as raw entries, except:
     - `source_outcome` is always exact literal `certified`
     - `rejection_reason == null`
     - `forbidden_tool_hits == []`
   - Certified gate math is exact and owned only by P10:
     - `T = len(answer_batch.json.certified)` for the current run
     - `C = len(certified_render_input.json.entries)`
     - when `T == 0`:
       - `gate.observed_success_rate = null`
       - `gate.passed = false`
       - `gate.gate_reason = "no_certified_candidates"`
     - when `T > 0`:
       - `gate.observed_success_rate = C / T`
     - when `T > 0` and `C == 0`:
       - `gate.passed = false`
       - `gate.gate_reason = "no_certified_displayable_entries"`
     - when `T > 0`, `C > 0`, and `C / T < min_success_rate`:
       - `gate.passed = false`
       - `gate.gate_reason = "below_min_success_rate"`
     - when `T > 0`, `C > 0`, and `C / T >= min_success_rate`:
       - `gate.passed = true`
       - `gate.gate_reason = null`
   - `available` is exact `gate.passed`.
   - `min_success_rate` applies only to this certified path and must never filter `raw_render_input.json`.
   - `omitted[*]` contains only omissions from certified-source candidates, preserves `answer_batch.json.certified` order, and uses the same omission contract as the raw artifact.
5. Lock the Stage 3 handoff boundary in code comments and the aggregate contract.
   - P10 must be the last stage that opens Stage 2 `result.json` artifacts for rendering purposes.
   - P11 must consume `raw_render_input.json` and `certified_render_input.json` only.
   - P11 must not:
     - reopen `answer_batch.json`
     - rescan `runs/<run_id>/02_answer/personas/...`
     - reapply forbidden-tool policy
     - recompute `T`, `C`, or `min_success_rate`

# Acceptance Checks

- `internal/stage/render/aggregate.go` reads only current-run Stage 2 artifacts:
  - `runs/<run_id>/02_answer/answer_batch.json`
  - the `result.json` files explicitly named by current-run certified entries
  - and the `result.json` files explicitly named by current-run rejected entries for raw-path candidate loading
- P10 never uses `result.raw.txt` as authoritative render input.
- Raw-displayable versus certified-displayable is fixed:
  - raw candidates come from P09 `certified` and P09 `rejected`
  - certified candidates come only from P09 `certified`
  - P09 `failed` entries never appear in either render-input artifact
- No persona or answer is invented:
  - missing or invalid Stage 2 references are omitted with explicit omission reasons
  - every raw candidate appears exactly once in `raw_render_input.json.entries` or `raw_render_input.json.omitted`
  - P10 never synthesizes placeholder `result` payloads
- `raw_render_input.json` is always written and is usable whenever displayable output exists:
  - `available == true` iff `counts.displayable_total > 0`
  - `entries[*].result` is copied from the authoritative Stage 2 `result.json` payload after successful parse + byte-hash verification
  - when rejected entries are included, raw entries preserve certified-then-rejected ordering as defined in this plan
  - raw omitted entries preserve the same certified-then-rejected source ordering and retain `source_outcome` for every omission
- `certified_render_input.json` is always written and contains only the certified subset:
  - every `entries[*].source_outcome == "certified"`
  - no rejected entry appears in `certified_render_input.json`
- Certified gate math is exact:
  - `T` is the total number of current-run entries in `answer_batch.json.certified`
  - `C` is the total number of certified-displayable entries successfully aggregated into `certified_render_input.json.entries`
  - `min_success_rate` is applied only to the certified path
  - `T == 0` forces `gate.passed == false` and `gate.gate_reason == "no_certified_candidates"`
  - `T > 0` uses `observed_success_rate = C / T`
  - `certified_render_input.json.available == true` only when `T > 0`, `C > 0`, and `C / T >= min_success_rate`
- P10 does not re-open or reclassify P09 outcomes:
  - P09 remains authoritative for `certified`, `failed`, and `rejected`
  - P10 only decides whether referenced authoritative `result.json` artifacts are displayable enough to aggregate
  - P10 does not choose whether rejected entries participate in the raw path
- P11 can consume P10 outputs without re-deriving Stage 2 logic:
  - it does not need to reopen `answer_batch.json`
  - it does not need to resolve Stage 2 persona directories
  - it does not need to compute success-rate gating on its own

# Handoff

- P11 consumes `runs/<run_id>/03_render/raw_render_input.json` and `runs/<run_id>/03_render/certified_render_input.json` as its only render inputs.
- P11 should use `certified_render_input.json` only when `available == true`.
- P11 may fall back to `raw_render_input.json` when `raw_render_input.json.available == true`.
- P10 remains the only stage that translates Stage 2 batch outcomes plus authoritative `result.json` artifacts into render-ready entry lists.
