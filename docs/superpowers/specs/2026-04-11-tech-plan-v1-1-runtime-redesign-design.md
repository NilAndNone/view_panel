# Tech Plan V1.1 Runtime Redesign Design

Date: 2026-04-11

Source inputs:

1. `docs/tech-plan-v1.md`
2. `docs/plans/tech-plan-v1/`
3. user review of the current executable plan set

## Goal

Define a new runtime-only plan set for `tech-plan-v1.1` that keeps the correct incident-driven architecture from v1, but restructures the execution plan around the real blockers to implementation. The redesign must preserve the single canonical input chain, harden Stage 2 input sealing, add a true product entrypoint plan, and separate hard runtime gates from advisory model review.

## Scope

This redesign covers the runtime plan set only.

It includes:

1. a new plan topology for runtime implementation
2. a new dependency graph
3. a corrected Stage 1 to Stage 2 input contract
4. a hard gate vs soft review split
5. a raw vs certified render-output split
6. a new output structure for the v1.1 runtime planning artifacts

It does not include:

1. persona/content pack planning
2. domain material planning
3. persona distinctness/content acceptance work
4. production release process design

Those remain outside this spec and should be planned as a separate workstream.

## Why V1.1 Exists

The current `tech-plan-v1` executable plan set got the main architectural direction right:

1. Stage 1 pre-renders the worker-sendable artifacts
2. Stage 2 is not allowed to recompose prompts from raw structured inputs
3. `run_root` is treated as the primary evidence location

However, the current plan set still has structural gaps that will cause implementation-time churn:

1. `dispatch_input_v1` field naming drifted away from the incident report's authoritative names
2. `wv-answer-stage` has no owner plan
3. there is no true CLI/config/schema-validation entrypoint plan
4. the Stage 2 seal chain is incomplete
5. model review is still too entangled with the deterministic runtime gate
6. render outputs are still too tightly coupled to certification gating

The v1.1 redesign exists to fix those issues before implementation starts.

## Core Decisions

1. Keep the current `docs/plans/tech-plan-v1/` plan set as a reference version.
2. Create a parallel runtime plan set for v1.1 instead of rewriting v1 in place.
3. Introduce a new `P00 CLI + Config + Schema Validation` plan as the first runtime entrypoint.
4. Treat the incident report's five `dispatch_input_v1` fields as the only authoritative field names.
5. Move `wv-answer-stage` ownership into the single-worker execution plan.
6. Split the current prepare gate into:
   - a hard deterministic gate
   - a non-blocking soft review
7. Split render outputs into:
   - raw outputs
   - certified outputs
8. Keep the content/persona-pack line out of this runtime redesign.

## Authoritative Dispatch Input Contract

`dispatch_input_v1` must use only these five field names:

1. `roleplay_prompt`
2. `discussion_question`
3. `supplementary_materials`
4. `output_contract`
5. `assumptions_and_constraints`

No aliases are allowed in the runtime plan set.

This is an audit and evidence requirement, not a style preference. The runtime plan set must not introduce a second naming system for the same input structure.

## V1.1 Runtime Plan Topology

The redesigned runtime plan set is:

1. `P00 CLI + Config + Schema Validation`
2. `P01 Codex Runtime Contract`
3. `P02 App-Server Client Lifecycle`
4. `P03 Run Layout + Artifact Writer`
5. `P04 Prepare Input Builder`
6. `P05 Prepare Hard Gate`
7. `P06 Prepare Soft Review`
8. `P07 Answer Workspace Seal`
9. `P08 Answer Single Worker Execution`
10. `P09 Answer Batch Orchestration`
11. `P10 Render Input Aggregation`
12. `P11 Render Outputs`

## Runtime Plan Roles

### P00 CLI + Config + Schema Validation

This is the new product entrypoint plan. It owns:

1. `cmd/worldview-panel/main.go`
2. `internal/config/config.go`
3. `internal/schema/validate.go`

It defines the runtime's CLI surface, config loading, schema validation entrypoints, and default values for `question`, `materials`, `persona-set`, `outdir`, `concurrency`, and `model`.

### P01 Codex Runtime Contract

This remains the pinned Codex/runtime contract plan:

1. pinned CLI version
2. `app-server` transport choice
3. protocol schema output location
4. runtime smoke command

### P02 App-Server Client Lifecycle

This remains the thin process and JSON-RPC lifecycle plan:

1. `initialize`
2. `initialized`
3. `configRequirements/read`
4. `thread/start`
5. `turn/start`
6. completion handling
7. interrupt handling
8. process shutdown

### P03 Run Layout + Artifact Writer

This remains the stable filesystem and writing layer:

1. `run_root` layout
2. canonical writers
3. JSON / text / JSONL semantics
4. byte-stable and file-stable SHA-256 primitives

It does not own stage-specific hash composition.

### P04 Prepare Input Builder

