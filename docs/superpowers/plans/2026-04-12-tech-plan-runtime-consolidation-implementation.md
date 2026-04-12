# Tech Plan Runtime Consolidation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate the repository onto one retained runtime plan set at `docs/plans/tech-plan-runtime/`, add `P12 End-to-End Smoke + Makefile Flow`, and update all planning entrypoints without changing runtime implementation code.

**Architecture:** This is a planning-artifact migration, not a runtime-code change. The work is split into a directory rename, plan-set top-level rewrites, targeted plan-file corrections, a new terminal smoke plan, and final consistency validation so Codex and humans see one canonical runtime plan surface.

**Tech Stack:** Markdown, JSON, shell file operations, `rg`, `jq`

---

## File Structure

### Create

- `docs/plans/tech-plan-runtime/12-end-to-end-smoke-and-makefile-flow.md`
- `docs/superpowers/plans/2026-04-12-tech-plan-runtime-consolidation-implementation.md`

### Rename

- `docs/plans/tech-plan-v1.1-runtime/` -> `docs/plans/tech-plan-runtime/`

### Modify

- `AGENTS.md`
- `README.md`
- `docs/plans/tech-plan-runtime/AGENTS.md`
- `docs/plans/tech-plan-runtime/README.md`
- `docs/plans/tech-plan-runtime/_index.json`
- `docs/plans/tech-plan-runtime/00-cli-config-schema-validation.md`
- `docs/plans/tech-plan-runtime/01-codex-runtime-contract.md`
- `docs/plans/tech-plan-runtime/02-app-server-client-lifecycle.md`
- `docs/plans/tech-plan-runtime/03-run-layout-and-artifact-writer.md`
- `docs/plans/tech-plan-runtime/04-prepare-input-builder.md`
- `docs/plans/tech-plan-runtime/05-prepare-hard-gate.md`
- `docs/plans/tech-plan-runtime/06-prepare-soft-review.md`
- `docs/plans/tech-plan-runtime/07-answer-workspace-seal.md`
- `docs/plans/tech-plan-runtime/08-answer-single-worker-execution.md`
- `docs/plans/tech-plan-runtime/09-answer-batch-orchestration.md`
- `docs/plans/tech-plan-runtime/10-render-input-aggregation.md`
- `docs/plans/tech-plan-runtime/11-render-outputs.md`
- `docs/superpowers/specs/2026-04-12-tech-plan-runtime-consolidation-design.md`

### Delete

- `docs/plans/tech-plan-v1.1-runtime/` after successful rename

## Task 1: Rename the retained plan directory

**Files:**
- Rename: `docs/plans/tech-plan-v1.1-runtime/` -> `docs/plans/tech-plan-runtime/`

- [ ] **Step 1: Move the retained plan-set directory**

Run:

```sh
mv docs/plans/tech-plan-v1.1-runtime docs/plans/tech-plan-runtime
```

Expected: `docs/plans/tech-plan-runtime/` exists and contains the former `P00` through `P11` files plus `AGENTS.md`, `README.md`, and `_index.json`.

- [ ] **Step 2: Confirm the old directory path is gone**

Run:

```sh
test ! -d docs/plans/tech-plan-v1.1-runtime
```

Expected: exit status `0`.

- [ ] **Step 3: Confirm the new directory path is present**

Run:

```sh
find docs/plans/tech-plan-runtime -maxdepth 1 -type f | sort
```

Expected output includes:

```text
docs/plans/tech-plan-runtime/00-cli-config-schema-validation.md
docs/plans/tech-plan-runtime/11-render-outputs.md
docs/plans/tech-plan-runtime/AGENTS.md
docs/plans/tech-plan-runtime/README.md
docs/plans/tech-plan-runtime/_index.json
```

- [ ] **Step 4: Commit the pure rename**

```sh
git add docs/plans/tech-plan-runtime
git commit -m "docs: rename runtime plan set directory"
```

## Task 2: Rewrite the plan-set dispatcher and overview

**Files:**
- Modify: `docs/plans/tech-plan-runtime/AGENTS.md`
- Modify: `docs/plans/tech-plan-runtime/README.md`

- [ ] **Step 1: Rewrite `docs/plans/tech-plan-runtime/AGENTS.md` to the new canonical directory and graph**

Replace the existing dispatcher body with this shape:

```md
# Tech Plan Runtime Execution Guide

## Purpose

Execute the retained runtime plan set using the files in `docs/plans/tech-plan-runtime/`.

## Plan Directory

`docs/plans/tech-plan-runtime/`

## Execution Graph

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
```

- [ ] **Step 2: Add hard dispatch rules for Stage 2 contamination protection**

Ensure `docs/plans/tech-plan-runtime/AGENTS.md` includes these exact rule themes:

