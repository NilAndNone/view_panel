# Tech Plan V1 Plan Set Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the executable plan set for `docs/tech-plan-v1.md` by creating the root `AGENTS.md`, the `docs/plans/tech-plan-v1/` overview files, and the 11 single-purpose plan documents defined in the approved design spec.

**Architecture:** This work is documentation-first. The deliverable is a plan set that becomes the execution interface for later implementation work. `AGENTS.md` dispatches Codex, `README.md` explains the system to humans, `_index.json` supports machine lookup, and each numbered plan file is the source of truth for one bounded implementation slice.

**Tech Stack:** Markdown, JSON, shell commands, repository documentation

---

## File Map

**Read First**
- `docs/tech-plan-v1.md`
- `docs/superpowers/specs/2026-04-11-tech-plan-v1-executable-plans-design.md`

**Create**
- `AGENTS.md`
- `docs/plans/tech-plan-v1/README.md`
- `docs/plans/tech-plan-v1/_index.json`
- `docs/plans/tech-plan-v1/01-codex-runtime-contract.md`
- `docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md`
- `docs/plans/tech-plan-v1/03-run-layout-and-artifact-writer.md`
- `docs/plans/tech-plan-v1/04-audit-log-pipeline.md`
- `docs/plans/tech-plan-v1/05-prepare-input-builder.md`
- `docs/plans/tech-plan-v1/06-prepare-review-gate.md`
- `docs/plans/tech-plan-v1/07-answer-worker-workspace.md`
- `docs/plans/tech-plan-v1/08-answer-single-worker-runner.md`
- `docs/plans/tech-plan-v1/09-answer-batch-orchestration.md`
- `docs/plans/tech-plan-v1/10-render-input-aggregation.md`
- `docs/plans/tech-plan-v1/11-render-stage-output.md`

**Validation Commands**
- `jq empty docs/plans/tech-plan-v1/_index.json`
- `rg '^plan_id:' docs/plans/tech-plan-v1/[0-9][0-9]-*.md`
- `rg '^status:' docs/plans/tech-plan-v1/[0-9][0-9]-*.md`
- `python` consistency check script in Task 15

### Task 1: Create The Dispatcher Entry Point

**Files:**
- Create: `AGENTS.md`
- Create: `docs/plans/tech-plan-v1/`

- [ ] **Step 1: Create the plan directory**

```bash
mkdir -p docs/plans/tech-plan-v1
```

- [ ] **Step 2: Create `AGENTS.md` with the approved execution graph and dispatch rules**

```bash
cat <<'EOF' > AGENTS.md
# Tech Plan V1 Execution Guide

## Purpose

Execute the plan set for `docs/tech-plan-v1.md` using the files in `docs/plans/tech-plan-v1/`.

## Plan Directory

`docs/plans/tech-plan-v1/`

## Execution Graph

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

## Dispatch Rules

1. Pick the lowest-numbered incomplete plan whose `depends_on` entries are all `done`.
2. Read only `AGENTS.md`, `docs/plans/tech-plan-v1/README.md`, the target plan file, and directly required dependency plans.
3. Do not start implementation from `README.md` alone.
4. Do not merge multiple plans into one execution unless the target plan says so.
5. After finishing a plan, update both the target plan frontmatter and `docs/plans/tech-plan-v1/_index.json`.

## Source Of Truth Order

1. The target plan file
2. `docs/plans/tech-plan-v1/_index.json`
3. `AGENTS.md`
4. `docs/plans/tech-plan-v1/README.md`
EOF
```

- [ ] **Step 3: Verify the dispatcher file contains the execution graph**

Run: `sed -n '1,120p' AGENTS.md`

Expected: the file shows the 11-edge execution graph and the five dispatch rules exactly once.

- [ ] **Step 4: Commit the dispatcher entry point**

```bash
git add AGENTS.md docs/plans/tech-plan-v1
git commit -m "docs: add tech plan v1 execution guide"
```

### Task 2: Create The Human Overview

**Files:**
- Create: `docs/plans/tech-plan-v1/README.md`

- [ ] **Step 1: Create `README.md` for human navigation**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1/README.md
# Tech Plan V1 Executable Plan Set

## Why This Exists

`docs/tech-plan-v1.md` defines the target system, but it is too broad to execute safely as one unit. This directory decomposes that strategy into bounded plans that can be completed one at a time without losing the Stage 1 -> Stage 2 -> Stage 3 authority chain.

## Artifacts

