---
plan_id: P08
title: Answer Single Worker Execution
status: proposed
depends_on:
  - P01
  - P02
  - P07
consumes:
  - docs/plans/tech-plan-runtime/01-codex-runtime-contract.md
  - docs/plans/tech-plan-runtime/02-app-server-client-lifecycle.md
  - docs/plans/tech-plan-runtime/07-answer-workspace-seal.md
produces:
  - runtime/skills/wv-answer-stage/SKILL.md
  - internal/stage/answer/runner.go
  - schemas/worldview_worker_result_v1.json
  - runs/<run_id>/02_answer/personas/<persona_id>/result.raw.txt
  - runs/<run_id>/02_answer/personas/<persona_id>/result.json
  - runs/<run_id>/02_answer/personas/<persona_id>/attestation.json
  - runs/<run_id>/02_answer/personas/<persona_id>/status.json
completion_evidence:
  - wv_answer_stage_skill_owned_by_p08
  - single_sealed_worker_input_boundary_enforced
  - authoritative_completed_agent_message_rule_fixed
  - worker_result_status_and_attestation_contract_defined
---

# Goal

Execute exactly one sealed answer worker for exactly one persona workspace and persist the per-worker artifacts that later plans can aggregate without guessing.

# Scope

- P08 owns the answer-stage skill file itself:
  - `runtime/skills/wv-answer-stage/SKILL.md`
  - this skill belongs here, not in P07 or any earlier plan
  - P07 may copy the selected skill bytes into the isolated workspace, but P08 owns the skill wording and the execution semantics that depend on it
  - the retained runtime plan set must keep `runtime/skills/wv-answer-stage/SKILL.md` ownership here even as later plans add smoke or operator tooling
- P08 owns only one sealed worker execution in `internal/stage/answer/runner.go`:
  - consume one sealed persona workspace prepared by P07
  - launch one app-server client through P02
  - run exactly one turn
  - persist only the single-worker execution artifacts
- P08 consumes only the already sealed Stage 2 worker inputs:
  - `<isolated_root>/workspace/AGENTS.md`
  - `<isolated_root>/input/prompt.txt`
  - `<run_root>/02_answer/personas/<persona_id>/outgoing_input.json`
  - `<isolated_root>/skill/SKILL.md`
- `outgoing_input.json` is the canonical execution record for what was sealed and sent:
  - P08 must read it before launch
  - P08 must verify the copied worker inputs against it
  - P08 must not treat any other file as the authoritative execution record
- `dispatch_input_v1.json` is never a P08 runtime input:
  - P08 must not read it directly
  - P08 must not reopen Stage 1 prepare artifacts to reconstruct worker input
- P08 must enforce the authoritative output rule:
  - `item/completed.agentMessage` is the only authoritative final model output
  - intermediate deltas, partial text, tool-call events, acknowledgements, and all other stream items are diagnostic only
  - no other terminal-like stream item may be normalized into a successful answer artifact
- Artifact ownership is fixed for one worker invocation under `<run_root>/02_answer/personas/<persona_id>/`:
  - `result.raw.txt`: diagnostic plain-text output only; present only when the current invocation reaches worker launch
  - `result.json`: canonical structured worker artifact on current-invocation success only; absent for interrupted, failed, and rejected-before-launch invocations
  - `attestation.json`: validation/certification-relevant facts for later aggregation; written for every invocation, including pre-launch verification rejection
  - `status.json`: explicit execution outcome and artifact-presence summary, written last for every invocation, including pre-launch verification rejection

# Out Of Scope

- Persona fan-out, worker pooling, queueing, or batch scheduling
- Fleet-level timeout handling or retry policy across multiple workers
- Forbidden-tool policy decisions or batch-level forbidden-tool aggregation
- Certified/failed/rejected batch modeling
- Render aggregation or render output behavior
- Any mutation of P07 sealing artifacts or workspace layout
- Any direct read of `dispatch_input_v1.json`

# Required Inputs

- `docs/plans/tech-plan-runtime/01-codex-runtime-contract.md`
- `docs/plans/tech-plan-runtime/02-app-server-client-lifecycle.md`
- `docs/plans/tech-plan-runtime/07-answer-workspace-seal.md`

# Implementation Tasks

