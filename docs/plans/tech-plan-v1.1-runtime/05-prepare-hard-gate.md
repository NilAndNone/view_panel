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
  - runs/<run_id>/01_prepare/prepare_gate_status_v1.json
completion_evidence:
  - deterministic_prepare_gate_defined
  - missing_artifacts_block_stage2
  - prepare_status_written
---

# Goal

Implement a deterministic, Go-only hard gate that blocks Stage 2 if and only if required prepare-time invariants are violated.

# Scope

- Add internal/stage/prepare/gate.go as the Stage 1 hard-blocking gate runner.
- Build schemas/prepare_gate_status_v1.json and emit the canonical machine-readable gate result at `01_prepare/prepare_gate_status_v1.json` under `<run_root>`.
- Define the authoritative Stage 2 handoff fields in the status artifact:
  - can_proceed_to_stage2 (bool) is authoritative for admission gating and must be derived directly from the hard-check result.
  - persona_ids (array of string) is authoritative for Stage 2 persona membership and lexicographic order.
  - P07 consumes `can_proceed_to_stage2` for admission gating and `persona_ids` for authoritative Stage 2 membership/order, and ignores process exit status.
  - Process exit status is optional and not required as a P07 input.
- All paths in this plan are `<run_root>`-relative (e.g. `01_prepare/...` means `<run_root>/01_prepare/...`).
- Enforce five blocking checks only, each with deterministic single-owner responsibility:
  1. `prepare-persona-discovery`: authoritative persona set is derived from immediate child directories under `<run_root>/01_prepare/personas`, with no non-directory entries and at least one persona.
  2. `prepare-required-artifacts`: handle only path resolution, existence, and readability for required files, and is the sole owner of `dispatch_input_v1.json` path-resolution, missing, and unreadable failures.
  3. `prepare-dispatch-input-schema`: run immediately after `prepare-required-artifacts`, assume `dispatch_input_v1.json` was already resolved/readable by that prior check, and validate content-level parse/type/exact-field-set violations only.
  4. `prepare-hash-and-bundle`: owns all manifest semantic validation and hash consistency checks.
  5. `prepare-audit-writability`: required audit/log paths under `<run_root>/audit` are writable; P03 owns materializing `<run_root>/audit/events.jsonl` and `<run_root>/audit/errors.log`, and P05 validates only their pre-existing writability.
- Execute checks in fixed order 1→2→3→4→5, emit all five check results in the stable `checks` array, and never fail-fast.
- Treat gate execution as deterministic and validate-only, with one explicit runtime write exception: emit `<run_root>/01_prepare/prepare_gate_status_v1.json`; do not mutate existing prepare or audit artifacts.
- Make control-flow ownership explicit without changing dependencies:
  - `internal/stage/prepare/prepare.go` (owned by `P04`) is the sole runtime call site that invokes the P05 gate after P04 canonical artifact assembly is complete.
  - That P04-owned call is responsible for causing `<run_root>/01_prepare/prepare_gate_status_v1.json` to be emitted before control can advance to any `P07` Stage 2 seal attempt that consumes it.
  - `P06` remains advisory and non-blocking: it does not own hard-gate invocation, does not emit or rewrite `prepare_gate_status_v1.json`, and does not become a required input for `P07`.
- Keep P05 as the only mandatory prepare gate in the chain and explicitly preserve the P05 -> P07 blocking dependency.

# Out Of Scope

- model review, persona quality review, advisory comments, or scoring logic.
- Stage 2 workspace sealing and worker prompt reconstruction (P07).
- Stage 3 raw/certified render split and rendering (P10, P11).
- Any additional heuristics, policy scoring, or semantic LLM checks.

# Required Inputs

- docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md
- docs/plans/tech-plan-v1.1-runtime/04-prepare-input-builder.md

# Implementation Tasks