1. `AGENTS.md` is the Codex dispatcher.
2. `_index.json` is the machine-readable index.
3. `01-*.md` through `11-*.md` are the executable plan files.

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
EOF
```

- [ ] **Step 2: Verify the overview calls out the source-of-truth order**

Run: `rg 'Source Of Truth|Acceptance Standard' docs/plans/tech-plan-v1/README.md -n`

Expected: both sections are present.

- [ ] **Step 3: Commit the human overview**

```bash
git add docs/plans/tech-plan-v1/README.md
git commit -m "docs: add tech plan v1 overview"
```

### Task 3: Create The Machine Index

**Files:**
- Create: `docs/plans/tech-plan-v1/_index.json`

- [ ] **Step 1: Create the initial plan index with statuses and dependencies**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1/_index.json
{
  "plan_set_id": "tech-plan-v1",
  "source": "docs/tech-plan-v1.md",
  "last_updated": "2026-04-11",
  "plans": [
    {
      "plan_id": "P01",
      "title": "Codex Runtime Contract",
      "status": "ready",
      "depends_on": [],
      "path": "docs/plans/tech-plan-v1/01-codex-runtime-contract.md",
      "produces": [
        "docs/operations/codex-runtime-contract.md",
        "third_party/codex-protocol/"
      ]
    },
    {
      "plan_id": "P02",
      "title": "App-Server Client Lifecycle",
      "status": "proposed",
      "depends_on": ["P01"],
      "path": "docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md",
      "produces": [
        "internal/appserver/client.go",
        "internal/appserver/messages.go",
        "internal/appserver/runner.go"
      ]
    },
    {
      "plan_id": "P03",
      "title": "Run Layout And Artifact Writer",
      "status": "ready",
      "depends_on": [],
      "path": "docs/plans/tech-plan-v1/03-run-layout-and-artifact-writer.md",
      "produces": [
        "internal/storage/layout.go",
        "internal/hash/hash.go"
      ]
    },
    {
      "plan_id": "P04",
      "title": "Audit Log Pipeline",
      "status": "proposed",
      "depends_on": ["P02", "P03"],
      "path": "docs/plans/tech-plan-v1/04-audit-log-pipeline.md",
      "produces": [
        "internal/audit/logger.go",
        "internal/audit/summary.go"
      ]
    },
    {
      "plan_id": "P05",
      "title": "Prepare Input Builder",
      "status": "proposed",
      "depends_on": ["P03"],
      "path": "docs/plans/tech-plan-v1/05-prepare-input-builder.md",
      "produces": [
        "internal/stage/prepare/prepare.go",
        "schemas/dispatch_input_v1.json"
      ]
    },
    {
      "plan_id": "P06",
      "title": "Prepare Review Gate",
      "status": "proposed",
      "depends_on": ["P04", "P05"],
      "path": "docs/plans/tech-plan-v1/06-prepare-review-gate.md",
      "produces": [
        "runtime/skills/wv-prepare-stage/SKILL.md",
        "schemas/prepare_review_v1.json"
      ]
    },
    {
      "plan_id": "P07",
      "title": "Answer Worker Workspace",
      "status": "proposed",
      "depends_on": ["P06"],
      "path": "docs/plans/tech-plan-v1/07-answer-worker-workspace.md",
      "produces": [
        "internal/stage/answer/workspace.go",
        "runs/<run_id>/02_answer/personas/<persona_id>/outgoing_input.json"
      ]
    },
    {
      "plan_id": "P08",
      "title": "Answer Single Worker Runner",
      "status": "proposed",
      "depends_on": ["P07"],
      "path": "docs/plans/tech-plan-v1/08-answer-single-worker-runner.md",
      "produces": [
        "internal/stage/answer/runner.go",
        "schemas/worldview_worker_result_v1.json"
      ]
    },
    {
      "plan_id": "P09",
      "title": "Answer Batch Orchestration",
      "status": "proposed",
      "depends_on": ["P08"],
      "path": "docs/plans/tech-plan-v1/09-answer-batch-orchestration.md",
      "produces": [
        "internal/stage/answer/batch.go",
        "internal/orchestrator/run.go"
      ]
    },
    {
      "plan_id": "P10",
      "title": "Render Input Aggregation",
      "status": "proposed",
      "depends_on": ["P09"],
      "path": "docs/plans/tech-plan-v1/10-render-input-aggregation.md",
      "produces": [
        "internal/stage/render/aggregate.go",
        "runs/<run_id>/03_render/render_input.json"
      ]
    },
    {
      "plan_id": "P11",
      "title": "Render Stage Output",
      "status": "proposed",
      "depends_on": ["P10"],
      "path": "docs/plans/tech-plan-v1/11-render-stage-output.md",
      "produces": [
        "internal/stage/render/render.go",
        "schemas/panel_result_v1.json"
      ]
    }
  ]
}
EOF
```

- [ ] **Step 2: Verify the JSON parses cleanly**

Run: `jq empty docs/plans/tech-plan-v1/_index.json`

Expected: no output and exit status `0`.