```md
## Dispatch Rules

1. Pick the lowest-numbered incomplete plan whose `depends_on` entries are all `done`.
2. Read only this `AGENTS.md`, `README.md`, the target plan file, and directly required dependency plans.
3. Do not start implementation from `README.md` alone.
4. `dispatch_input_v1` uses only:
   - `roleplay_prompt`
   - `discussion_question`
   - `supplementary_materials`
   - `output_contract`
   - `assumptions_and_constraints`
5. Stage 2 plans must not reopen `dispatch_input_v1.json`.
6. Worker workspace must live outside the repo tree, with controlled `HOME` and `CODEX_HOME`.
7. Raw and certified render outputs are separate permanent boundaries.
8. After finishing a plan, update both the target plan frontmatter and `docs/plans/tech-plan-runtime/_index.json`.
```

- [ ] **Step 3: Rewrite `docs/plans/tech-plan-runtime/README.md` as the single retained runtime plan-set overview**

Ensure it includes these sections and statements:

```md
## Why This Exists
This is the repository's single retained runtime plan set.

## What This Runtime Plan Solves
1. canonical Stage 1 input assembly
2. deterministic prepare gating
3. Stage 2 seal-chain closure
4. authoritative worker-result boundaries
5. raw versus certified render splitting

## What This Runtime Plan Does Not Solve
1. persona/content-pack quality
2. persona distinctness
3. domain-material curation

Those belong to a separate parallel content-planning line.
```

- [ ] **Step 4: Commit the plan-set overview rewrite**

```sh
git add docs/plans/tech-plan-runtime/AGENTS.md docs/plans/tech-plan-runtime/README.md
git commit -m "docs: rewrite runtime plan set entry docs"
```

## Task 3: Update the machine index and add P12

**Files:**
- Modify: `docs/plans/tech-plan-runtime/_index.json`

- [ ] **Step 1: Rename the plan-set identity in `_index.json`**

Change the header fields to:

```json
{
  "plan_set_id": "tech-plan-runtime",
  "source": "docs/superpowers/specs/2026-04-12-tech-plan-runtime-consolidation-design.md",
  "last_updated": "2026-04-12"
}
```

- [ ] **Step 2: Rewrite every `path` entry to the new canonical directory**

Run:

```sh
perl -0pi -e 's#docs/plans/tech-plan-v1\\.1-runtime/#docs/plans/tech-plan-runtime/#g' docs/plans/tech-plan-runtime/_index.json
```

Expected: no remaining `tech-plan-v1.1-runtime` strings in `_index.json`.

- [ ] **Step 3: Add the new `P12` index entry**

Append this plan object to the `plans` array:

```json
{
  "plan_id": "P12",
  "title": "End-to-End Smoke + Makefile Flow",
  "status": "ready",
  "depends_on": ["P11"],
  "path": "docs/plans/tech-plan-runtime/12-end-to-end-smoke-and-makefile-flow.md",
  "produces": [
    "Makefile",
    "tools/smokecheck/main.go",
    "testdata/smoke/materials/",
    "testdata/smoke/runtime/",
    "docs/operations/runtime-smoke-and-release-checklist.md"
  ]
}
```

- [ ] **Step 4: Validate the index JSON**

Run:

```sh
jq empty docs/plans/tech-plan-runtime/_index.json
```

Expected: no output and exit status `0`.

- [ ] **Step 5: Commit the machine-index update**

```sh
git add docs/plans/tech-plan-runtime/_index.json
git commit -m "docs: update runtime plan index for consolidated plan set"
```

## Task 4: Apply mechanical path updates across the retained plans

**Files:**
- Modify: `docs/plans/tech-plan-runtime/00-cli-config-schema-validation.md`
- Modify: `docs/plans/tech-plan-runtime/01-codex-runtime-contract.md`
- Modify: `docs/plans/tech-plan-runtime/02-app-server-client-lifecycle.md`
- Modify: `docs/plans/tech-plan-runtime/03-run-layout-and-artifact-writer.md`
- Modify: `docs/plans/tech-plan-runtime/04-prepare-input-builder.md`
- Modify: `docs/plans/tech-plan-runtime/05-prepare-hard-gate.md`
- Modify: `docs/plans/tech-plan-runtime/06-prepare-soft-review.md`
- Modify: `docs/plans/tech-plan-runtime/07-answer-workspace-seal.md`
- Modify: `docs/plans/tech-plan-runtime/08-answer-single-worker-execution.md`
- Modify: `docs/plans/tech-plan-runtime/09-answer-batch-orchestration.md`
- Modify: `docs/plans/tech-plan-runtime/10-render-input-aggregation.md`
- Modify: `docs/plans/tech-plan-runtime/11-render-outputs.md`

- [ ] **Step 1: Replace the old plan-directory string in all retained plans**

Run:

