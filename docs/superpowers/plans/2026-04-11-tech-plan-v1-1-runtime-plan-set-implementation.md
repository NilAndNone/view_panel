# Tech Plan V1.1 Runtime Plan Set Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the parallel `tech-plan-v1.1-runtime` executable runtime plan set, preserving the approved v1.1 runtime redesign while keeping the existing `tech-plan-v1` plan set as a reference version.

**Architecture:** This is a documentation-first planning task. The deliverable is a second runtime-only plan set with its own dispatcher, overview, machine index, and twelve numbered plan files. The new set re-baselines the runtime graph around the true implementation blockers: CLI/config entrypoint, canonical Stage 1 assembly, deterministic hard gating, Stage 2 seal-chain closure, and raw versus certified render outputs.

**Tech Stack:** Markdown, JSON, shell commands, repository documentation

---

## File Map

**Read First**
- `docs/tech-plan-v1.md`
- `docs/plans/tech-plan-v1/README.md`
- `docs/plans/tech-plan-v1/_index.json`
- `docs/superpowers/specs/2026-04-11-tech-plan-v1-1-runtime-redesign-design.md`

**Create**
- `docs/plans/tech-plan-v1.1-runtime/AGENTS.md`
- `docs/plans/tech-plan-v1.1-runtime/README.md`
- `docs/plans/tech-plan-v1.1-runtime/_index.json`
- `docs/plans/tech-plan-v1.1-runtime/00-cli-config-schema-validation.md`
- `docs/plans/tech-plan-v1.1-runtime/01-codex-runtime-contract.md`
- `docs/plans/tech-plan-v1.1-runtime/02-app-server-client-lifecycle.md`
- `docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md`
- `docs/plans/tech-plan-v1.1-runtime/04-prepare-input-builder.md`
- `docs/plans/tech-plan-v1.1-runtime/05-prepare-hard-gate.md`
- `docs/plans/tech-plan-v1.1-runtime/06-prepare-soft-review.md`
- `docs/plans/tech-plan-v1.1-runtime/07-answer-workspace-seal.md`
- `docs/plans/tech-plan-v1.1-runtime/08-answer-single-worker-execution.md`
- `docs/plans/tech-plan-v1.1-runtime/09-answer-batch-orchestration.md`
- `docs/plans/tech-plan-v1.1-runtime/10-render-input-aggregation.md`
- `docs/plans/tech-plan-v1.1-runtime/11-render-outputs.md`

**Validation Commands**
- `jq empty docs/plans/tech-plan-v1.1-runtime/_index.json`
- `rg '^plan_id:' docs/plans/tech-plan-v1.1-runtime/[0-9][0-9]-*.md`
- `rg '^status:' docs/plans/tech-plan-v1.1-runtime/[0-9][0-9]-*.md`
- `python` metadata consistency script in Task 16

### Task 1: Create The V1.1 Runtime Dispatcher

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/AGENTS.md`
- Create: `docs/plans/tech-plan-v1.1-runtime/`

- [ ] **Step 1: Create the new runtime plan-set directory**

```bash
mkdir -p docs/plans/tech-plan-v1.1-runtime
```

- [ ] **Step 2: Create `AGENTS.md` for the v1.1 runtime plan set**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/AGENTS.md
# Tech Plan V1.1 Runtime Execution Guide

## Purpose

Execute the runtime-only v1.1 plan set using the files in `docs/plans/tech-plan-v1.1-runtime/`.

## Plan Directory

`docs/plans/tech-plan-v1.1-runtime/`

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
P10 -> P11
```

## Dispatch Rules

1. Pick the lowest-numbered incomplete plan whose `depends_on` entries are all `done`.
2. Read only this `AGENTS.md`, `README.md`, the target plan file, and directly required dependency plans.
3. Do not start implementation from `README.md` alone.
4. Do not merge multiple plans into one execution unless the target plan says so.
5. After finishing a plan, update both the target plan frontmatter and `docs/plans/tech-plan-v1.1-runtime/_index.json`.

## Source Of Truth Order

1. The target plan file
2. `docs/plans/tech-plan-v1.1-runtime/_index.json`
3. `docs/plans/tech-plan-v1.1-runtime/AGENTS.md`
4. `docs/plans/tech-plan-v1.1-runtime/README.md`
EOF
```

- [ ] **Step 3: Verify the dispatcher shows the approved graph**

Run: `sed -n '1,120p' docs/plans/tech-plan-v1.1-runtime/AGENTS.md`

Expected: the file shows all approved v1.1 runtime edges and the five dispatch rules exactly once.

- [ ] **Step 4: Commit the dispatcher**

```bash
git add docs/plans/tech-plan-v1.1-runtime/AGENTS.md
git commit -m "docs: add v1.1 runtime dispatcher"
```

### Task 2: Create The Human Overview

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/README.md`

- [ ] **Step 1: Create the v1.1 runtime README**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/README.md
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
EOF
```

- [ ] **Step 2: Verify the README distinguishes v1 from v1.1**

Run: `rg 'Relationship To V1|Key Changes|Out Of Scope' docs/plans/tech-plan-v1.1-runtime/README.md -n`

Expected: all three sections are present.

- [ ] **Step 3: Commit the human overview**

```bash
git add docs/plans/tech-plan-v1.1-runtime/README.md
git commit -m "docs: add v1.1 runtime overview"
```

### Task 3: Create The Machine Index

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/_index.json`