- [ ] **Step 3: Commit the machine index**

```bash
git add docs/plans/tech-plan-v1/_index.json
git commit -m "docs: add tech plan v1 machine index"
```

### Task 4: Write Plan P01

**Files:**
- Create: `docs/plans/tech-plan-v1/01-codex-runtime-contract.md`

- [ ] **Step 1: Create the P01 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1/01-codex-runtime-contract.md
---
plan_id: P01
title: Codex Runtime Contract
status: ready
depends_on: []
consumes:
  - docs/tech-plan-v1.md
  - docs/superpowers/specs/2026-04-11-tech-plan-v1-executable-plans-design.md
produces:
  - docs/operations/codex-runtime-contract.md
  - third_party/codex-protocol/
completion_evidence:
  - codex_cli_version_pinned
  - app_server_transport_fixed_to_stdio
  - schema_generation_location_defined
---

# Goal

Define the supported Codex runtime contract so every later plan targets the same CLI version, transport, schema bundle location, and smoke-test flow.

# Scope

- Pin one supported `@openai/codex` version.
- Lock `codex app-server --listen stdio://` as the only transport for v1.
- Define where generated protocol schema is stored.
- Record install, login, schema generation, and smoke-test commands.

# Out Of Scope

- Implementing the Go client.
- Building any stage logic.
- Introducing WebSocket support.

# Required Inputs

- `docs/tech-plan-v1.md`
- `docs/superpowers/specs/2026-04-11-tech-plan-v1-executable-plans-design.md`

# Implementation Tasks

- Create `docs/operations/codex-runtime-contract.md`.
- Record the pinned CLI version and the reason it must stay locked.
- Record `stdio://` as the only supported app-server transport.
- Record the schema generation command and the smoke-test command.

# Acceptance Checks

- The runtime contract document names exactly one supported CLI version.
- The contract declares `stdio://` as the only supported transport.
- The schema output directory is fixed to `third_party/codex-protocol/`.
- The smoke-test command is written exactly.

# Handoff

P02 consumes the pinned CLI, transport, and schema directory decisions.
EOF
```

- [ ] **Step 2: Verify the plan status and dependency fields**

Run: `sed -n '1,40p' docs/plans/tech-plan-v1/01-codex-runtime-contract.md`

Expected: `status: ready` and `depends_on: []` appear in frontmatter.

- [ ] **Step 3: Commit P01**

```bash
git add docs/plans/tech-plan-v1/01-codex-runtime-contract.md
git commit -m "docs: add plan p01 runtime contract"
```

### Task 5: Write Plan P02

**Files:**
- Create: `docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md`

- [ ] **Step 1: Create the P02 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md
---
plan_id: P02
title: App-Server Client Lifecycle
status: proposed
depends_on:
  - P01
consumes:
  - docs/plans/tech-plan-v1/01-codex-runtime-contract.md
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
- Expose a single-turn runner API for later stages.

# Out Of Scope

- Stage-specific prompt construction.
- Audit log persistence.
- Batch orchestration.

# Required Inputs

- `docs/plans/tech-plan-v1/01-codex-runtime-contract.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

- Add protocol message types in `internal/appserver/messages.go`.
- Add the process-backed client in `internal/appserver/client.go`.
- Add a single-turn lifecycle helper in `internal/appserver/runner.go`.
- Add tests or canned transcript checks for handshake and final completion parsing.

# Acceptance Checks

- The client can complete the full handshake sequence in order.
- `configRequirements/read` is exposed before `thread/start`.
- `item/completed.agentMessage` is treated as the final worker output source.
- Interrupt flow is defined before process kill.

# Handoff

