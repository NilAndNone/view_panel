---
plan_id: P06
title: Prepare Soft Review
status: proposed
depends_on:
  - P04
consumes:
  - docs/plans/tech-plan-runtime/04-prepare-input-builder.md
produces:
  - internal/stage/prepare/review.go
  - runtime/skills/wv-prepare-stage/SKILL.md
  - schemas/prepare_review_v1.json
  - runs/<run_id>/01_prepare/prepare_review_v1.json
completion_evidence:
  - advisory_review_contract_defined
  - advisory_review_artifact_written
  - review_failure_recorded_without_blocking
---

# Goal

Add a prepare-stage advisory review branch that inspects immutable Stage 1 artifacts through an explicit runtime-to-skill review contract, emits a separate machine-readable warning artifact, and never owns Stage 2 blocking.

# Scope

- Add `internal/stage/prepare/review.go` as the non-blocking P06 review runner.
- The authoritative review source is the canonical P04 persona bundle under `<run_root>/01_prepare/personas/<persona_id>/`; P06 reads that bundle as-is and never constructs an alternate source of truth.
- The runtime-to-skill input contract is split explicitly into primary review subject and evidence-only context:
  - primary review subject:
    - `dispatch_input_v1.json` is the authoritative structured source for each persona and anchors persona identity, assumptions/constraints, supplementary-material assembly, and the expected Stage 2 worker contract
    - `agents.md` and `prompt.txt` are the primary rendered review subjects and are evaluated against `dispatch_input_v1.json` and across peer personas
  - evidence-only context:
    - `manifest.json` may be read only for bundle provenance, artifact membership, and relative-path evidence
    - `hashes.json` may be read only for immutable file-identity and integrity evidence
  - evidence-only files may support or cite a finding, but they are never standalone review subjects and never override the authority of `dispatch_input_v1.json`, `agents.md`, or `prompt.txt`
- Use only canonical P04 outputs under `<run_root>/01_prepare/personas/<persona_id>/` as read-only review inputs; do not rewrite or replace:
  - `dispatch_input_v1.json`
  - `agents.md`
  - `prompt.txt`
  - `manifest.json`
  - `hashes.json`
- Build `schemas/prepare_review_v1.json` and emit the canonical advisory artifact at `<run_root>/01_prepare/prepare_review_v1.json`.
- All runtime paths in this plan are `<run_root>`-relative (for example `01_prepare/...` means `<run_root>/01_prepare/...`).
- `review_mode` is advisory-only and non-blocking by definition:
  - P06 may emit warnings about weak persona differentiation, suspicious material assembly, and output-contract issues.
  - P06 never sets or derives `can_proceed_to_stage2`.
  - P06 never rewrites or replaces `prepare_gate_status_v1.json`.
  - P07 does not depend on P06 by default and must not consume `prepare_review_v1.json` as a required input.
- Integration order inside `prepare.go` is explicit:
  - P04 canonical artifact assembly completes first.
  - P06 runs next against those immutable P04 outputs and must attempt to write `01_prepare/prepare_review_v1.json` before the prepare stage returns.
  - P05 may later block Stage 2, but that later hard-gate outcome does not suppress or redefine the already-required P06 advisory artifact.
  - P05 remains the sole blocker and sole owner of `can_proceed_to_stage2`.
- The advisory artifact is top-level and machine-readable. It must use exactly these top-level keys:
  - `schema_version`
  - `stage`
  - `review_mode`
  - `status`
  - `summary`
  - `findings`
- `status` is deterministic and limited to these literals:
  - `completed_clean` when the review session completes and emits zero findings.
  - `completed_with_warnings` when the review session completes and emits one or more warning findings.
  - `review_unavailable` when the runtime wrapper must synthesize an advisory fallback because the review session cannot complete, times out, returns invalid output, or is otherwise rejected; this remains advisory and non-blocking and is never accepted as a pass-through skill response.
- `summary` is a closed object with exact keys:
  - `non_blocking`, which must be `true`
  - `stage2_dependency`, which must be `"none"`
  - `review_completed`, a boolean
  - `persona_count`, an integer >= 0; for completed review artifacts it must equal the number of prepared personas discovered for review input
  - `finding_count`, an integer >= 0
  - `category_counts`, a closed object with exact integer keys:
    - `weak_persona_differentiation`
    - `suspicious_material_assembly`
    - `output_contract_issue`
    - `review_execution`
