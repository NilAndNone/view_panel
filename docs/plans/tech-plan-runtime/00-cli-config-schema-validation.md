---
plan_id: P00
title: CLI + Config + Schema Validation
status: ready
depends_on: []
consumes:
  - docs/superpowers/specs/2026-04-12-tech-plan-runtime-consolidation-design.md
  - docs/tech-plan-v1.md
produces:
  - cmd/worldview-panel/main.go
  - internal/config/config.go
  - internal/schema/validate.go
completion_evidence:
  - startup_entrypoint_and_defaulting_defined
  - startup_validation_entrypoints_defined
  - startup_validation_failure_blocks_app_server_start
---

## Goal
Create the runtime entrypoint for worldview-panel that parses startup inputs, applies deterministic defaults, and enforces startup schema validation before any app-server lifecycle can start.

## Scope
- Own startup command parsing and input normalization in `cmd/worldview-panel/main.go`.
- Own startup configuration loading and defaulting in `internal/config/config.go`.
- Own startup validation entrypoints in `internal/schema/validate.go`.
- P00 is the product-entry plan for the retained runtime and owns the operator-facing startup contract.
- Full-flow smoke remains out of scope here; terminal end-to-end validation belongs to `P12`.
- Define one explicit startup contract for downstream handoff:
  `StartupConfig { question: string, materials: []string, persona_set: string, outdir: string, concurrency: int, model: string }`
- Restrict startup validation ownership to P00 for the following checks:
  - required/blank checks
  - path existence and path-kind checks for startup path-like inputs
  - deterministic duplicate detection on startup list inputs
  - numeric range checks for `concurrency`
- P00-owned required-vs-default matrix (startup inputs):
  - `question`: required, no default, must be non-blank after trimming.
  - `materials`: required, no default, must be a deterministic list; each entry is path-like and must resolve as an existing file or directory.
  - `persona_set` (CLI: `--persona-set`): required, no default, non-blank symbolic startup identifier (do not treat as a filesystem path).
  - `outdir`: optional, default `.`, normalize to absolute path, then create if missing.
  - `concurrency`: optional, default `1`, must be `>= 1`.
  - `model`: optional, default `gpt-5.3-codex-spark` (literal, deterministic, owned by P00), must be non-blank when explicitly provided.
- P00 validates exactly these path-like startup inputs:
  - `materials` entries
  - `outdir`
- `materials` acceptance must be explicit:
  - each entry may be a file, a directory, or both acceptable entry kinds in scope.
  - duplicate detection for startup list inputs compares canonicalized paths (`filepath.Clean` + absolute/path resolution) and errors when canonical entries repeat.
- `outdir` concrete semantics are fixed in P00:
  - trim and canonicalize (`filepath.Clean` + absolute resolution),
  - create directory if absent,
  - preserve the canonicalized directory as the `outdir` parent handed to `P03`; do not append `runs/` or `<run_id>` in P00,
  - reject if resolved value is not a directory.
- Treat startup validation as a hard stop that prevents app-server lifecycle startup on any failure.
- Keep startup validation scoped to P00 and do not validate `dispatch_input_v1.json`, canonical prepare artifacts, hard-gate outputs, or Stage 2 execution contracts owned by P04/P05/P07+.

## Out Of Scope
- App-server transport, handshake, lifecycle, or completion handling (`P02`).
- Run layout, artifact writer, hash primitives, Stage 1 assembly, hard gate, and run sealing ownership (`P03`, `P04`, `P05`, `P06`).
- Stage 2 soft/hard execution logic and render chain logic.

## Required Inputs
- docs/superpowers/specs/2026-04-12-tech-plan-runtime-consolidation-design.md
- docs/tech-plan-v1.md