1. Implement `prepare-persona-discovery` in `internal/stage/prepare/gate.go`.
   - enumerate immediate child entries of `<run_root>/01_prepare/personas`.
   - each immediate child directory is one authoritative `<persona_id>`.
   - each immediate non-directory child entry is a hard failure.
   - record non-directory-entry failures at the check level on `prepare-persona-discovery` with `P05_E_PERSONA_ENTRY_NON_DIRECTORY`; do not create a synthetic `persona_results` row for the offending entry.
   - on directory enumeration or persona-id extraction failure, fail with `P05_E_PERSONA_IO_ERROR`.
   - sort the discovered ids lexicographically.
   - if zero persona directories are discovered, fail the check at the check level with `P05_E_PERSONA_SET_EMPTY`.
   - `prepare-persona-discovery` must emit exactly one winning check-level `error_code` by this deterministic precedence:
     1. directory enumeration or persona-id extraction failure -> `P05_E_PERSONA_IO_ERROR`
     2. otherwise any immediate non-directory child entry present -> `P05_E_PERSONA_ENTRY_NON_DIRECTORY`
     3. otherwise zero authoritative persona directories discovered -> `P05_E_PERSONA_SET_EMPTY`
   - if enumeration completes successfully and authoritative persona directories are established, but the check still fails only at the check level, `persona_results` must still contain one row per discovered authoritative persona with `status: pass` and no `error_code` or `error_message`.
   - if enumeration does not complete cleanly and the authoritative persona set cannot be established, `persona_results` MUST be exactly `[]`.
   - write only this sorted authority set to `persona_ids`; non-directory entries never appear in `persona_ids`.
2. Implement `prepare-required-artifacts`: validate required prepare artifacts for each authoritative persona.
  - dispatch_input_v1.json
  - agents.md
  - prompt.txt
  - manifest.json
  - hashes.json
  - evaluate artifacts in this canonical order for per-persona error selection:
    1. dispatch_input_v1.json
    2. agents.md
    3. prompt.txt
    4. manifest.json
    5. hashes.json
  - run this check immediately before `prepare-dispatch-input-schema`.
  - for `dispatch_input_v1.json`, this check is the sole owner of path resolution, missing-file, and unreadable-file failures.
  - for each artifact in that order, resolve it through `PreparePersonaArtifactPath` (persona-root-relative) and require resolve success, then evaluate existence, then readability.
  - emit exactly one `persona_results` row per authoritative persona; if multiple artifact failures are true for that persona, the first failing condition in the ordered traversal above becomes the row's sole canonical `error_code`/`error_message`, and later failures do not replace it.
  - if path resolution fails, fail with `P05_E_REQUIRED_ARTIFACT_PATH_INVALID`
  - map missing artifact failures to specific canonical codes:
    - missing dispatch_input_v1.json -> `P05_E_REQUIRED_ARTIFACT_MISSING_DISPATCH_INPUT`
    - missing agents.md -> `P05_E_REQUIRED_ARTIFACT_MISSING_AGENTS`
    - missing prompt.txt -> `P05_E_REQUIRED_ARTIFACT_MISSING_PROMPT`
    - missing manifest.json -> `P05_E_REQUIRED_ARTIFACT_MISSING_MANIFEST`
    - missing hashes.json -> `P05_E_REQUIRED_ARTIFACT_MISSING_HASHES`
  - if a required artifact exists but cannot be read, fail with `P05_E_REQUIRED_ARTIFACT_UNREADABLE`
  - `prepare-dispatch-input-schema` must not emit any `P05_E_REQUIRED_ARTIFACT_*` code or remap these failures.
3. Implement `prepare-dispatch-input-schema`: strict per-persona `dispatch_input_v1.json` validation for each authoritative persona id.
  - consume only the already-resolved/readable `dispatch_input_v1.json` artifact established by the immediately prior `prepare-required-artifacts` check.
  - do not perform path resolution, missing-file detection, unreadable-file detection, or emit any `P05_E_REQUIRED_ARTIFACT_*` code here.
  - content-level type/schema validation in this check means enforcing the exact P04 `schemas/dispatch_input_v1.json` contract after field-set validation: `roleplay_prompt`, `discussion_question`, `supplementary_materials`, `output_contract`, and `assumptions_and_constraints` must each be present as JSON string values; arrays, objects, numbers, booleans, and null are invalid at any approved field.
  - use this deterministic ordered evaluation, with the first matching condition becoming the row's sole canonical `error_code`:
    1. malformed JSON, decode failure, or a non-object top-level value -> `P05_E_DISPATCH_INPUT_INVALID`
    2. otherwise, compare the parsed top-level object key set against the exact approved key set below; any missing required key or any additional key -> `P05_E_DISPATCH_INPUT_FIELDS_INVALID`
    3. otherwise, validate the approved field values against the exact string-typed schema above; any approved field value that is not a JSON string -> `P05_E_DISPATCH_INPUT_INVALID`
  - require all five approved keys exactly:
    - roleplay_prompt
    - discussion_question
    - supplementary_materials
    - output_contract
    - assumptions_and_constraints
  - require the top-level field set to match exactly the five approved keys above; field-set validation runs only after successful top-level object parsing and before any approved-field type/schema validation.
  - this check does not add any non-empty-text rule or other semantic validation beyond that exact string-typed contract.
  - if the immediately prior `prepare-required-artifacts` check already failed a persona on `dispatch_input_v1.json` path-resolution, missing, or unreadable grounds, record that persona result here as `status: fail` with no `error_code`; use `error_message` only to point back to `prepare-required-artifacts`, so this check owns only content-level failures.
  - emit exactly one `persona_results` entry per authoritative persona.