- [ ] **Step 1: Create the initial v1.1 runtime index**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/_index.json
{
  "plan_set_id": "tech-plan-v1.1-runtime",
  "source": "docs/superpowers/specs/2026-04-11-tech-plan-v1-1-runtime-redesign-design.md",
  "last_updated": "2026-04-11",
  "plans": [
    {
      "plan_id": "P00",
      "title": "CLI + Config + Schema Validation",
      "status": "ready",
      "depends_on": [],
      "path": "docs/plans/tech-plan-v1.1-runtime/00-cli-config-schema-validation.md",
      "produces": [
        "cmd/worldview-panel/main.go",
        "internal/config/config.go",
        "internal/schema/validate.go"
      ]
    },
    {
      "plan_id": "P01",
      "title": "Codex Runtime Contract",
      "status": "ready",
      "depends_on": [],
      "path": "docs/plans/tech-plan-v1.1-runtime/01-codex-runtime-contract.md",
      "produces": [
        "docs/operations/codex-runtime-contract.md",
        "third_party/codex-protocol/"
      ]
    },
    {
      "plan_id": "P02",
      "title": "App-Server Client Lifecycle",
      "status": "proposed",
      "depends_on": ["P00", "P01"],
      "path": "docs/plans/tech-plan-v1.1-runtime/02-app-server-client-lifecycle.md",
      "produces": [
        "internal/appserver/client.go",
        "internal/appserver/messages.go",
        "internal/appserver/runner.go"
      ]
    },
    {
      "plan_id": "P03",
      "title": "Run Layout + Artifact Writer",
      "status": "ready",
      "depends_on": [],
      "path": "docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md",
      "produces": [
        "internal/storage/layout.go",
        "internal/hash/hash.go"
      ]
    },
    {
      "plan_id": "P04",
      "title": "Prepare Input Builder",
      "status": "proposed",
      "depends_on": ["P03"],
      "path": "docs/plans/tech-plan-v1.1-runtime/04-prepare-input-builder.md",
      "produces": [
        "internal/materials/loader.go",
        "internal/persona/library.go",
        "internal/stage/prepare/prepare.go",
        "schemas/dispatch_input_v1.json"
      ]
    },
    {
      "plan_id": "P05",
      "title": "Prepare Hard Gate",
      "status": "proposed",
      "depends_on": ["P03", "P04"],
      "path": "docs/plans/tech-plan-v1.1-runtime/05-prepare-hard-gate.md",
      "produces": [
        "internal/stage/prepare/gate.go",
        "schemas/prepare_gate_status_v1.json"
      ]
    },
    {
      "plan_id": "P06",
      "title": "Prepare Soft Review",
      "status": "proposed",
      "depends_on": ["P04"],
      "path": "docs/plans/tech-plan-v1.1-runtime/06-prepare-soft-review.md",
      "produces": [
        "runtime/skills/wv-prepare-stage/SKILL.md",
        "schemas/prepare_review_v1.json"
      ]
    },
    {
      "plan_id": "P07",
      "title": "Answer Workspace Seal",
      "status": "proposed",
      "depends_on": ["P03", "P05"],
      "path": "docs/plans/tech-plan-v1.1-runtime/07-answer-workspace-seal.md",
      "produces": [
        "internal/stage/answer/workspace.go",
        "runs/<run_id>/02_answer/personas/<persona_id>/workspace/AGENTS.md",
        "runs/<run_id>/02_answer/personas/<persona_id>/input/prompt.txt",
        "runs/<run_id>/02_answer/personas/<persona_id>/outgoing_input.json"
      ]
    },
    {
      "plan_id": "P08",
      "title": "Answer Single Worker Execution",
      "status": "proposed",
      "depends_on": ["P01", "P02", "P07"],
      "path": "docs/plans/tech-plan-v1.1-runtime/08-answer-single-worker-execution.md",
      "produces": [
        "runtime/skills/wv-answer-stage/SKILL.md",
        "internal/stage/answer/runner.go",
        "schemas/worldview_worker_result_v1.json"
      ]
    },
    {
      "plan_id": "P09",
      "title": "Answer Batch Orchestration",
      "status": "proposed",
      "depends_on": ["P08"],
      "path": "docs/plans/tech-plan-v1.1-runtime/09-answer-batch-orchestration.md",
      "produces": [
        "internal/stage/answer/batch.go",
        "internal/orchestrator/run.go"
      ]
    },
    {
      "plan_id": "P10",
      "title": "Render Input Aggregation",
      "status": "proposed",
      "depends_on": ["P03", "P09"],
      "path": "docs/plans/tech-plan-v1.1-runtime/10-render-input-aggregation.md",
      "produces": [
        "internal/stage/render/aggregate.go",
        "runs/<run_id>/03_render/raw_render_input.json",
        "runs/<run_id>/03_render/certified_render_input.json"
      ]
    },
    {
      "plan_id": "P11",
      "title": "Render Outputs",
      "status": "proposed",
      "depends_on": ["P10"],
      "path": "docs/plans/tech-plan-v1.1-runtime/11-render-outputs.md",
      "produces": [
        "internal/stage/render/render.go",
        "runtime/skills/wv-render-stage/SKILL.md",
        "schemas/panel_result_v1.json"
      ]
    }
  ]
}
EOF
```

- [ ] **Step 2: Verify the new index parses**

Run: `jq empty docs/plans/tech-plan-v1.1-runtime/_index.json`

Expected: no output and exit status `0`.

- [ ] **Step 3: Commit the machine index**

```bash
git add docs/plans/tech-plan-v1.1-runtime/_index.json
git commit -m "docs: add v1.1 runtime machine index"
```

### Task 4: Write Plan P00

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/00-cli-config-schema-validation.md`

- [ ] **Step 1: Create the P00 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/00-cli-config-schema-validation.md
---
plan_id: P00
title: CLI + Config + Schema Validation
status: ready
depends_on: []
consumes:
  - docs/superpowers/specs/2026-04-11-tech-plan-v1-1-runtime-redesign-design.md
  - docs/tech-plan-v1.md
produces:
  - cmd/worldview-panel/main.go
  - internal/config/config.go
  - internal/schema/validate.go
