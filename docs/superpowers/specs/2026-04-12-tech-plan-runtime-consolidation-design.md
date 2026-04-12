# Tech Plan Runtime Consolidation Design

Date: 2026-04-12

Source inputs:

1. the retained runtime plan set that will live at `docs/plans/tech-plan-runtime/`
2. the already approved runtime redesign decisions encoded in the retained plan set
3. user review of the current runtime plan set

## Goal

Consolidate the repository onto one retained runtime plan set, remove versioned plan-directory naming, and correct the remaining planning gaps around product entry, end-to-end smoke ownership, top-level dispatch rules, and external content-planning boundaries.

## Scope

This redesign covers planning artifacts only.

It includes:

1. finalizing the retained plan directory at `docs/plans/tech-plan-runtime/`
2. updating all retained runtime plan-set references to the new canonical directory
3. keeping a single retained runtime plan set in the repository
4. adding a new `P12 End-to-End Smoke + Makefile Flow` plan
5. tightening top-level runtime-plan documentation around:
   - authoritative Stage 1 field naming
   - Stage 2 contamination and seal-chain boundaries
   - raw versus certified render-output boundaries
   - product-entry versus end-to-end ownership
6. explicitly documenting persona/content-pack planning as a separate parallel workstream rather than part of the runtime plan set

It does not include:

1. runtime implementation changes
2. schema changes beyond planning text corrections
3. persona/content-pack plan authoring
4. redesigning the core runtime topology introduced in the retained v1.1 runtime plan set

## Why This Consolidation Exists

The retained runtime plan set is already structurally close to implementation-ready:

1. the Stage 1 to Stage 2 input chain is mostly corrected
2. Stage 2 sealing no longer relies on ad hoc prompt recomposition
3. Stage 2 authoritative output rules are explicit
4. Stage 3 raw versus certified splitting already exists

However, there are still planning-level problems that will cause unnecessary churn if left in place:

1. the runtime plan set still carries versioned directory naming even though the repository should retain only one active runtime plan set
2. the product-entry story is still spread across plan text rather than reinforced at the plan-set level
3. the end-to-end smoke flow is implemented in the repository but not owned as a first-class runtime plan
4. the top-level plan dispatch rules should state Stage 2 contamination protections more explicitly
5. the runtime plan set should make it impossible to mistake runtime stability work for persona/content quality work

This consolidation exists to fix those planning gaps without reopening the core runtime architecture.

## Core Decisions

1. Retain exactly one runtime plan set in the repository.
2. Rename the retained directory to `docs/plans/tech-plan-runtime/`.
3. Keep `docs/tech-plan-v1.md` as the historical source document for the original runtime direction.
4. Preserve the existing runtime topology from `P00` through `P11`.
5. Add a new `P12 End-to-End Smoke + Makefile Flow` plan rather than folding smoke ownership into `P00` or `P11`.
6. Treat `P00` as the product-entry plan and `P12` as the end-to-end validation plan.
7. Keep persona/content-pack work outside the runtime plan set, but state clearly that it remains required for product quality.

## Canonical Runtime Plan Directory

The retained plan directory becomes:

- `docs/plans/tech-plan-runtime/`

The following former runtime-plan location is removed from active planning use:

- the previous version-tagged runtime plan directory

Git history remains the version record. The repository should not keep multiple version-tagged runtime plan directories in parallel.

## Retained Runtime Topology

The consolidated runtime plan set is:

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
13. `P12 End-to-End Smoke + Makefile Flow`

This is not a fresh topology redesign. It is the retained runtime graph plus one new terminal validation plan.

## Execution Graph

