# Codex Runtime Contract

## Status

This runtime contract is pinned and checked in for the bootstrap slice. The bundled protocol artifacts under `third_party/codex-protocol/0.0.0/` are explicit checked-in placeholders, not live-generated captures.

## Pinned CLI Contract

- Supported package: `@openai/codex`
- Supported pinned version: `0.0.0`
- Observed local CLI version on April 11, 2026: `codex-cli 0.0.0`
- Canonical launcher base: `codex app-server --listen stdio://`
- Allowed launcher suffixes: startup may append additional arguments after that canonical base and transport form, for example `--model ...`
- Unsupported launcher forms: any `codex app-server` argument order or transport mode that changes the canonical base prefix above

## Authentication Precondition

The only supported authentication precondition is a valid non-interactive login session established with `codex login` in the same user context that later launches `codex app-server`.

Unsupported modes:
- Per-command API key overrides
- Alternate auth injection strategies defined outside the shared user login session
- Interactive login prompts during runtime execution

Failure policy:
- If the shared login session is missing or invalid, runtime execution must fail closed before worker execution begins.

## Versioned Protocol Bundle

Pinned bundle root:
- `third_party/codex-protocol/0.0.0/openapi.json`
- `third_party/codex-protocol/0.0.0/messages.json`
- `third_party/codex-protocol/0.0.0/schema-index.json`
- `third_party/codex-protocol/0.0.0/smoke-transcript.jsonl`

Bootstrap-slice note:
- The repository does not yet contain live-generated Codex protocol exports.
- The checked-in bundle is intentionally explicit placeholder data so downstream code can pin paths and fail closed on mismatch.
- Regeneration later must replace placeholder payloads in place under the same versioned directory or advance the pinned version everywhere together.
- Startup must resolve the pinned bundle with a repo-root-aware path. The local checker does not assume the current working directory when `third_party/codex-protocol/<version>/` is not supplied explicitly.

## Smoke Lifecycle Contract

The required smoke lifecycle is fixed to this order:
1. Launch `codex app-server --listen stdio://`.
2. Send JSON-RPC `initialize` and require a successful response with protocol metadata.
3. Send `initialized` notification.
4. Send `configRequirements/read` and require that required config requirements are satisfiable.
5. Send `thread/start` and capture `threadId`.
6. Send `turn/start` with a minimal test message and require a terminal `item/completed.agentMessage` event.
7. Request graceful shutdown and require process exit code `0` on the happy path.

## Startup Enforcement Split

Startup writes one local runtime-contract artifact before any stage work begins:

- `runs/<run_id>/audit/runtime_contract_status.json`

That artifact is the startup guard result for the current run.

### Blocking startup checks

Any failure in these checks must set `status == "blocked"` and stop the run before worker execution:

- `codex` executable availability
- `codex --version` observability
- observed-versus-pinned CLI version equality
- presence of the repo-root-resolved versioned protocol bundle path
- completeness of required bundle files:
  - `openapi.json`
  - `messages.json`
  - `schema-index.json`
  - `smoke-transcript.jsonl`
- launcher compatibility with canonical base `codex app-server --listen stdio://`

These failures are recorded in `blocking_errors`.

### Warning-only facts

These facts do not block startup by themselves. They set `status == "warning"` when no blocking check has failed:

- the checked-in protocol bundle is still placeholder content
- transport framing is not strongly verified beyond the current sequential-JSON stdout assumption

These facts are recorded in `warnings`.

## Mismatch Policy

Execution must fail closed when any blocking startup check drifts from the pinned contract. Warning-only facts remain visible in the audit artifact but do not stop the run by themselves.

Ownership split:
- P00 owns startup CLI/config/schema validation.
- P01 owns the pinned CLI version, canonical launcher, auth precondition, and protocol bundle location.
- P02 later consumes P00 and P01, performs live handshake/runtime compatibility checks, and aborts execution on drift.
- Later stage plans must consume this contract and must not redefine transport, launcher, or static protocol pinning rules.
