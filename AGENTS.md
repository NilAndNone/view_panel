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

## Content Authoring Boundary

Authored content now has a separate build path:

- source authoring tree: `content/src/`
- stable builder/smoke fixture tree: `testdata/content/builder_fixture/src/`
- catalog / compiler / bundle writer: `internal/content/`
- bundle build CLI: `tools/build_content_bundle/main.go`
- compiled bundle root: `content/build/<bundle_id>/`

Keep this contract intact:

- runtime stages and the main panel CLI continue to consume compiled JSON under `content/build/<bundle_id>/runtime/`
- rich authoring files under `content/src/` are build inputs only and must not be read directly by runtime stages
- the normal authoring workflow is `content/src/` -> `make build-content CONTENT_BUNDLE_ID=<bundle_id>` -> `content/build/<bundle_id>/runtime/`
- retained direct workflows are: run `worldview-panel` against compiled runtime JSON, and run distinctness eval against the same compiled bundle plus `content/src/evals/distinctness/questions.json`
- if distinctness eval is invoked directly instead of `make content-eval`, it must receive an explicit `-out` path outside `content/build/<bundle_id>/`
- default `make content-eval` output now lives under `out/content-eval/<bundle_id>/latest/` so `make build-content` can replace `content/build/<bundle_id>/` without wiping retained eval history
- builder-path infra tests and `make content-smoke-build` must stay pinned to the dedicated fixture tree, not the evolving live product cohort
- if you change bundle build behavior or compiled bundle shape, update both `README.md` and this file deliberately

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
- `internal/appserver/stream.go`
- `internal/appserver/client.go`
- `internal/appserver/runner.go`

Prepare chain:

- `internal/materials/loader.go`
- `internal/materials/merge.go`
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
- the checked-in protocol bundle under `third_party/codex-protocol/0.0.0/` is live captured content for that pinned CLI
- refresh the checked-in bundle with `make capture-protocol` when the pinned CLI or protocol contract changes
- do not treat the checked-in bundle as proof that a newer or unpinned CLI is compatible; startup/runtime checks still own compatibility enforcement

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
- `P09` owns batch policy, retry policy, forbidden-tool rejection, and `answer_batch.json`
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

Multi-material merge semantics are explicit:

- `roleplay_prompt`, `discussion_question`, `output_contract`, and `assumptions_and_constraints` allow at most one unique non-blank value across all selected canonical material files
- `supplementary_materials` concatenates non-blank values in deterministic input order with `\n\n`

Stage 2 sealing rules:

- `P07` must not open `dispatch_input_v1.json`
- sealing reads only Stage 1 `agents.md`, `prompt.txt`, and `hashes.json`
- `outgoing_input.json` is the canonical Stage 2 seal record; the current `schema_version` is `answer_outgoing_input_v2`
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
tar -cf - --exclude=.git --exclude=out --exclude=runs --exclude=content/build --exclude=go_bin . | (cd "$tmpdir" && tar -xf -) && \
cd "$tmpdir" && \
go test ./...
```

The repository `Makefile` avoids this by mirroring into a local temp directory first.
Those mirror copies intentionally skip generated trees such as `out/`, `runs/`, `content/build/`, and `go_bin`.
That temp root is configurable with `TMP_ROOT`; if the default is unsuitable on the current machine, override it explicitly.

Example:

```sh
make build TMP_ROOT="$HOME"
```

### Shared-storage repo-root binary caveat

In a shared-storage checkout, do not treat repo-root `./go_bin` as the primary manual run path.
Even when `make build` succeeds, the binary written back into the repo may not be executable in place because shared storage can be `noexec` or can drop execute bits.

Preferred manual workflows:

- `make build` and run the default output path `$HOME/worldview-panel_bin`
- or `make build BIN_PATH=/abs/path/to/worldview-panel` and run that absolute path; the Makefile creates `$(dir BIN_PATH)` for you
- or mirror the repo into a local exec-capable directory and run the binary there

Manual run caveat:

- repo discovery is cwd-based; even if the binary itself lives at `$HOME/worldview-panel_bin` or another absolute path, start the process with `cwd` inside the repo tree so startup/runtime checks can resolve the checked-in repository context

Keep this caveat visible in `README.md` whenever build/run instructions are updated.

### App-server transport seam

`internal/appserver/stream.go` now owns stdout transport probing and decoding.

Current behavior:

- probe stdout after leading transport whitespace
- if the stream starts with `Content-Length:`, decode as framed stdio
- otherwise fall back to sequential JSON decoding

If the Codex app-server transport changes again, update `internal/appserver/stream.go` / `internal/appserver/client.go` and the runtime-contract docs instead of patching stage code around it.

### Materials seam

Multiple `--materials` inputs now use the explicit canonical merge contract.

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