P04 and P08 consume this client lifecycle instead of inventing their own protocol loops.
EOF
```

- [ ] **Step 2: Verify the plan depends on P01**

Run: `rg '^depends_on:|^  - P01' docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md -n`

Expected: the frontmatter shows `P01` as the only dependency.

- [ ] **Step 3: Commit P02**

```bash
git add docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md
git commit -m "docs: add plan p02 app-server client"
```

### Task 6: Write Plan P03

**Files:**
- Create: `docs/plans/tech-plan-v1/03-run-layout-and-artifact-writer.md`

- [ ] **Step 1: Create the P03 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1/03-run-layout-and-artifact-writer.md
---
plan_id: P03
title: Run Layout And Artifact Writer
status: ready
depends_on: []
consumes:
  - docs/tech-plan-v1.md
  - docs/superpowers/specs/2026-04-11-tech-plan-v1-executable-plans-design.md
produces:
  - internal/storage/layout.go
  - internal/hash/hash.go
completion_evidence:
  - run_directory_layout_fixed
  - shared_writer_helpers_defined
  - sha256_helpers_available
---

# Goal

Define the on-disk run layout and the shared writers that every stage uses to persist canonical artifacts.

# Scope

- Create a typed run layout for `00_request`, `01_prepare`, `02_answer`, `03_render`, and `audit`.
- Add helpers for JSON, text, and JSONL writing.
- Add SHA-256 helpers for dispatch bundles and outgoing worker inputs.

# Out Of Scope

- Protocol transport logic.
- Stage-specific business logic.
- Global log indexing.

# Required Inputs

- `docs/tech-plan-v1.md`
- `docs/superpowers/specs/2026-04-11-tech-plan-v1-executable-plans-design.md`

# Implementation Tasks

- Add `internal/storage/layout.go` with canonical path constructors.
- Add `internal/hash/hash.go` with SHA-256 helpers.
- Add writer utilities that later plans can use without re-implementing path logic.
- Add focused tests for deterministic paths and hash stability.

# Acceptance Checks

- The run layout exposes stable paths for every named stage directory.
- Writers can create JSON, text, and JSONL artifacts under that layout.
- Hash helpers can produce the dispatch and combined hashes required by later stages.

# Handoff

P04, P05, P07, P08, P10, and P11 consume these path and writer helpers.
EOF
```

- [ ] **Step 2: Verify the plan status is `ready`**

Run: `rg '^status: ready$' docs/plans/tech-plan-v1/03-run-layout-and-artifact-writer.md -n`

Expected: one match.

- [ ] **Step 3: Commit P03**

```bash
git add docs/plans/tech-plan-v1/03-run-layout-and-artifact-writer.md
git commit -m "docs: add plan p03 run layout"
```

### Task 7: Write Plan P04

**Files:**
- Create: `docs/plans/tech-plan-v1/04-audit-log-pipeline.md`

- [ ] **Step 1: Create the P04 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1/04-audit-log-pipeline.md
---
plan_id: P04
title: Audit Log Pipeline
status: proposed
depends_on:
  - P02
  - P03
consumes:
  - docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md
  - docs/plans/tech-plan-v1/03-run-layout-and-artifact-writer.md
produces:
  - internal/audit/logger.go
  - internal/audit/summary.go
completion_evidence:
  - protocol_in_out_logs_defined
  - normalized_events_defined
  - run_summary_and_global_index_defined
---

# Goal

Implement the audit pipeline that makes `run_root` the primary evidence source for every run and every worker.

# Scope

- Persist `protocol.out.jsonl` and `protocol.in.jsonl`.
- Persist normalized `events.jsonl`.
- Persist run-level `run_summary.json`.
- Persist a locator-only global `run_index.jsonl`.

# Out Of Scope

- Generating Stage 1 or Stage 2 content.
- Worker prompt rendering.
- Render-stage aggregation.

# Required Inputs

- `docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md`
- `docs/plans/tech-plan-v1/03-run-layout-and-artifact-writer.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

- Add `internal/audit/logger.go` for raw protocol and normalized event recording.
- Add `internal/audit/summary.go` for stage summary and global index writing.
- Define the event fields required by the incident-driven audit chain.
- Add tests for append-only JSONL output and summary completeness.

# Acceptance Checks

- Every worker-capable stage can emit both raw protocol logs and normalized events.
- The global index is documented as a locator, not the source of truth.
- Missing audit artifacts can be surfaced as run failure conditions.

# Handoff

P06 and P09 use this audit layer to record stage decisions and worker outcomes.
EOF
```

- [ ] **Step 2: Verify P04 depends on both P02 and P03**

Run: `rg '^  - P0[23]$' docs/plans/tech-plan-v1/04-audit-log-pipeline.md -n`

Expected: two matches, one for `P02` and one for `P03`.

- [ ] **Step 3: Commit P04**

```bash
git add docs/plans/tech-plan-v1/04-audit-log-pipeline.md
git commit -m "docs: add plan p04 audit pipeline"
```

### Task 8: Write Plan P05

**Files:**
- Create: `docs/plans/tech-plan-v1/05-prepare-input-builder.md`

- [ ] **Step 1: Create the P05 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1/05-prepare-input-builder.md
---
plan_id: P05
title: Prepare Input Builder
status: proposed
depends_on:
  - P03
consumes:
  - docs/plans/tech-plan-v1/03-run-layout-and-artifact-writer.md
  - docs/tech-plan-v1.md
produces:
  - internal/materials/loader.go
  - internal/persona/library.go
  - internal/stage/prepare/prepare.go
  - schemas/dispatch_input_v1.json
completion_evidence:
  - dispatch_input_v1_built_once
  - agents_md_prerendered
  - prompt_txt_prerendered
---

# Goal

Build the canonical Stage 1 packet for each persona so Stage 2 never has to re-render worker inputs.

# Scope

