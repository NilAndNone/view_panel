---
plan_id: P01
title: Codex Runtime Contract
status: ready
depends_on: []
consumes:
  - docs/plans/tech-plan-runtime/README.md
produces:
  - docs/operations/codex-runtime-contract.md
  - third_party/codex-protocol/
completion_evidence:
  - codex_cli_version_pinned
  - app_server_transport_fixed_to_stdio
  - schema_entrypoints_and_outputs_defined
  - smoke_lifecycle_contract_defined
  - launcher_and_auth_preconditions_defined
  - mismatch_policy_and_enforcement_defined
---

# Goal

Define the canonical Codex runtime contract used by all v1.1 runtime stages.

# Scope

- Pin one supported "@openai/codex" version for the runtime.
- Lock the app-server transport to the one canonical launcher form:
  `codex app-server --listen stdio://`
  as the only supported launch mode.
- Define the Codex protocol bundle location and concrete live-captured entrypoints required by downstream plans.
- Define one supported authentication precondition and its failure behavior.
- Define the exact smoke lifecycle contract required by P02 and P08 before any worker execution.
- Define explicit schema/protocol mismatch policy, including who enforces it.

# Out Of Scope

- Runtime CLI argument parsing, flag defaults, config merge order, and config schema validation.
- Stage-specific runner, lifecycle, or orchestration implementation.
- Render logic and content/persona acceptance behavior.

# Required Inputs

- docs/plans/tech-plan-runtime/README.md

# Implementation Tasks

- Create docs/operations/codex-runtime-contract.md.
- Record exactly one pinned CLI version for @openai/codex (major/minor/patch).
- Record the canonical protocol-launcher form:
  `codex app-server --listen stdio://`
  as the only supported launcher base prefix; runtime may append explicit suffix args after that base form, but must not change the prefix order or transport.
- Record live-captured protocol artifacts in versioned locations under `third_party/codex-protocol/<pinned_version>/`:
  - `openapi.json` (CLI-reported protocol schema)
  - `messages.json` (generated message/event/type index for method compatibility checks)
  - `schema-index.json` (artifact manifest with file hashes and generation timestamp)
  - `smoke-transcript.jsonl` (filtered minimal live smoke transcript captured from the observed transport path)
- Record one supported auth precondition:
  a valid non-interactive login session produced by `codex login` in the same user context and reused by `codex app-server`; no per-command API-key override mode is supported.
- Record and standardize the smoke contract used by P02/P08:
  1. Start process with canonical launcher above.
  2. Send JSON-RPC `initialize` and record the first matching response from the observed capture transport.
  3. Send `initialized` notification.
  4. Send `configRequirements/read` and record the first matching response.
  5. Send `thread/start`, capture `threadId`.
  6. Send `turn/start` with a minimal test message and require a terminal `item/completed.agentMessage` event to parse.
  7. The retained checked-in transcript may stop at that terminal event; graceful shutdown and exit code `0` are not currently required capture evidence.
- Record explicit contract mismatch policy:
  - If pinned version, launcher form, auth precondition, or schema bundle is missing/mismatched, execution is blocked.
  - If schema/protocol drift is detected (missing/changed JSON-RPC methods/events versus `messages.json`), fail closed.
  - Enforcer ownership:
    - P00 validates startup-input prerequisites in its own CLI/config/schema scope before runtime invocation.
    - P01 defines the pinned launcher, auth, and smoke prerequisite contract consumed downstream.
    - P02 consumes both P00 startup-input validation and this runtime contract, performs live handshake validation, owns framed-transport support plus sequential fallback, and aborts worker execution on drift.
    - P08 consumes only outputs guaranteed by the smoke contract and does not redefine compatibility checks.

# Acceptance Checks

- Contract document names one (and only one) supported CLI version.
- Contract document declares the single canonical launcher `codex app-server --listen stdio://` and prohibits alternatives.
- Contract document allows only suffix arguments appended after that canonical launcher prefix.
- Protocol schema output directory and concrete versioned entrypoints are fixed to `third_party/codex-protocol/<pinned_version>/`:
  - `openapi.json`
  - `messages.json`
  - `schema-index.json`
  - `smoke-transcript.jsonl`
- Contract document defines one supported auth precondition and its required failure behavior.
- Contract includes a concrete smoke lifecycle path with required RPC ordering and successful completion of `item/completed.agentMessage`, while allowing the retained capture transcript to stop before graceful shutdown evidence.
- Mismatch policy is explicit, including drift handling and explicit ownership split across P00, P01, and P02.

# Handoff

P02 consumes this contract alongside P00 startup-input validation, and P08 consumes this contract for transport, schema, and smoke assumptions.