4. Implement `prepare-hash-and-bundle`: validate hash ledger completeness and integrity.
   - if the immediately prior `prepare-required-artifacts` check already failed a persona on path-invalid, missing, or unreadable grounds for any artifact needed here (`manifest.json`, `hashes.json`, `dispatch_input_v1.json`, `agents.md`, `prompt.txt`), still emit that persona result here as `status: fail` with no `error_code`; use `error_message` only to point back to `prepare-required-artifacts`, and do not remap that failure inside check 4.
   - only after `prepare-required-artifacts` established all five needed artifacts as resolved/readable for a persona may this check emit its own canonical error codes for that persona.
   - a fresh open/read failure on `hashes.json` or `manifest.json` first encountered during this check, after `prepare-required-artifacts` had already established readability, is a local check-4 failure and must not be remapped back to check 2.
   - if a fresh open/read of `hashes.json` fails during this check before JSON parsing, fail with `P05_E_HASH_EVIDENCE_UNREADABLE`.
   - parse `hashes.json` from `<run_root>/01_prepare/personas/<persona_id>/hashes.json` and require a top-level JSON object with exactly these four keys and no others:
     - dispatch_input_sha256
     - agents_sha256
     - prompt_sha256
     - bundle_sha256
   - malformed JSON, non-object top-level shape, any required key present with a non-string value, or any additional key beyond the exact required set -> `P05_E_HASH_LEDGER_INVALID`.
   - if any required hash key is missing, fail that persona with `P05_E_HASH_LEDGER_KEY_MISSING`.
   - if a fresh open/read of `manifest.json` fails during this check before JSON parsing or semantic validation, fail with `P05_E_HASH_EVIDENCE_UNREADABLE`.
   - load `manifest.json` from `<run_root>/01_prepare/personas/<persona_id>/manifest.json` and own all manifest semantic validation:
     - malformed JSON, missing required schema fields, or wrong required manifest literals -> `P05_E_MANIFEST_INVALID`.
     - require manifest literals `schema_version == "prepare_manifest_v1"` and `stage == "01_prepare"`; if either literal is missing, non-string, or not exactly that value, fail with `P05_E_MANIFEST_INVALID`.
     - manifest-level persona parity is canonical: missing or non-string `manifest.persona_id` -> `P05_E_MANIFEST_INVALID`; present string `manifest.persona_id` unequal to `<persona_id>` -> `P05_E_MANIFEST_PERSONA_ID_MISMATCH`.
     - exactly three artifact objects, in canonical order:
       1. { "path": "dispatch_input_v1.json", "hash_ref": "dispatch_input_sha256", "size_bytes": <byte_count> }
       2. { "path": "agents.md", "hash_ref": "agents_sha256", "size_bytes": <byte_count> }
       3. { "path": "prompt.txt", "hash_ref": "prompt_sha256", "size_bytes": <byte_count> }
     - each object must contain exactly `path`, `hash_ref`, and `size_bytes`
     - each `size_bytes` must equal the actual byte size of the corresponding artifact payload.
     - each `hash_ref` must bind to an existing key in `hashes.json`
   - require all required hash string values to be 64-char lowercase hex digests; any invalid hash format fails with `P05_E_HASH_FORMAT_INVALID`.
   - recompute per-file SHA-256 for dispatch_input_v1.json, agents.md, and prompt.txt and compare with the matching hashes.json keys by `hash_ref` binding; any per-file hash mismatch fails with `P05_E_HASH_MISMATCH`.
   - `manifest.json` is invalid if any required contract/order/hash_ref binding fails.
   - recompute bundle_sha256 from canonical artifact bytes and compare with the ledger:
    - exact order: dispatch_input_v1.json -> agents.md -> prompt.txt
    - for each artifact payload, append uint64_be(len(payload_bytes)) then raw payload bytes to the hash stream
    - `P05_E_HASH_PAYLOAD_UNREADABLE` is reserved for a new artifact payload read failure first encountered during this check after `prepare-required-artifacts` had already established that payload as readable.
    - if recomputed bundle_sha256 does not match the ledger value, fail with `P05_E_BUNDLE_MISMATCH`.
   - deterministic per-persona error selection for this check must follow this ordered evaluation, with the first matching condition becoming the row's sole canonical `error_code`:
     1. fresh open/read failure on `hashes.json` after `prepare-required-artifacts` had already established readability -> `P05_E_HASH_EVIDENCE_UNREADABLE`
     2. malformed `hashes.json`, non-object top-level shape, any required value with non-string type, or any additional key beyond the exact required set -> `P05_E_HASH_LEDGER_INVALID`
     3. missing required key in `hashes.json` -> `P05_E_HASH_LEDGER_KEY_MISSING`
     4. invalid required hash value format -> `P05_E_HASH_FORMAT_INVALID`
     5. fresh open/read failure on `manifest.json` after `prepare-required-artifacts` had already established readability -> `P05_E_HASH_EVIDENCE_UNREADABLE`
     6. malformed `manifest.json`, missing required schema fields, wrong required manifest literals (`schema_version != "prepare_manifest_v1"` or `stage != "01_prepare"`), or missing/non-string `manifest.persona_id` -> `P05_E_MANIFEST_INVALID`
     7. present string `manifest.persona_id` unequal to `<persona_id>` -> `P05_E_MANIFEST_PERSONA_ID_MISMATCH`
     8. manifest artifact contract/order/hash_ref binding/size_bytes violations -> `P05_E_MANIFEST_INVALID`
     9. payload read plus per-file digest comparison in canonical artifact order `dispatch_input_v1.json` -> `agents.md` -> `prompt.txt`:
        - first new read failure encountered during this check, after `prepare-required-artifacts` had already established readability -> `P05_E_HASH_PAYLOAD_UNREADABLE`
        - otherwise first per-file digest mismatch -> `P05_E_HASH_MISMATCH`
     10. bundle digest comparison -> `P05_E_BUNDLE_MISMATCH`
   - later simultaneously true failures for the same persona must not replace an earlier selected canonical error.
   - fail hard on any missing key, malformed hash_ref binding, invalid hash format, per-file hash mismatch, bundle mismatch, or unreadable payload using the canonical codes above.