completion_evidence:
  - cli_entrypoint_defined
  - runtime_config_defaults_defined
  - schema_validation_entrypoints_defined
---

# Goal

Create the executable runtime entrypoint so the worldview panel can be invoked as a product, not just as a collection of internal modules.

# Scope

- Define `cmd/worldview-panel/main.go`.
- Define runtime config loading in `internal/config/config.go`.
- Define shared schema validation entrypoints in `internal/schema/validate.go`.
- Fix default handling for `question`, `materials`, `persona-set`, `outdir`, `concurrency`, and `model`.

# Out Of Scope

- App-server process lifecycle internals.
- Stage-specific packet assembly.
- Render-stage aggregation logic.

# Required Inputs

- `docs/superpowers/specs/2026-04-11-tech-plan-v1-1-runtime-redesign-design.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

- Add the CLI entrypoint in `cmd/worldview-panel/main.go`.
- Add runtime config loading and default resolution in `internal/config/config.go`.
- Add shared schema validation helpers in `internal/schema/validate.go`.
- Add tests for config parsing, required-flag handling, and validator entrypoint behavior.

# Acceptance Checks

- The runtime has a documented CLI entrypoint binary.
- Config loading defines defaults for all required runtime parameters.
- Shared schema validation is callable from later stage plans.

# Handoff

P02, P04, P05, and later stage plans depend on this entrypoint and validation surface.
EOF
```

- [ ] **Step 2: Verify P00 is the new zero-dependency runtime root**

Run: `sed -n '1,40p' docs/plans/tech-plan-v1.1-runtime/00-cli-config-schema-validation.md`

Expected: `status: ready` and `depends_on: []`.

- [ ] **Step 3: Commit P00**

```bash
git add docs/plans/tech-plan-v1.1-runtime/00-cli-config-schema-validation.md
git commit -m "docs: add v1.1 runtime plan p00"
```

### Task 5: Write Plan P01

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/01-codex-runtime-contract.md`

- [ ] **Step 1: Create the P01 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/01-codex-runtime-contract.md
---
plan_id: P01
title: Codex Runtime Contract
status: ready
depends_on: []
consumes:
  - docs/tech-plan-v1.md
  - docs/superpowers/specs/2026-04-11-tech-plan-v1-1-runtime-redesign-design.md
produces:
  - docs/operations/codex-runtime-contract.md
  - third_party/codex-protocol/
completion_evidence:
  - codex_cli_version_pinned
  - app_server_transport_fixed_to_stdio
  - schema_generation_location_defined
---

# Goal

Define the supported Codex runtime contract that all v1.1 runtime plans build against.

# Scope

- Pin one supported `@openai/codex` version.
- Lock `codex app-server --listen stdio://` as the only transport for v1.1.
- Define the protocol schema output location.
- Record install, login, schema generation, and runtime smoke commands.

# Out Of Scope

- CLI/config entrypoint behavior.
- Stage-specific runner logic.
- Render-stage behavior.

# Required Inputs

- `docs/tech-plan-v1.md`
- `docs/superpowers/specs/2026-04-11-tech-plan-v1-1-runtime-redesign-design.md`

# Implementation Tasks

- Create `docs/operations/codex-runtime-contract.md`.
- Record the pinned CLI version and why it must stay locked.
- Record `stdio://` as the only supported app-server transport.
- Record the schema generation path and smoke command.

# Acceptance Checks

- Exactly one supported CLI version is named.
- `stdio://` is the only supported app-server transport.
- The protocol schema output directory is fixed.
- Install, login, schema generation, and smoke commands are all recorded.

# Handoff

P02 and P08 consume this runtime contract.
EOF
```

- [ ] **Step 2: Verify the plan keeps runtime contract scope separate from P00**

Run: `rg 'CLI/config entrypoint|stdio://|schema generation' docs/plans/tech-plan-v1.1-runtime/01-codex-runtime-contract.md -n`

Expected: runtime contract content is present and CLI/config is explicitly out of scope.

- [ ] **Step 3: Commit P01**

```bash
git add docs/plans/tech-plan-v1.1-runtime/01-codex-runtime-contract.md
git commit -m "docs: add v1.1 runtime plan p01"
```

### Task 6: Write Plan P02

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/02-app-server-client-lifecycle.md`

- [ ] **Step 1: Create the P02 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/02-app-server-client-lifecycle.md
---
plan_id: P02
title: App-Server Client Lifecycle
status: proposed
depends_on:
  - P00
  - P01
consumes:
  - docs/plans/tech-plan-v1.1-runtime/00-cli-config-schema-validation.md
  - docs/plans/tech-plan-v1.1-runtime/01-codex-runtime-contract.md
produces:
  - internal/appserver/client.go
  - internal/appserver/messages.go
  - internal/appserver/runner.go
completion_evidence:
  - initialize_initialized_flow_implemented
  - requirements_read_supported
  - single_turn_lifecycle_wrapped
---

# Goal

Implement the thin Go app-server client that manages one process, one thread, and one turn lifecycle over JSON-RPC on stdio.

# Scope

- Define protocol message structs for requests, responses, and notifications.
- Spawn `codex app-server --listen stdio://`.
- Implement `initialize`, `initialized`, `configRequirements/read`, `thread/start`, `turn/start`, `turn/interrupt`, and clean shutdown.
- Expose a `RunSingleTurn` helper for later runtime stages.

# Out Of Scope

- Stage-specific input assembly.
- Audit-log persistence.
- Batch orchestration across personas.

# Required Inputs

- `docs/plans/tech-plan-v1.1-runtime/00-cli-config-schema-validation.md`
- `docs/plans/tech-plan-v1.1-runtime/01-codex-runtime-contract.md`

# Implementation Tasks