1. Create the answer-stage skill owned by P08 at `runtime/skills/wv-answer-stage/SKILL.md`.
   - keep the skill narrowly scoped to one answer-stage worker
   - make the skill assume:
     - `workspace/AGENTS.md` is the local instruction anchor visible from the worker cwd
     - `input/prompt.txt` is the sealed worker-side prompt snapshot
     - no batch or render context exists
   - require the skill to aim at one final answer for one persona only
   - do not place certification, batch aggregation, or forbidden-tool policy ownership into the skill text
2. Implement a single-worker answer execution entrypoint in `internal/stage/answer/runner.go`.
   - accept one already sealed P07 execution record at minimum:
     - persona identifier
     - run-root persona artifact directory
     - isolated `cwd`
     - isolated `HOME`
     - isolated `workspace/AGENTS.md` path
     - isolated `input/prompt.txt` path
     - isolated `skill/SKILL.md` path
     - run-tree `outgoing_input.json` path
   - do not rescan persona directories to discover work
   - do not accept or process arrays of persona work items
3. Validate the sealed worker inputs before launch using only the P07 snapshot plus `outgoing_input.json`.
   - read exactly these files for execution preparation:
     - isolated `workspace/AGENTS.md`
     - isolated `input/prompt.txt`
     - isolated `skill/SKILL.md`
     - run-tree `outgoing_input.json`
   - require `outgoing_input.json` to remain the canonical record by enforcing:
     - `schema_version == "answer_outgoing_input_v1"`
     - `stage == "02_answer"`
     - `agent_instructions_path == "workspace/AGENTS.md"`
     - `prompt_path == "input/prompt.txt"`
   - verify the actual isolated file bytes against `outgoing_input.json`:
     - SHA-256 of isolated `workspace/AGENTS.md` must equal `agent_instructions_sha256`
     - SHA-256 of isolated `input/prompt.txt` must equal `prompt_sha256`
     - SHA-256 of isolated `skill/SKILL.md` must equal `skill_sha256`
     - recomputed length-prefixed combined hash over isolated `AGENTS.md`, prompt snapshot, and skill bytes must equal `combined_input_sha256`
   - compute and retain a SHA-256 of the exact `outgoing_input.json` bytes for later attestation
   - reject execution if any verification fails
   - if verification fails before worker launch:
     - treat the invocation outcome as `failed`
     - set `last_reached_phase` to exact literal `prelaunch_verification`
     - set `close_outcome_kind` to exact literal `rejected_before_launch`
     - do not launch the worker
     - do not write `result.raw.txt`
     - do not write `result.json`
     - still write `attestation.json`
     - still write `status.json` last
   - do not read `dispatch_input_v1.json`, `prepare_gate_status_v1.json`, or any Stage 1 persona files during this step
4. Construct the one worker turn input and execute it through P02 without redefining lifecycle ownership.
   - start the app-server only through `StartAppServer` from P02
   - run the turn only through `RunSingleTurn` from P02
   - use the P07-provided isolated `cwd` and `HOME` exactly; do not substitute repo-root or user-home values
   - build the one turn input from the sealed answer-stage skill plus the sealed prompt snapshot in one deterministic order:
     - skill text bytes first
     - exact separator bytes UTF-8 `0x0A 0x0A` (`\n\n`)
     - prompt snapshot text bytes second
   - rely on isolated `workspace/AGENTS.md` as the worker-visible instruction file by cwd placement; do not inline it into a second prompt copy
   - never retry launch, initialize, thread creation, turn start, or interrupt
   - never launch more than one worker from this entrypoint
5. Normalize the returned stream so only one output source is authoritative.
   - preserve the full in-memory diagnostic transcript returned by P02 long enough to derive artifacts
   - treat only `item/completed.agentMessage` as authoritative final model output
   - extract the final answer text for canonical success artifacts only from that completed item
   - treat all non-authoritative stream items as diagnostic facts only, including:
     - partial text deltas
     - reasoning or progress events
     - tool-call requests/results
     - interrupt acknowledgements
     - any other item that is not `item/completed.agentMessage`
   - if P02 returns `Completed` but no authoritative `item/completed.agentMessage` payload is available, downgrade the worker result to failure for artifact-writing purposes and write no success `result.json`
