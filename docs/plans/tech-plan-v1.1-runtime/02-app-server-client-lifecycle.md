---
plan_id: P02
title: App-Server Client Lifecycle
status: proposed
depends_on:
  - P00
  - P01
consumes:
  - docs/plans/tech-plan-v1.1-runtime/00-cli-config-schema-validation.md
  - docs/plans/tech-plan-v1.1-runtime/01-codex-runtime-contract.md
produces:
  - internal/appserver/client.go
  - internal/appserver/messages.go
  - internal/appserver/runner.go
completion_evidence:
  - lifecycle_sequence_and_phase_transitions_enforced
  - stdio_transport_single_owner_reader_writer_enforced
  - launch_context_contract_cwd_home_environment_explicit
  - timeout_interrupt_and_close_hierarchy_single_ownership
  - protocol_compatibility_checks_limited_to_runtime_layer
  - run_single_turn_result_in_memory_model_defined
  - deterministic_close_and_kill_policy_defined
---

# Goal
Implement a thin, runtime-only JSON-RPC lifecycle for exactly one Codex app-server turn: process launch, ordered handshake/runtime sequence, completion or interrupt, and bounded shutdown with explicit error paths.

# Scope
- Own only the app-server transport and lifecycle for one turn in `internal/appserver/`:
  - `internal/appserver/messages.go`: protocol message models, request IDs, and request/response/event payloads for this lifecycle.
  - `internal/appserver/client.go`: process launch wrapper around `codex app-server --listen stdio://`, JSON-RPC transport, stream ownership, launch context contract, and in-flight request correlation.
  - `internal/appserver/runner.go`: ordered lifecycle orchestration for one configured thread/turn.
- Enforce the runtime sequence:
  1. `initialize`
  2. `initialized` (notification)
  3. `configRequirements/read`
  4. `thread/start`
  5. `turn/start`
  6. terminal completion, pre-`turn/start` timeout hard-stop, or post-`turn/start` interrupt timeout flow
  7. close/shutdown
- Define deterministic ownership:
  - `stdout`: protocol codec read-only owner in `client.go`.
  - `stdin`: protocol encoder write-only owner in `client.go`.
  - `stderr`: diagnostics sink only.
- Expose one API layer for P08: a started-client lifecycle layer with explicit ownership boundaries. `StartAppServer` owns launch and returns a started client, and `RunSingleTurn` consumes that client for a single-turn run without owning launch.

# Out Of Scope
- Startup CLI parsing, config merging, defaulting, and startup schema validation (`P00`).
- Runtime contract version pinning, transport pinning, and schema pin checks (`P01`).
- Workspace sealing, batch orchestration, hard/soft gates, render behavior.
- Any retry policy for model output, prompt routing, tool policy, or forbidden-tool handling.
- Transcript/artifact persistence and operator-visibility file emission.

# Required Inputs
- docs/plans/tech-plan-v1.1-runtime/00-cli-config-schema-validation.md
- docs/plans/tech-plan-v1.1-runtime/01-codex-runtime-contract.md

# Implementation Tasks
1. In `internal/appserver/messages.go`, define transport-contract types only for required lifecycle methods and terminal completion:
   - `initialize` request/response payloads (including compatibility fields required by smoke lifecycle).
   - `initialized` notification payload type.
   - `configRequirements/read` request/response payloads.
   - `thread/start` request/response payloads.
   - `turn/start` request/response payloads and minimum event payloads consumed during streaming.
   - `turn/interrupt` notification payloads (no response payload contract; acknowledgements remain optional/diagnostic only).
   - Canonical JSON-RPC envelope types:
      - request
      - response
      - notification
      - error
      - event-like message.
