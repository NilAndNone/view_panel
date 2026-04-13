# view_panel Codex Guide

## Purpose

This repository is no longer a plan-only workspace. It contains an implemented runtime prototype for the worldview panel flow.

Use this file as the repository-level operating guide for Codex.

Human-facing usage lives in `README.md`.

## What The Repository Does

The runtime executes a fixed three-stage flow:

1. `01_prepare`
2. `02_answer`
3. `03_render`

The CLI entrypoint is:

- `cmd/worldview-panel/main.go`

The top-level sequencing helper is:

- `internal/orchestrator/run.go`

## Current State

Treat the repository as:

- implemented
- runnable as a prototype
- not production-ready

Do not treat the root as a pending `tech-plan-v1` execution workspace anymore.

The retained planning reference now lives under:

- `docs/plans/tech-plan-runtime/`

Earlier execution-plan artifacts were pruned. They are no longer part of the repository-level dispatch contract.

## Source Of Truth Order

For the current codebase, use this order:

1. actual code under `cmd/`, `internal/`, `runtime/skills/`, `schemas/`
2. `README.md`
3. `docs/operations/codex-runtime-contract.md`
4. `docs/plans/tech-plan-runtime/`

If code and plan docs disagree, prefer the code, then update the docs deliberately.

## Important Entry Points

Startup:

- `cmd/worldview-panel/main.go`
- `internal/config/config.go`
- `internal/schema/validate.go`

Current startup surface also includes:

- `review_enabled`
- `worker_timeout_ms`
- `max_attempts_per_persona`
- `forbidden_tool_names`

Shared runtime foundations:

- `internal/storage/layout.go`
- `internal/hash/hash.go`

App-server runtime:

- `internal/appserver/messages.go`
- `internal/appserver/client.go`
- `internal/appserver/runner.go`

Prepare chain:

- `internal/materials/loader.go`
- `internal/persona/library.go`
- `internal/stage/prepare/prepare.go`
- `internal/stage/prepare/gate.go`
- `internal/stage/prepare/review.go`

Answer chain:

- `internal/stage/answer/workspace.go`
- `internal/stage/answer/runner.go`
- `internal/stage/answer/batch.go`

Render chain:

- `internal/stage/render/aggregate.go`
- `internal/stage/render/render.go`

## Runtime Contract

Pinned Codex runtime assumptions are documented here:

- `docs/operations/codex-runtime-contract.md`

Startup writes the current-run runtime-contract guard result to:

- `runs/<run_id>/audit/runtime_contract_status.json`

Checked-in protocol bundle:

- `third_party/codex-protocol/0.0.0/`

Current reality:

- the pinned version is based on the locally observed CLI version `codex-cli 0.0.0`
- the protocol bundle is placeholder content
- do not silently “trust” it as a live-generated schema export

If you change the Codex runtime contract, update both:

1. `docs/operations/codex-runtime-contract.md`
2. `third_party/codex-protocol/<version>/`

## Shared Ownership Boundaries

Keep these ownership rules intact:

- `internal/storage` owns run-root layout and canonical write helpers
- `internal/hash` owns canonical hashing helpers
- stage packages must not invent a second path / writer / hash convention
- `P07` sealing may create isolated external workspaces, but run-tree artifacts still belong to `internal/storage`
- `P08` owns single-worker execution only
- `P09` owns batch policy only
- `P10` reads Stage 2 only through `answer_batch.json` and referenced `result.json`
- `P11` reads Stage 3 only through `raw_render_input.json` and `certified_render_input.json`

## Canonical Artifact Chain

Prepare artifacts:

- `runs/<run_id>/01_prepare/personas/<persona_id>/dispatch_input_v1.json`
- `runs/<run_id>/01_prepare/personas/<persona_id>/agents.md`
- `runs/<run_id>/01_prepare/personas/<persona_id>/prompt.txt`
- `runs/<run_id>/01_prepare/personas/<persona_id>/manifest.json`
- `runs/<run_id>/01_prepare/personas/<persona_id>/hashes.json`
- `runs/<run_id>/01_prepare/prepare_review_v1.json`
- `runs/<run_id>/01_prepare/prepare_gate_status_v1.json`

Answer artifacts:

- `runs/<run_id>/02_answer/personas/<persona_id>/workspace/AGENTS.md`
- `runs/<run_id>/02_answer/personas/<persona_id>/input/prompt.txt`
- `runs/<run_id>/02_answer/personas/<persona_id>/outgoing_input.json`
- `runs/<run_id>/02_answer/personas/<persona_id>/result.raw.txt`
- `runs/<run_id>/02_answer/personas/<persona_id>/result.json`
- `runs/<run_id>/02_answer/personas/<persona_id>/attestation.json`
- `runs/<run_id>/02_answer/personas/<persona_id>/status.json`
- `runs/<run_id>/02_answer/answer_batch.json`

Render artifacts:

- `runs/<run_id>/03_render/raw_render_input.json`
- `runs/<run_id>/03_render/certified_render_input.json`
- `runs/<run_id>/03_render/raw_panel.json`
- `runs/<run_id>/03_render/raw_panel.md`
- `runs/<run_id>/03_render/raw_cards.json`
- `runs/<run_id>/03_render/certified_panel.json`
- `runs/<run_id>/03_render/certified_panel.md`
- `runs/<run_id>/03_render/certified_cards.json`
- `runs/<run_id>/03_render/status.json`

Audit artifacts:

- `runs/<run_id>/audit/events.jsonl`
- `runs/<run_id>/audit/errors.log`
- `runs/<run_id>/audit/runtime_contract_status.json`

## Input Contracts You Must Not Drift

Stage 1 canonical dispatch fields are exactly:

- `roleplay_prompt`
- `discussion_question`
- `supplementary_materials`
- `output_contract`
- `assumptions_and_constraints`

Do not introduce aliases for those field names.

Stage 2 sealing rules:

- `P07` must not open `dispatch_input_v1.json`
- sealing reads only Stage 1 `agents.md`, `prompt.txt`, and `hashes.json`
- `outgoing_input.json` is the canonical Stage 2 seal record
- the sealed record currently includes `execution_cwd`, `execution_home_dir`, and `execution_codex_home_dir`
- P07/P08 must keep `HOME` and `CODEX_HOME` inside the isolated worker boundary
- P08 verifies the sealed env and input hashes against `outgoing_input.json` before launch

Authoritative final output rule:

- only `item/completed.agentMessage` counts as the authoritative final worker answer

## Known Operational Constraints

### Shared-storage Go locking issue

When the repo is run directly from Android/Termux shared storage, Go commands may fail because `go.mod` locking is not supported there.

Typical symptom:

- `go: RLock .../go.mod: function not implemented`

When this happens, do not debug the code first.
Mirror the repo into a local lock-capable directory and run Go commands there.

Example:

```sh
tmpdir=$(mktemp -d /data/data/com.termux/files/home/view_panel_test_XXXXXX) && \
tar -cf - --exclude=.git . | (cd "$tmpdir" && tar -xf -) && \
cd "$tmpdir" && \
go test ./...
```

### App-server transport seam

Current `internal/appserver/client.go` assumes stdout can be decoded as sequential JSON values.

If the real Codex app-server transport uses framed stdio, update the transport layer there instead of patching stage code around it.

### Materials seam

Multiple `--materials` inputs currently use a conservative deterministic strategy rather than a full merge contract.

If you change this, update:

1. `README.md`
2. startup expectations in `cmd/worldview-panel/main.go`
3. any prepare-stage assumptions that depend on material shape

## Codex Working Rules

When modifying this repository:

1. Start from the actual code path you are changing, not from old plan docs.
2. Preserve shared ownership boundaries.
3. Do not reintroduce stage-local path/hash/write implementations when `internal/storage` and `internal/hash` already own them.
4. Keep `internal/orchestrator/run.go` thin.
5. Do not make `P08` own batch policy.
6. Do not make `P11` reopen Stage 2 artifacts.
7. If you change CLI behavior, update `README.md`.
8. If you change runtime contract assumptions, update `docs/operations/codex-runtime-contract.md`.

## Recommended First Reads For Future Work

If you are continuing implementation, read in this order:

1. `AGENTS.md`
2. `README.md`
3. `docs/operations/codex-runtime-contract.md`
4. the specific package you are changing
5. only then the corresponding plan doc under `docs/plans/tech-plan-runtime/` if needed