5. Implement `prepare-audit-writability`: validate audit/log writability without mutation.
   - treat `<run_root>/audit/events.jsonl` and `<run_root>/audit/errors.log` as canonical P03-owned audit files that should already exist before P05 runs; P05 validates only their pre-existing writability and does not materialize them.
   - check `<run_root>/audit/events.jsonl` already exists and is writable.
   - check `<run_root>/audit/errors.log` already exists and is writable.
   - if either path is missing, fail with `P05_E_AUDIT_PATH_NOT_WRITABLE`; this same canonical code intentionally also covers the present-but-not-writable case.
   - if present, the path must be openable for append/write; otherwise fail with `P05_E_AUDIT_PATH_NOT_WRITABLE`.
   - do not create or append files in P05.
6. Emit `01_prepare/prepare_gate_status_v1.json` as the single source of truth and define `schemas/prepare_gate_status_v1.json` as its structural schema contract.
   - `schemas/prepare_gate_status_v1.json` validates the runtime status artifact's structural shape, not derived runtime semantics.
   - status artifact top-level keys are exactly:
     - schema_version
     - stage
     - status
     - can_proceed_to_stage2
     - persona_ids
     - checks
   - pin exact status literals:
     - schema_version: "prepare_gate_status_v1"
     - stage: "01_prepare"
     - top-level status must be exactly `pass` or `fail`.
  - define `checks` as a stable ordered array with exact `check_id` values in this order:
    1. `prepare-persona-discovery`
    2. `prepare-required-artifacts`
    3. `prepare-dispatch-input-schema`
    4. `prepare-hash-and-bundle`
    5. `prepare-audit-writability`
  - `schemas/prepare_gate_status_v1.json` must at minimum encode:
    - top-level `type: object`, the exact top-level key set above, and `additionalProperties: false`
    - exact literals `schema_version == "prepare_gate_status_v1"` and `stage == "01_prepare"`
    - `status` as enum `pass|fail` and `can_proceed_to_stage2` as type `boolean`
    - `persona_ids` as an array of strings
    - `checks` as an array of exactly five objects in the fixed order above, using the exact `check_id` value for each position
    - check result objects with exact keys `check_id`, `status`, `persona_results`, optional `error_code`, optional `error_message`, and `additionalProperties: false`
    - persona result objects with exact keys `persona_id`, `status`, optional `error_code`, optional `error_message`, and `additionalProperties: false`
    - the `prepare-audit-writability` check entry requiring `persona_results: []`
  - schema scope is structural only; lexicographic ordering, status derivation, and cross-check back-reference semantics remain runtime rules enforced by this plan.
   - always emit all five checks in this fixed order even after earlier failures.
  - define canonical object shapes:
     - check result object: `{ "check_id", "status", "error_code?", "error_message?", "persona_results" }`
     - persona result object: `{ "persona_id", "status", "error_code?", "error_message?" }`
     - derive `status` from local evidence only:
       - a check object is `status: fail` if it has a check-level `error_code` or `error_message`, or if any row in `persona_results` has `status: fail`; otherwise the check object is `status: pass`.
       - a persona result object is `status: fail` if that check selected a canonical local `error_code` for the persona, or if this plan explicitly requires the row to back-reference `prepare-required-artifacts` with no local `error_code`; otherwise the persona result object is `status: pass`.
     - `prepare-persona-discovery` derives its check `status` only from its own check-level error fields; any persona rows it emits are observational and must all be `status: pass`.
     - persona-scoped checks emit exactly one `persona_results` row per authoritative persona and at most one canonical `error_code` per row.
     - `prepare-required-artifacts`, `prepare-dispatch-input-schema`, and `prepare-hash-and-bundle` are persona-scoped checks and must not synthesize separate check-level failure codes; each such check is `status: fail` iff at least one emitted persona row is `status: fail`, including back-reference rows with no local `error_code`.
     - for run-level checks (`prepare-audit-writability`), `persona_results` MUST be exactly `[]`.
     - `prepare-audit-writability` derives its check `status` only from its own check-level error fields.
     - for `prepare-persona-discovery`, `persona_results` contains only discovered authoritative personas and is lexicographically sorted by `persona_id`; failures not attributable to an authoritative persona, including non-directory entries, empty-set failure, and directory-enumeration I/O failure, are encoded only at the check level via `error_code`/`error_message`.
     - `prepare-persona-discovery` selects its single check-level `error_code` by precedence `P05_E_PERSONA_IO_ERROR` -> `P05_E_PERSONA_ENTRY_NON_DIRECTORY` -> `P05_E_PERSONA_SET_EMPTY`.
     - if `prepare-persona-discovery` successfully establishes authoritative persona directories but fails only at check level, `persona_results` MUST still contain one lexicographically sorted row per authoritative persona, each with `status: pass` and no `error_code` or `error_message`.
     - if `prepare-persona-discovery` cannot establish any authoritative persona, its `persona_results` MUST be exactly `[]`.
     - do not create synthetic `persona_results` rows for filesystem entries that are not authoritative personas.
     - for `prepare-required-artifacts`, `prepare-dispatch-input-schema`, and `prepare-hash-and-bundle`, `persona_results` contains one entry per authoritative persona and is lexicographically sorted by `persona_id`; if `prepare-persona-discovery` did not establish any authoritative persona, each of these checks must still be emitted with `persona_results: []`, no synthetic check-level error, and `status: pass`.
     - for `prepare-required-artifacts` and `prepare-hash-and-bundle`, when multiple failure conditions are true for the same persona, the check-specific ordered evaluation rule selects the first matching canonical error and later conditions do not replace it.
     - for `prepare-dispatch-input-schema` and `prepare-hash-and-bundle`, a persona blocked by an earlier `prepare-required-artifacts` path-invalid/missing/unreadable failure must still appear as `status: fail` with no `error_code`; `error_message` must point back to `prepare-required-artifacts`.
  - check and persona `status` values are exactly `pass` or `fail`.
   - ordering semantics:
     - `checks` follows the exact `check_id` order above.
     - `persona_ids` and each `persona_results` list are lexicographically sorted by `persona_id`.
 - define canonical error codes needed by this gate:
    - `P05_E_PERSONA_SET_EMPTY`
    - `P05_E_PERSONA_ENTRY_NON_DIRECTORY`
    - `P05_E_PERSONA_IO_ERROR`
    - `P05_E_DISPATCH_INPUT_INVALID`
    - `P05_E_DISPATCH_INPUT_FIELDS_INVALID`
 - `P05_E_MANIFEST_INVALID`
  - `P05_E_MANIFEST_PERSONA_ID_MISMATCH`
  - `P05_E_HASH_EVIDENCE_UNREADABLE`
  - `P05_E_HASH_LEDGER_INVALID`
  - `P05_E_HASH_LEDGER_KEY_MISSING`
  - `P05_E_HASH_FORMAT_INVALID`
  - `P05_E_HASH_MISMATCH`
  - `P05_E_HASH_PAYLOAD_UNREADABLE`
  - `P05_E_BUNDLE_MISMATCH`
  - `P05_E_AUDIT_PATH_NOT_WRITABLE`
  - `P05_E_REQUIRED_ARTIFACT_MISSING_DISPATCH_INPUT`
  - `P05_E_REQUIRED_ARTIFACT_MISSING_AGENTS`
  - `P05_E_REQUIRED_ARTIFACT_MISSING_PROMPT`
  - `P05_E_REQUIRED_ARTIFACT_MISSING_MANIFEST`
  - `P05_E_REQUIRED_ARTIFACT_MISSING_HASHES`
  - `P05_E_REQUIRED_ARTIFACT_UNREADABLE`
  - `P05_E_REQUIRED_ARTIFACT_PATH_INVALID`
   - `P05_E_REQUIRED_ARTIFACT_*` codes are emitted only by `prepare-required-artifacts`, including all `dispatch_input_v1.json` path-resolution, missing, and unreadable failures.
   - `prepare-dispatch-input-schema` emits only `P05_E_DISPATCH_INPUT_INVALID` or `P05_E_DISPATCH_INPUT_FIELDS_INVALID`, and only after the immediately prior `prepare-required-artifacts` check established that `dispatch_input_v1.json` resolved and was readable for that persona.
   - `prepare-dispatch-input-schema` precedence is deterministic: malformed JSON / decode failure / non-object top-level -> `P05_E_DISPATCH_INPUT_INVALID`; otherwise exact top-level field-set mismatch -> `P05_E_DISPATCH_INPUT_FIELDS_INVALID`; otherwise any approved field value that is not a JSON string -> `P05_E_DISPATCH_INPUT_INVALID`.
   - `P05_E_HASH_EVIDENCE_UNREADABLE` is emitted only by `prepare-hash-and-bundle` for a fresh open/read failure of `hashes.json` or `manifest.json` first encountered during check 4 after `prepare-required-artifacts` had already established readability.
   - keep `can_proceed_to_stage2` aligned to `status` (pass => true, fail => false).
   - when any mandatory check fails, status must be fail and can_proceed_to_stage2 must be false.
   - emit no additional policy checks.
