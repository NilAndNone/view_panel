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

Build the canonical Stage 1 packet for each persona by assembling question, materials, and persona inputs into `dispatch_input_v1` at one source of truth in P05.

# Scope

- Load question, materials, persona metadata, and persona prompts from their configured sources and define precedence in P05 so downstream stages consume only one normalized form.
- Authoritative field mapping is fixed in this plan:
  - `dispatch_input_v1.roleplay` and related persona sections come from persona prompt files and persona metadata.
  - `dispatch_input_v1.question` and discussion framing come from the loaded question source.
  - `dispatch_input_v1.materials` and supplementary context come from material loader outputs.
  - Shared constraints and contract fields come from the shared plan/design source and are copied identically for every persona.
- Build `dispatch_input_v1` as the normalized, authoritative Stage 1 packet and write it once; downstream stages do not reassemble or mutate this packet.
- Pre-render `agents.md` and `prompt.txt` for every persona as part of the same canonical artifact build.
- Write `dispatch_input_v1.json`, `agents.md`, `prompt.txt`, `manifest.json`, and `hashes.json` into per-persona directories under `01_prepare/`.

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

- Every persona has one canonical `01_prepare/<persona_id>/dispatch_input_v1.json`.
- `01_prepare/<persona_id>/agents.md` and `01_prepare/<persona_id>/prompt.txt` are produced before any Stage 2 execution begins.
- P05 produces these per-persona files: `dispatch_input_v1.json`, `agents.md`, `prompt.txt`, `manifest.json`, and `hashes.json`, and these files are the prerendered source consumed by later plans.
- `01_prepare/<persona_id>/manifest.json` names the canonical file set and source references for each artifact.
- `01_prepare/<persona_id>/hashes.json` includes deterministic fields:
  - `dispatch_input_sha256` for `dispatch_input_v1.json`
  - `agents_sha256` for `agents.md`
  - `prompt_sha256` for `prompt.txt`
  - `bundle_sha256` for the declared artifact bundle (inputs+rendered artifacts), computed over canonical content values, not filesystem path strings.

# Handoff

P06 reviews the canonical packet and applies the Stage 2 gate.
P07 consumes the prerendered artifacts from `01_prepare/<persona_id>/` without re-rendering inputs.