- Add protocol message types in `internal/appserver/messages.go`.
- Add the process-backed client in `internal/appserver/client.go`.
- Add a `RunSingleTurn` lifecycle helper in `internal/appserver/runner.go`.
- Add tests or transcript checks for handshake ordering, final completion parsing, and interrupt-first shutdown.

# Acceptance Checks

- The client completes the full handshake sequence in order.
- `configRequirements/read` happens before `thread/start`.
- `item/completed.agentMessage` is the only authoritative final output source.
- Normal completion shuts down cleanly, while timeout/error paths attempt interrupt before process kill.

# Handoff

P08 consumes this client lifecycle instead of inventing a new protocol loop.
EOF
```

- [ ] **Step 2: Verify P02 depends on both P00 and P01**

Run: `rg '^  - P0[01]$' docs/plans/tech-plan-v1.1-runtime/02-app-server-client-lifecycle.md -n`

Expected: one match for `P00` and one for `P01`.

- [ ] **Step 3: Commit P02**

```bash
git add docs/plans/tech-plan-v1.1-runtime/02-app-server-client-lifecycle.md
git commit -m "docs: add v1.1 runtime plan p02"
```

### Task 7: Write Plan P03

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md`

- [ ] **Step 1: Create the P03 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md
---
plan_id: P03
title: Run Layout + Artifact Writer
status: ready
depends_on: []
consumes:
  - docs/tech-plan-v1.md
  - docs/superpowers/specs/2026-04-11-tech-plan-v1-1-runtime-redesign-design.md
produces:
  - internal/storage/layout.go
  - internal/hash/hash.go
completion_evidence:
  - run_directory_layout_fixed
  - shared_writer_helpers_defined
  - sha256_primitives_available
---

# Goal

Define the stable run-root layout, shared artifact-writing helpers, and generic SHA-256 primitives used by all runtime stages.

# Scope

- Define canonical paths for request, prepare, answer, render, and audit areas under `run_root`.
- Define deterministic JSON and text artifact writes with overwrite behavior.
- Define append-oriented JSONL writes for protocol and event streams.
- Define byte-stable and file-stable SHA-256 primitives.

# Out Of Scope

- Stage-specific hash bundle composition.
- App-server process lifecycle behavior.
- Persona/content-pack planning.

# Required Inputs

- `docs/tech-plan-v1.md`
- `docs/superpowers/specs/2026-04-11-tech-plan-v1-1-runtime-redesign-design.md`

# Implementation Tasks

- Add canonical path constructors in `internal/storage/layout.go`.
- Add shared write helpers for JSON, text, and JSONL artifacts.
- Add generic SHA-256 helpers in `internal/hash/hash.go`.
- Add focused tests for stable paths, overwrite/append semantics, and hash stability.

# Acceptance Checks

- All named runtime areas under `run_root` have stable path constructors.
- JSON/text writes are deterministic canonical writes.
- JSONL writes are append-oriented event streams.
- Hash helpers expose only generic primitives; stage-specific hash composition remains in consuming plans.

# Handoff

P04, P05, P07, P10, and P11 consume this path/writer/hash foundation.
EOF
```

- [ ] **Step 2: Verify P03 keeps stage-specific hash composition out of scope**

Run: `rg 'stage-specific hash|generic primitives|append-oriented' docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md -n`

Expected: those boundary terms are present.

- [ ] **Step 3: Commit P03**

```bash
git add docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md
git commit -m "docs: add v1.1 runtime plan p03"
```

### Task 8: Write Plan P04

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/04-prepare-input-builder.md`

- [ ] **Step 1: Create the P04 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/04-prepare-input-builder.md
---
plan_id: P04
title: Prepare Input Builder
status: proposed
depends_on:
  - P03
consumes:
  - docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md
  - docs/tech-plan-v1.md
produces:
  - internal/materials/loader.go
  - internal/persona/library.go
  - internal/stage/prepare/prepare.go
  - schemas/dispatch_input_v1.json
completion_evidence:
  - authoritative_dispatch_fields_fixed
  - prepare_artifacts_prerendered
  - manifest_and_hashes_written
---

# Goal

Assemble the only canonical Stage 1 input packet and prerender the exact worker-sendable artifacts before any Stage 2 execution can begin.

# Scope

- Use only these authoritative `dispatch_input_v1` field names:
  - `roleplay_prompt`
  - `discussion_question`
  - `supplementary_materials`
  - `output_contract`
  - `assumptions_and_constraints`
- Build one canonical `dispatch_input_v1.json` per persona.
- Prerender `agents.md` and `prompt.txt` per persona.
- Write `manifest.json` and `hashes.json` per persona.

# Out Of Scope

- Blocking prepare-gate logic.
- Model review of prepared inputs.
- Stage 2 execution.

# Required Inputs

- `docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

- Add materials loading in `internal/materials/loader.go`.
- Add persona loading in `internal/persona/library.go`.
- Add canonical Stage 1 assembly in `internal/stage/prepare/prepare.go`.
- Add `schemas/dispatch_input_v1.json` using only the five authoritative field names.
- Materialize per-persona outputs under:
  - `01_prepare/personas/<id>/dispatch_input_v1.json`
  - `01_prepare/personas/<id>/agents.md`
  - `01_prepare/personas/<id>/prompt.txt`
  - `01_prepare/personas/<id>/manifest.json`
  - `01_prepare/personas/<id>/hashes.json`

# Acceptance Checks

- No alias field names are introduced for `dispatch_input_v1`.
- Stage 2 no longer needs to assemble worker inputs from raw structured fields.
- `hashes.json` contains `dispatch_input_sha256`, `agents_sha256`, `prompt_sha256`, and `bundle_sha256`.
- `bundle_sha256` is derived from canonical content, not filesystem paths.

# Handoff

P05 performs the hard deterministic gate on these artifacts. P06 may review them. P07 seals them into the worker-side execution inputs.
EOF
```