- `findings` is an array of warning-only objects with exact keys:
  - `finding_id`
  - `severity`, with exact literal `warning`
  - `category`, with exact enum:
    - `weak_persona_differentiation`
    - `suspicious_material_assembly`
    - `output_contract_issue`
    - `review_execution`
  - `persona_id`, as string or null
  - `artifact_path`, as `<run_root>`-relative string or null
  - `message`, as string
  - `evidence`, as array of strings
  - `suggested_follow_up`, as string or null
  - `blocks_stage2`, which must be `false`
  - `review_execution` is reserved exclusively for the runtime-synthesized fallback artifact with `status = review_unavailable`; completed review artifacts (`completed_clean` and `completed_with_warnings`) must not emit `review_execution` findings and must keep `summary.category_counts.review_execution = 0`
- Success-path count invariants are explicit for emitted `prepare_review_v1.json` artifacts:
  - when `status` is `completed_clean` or `completed_with_warnings`, `summary.persona_count` must equal the number of prepared personas discovered for review input
  - when `status` is `completed_clean` or `completed_with_warnings`, `summary.finding_count` must equal the exact number of objects in `findings`
  - when `status` is `completed_clean` or `completed_with_warnings`, emitted findings may use only `weak_persona_differentiation`, `suspicious_material_assembly`, and `output_contract_issue`; `review_execution` is forbidden on this completed path
  - when `status` is `completed_clean` or `completed_with_warnings`, `summary.category_counts.weak_persona_differentiation`, `summary.category_counts.suspicious_material_assembly`, and `summary.category_counts.output_contract_issue` must each equal the exact number of emitted findings in that category
  - when `status` is `completed_clean` or `completed_with_warnings`, `summary.category_counts.review_execution` must be `0`
  - when `status` is `completed_clean` or `completed_with_warnings`, the sum of all four category counts must equal `summary.finding_count`
  - the clean success case is exact: `findings == []`, `status = completed_clean`, `summary.review_completed = true`, `summary.finding_count = 0`, and every `summary.category_counts.* = 0`
  - the warning success case is exact: `findings` contains one or more warning objects and `status = completed_with_warnings`
- Failure handling is explicit and still non-blocking:
  - the runtime wrapper is the only owner allowed to synthesize `status = review_unavailable`; it must reject any skill response that already uses that status and synthesize the fallback artifact itself instead of passing the skill response through
  - if the review runtime cannot start, cannot finish, returns invalid JSON, returns a schema-invalid response, or returns a schema-valid response with `status = review_unavailable`, P06 must still synthesize and write `01_prepare/prepare_review_v1.json`
  - that synthesized artifact must set `status = review_unavailable`, `summary.review_completed = false`, `summary.non_blocking = true`, and `summary.stage2_dependency = "none"`
  - when `status = review_unavailable`, `findings` may contain only `review_execution` warning objects, and `review_execution` is not permitted in any non-fallback artifact
  - when `status = review_unavailable`, `summary.finding_count` must equal `len(findings)` and be `>= 1`
  - when `status = review_unavailable`, `summary.category_counts.review_execution` must equal `summary.finding_count`
  - when `status = review_unavailable`, `summary.category_counts.weak_persona_differentiation = 0`, `summary.category_counts.suspicious_material_assembly = 0`, and `summary.category_counts.output_contract_issue = 0`
  - this failure path never blocks Stage 2 and never mutates canonical P04 artifacts

# Out Of Scope

- hard-gate ownership, `can_proceed_to_stage2`, or any rewrite of `01_prepare/prepare_gate_status_v1.json`
- any mutation, repair, normalization, or replacement of `dispatch_input_v1.json`, `agents.md`, `prompt.txt`, `manifest.json`, or `hashes.json`
- making P07 depend on P06 or requiring `prepare_review_v1.json` for Stage 2 sealing
- content-pack redesign, persona-pack redesign, or persona acceptance policy
- automatic fixups, auto-editing prompts, or score-based gating

