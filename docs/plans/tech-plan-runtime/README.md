# Tech Plan Runtime Plan Set

## Why This Exists

This is the repository's single retained runtime plan set. It keeps the incident-correct runtime architecture while presenting one canonical execution graph for product entry, runtime sealing, authoritative result handling, render splitting, and end-to-end smoke.

## What This Runtime Plan Solves

1. canonical Stage 1 input assembly and fixed `dispatch_input_v1` naming
2. deterministic prepare hard gating plus non-blocking soft review
3. Stage 2 seal-chain closure without ad hoc prompt recomposition
4. authoritative worker-result boundaries rooted in `item/completed.agentMessage`
5. raw versus certified render splitting as separate Stage 3 contracts
6. one stable full-flow smoke entrypoint owned by `P12`

## What This Runtime Plan Does Not Solve

1. persona pack schema design
2. persona distinctness or psychology authoring
3. domain-material curation
4. product-level content quality acceptance

Those belong to a separate parallel content-planning line.

## Historical Background

This retained runtime plan set absorbed the useful runtime constraints from earlier planning iterations.

The active technical source now lives entirely under `docs/plans/tech-plan-runtime/`.

Git history, not parallel planning documents, preserves version history.

## Plan List

1. `P00` CLI + Config + Schema Validation
2. `P01` Codex Runtime Contract
3. `P02` App-Server Client Lifecycle
4. `P03` Run Layout + Artifact Writer
5. `P04` Prepare Input Builder
6. `P05` Prepare Hard Gate
7. `P06` Prepare Soft Review
8. `P07` Answer Workspace Seal
9. `P08` Answer Single Worker Execution
10. `P09` Answer Batch Orchestration
11. `P10` Render Input Aggregation
12. `P11` Render Outputs
13. `P12` End-to-End Smoke + Makefile Flow

## Source Of Truth

1. Each plan file is authoritative for its own scope, dependencies, outputs, and acceptance.
2. `_index.json` mirrors plan metadata for machine lookup.
3. `AGENTS.md` carries the execution graph and dispatch rules.
4. This `README.md` explains the retained runtime plan set but does not own live status.