6. Persist `result.raw.txt` as a diagnostic artifact only.
   - write `result.raw.txt` for every invocation that reaches worker launch
   - do not write `result.raw.txt` when execution is rejected before worker launch
   - classify `result.raw.txt` with `result_raw_mode` as exactly one of:
     - `authoritative_text`
     - `diagnostic_text`
     - `empty`
   - if an authoritative `item/completed.agentMessage` exists, write the extracted final text from that item to `result.raw.txt`
   - otherwise write best-effort plain text reconstructed from diagnostic assistant-text stream items in receive order, or an empty file if no text was observed
   - use `result_raw_mode == "authoritative_text"` when the extracted completed-item text is written
   - use `result_raw_mode == "diagnostic_text"` when non-authoritative assistant text is written
   - use `result_raw_mode == "empty"` when `result.raw.txt` is written as an empty file
   - do not treat `result.raw.txt` as canonical model output in any later logic
7. Persist `result.json` as the canonical structured worker artifact on success only.
   - create `schemas/worldview_worker_result_v1.json` for the success artifact contract
   - delete any pre-existing `result.json` before persisting the current invocation outcome; only the current successful invocation may recreate it
   - write `result.json` only when an authoritative `item/completed.agentMessage` exists and the worker outcome is successful
   - if the current invocation ends `failed`, `interrupted`, or is rejected before launch, leave `result.json` absent
   - `result.json` must contain at least these top-level keys:
     - `schema_version`, exact literal `worldview_worker_result_v1`
     - `stage`, exact literal `02_answer`
     - `persona_id`
     - `source_outgoing_input_path`, exact literal `outgoing_input.json`
     - `source_outgoing_input_sha256`
     - `combined_input_sha256`
     - `authoritative_item_type`, exact literal `item/completed.agentMessage`
     - `authoritative_item_id`
     - `text`
     - `text_sha256`
     - `authoritative_item`
   - `authoritative_item` must preserve the exact completed-item payload returned by P02, not a lossy text-only rewrite
8. Persist `attestation.json` as the per-worker fact record for later aggregation.
   - always write `attestation.json` for every P08 invocation, including rejection before worker launch
   - `attestation.json` must contain at least these top-level keys:
     - `schema_version`, exact literal `worldview_worker_attestation_v1`
     - `stage`, exact literal `02_answer`
     - `persona_id`
     - `source_outgoing_input_path`, exact literal `outgoing_input.json`
     - `source_outgoing_input_sha256`
     - `combined_input_sha256`
     - `agent_instructions_sha256`
     - `prompt_sha256`
     - `skill_path`
     - `skill_sha256`
     - `runner_terminal_outcome`
     - `last_reached_phase`
     - `close_outcome_kind`
     - `authoritative_output_present`
     - `authoritative_item_type`
     - `authoritative_item_id`
     - `authoritative_text_sha256`
     - `result_json_sha256`
     - `result_raw_txt_sha256`
     - `result_raw_mode`
     - `observed_tool_calls`
     - `diagnostic_stream_item_count`
   - null and sentinel values are part of the contract:
     - when `authoritative_output_present == true`, `authoritative_item_type` must be exact literal `item/completed.agentMessage`
     - when `authoritative_output_present == false`, `authoritative_item_type`, `authoritative_item_id`, and `authoritative_text_sha256` must be `null`
     - when `result.json` is absent, `result_json_sha256` must be `null`
     - when `result.raw.txt` is absent, `result_raw_txt_sha256` and `result_raw_mode` must be `null`
     - when `result.raw.txt` is present, `result_raw_mode` must be one of `authoritative_text`, `diagnostic_text`, or `empty`
   - `observed_tool_calls` must remain diagnostic facts only:
     - record tool name and stable call identifier when present
     - do not decide allowed versus forbidden here
     - do not emit batch-level verdicts here
9. Persist `status.json` as the authoritative per-worker execution summary and completion record.
   - always write `status.json` last for every P08 invocation, including rejection before worker launch, after all other current-invocation artifacts are finalized or intentionally omitted
   - `status.json` must contain at least these top-level keys:
     - `schema_version`, exact literal `worldview_worker_status_v1`
     - `stage`, exact literal `02_answer`
     - `persona_id`
     - `outcome`, one of `completed`, `interrupted`, `failed`
     - `last_reached_phase`
     - `close_outcome_kind`
     - `authoritative_output_present`
     - `authoritative_item_type`
     - `authoritative_item_id`
     - `result_raw_present`
     - `result_json_present`
     - `attestation_present`
     - `source_outgoing_input_path`, exact literal `outgoing_input.json`
     - `result_raw_path`, exact literal `result.raw.txt` or `null`
     - `result_json_path`, exact literal `result.json` or `null`
     - `attestation_path`, exact literal `attestation.json`
   - `status.json` is the quick-scan worker outcome record for P09:
     - if verification fails before worker launch, `outcome == "failed"` and `close_outcome_kind == "rejected_before_launch"`
     - `result_json_present == true` only when `outcome == "completed"` and an authoritative output exists
     - `result_raw_present == true` only when the current invocation reached worker launch
     - `attestation_present` must be `true` whenever `status.json` exists
     - if `authoritative_output_present == false`, `authoritative_item_type` and `authoritative_item_id` must be `null`
