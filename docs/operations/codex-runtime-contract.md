# Codex Runtime Contract

## Status

This runtime contract is pinned and checked in for the current prototype slice. The bundled protocol artifacts under `third_party/codex-protocol/0.0.0/` are live captures from the pinned CLI, not placeholder stubs.

## Pinned CLI Contract

- Supported package: `@openai/codex`
- Supported pinned version: `0.0.0`
- Observed local CLI version on April 11, 2026: `codex-cli 0.0.0`
- Canonical launcher base: `codex app-server --listen stdio://`
- Allowed launcher suffixes: startup may append additional arguments after that canonical base and transport form, for example `--model ...`
- Unsupported launcher forms: any `codex app-server` argument order or transport mode that changes the canonical base prefix above

## Transport Decode Contract

Runtime decode support is owned by P02 in `internal/appserver/`.

- stdout is probed after leading transport whitespace
- if the observed stream prefix starts with `Content-Length:` (case-insensitive), the runtime decodes framed stdio
- otherwise the runtime falls back to sequential JSON decoding
- stderr remains diagnostics-only and is never parsed as protocol traffic

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

Current capture contract:
- `openapi.json` stores the aggregate schema bundle captured from `codex app-server generate-json-schema --out <temp>`, under the repository's legacy filename.
- `messages.json` stores a live-derived message index extracted from the generated schema bundle.
- `schema-index.json` stores live-capture metadata plus SHA-256 hashes for `openapi.json`, `messages.json`, and `smoke-transcript.jsonl`.
- `smoke-transcript.jsonl` stores a filtered minimal live smoke transcript over the currently observed fallback sequential-JSON stdio transport. It records the handshake through terminal agent-message completion, not every intermediate notification or the shutdown tail. This transcript is evidence for the observed capture path, not a claim that framed stdio is unsupported.
- The capture entrypoint is `make capture-protocol`, which mirrors the repo into a local lock-capable directory, runs `go run ./tools/capture_protocol_bundle`, and copies the refreshed bundle back into the checked-in tree.
- Startup must resolve the pinned bundle with a repo-root-aware path. The local checker does not assume the current working directory when `third_party/codex-protocol/<version>/` is not supplied explicitly.

## Smoke Capture Contract

The current checked-in smoke bundle is intentionally narrower than a full operational smoke test. The capture tool does this today:
1. Launch `codex app-server --listen stdio://`.
2. Send JSON-RPC `initialize` over the currently observed sequential-JSON capture path and record the first matching response.
3. Send `initialized` notification.
4. Send `configRequirements/read` and record the first matching response.
5. Send `thread/start` and require a matching response that contains `result.thread.id`.
6. Send `turn/start` with a minimal test message and require a terminal `item/completed` event whose `params.item.type == "agentMessage"` and `params.item.phase == "final_answer"`.

Checked-in transcript note:
- the checked-in `smoke-transcript.jsonl` is the filtered minimal evidence artifact for steps `1` through `6`
- the runtime client itself supports framed `Content-Length` transport plus sequential JSON fallback; the checked-in transcript only proves the currently observed capture path
- the capture tool does not currently assert initialize-response payload details beyond matching the request id
- the capture tool does not currently assert `configRequirements/read` semantics beyond receiving a matching response
- the capture tool does not currently persist every intermediate notification
- the capture tool does not currently attempt graceful shutdown or assert process exit code `0`

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
- live-bundle validation:
  - `schema-index.json`, `messages.json`, and `smoke-transcript.jsonl` metadata version must match the runtime checker pinned version
  - `schema-index.json` must be marked as `generated_from_live_cli == true`
  - `schema-index.json` hashes for `openapi.json`, `messages.json`, and `smoke-transcript.jsonl` must match the checked-in files
  - `messages.json` must include the required handshake methods and terminal-completion metadata
  - `smoke-transcript.jsonl` must include the minimal handshake requests and a terminal agent-message completion event
- launcher compatibility with canonical base `codex app-server --listen stdio://`

These failures are recorded in `blocking_errors`.

### Warning-only facts

These facts do not block startup by themselves. They set `status == "warning"` when no blocking check has failed:

- the configured protocol bundle looks like placeholder content, even though the required files exist and passed the blocking bundle checks
- startup has not independently marked transport framing as verified, even though the runtime decoder can handle framed stdio and sequential JSON fallback

These facts are recorded in `warnings`.

## Mismatch Policy

Execution must fail closed when any blocking startup check drifts from the pinned contract. Warning-only facts remain visible in the audit artifact but do not stop the run by themselves.

Ownership split:
- P00 owns startup CLI/config/schema validation.
- P01 owns the pinned CLI version, canonical launcher, auth precondition, and protocol bundle location.
- P02 later consumes P00 and P01, performs live handshake/runtime compatibility checks, owns framed-transport support plus sequential fallback in the runtime layer, and aborts execution on drift.
- Later stage plans must consume this contract and must not redefine transport, launcher, or static protocol pinning rules.
