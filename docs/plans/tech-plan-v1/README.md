# Tech Plan V1 Executable Plan Set

## Why This Exists

`docs/tech-plan-v1.md` defines the target system, but it is too broad to execute safely as one unit. This directory decomposes that strategy into bounded plans that can be completed one at a time while preserving explicit `depends_on` execution order between plans.

## Artifacts

1. `AGENTS.md` is the Codex dispatcher.
2. `_index.json` is the machine-readable index.
3. `01-*.md` through `11-*.md` are the executable plan files, where the two-digit prefix maps to the plan ID (`P01` -> `01-*.md`, `P02` -> `02-*.md`, etc.).

## Plan List

1. `P01` Codex Runtime Contract
2. `P02` App-Server Client Lifecycle
3. `P03` Run Layout And Artifact Writer
4. `P04` Audit Log Pipeline
5. `P05` Prepare Input Builder
6. `P06` Prepare Review Gate
7. `P07` Answer Worker Workspace
8. `P08` Answer Single Worker Runner
9. `P09` Answer Batch Orchestration
10. `P10` Render Input Aggregation
11. `P11` Render Stage Output

## Execution Order

`P01` and `P03` are the only independent starting points. After that, the plans follow the dependency graph in `AGENTS.md` and the `depends_on` values in each plan file.

## Source Of Truth

1. Each plan file is authoritative for its own scope, dependencies, status, outputs, and acceptance.
2. `_index.json` mirrors plan metadata for machine lookup.
3. `AGENTS.md` carries the execution graph and dispatch rules.
4. This `README.md` explains the plan set but does not own live status.

## Acceptance Standard

The plan set is usable only if a fresh Codex session can identify the next ready plan from `AGENTS.md`, open one target plan file, and begin work without reopening `docs/tech-plan-v1.md` for missing intent.