10. Keep P08 narrow to one sealed worker and hand off batch semantics to P09.
   - do not add batch loops, worker fan-out APIs, or shared fleet coordination structures here
   - do not define certified, rejected, or fleet-failed worker classes here
   - do not aggregate forbidden-tool detections across workers here
   - do not let later render concerns change the single-worker artifact contract owned by this plan

# Acceptance Checks

- `runtime/skills/wv-answer-stage/SKILL.md` is created here and explicitly owned by P08 rather than P07 or any earlier plan.
- P08 consumes only one sealed worker snapshot:
  - isolated `workspace/AGENTS.md`
  - isolated `input/prompt.txt`
  - isolated `skill/SKILL.md`
  - run-tree `outgoing_input.json`
- P08 never opens `dispatch_input_v1.json`.
- Before worker launch, P08 verifies:
  - isolated `AGENTS.md` SHA-256 matches `outgoing_input.json.agent_instructions_sha256`
  - isolated prompt SHA-256 matches `outgoing_input.json.prompt_sha256`
  - isolated skill SHA-256 matches `outgoing_input.json.skill_sha256`
  - recomputed combined hash matches `outgoing_input.json.combined_input_sha256`
- P08 uses the P07-provided isolated `cwd` and `HOME` directly and does not launch from the repo tree or current user home.
- P08 delegates lifecycle execution to P02 and does not re-implement transport ordering, timeout hierarchy, or close/kill ownership.
- Exactly one worker turn is executed per P08 call; no persona iteration or worker pool is introduced.
- The worker turn input bytes are exactly: sealed skill bytes, then UTF-8 separator bytes `0x0A 0x0A` (`\n\n`), then sealed prompt snapshot bytes.
- `item/completed.agentMessage` is the only success-bearing final output source.
- Intermediate deltas and all non-authoritative stream items remain diagnostic only and do not become success artifacts.
- If pre-launch seal verification fails, P08 writes `attestation.json` and then `status.json`, sets `status.json.outcome == "failed"` with `close_outcome_kind == "rejected_before_launch"`, and leaves both `result.raw.txt` and `result.json` absent for that invocation.
- `result.raw.txt` is written for every launched execution and is explicitly diagnostic only.
- `result.json` is written only on success with an authoritative completed agent message and matches `schemas/worldview_worker_result_v1.json`.
- Before the current invocation outcome is persisted, any pre-existing `result.json` is deleted; interrupted, failed, and rejected-before-launch invocations leave `result.json` absent.
- `result.json` preserves the exact completed-item payload under `authoritative_item`.
- `attestation.json` is always written for every P08 invocation and records:
  - the executed seal-chain hashes
  - the execution outcome facts
  - the authoritative-output presence facts
  - observed tool calls as diagnostic data only
- Failure-path sentinel values are fixed:
  - no authoritative output => `authoritative_item_type == null`, `authoritative_item_id == null`, and `authoritative_text_sha256 == null`
  - no `result.json` => `result_json_sha256 == null`
  - no `result.raw.txt` => `result_raw_txt_sha256 == null` and `result_raw_mode == null`
- `status.json` is always written last and makes the execution outcome explicit without requiring P09 to infer from missing files.
- If execution ends `interrupted` or `failed`, `status.json.result_json_present == false` and no stale success payload is left in `result.json`.
- P08 does not define fleet-level timeout policy, forbidden-tool verdict policy, or batch certified/failed/rejected modeling.

# Handoff

- P09 consumes the P08 per-worker outputs without redefining them:
  - `status.json` for explicit worker outcome and artifact presence
  - `attestation.json` for certification-relevant facts and diagnostic tool-use observations
  - `result.json` only when `status.json.result_json_present == true`
  - `result.raw.txt` for diagnostics only, never as authoritative answer input
- P09 may aggregate timeouts, tool-policy violations, and batch outcome classes later, but those concerns must not be pulled back into P08.