# Required Inputs

- docs/plans/tech-plan-runtime/04-prepare-input-builder.md

# Implementation Tasks

1. Add `schemas/prepare_review_v1.json` as the exact structural contract for `<run_root>/01_prepare/prepare_review_v1.json`.
   - top-level `type: object`
   - top-level `required` is exactly `schema_version`, `stage`, `review_mode`, `status`, `summary`, and `findings`
   - top-level `additionalProperties: false`
   - enforce exact literals:
     - `schema_version == "prepare_review_v1"`
     - `stage == "01_prepare"`
     - `review_mode == "advisory"`
   - enforce `status` enum exactly:
     - `completed_clean`
     - `completed_with_warnings`
     - `review_unavailable`
   - the schema still admits `review_unavailable` for the canonical runtime-written artifact, but the runtime must not accept that status as a pass-through skill response
   - `summary` is a closed object with required keys `non_blocking`, `stage2_dependency`, `review_completed`, `persona_count`, `finding_count`, and `category_counts`
   - `summary.non_blocking` is exact boolean literal `true`
   - `summary.stage2_dependency` is exact string literal `"none"`
   - `summary.review_completed` is boolean
   - `summary.persona_count` and `summary.finding_count` are integers with minimum `0`
   - completed artifacts must set `summary.persona_count` to the number of prepared personas discovered for review input; the runtime-synthesized fallback uses that same discovered persona count
   - `summary.category_counts` is a closed object with required integer keys `weak_persona_differentiation`, `suspicious_material_assembly`, `output_contract_issue`, and `review_execution`, each with minimum `0`
   - `findings` is an array whose item schema is a closed object with required keys `finding_id`, `severity`, `category`, `persona_id`, `artifact_path`, `message`, `evidence`, `suggested_follow_up`, and `blocks_stage2`
   - `severity` is exact literal `warning`
   - `category` enum is exactly `weak_persona_differentiation`, `suspicious_material_assembly`, `output_contract_issue`, and `review_execution`
   - `persona_id` is `string` or `null`
   - `artifact_path` is `string` or `null`
   - `message` is `string`
   - `evidence` is an array of strings
   - `suggested_follow_up` is `string` or `null`
   - `blocks_stage2` is exact boolean literal `false`
2. Update `runtime/skills/wv-prepare-stage/SKILL.md` so the advisory reviewer consumes the immutable prepared persona bundle and emits only schema-valid warning data.
  - the skill input is the already-written prepare directory set under `<run_root>/01_prepare/personas/<persona_id>/`
  - `dispatch_input_v1.json` is the authoritative structured source for each persona in the review request
  - `agents.md` and `prompt.txt` are the primary rendered review subjects and are evaluated against `dispatch_input_v1.json` plus peer personas
  - `manifest.json` and `hashes.json` are evidence-only inputs; the skill may cite them only for provenance or integrity support and must not treat them as standalone review subjects
  - if `dispatch_input_v1.json`, `agents.md`, and `prompt.txt` disagree, the skill treats `dispatch_input_v1.json` as authoritative and reports an advisory mismatch instead of rewriting any file
  - the skill reviews only these warning categories:
    - weak persona differentiation across the prepared persona set
    - suspicious material assembly within rendered supplementary materials or assumptions/constraints
    - output-contract issues that could confuse or destabilize Stage 2 worker responses
   - the skill must not produce hard-fail language, must not set any gate boolean, and must not instruct the runtime to rewrite canonical Stage 1 files
   - when no warning is warranted, the skill returns `status: "completed_clean"` with `findings: []`
   - when warnings are warranted, the skill returns `status: "completed_with_warnings"` and warning findings only from `weak_persona_differentiation`, `suspicious_material_assembly`, or `output_contract_issue`; it must not emit `review_execution`
   - the skill must never return `status: "review_unavailable"` or any already-synthesized fallback artifact
   - if the skill itself cannot honor the contract, the runtime owns synthesis of the `review_unavailable` artifact; any schema-valid skill response with `status: "review_unavailable"` is rejected and replaced by a runtime-synthesized fallback, and the skill does not become a gate