The consolidated execution graph is:

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
P02 -> P11
P10 -> P11
P11 -> P12
```

`P12` is deliberately terminal. It validates the full runtime chain but does not change the ownership of earlier plans.

## Top-Level Planning Corrections

### 1. Product-entry ownership stays in P00

`P00` remains the owner of:

1. `cmd/worldview-panel/main.go`
2. `internal/config/config.go`
3. `internal/schema/validate.go`

The plan-set README and `_index.json` must state this clearly. `P00` is the product-entry plan, but it does not own end-to-end smoke.

### 2. Smoke ownership moves into P12

`P12` owns the stable end-to-end runtime validation surface:

1. `Makefile`
2. `tools/smokecheck/main.go`
3. `testdata/smoke/materials/...`
4. `testdata/smoke/runtime/...`
5. `docs/operations/runtime-smoke-and-release-checklist.md`

Fixed operator entrypoints:

1. `make build`
2. `make smoke`
3. `make smoke-check RUN_ID=<run_id>`

`P12` does not own runtime semantics. It owns how the full runtime is exercised and validated as one product flow.

### 3. Stage 2 contamination protections become top-level rules

The consolidated plan-set `AGENTS.md` must state these as hard dispatch rules:

1. `dispatch_input_v1` uses only:
   - `roleplay_prompt`
   - `discussion_question`
   - `supplementary_materials`
   - `output_contract`
   - `assumptions_and_constraints`
2. Stage 2 plans must not reopen `dispatch_input_v1.json`.
3. worker workspaces must live outside the repo tree
4. worker `cwd` must be the isolated workspace
5. worker `HOME` and `CODEX_HOME` must be controlled so repo-level and user-level `AGENTS.md` files cannot contaminate persona sessions
6. raw and certified render outputs are separate permanent boundaries, not a temporary development convenience

### 4. Content planning remains external but mandatory for product quality

The runtime plan-set README must say explicitly:

1. runtime planning solves process closure, artifact authority, and runtime evidence
2. runtime planning does not guarantee persona distinctness or product-value differentiation
3. persona/content-pack planning is a separate parallel workstream, not an optional afterthought

## P12 End-to-End Smoke + Makefile Flow

### Goal

Own the real end-to-end runtime validation flow so the repository has one stable operator entrypoint for exercising the complete chain from CLI to panel artifacts.

### Scope

`P12` owns:

1. `Makefile` entrypoints for build and smoke
2. smoke fixture inputs
3. smoke result verification logic
4. runtime smoke and release checklist documentation

`P12` does not own:

1. CLI argument design
2. app-server lifecycle semantics
3. prepare, answer, or render stage logic
4. certification-policy math itself

### Required Inputs

`P12` depends on the runtime chain already existing:

1. `P00` product entry
2. `P08` single worker execution
3. `P09` batch orchestration
4. `P10` render aggregation
5. `P11` render outputs

### Fixed Entrypoints

The end-to-end operator surface is fixed to:

1. `make build`
2. `make smoke`
3. `make smoke-check RUN_ID=<run_id>`

`make smoke` must:

1. build the binary
2. run the real runtime flow using the fixed smoke dataset
3. locate the resulting smoke run
4. invoke the checker

### Acceptance

`P12` is complete only if:

1. `make smoke` is the single recommended full-flow smoke entrypoint
2. the smoke dataset is a fixed minimal repeatable set:
   - `2 personas`
   - `1 material`
3. smoke produces at least:
   - `01_prepare/prepare_gate_status_v1.json`
   - `02_answer/answer_batch.json`
   - `03_render/raw_render_input.json`
   - `03_render/certified_render_input.json`
   - `03_render/status.json`
4. `smoke-check` validates structure and branch-state coherence, not just file existence
5. the Makefile handles shared-storage operational realities:
   - `go.mod` locking limitations
   - repo-path `noexec` limitations
6. the formal operator documentation lives in:
   - `README.md` for entrypoint discovery
   - `docs/operations/runtime-smoke-and-release-checklist.md` for full procedure

## Files To Change

### Repository-level files

Modify:

1. `AGENTS.md`
2. `README.md`

### Plan-set directory rename

Rename:

1. ensure the retained runtime plan set lives at `docs/plans/tech-plan-runtime/`

### Plan-set top-level files

Modify:

1. `docs/plans/tech-plan-runtime/AGENTS.md`
2. `docs/plans/tech-plan-runtime/README.md`
3. `docs/plans/tech-plan-runtime/_index.json`

### Plan files to adjust materially

Modify:

1. `docs/plans/tech-plan-runtime/00-cli-config-schema-validation.md`
2. `docs/plans/tech-plan-runtime/07-answer-workspace-seal.md`
3. `docs/plans/tech-plan-runtime/08-answer-single-worker-execution.md`
4. `docs/plans/tech-plan-runtime/10-render-input-aggregation.md`
5. `docs/plans/tech-plan-runtime/11-render-outputs.md`

Create:

1. `docs/plans/tech-plan-runtime/12-end-to-end-smoke-and-makefile-flow.md`

### Plan files that only need mechanical path updates

Modify path references only:

1. `docs/plans/tech-plan-runtime/01-codex-runtime-contract.md`
2. `docs/plans/tech-plan-runtime/02-app-server-client-lifecycle.md`
3. `docs/plans/tech-plan-runtime/03-run-layout-and-artifact-writer.md`
4. `docs/plans/tech-plan-runtime/04-prepare-input-builder.md`
5. `docs/plans/tech-plan-runtime/05-prepare-hard-gate.md`
6. `docs/plans/tech-plan-runtime/06-prepare-soft-review.md`
7. `docs/plans/tech-plan-runtime/09-answer-batch-orchestration.md`

## Success Standard

The consolidation is accepted only if:

1. the repository retains one active runtime plan directory:
   - `docs/plans/tech-plan-runtime/`
2. all retained plan-set references point to that directory and no longer point to any version-tagged runtime plan directory
3. the plan-set execution graph includes `P12` as the terminal full-flow validation plan
4. `P00` is clearly documented as the product-entry plan
5. `P12` is clearly documented as the Makefile-driven end-to-end smoke plan
6. the plan-set `AGENTS.md` hardens Stage 2 contamination protections at the dispatch-rule level
7. the plan-set README clearly separates runtime planning from persona/content-pack planning

## Next Step

The next step is to write one implementation plan that:

1. renames the plan directory
2. updates all internal references
3. adds `P12`
4. rewrites the affected top-level planning documents
5. leaves runtime implementation code untouched
