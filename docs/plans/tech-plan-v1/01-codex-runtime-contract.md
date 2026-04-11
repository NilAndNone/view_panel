---
plan_id: P01
title: Codex Runtime Contract
status: ready
depends_on: []
consumes:
  - docs/tech-plan-v1.md
  - docs/superpowers/specs/2026-04-11-tech-plan-v1-executable-plans-design.md
produces:
  - docs/operations/codex-runtime-contract.md
  - third_party/codex-protocol/
completion_evidence:
  - codex_cli_version_pinned
  - app_server_transport_fixed_to_stdio
  - schema_generation_location_defined
---

# Goal

Define the supported Codex runtime contract so every later plan targets the same CLI version, transport, schema bundle location, and smoke-test flow.

# Scope

- Pin one supported `@openai/codex` version.
- Lock `codex app-server --listen stdio://` as the only transport for v1.
- Define where generated protocol schema is stored.
- Record install, login, schema generation, and smoke-test commands.

# Out Of Scope

- Implementing the Go client.
- Building any stage logic.
- Introducing WebSocket support.

# Required Inputs

- `docs/tech-plan-v1.md`
- `docs/superpowers/specs/2026-04-11-tech-plan-v1-executable-plans-design.md`

# Implementation Tasks

- Create `docs/operations/codex-runtime-contract.md`.
- Record the pinned CLI version and the reason it must stay locked.
- Record `stdio://` as the only supported app-server transport.
- Record the schema generation command and the smoke-test command.

# Acceptance Checks

- The runtime contract document names exactly one supported CLI version.
- The contract declares `stdio://` as the only supported transport.
- The schema output directory is fixed to `third_party/codex-protocol/`.
- The runtime contract document records all required command categories:
  - install command
  - login command
  - schema generation command
  - smoke-test command

# Handoff

P02 consumes the pinned CLI, transport, and schema directory decisions.
