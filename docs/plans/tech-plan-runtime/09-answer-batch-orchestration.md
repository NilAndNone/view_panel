---
plan_id: P09
title: Answer Batch Orchestration
status: ready
depends_on:
  - P08
consumes:
  - docs/plans/tech-plan-runtime/08-answer-single-worker-execution.md
produces:
  - internal/stage/answer/batch.go
  - internal/orchestrator/run.go
  - schemas/worldview_answer_batch_v1.json
  - runs/<run_id>/02_answer/answer_batch.json
completion_evidence:
  - stage2_batch_outcome_contract_owned_by_p09
  - batch_timeout_and_concurrency_policy_kept_out_of_p08
  - forbidden_tool_rejection_owned_by_batch_layer
  - certified_subset_handoff_persisted_for_p10
---

# Goal

Execute the full Stage 2 answer batch over all sealed personas, keep all multi-worker policy at the batch layer, and persist one current-run batch artifact that tells later stages exactly which persona outputs are `certified`, `failed`, or `rejected`, with `rejected` preserved as the deterministic Stage 3 raw-path candidate class.

# Scope

- P09 owns Stage 2 batch orchestration in `internal/stage/answer/batch.go` and the Stage 2 integration point in `internal/orchestrator/run.go`.
- P09 owns all multi-persona runtime policy that P08 intentionally does not define:
  - persona fan-out
  - bounded concurrency
  - per-persona timeout and cancellation handling
  - Stage 2 retry policy
  - forbidden-tool detection and rejection policy
  - mutually exclusive per-persona outcome classification as `certified`, `failed`, or `rejected`
  - one durable current-run Stage 2 batch artifact for downstream consumption
- P09 may classify each persona from the direct in-memory P08 return envelope, the persisted P08 artifacts, or both, but the durable contract for later stages is `runs/<run_id>/02_answer/answer_batch.json`.
- P09 must not move any batch outcome ownership back into P08:
  - P08 remains a single-worker runner only
  - P08 `close_outcome_kind` values remain worker-execution facts, not batch verdicts
  - in particular, P08 `close_outcome_kind == "rejected_before_launch"` maps to P09 persona outcome `failed`, not P09 persona outcome `rejected`
- P10 must consume Stage 2 handoff only from the P09 batch artifact instead of reopening persona directories to re-derive Stage 2 verdicts:
  - `answer_batch.json.certified` is the complete certified-path source set
  - `answer_batch.json.rejected` is the complete rejected raw-path source set
  - P10 must decide for each persisted rejected entry whether it is displayable enough to place in `raw_render_input.json.entries`; every rejected entry not placed there must be recorded in `raw_render_input.json.omitted`

# Out Of Scope

- Any change to the single-worker artifact contract owned by P08
- Stage 1 prepare assembly, hard gate, or soft review behavior
- Render aggregation, render output generation, or any P10/P11 redesign
- Persona/content quality policy beyond forbidden-tool rejection
- Any use of `result.raw.txt` as authoritative answer input
- Moving concurrency, timeout, retry, or policy verdict logic into P08

# Required Inputs

- `docs/plans/tech-plan-runtime/08-answer-single-worker-execution.md`

# Implementation Tasks

1. Create the Stage 2 batch orchestration types in `internal/stage/answer/batch.go`.
   - define a batch request that accepts only already sealed Stage 2 worker items plus Stage 2 batch policy:
     - run identifier
     - Stage 2 answer root
     - ordered persona work list
     - `max_concurrency`
     - `worker_timeout_ms`
     - `max_attempts_per_persona`
     - `forbidden_tool_names`
   - the initial retry policy is owned here and is explicit:
     - implement `max_attempts_per_persona` in the batch contract
     - set the initial executable policy to exactly `1`
     - do not add retries inside P08
   - define a batch result type that returns both:
     - the in-memory per-persona classified outcomes for immediate current-run handoff
     - the persisted `answer_batch.json` path for durable downstream use
   - keep `internal/orchestrator/run.go` thin:
     - it calls one batch entrypoint
     - it does not own worker pooling, classification, or forbidden-tool policy
