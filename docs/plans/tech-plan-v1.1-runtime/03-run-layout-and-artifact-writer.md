---
plan_id: P03
title: Run Layout + Artifact Writer
status: ready
depends_on: []
consumes:
  - docs/superpowers/specs/2026-04-11-tech-plan-v1-1-runtime-redesign-design.md
  - docs/tech-plan-v1.md
produces:
  - internal/storage/layout.go
  - internal/hash/hash.go
completion_evidence:
  - run_directory_layout_fixed
  - shared_writer_helpers_defined
  - sha256_primitives_available
---

# Goal

Create a deterministic, stage-agnostic filesystem foundation so all downstream runtime stages share one stable `run_root` contract. `P03` is an independent root plan (no stage dependencies) that enables `P04`, `P05`, `P07`, and `P10` without owning their stage-specific logic.

# Scope

- Define canonical `run_root` layout for a run under `outdir`, including all runtime areas and persona-aware subpaths:
  - `runs/<run_id>/request/...`
  - `runs/<run_id>/01_prepare/...` (request->prepare transition point)
  - `runs/<run_id>/02_answer/...` (stage 2 artifacts only)
  - `runs/<run_id>/03_render/...` (render input and outputs)
  - `runs/<run_id>/audit/...` (audit evidence and shared logs)
- Freeze the approved subtree shapes that downstream logic must use:
  - `01_prepare` persona scope: `runs/<run_id>/01_prepare/personas/<persona_id>/<artifact_name>`
  - `02_answer` persona scope: `runs/<run_id>/02_answer/personas/<persona_id>/<artifact_name>`
- Include required writable audit/log paths under audit root so downstream checks do not define them independently:
  - `runs/<run_id>/audit/events.jsonl`
  - `runs/<run_id>/audit/errors.log`
- Make canonical audit file stub materialization part of the run layout contract:
  - `internal/storage/layout.go` owns creation of `runs/<run_id>/audit/events.jsonl` and `runs/<run_id>/audit/errors.log`.
  - `EnsureRunLayout(runRoot)` MUST create those files if absent as empty regular files and MUST NOT truncate them if they already exist.
  - Downstream stages must treat those files as pre-existing layout artifacts, not files they create on demand.
- Make directory materialization ownership explicit and exclusive:
  - `internal/storage/layout.go` owns all run-stage directory creation.
  - Stage code must not call `os.MkdirAll` under `runs/<run_id>` directly.
  - The package-level contract is:
    - `EnsureRunLayout(runRoot)` is the explicit public API for staging all canonical directories and the canonical audit file stubs.
    - Shared writers (`WriteJSON`, `WriteText`, `AppendJSONL`) MUST call `EnsureRunLayout` before writing, so callers may safely rely on writers alone and still inherit the audit stub guarantee.
  - Callers that invoke `EnsureRunLayout` themselves must do so before any artifact write.
- Add shared path helpers under `internal/storage/layout.go` that enforce exact shape and boundary behavior:
  - `RunRoot(outdir, runID)`
  - `RequestDir(runRoot)`
  - `PrepareRoot(runRoot)`
  - `AnswerRoot(runRoot)`
  - `RenderRoot(runRoot)`
  - `AuditRoot(runRoot)`
  - `PreparePersonaDir(runRoot, personaID)`
  - `AnswerPersonaDir(runRoot, personaID)`
  - `PreparePersonaArtifactPath(runRoot, personaID, artifactName) (string, error)`, an internal wrapper around `ResolvePath(PreparePersonaDir(runRoot, personaID), artifactName)` that returns an error for invalid `artifactName` inputs
  - `AnswerPersonaArtifactPath(runRoot, personaID, artifactName) (string, error)`, an internal wrapper around `ResolvePath(AnswerPersonaDir(runRoot, personaID), artifactName)` that returns an error for invalid `artifactName` inputs
  - `RequestLogPath(runRoot)`
  - Every request-relative artifact path other than `RequestLogPath(runRoot)` is built as `ResolvePath(RequestDir(runRoot), rel)`, and inherits the same validation.
  - `AuditEventsPath(runRoot)`
  - `AuditErrorsPath(runRoot)`
  - `ResolvePath(base, rel string)` for normalized, boundary-checked joins under `base`.