- Load question, materials, persona metadata, and persona prompts.
- Build `dispatch_input_v1` with all required fields.
- Pre-render `agents.md` and `prompt.txt` for every persona.
- Write manifest and hash artifacts into `01_prepare/`.

# Out Of Scope

- Codex review of prepared inputs.
- Stage 2 execution.
- Render-stage synthesis.

# Required Inputs

- `docs/plans/tech-plan-v1/03-run-layout-and-artifact-writer.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

- Add materials loading in `internal/materials/loader.go`.
- Add persona loading in `internal/persona/library.go`.
- Add Stage 1 packet generation in `internal/stage/prepare/prepare.go`.
- Add `schemas/dispatch_input_v1.json` and validate generated packets before writing.

# Acceptance Checks

- Every persona gets one canonical `dispatch_input_v1.json`.
- `agents.md` and `prompt.txt` are materialized before any Stage 2 execution begins.
- Hash artifacts cover dispatch input, rendered agents, rendered prompt, and the combined bundle.

# Handoff

P06 reviews the canonical packet. P07 consumes the rendered worker inputs without re-rendering them.
EOF
```

- [ ] **Step 2: Verify the plan mentions both `agents.md` and `prompt.txt`**

Run: `rg 'agents.md|prompt.txt' docs/plans/tech-plan-v1/05-prepare-input-builder.md -n`

Expected: both artifact names appear in scope or acceptance.

- [ ] **Step 3: Commit P05**

```bash
git add docs/plans/tech-plan-v1/05-prepare-input-builder.md
git commit -m "docs: add plan p05 prepare input builder"
```

### Task 9: Write Plan P06

**Files:**
- Create: `docs/plans/tech-plan-v1/06-prepare-review-gate.md`

- [ ] **Step 1: Create the P06 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1/06-prepare-review-gate.md
---
plan_id: P06
title: Prepare Review Gate
status: proposed
depends_on:
  - P04
  - P05
consumes:
  - docs/plans/tech-plan-v1/04-audit-log-pipeline.md
  - docs/plans/tech-plan-v1/05-prepare-input-builder.md
produces:
  - runtime/skills/wv-prepare-stage/SKILL.md
  - schemas/prepare_review_v1.json
completion_evidence:
  - prepare_review_written
  - prepare_status_written
  - stage2_gate_enforced
---

# Goal

Insert a review-only gate between canonical input preparation and Stage 2 execution.

# Scope

- Add the `wv-prepare-stage` skill.
- Define the `prepare_review_v1` schema.
- Run one review session over prepared persona inputs.
- Persist `prepare_review.json` and `prepare_status.json`.

# Out Of Scope

- Rewriting the canonical packet.
- Starting any Stage 2 worker.
- Rendering final panel output.

# Required Inputs

- `docs/plans/tech-plan-v1/04-audit-log-pipeline.md`
- `docs/plans/tech-plan-v1/05-prepare-input-builder.md`

# Implementation Tasks

- Add `runtime/skills/wv-prepare-stage/SKILL.md`.
- Add `schemas/prepare_review_v1.json`.
- Extend the Stage 1 pipeline to run the review session after packet generation.
- Fail the stage if review or completeness checks do not pass.

# Acceptance Checks

- The review step cannot mutate any canonical Stage 1 artifact.
- `prepare_status.json` is the gate checked before Stage 2 starts.
- Review decisions are captured in the audit stream.

# Handoff

P07 can start only when this gate marks the run ready for Stage 2.
EOF
```

- [ ] **Step 2: Verify P06 names both output review files**

Run: `rg 'prepare_review.json|prepare_status.json' docs/plans/tech-plan-v1/06-prepare-review-gate.md -n`

Expected: both files appear.

- [ ] **Step 3: Commit P06**

```bash
git add docs/plans/tech-plan-v1/06-prepare-review-gate.md
git commit -m "docs: add plan p06 prepare review gate"
```

### Task 10: Write Plan P07

**Files:**
- Create: `docs/plans/tech-plan-v1/07-answer-worker-workspace.md`

- [ ] **Step 1: Create the P07 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1/07-answer-worker-workspace.md
---
plan_id: P07
title: Answer Worker Workspace
status: proposed
depends_on:
  - P06
consumes:
  - docs/plans/tech-plan-v1/06-prepare-review-gate.md
produces:
  - internal/stage/answer/workspace.go
  - runs/<run_id>/02_answer/personas/<persona_id>/workspace/AGENTS.md
  - runs/<run_id>/02_answer/personas/<persona_id>/outgoing_input.json
completion_evidence:
  - workspace_isolation_defined
  - outgoing_input_written
  - rerendering_blocked
---

# Goal

Define the per-persona workspace contract so every Stage 2 worker receives exactly the prepared persona context and nothing else.

# Scope

- Create an isolated workspace for each persona under `02_answer/`.
- Write persona-specific `AGENTS.md` into that workspace.
- Write `outgoing_input.json` with artifact paths and hashes.
- Enforce that Stage 2 can only consume pre-rendered `agents.md` and `prompt.txt`.

# Out Of Scope

- Running the worker itself.
- Batch concurrency control.
- Render-stage aggregation.

# Required Inputs

- `docs/plans/tech-plan-v1/06-prepare-review-gate.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