2. In `internal/appserver/client.go`, implement transport with strict process and stream ownership:
   - Define a launch context contract (for example, `AppServerLaunchContext`) with:
      - `ExtraArgs`: explicit additional args, appended in one explicit position *after* the P01-defined canonical launcher.
      - `Environment`: explicit env map passed to process
      - `CurrentWorkingDirectory`: required P07/P08 workspace cwd
      - `HomeDir`: controlled HOME override
   - Add `StartAppServer(ctx, launchContext)` that launches using the P01 canonical launcher (`codex app-server --listen stdio://`) with context-based cancellation, and appends allowed `ExtraArgs` only after that base argv form.
   - Treat `ctx` as launch-time cancellation only; once the process is successfully started, lifetime is owned solely by the returned client and ends on `Close()` / `ForceKill()`.
   - P02 applies the provided launch context only; P02 does not decide sealing policy or compute workspace strategy.
   - Launch environment is deterministic and explicit:
      - apply inherited environment only through contract-driven construction
      - enforce `HomeDir` when provided
      - apply `Environment` overrides
      - never mutate env or cwd policy outside launch context.
   - Create single-reader and single-writer ownership per process:
      - `stdin`: only request encoder writes.
      - `stdout`: only protocol decoder reads.
      - `stderr`: buffered capture for diagnostics, never parsed as JSON-RPC.
   - Implement `sendRequest(ctx, method, params)` with:
      - request ID increment from atomic counter.
      - dedicated response channel map keyed by ID.
      - request timeout exactly 30s for request/response operations (interrupt is sent via best-effort notification).
      - timeout or JSON-RPC error fails the call without retries.
      - keep raw JSON-RPC response correlation and dispatch in `client.go`; only correlate responses to typed request calls there.
   - Implement `sendNotification(method, params)` as best-effort fire-and-forget (no retry, no duplicate queue growth, no requeue).
   - send `turn/interrupt` through this path (best-effort write/flush only).
   - Implement `ReadMessage(ctx)` or callback-based delivery of protocol events to the runner:
      - emit typed lifecycle/event messages required by runner.
      - never expose raw response envelopes or response-channel internals to runner.
   - Add lifecycle hard-stop entrypoint(s) owned by the client layer for timeout recovery:
      - `ForceKill()` (alias `HardStop()` acceptable).
      - immediate kill/reap semantics with no graceful wait and no retry.
      - return an explicit exited-or-killed signal for timeout outcome selection.
   - Implement `Close(ctx)` that performs deterministic shutdown:
      - stop only new writes (quiesce writers/readers entry) while continuing to drain protocol/stderr output until process exit or forced kill.
      - allocate a fresh internal 10s shutdown budget per call from a detached base context (independent of caller/timeout context);
      - do not let caller context cancellation collapse or shrink the shutdown budget;
      - wait for process exit with that 10s deadline while draining child output;
      - stop goroutines only after process reap;
      - force-kill only after deadline expiry.