```sh
rg -l 'docs/plans/tech-plan-v1\.1-runtime/' docs/plans/tech-plan-runtime | \
  xargs perl -0pi -e 's#docs/plans/tech-plan-v1\\.1-runtime/#docs/plans/tech-plan-runtime/#g'
```

Expected: all retained plan files now reference `docs/plans/tech-plan-runtime/`.

- [ ] **Step 2: Verify there are no stale directory references left in the retained plan set**

Run:

```sh
rg -n 'docs/plans/tech-plan-v1\.1-runtime/' docs/plans/tech-plan-runtime
```

Expected: no matches.

- [ ] **Step 3: Commit the mechanical path migration**

```sh
git add docs/plans/tech-plan-runtime
git commit -m "docs: migrate retained runtime plan references"
```

## Task 5: Strengthen the materially affected plan files

**Files:**
- Modify: `docs/plans/tech-plan-runtime/00-cli-config-schema-validation.md`
- Modify: `docs/plans/tech-plan-runtime/07-answer-workspace-seal.md`
- Modify: `docs/plans/tech-plan-runtime/08-answer-single-worker-execution.md`
- Modify: `docs/plans/tech-plan-runtime/10-render-input-aggregation.md`
- Modify: `docs/plans/tech-plan-runtime/11-render-outputs.md`

- [ ] **Step 1: Clarify `P00` as product entry but not smoke owner**

Add language like:

```md
- P00 is the product-entry plan for `cmd/worldview-panel/main.go`, startup config, and startup validation.
- P00 does not own end-to-end smoke. Full-flow validation belongs to P12.
```

- [ ] **Step 2: Harden `P07` against AGENTS contamination**

Ensure `07-answer-workspace-seal.md` explicitly states:

```md
- worker workspace must live outside the repo tree
- worker `cwd` must be the isolated workspace
- worker `HOME` must be the isolated home directory
- worker `CODEX_HOME` must be set under the isolated home boundary
- repo-root and user-home `AGENTS.md` files must be unreachable by ancestor traversal during persona execution
```

- [ ] **Step 3: Reaffirm `P08` skill ownership**

Ensure `08-answer-single-worker-execution.md` contains:

```md
- P08 owns `runtime/skills/wv-answer-stage/SKILL.md`
- P07 may copy the skill bytes, but it does not own skill wording or worker-execution semantics
```

- [ ] **Step 4: Freeze raw/certified render splitting in `P10`**

Add language like:

```md
- raw and certified render-input artifacts are separate permanent Stage 3 contracts
- raw exists for visibility into displayable-but-not-certified outputs
- certified exists for trusted output only
- this split must not be collapsed back into a single render input
```

- [ ] **Step 5: Make `P11` hand off directly to `P12`**

Add a handoff block like:

```md
## Handoff

- P12 consumes the completed Stage 3 output boundary from P11.
- P12 validates the real CLI-to-panel flow through Makefile entrypoints.
- P12 does not redefine render semantics; it verifies the chain as-operated.
```

- [ ] **Step 6: Commit the targeted plan corrections**

```sh
git add \
  docs/plans/tech-plan-runtime/00-cli-config-schema-validation.md \
  docs/plans/tech-plan-runtime/07-answer-workspace-seal.md \
  docs/plans/tech-plan-runtime/08-answer-single-worker-execution.md \
  docs/plans/tech-plan-runtime/10-render-input-aggregation.md \
  docs/plans/tech-plan-runtime/11-render-outputs.md
git commit -m "docs: strengthen retained runtime plan boundaries"
```

## Task 6: Create P12 End-to-End Smoke + Makefile Flow

**Files:**
- Create: `docs/plans/tech-plan-runtime/12-end-to-end-smoke-and-makefile-flow.md`

- [ ] **Step 1: Add the plan frontmatter**

Create the file with this header:

```md
---
plan_id: P12
title: End-to-End Smoke + Makefile Flow
status: ready
depends_on:
  - P11
consumes:
  - docs/plans/tech-plan-runtime/11-render-outputs.md
produces:
  - Makefile
  - tools/smokecheck/main.go
  - testdata/smoke/materials/
  - testdata/smoke/runtime/
  - docs/operations/runtime-smoke-and-release-checklist.md
completion_evidence:
  - make_smoke_is_canonical_entrypoint
  - smoke_dataset_is_fixed_minimum
  - smokecheck_validates_branch_state_coherence
---
```

- [ ] **Step 2: Write the goal, scope, and out-of-scope sections**

Use this structure:

```md
# Goal
Own the real end-to-end runtime validation flow so operators can run one Makefile-driven smoke path from CLI to panel artifacts.

# Scope
- `Makefile`
- smoke fixtures
- smoke verifier
- runtime smoke and release checklist

# Out Of Scope
- CLI argument design
- app-server lifecycle semantics
- prepare/answer/render logic ownership
- certification-policy math
```