- Add `internal/stage/answer/workspace.go`.
- Build workspace path helpers on top of the shared layout layer.
- Copy prepared `agents.md` into the worker workspace as `AGENTS.md`.
- Write `outgoing_input.json` with persona ID, skill path, and all required hashes.

# Acceptance Checks

- Each persona gets a dedicated workspace under `02_answer/personas/<persona_id>/workspace/`.
- `outgoing_input.json` records the hashes for the rendered prompt and persona instructions.
- No Stage 2 code path re-renders the prompt from `dispatch_input_v1.json`.

# Handoff

P08 runs one worker using only the artifacts created by this workspace layer.
EOF
```

- [ ] **Step 2: Verify the plan forbids re-rendering**

Run: `rg 're-renders|re-renders the prompt|re-renders the prompt from' docs/plans/tech-plan-v1/07-answer-worker-workspace.md -n`

Expected: at least one match in scope or acceptance.

- [ ] **Step 3: Commit P07**

```bash
git add docs/plans/tech-plan-v1/07-answer-worker-workspace.md
git commit -m "docs: add plan p07 answer workspace"
```

### Task 11: Write Plan P08

**Files:**
- Create: `docs/plans/tech-plan-v1/08-answer-single-worker-runner.md`

- [ ] **Step 1: Create the P08 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1/08-answer-single-worker-runner.md
---
plan_id: P08
title: Answer Single Worker Runner
status: proposed
depends_on:
  - P07
consumes:
  - docs/plans/tech-plan-v1/07-answer-worker-workspace.md
  - docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md
produces:
  - internal/stage/answer/runner.go
  - schemas/worldview_worker_result_v1.json
completion_evidence:
  - single_worker_turn_executes
  - final_agent_message_captured
  - result_artifacts_written
---

# Goal

Run one Stage 2 worker from start to finish and persist only the final structured result and attestation artifacts.

# Scope

- Start one app-server process for one persona.
- Run one thread and one turn.
- Parse only `item/completed.agentMessage` as final output.
- Validate the worker result against `worldview_worker_result_v1`.
- Persist raw result text, parsed result JSON, attestation, and status.

# Out Of Scope

- Batch scheduling across personas.
- Retry policy across workers.
- Render-stage synthesis.

# Required Inputs

- `docs/plans/tech-plan-v1/07-answer-worker-workspace.md`
- `docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md`

# Implementation Tasks

- Add `schemas/worldview_worker_result_v1.json`.
- Add `internal/stage/answer/runner.go`.
- Wire the runner to read workspace inputs and write worker outputs.
- Reject outputs that fail schema validation or lack a final completed message.

# Acceptance Checks

- The runner reads only pre-rendered inputs from the workspace contract.
- Final output comes from `item/completed.agentMessage` only.
- `result.raw.txt`, `result.json`, `attestation.json`, and `status.json` are all written.

# Handoff

P09 scales this single-worker runner across the persona set.
EOF
```

- [ ] **Step 2: Verify the plan names the four result artifacts**

Run: `rg 'result.raw.txt|result.json|attestation.json|status.json' docs/plans/tech-plan-v1/08-answer-single-worker-runner.md -n`

Expected: all four artifact names appear.

- [ ] **Step 3: Commit P08**

```bash
git add docs/plans/tech-plan-v1/08-answer-single-worker-runner.md
git commit -m "docs: add plan p08 single worker runner"
```

### Task 12: Write Plan P09

**Files:**
- Create: `docs/plans/tech-plan-v1/09-answer-batch-orchestration.md`

- [ ] **Step 1: Create the P09 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1/09-answer-batch-orchestration.md
---
plan_id: P09
title: Answer Batch Orchestration
status: proposed
depends_on:
  - P08
consumes:
  - docs/plans/tech-plan-v1/08-answer-single-worker-runner.md
  - docs/plans/tech-plan-v1/04-audit-log-pipeline.md
produces:
  - internal/stage/answer/batch.go
  - internal/orchestrator/run.go
completion_evidence:
  - persona_fanout_supported
  - forbidden_tool_detection_defined
  - aggregate_stage2_status_written
---

# Goal

Execute the full persona batch for Stage 2 with concurrency control, worker failure handling, and strict certification rules.

# Scope