2. Implement bounded Stage 2 fan-out in `internal/stage/answer/batch.go`.
   - launch at most `max_concurrency` P08 workers at a time
   - do not create unbounded goroutines
   - dispatch exactly one P08 attempt at a time per persona
   - preserve the original Stage 2 persona order as the canonical output order:
     - do not emit `answer_batch.json` arrays in goroutine completion order
     - sort final `certified`, `failed`, and `rejected` entries by the original dispatch order
   - ensure each persona reaches exactly one terminal batch outcome
3. Keep all timeout and cancellation ownership in P09.
   - wrap each P08 execution in a per-persona context bounded by `worker_timeout_ms`
   - if a persona exceeds its worker timeout:
     - interrupt/cancel that worker through context
     - classify the persona as `failed`
     - set `failure_reason` to exact literal `worker_timeout`
   - if the parent Stage 2 context or run-level deadline ends before a persona finishes:
     - classify in-flight personas as `failed` with `failure_reason == "batch_timeout"` when the Stage 2 deadline expired
     - classify in-flight personas as `failed` with `failure_reason == "batch_cancelled_in_flight"` when parent cancellation arrives after dispatch for a non-timeout reason
     - classify not-yet-started personas as `failed` with `failure_reason == "batch_cancelled_before_start"` when cancellation arrives before dispatch
   - if P08 returns a runner error without a completed worker result, classify `failed` with one of:
     - `launch_error` when dispatch occurred but no trustworthy worker artifact establishes a more specific terminal worker outcome
     - `worker_failed` when trustworthy `status.json` exists and `status.json.outcome == "failed"`
     - `worker_interrupted` when trustworthy `status.json` exists and `status.json.outcome == "interrupted"`
     - `missing_worker_artifact`
     - `invalid_worker_artifact`
   - P08 must remain free of batch retry, fleet timeout, or fan-out logic
4. Define the mutually exclusive per-persona outcome model in P09 and nowhere earlier.
   - every persona must appear in exactly one of these three classes:
     - `certified`
     - `failed`
     - `rejected`
   - classification order is fixed and must be implemented exactly in this order:
     1. `failed` when no structurally valid candidate answer exists
     2. `rejected` when a structurally valid candidate exists but batch policy disqualifies it
     3. `certified` when a structurally valid candidate exists and no batch-policy rejection applies
   - a structurally valid candidate answer exists only when all of the following are true for the current invocation:
     - P08 completed without a batch-layer transport error
     - `status.json` exists
     - `attestation.json` exists
     - `status.json.outcome == "completed"`
     - `status.json.authoritative_output_present == true`
     - `status.json.result_json_present == true`
     - `result.json` exists at the path named by `status.json`
   - if any required success fact above is missing or false, classify the persona as `failed`
   - the structural-failure taxonomy is deterministic and must use the first matching exact literal below:
     1. `missing_worker_artifact` when one or more required worker artifacts needed for structural validation are absent
     2. `invalid_worker_artifact` when a required worker artifact exists but cannot be parsed, fails schema validation, or is internally contradictory enough that P09 cannot trust it as a source of facts
     3. `worker_interrupted` when trustworthy `status.json` exists and `status.json.outcome == "interrupted"`
     4. `worker_failed` when trustworthy `status.json` exists and `status.json.outcome == "failed"`
     5. `authoritative_output_missing` when trustworthy `status.json` exists, `status.json.outcome == "completed"`, and `status.json.authoritative_output_present == false`
     6. `result_json_missing` when trustworthy `status.json` exists, `status.json.outcome == "completed"`, `status.json.authoritative_output_present == true`, and `status.json.result_json_present == false`
     7. `result_json_artifact_missing` when trustworthy `status.json` exists, all prior structural success facts are satisfied, and the `result.json` file named by `status.json` is absent at read time
   - P09 must not collapse artifact-present but structurally invalid worker outcomes into `invalid_worker_artifact` once trustworthy parsed artifacts establish which structural success condition failed
   - P08 pre-launch verification rejection is still a P09 failure:
     - `status.json.close_outcome_kind == "rejected_before_launch"` becomes P09 `failed`
     - it must never become P09 `rejected`
   - P09 `rejected` is reserved for batch-policy rejection of an otherwise structurally valid candidate
