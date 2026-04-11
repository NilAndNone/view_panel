# Tech Plan V1 Executable Plans Design

Date: 2026-04-11

Source: `docs/tech-plan-v1.md`

## Goal

Convert `docs/tech-plan-v1.md` from a single technical strategy document into a plan set that Codex can execute incrementally. The target is not a rewrite of the original content. The target is a durable execution system made of one Codex-oriented entry point, one human-oriented overview, one machine index, and a bounded set of small plan files.

## Agreed Decisions

1. Use a multi-file plan set instead of keeping all executable detail inside `docs/tech-plan-v1.md`.
2. Optimize for coverage and execution clarity, even if that produces more files.
3. Split by milestone, then further split shared infrastructure into separate executable plans.
4. Keep each plan small enough to be completed in one continuous development session.
5. Put the Codex-facing execution graph in `AGENTS.md`.
6. Keep human background and rationale in `docs/plans/tech-plan-v1/README.md`.
7. Treat individual plan files as the source of truth for scope, status, dependencies, and acceptance.

## Output Structure

The plan set will live under `docs/plans/tech-plan-v1/` and will be driven by a repository-root `AGENTS.md`.

Required outputs:

1. `AGENTS.md`
2. `docs/plans/tech-plan-v1/README.md`
3. `docs/plans/tech-plan-v1/_index.json`
4. `docs/plans/tech-plan-v1/01-codex-runtime-contract.md`
5. `docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md`
6. `docs/plans/tech-plan-v1/03-run-layout-and-artifact-writer.md`
7. `docs/plans/tech-plan-v1/04-audit-log-pipeline.md`
8. `docs/plans/tech-plan-v1/05-prepare-input-builder.md`
9. `docs/plans/tech-plan-v1/06-prepare-review-gate.md`
10. `docs/plans/tech-plan-v1/07-answer-worker-workspace.md`
11. `docs/plans/tech-plan-v1/08-answer-single-worker-runner.md`
12. `docs/plans/tech-plan-v1/09-answer-batch-orchestration.md`
13. `docs/plans/tech-plan-v1/10-render-input-aggregation.md`
14. `docs/plans/tech-plan-v1/11-render-stage-output.md`

## Plan Decomposition

### P01: Codex Runtime Contract

Lock the `codex` runtime contract that later plans depend on. This plan defines the pinned CLI version, the `app-server` transport choice, schema generation location, and compatibility expectations. It does not implement business logic.

### P02: App-Server Client Lifecycle

Implement the thin Go client for `stdio` JSON-RPC lifecycle management. This plan covers handshake, per-process lifecycle, turn completion handling, interruption, and close behavior. It does not own storage layout or stage-specific orchestration.

### P03: Run Layout And Artifact Writer

Define and implement the `runs/<run_id>` directory layout, artifact path helpers, file-writing primitives, and hash helpers. This plan only handles filesystem contract and artifact persistence.

### P04: Audit Log Pipeline

Implement raw protocol capture, normalized event logging, run-level summary output, and lightweight global indexing. This plan closes the audit chain but does not build stage logic itself.

### P05: Prepare Input Builder

Load personas and materials, build canonical `dispatch_input_v1`, pre-render `agents.md` and `prompt.txt`, and write manifest and hash artifacts. This plan establishes the authoritative Stage 1 packet.

### P06: Prepare Review Gate

Run the `wv-prepare-stage` review step and convert its output into an explicit gate for Stage 2. This plan is review-only and must not mutate the canonical packet emitted by P05.

### P07: Answer Worker Workspace

Create per-persona isolated workspaces, write worker `AGENTS.md`, and produce `outgoing_input.json` strictly from Stage 1 artifacts. This plan defines what a worker is allowed to receive.

### P08: Answer Single Worker Runner

Execute one Stage 2 worker end to end, capture final `item/completed.agentMessage`, validate the structured result, and write worker result artifacts. This plan owns the single-worker happy path and terminal state handling.

### P09: Answer Batch Orchestration

Fan out across personas with concurrency control, timeout handling, forbidden-tool detection, aggregate pass/fail accounting, and certified worker result collection. This plan owns Stage 2 at batch level.

### P10: Render Input Aggregation

Collect only certified Stage 2 results, enforce the minimum success threshold, and build `render_input.json`. This plan explicitly forbids filling in missing worker answers.

### P11: Render Stage Output

Run the render stage and write `panel.json`, `panel.md`, and `cards.json`, then finalize end-of-run status. This is the last stage in the chain and must not modify Stage 2 results.

## Execution Graph

The Codex-facing execution graph lives in `AGENTS.md` and should remain intentionally short.

```text
P01 -> P02
P03 -> P04
P02 -> P04
P03 -> P05
P04 -> P06
P05 -> P06
P06 -> P07
P07 -> P08
P08 -> P09
P09 -> P10
P10 -> P11
```

Interpretation:

1. `P01` and `P03` are the only independent starting points.
2. `P04` depends on both protocol access and artifact layout.
3. `P05` begins the Stage 1 authority chain.
4. `P06` gates all Stage 2 work.
5. `P07`, `P08`, and `P09` must stay ordered because they represent the worker input, single-run execution, and batch orchestration layers of one stage.
6. `P10` and `P11` must stay ordered because render output is only valid after certified result aggregation.

## AGENTS.md Responsibilities

`AGENTS.md` is the Codex execution entry point. It should contain:

1. The purpose of the plan set.
2. The location of the plan directory.
3. The execution graph above.
4. The rule for selecting the next executable plan.
5. The rule for updating plan status.
6. The rule that a Codex session must not start implementation from `README.md` alone.

Recommended operational rules for `AGENTS.md`:

```text
Pick the lowest-numbered incomplete plan whose depends_on are all completed.
Read only AGENTS.md, README.md, the target plan file, and directly required dependency plans.
Do not start implementation from README.md alone.
Do not merge multiple plans into one execution unless the target plan says so.
After finishing a plan, update the plan frontmatter and docs/plans/tech-plan-v1/_index.json.
```

`AGENTS.md` should stay short. It is a dispatcher, not a substitute for the plan files.

## README.md Responsibilities

`docs/plans/tech-plan-v1/README.md` is the human-oriented overview. It should explain:

1. Why `docs/tech-plan-v1.md` was decomposed.
2. What each plan covers.
3. How the plans depend on each other.
4. How to read the plan set from top to bottom.
5. What artifacts define the system of record.

`README.md` must not become the place where live plan status is maintained.

## _index.json Responsibilities

`docs/plans/tech-plan-v1/_index.json` is the machine-readable index. It exists to support automation and quick lookup. Each entry should include:

1. `plan_id`
2. `title`
3. `status`
4. `depends_on`
5. `path`
6. `produces`

If `_index.json` and a plan file disagree, the plan file wins and `_index.json` must be corrected.

## Plan File Contract

Each executable plan file must have frontmatter with the following fields:

```yaml
plan_id: P07
title: Answer Worker Workspace
status: proposed
depends_on:
  - P06
consumes:
  - runs/<run_id>/01_prepare/...
produces:
  - runs/<run_id>/02_answer/personas/<persona_id>/workspace/AGENTS.md
  - runs/<run_id>/02_answer/personas/<persona_id>/outgoing_input.json
completion_evidence:
  - artifact_paths_exist
  - hashes_recorded
  - no_prompt_rerender
```

Each plan body must contain exactly these working sections:

1. `Goal`
2. `Scope`
3. `Out Of Scope`
4. `Required Inputs`
5. `Implementation Tasks`
6. `Acceptance Checks`
7. `Handoff`

The plan must be executable without reopening `docs/tech-plan-v1.md` for missing intent.

## Status Model

Allowed plan states:

1. `proposed`
2. `ready`
3. `in_progress`
4. `blocked`
5. `done`

State meanings:

1. `proposed` means the plan exists but cannot start yet because dependencies are not satisfied.
2. `ready` means all dependencies are complete and the plan can begin.
3. `in_progress` means this is the active plan currently being executed.
4. `blocked` means dependencies are satisfied but execution is waiting on an external decision or unavailable prerequisite.
5. `done` means the plan's acceptance checks and completion evidence are all satisfied.

Default scheduling rule:

1. Only one plan is `in_progress` at a time unless the execution rules explicitly permit safe parallel work.
2. A plan moves to `ready` automatically once all `depends_on` plans are `done`.
3. A plan cannot move to `done` without evidence tied to its outputs and acceptance checks.

## Drift Control

The source of truth order is:

1. Individual plan files
2. `_index.json`
3. `AGENTS.md`
4. `README.md`

Update rules:

1. Change plan content in the plan file first.
2. If plan metadata changes, update `_index.json` in the same change.
3. Update `AGENTS.md` only when the execution graph or dispatch rules change.
4. Update `README.md` only when the global explanation or navigation changes.
5. Never update plan status in `README.md` or `AGENTS.md` without updating the plan file.

Conflict rules:

1. If a plan file and `_index.json` conflict, the plan file is authoritative.
2. If `AGENTS.md` and plan `depends_on` conflict, stop scheduling new work until the conflict is resolved.
3. If `README.md` conflicts with the other artifacts, treat `README.md` as stale documentation rather than execution truth.

## Plan Set Acceptance

The plan set is acceptable only if all of the following are true:

1. All required files exist.
2. Every `depends_on` points to a real plan.
3. There are no dependency cycles.
4. The execution graph in `AGENTS.md` matches plan frontmatter.
5. Every plan contains all required sections.
6. Every plan clearly states its outputs and completion conditions.
7. A fresh Codex session can identify the next ready plan using `AGENTS.md`, open one target plan file, and start work without reopening `docs/tech-plan-v1.md` for missing intent.

## Handoff To Planning

This design does not itself create the final plan files. The next step is to translate this design into an implementation plan that writes:

1. The repository-root `AGENTS.md`
2. The `docs/plans/tech-plan-v1/` directory structure
3. The `_index.json` index
4. The 11 executable plan files

That planning step should preserve the plan boundaries and execution graph defined here.