- Fan out the single-worker runner across the persona set.
- Add concurrency limits and timeout handling.
- Interrupt and terminate workers that overrun.
- Mark any worker that emits tool-type items as failed.
- Aggregate per-persona outcomes into stage-level status.

# Out Of Scope

- Stage 1 prompt generation.
- Render input aggregation.
- UI presentation of worker results.

# Required Inputs

- `docs/plans/tech-plan-v1/08-answer-single-worker-runner.md`
- `docs/plans/tech-plan-v1/04-audit-log-pipeline.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

- Add `internal/stage/answer/batch.go`.
- Extend `internal/orchestrator/run.go` to drive the full Stage 2 fan-out.
- Add forbidden-item detection for command, file, MCP, dynamic tool, collaboration, web, and image items.
- Write certified outcome summaries that later render stages can trust.

# Acceptance Checks

- Batch orchestration can run all personas with a configurable concurrency limit.
- Timeouts trigger interrupt-first shutdown before process kill.
- Any forbidden tool item marks that persona as failed.
- Stage-level status distinguishes certified results from failed or rejected ones.

# Handoff

P10 consumes only the certified outputs from this stage-level result set.
EOF
```

- [ ] **Step 2: Verify the plan names forbidden tool categories**

Run: `rg 'command|file|MCP|dynamic tool|collaboration|web|image' docs/plans/tech-plan-v1/09-answer-batch-orchestration.md -n`

Expected: matches cover all forbidden tool categories.

- [ ] **Step 3: Commit P09**

```bash
git add docs/plans/tech-plan-v1/09-answer-batch-orchestration.md
git commit -m "docs: add plan p09 answer batch orchestration"
```

### Task 13: Write Plan P10

**Files:**
- Create: `docs/plans/tech-plan-v1/10-render-input-aggregation.md`

- [ ] **Step 1: Create the P10 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1/10-render-input-aggregation.md
---
plan_id: P10
title: Render Input Aggregation
status: proposed
depends_on:
  - P09
consumes:
  - docs/plans/tech-plan-v1/09-answer-batch-orchestration.md
produces:
  - internal/stage/render/aggregate.go
  - runs/<run_id>/03_render/render_input.json
completion_evidence:
  - certified_results_only
  - minimum_success_rate_enforced
  - missing_personas_not_backfilled
---

# Goal

Build the render-stage input only from certified Stage 2 outputs and fail fast when the minimum success threshold is not met.

# Scope

- Read Stage 2 persona statuses.
- Select only certified worker results.
- Enforce the configured minimum success rate.
- Write `render_input.json`.

# Out Of Scope

- Producing final panel output.
- Rewriting Stage 2 answers.
- Guessing missing worker results.

# Required Inputs

- `docs/plans/tech-plan-v1/09-answer-batch-orchestration.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

- Add `internal/stage/render/aggregate.go`.
- Filter Stage 2 results down to schema-valid, certified worker outputs.
- Compute success-rate gating before render begins.
- Write `render_input.json` for the final stage.

# Acceptance Checks

- The aggregator excludes failed, uncertified, or tool-contaminated worker outputs.
- The minimum success-rate gate is enforced before render execution.
- Missing personas are never backfilled or invented.

# Handoff

P11 consumes only `render_input.json` and the certified result set behind it.
EOF
```

- [ ] **Step 2: Verify the plan forbids backfilling**

Run: `rg 'backfilled|invented|guessing' docs/plans/tech-plan-v1/10-render-input-aggregation.md -n`

Expected: at least one match.

- [ ] **Step 3: Commit P10**

```bash
git add docs/plans/tech-plan-v1/10-render-input-aggregation.md
git commit -m "docs: add plan p10 render aggregation"
```

### Task 14: Write Plan P11

**Files:**
- Create: `docs/plans/tech-plan-v1/11-render-stage-output.md`

- [ ] **Step 1: Create the P11 plan file**

```bash
cat <<'EOF' > docs/plans/tech-plan-v1/11-render-stage-output.md
---
plan_id: P11
title: Render Stage Output
status: proposed
depends_on:
  - P10
consumes:
  - docs/plans/tech-plan-v1/10-render-input-aggregation.md
  - docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md
produces:
  - internal/stage/render/render.go
  - runtime/skills/wv-render-stage/SKILL.md
  - schemas/panel_result_v1.json
completion_evidence:
  - render_session_runs_once
  - panel_outputs_written
  - run_completion_status_finalized
---

# Goal

Run the render stage once over the certified result set and write the final panel artifacts and end-of-run status.

# Scope

- Add the `wv-render-stage` skill.
- Add the `panel_result_v1` schema.
- Run one render session over `render_input.json`.
- Write `panel.json`, `panel.md`, `cards.json`, and final render status.

# Out Of Scope

- Editing Stage 2 answers.
- Filling in missing worker responses.
- Changing the Stage 1 packet.

# Required Inputs

- `docs/plans/tech-plan-v1/10-render-input-aggregation.md`
- `docs/plans/tech-plan-v1/02-app-server-client-lifecycle.md`

# Implementation Tasks

- Add `runtime/skills/wv-render-stage/SKILL.md`.
- Add `schemas/panel_result_v1.json`.
- Add `internal/stage/render/render.go` to execute the render session and persist outputs.
- Finalize end-of-run render status and audit summary links.

# Acceptance Checks

- Render reads only `render_input.json` and certified worker results.
- `panel.json`, `panel.md`, and `cards.json` are written together.
- The render stage does not modify any Stage 2 worker result.

# Handoff

This is the terminal plan in the chain. Its outputs feed user-facing inspection and final run reporting.
EOF
```