- Define canonical artifact writer primitives under `internal/storage/layout.go`:
  - atomic JSON write with canonical key-order marshal
  - atomic text write
  - JSONL append writer with one-object-per-line convention and fixed newline policy
- Define SHA-256 primitives under `internal/hash/hash.go`:
  - canonical payload hash for JSON payloads
  - file hash helpers
  - byte-stable helper set for cross-stage integrity checks
- Lock serialization and hashing contract consumed by all downstream stages:
  - all stage artifact paths must be constructed via helpers from this package
  - all stage writers must go through shared canonical write helpers
  - `SHA256HexJSON(value)` MUST hash the same byte sequence emitted by `WriteJSON(value)` (before persistence).
- Keep this plan responsible for layout + writer primitives only; do not define gate policy, prompt assembly, batch orchestration, audit normalization, or stage-specific artifact semantics.

# Out Of Scope

- Dispatch/prepare semantic validation logic (`P04`/`P05`)
- Stage 2 workspace sealing and runtime input chaining (`P07`)
- Render aggregation semantics and gating logic (`P10`)
- Audit-event normalization, model review, or decision outcomes
- Any stage-specific schema contract not directly tied to stable path + writer primitives

# Required Inputs

- `docs/superpowers/specs/2026-04-11-tech-plan-v1-1-runtime-redesign-design.md`
- `docs/tech-plan-v1.md`

# Implementation Tasks

1. Create `internal/storage/layout.go` with canonical directory constants and run layout helpers:
  - `RunRoot(outdir, runID)` and run roots for `request`, `01_prepare`, `02_answer`, `03_render`, and `audit`.
  - `RequestDir(runRoot)` and `RequestLogPath(runRoot)` for request and request->prepare transition metadata.
  - `PreparePersonaDir(runRoot, personaID)` and `AnswerPersonaDir(runRoot, personaID)` that return exactly:
    - `runs/<run_id>/01_prepare/personas/<persona_id>/`
    - `runs/<run_id>/02_answer/personas/<persona_id>/`
  - `PreparePersonaArtifactPath(runRoot, personaID, artifact)` and `AnswerPersonaArtifactPath(runRoot, personaID, artifact)` as wrapper APIs that delegate to `ResolvePath` with `artifact` as the relative path argument and return an error on invalid relative artifact paths.
  - `RequestLogPath(runRoot)`, `AuditEventsPath(runRoot)`, and `AuditErrorsPath(runRoot)` for writable canonical log/artifacts.
  - `ResolvePath` helper that normalizes and validates paths under an explicit base directory, rejecting traversal and symlink-escape attempts.
    - It resolves as far as existing path segments permit under `base`, rejects any existing segment that is or links outside `base`, and then lexically joins the final leaf and remaining segments under the last validated parent so missing intermediate directories are not implicitly trusted.
2. Add shared directory materialization APIs in `internal/storage/layout.go`:
   - `EnsureRunLayout(runRoot)` creates `request`, `01_prepare`, `02_answer`, `03_render`, and `audit` roots.
   - `EnsureRunLayout(runRoot)` also materializes `AuditEventsPath(runRoot)` and `AuditErrorsPath(runRoot)` as empty files when absent, without truncating existing contents.
   - Stage code must treat this as the only allowed path materializer for filesystem layout.
3. Add shared canonical writer helpers in `internal/storage/layout.go`:
  - `WriteJSON(path, value)`:
     - compute canonical JSON bytes through the same internal marshaler used by `SHA256HexJSON`,
     - write atomically via temp file replace,
     - write no extra whitespace or terminator.
   - `WriteText(path, content)` using atomic write and ensuring parents via `EnsureRunLayout`.
   - `AppendJSONL(path, value)` with line atomicity by file lock-free append, one canonical JSON object per line, and one trailing `\n` per object.
   - All shared writers must preserve the `EnsureRunLayout(runRoot)` contract so writer-only callers still observe pre-existing `audit/events.jsonl` and `audit/errors.log` stubs before downstream gates execute.
4. Add `internal/hash/hash.go` with reusable SHA-256 primitives:
   - `SHA256Hex(data []byte) string`
   - `SHA256HexFile(path string) (string, error)`
   - `SHA256HexJSON(value any) (string, error)` using the exact canonical JSON bytes expected by `WriteJSON`.
5. Add package-level contract documentation (comments + public surface) that locks the shared path/writer/hash APIs and explicitly forbids each stage from inventing independent filesystem or hash conventions.
# Acceptance Checks