- [ ] **Step 3: Write the implementation tasks and acceptance checks**

Include these exact operator commands:

```md
- `make build`
- `make smoke`
- `make smoke-check RUN_ID=<run_id>`
```

And include these acceptance points:

```md
- `make smoke` is the single recommended full-flow smoke entrypoint
- the smoke dataset is fixed to `2 personas` and `1 material`
- smoke produces `01_prepare/prepare_gate_status_v1.json`
- smoke produces `02_answer/answer_batch.json`
- smoke produces `03_render/raw_render_input.json`
- smoke produces `03_render/certified_render_input.json`
- smoke produces `03_render/status.json`
- `smoke-check` validates structure and raw/certified branch-state coherence
- the Makefile handles shared-storage `go.mod` locking and repo `noexec` limitations
```

- [ ] **Step 4: Commit the new terminal validation plan**

```sh
git add docs/plans/tech-plan-runtime/12-end-to-end-smoke-and-makefile-flow.md
git commit -m "docs: add end-to-end smoke plan"
```

## Task 7: Update repository-level human and Codex entrypoints

**Files:**
- Modify: `AGENTS.md`
- Modify: `README.md`

- [ ] **Step 1: Point the repository-level Codex guide at the new retained plan-set path**

Update `AGENTS.md` so the retained plan-set reference is:

```md
- `docs/plans/tech-plan-runtime/`
```

And update the source-of-truth / first-read guidance to use the same canonical path.

- [ ] **Step 2: Update the repository-level README if it mentions the old plan-set path**

Run:

```sh
rg -n 'tech-plan-v1\.1-runtime|tech-plan-runtime' README.md AGENTS.md
```

Expected: any retained plan-set mentions use only `docs/plans/tech-plan-runtime/`.

- [ ] **Step 3: Commit the repository-level entrypoint update**

```sh
git add AGENTS.md README.md
git commit -m "docs: point repository entry docs at consolidated runtime plans"
```

## Task 8: Final consistency validation

**Files:**
- Verify: `docs/plans/tech-plan-runtime/`
- Verify: `docs/superpowers/specs/2026-04-12-tech-plan-runtime-consolidation-design.md`

- [ ] **Step 1: Confirm there is only one retained runtime plan directory**

Run:

```sh
find docs/plans -maxdepth 1 -mindepth 1 -type d | sort
```

Expected output includes `docs/plans/tech-plan-runtime` and does not include `docs/plans/tech-plan-v1.1-runtime`.

- [ ] **Step 2: Confirm all retained plan files have `plan_id` and the new path**

Run:

```sh
rg -n '^plan_id:' docs/plans/tech-plan-runtime/[0-9][0-9]-*.md
rg -n 'docs/plans/tech-plan-v1\.1-runtime/' docs/plans/tech-plan-runtime AGENTS.md README.md docs/superpowers/specs/2026-04-12-tech-plan-runtime-consolidation-design.md
```

Expected:

```text
- every plan file reports one `plan_id`
- the second command returns no matches
```

- [ ] **Step 3: Validate index and graph coherence**

Run:

```sh
jq -r '.plans[].plan_id' docs/plans/tech-plan-runtime/_index.json
rg -n 'P11 -> P12|tech-plan-runtime' docs/plans/tech-plan-runtime/AGENTS.md docs/plans/tech-plan-runtime/README.md
```

Expected:

```text
- `P00` through `P12` are present in `_index.json`
- `P11 -> P12` is present in the execution graph
- both plan-set top-level docs use `tech-plan-runtime`
```

- [ ] **Step 4: Commit the finalized consolidation**

```sh
git add AGENTS.md README.md docs/plans/tech-plan-runtime docs/superpowers/specs/2026-04-12-tech-plan-runtime-consolidation-design.md
git commit -m "docs: consolidate runtime plan set"
```

## Self-Review

Spec coverage:

- directory consolidation: covered by Tasks 1, 4, 7, 8
- retained single runtime plan set: covered by Tasks 1, 2, 3, 8
- `P12` smoke ownership: covered by Tasks 3 and 6
- Stage 2 contamination hardening in top-level docs: covered by Tasks 2 and 5
- runtime/content separation: covered by Task 2

Placeholder scan:

- no `TODO`, `TBD`, or deferred placeholders remain

Type consistency:

- canonical retained path is always `docs/plans/tech-plan-runtime/`
- new terminal plan id is always `P12`
- `_index.json` and `AGENTS.md` both use `P11 -> P12`

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-12-tech-plan-runtime-consolidation-implementation.md`. Two execution options:

1. Subagent-Driven (recommended) - I dispatch a fresh subagent per task, review between tasks, fast iteration
2. Inline Execution - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