3. Implement `internal/stage/prepare/review.go` as the P06 advisory runner.
  - enumerate authoritative personas from `<run_root>/01_prepare/personas/<persona_id>` child directories using the already-prepared directory set from P04
  - assemble a read-only review request from the canonical P04 artifacts for each persona using this fixed contract:
    - primary review subject: `dispatch_input_v1.json`, `agents.md`, `prompt.txt`
    - evidence-only context: `manifest.json`, `hashes.json`
  - treat `dispatch_input_v1.json` as the authoritative structured source in the review request; if it conflicts with `agents.md` or `prompt.txt`, preserve the canonical files and surface an advisory mismatch finding instead of redefining the contract
  - preserve lexicographic `persona_id` order in the review input so cross-persona differentiation analysis is deterministic
  - invoke the review session through `runtime/skills/wv-prepare-stage/SKILL.md`
  - validate any returned advisory JSON against `schemas/prepare_review_v1.json`
  - if the skill returns a schema-valid response with `status = "review_unavailable"`, reject it as an invalid pass-through skill response and synthesize the runtime fallback instead of writing it verbatim
   - for any schema-valid completed response (`completed_clean` or `completed_with_warnings`), enforce the success-path count invariants before writing the artifact:
     - `summary.persona_count` equals the number of prepared personas discovered for review input
     - `summary.finding_count` equals `len(findings)`
     - completed-response findings use only `weak_persona_differentiation`, `suspicious_material_assembly`, and `output_contract_issue`; reject any completed response that emits `review_execution`
     - `summary.category_counts.weak_persona_differentiation`, `summary.category_counts.suspicious_material_assembly`, and `summary.category_counts.output_contract_issue` each equal the number of emitted findings in that category
     - `summary.category_counts.review_execution` equals `0`
     - the sum of all `summary.category_counts` values equals `summary.finding_count`
     - if `findings` is empty, require `status = "completed_clean"`, `summary.review_completed = true`, and all category counts equal `0`
     - if `findings` is non-empty, require `status = "completed_with_warnings"`
   - on a schema-valid completed response, write the artifact to `<run_root>/01_prepare/prepare_review_v1.json`
  - on session-start failure, transport failure, timeout, empty response, schema-invalid response, skill-returned `review_unavailable`, or completed-response count mismatch, synthesize a fallback artifact instead of propagating a blocking error:
    - `schema_version = "prepare_review_v1"`
    - `stage = "01_prepare"`
    - `review_mode = "advisory"`
    - `status = "review_unavailable"`
    - `summary.non_blocking = true`
    - `summary.stage2_dependency = "none"`
    - `summary.review_completed = false`
    - `summary.persona_count` equals the number of prepared personas discovered for review input
    - `summary.finding_count` equals `len(findings)` and must be `>= 1`
    - `summary.category_counts.weak_persona_differentiation = 0`
    - `summary.category_counts.suspicious_material_assembly = 0`
    - `summary.category_counts.output_contract_issue = 0`
    - `summary.category_counts.review_execution` equals `summary.finding_count`
    - `findings` contains only `review_execution` warnings, each with `blocks_stage2 = false` and a machine-readable failure message
  - the only runtime write owned by P06 is `<run_root>/01_prepare/prepare_review_v1.json`
  - do not mutate, replace, or delete any existing Stage 1 artifact
4. Modify `internal/stage/prepare/prepare.go` to call the advisory review runner after P04 canonical artifact assembly completes.
  - call the advisory review runner immediately after P04 artifact assembly and before the prepare stage returns any later P05 hard-gate result
  - P06 does not wait for P05 admission and does not influence P05 evaluation or output
  - even if P05 later blocks Stage 2, `prepare_review_v1.json` is still a required P06 output and remains advisory evidence only
  - the soft review runs against already-written P04 outputs only
  - P04 success remains authoritative for Stage 1 assembly; P06 never changes P04 artifact contents
  - do not thread any P06 result into `prepare_gate_status_v1.json`
  - do not add any P06 dependency edge to P07
  - if the advisory review returns warnings or emits `review_unavailable`, still return the prepare stage as non-blocked and leave Stage 2 gating to P05 only