7. Ensure P05 remains validate-only and deterministic:
   - the only runtime write allowed in P05 is emitting `<run_root>/01_prepare/prepare_gate_status_v1.json`.
   - do not mutate, rewrite, delete, or repair any pre-existing prepare or audit artifact.
   - no new policy checks.
   - no Stage 2 sealed-input ownership logic.
   - keep all existing behavior restricted to the five checks.

# Acceptance Checks

- For every authoritative persona (derived from `<run_root>/01_prepare/personas` child directories), `dispatch_input_v1.json` loads successfully with all five required keys and no unknown fields, and each approved field value is a JSON string exactly as defined by P04 `schemas/dispatch_input_v1.json`.
- `prepare-required-artifacts` runs before `prepare-dispatch-input-schema` and is the sole source of `dispatch_input_v1.json` path-resolution, missing, and unreadable failure codes.
- `prepare-dispatch-input-schema` assumes the immediately prior `prepare-required-artifacts` check already established `dispatch_input_v1.json` readability; it emits only `P05_E_DISPATCH_INPUT_INVALID` or `P05_E_DISPATCH_INPUT_FIELDS_INVALID` for content-level failures, and records prior I/O blockers without a competing `error_code`.
- `prepare-dispatch-input-schema` uses deterministic schema precedence: malformed JSON / decode failure / non-object top-level -> `P05_E_DISPATCH_INPUT_INVALID`; otherwise missing or additional approved top-level keys -> `P05_E_DISPATCH_INPUT_FIELDS_INVALID`; otherwise any approved field value that is not a JSON string -> `P05_E_DISPATCH_INPUT_INVALID`.
- For every authoritative persona, `manifest.json` parses and requires `schema_version == "prepare_manifest_v1"` and `stage == "01_prepare"`; if either required literal is missing, non-string, or wrong, fail with `P05_E_MANIFEST_INVALID`.
- For every authoritative persona, `manifest.persona_id` mapping is canonical: missing or non-string `manifest.persona_id` -> `P05_E_MANIFEST_INVALID`; present string `manifest.persona_id` unequal to `<persona_id>` -> `P05_E_MANIFEST_PERSONA_ID_MISMATCH`.
- For every authoritative persona, required artifact presence check verifies all five files exist:
  - dispatch_input_v1.json
  - agents.md
  - prompt.txt
  - manifest.json
  - hashes.json
  - any missing file maps to the canonical code above for that artifact