## Implementation Tasks
1. In `cmd/worldview-panel/main.go`, define deterministic startup ordering:
  - Parse exactly the six startup inputs: `question`, `materials`, `persona-set`, `outdir`, `concurrency`, `model`.
  - Apply explicit required-vs-default logic from the matrix above.
  - Normalize values before validation (trim strings, deterministic list splitting for `materials`, canonicalize path-like inputs).
  - Call startup validation entrypoints and abort with non-zero exit when validation fails.
  - Ensure app-server startup is unreachable after startup validation failure.
2. In `internal/config/config.go`, define startup config behavior:
  - Add `StartupConfig` typed structure with exactly the six fields above.
  - Resolve defaults for optional fields in one place and one pass.
  - Normalize `materials` deterministically before validation (split order, trimming, duplicate handling rule).
  - Return one canonical validated object for downstream handoff.
3. In `internal/schema/validate.go`, define startup schema/validator entrypoints:
  - Add a startup-specific validation function used by `main.go`.
  - Keep validation scoped to startup contract only; do not add artifact/dispatch validation.
  - Return stable, machine-readable validation diagnostics (code + field path + message).
4. Add explicit validation test planning (no test execution in this plan):
  - Test cases for config parsing and normalization edge cases: empty string, whitespace-only, duplicate startup list entries, canonical path behavior.
  - Required-flag matrix tests: missing required fields and invalid `concurrency`.
  - Path checks for startup path-like fields: existence and path-kind behavior.
  - Startup acceptance matrix for contract shape/range/path checks and canonical error outputs.
5. Publish the startup contract contractually:
  - Define that downstream `P02` consumes only the canonical validated `StartupConfig`.
  - Record no alternate startup contract or duplicate validator in follow-on plans.

## Acceptance Checks
- `cmd/worldview-panel/main.go` and `internal/schema/validate.go` produce one canonical startup validation result path for CLI input (required-vs-default matrix + typed diagnostics), not multiple validation branches.
- Downstream plans consume the canonical `StartupConfig` and do not re-run startup parsing or path checks in P00-owned scope.
- Startup hard-stop is observable: for each failure case, startup returns non-zero, emits a machine-readable startup diagnostic, and never invokes app-server startup entrypoint logic (mocked launch/spawn assertion).
- Concrete pass/fail examples:
  - Invalid `concurrency`:
    - Fail: `--concurrency=-1` => explicit `concurrency must be >= 1`.
    - Fail: `--concurrency=0` => explicit `concurrency must be >= 1`.
    - Pass: `--concurrency=4` => normalized as `4`.
  - Missing required startup inputs:
    - Fail: missing `--question` => required-field error for `question`.
    - Fail: missing `--materials` => required-field error for `materials`.
    - Fail: missing `--persona-set` => required-field error for `persona_set`.
  - Missing/nonexistent paths:
    - Fail: `--persona-set=` => required symbolic identifier check.
    - Fail: `--materials=/tmp/missing-materials.md` => explicit missing-path error for `materials`.
    - Fail: `--outdir=/tmp/existing-file` where existing-file is not a directory => explicit outdir-kind error.
  - Empty/blank startup inputs:
    - Fail: `--question=""` => `question` is required and non-blank.
    - Fail: `--question="  "` => trim-to-empty rejection.
  - Model default:
    - Pass: omit `--model` => canonical startup config uses `gpt-5.3-codex-spark`.
  - Duplicate startup list:
    - Fail: `--materials=a.md,a.md` => duplicate-material error with stable diagnostic code/path when canonicalized duplicates remain.
  - Outdir behavior:
    - Pass: `--outdir=/tmp/run-output-root` (non-existent directory) => created and canonicalized; downstream P03 run root is `/tmp/run-output-root/runs/<run_id>`.
    - Pass: omit `--outdir` => default `.` canonicalized; downstream P03 run root is `<cwd>/runs/<run_id>`.
- `internal/schema/validate.go` is the single canonical startup schema validator for CLI/config startup shape.

## Handoff
- Publish startup `StartupConfig` and startup validation behavior for `P02` handoff only.
- Publish a clear split that P00 validates startup entry only and does not perform Stage 1/Stage 2 or hard-gate style validations.