- [ ] **Step 2: Verify the five authoritative field names appear exactly as required**

Run: `rg 'roleplay_prompt|discussion_question|supplementary_materials|output_contract|assumptions_and_constraints' docs/plans/tech-plan-v1.1-runtime/04-prepare-input-builder.md -n`

Expected: all five field names appear.

- [ ] **Step 3: Commit P04**

```bash
git add docs/plans/tech-plan-v1.1-runtime/04-prepare-input-builder.md
git commit -m "docs: add v1.1 runtime plan p04"
```

### Task 9: Write Plan P05

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/05-prepare-hard-gate.md`

- [ ] **Step 1: Create the P05 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/05-prepare-hard-gate.md
---
plan_id: P05
title: Prepare Hard Gate
status: proposed
depends_on:
  - P03
  - P04
consumes:
  - docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md
  - docs/plans/tech-plan-v1.1-runtime/04-prepare-input-builder.md
produces:
  - internal/stage/prepare/gate.go
  - schemas/prepare_gate_status_v1.json
completion_evidence:
  - deterministic_prepare_gate_defined
  - missing_artifacts_block_stage2
  - prepare_status_written
---

# Goal

Block Stage 2 until the deterministic runtime integrity checks over Stage 1 artifacts pass.

# Scope

- Validate the five required `dispatch_input_v1` fields are present.
- Validate canonical prepare artifacts exist for each persona.
- Validate required hash artifacts exist and are complete.
- Validate prepare directories are complete.
- Validate required audit/log paths are writable.
- Persist a run-level prepare hard-gate status.

# Out Of Scope

- Model review of persona quality.
- Persona differentiation judgments.
- Stage 2 workspace sealing.

# Required Inputs

- `docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md`
- `docs/plans/tech-plan-v1.1-runtime/04-prepare-input-builder.md`

# Implementation Tasks

- Add deterministic prepare-gate logic in `internal/stage/prepare/gate.go`.
- Add a machine-readable gate status schema in `schemas/prepare_gate_status_v1.json`.
- Fail the runtime before Stage 2 when any required prepare artifact or required writable path is missing.
- Record the run-level gate decision under the prepare area.

# Acceptance Checks

- This is the only mandatory prepare-stage blocking gate.
- Missing required fields, artifacts, hashes, or required writable paths block Stage 2.
- The gate decision is recorded as a run-level prepare status artifact.

# Handoff

P07 may start only when this hard gate marks the prepared run ready for Stage 2 sealing.
EOF
```

- [ ] **Step 2: Verify the plan remains purely deterministic**

Run: `rg 'only mandatory|deterministic|Model review' docs/plans/tech-plan-v1.1-runtime/05-prepare-hard-gate.md -n`

Expected: the file clearly states deterministic blocking behavior and excludes model review.

- [ ] **Step 3: Commit P05**

```bash
git add docs/plans/tech-plan-v1.1-runtime/05-prepare-hard-gate.md
git commit -m "docs: add v1.1 runtime plan p05"
```

### Task 10: Write Plan P06

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/06-prepare-soft-review.md`

- [ ] **Step 1: Create the P06 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/06-prepare-soft-review.md
---
plan_id: P06
title: Prepare Soft Review
status: proposed
depends_on:
  - P04
consumes:
  - docs/plans/tech-plan-v1.1-runtime/04-prepare-input-builder.md
produces:
  - runtime/skills/wv-prepare-stage/SKILL.md
  - schemas/prepare_review_v1.json
completion_evidence:
  - advisory_prepare_review_defined
  - blocking_behavior_removed_by_default
  - review_report_written
---

# Goal

Provide a non-blocking advisory review branch over prepared inputs without entangling model review with the runtime hard gate.

# Scope

- Add `runtime/skills/wv-prepare-stage/SKILL.md`.
- Add `schemas/prepare_review_v1.json`.
- Run one advisory review session over the prepared persona set.
- Write a review report and warning summary.

# Out Of Scope

- Blocking Stage 2 by default.
- Mutating canonical Stage 1 artifacts.
- Replacing the deterministic prepare hard gate.

# Required Inputs

- `docs/plans/tech-plan-v1.1-runtime/04-prepare-input-builder.md`

# Implementation Tasks

- Add the advisory prepare-review skill.
- Add the machine-readable review schema.
- Run a review session against the prepared persona set after canonical artifacts exist.
- Record review output as advisory artifacts only.

# Acceptance Checks

- The review branch is non-blocking by default.
- The review session cannot mutate canonical Stage 1 artifacts.
- Review output is recorded separately from the hard gate status.

# Handoff

P07 does not depend on this plan by default, but later operators may use its review outputs to inspect weak persona differentiation or suspicious assembly.
EOF
```

- [ ] **Step 2: Verify the plan explicitly says it is non-blocking**

Run: `rg 'non-blocking by default|does not depend on this plan by default' docs/plans/tech-plan-v1.1-runtime/06-prepare-soft-review.md -n`

Expected: both ideas are present.

- [ ] **Step 3: Commit P06**

```bash
git add docs/plans/tech-plan-v1.1-runtime/06-prepare-soft-review.md
git commit -m "docs: add v1.1 runtime plan p06"
```