- [ ] **Step 2: Verify the plan names all three render outputs**

Run: `rg 'panel.json|panel.md|cards.json' docs/plans/tech-plan-v1/11-render-stage-output.md -n`

Expected: all three output files appear.

- [ ] **Step 3: Commit P11**

```bash
git add docs/plans/tech-plan-v1/11-render-stage-output.md
git commit -m "docs: add plan p11 render output"
```

### Task 15: Run Plan-Set Consistency Checks

**Files:**
- Modify: `AGENTS.md`
- Modify: `docs/plans/tech-plan-v1/README.md`
- Modify: `docs/plans/tech-plan-v1/_index.json`
- Modify: `docs/plans/tech-plan-v1/*.md`

- [ ] **Step 1: Verify that all 11 plan files exist and declare `plan_id`**

Run: `rg '^plan_id:' docs/plans/tech-plan-v1/[0-9][0-9]-*.md`

Expected: 11 matches, one per numbered plan file.

- [ ] **Step 2: Run a consistency script against `_index.json` and the plan frontmatter**

```bash
python - <<'PY'
import json
from pathlib import Path

root = Path("docs/plans/tech-plan-v1")
index = json.loads(root.joinpath("_index.json").read_text())
expected = {item["plan_id"]: item for item in index["plans"]}

for path in sorted(root.glob("[0-9][0-9]-*.md")):
    lines = path.read_text().splitlines()
    frontmatter = {}
    current_key = None
    in_frontmatter = False
    for line in lines:
        if line.strip() == "---":
            in_frontmatter = not in_frontmatter
            if not in_frontmatter and frontmatter:
                break
            continue
        if not in_frontmatter:
            continue
        if line.startswith("  - "):
            frontmatter.setdefault(current_key, []).append(line[4:].strip())
            continue
        if ": " in line:
            key, value = line.split(": ", 1)
            current_key = key
            if value == "[]":
                frontmatter[key] = []
            elif value:
                frontmatter[key] = value.strip()
            else:
                frontmatter[key] = []
    plan_id = frontmatter["plan_id"]
    assert plan_id in expected, f"missing from index: {plan_id}"
    idx = expected[plan_id]
    assert frontmatter["title"] == idx["title"], f"title mismatch for {plan_id}"
    assert frontmatter["status"] == idx["status"], f"status mismatch for {plan_id}"
    assert frontmatter.get("depends_on", []) == idx["depends_on"], f"deps mismatch for {plan_id}"
print("ok: index matches plan frontmatter")
PY
```

Expected: `ok: index matches plan frontmatter`

- [ ] **Step 3: Confirm the AGENTS graph still matches the approved edges**

Run: `sed -n '1,80p' AGENTS.md`

Expected: the execution graph still shows the approved 11 edges and the dispatcher rules have not drifted.

- [ ] **Step 4: Commit the final synchronized plan set**

```bash
git add AGENTS.md docs/plans/tech-plan-v1
git commit -m "docs: finalize tech plan v1 executable plan set"
```

## Self-Review

### Spec Coverage

The task list covers every design requirement from `docs/superpowers/specs/2026-04-11-tech-plan-v1-executable-plans-design.md`:

- `AGENTS.md` execution graph and dispatch rules: Tasks 1 and 15
- Human overview and source-of-truth split: Task 2
- Machine-readable index: Task 3
- Eleven executable plans with fixed frontmatter and sections: Tasks 4 through 14
- Status model and dependency graph consistency: Tasks 3 and 15
- Plan-set acceptance and Codex-ready handoff: Tasks 2, 3, and 15

No spec gaps remain.

### Placeholder Scan

This plan contains no `TBD`, `TODO`, or “implement later” language. Each task names exact files, exact content, exact commands, and an expected verification result.

### Type Consistency

`plan_id`, `status`, `depends_on`, `produces`, and `completion_evidence` use the same names in the plan file template, the `_index.json` schema, and the consistency check script.
