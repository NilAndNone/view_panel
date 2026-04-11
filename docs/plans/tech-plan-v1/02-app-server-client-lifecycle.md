---
plan_id: P02
title: App-Server Client Lifecycle
status: proposed
depends_on:
  - P01
consumes:
  - docs/plans/tech-plan-v1/01-codex-runtime-contract.md
produces:
  - internal/appserver/client.go
  - internal/appserver/messages.go
  - internal/appserver/runner.go
completion_evidence:
  - initialize_initialized_flow_implemented
  - requirements_read_supported
  - single_turn_lifecycle_wrapped
---

# Goal

Implement the thin Go app-server client that manages one process, one thread, and one turn lifecycle over JSON-RPC on stdio.

# Scope

- Define protocol message structs for requests, responses, and notifications.
- Spawn `codex app-server --listen stdio://`.
- Implement `initialize`, `initialized`, `configRequirements/read`, `thread/start`, `turn/start`, `turn/interrupt`, and clean shutdown.
- Expose a single-turn runner API for later stages.

# Out Of Scope

- Stage-specific prompt construction.
- Audit log persistence.
- Batch orchestration.

# Required Inputs

- `docs/plans/tech-plan-v1/01-codex-runtime-contract.md`
- `docs/plans/tech-plan-v1.md`

# Implementation Tasks

- Add protocol message types in `internal/appserver/messages.go`.
- Add the process-backed client in `internal/appserver/client.go`.
- Add a single-turn lifecycle helper in `internal/appserver/runner.go` with explicit request and response semantics through `RunSingleTurn`.
- Add tests or canned transcript checks for handshake and final completion parsing.

# Acceptance Checks

- The client can complete the full handshake sequence in order.
- `configRequirements/read` is exposed before `thread/start`.
- `item/completed.agentMessage` is treated as the final worker output source.
- `RunSingleTurn` runs normal completion through handshake and turn result parsing, treats the final completed output as authoritative, then performs clean app-server process shutdown.
- `RunSingleTurn` timeout/error behavior attempts `turn/interrupt` before any process kill, with process termination as fallback.

# Handoff

P04 and P08 consume this client lifecycle instead of inventing their own protocol loops.