### Task 11: Write Plan P07

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/07-answer-workspace-seal.md`

- [ ] **Step 1: Create the P07 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/07-answer-workspace-seal.md
---
plan_id: P07
title: Answer Workspace Seal
status: proposed
depends_on:
  - P03
  - P05
consumes:
  - docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md
  - docs/plans/tech-plan-v1.1-runtime/05-prepare-hard-gate.md
produces:
  - internal/stage/answer/workspace.go
  - runs/<run_id>/02_answer/personas/<persona_id>/workspace/AGENTS.md
  - runs/<run_id>/02_answer/personas/<persona_id>/input/prompt.txt
  - runs/<run_id>/02_answer/personas/<persona_id>/outgoing_input.json
completion_evidence:
  - worker_side_prompt_snapshot_written
  - seal_chain_hashes_written
  - workspace_contamination_blocked
---

# Goal

Seal the exact Stage 2 worker-side inputs so the actual execution input chain is auditable and cannot drift from the canonical prepared artifacts.

# Scope

- Create isolated worker execution directories outside the repo tree.
- Copy prepared `agents.md` into `workspace/AGENTS.md`.
- Copy a worker-side prompt snapshot into `input/prompt.txt`.
- Write `outgoing_input.json` with the worker-side input contract and hash chain.
- Use controlled worker `cwd` and controlled `HOME`.

# Out Of Scope

- Running the worker turn itself.
- Batch orchestration.
- Persona/content-pack planning.

# Required Inputs

- `docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md`
- `docs/plans/tech-plan-v1.1-runtime/05-prepare-hard-gate.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

- Add `internal/stage/answer/workspace.go`.
- Define workspace creation outside the repo tree.
- Copy `01_prepare/.../agents.md` into `02_answer/.../workspace/AGENTS.md`.
- Copy a worker-side prompt snapshot into `02_answer/.../input/prompt.txt`.
- Write `outgoing_input.json` with at least:
  - `persona_id`
  - `agent_instructions_path`
  - `agent_instructions_sha256`
  - `prompt_path`
  - `prompt_sha256`
  - `skill_path`
  - `skill_sha256`
  - `source_dispatch_input_sha256`
  - `combined_input_sha256`

# Acceptance Checks

- Stage 2 can read only:
  - `workspace/AGENTS.md`
  - `input/prompt.txt`
  - `outgoing_input.json`
  - the answer-stage skill
- Stage 2 cannot read `dispatch_input_v1.json` directly.
- Worker execution uses an isolated workspace, controlled `cwd`, and controlled `HOME`.
- Repo-root and user-home `AGENTS.md` contamination is blocked by contract.

# Handoff

P08 executes one worker only through this sealed input chain.
EOF
```

- [ ] **Step 2: Verify the seal-chain fields are all present**

Run: `rg 'skill_sha256|source_dispatch_input_sha256|combined_input_sha256|input/prompt.txt' docs/plans/tech-plan-v1.1-runtime/07-answer-workspace-seal.md -n`

Expected: all four terms appear.

- [ ] **Step 3: Commit P07**

```bash
git add docs/plans/tech-plan-v1.1-runtime/07-answer-workspace-seal.md
git commit -m "docs: add v1.1 runtime plan p07"
```

### Task 12: Write Plan P08

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/08-answer-single-worker-execution.md`

- [ ] **Step 1: Create the P08 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/08-answer-single-worker-execution.md
---
plan_id: P08
title: Answer Single Worker Execution
status: proposed
depends_on:
  - P01
  - P02
  - P07
consumes:
  - docs/plans/tech-plan-v1.1-runtime/01-codex-runtime-contract.md
  - docs/plans/tech-plan-v1.1-runtime/02-app-server-client-lifecycle.md
  - docs/plans/tech-plan-v1.1-runtime/07-answer-workspace-seal.md
produces:
  - runtime/skills/wv-answer-stage/SKILL.md
  - internal/stage/answer/runner.go
  - schemas/worldview_worker_result_v1.json
completion_evidence:
  - answer_stage_skill_owned_here
  - final_completed_message_authoritative
  - worker_result_artifacts_written
---

# Goal

Run one sealed Stage 2 worker from start to finish and write the canonical worker result artifacts.

# Scope

- Add `runtime/skills/wv-answer-stage/SKILL.md`.
- Add the worker result schema.
- Execute one worker turn using only sealed Stage 2 inputs.
- Treat `item/completed.agentMessage` as the only authoritative final model output.
- Write raw, structured, attestation, and status artifacts.

# Out Of Scope

- Persona fan-out across the batch.
- Render-stage aggregation.
- Persona/content-pack quality planning.

# Required Inputs

- `docs/plans/tech-plan-v1.1-runtime/01-codex-runtime-contract.md`
- `docs/plans/tech-plan-v1.1-runtime/02-app-server-client-lifecycle.md`
- `docs/plans/tech-plan-v1.1-runtime/07-answer-workspace-seal.md`

# Implementation Tasks

- Add `runtime/skills/wv-answer-stage/SKILL.md`.
- Add `schemas/worldview_worker_result_v1.json`.
- Add `internal/stage/answer/runner.go`.
- Execute one worker using only `workspace/AGENTS.md`, `input/prompt.txt`, `outgoing_input.json`, and the answer-stage skill.
- Write:
  - `result.raw.txt`
  - `result.json`
  - `attestation.json`
  - `status.json`

# Acceptance Checks

- `wv-answer-stage` is owned by this plan.
- `item/completed.agentMessage` is the only authoritative final model output.
- `result.json` is the canonical structured success artifact.
- `result.raw.txt` is diagnostic only.
- `status.json` records success or rejection/failure reason.
- `attestation.json` records validation and certification outcome.

# Handoff

P09 fans this sealed single-worker execution pattern out across the persona set.
EOF
```

- [ ] **Step 2: Verify the answer-stage skill is owned here**

Run: `rg 'wv-answer-stage|result.raw.txt|result.json|attestation.json|status.json' docs/plans/tech-plan-v1.1-runtime/08-answer-single-worker-execution.md -n`

Expected: all items appear.

- [ ] **Step 3: Commit P08**

```bash
git add docs/plans/tech-plan-v1.1-runtime/08-answer-single-worker-execution.md
git commit -m "docs: add v1.1 runtime plan p08"
```

### Task 13: Write Plan P09

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/09-answer-batch-orchestration.md`

