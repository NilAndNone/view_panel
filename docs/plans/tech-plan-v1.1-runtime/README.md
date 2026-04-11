# Tech Plan V1.1 Runtime Plan Set

## Why This Exists

This plan set is a runtime-only redesign of `tech-plan-v1`. It keeps the correct incident-driven architecture from v1, but re-baselines the plan graph around the real implementation blockers: executable entrypoint, canonical Stage 1 assembly, deterministic hard gating, Stage 2 seal-chain closure, and raw versus certified render outputs.

## Relationship To V1

1. `docs/plans/tech-plan-v1/` remains the reference version.
2. `docs/plans/tech-plan-v1.1-runtime/` is the new runtime execution plan set.
3. This directory does not cover persona/content-pack planning.

## Key Changes

1. Adds `P00 CLI + Config + Schema Validation`.
2. Rewrites the prepare chain into `P04` canonical assembly, `P05` hard gate, and `P06` non-blocking soft review.
3. Rewrites the Stage 2 seal chain in `P07`.
4. Moves `wv-answer-stage` ownership into `P08`.
5. Splits render outputs into raw and certified paths in `P10` and `P11`.

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

## Source Of Truth

1. Each plan file is authoritative for its own scope, dependencies, outputs, and acceptance.
2. `_index.json` mirrors plan metadata for machine lookup.
3. `AGENTS.md` carries the execution graph and dispatch rules.
4. This `README.md` explains the runtime plan set but does not own live status.

## Out Of Scope

This runtime plan set does not include persona pack schema, domain material packs, or persona distinctness/content acceptance work. Those belong to a separate content planning line.

