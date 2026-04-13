---
plan_id: P12
title: End-to-End Smoke + Makefile Flow
status: ready
depends_on:
  - P11
consumes:
  - docs/plans/tech-plan-runtime/11-render-outputs.md
produces:
  - Makefile
  - tools/smokecheck/main.go
  - testdata/smoke/materials/
  - testdata/smoke/runtime/
  - docs/operations/runtime-smoke-and-release-checklist.md
completion_evidence:
  - make_smoke_is_canonical_entrypoint
  - smoke_dataset_is_fixed_minimum
  - smokecheck_validates_branch_state_coherence
---

# Goal

Own the real end-to-end runtime validation flow so operators can exercise one full CLI-to-panel path through stable Makefile entrypoints instead of stitching together ad hoc smoke commands.

# Scope

- P12 owns the human-facing and operator-facing smoke entrypoints in `Makefile`.
- P12 owns the fixed smoke fixtures under `testdata/smoke/`.
- P12 owns the machine-checkable smoke verifier in `tools/smokecheck/main.go`.
- P12 owns the formal run, release, and debug checklist in `docs/operations/runtime-smoke-and-release-checklist.md`.
- P12 treats `make build`, `make capture-protocol`, `make smoke`, and `make smoke-check RUN_ID=<run_id>` as the canonical operator surface for protocol evidence refresh plus end-to-end smoke.

# Out Of Scope

- CLI argument design and startup validation semantics owned by P00.
- App-server lifecycle semantics owned by P02.
- Any prepare, answer, or render stage logic owned by P04 through P11.
- Any redesign of certified-gate math or render semantics owned upstream by P10 and P11.

# Required Inputs

- `docs/plans/tech-plan-runtime/11-render-outputs.md`
- the retained runtime flow from P00 through P11
- the existing smoke-support artifacts already implemented in the repository:
  - `Makefile`
  - `tools/smokecheck/main.go`
  - `testdata/smoke/materials/`
  - `testdata/smoke/runtime/`
  - `docs/operations/runtime-smoke-and-release-checklist.md`

# Implementation Tasks

1. Define `Makefile` as the canonical full-flow smoke surface.
   - `make build` is the canonical binary build path.
   - `make capture-protocol` is the canonical pinned protocol-bundle refresh path.
   - `make smoke` is the canonical end-to-end smoke path.
   - `make smoke-check RUN_ID=<run_id>` is the canonical post-run verifier.
   - No alternative README-only long command becomes the preferred smoke path.
   - `make smoke` may use the current checked-in live bundle as-is; protocol refresh remains an explicit separate operator step.
2. Lock the smoke dataset to one fixed minimal repeatable slice.
   - exactly `2 personas`
   - exactly `1 material`
   - the retained smoke fixtures under `testdata/smoke/` are the canonical dataset
   - P12 does not own larger certification or load-test datasets
3. Treat `make smoke` as one deterministic end-to-end contract.
   - build the binary
   - launch the real runtime flow
   - write the run under the configured smoke output root
   - locate the resulting run id
   - invoke `make smoke-check` for that run
4. Lock the smoke-check responsibility to structural and branch-state validation.
   - validate `01_prepare/prepare_gate_status_v1.json`
   - validate `02_answer/answer_batch.json`
   - validate `03_render/raw_render_input.json`
   - validate `03_render/certified_render_input.json`
   - validate `03_render/status.json`
   - reject incoherent raw/certified availability and rendered/skipped/failed states
   - do not turn smoke-check into a semantic answer-quality reviewer
5. Keep operational reality in scope for the smoke harness.
   - shared-storage `go.mod` locking limitations are handled by the Makefile flow
   - shared-storage `noexec` limitations on repo-local binaries are handled by the Makefile flow
   - protocol-bundle capture refresh is also handled by the Makefile mirror flow
   - the checklist documents those constraints for operators
6. Keep P12 terminal.
   - P12 validates the full runtime chain after P11 outputs exist
   - P12 does not redefine any earlier ownership boundary
   - P12 does not become a substitute for unit-level runtime contracts

# Acceptance Checks

- `make build` exists as the canonical build entrypoint.
- `make capture-protocol` exists as the canonical protocol-bundle refresh entrypoint.
- `make smoke` exists as the canonical full-flow smoke entrypoint.
- `make smoke-check RUN_ID=<run_id>` exists as the canonical verification entrypoint.
- The retained smoke dataset is fixed to `2 personas` and `1 material`.
- A successful smoke run produces at least:
  - `01_prepare/prepare_gate_status_v1.json`
  - `02_answer/answer_batch.json`
  - `03_render/raw_render_input.json`
  - `03_render/certified_render_input.json`
  - `03_render/status.json`
- `smoke-check` validates branch-state coherence instead of only checking file existence.
- The Makefile-based smoke flow explicitly handles shared-storage `go.mod` locking and repo `noexec` realities.
- `README.md` can point users at `make smoke`, but P12 remains the owner of the smoke contract.

# Handoff

- P12 is the terminal validation plan for the retained runtime chain.
- Repository-level documentation can treat `make smoke` as the recommended full-flow runtime check because P12 owns that operator contract.
- Repository-level documentation can tell operators to run `make capture-protocol` before smoke or release when pinned protocol evidence needs refreshing, without redefining `make smoke` as the bundle-refresh owner.
- Future runtime changes that alter the full-flow verification surface must update P12 rather than scattering smoke ownership into P00 or P11.
