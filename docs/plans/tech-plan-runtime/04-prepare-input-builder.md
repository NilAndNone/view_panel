---
plan_id: P04
title: Prepare Input Builder
status: proposed
depends_on:
  - P03
consumes:
  - docs/plans/tech-plan-runtime/03-run-layout-and-artifact-writer.md
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

Create the canonical Stage 1 assembly path so each persona receives exactly one prepared input bundle and immutable per-persona evidence artifacts used by downstream Stage 2 sealing.

# Scope

- Build one canonical dispatch_input_v1.json per persona at 01_prepare/personas/<persona_id>/dispatch_input_v1.json as a flat top-level JSON object using only these five fields (no aliases); each approved field value is the fully rendered text section for that persona and must serialize as a JSON string:
  - roleplay_prompt
  - discussion_question
  - supplementary_materials
  - output_contract
  - assumptions_and_constraints
- Canonical five-field mapping must be explicit, one-to-one, and source-anchored:
  - `roleplay_prompt` comes from the upstream `roleplay_prompt` source in the canonical incident model.
  - `discussion_question` comes from the upstream `discussion_question` source in the canonical incident model.
  - `supplementary_materials` comes from the canonical incident `supplementary_materials` source.
  - `output_contract` comes from the canonical incident `output_contract` source.
  - `assumptions_and_constraints` comes from the canonical incident `assumptions_and_constraints` source.
- Build all Stage 1 artifacts for each persona under 01_prepare/personas/<persona_id>/; the persona directory contains five Stage 1 artifacts total:
  - Sendable artifacts:
    - agents.md
    - prompt.txt
    - dispatch_input_v1.json
  - Evidence artifacts (not sendable):
    - manifest.json
    - hashes.json
- For P04, the configured persona-set index for the run (for example `runtime/persona-index.json` from the request/CLI contract) is the sole authoritative selection and resolution input for Stage 1 persona generation; `runtime/personas/` is only the backing definition store for referenced personas and is not authoritative for membership on its own.
- The emitted `01_prepare/personas/<persona_id>/` directory set is the authoritative prepare-stage persona set consumed downstream; downstream stages consume the prepared directory set, not the configured persona-set index.
- Persona selection and ordering are deterministic and fixed for P04:
  - membership is exactly the persona entries declared by the persona-set index after validation, and declared order is preserved during persona selection and resolution
  - P04 must deterministically resolve the selected persona-set and render exactly one `01_prepare/personas/<persona_id>/` directory per resolved persona
  - P04 must not discover personas by directory scan, map iteration, or implicit inclusion of unindexed persona files
  - P04 must reject duplicate persona identifiers, missing persona definitions, or invalid persona-set entries rather than silently filtering or reordering
  - once the selected persona-set is fully resolved, P04 emits the per-persona directories and their bundles in lexicographic `persona_id` order; that emitted directory order defines the prepare-stage authoritative set consumed downstream
- Sendable artifact rendering is source-anchored and deterministic:
  - `agents.md` is rendered from the same canonical per-persona `roleplay_prompt` string used for `dispatch_input_v1.json`, and from no other source
  - `prompt.txt` is rendered from the same canonical per-persona strings used for `dispatch_input_v1.json`, using only `discussion_question`, `supplementary_materials`, `output_contract`, and `assumptions_and_constraints` in that exact order
  - `prompt.txt` excludes `roleplay_prompt`, and `agents.md` excludes the other four canonical fields
- Resolve and render per-persona materials and personas into deterministic Stage 1 artifact values.

# Out Of Scope

- Prepare hard gate decisions (P05).
- Prepare soft review (P06).
- Stage 2 workspace seal-chain work and worker prompt reconstruction (P07).
- Any Stage 2 recomposition behavior in Stage 1 code.

# Required Inputs