- For every authoritative persona, `prepare-required-artifacts` path failures are mapped explicitly:
  - path-resolution or persona-root-relative validation failure -> `P05_E_REQUIRED_ARTIFACT_PATH_INVALID`
  - unreadable required artifact -> `P05_E_REQUIRED_ARTIFACT_UNREADABLE`
  - missing dispatch_input_v1.json -> `P05_E_REQUIRED_ARTIFACT_MISSING_DISPATCH_INPUT`
  - missing agents.md -> `P05_E_REQUIRED_ARTIFACT_MISSING_AGENTS`
  - missing prompt.txt -> `P05_E_REQUIRED_ARTIFACT_MISSING_PROMPT`
  - missing manifest.json -> `P05_E_REQUIRED_ARTIFACT_MISSING_MANIFEST`
  - missing hashes.json -> `P05_E_REQUIRED_ARTIFACT_MISSING_HASHES`
- For every authoritative persona, `prepare-required-artifacts` emits exactly one persona row and selects its canonical error by first-match ordered traversal over `dispatch_input_v1.json` -> `agents.md` -> `prompt.txt` -> `manifest.json` -> `hashes.json`, with per-artifact condition order path-invalid -> missing -> unreadable.
- For every authoritative persona, `hashes.json` must parse as a top-level JSON object whose exact key set is `dispatch_input_sha256`, `agents_sha256`, `prompt_sha256`, and `bundle_sha256`, with no extra keys and string values only.
- For every authoritative persona, hash/bundle failures map explicitly:
  - fresh open/read failure on `hashes.json` or `manifest.json` during check 4 after check 2 had already established readability -> `P05_E_HASH_EVIDENCE_UNREADABLE`
  - malformed `hashes.json`, non-object top-level shape, non-string required values, or any extra key -> `P05_E_HASH_LEDGER_INVALID`
  - missing required hash keys -> `P05_E_HASH_LEDGER_KEY_MISSING`
  - invalid hash format -> `P05_E_HASH_FORMAT_INVALID`
  - per-file hash mismatch -> `P05_E_HASH_MISMATCH`
  - new unreadable payload encountered during check 4 after check 2 had already established readability -> `P05_E_HASH_PAYLOAD_UNREADABLE`
  - bundle mismatch -> `P05_E_BUNDLE_MISMATCH`