3. In `internal/appserver/runner.go`, implement the one concrete ordered lifecycle helper:
   - `RunSingleTurn(ctx, client, options)` returns one typed `RunSingleTurnResult`.
   - `RunSingleTurn` is client-owned runtime orchestration: it requires a successfully started client from `StartAppServer` and does not perform app-server launch.
   - options include thread/turn startup context and a run timeout.
   - Mandatory pre-checks:
      - return typed failure when client is not started: terminal `Failed`, `LastReachedPhase = NotStarted`, and `RunSingleTurnCloseOutcome.Kind = notStarted` (no child process exists; start never succeeded; caller invoked `RunSingleTurn` on an unstarted client handle).
      - if `ctx` is already canceled before first request and the client has already started, return terminal `Failed`, `LastReachedPhase = NotStarted`, and run the normal non-timeout `Close()` path (entry-cancelled overlap resolves to normal close, not pre-`turn/start` hard-stop).
   - `initialize` call:
      - call once and surface protocol-version plus capabilities for immediate compatibility checks.
   - `initialized` call:
      - send notification immediately after initialize success.
   - `configRequirements/read` call:
      - require success before thread creation.
      - enforce only live compatibility checks in this layer (method/event presence needed for lifecycle).
   - `thread/start` call:
      - persist thread identifier from response.
   - `turn/start` call:
      - start exactly one turn on active thread and persist turn identifier.
   - Stream loop:
      - process JSON-RPC messages until terminal event.
      - accept only `item/completed.agentMessage` as terminal completion.
      - treat any other completion-like terminal as explicit phase error.
      - append every decoded event to in-memory transcript for return.
   - Mid-flight `ctx.Done()` handling:
      - if `ctx.Done()` happens before `turn/start` acceptance: classify as `Failed` and take immediate hard-stop path (`ForceKill()` / `HardStop()`) without sending `turn/interrupt`.
      - if `ctx.Done()` happens after `turn/start` acceptance: apply the same branch as post-`turn/start` interrupt flow (single `turn/interrupt` attempt + 5s bounded grace).
   - Completion path:
      - return normalized completion with item id, text, and completion metadata.
   - Timeout/interrupt path:
      - classify timeout/cancel behavior by whether `turn/start` completed:
         - pre-`turn/start` timeout branch:
           - do not send `turn/interrupt`;
           - invoke client-owned hard-stop path (`ForceKill()` / `HardStop()`) immediately;
           - classify terminal outcome as `Failed` for request/turn/start timeout and preserve `LastReachedPhase` as the last completed milestone;
           - close outcome is `forcedKill` unless `alreadyExited`.
         - post-`turn/start` timeout branch:
            - send `turn/interrupt` exactly once.
            - treat `turn/interrupt` as write/flush-only (do not wait for a separate interrupt RPC response budget).
            - start the single 5s interrupt grace budget immediately **before** the `turn/interrupt` write/flush attempt.
            - `turn/interrupt` write/flush must complete or abort within that same 5s budget; it must not block beyond it.
            - any terminal wait for interrupt handling must fit inside that same 5s budget.
            - any interrupt acknowledgement observed in that window is optional/diagnostic and does not extend the budget.
            - success branch (terminal completes within grace):
               - if `item/completed.agentMessage` arrives inside grace, classify as `Completed` and use non-timeout close behavior.
            - failure branch (grace budget exhausted):
               - if still not terminal after 5s:
                  - mark `Interrupted`,
                  - call client hard-stop API (`ForceKill()` / `HardStop()`) where:
                     - process exited during 5s grace yields close outcome `alreadyExited`;
                     - otherwise force-kill and close outcome is `forcedKill`.
                  - never call `Close` or enter another graceful-shutdown branch after timeout grace expiry.
   - Finalization path:
      - for non-timeout finalization paths, always attempt bounded close and return close outcome separately from turn completion status.
      - if `Close()` times out on its 10s budget and performs a forced kill, map close outcome to `forcedKill`.
      - if `Close()` finds a clean pre-existing process exit (`ProcessExitCode == 0`), normalize close outcome as `graceful`.
      - if non-timeout `Close()` observes a non-zero exit code or a close failure, map close outcome to `closeFailed`.
      - reserve `alreadyExited` for timeout hard-stop branches only.
4. In both `runner.go` and `client.go`, enforce deterministic non-retry policy:
   - never retry `initialize`, `configRequirements/read`, `thread/start`, `turn/start`, or `turn/interrupt`.
   - never reopen transports inside a single lifecycle call.
   - surface every error with explicit phase and request context.
   - classify `sendRequest` timeouts in `initialize`, `configRequirements/read`, `thread/start`, or `turn/start` as terminal `Failed`, not `Interrupted`.
5. Define exactly one result model in `runner.go`:
   - `LifecyclePhase` enum-like constants:
      - `NotStarted`
      - `InitializeResponseReceived`, `InitializedNotificationSent`, `RequirementsRead`, `ThreadStarted`, `TurnStarted`
   - `RunSingleTurnTerminalOutcome` enum-like constants:
      - `Completed`, `Interrupted`, `Failed`.
   - `RunSingleTurnCloseOutcome` owns shutdown state and exit metadata (single owner):
      - `Kind` (one of: `graceful`, `alreadyExited`, `forcedKill`, `closeFailed`, `notStarted`)
      - `ProcessExitCode` (the only place exit code is represented)
      - `Error` (if close-related failure was captured)
   - `RunSingleTurnResult` includes:
      - terminal outcome (`RunSingleTurnTerminalOutcome`)
      - `LastReachedPhase` (`LifecyclePhase`)
      - thread id
      - turn id
      - optional completed item JSON payload
      - in-memory transcript slice
      - one `RunSingleTurnCloseOutcome` field
      - structured error path.
      - `LastReachedPhase` is the last successfully completed lifecycle milestone (starts at `NotStarted`); failed/in-flight operations never advance it and are represented in terminal outcome / structured error.
   - P02 does not produce or persist file-path artifacts.