- `internal/storage/layout.go` defines the canonical `run_root` and all required areas: `request`, `01_prepare`, `02_answer`, `03_render`, and `audit`.
- Concrete path matrix is locked:
  - `RunRoot(outdir, run_id) = outdir + "/runs/<run_id>"`.
  - `RequestDir(runRoot) = "<run_root>/request"`.
  - `RequestLogPath(runRoot) = "<run_root>/request/log.jsonl"`.
  - Any request artifact path except `RequestLogPath` is produced by `ResolvePath(RequestDir(runRoot), <request-relative-rel>)`.
  - `PrepareRoot(runRoot) = "<run_root>/01_prepare"`.
  - `AnswerRoot(runRoot) = "<run_root>/02_answer"`.
  - `RenderRoot(runRoot) = "<run_root>/03_render"`.
  - `AuditRoot(runRoot) = "<run_root>/audit"`.
  - `AuditEventsPath(runRoot) = "<run_root>/audit/events.jsonl"`.
  - `AuditErrorsPath(runRoot) = "<run_root>/audit/errors.log"`.
  - `PreparePersonaArtifactPath(runRoot, "<id>", "dispatch_input_v1.json") = "<run_root>/01_prepare/personas/<id>/dispatch_input_v1.json"`.
  - `AnswerPersonaArtifactPath(runRoot, "<id>", "workspace/AGENTS.md") = "<run_root>/02_answer/personas/<id>/workspace/AGENTS.md"`.
  - `AnswerPersonaArtifactPath(runRoot, "<id>", "input/prompt.txt") = "<run_root>/02_answer/personas/<id>/input/prompt.txt"`.
  - `AnswerPersonaArtifactPath(runRoot, "<id>", "outgoing_input.json") = "<run_root>/02_answer/personas/<id>/outgoing_input.json"`.
- Directory materialization and ownership checks:
  - `EnsureRunLayout(runRoot)` creates all stage roots (including persona roots' immediate parent directories).
  - `EnsureRunLayout(runRoot)` materializes `<run_root>/audit/events.jsonl` and `<run_root>/audit/errors.log` as existing regular files, even before any stage emits audit content.
  - If either canonical audit file already exists, `EnsureRunLayout(runRoot)` leaves existing contents intact.
  - All write helpers create missing parents for target artifact paths if needed.
  - All shared writers preserve the same audit stub guarantee by calling `EnsureRunLayout(runRoot)` before writing.
  - No downstream plan may call `os.MkdirAll` for `run_root`-relative paths.
- `ResolvePath` traversal and boundary checks are explicit:
  - Allowed: `ResolvePath("<run_root>/01_prepare/personas/p1", "agents.md")` -> `<run_root>/01_prepare/personas/p1/agents.md`.
  - Rejected: `ResolvePath("<run_root>/01_prepare/personas/p1", "../../outside.txt")`.
  - Rejected: `ResolvePath("<run_root>/02_answer/personas/p1", "../request/../../../../etc/passwd")`.
  - Rejected: any normalized path whose final resolved target is not under `base`.
- JSON canonicalization and hashing evidence:
  - `WriteJSON` writes bytes from the same canonical marshal routine used by `SHA256HexJSON`.
  - `WriteJSON` writes no trailing newline and no pretty-printing/indentation.
  - `SHA256HexJSON(v)` returns `SHA256Hex(canonicalJSON(v))`.
  - For a stable input object, `SHA256HexJSON` output is identical regardless of map key declaration order.
- JSONL behavior is observable:
  - `AppendJSONL(path, v1); AppendJSONL(path, v2)` preserves existing lines and appends two lines.
  - Each line is a canonical JSON object string plus exactly one `\n`.
  - Appending never rewrites prior lines.
- A downstream stage can build all artifact paths from this package only, with no need to hand-construct ad hoc paths.
- Audit/log helper coverage includes writable canonical log paths for stage checks, and the canonical audit files already exist as part of the run layout contract before those checks execute.

# Handoff

- `P04`, `P05`, `P07`, and `P10` consume `internal/storage/layout.go` and `internal/hash/hash.go` as foundational infrastructure.
- This plan must be completed before those stages assume path/hash primitives in their implementations; it remains strictly foundational and does not absorb their stage behavior.