5. Make forbidden-tool handling concrete and fully owned by P09.
   - source observed tool calls from the direct P08 return envelope when available
   - if the in-memory envelope is unavailable or incomplete, fall back to `attestation.json.observed_tool_calls`
   - normalize both configured forbidden names and observed tool names using exactly:
     - `strings.TrimSpace`
     - then `strings.ToLower`
   - deduplicate the normalized forbidden-tool set before classification
   - an observed tool call counts as a forbidden-tool hit only when:
     - the persona already has a structurally valid candidate answer
     - the normalized observed tool name is non-empty
     - the normalized observed tool name exactly matches a normalized forbidden name
   - if one or more forbidden-tool hits are present, classify the persona as `rejected`
   - for rejected personas, set `rejection_reason` to exact literal `forbidden_tool`
   - record every forbidden-tool hit in the batch artifact with at least:
     - original `tool_name`
     - normalized `tool_name_normalized`
     - stable `call_id` when present, otherwise `null`
   - a failed persona stays `failed` even if diagnostic tool-use observations are present:
     - forbidden-tool policy must not upgrade or rewrite a failure into `rejected`
   - do not retry a persona after a forbidden-tool rejection in this plan
6. Persist a durable Stage 2 batch contract for downstream stages.
   - create `schemas/worldview_answer_batch_v1.json`
   - write `runs/<run_id>/02_answer/answer_batch.json` as the canonical P09 output
   - `answer_batch.json` must contain at least these top-level keys:
     - `schema_version`, exact literal `worldview_answer_batch_v1`
     - `stage`, exact literal `02_answer`
     - `run_id`
     - `max_concurrency`
     - `worker_timeout_ms`
     - `max_attempts_per_persona`
     - `forbidden_tool_names`
     - `counts`
     - `certified`
     - `failed`
     - `rejected`
   - `counts` must contain at least:
     - `total_personas`
     - `certified`
     - `failed`
     - `rejected`
   - path basis is fixed for every non-null `status_path`, `attestation_path`, and `result_json_path` value in `answer_batch.json`:
     - persist them as POSIX-style relative paths rooted at `runs/<run_id>/02_answer/`, which is also the parent directory of `answer_batch.json`
     - consumers must resolve them by joining the directory containing `answer_batch.json` with the stored relative path
     - persona worker artifacts therefore use literals such as `personas/<persona_id>/status.json`, `personas/<persona_id>/attestation.json`, and `personas/<persona_id>/result.json`
     - do not copy P08 persona-local path literals unchanged from `status.json` because values like `attestation.json` and `result.json` there are only relative to the persona directory, not to `answer_batch.json`
     - when a dispatched persona is classified as `missing_worker_artifact` or `invalid_worker_artifact`, keep the expected batch-level relative `status_path` and `attestation_path` instead of `null`
   - `result_json_sha256` basis is fixed for every non-null occurrence in `answer_batch.json`:
     - compute it as the SHA-256 of the exact bytes of the `result.json` file stored at `result_json_path`
     - do not canonicalize JSON, normalize whitespace, or re-serialize parsed objects before hashing
     - P10 must verify the same byte-level basis when it consumes the artifact
   - each `certified` entry must contain at least:
     - `persona_id`
     - `outcome`, exact literal `certified`
     - `attempt_count`
     - `status_path`
     - `attestation_path`
     - `result_json_path`
     - `result_json_sha256`
     - `authoritative_text_sha256`
   - each `failed` entry must contain at least:
     - `persona_id`
     - `outcome`, exact literal `failed`
     - `attempt_count`
     - `failure_reason`
     - `status_path`
     - `attestation_path`
     - `result_json_path`
     - `runner_terminal_outcome`
     - `last_reached_phase`
     - `close_outcome_kind`
   - each `rejected` entry must contain at least:
     - `persona_id`
     - `outcome`, exact literal `rejected`
     - `attempt_count`
     - `rejection_reason`, exact literal `forbidden_tool`
     - `status_path`
     - `attestation_path`
     - `result_json_path`
     - `result_json_sha256`
     - `authoritative_text_sha256`
     - `forbidden_tool_hits`
   - rejected-entry handoff semantics are fixed:
     - every `rejected` entry is a mandatory raw-path candidate for P10
     - P10 must either aggregate it into `raw_render_input.json.entries` or record it in `raw_render_input.json.omitted`
     - P10 must not opt the rejected class in or out by policy choice
   - nullability and no-invocation sentinels are part of the contract:
     - when no P08 invocation occurred because `failure_reason == "batch_cancelled_before_start"`:
       - `attempt_count` must be `0`
       - `status_path` must be `null`
       - `attestation_path` must be `null`
       - `result_json_path` must be `null`
       - `runner_terminal_outcome` must be exact literal `not_invoked`
       - `last_reached_phase` must be exact literal `batch_pre_dispatch`
       - `close_outcome_kind` must be exact literal `not_invoked`
     - once a persona has been dispatched into P08, `status_path` and `attestation_path` must be the expected batch-relative paths `personas/<persona_id>/status.json` and `personas/<persona_id>/attestation.json`, even when the worker later lands in `launch_error`, `missing_worker_artifact`, or `invalid_worker_artifact`
     - `status_path` and `attestation_path` may be `null` only when cancellation happens before a worker invocation begins
     - for dispatched failed entries, `runner_terminal_outcome`, `last_reached_phase`, and `close_outcome_kind` are carry-through fields copied only from trustworthy parsed worker artifacts and otherwise forced to `null`
     - trustworthy parsed worker artifacts are used with this precedence:
       1. parsed `attestation.json`: copy `runner_terminal_outcome`, `last_reached_phase`, and `close_outcome_kind`
       2. otherwise, parsed `status.json`: copy `last_reached_phase` and `close_outcome_kind`, and force `runner_terminal_outcome` to `null`
       3. otherwise: force all three fields to `null`
     - required post-dispatch failed-entry rules are exact:
       - `launch_error`: `status_path` and `attestation_path` use the expected batch-relative paths; `runner_terminal_outcome`, `last_reached_phase`, and `close_outcome_kind` must all be `null`
       - `missing_worker_artifact`: `status_path` and `attestation_path` use the expected batch-relative paths; copy carry-through fields only from whichever required artifact still exists and parses trustworthily under the precedence above; do not synthesize non-null values from file absence
       - `invalid_worker_artifact`: `status_path` and `attestation_path` use the expected batch-relative paths; `runner_terminal_outcome`, `last_reached_phase`, and `close_outcome_kind` must all be `null` because malformed or contradictory worker artifacts are not trustworthy sources
     - `result_json_path`, `result_json_sha256`, and `authoritative_text_sha256` must be `null` for every `failed` entry
     - `forbidden_tool_hits` must be a non-empty array for every `rejected` entry
   - `answer_batch.json` must be written after all persona outcomes are finalized for the current run, even when zero personas are certified