# Acceptance Checks
- Ordering is enforced in runner code:
  - `initialize` before `initialized`.
  - `configRequirements/read` before `thread/start`.
  - `thread/start` before `turn/start`.
- `item/completed.agentMessage` is the only accepted completion terminal:
  - any other terminal-like message becomes explicit `RunError` with phase and request context.
- P01 boundary is enforced:
  - P02 enforces only live handshake/runtime checks required for this turn lifecycle.
  - P01 remains owner of version pin, launcher canonicality, schema artifact source, and static mismatch policy.
- Timeout / kill ownership is single and explicit:
  - request timeout (`sendRequest`): 30s.
  - interrupt grace window (`RunSingleTurn`): 5s.
  - close/grace wait (`Close`): 10s, then kill.
- Transport and lifecycle ownership are non-ambiguous:
  - `client.go` alone owns all stdio pipes and message codec wiring.
  - `runner.go` does not spawn parsers/writers or touch pipes directly.
- Retry policy is verifiable:
  - zero automatic retries on handshake/thread/turn/interrupt calls.
  - do not create or observe a 30s `turn/interrupt` RPC response budget; interrupt is write/flush best effort and only diagnostic ack handling is allowed.
  - no speculative repeats during timeout branches.
- Per-request timeout classification:
  - timeout while in `initialize`, `configRequirements/read`, `thread/start`, or `turn/start` yields terminal `Failed`.
  - `Interrupted` applies only to timeout flow after `turn/start` is accepted.
- if `RunSingleTurn` is entered with `ctx` already canceled before first request and the client never started, return terminal `Failed`, `LastReachedPhase = NotStarted`, and `RunSingleTurnCloseOutcome.Kind = notStarted`.
- Shutdown behavior is deterministic:
  - non-timeout close is bounded: for `Close()`, kill only occurs after wait expiry.
  - non-timeout `Close()` budget-exhaustion is represented as hard stop by emitting `RunSingleTurnCloseOutcome.Kind = forcedKill`.
  - Happy path concrete check:
    - all required RPCs succeed, terminal event is `item/completed.agentMessage`,
      terminal outcome is `Completed`, `LastReachedPhase == TurnStarted`, close outcome is `graceful` with `RunSingleTurnCloseOutcome.ProcessExitCode == 0`, and result includes thread/turn ids and completed item payload.
  - non-timeout `Close()` clean exits remain normalized to `RunSingleTurnCloseOutcome.Kind == graceful`.
  - non-timeout `Close()` non-zero exit or close failure remains normalized to `RunSingleTurnCloseOutcome.Kind == closeFailed`.
  - Pre-`turn/start` timeout check:
    - `turn/start` not yet accepted, no interrupt sent, hard-stop path invoked immediately, `LastReachedPhase` remains the last completed phase or `NotStarted`, terminal outcome is `Failed` for timeout/request-timeout, and close outcome is `forcedKill` unless `alreadyExited`.
  - Post-`turn/start` timeout check:
    - `turn/start` accepted, no terminal completion before timeout, one interrupt sent, and one 5s grace budget is observed.
    - if `item/completed.agentMessage` arrives during grace, classify as `Completed` and use normal non-timeout close path.
    - if no terminal event arrives by grace expiry, final outcome is `Interrupted` and close outcome is `forcedKill` unless `alreadyExited`.
  - Compatibility/mismatch check:
    - malformed response or unexpected terminal event before completion returns `Failed` terminal outcome with explicit phase, request id, and reason, and `LastReachedPhase` remains the last successfully completed phase (or `NotStarted` if none).

# Handoff
- P08 calls `StartAppServer` to own process launch, then passes the started client to `RunSingleTurn`; it must not treat `RunSingleTurn` as a standalone launch-owning API.
- P08 must still not re-implement init/config/thread/turn sequencing, launch env/cwd ownership, stream ownership, timeout hierarchy, or close/kill policy.
