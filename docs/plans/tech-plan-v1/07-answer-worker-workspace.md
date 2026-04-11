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

- Create an isolated workspace for each persona under 02_answer/.
- Write persona-specific AGENTS.md into that workspace.
- Write outgoing_input.json with artifact paths and hashes.
- Restrict Stage 2 input handling to the prepared workspace artifacts only.

# Out Of Scope

- Running the worker itself.
- Batch concurrency control.
- Render-stage aggregation.

# Required Inputs

- docs/plans/tech-plan-v1/06-prepare-review-gate.md
- docs/tech-plan-v1.md

# Implementation Tasks

- Add internal/stage/answer/workspace.go.
- Build workspace path helpers on top of the shared layout layer.
- Copy prepared source agents.md from 01_prepare/.../agents.md into the worker workspace as canonical workspace/AGENTS.md.
- Write outgoing_input.json with keys required by the Stage 2 contract:
  - persona_id
  - agent_instructions_path
  - agent_instructions_sha256
  - prompt_path
  - prompt_sha256
  - skill_path
- Add explicit enforcement in Stage 2 path resolution that only the following are read:
  - canonical workspace instruction file workspace/AGENTS.md
  - canonical pre-rendered prompt file by prompt_path from outgoing_input.json
- Forbid Stage 2 from reading dispatch_input_v1.json directly.
- Forbid ad hoc Stage 2 reads that are not under the workspace/input contract or not declared in outgoing_input.json.

# Acceptance Checks

- Each persona gets a dedicated workspace under 02_answer/personas/<persona_id>/workspace/.
- outgoing_input.json contains the required key set and records hashes for both the rendered prompt and persona instructions.
- outgoing_input.json uses canonical path references for:
  - prepared source artifact 01_prepare/.../agents.md as instruction source
  - worker-facing workspace/AGENTS.md
  - pre-rendered prompt.txt
- Stage 2 cannot consume dispatch_input_v1.json and cannot use any ad hoc inputs that bypass the outgoing input contract.

# Handoff

P08 runs one worker using only the artifacts created by this workspace layer.