# Acceptance Checks

- `schemas/prepare_review_v1.json` validates the structural contract of `<run_root>/01_prepare/prepare_review_v1.json` with exact top-level keys `schema_version`, `stage`, `review_mode`, `status`, `summary`, and `findings`, and rejects additional top-level fields.
- `prepare_review_v1.json` always uses `schema_version == "prepare_review_v1"`, `stage == "01_prepare"`, and `review_mode == "advisory"`.
- `prepare_review_v1.json.status` is always one of:
  - `completed_clean`
  - `completed_with_warnings`
  - `review_unavailable`
- `summary.non_blocking` is always `true` and `summary.stage2_dependency` is always `"none"`.
- `findings` entries are warning-only and each `blocks_stage2` is always `false`.
- `findings.category` values are limited to:
  - `weak_persona_differentiation`
  - `suspicious_material_assembly`
  - `output_contract_issue`
  - `review_execution`
- The runtime-to-skill review contract is explicit: `dispatch_input_v1.json` is the authoritative structured source, `agents.md` and `prompt.txt` are the primary rendered review subjects, and `manifest.json` plus `hashes.json` are evidence-only context that may support findings but never become standalone review subjects.
- `review_execution` is exclusive to the runtime-synthesized `status == review_unavailable` fallback artifact.
- When the review session completes with no advisory concerns, `status == completed_clean`, `summary.review_completed == true`, `findings == []`, `summary.finding_count == 0`, and every `summary.category_counts.* == 0`.
- When the review session completes with no advisory concerns or with advisory concerns, `summary.persona_count` equals the number of prepared personas discovered for review input.
- When the review session completes with advisory concerns, `status == completed_with_warnings`, `summary.review_completed == true`, `summary.finding_count == len(findings) > 0`, every finding category is limited to `weak_persona_differentiation`, `suspicious_material_assembly`, or `output_contract_issue`, `summary.category_counts.review_execution == 0`, each non-`review_execution` `summary.category_counts.<category>` equals the number of emitted findings in that category, and the sum of category counts equals `summary.finding_count`.
- The skill contract accepts only completed advisory responses; if the skill returns a schema-valid artifact with `status == review_unavailable`, the runtime rejects that pass-through response and synthesizes the fallback artifact itself.
- When the review session cannot complete or returns schema-invalid output, P06 still writes `<run_root>/01_prepare/prepare_review_v1.json` with `status == review_unavailable`, `summary.review_completed == false`, `summary.finding_count == len(findings) >= 1`, `summary.category_counts.review_execution == summary.finding_count`, `summary.category_counts.weak_persona_differentiation == 0`, `summary.category_counts.suspicious_material_assembly == 0`, `summary.category_counts.output_contract_issue == 0`, and only `review_execution` warning findings that record the failure without blocking Stage 2.
- P06 reads P04 outputs as immutable inputs only and never rewrites or replaces:
  - `<run_root>/01_prepare/personas/<persona_id>/dispatch_input_v1.json`
  - `<run_root>/01_prepare/personas/<persona_id>/agents.md`
  - `<run_root>/01_prepare/personas/<persona_id>/prompt.txt`
  - `<run_root>/01_prepare/personas/<persona_id>/manifest.json`
  - `<run_root>/01_prepare/personas/<persona_id>/hashes.json`
  - `<run_root>/01_prepare/prepare_gate_status_v1.json`
- P06 writes only `<run_root>/01_prepare/prepare_review_v1.json`; it does not own or modify any canonical Stage 1 artifact or hard-gate status artifact.
- `prepare.go` runs P06 immediately after P04 artifact assembly; `prepare_review_v1.json` is still required even when P05 later blocks Stage 2, and P05 remains the sole blocker.
- P07 remains dependent on P05 only for Stage 2 admission and does not require P06 or `prepare_review_v1.json` by default.

# Handoff

- P06 provides optional advisory evidence at `<run_root>/01_prepare/prepare_review_v1.json`, including runs where P05 later blocks Stage 2.
- P05 remains the sole prepare-stage blocking owner.
- P07 may ignore P06 by default unless a later plan explicitly adds optional advisory consumption.