- `docs/plans/tech-plan-runtime/03-run-layout-and-artifact-writer.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

 1. Add strict, exact-field `schemas/dispatch_input_v1.json` as the minimal executable schema for this plan set:
   - top-level `type: object`
   - `required` is exactly `roleplay_prompt`, `discussion_question`, `supplementary_materials`, `output_contract`, and `assumptions_and_constraints`
   - `additionalProperties: false`
   - each approved property schema is exactly `{ "type": "string" }`
   - the schema intentionally treats all five approved fields as already-rendered text sections; arrays, objects, numbers, booleans, and null are invalid for every approved field
2. Implement internal/materials/loader.go to load the five canonical Stage 1 fields into deterministic models using explicit one-to-one mappings:
   - `roleplay_prompt` ← `incident.roleplay_prompt`
   - `discussion_question` ← `incident.discussion_question`
   - `supplementary_materials` ← `incident.supplementary_materials`
   - `output_contract` ← `incident.output_contract`
   - `assumptions_and_constraints` ← `incident.assumptions_and_constraints`
3. Implement internal/persona/library.go to resolve personas and validate persona pack paths for the prepare stage:
   - the authoritative input is the configured persona-set index path for the run, and it is the sole selection/resolution input for Stage 1 persona generation; do not use filesystem enumeration of `runtime/personas/` to define membership
   - load the persona-set index as an ordered list and preserve that declared order exactly through persona resolution in the returned prepare persona slice
   - validate every referenced persona definition path before prepare execution
   - reject duplicate persona identifiers, missing persona definitions, and invalid persona-set entries; P04 must not sort, filter, or silently deduplicate
4. Implement internal/stage/prepare/prepare.go:
  - Deterministically resolve the validated persona-set into exactly one prepared persona directory per resolved persona, then emit `01_prepare/personas/<persona_id>/` directories and their artifacts in lexicographic `persona_id` order; the emitted directory set is the authoritative prepare-stage persona set consumed downstream.
  - Build dispatch_input_v1.json per persona from the approved five fields only, serializing each approved field exactly once as a top-level JSON string property.
  - Prerender agents.md and prompt.txt for each persona from the same canonical in-memory Stage 1 field model used to build dispatch_input_v1.json; do not reread or independently template sendable content from alternate sources.
  - Render `agents.md` as the exact canonical `roleplay_prompt` text for that persona, with no prepended headers, appended footers, or injected copies of the other four fields.
  - Render `prompt.txt` as the exact canonical concatenation:
    `discussion_question + "\n\n" + supplementary_materials + "\n\n" + output_contract + "\n\n" + assumptions_and_constraints`
    with no extra sections, labels, wrappers, metadata preamble, or inclusion of `roleplay_prompt`.
  - Emit manifest.json as the authoritative artifact inventory and path manifest:
    - Required keys: `schema_version = "prepare_manifest_v1"`, `stage = "01_prepare"`, `persona_id`, `artifacts`.
    - `manifest.json.artifacts` is shape-stable and must be an array of exactly three objects in this order:
      1. `{ "path": "dispatch_input_v1.json", "hash_ref": "dispatch_input_sha256", "size_bytes": <byte_size> }`
      2. `{ "path": "agents.md", "hash_ref": "agents_sha256", "size_bytes": <byte_size> }`
      3. `{ "path": "prompt.txt", "hash_ref": "prompt_sha256", "size_bytes": <byte_size> }`
    - `manifest.json.artifacts` must inventory only the three sendable bundle members, not all five Stage 1 files.
    - `path` values must be persona-root-relative and exactly match the file names above.
    - `hash_ref` values must exactly equal `dispatch_input_sha256`, `agents_sha256`, and `prompt_sha256` for the corresponding artifacts above.
  - Emit hashes.json as the authoritative hash ledger:
    - Required keys: `dispatch_input_sha256`, `agents_sha256`, `prompt_sha256`, `bundle_sha256`.
    - `dispatch_input_sha256` is the SHA-256 of the exact canonical bytes written to `dispatch_input_v1.json`.
    - `agents_sha256` is the SHA-256 of the exact bytes written to `agents.md`.
    - `prompt_sha256` is the SHA-256 of the exact bytes written to `prompt.txt`.
    - `hashes.json` is authoritative for all four required hash values and is the sole reference source for manifest entries.
    - `bundle_sha256` must be computed from canonical content bytes (not file paths) by explicitly length-prefixing each artifact payload and concatenating in order:
      `dispatch_input_v1.json` -> `agents.md` -> `prompt.txt`.
    - For each artifact, append `uint64_be(length)` bytes followed by the exact raw canonical content bytes before hashing.
  - Use internal/storage/layout.go path helpers and shared writer helpers for every artifact write.
5. Add a hard assertion in Stage 1 that dispatch_input_v1.json is the only Stage 1 sendable input structure; no alternate naming or fields may be produced.
6. Do not emit any Stage 2 sealed-input ownership rules in P04 artifacts; P07 owns worker snapshot creation and Stage 2 allowlist.

# Acceptance Checks

- For each prepared persona, all five required files exist under 01_prepare/personas/<persona_id>/.
- P04 resolves personas only from the configured persona-set index for the run; `runtime/personas/` directory contents alone cannot add, remove, or reorder prepare personas.
- P04 deterministically resolves exactly one prepared persona directory per selected persona from the validated persona-set index, rejects duplicate persona identifiers or unresolved persona references, and emits `01_prepare/personas/<persona_id>/` directories in lexicographic `persona_id` order.
- The emitted `01_prepare/personas/<persona_id>/` directory set is the authoritative prepare-stage persona set consumed downstream.
- dispatch_input_v1.json contains exactly roleplay_prompt, discussion_question, supplementary_materials, output_contract, and assumptions_and_constraints with no alias or extra fields, and each approved field value is a JSON string.
- For each persona, `agents.md` bytes are exactly the canonical `roleplay_prompt` text used for that persona's `dispatch_input_v1.json.roleplay_prompt`.
- For each persona, `prompt.txt` bytes are exactly:
  `dispatch_input_v1.json.discussion_question + "\n\n" + dispatch_input_v1.json.supplementary_materials + "\n\n" + dispatch_input_v1.json.output_contract + "\n\n" + dispatch_input_v1.json.assumptions_and_constraints`
  with no extra wrapper text and no inclusion of `dispatch_input_v1.json.roleplay_prompt`.
- schemas/dispatch_input_v1.json rejects any non-object top-level value, missing approved field, extra field, or non-string value at any approved field.
- manifest.json includes required keys and exactly three artifact entries, each with persona-root-relative paths.
- manifest.json artifact `hash_ref` values are exact:
  - dispatch_input_v1.json → dispatch_input_sha256
  - agents.md → agents_sha256
  - prompt.txt → prompt_sha256
- hashes.json includes exactly `dispatch_input_sha256`, `agents_sha256`, `prompt_sha256`, and `bundle_sha256`.
- For each persona, `bundle_sha256` is a SHA-256 digest over the canonical artifact byte sequence, not path values.
- For each persona, emitted `manifest.json` artifact paths are persona-root-relative and emitted `hashes.json` values are present for every required hash key.

# Handoff

P05 and P06 consume the emitted `01_prepare/personas/<persona_id>/` directory set and these exact Stage 1 artifacts as immutable evidence.
P04 does not define Stage 2 sealed-input construction or allowlist rules.