- If `prepare-required-artifacts` already failed a persona on `manifest.json`, `hashes.json`, `dispatch_input_v1.json`, `agents.md`, or `prompt.txt` path-invalid/missing/unreadable grounds, `prepare-hash-and-bundle` still emits that persona row as `status: fail` with no `error_code` and an `error_message` that points back to `prepare-required-artifacts`.
- For every authoritative persona, `prepare-hash-and-bundle` emits exactly one persona row and selects its canonical content-level error by the documented ordered evaluation; later simultaneously true conditions do not replace the first matching code.
- For every authoritative persona, manifest has exactly three entries with exact path/hash_ref/size_bytes triplets in canonical order:
  - dispatch_input_v1.json -> dispatch_input_sha256
  - agents.md -> agents_sha256
  - prompt.txt -> prompt_sha256
- For every authoritative persona, recomputed per-artifact hashes and bundle_sha256 match ledger values.
- Bundle hashing uses canonical artifact bytes and framing `uint64_be(len(payload_bytes))+payload_bytes` in order: `dispatch_input_v1.json`, `agents.md`, `prompt.txt`.
- Prepare layout checks confirm canonical prepare directories for all authoritative persona ids discovered from `<run_root>/01_prepare/personas`.
- Any non-directory entry under `<run_root>/01_prepare/personas` is a deterministic hard failure recorded on the `prepare-persona-discovery` check itself with `P05_E_PERSONA_ENTRY_NON_DIRECTORY`; `persona_results` remains limited to authoritative persona directories and never includes a synthetic row for the offending entry.
- Zero authoritative persona directories under `<run_root>/01_prepare/personas` is a deterministic hard failure recorded on the `prepare-persona-discovery` check itself with `P05_E_PERSONA_SET_EMPTY`.
- `prepare-persona-discovery` chooses exactly one winning check-level code by precedence `P05_E_PERSONA_IO_ERROR` -> `P05_E_PERSONA_ENTRY_NON_DIRECTORY` -> `P05_E_PERSONA_SET_EMPTY`.
- If authoritative persona directories are established and `prepare-persona-discovery` still fails only at check level, its `persona_results` still emits one lexicographically sorted `status: pass` row per authoritative persona with no error fields.
- Required audit path checks confirm writability of pre-existing P03-owned `<run_root>/audit/events.jsonl` and `<run_root>/audit/errors.log` files without mutating those files.
- P03 owns materializing `<run_root>/audit/events.jsonl` and `<run_root>/audit/errors.log` as part of the canonical run-layout dependency; P05 only validates that those files already exist and are writable.
- Missing audit files and present-but-not-writable audit files both map to `P05_E_AUDIT_PATH_NOT_WRITABLE`.
- P05 runtime writes are limited to `<run_root>/01_prepare/prepare_gate_status_v1.json`; it does not mutate pre-existing prepare or audit artifacts.
- `schemas/prepare_gate_status_v1.json` validates the structural contract of `01_prepare/prepare_gate_status_v1.json`: exact top-level keys/literals, exact five ordered `check_id` entries, exact check/persona-result object shapes, boolean `can_proceed_to_stage2`, and `prepare-audit-writability` with `persona_results: []`.
- Prepare status artifact checks verify shape/content at `<run_root>/01_prepare/prepare_gate_status_v1.json`:
  - keys are exactly: schema_version, stage, status, can_proceed_to_stage2, persona_ids, checks.
  - `checks` includes the five exact check IDs in the defined order.
  - `checks` always has five entries, one for each check id.
  - every check executes to completion and does not short-circuit execution.
  - check-object status derivation is deterministic:
    - `prepare-persona-discovery` and `prepare-audit-writability` fail only from their own check-level error fields.
    - `prepare-required-artifacts`, `prepare-dispatch-input-schema`, and `prepare-hash-and-bundle` fail iff at least one emitted persona row is `status: fail`.
    - back-reference rows in `prepare-dispatch-input-schema` and `prepare-hash-and-bundle` still count as failing persona rows even when they carry no local `error_code`.
    - if `prepare-persona-discovery` establishes no authoritative persona, checks 2-4 still emit with `persona_results: []`, no synthetic check-level error, and `status: pass`.
  - sorted `persona_ids` and sorted `persona_results`.
  - `prepare-audit-writability` has `persona_results: []`.
  - `prepare-persona-discovery` uses check-level errors for non-authoritative failures and emits `persona_results: []` when no authoritative persona is established.
  - `prepare-required-artifacts`, `prepare-dispatch-input-schema`, and `prepare-hash-and-bundle` emit one `persona_results` entry per authoritative persona in lexicographic order.
  - top-level status and can_proceed_to_stage2 are mutually consistent.
  - per-check/per-persona outcomes map to canonical error codes.
- On any failed mandatory check, the gate status is fail, can_proceed_to_stage2 is false, and P07 is blocked.

# Handoff

- P07 is allowed to run only when can_proceed_to_stage2 == true in `<run_root>/01_prepare/prepare_gate_status_v1.json`.
- P06 remains non-blocking and advisory; it does not alter or defer hard-gate outcomes.