7. Wire the Stage 2 batch result into `internal/orchestrator/run.go` without re-deriving it later.
   - replace any ad hoc Stage 2 persona loop in `internal/orchestrator/run.go` with one call into the P09 batch entrypoint
   - pass Stage 2 policy inputs only once:
     - ordered persona work list
     - `max_concurrency`
     - `worker_timeout_ms`
     - `max_attempts_per_persona`
     - `forbidden_tool_names`
   - return the P09 in-memory batch result and persisted artifact path as the Stage 2 handoff
   - the Stage 2 handoff is fixed:
     - `certified` entries are the only certified-path candidates
     - `rejected` entries are raw-path candidates only
   - do not make P10 rescan all persona directories to decide who is certified or whether rejected entries participate in raw aggregation
   - do not make `internal/orchestrator/run.go` reinterpret P08 `status.json` or `attestation.json` outside the P09 batch layer

# Acceptance Checks

- P09 owns the only Stage 2 batch verdict model and defines exactly three mutually exclusive persona outcomes:
  - `certified`
  - `failed`
  - `rejected`
- P08 remains a single-worker contract only and does not gain:
  - batch loops
  - concurrency policy
  - timeout policy
  - retry policy
  - forbidden-tool verdict logic
- P09 handoff to P10 is explicit and deterministic:
  - `answer_batch.json.certified` remains the complete certified-path source set
  - `answer_batch.json.rejected` is the complete raw-path rejected source set
  - P10 owns only displayability checks and certified gate math, not rejected-entry participation policy
  - certified/failed/rejected batch classes
- P09 classification order is fixed:
  - first prove structural success
  - then apply forbidden-tool rejection
  - otherwise certify