- [ ] **Step 1: Create the P09 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/09-answer-batch-orchestration.md
---
plan_id: P09
title: Answer Batch Orchestration
status: proposed
depends_on:
  - P08
consumes:
  - docs/plans/tech-plan-v1.1-runtime/08-answer-single-worker-execution.md
  - docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md
produces:
  - internal/stage/answer/batch.go
  - internal/orchestrator/run.go
completion_evidence:
  - persona_fanout_supported
  - forbidden_tool_failure_defined
  - certified_failed_rejected_model_defined
---

# Goal

Execute the Stage 2 persona batch with concurrency control, failure handling, and machine-readable certification outcomes.

# Scope

- Fan out the single-worker execution pattern across the persona set.
- Add concurrency limits and timeout handling.
- Interrupt and terminate workers that overrun.
- Mark any forbidden tool item as a `failed` Stage 2 outcome.
- Derive mutually exclusive `certified`, `failed`, and `rejected` persona outcomes from worker artifacts.

# Out Of Scope

- Stage 1 packet assembly.
- Render input aggregation.
- UI presentation of worker outputs.

# Required Inputs

- `docs/plans/tech-plan-v1.1-runtime/08-answer-single-worker-execution.md`
- `docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

- Add `internal/stage/answer/batch.go`.
- Extend `internal/orchestrator/run.go` to drive the full Stage 2 fan-out.
- Add forbidden-tool detection for command, file, MCP, dynamic tool, collaboration, web, and image items.
- Derive per-persona outcomes from P08 worker artifacts under `02_answer`.
- Record the certified subset for later render filtering through existing per-persona status/result artifacts.

# Acceptance Checks

- Batch orchestration runs all personas with a configurable concurrency limit.
- Timeout handling attempts interrupt before process kill.
- Forbidden tool items map canonically to `failed`.
- `certified`, `failed`, and `rejected` are mutually exclusive Stage 2 outcomes.

# Handoff

P10 reads only current-run Stage 2 artifacts and filters the certified subset from them.
EOF
```

- [ ] **Step 2: Commit P09**

```bash
git add docs/plans/tech-plan-v1.1-runtime/09-answer-batch-orchestration.md
git commit -m "docs: add v1.1 runtime plan p09"
```

### Task 14: Write Plan P10

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/10-render-input-aggregation.md`

- [ ] **Step 1: Create the P10 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/10-render-input-aggregation.md
---
plan_id: P10
title: Render Input Aggregation
status: proposed
depends_on:
  - P03
  - P09
consumes:
  - docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md
  - docs/plans/tech-plan-v1.1-runtime/09-answer-batch-orchestration.md
produces:
  - internal/stage/render/aggregate.go
  - runs/<run_id>/03_render/raw_render_input.json
  - runs/<run_id>/03_render/certified_render_input.json
completion_evidence:
  - raw_and_certified_inputs_split
  - minimum_success_rate_applies_only_to_certified_path
  - zero_denominator_guard_defined
---

# Goal

Build raw and certified render inputs from current-run Stage 2 artifacts while keeping certification gating out of the raw-display path.

# Scope

- Read only current-run Stage 2 per-persona artifacts under `02_answer`.
- Build `raw_render_input.json` from displayable current-run outputs.
- Build `certified_render_input.json` from the certified subset only.
- Apply `min_success_rate` only to the certified path.

# Out Of Scope

- Producing final render outputs.
- Rewriting Stage 2 answers.
- Guessing missing or stale worker results.

# Required Inputs

- `docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md`
- `docs/plans/tech-plan-v1.1-runtime/09-answer-batch-orchestration.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

- Add `internal/stage/render/aggregate.go`.
- Filter current-run Stage 2 artifacts into raw-displayable and certified subsets.
- Write `raw_render_input.json`.
- Write `certified_render_input.json`.
- Apply the certified gate rule:
  - fail immediately if `T = 0`
  - pass only if `T > 0`, `C > 0`, and `C / T >= min_success_rate`
  - use default `min_success_rate = 0.8`

# Acceptance Checks

- Raw render input can be produced without certified-threshold success when displayable output exists.
- Certified render input contains only certified current-run outputs.
- The certified gate uses explicit denominator and threshold rules.
- Missing personas are never backfilled or invented.

# Handoff

P11 renders raw outputs from `raw_render_input.json` and certified outputs from `certified_render_input.json`.
EOF
```

- [ ] **Step 2: Verify the plan encodes the `T = 0` guard**

Run: `rg 'T = 0|C / T >= min_success_rate|raw_render_input.json|certified_render_input.json' docs/plans/tech-plan-v1.1-runtime/10-render-input-aggregation.md -n`

Expected: all four terms appear.

- [ ] **Step 3: Commit P10**

```bash
git add docs/plans/tech-plan-v1.1-runtime/10-render-input-aggregation.md
git commit -m "docs: add v1.1 runtime plan p10"
```

### Task 15: Write Plan P11

**Files:**
- Create: `docs/plans/tech-plan-v1.1-runtime/11-render-outputs.md`

- [ ] **Step 1: Create the P11 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1.1-runtime/11-render-outputs.md
---
plan_id: P11
title: Render Outputs
status: proposed
depends_on:
  - P10
consumes:
  - docs/plans/tech-plan-v1.1-runtime/10-render-input-aggregation.md
  - docs/plans/tech-plan-v1.1-runtime/02-app-server-client-lifecycle.md
produces:
  - internal/stage/render/render.go
  - runtime/skills/wv-render-stage/SKILL.md
  - schemas/panel_result_v1.json
completion_evidence:
  - raw_outputs_written
  - certified_outputs_conditionally_written
  - render_status_written
---

# Goal

Run the render stage over raw and certified render inputs and write the final display outputs under the render output root.

# Scope

- Add `runtime/skills/wv-render-stage/SKILL.md`.
- Add `schemas/panel_result_v1.json`.
- Render raw outputs from `raw_render_input.json`.
- Render certified outputs from `certified_render_input.json` only when the certified gate has passed.
- Write terminal render status under the render output root.

# Out Of Scope

- Modifying Stage 2 worker artifacts.
- Filling in missing worker responses.
- Reassembling Stage 1 inputs.

# Required Inputs

- `docs/plans/tech-plan-v1.1-runtime/10-render-input-aggregation.md`
- `docs/plans/tech-plan-v1.1-runtime/02-app-server-client-lifecycle.md`

# Implementation Tasks

- Add `runtime/skills/wv-render-stage/SKILL.md`.
- Add `schemas/panel_result_v1.json`.
- Add `internal/stage/render/render.go`.
- Write under `03_render/` only:
  - `raw_panel.json`
  - `raw_panel.md`
  - `raw_cards.json`
  - `certified_panel.json`
  - `certified_panel.md`
  - `certified_cards.json`
  - `status.json`

# Acceptance Checks

- `02_answer` is read-only input to render.
- Render writes only under `03_render/`.
- Raw outputs can exist without certified outputs.
- Certified outputs are written only when the certified gate passed upstream.
- `03_render/status.json` records the terminal render outcome.

# Handoff

This is the terminal plan in the v1.1 runtime execution chain.
EOF
```