This is the canonical Stage 1 assembly plan and is one of the most important rewrites in v1.1.

It must:

1. build one canonical `dispatch_input_v1.json` per persona
2. prerender `agents.md`
3. prerender `prompt.txt`
4. write `manifest.json`
5. write `hashes.json`

Canonical per-persona outputs live under:

1. `01_prepare/personas/<id>/dispatch_input_v1.json`
2. `01_prepare/personas/<id>/agents.md`
3. `01_prepare/personas/<id>/prompt.txt`
4. `01_prepare/personas/<id>/manifest.json`
5. `01_prepare/personas/<id>/hashes.json`

### P05 Prepare Hard Gate

This is a pure Go, deterministic, blocking gate.

It checks:

1. the five required `dispatch_input_v1` fields exist
2. all canonical prepare artifacts exist
3. hash artifacts exist and are complete
4. prepare directories are complete
5. required audit/log paths are writable

It does not run model review and does not assess persona quality.

### P06 Prepare Soft Review

This is a non-blocking review branch.

It may:

1. generate a Codex review report
2. emit warnings about weak persona differentiation
3. flag suspicious material assembly
4. flag output-contract issues

By default it does not block Stage 2.

### P07 Answer Workspace Seal

This is the second critical rewrite in v1.1 and is the center of the Stage 2 incident response.

It must create the worker-side sealed inputs:

1. `02_answer/personas/<id>/workspace/AGENTS.md`
2. `02_answer/personas/<id>/input/prompt.txt`
3. `02_answer/personas/<id>/outgoing_input.json`

It must also harden the input chain with:

1. `skill_sha256`
2. `source_dispatch_input_sha256`
3. `combined_input_sha256`

It must enforce:

1. worker workspace outside the repo tree
2. worker `cwd` locked to that isolated directory
3. controlled `HOME`
4. no repo-root `AGENTS.md` contamination
5. no `~/.codex/AGENTS.md` contamination

### P08 Answer Single Worker Execution

This plan owns both:

1. `wv-answer-stage/SKILL.md`
2. the single-worker runner

It consumes only:

1. `workspace/AGENTS.md`
2. the worker-side prompt snapshot
3. `outgoing_input.json`
4. the answer-stage skill

It treats `item/completed.agentMessage` as the only authoritative final model output.

It writes:

1. `result.raw.txt`
2. `result.json`
3. `attestation.json`
4. `status.json`

### P09 Answer Batch Orchestration

This remains the Stage 2 fan-out layer and owns:

1. concurrency
2. timeout handling
3. forbidden-tool detection
4. per-persona aggregation
5. batch-level outcome reporting

It keeps the persona result model:

1. `certified`
2. `failed`
3. `rejected`

### P10 Render Input Aggregation

This plan is rewritten to produce two classes of render input:

1. `raw_render_input.json`
2. `certified_render_input.json`

It must:

1. read only current-run Stage 2 artifacts
2. compute the certification gate
3. allow raw rendering when there is displayable output
4. reserve certification gating for the certified path only

### P11 Render Outputs

This plan is rewritten to produce two classes of outputs under the render root:

1. raw render outputs
2. certified render outputs

It must write only under the render output root and must never modify `02_answer`.

## Approved Dependency Graph

The approved runtime graph for v1.1 is:

```text
P00 -> P02
P01 -> P02
P03 -> P04
P03 -> P05
P03 -> P07
P03 -> P10
P04 -> P05
P04 -> P06
P05 -> P07
P01 -> P08
P02 -> P08
P07 -> P08
P08 -> P09
P09 -> P10
P10 -> P11
```

Interpretation:

1. `P00`, `P01`, and `P03` are the primary starting points.
2. `P04 -> P05 -> P07 -> P08` is the runtime input hardening chain.
3. `P06` is a non-blocking branch off `P04`.
4. `P09 -> P10 -> P11` remains the Stage 2 to render chain.

## Input Seal Chain

The canonical input chain must be:

1. `P04` canonical assembly
2. `P04` prerendered sendable artifacts
3. `P04` manifest and hashes
4. `P07` worker-side sealed snapshot
5. `P07` outgoing seal record

### P04 Canonical Assembly

Each persona gets one canonical:

1. `dispatch_input_v1.json`

### P04 Prerendered Sendable Artifacts

Each persona gets:

1. `agents.md`
2. `prompt.txt`

After that point, Stage 2 may not recompose the worker prompt from `dispatch_input_v1.json`.

### P04 Manifest And Hashes

Each persona gets:

1. `manifest.json`
2. `hashes.json`

Required hash names:

1. `dispatch_input_sha256`
2. `agents_sha256`
3. `prompt_sha256`
4. `bundle_sha256`

`bundle_sha256` must hash canonical content, not filesystem paths.

### P07 Worker-Side Sealed Snapshot