- `status.json.close_outcome_kind == "rejected_before_launch"` is mapped to P09 `failed`, not P09 `rejected`.
- The initial Stage 2 retry policy is explicit and centralized in P09 with `max_attempts_per_persona == 1`.
- P09 owns all per-persona timeout handling and maps it to stable failure reasons without pushing timeout logic into P08:
  - already-started personas cancelled by a non-timeout parent context use `failure_reason == "batch_cancelled_in_flight"`
  - not-yet-started personas cancelled before dispatch keep `failure_reason == "batch_cancelled_before_start"`
- Forbidden-tool matching is concrete and deterministic:
  - normalized with `strings.TrimSpace` then `strings.ToLower`
  - exact match against the deduplicated normalized forbidden set
  - only applied after structural success is established
- A persona with forbidden-tool observations but no structurally valid candidate answer is still `failed`, never `rejected`.
- Artifact-present P08 outcomes map to exact disjoint `failure_reason` literals instead of being collapsed into an overlapping catch-all:
  - trustworthy `status.json.outcome == "interrupted"` => `worker_interrupted`
  - trustworthy `status.json.outcome == "failed"` => `worker_failed`
  - `status.json.authoritative_output_present == false` => `authoritative_output_missing`
  - `status.json.result_json_present == false` => `result_json_missing`
  - `status.json` claims success but the referenced `result.json` is absent => `result_json_artifact_missing`
- `schemas/worldview_answer_batch_v1.json` defines the durable P09 batch artifact contract.
- `runs/<run_id>/02_answer/answer_batch.json` is always written for a completed Stage 2 batch closeout, even when:
  - every persona failed
  - every persona was rejected
  - the certified set is empty
- Every non-null `answer_batch.json` artifact path is deterministic:
  - `status_path`, `attestation_path`, and `result_json_path` are POSIX-style relative paths resolved from the directory containing `answer_batch.json`
  - P10 must not guess a persona-directory base to open them
- `result_json_sha256` is frozen across P09 and P10 as SHA-256 over the exact on-disk bytes of the referenced `result.json`, with no canonicalization or reserialization.
- `answer_batch.json` contains `certified`, `failed`, and `rejected` arrays sorted in original dispatch order, not goroutine finish order.
- Every persona appears in exactly one `answer_batch.json` outcome array.
- Failed entries with `failure_reason == "batch_cancelled_before_start"` use explicit no-invocation sentinels:
  - `attempt_count == 0`
  - `runner_terminal_outcome == "not_invoked"`
  - `last_reached_phase == "batch_pre_dispatch"`
  - `close_outcome_kind == "not_invoked"`
  - `status_path == null`
  - `attestation_path == null`
- Failed entries for dispatched personas keep deterministic path and carry-through rules:
  - `status_path` and `attestation_path` are the expected batch-relative worker paths for `launch_error`, `missing_worker_artifact`, and `invalid_worker_artifact`
  - `runner_terminal_outcome`, `last_reached_phase`, and `close_outcome_kind` are copied only from trustworthy parsed artifacts
  - parsed `attestation.json` may populate all three carry-through fields
  - parsed `status.json` may populate only `last_reached_phase` and `close_outcome_kind`
  - `launch_error` and `invalid_worker_artifact` force all three carry-through fields to `null`
- `answer_batch.json.certified[*]` includes enough information for P10 to open the certified `result.json` files directly without recomputing Stage 2 certification logic.
- `answer_batch.json.rejected[*]` includes enough information for P10 to optionally open rejected `result.json` files for the raw render path without recomputing Stage 2 rejection logic.
- P09 does not introduce any render formatting or P10/P11 redesign into Stage 2 orchestration.

# Handoff

- P10 consumes P09 output from `runs/<run_id>/02_answer/answer_batch.json`.
- P10 should treat `answer_batch.json.certified` as the certified Stage 2 subset for the current run.
- For the certified render path, P10 may open only the `result.json` files referenced by certified entries, resolving `result_json_path` relative to the directory containing `answer_batch.json`, plus any explicitly referenced supporting artifacts it needs.
- For the raw render path, P10 may, but is not required to, open `result.json` files referenced by rejected entries using the same relative-path basis.
- P10 must not rebuild the Stage 2 `certified` versus `failed` versus `rejected` model by rescanning every persona `status.json` and `attestation.json`.