- [ ] **Step 2: Verify P11 distinguishes raw and certified outputs**

Run: `rg 'raw_panel|certified_panel|03_render/status.json|02_answer is read-only' docs/plans/tech-plan-v1.1-runtime/11-render-outputs.md -n`

Expected: all four concepts appear.

- [ ] **Step 3: Commit P11**

```bash
git add docs/plans/tech-plan-v1.1-runtime/11-render-outputs.md
git commit -m "docs: add v1.1 runtime plan p11"
```

### Task 16: Run Metadata And Graph Consistency Checks

**Files:**
- Modify: `docs/plans/tech-plan-v1.1-runtime/_index.json`
- Modify: `docs/plans/tech-plan-v1.1-runtime/AGENTS.md`
- Modify: `docs/plans/tech-plan-v1.1-runtime/[0-9][0-9]-*.md`

- [ ] **Step 1: Verify all 12 runtime plan files exist and declare `plan_id`**

Run: `rg '^plan_id:' docs/plans/tech-plan-v1.1-runtime/[0-9][0-9]-*.md`

Expected: 12 matches, one per runtime plan file.

- [ ] **Step 2: Verify the machine index parses**

Run: `jq empty docs/plans/tech-plan-v1.1-runtime/_index.json`

Expected: no output and exit status `0`.

- [ ] **Step 3: Run a frontmatter and graph consistency script**

```bash
python - <<'PY'
import json
from pathlib import Path
import yaml

root = Path("docs/plans/tech-plan-v1.1-runtime")
index = json.loads(root.joinpath("_index.json").read_text())
lookup = {item["plan_id"]: item for item in index["plans"]}
expected_deps = {
    "P00": [],
    "P01": [],
    "P02": ["P00", "P01"],
    "P03": [],
    "P04": ["P03"],
    "P05": ["P03", "P04"],
    "P06": ["P04"],
    "P07": ["P03", "P05"],
    "P08": ["P01", "P02", "P07"],
    "P09": ["P08"],
    "P10": ["P03", "P09"],
    "P11": ["P10"],
}

for path in sorted(root.glob("[0-9][0-9]-*.md")):
    text = path.read_text()
    frontmatter = yaml.safe_load(text.split("---", 2)[1])
    item = lookup[frontmatter["plan_id"]]
    assert item["title"] == frontmatter["title"], (frontmatter["plan_id"], "title")
    assert item["status"] == frontmatter["status"], (frontmatter["plan_id"], "status")
    assert item["depends_on"] == expected_deps[frontmatter["plan_id"]], (frontmatter["plan_id"], item["depends_on"])
    assert item["produces"] == frontmatter["produces"], (frontmatter["plan_id"], item["produces"], frontmatter["produces"])
print("ok: index matches v1.1 runtime plan frontmatter and approved dependency graph")
PY
```

Expected: `ok: index matches v1.1 runtime plan frontmatter and approved dependency graph`

- [ ] **Step 4: Inspect `AGENTS.md` and verify the graph edges match the approved runtime graph**

Run: `sed -n '1,120p' docs/plans/tech-plan-v1.1-runtime/AGENTS.md`

Expected: the execution graph matches the approved v1.1 runtime graph exactly.

- [ ] **Step 5: Commit the synchronized v1.1 runtime plan set**

```bash
git add docs/plans/tech-plan-v1.1-runtime
git commit -m "docs: finalize v1.1 runtime plan set"
```

## Self-Review

### Spec Coverage

This plan covers every approved v1.1 redesign requirement:

- parallel v1.1 runtime directory instead of overwriting v1: Tasks 1 through 16
- new `P00` entrypoint plan: Task 4
- five authoritative `dispatch_input_v1` field names: Task 8
- hard gate / soft review split: Tasks 9 and 10
- Stage 2 seal chain and worker-side prompt snapshot: Task 11
- `wv-answer-stage` ownership in the single-worker plan: Task 12
- raw vs certified render split: Tasks 14 and 15
- metadata and graph consistency for the new runtime plan set: Task 16

No spec gaps remain.

### Placeholder Scan

This plan contains no placeholders, no `TBD`, and no “fill in later” language. Every task names exact files, exact sections, exact commands, and expected verification output.

### Type Consistency

The v1.1 runtime plan IDs, dependency graph, and file names are used consistently in:

1. `AGENTS.md`
2. `README.md`
3. `_index.json`
4. the numbered plan files
5. the final metadata consistency script