Before Stage 2 execution, each persona gets:

1. `workspace/AGENTS.md`
2. `input/prompt.txt`
3. `outgoing_input.json`

### P07 Outgoing Seal Record

`outgoing_input.json` must at least include:

1. `persona_id`
2. `agent_instructions_path`
3. `agent_instructions_sha256`
4. `prompt_path`
5. `prompt_sha256`
6. `skill_path`
7. `skill_sha256`
8. `source_dispatch_input_sha256`
9. `combined_input_sha256`

`combined_input_sha256` must be defined over the canonical bytes of:

1. worker system input
2. worker user input
3. answer-stage skill content

## Stage 2 Input Allowlist

Stage 2 may read only:

1. `workspace/AGENTS.md`
2. `input/prompt.txt`
3. `outgoing_input.json`
4. `wv-answer-stage/SKILL.md`

Stage 2 must not directly read:

1. `dispatch_input_v1.json`
2. repo-root `AGENTS.md`
3. `~/.codex/AGENTS.md`
4. any ad hoc reconstructed prompt

## Hard Gate And Soft Review Split

### Hard Gate

`P05` is the only mandatory blocking prepare gate.

It blocks Stage 2 when deterministic runtime conditions are not satisfied.

### Soft Review

`P06` is advisory by default.

It may produce:

1. warnings
2. review reports
3. follow-up guidance

It does not block the main runtime chain unless a future configuration explicitly changes that behavior.

## Raw And Certified Render Split

### P10 Render Inputs

`P10` must produce:

1. `raw_render_input.json`
2. `certified_render_input.json`

### P11 Render Outputs

`P11` must produce:

1. `raw_panel.json`
2. `raw_panel.md`
3. `raw_cards.json`
4. `certified_panel.json`
5. `certified_panel.md`
6. `certified_cards.json`
7. `03_render/status.json`

### Certification Rule

The minimum success-rate rule applies only to the certified path.

That means:

1. raw outputs may still be produced when there is displayable output
2. certified outputs require the threshold to pass

The gate condition must include the explicit zero-denominator guard:

1. fail immediately if `T = 0`
2. pass only if `T > 0`, `C > 0`, and `C / T >= min_success_rate`

Default:

1. `min_success_rate = 0.8`

## Output Structure For V1.1 Runtime Planning

This redesign should not overwrite the current v1 plan set.

The target output structure is:

1. `docs/plans/tech-plan-v1.1-runtime/AGENTS.md`
2. `docs/plans/tech-plan-v1.1-runtime/README.md`
3. `docs/plans/tech-plan-v1.1-runtime/_index.json`
4. `docs/plans/tech-plan-v1.1-runtime/00-cli-config-schema-validation.md`
5. `docs/plans/tech-plan-v1.1-runtime/01-codex-runtime-contract.md`
6. `docs/plans/tech-plan-v1.1-runtime/02-app-server-client-lifecycle.md`
7. `docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md`
8. `docs/plans/tech-plan-v1.1-runtime/04-prepare-input-builder.md`
9. `docs/plans/tech-plan-v1.1-runtime/05-prepare-hard-gate.md`
10. `docs/plans/tech-plan-v1.1-runtime/06-prepare-soft-review.md`
11. `docs/plans/tech-plan-v1.1-runtime/07-answer-workspace-seal.md`
12. `docs/plans/tech-plan-v1.1-runtime/08-answer-single-worker-execution.md`
13. `docs/plans/tech-plan-v1.1-runtime/09-answer-batch-orchestration.md`
14. `docs/plans/tech-plan-v1.1-runtime/10-render-input-aggregation.md`
15. `docs/plans/tech-plan-v1.1-runtime/11-render-outputs.md`

## Success Criteria For The V1.1 Runtime Spec

The redesign is acceptable only if all of the following are true:

1. the runtime plan set has a true executable entrypoint plan
2. `dispatch_input_v1` uses only the incident report's five required field names
3. the Stage 1 to Stage 2 input chain is single-path and auditable
4. Stage 2 input sealing records the actual worker-side input chain with hashes
5. `P05` is the only mandatory prepare gate
6. `P06` is non-blocking by default
7. raw render outputs are available without certified-threshold success
8. certified outputs remain threshold-gated
9. the runtime redesign does not depend on the content-pack workstream in order to begin implementation

## Out Of Scope Workstream

The following should be treated as a separate planning line and are not part of this runtime redesign:

1. persona pack schema
2. domain material pack design
3. persona distinctness acceptance
4. anti-template content policy
5. domain anchoring for persona packs

## Handoff To Planning

The next step is to turn this redesign into a new implementation plan that writes the parallel `tech-plan-v1.1-runtime` plan set, preserving:

1. the new topology
2. the new dependency graph
3. the five authoritative `dispatch_input_v1` field names
4. the hard-gate / soft-review split
5. the raw / certified render split
