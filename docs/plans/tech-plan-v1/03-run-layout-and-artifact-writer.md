---
plan_id: P03
title: Run Layout And Artifact Writer
status: ready
depends_on: []
consumes:
  - docs/tech-plan-v1.md
  - docs/superpowers/specs/2026-04-11-tech-plan-v1-executable-plans-design.md
produces:
  - internal/storage/layout.go
  - internal/hash/hash.go
completion_evidence:
  - run_directory_layout_fixed
  - shared_writer_helpers_defined
  - sha256_helpers_available
---

# Goal

Define the on-disk run layout and the shared writers that every stage uses to persist canonical artifacts.

# Scope

- Create a typed run layout for `00_request`, `01_prepare`, `02_answer`, `03_render`, and `audit`.
- Define the ownership boundary for P03 where the storage layer owns canonical path constructors plus shared artifact-writing helpers.
- Keep hash helpers as a separate concern in the hash module.
- Add helpers for JSON, text, and JSONL writing.
- Add SHA-256 helpers for byte-stable and file-stable hashing.

# Out Of Scope

- Protocol transport logic.
- Stage-specific business logic.
- Global log indexing.

# Required Inputs

- `docs/tech-plan-v1.md`
- `docs/superpowers/specs/2026-04-11-tech-plan-v1-executable-plans-design.md`

# Implementation Tasks

- Add `internal/storage/layout.go` with canonical path constructors.
- In storage, add shared artifact writer helpers used by all stages.
- Define JSON and text artifact helpers as single-shot writes to a canonical target path, using deterministic file content and overwrite semantics for canonical artifact files.
- Define JSONL helpers as event stream writers using append-oriented behavior, one JSON object per line in line-delimited format.
- Add `internal/hash/hash.go` with SHA-256 helpers for byte-level stable hashing and canonical content hashing.
- Add focused tests for deterministic paths and hash stability.

# Acceptance Checks

- The run layout exposes stable paths for every named stage directory.
- Writers can create JSON, text, and JSONL artifacts under that layout.
- Hash helpers can compute deterministic SHA-256 digests for byte and file inputs.
- JSON/text writers overwrite canonical artifact files; JSONL writers append records to existing streams.
- P03 defines only stable hashing primitives, while consuming stage plans define exact hash payload composition and stage-specific hash semantics.

# Handoff

P04, P05, P07, P08, P10, and P11 consume these path and writer helpers.
