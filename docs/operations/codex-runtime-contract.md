# Codex Runtime Contract

## Status

This runtime contract is pinned and checked in for the bootstrap slice. The bundled protocol artifacts under `third_party/codex-protocol/0.0.0/` are explicit checked-in placeholders, not live-generated captures.

## Pinned CLI Contract

- Supported package: `@openai/codex`
- Supported pinned version: `0.0.0`
- Observed local CLI version on April 11, 2026: `codex-cli 0.0.0`
- Canonical launcher: `codex app-server --listen stdio://`
- Unsupported launcher forms: every other `codex app-server` argument order, transport flag, or alternate transport mode

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

## Smoke Lifecycle Contract

The required smoke lifecycle is fixed to this order:
1. Launch `codex app-server --listen stdio://`.
2. Send JSON-RPC `initialize` and require a successful response with protocol metadata.
3. Send `initialized` notification.
4. Send `configRequirements/read` and require that required config requirements are satisfiable.
5. Send `thread/start` and capture `threadId`.
6. Send `turn/start` with a minimal test message and require a terminal `item/completed.agentMessage` event.
7. Request graceful shutdown and require process exit code `0` on the happy path.

## Mismatch Policy

Execution must fail closed when any of the following drift from the pinned contract:
- Pinned CLI version
- Canonical launcher form
- Authentication precondition
- Versioned protocol bundle presence
- Required JSON-RPC methods or terminal event shape recorded in `messages.json`

Ownership split:
- P00 owns startup CLI/config/schema validation.
- P01 owns the pinned CLI version, canonical launcher, auth precondition, and protocol bundle location.
- P02 later consumes P00 and P01, performs live handshake/runtime compatibility checks, and aborts execution on drift.
- Later stage plans must consume this contract and must not redefine transport, launcher, or static protocol pinning rules.
