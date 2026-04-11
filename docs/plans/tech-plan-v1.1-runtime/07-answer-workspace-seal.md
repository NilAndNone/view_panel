---
plan_id: P07
title: Answer Workspace Seal
status: proposed
depends_on:
  - P03
  - P05
consumes:
  - docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md
  - docs/plans/tech-plan-v1.1-runtime/04-prepare-input-builder.md
  - docs/plans/tech-plan-v1.1-runtime/05-prepare-hard-gate.md
produces:
  - internal/stage/answer/workspace.go
  - runs/<run_id>/02_answer/personas/<persona_id>/workspace/AGENTS.md
  - runs/<run_id>/02_answer/personas/<persona_id>/input/prompt.txt
  - runs/<run_id>/02_answer/personas/<persona_id>/outgoing_input.json
completion_evidence:
  - gate_controlled_stage2_seal_defined
  - outgoing_input_hash_chain_locked
  - isolated_execution_workspace_contract_defined
---

# Goal

Create the deterministic Stage 2 sealing step that turns hard-gated Stage 1 artifacts into per-persona answer snapshots plus a separate isolated execution workspace, without launching any worker.

# Scope

- Add `internal/stage/answer/workspace.go` as the sole owner of Stage 2 input sealing.
- P07 runs only after the P05 hard gate and consumes `<run_root>/01_prepare/prepare_gate_status_v1.json` as the authoritative admission artifact:
  - `can_proceed_to_stage2` is the only blocking field P07 uses to decide whether sealing may proceed.
  - P07 must ignore process exit status and must not infer Stage 2 admission from any other source.
  - if `can_proceed_to_stage2 == false`, P07 returns a blocked result and must not create any persona-level Stage 2 seal artifacts.
- P07 consumes the canonical Stage 1 persona bundle indirectly through immutable P04 outputs:
  - `<run_root>/01_prepare/personas/<persona_id>/agents.md`
  - `<run_root>/01_prepare/personas/<persona_id>/prompt.txt`
  - `<run_root>/01_prepare/personas/<persona_id>/hashes.json`
- `dispatch_input_v1.json` is not a Stage 2 worker input and must not be read, copied, or reopened by P07. The only P04 dispatch linkage that may flow into Stage 2 is the canonical `dispatch_input_sha256` value read from the P04 hash ledger.
- P07 owns two distinct outputs with different lifetimes and purposes:
  1. Audit/sealed snapshot artifacts under `<run_root>/02_answer/personas/<persona_id>/...`
     - `workspace/AGENTS.md`
     - `input/prompt.txt`
     - `outgoing_input.json`
     - these are the stable run-tree audit record of what Stage 2 was sealed to use.
  2. The actual isolated execution workspace outside the repo tree and outside `<run_root>`
     - this is a runtime-owned directory used later by P08 for worker launch.
     - it must contain only copied execution inputs and a controlled `HOME`.
     - it is execution state, not the canonical audit record.
- Copy/reference boundaries are explicit and must not be blurred:
  - copied into the run-tree audit snapshot:
    - `02_answer/personas/<persona_id>/workspace/AGENTS.md`
    - `02_answer/personas/<persona_id>/input/prompt.txt`
    - `02_answer/personas/<persona_id>/outgoing_input.json`
  - referenced in the canonical audit record but not snapshotted under `02_answer`:
    - the selected Stage 2 skill source path
  - copied into the isolated execution workspace:
    - `workspace/AGENTS.md`
    - `input/prompt.txt`
    - `skill/SKILL.md`
- The Stage 2 skill file is an explicit runtime input to P07, not content authored by this plan:
  - P07 accepts one selected skill source file path.
  - that path must resolve under the repo root and must point to a readable regular file.
  - P07 records the repo-relative source path in `outgoing_input.json` and copies the selected skill bytes into the isolated execution workspace.
  - P07 does not own the wording or design of the skill file itself.
- Seal-chain fields are fixed for `outgoing_input.json`:
  - required fields:
    - `schema_version`, exact literal `answer_outgoing_input_v1`
    - `stage`, exact literal `02_answer`
    - `persona_id`
    - `agent_instructions_path`
    - `agent_instructions_sha256`
    - `prompt_path`
    - `prompt_sha256`
    - `skill_path`
    - `skill_sha256`
    - `source_dispatch_input_sha256`
    - `combined_input_sha256`
  - P07 must not rename, omit, or relax any required field or exact literal above.
- Hash ownership and derivation are explicit:
  - `source_dispatch_input_sha256` is copied verbatim from `01_prepare/personas/<persona_id>/hashes.json.dispatch_input_sha256`; it must come from the canonical P04 hash ledger and must not be recomputed from `dispatch_input_v1.json`.
  - `agent_instructions_sha256` is the SHA-256 of the exact bytes written to `02_answer/personas/<persona_id>/workspace/AGENTS.md` and must equal the P04 `agents_sha256` ledger entry.
  - `prompt_sha256` is the SHA-256 of the exact bytes written to `02_answer/personas/<persona_id>/input/prompt.txt` and must equal the P04 `prompt_sha256` ledger entry.
  - `skill_sha256` is the SHA-256 of the exact bytes read from the selected skill source file and copied to the isolated execution workspace.
  - `combined_input_sha256` is defined only over the actual Stage 2 worker content inputs, not conceptual upstream inputs. It must be computed as:
    - `SHA256(`
    - `uint64_be(len(agent_instructions_bytes)) || agent_instructions_bytes ||`
    - `uint64_be(len(prompt_bytes)) || prompt_bytes ||`
    - `uint64_be(len(skill_bytes)) || skill_bytes`
    - `)`
  - `combined_input_sha256` must not include `dispatch_input_v1.json` bytes, absolute temp-directory paths, repo-root paths, or HOME/cwd strings.
- `outgoing_input.json` is the canonical Stage 2 seal record under `<run_root>` and must remain host-stable:
  - `agent_instructions_path` is exactly `workspace/AGENTS.md`.
  - `prompt_path` is exactly `input/prompt.txt`.
  - `skill_path` is the repo-relative selected skill source path.
  - ephemeral isolated-workspace absolute paths must not be written into `outgoing_input.json`.
- The isolated execution workspace contract is fixed in P07 even though worker launch is deferred to P08:
  - P07 receives an explicit `isolatedBaseDir` and must reject it if it resolves under the repo root, under `<run_root>`, or anywhere under the current user HOME tree.
  - per persona, the isolated execution root is exactly `<isolatedBaseDir>/<run_id>/<persona_id>/`.
  - the resolved isolated execution root itself must also be rejected if it resolves under the repo root, under `<run_root>`, or anywhere under the current user HOME tree.
  - required isolated subdirectories are:
    - `workspace/`
    - `input/`
    - `skill/`
    - `home/`
  - required copied files are:
    - `<isolated_root>/workspace/AGENTS.md`
    - `<isolated_root>/input/prompt.txt`
    - `<isolated_root>/skill/SKILL.md`
  - required execution environment values returned by P07 are:
    - `cwd = <isolated_root>/workspace`
    - `HOME = <isolated_root>/home`
  - the isolated workspace must use real directories and regular copied files only; no symlink, bind, or path alias back into the repo tree, `<run_root>`, or the user home may be used.
- AGENTS contamination must be blocked by construction:
  - repo-root `AGENTS.md` must be unreachable by ancestor traversal from the worker `cwd` because `cwd` is outside the repo tree.
  - home-tree `AGENTS.md` contamination must be unreachable by ancestor traversal from the worker `cwd` because `<isolated_root>` itself is outside the current user HOME tree.
  - `~/.codex/AGENTS.md` contamination must be blocked because the worker `HOME` is the isolated `home/` directory, not the user home.
  - inside the isolated workspace, the intended worker instructions file is `<isolated_root>/workspace/AGENTS.md`; P07 must not create any additional parent-level `AGENTS.md` file under `<isolated_root>`.
- P07 is strictly pre-execution:
  - do not launch a worker.
  - do not open an app-server session.
  - do not batch personas beyond sealing their inputs.
  - do not define result schemas or worker-output handling.

# Out Of Scope

- Answer-worker execution, transport lifecycle, or session management (`P08`, `P02`)
- Multi-persona scheduling, retries, or orchestration (`P09`)
- Any mutation of P04 or P05 artifacts
- Any dependency on `prepare_review_v1.json` or P06 advisory review output
- Any direct Stage 2 read of `dispatch_input_v1.json`
- Any redesign of the answer skill content itself

# Required Inputs

- `docs/plans/tech-plan-v1.1-runtime/03-run-layout-and-artifact-writer.md`
- `docs/plans/tech-plan-v1.1-runtime/04-prepare-input-builder.md`
- `docs/plans/tech-plan-v1.1-runtime/05-prepare-hard-gate.md`

# Implementation Tasks

1. Implement the gate-controlled Stage 2 seal entrypoint in `internal/stage/answer/workspace.go`.
   - load `<run_root>/01_prepare/prepare_gate_status_v1.json` and require `can_proceed_to_stage2 == true` before any persona sealing starts.
   - use `persona_ids` from the gate artifact as the authoritative Stage 2 persona set and preserve that order exactly.
   - do not rescan `<run_root>/01_prepare/personas` to define the Stage 2 persona set.
   - do not read or require `<run_root>/01_prepare/prepare_review_v1.json`.
   - accept explicit runtime inputs for:
     - `repoRoot`
     - `runRoot`
     - selected answer-skill source path
     - `isolatedBaseDir`
   - reject the seal attempt if the selected answer-skill source path does not resolve under `repoRoot`, is unreadable, or is not a regular file.
   - reject the seal attempt if `isolatedBaseDir` resolves under `repoRoot` or under `runRoot`.
2. Read and verify the Stage 1 source artifacts for each authoritative persona without opening `dispatch_input_v1.json`.
   - for each `persona_id` from the gate artifact, read exactly:
     - `<run_root>/01_prepare/personas/<persona_id>/agents.md`
     - `<run_root>/01_prepare/personas/<persona_id>/prompt.txt`
     - `<run_root>/01_prepare/personas/<persona_id>/hashes.json`
   - parse `hashes.json` and require the canonical P04 keys:
     - `dispatch_input_sha256`
     - `agents_sha256`
     - `prompt_sha256`
     - `bundle_sha256`
   - recompute SHA-256 over the read `agents.md` bytes and require an exact match to `hashes.json.agents_sha256` before any Stage 2 snapshot is written.
   - recompute SHA-256 over the read `prompt.txt` bytes and require an exact match to `hashes.json.prompt_sha256` before any Stage 2 snapshot is written.
   - copy `hashes.json.dispatch_input_sha256` into the in-memory seal record as `source_dispatch_input_sha256`.
   - never open, hash, copy, or pass through `<run_root>/01_prepare/personas/<persona_id>/dispatch_input_v1.json`.
3. Write the canonical run-tree audit snapshot artifacts under `<run_root>/02_answer/personas/<persona_id>/`.
   - resolve all run-tree artifact paths through `AnswerPersonaArtifactPath` from P03.
   - write `workspace/AGENTS.md` as the exact verified bytes from the Stage 1 `agents.md` artifact.
   - write `input/prompt.txt` as the exact verified bytes from the Stage 1 `prompt.txt` artifact.
   - compute `agent_instructions_sha256` from the exact bytes written to `workspace/AGENTS.md`; it must still equal the P04 `agents_sha256` ledger value.
   - compute `prompt_sha256` from the exact bytes written to `input/prompt.txt`; it must still equal the P04 `prompt_sha256` ledger value.
   - treat these run-tree files as the stable audit snapshot of Stage 2 input sealing. Reruns may overwrite them atomically with identical content, but P07 must not invent alternate filenames or layouts.
4. Build `outgoing_input.json` as the canonical Stage 2 seal record and write it last for each persona.
   - `outgoing_input.json` must contain these exact top-level keys in this exact semantic contract:
     - `schema_version`, exact literal `answer_outgoing_input_v1`
     - `stage`, exact literal `02_answer`
     - `persona_id`
     - `agent_instructions_path`, exact literal `workspace/AGENTS.md`
     - `agent_instructions_sha256`
     - `prompt_path`, exact literal `input/prompt.txt`
     - `prompt_sha256`
     - `skill_path`, repo-relative selected skill source path
     - `skill_sha256`
     - `source_dispatch_input_sha256`
     - `combined_input_sha256`
   - compute `skill_sha256` from the exact skill bytes selected from the repo-root-relative skill source path.
   - compute `combined_input_sha256` from the exact content bytes that the worker will later see: Stage 2 `AGENTS.md`, Stage 2 `prompt.txt`, and the copied skill payload, using the fixed length-prefixed order defined above.
   - do not include absolute isolated-workspace paths in `outgoing_input.json`.
   - write `outgoing_input.json` only after the persona's run-tree snapshots and isolated execution copies have been prepared successfully, so it serves as the persona-level seal completion record.
5. Materialize the isolated external execution workspace for later P08 use, without launching the worker.
   - reject `isolatedBaseDir` if its resolved path is under the repo root, under `<run_root>`, or under the current user HOME tree.
   - for each persona, create `<isolatedBaseDir>/<run_id>/<persona_id>/` as the isolated execution root.
   - reject the persona isolated root if its resolved path is under the repo root, under `<run_root>`, or under the current user HOME tree after appending `<run_id>/<persona_id>/`.
   - create exactly these subdirectories under that root:
     - `workspace/`
     - `input/`
     - `skill/`
     - `home/`
   - copy the verified Stage 2 `AGENTS.md` bytes to `<isolated_root>/workspace/AGENTS.md`.
   - copy the verified Stage 2 `prompt.txt` bytes to `<isolated_root>/input/prompt.txt`.
   - copy the selected skill bytes to `<isolated_root>/skill/SKILL.md`.
   - do not symlink, hard-link, or otherwise reference files in place from the repo tree or `<run_root>`.
   - return execution metadata for later P08 use at minimum:
     - isolated root path
     - `cwd = <isolated_root>/workspace`
     - `HOME = <isolated_root>/home`
     - `<isolated_root>/workspace/AGENTS.md`
     - `<isolated_root>/input/prompt.txt`
     - `<isolated_root>/skill/SKILL.md`
   - do not persist these host-specific absolute isolated paths into `outgoing_input.json`.
   - P07 may create `<isolated_root>/home/.codex/` as an empty directory, but it must not create `AGENTS.md` there.
6. Keep P07 strictly narrow and pre-execution.
   - do not start any worker process from `workspace.go`.
   - do not create answer result, transcript, or status artifacts.
   - do not define any batching API beyond sealing the persona set selected by the P05 gate artifact.
   - do not allow P08 or P09 concerns to leak into the P07 data contract.

# Acceptance Checks

- P07 reads `<run_root>/01_prepare/prepare_gate_status_v1.json` as the sole Stage 2 admission artifact and obeys `can_proceed_to_stage2` directly.
- If `can_proceed_to_stage2 == false`, P07 produces no persona-level Stage 2 seal artifacts under `<run_root>/02_answer/personas/<persona_id>/` and no isolated execution workspace.
- The authoritative Stage 2 persona set and ordering come from `prepare_gate_status_v1.json.persona_ids`; P07 does not rescan prepare directories to redefine membership or order.
- P07 does not read or require `<run_root>/01_prepare/prepare_review_v1.json`.
- For each authoritative persona, P07 reads only these Stage 1 per-persona artifacts as sealing sources:
  - `agents.md`
  - `prompt.txt`
  - `hashes.json`
- For each authoritative persona, P07 never opens `dispatch_input_v1.json`.
- For each authoritative persona, `source_dispatch_input_sha256` in `outgoing_input.json` equals `01_prepare/personas/<persona_id>/hashes.json.dispatch_input_sha256` exactly.
- For each authoritative persona, `02_answer/personas/<persona_id>/workspace/AGENTS.md` is byte-identical to `01_prepare/personas/<persona_id>/agents.md`.
- For each authoritative persona, `02_answer/personas/<persona_id>/input/prompt.txt` is byte-identical to `01_prepare/personas/<persona_id>/prompt.txt`.
- For each authoritative persona, recomputed SHA-256 over the Stage 2 snapshot files matches the canonical P04 ledger values:
  - `agent_instructions_sha256 == hashes.json.agents_sha256`
  - `prompt_sha256 == hashes.json.prompt_sha256`
- `outgoing_input.json` uses the exact top-level key set:
  - `schema_version`
  - `stage`
  - `persona_id`
  - `agent_instructions_path`
  - `agent_instructions_sha256`
  - `prompt_path`
  - `prompt_sha256`
  - `skill_path`
  - `skill_sha256`
  - `source_dispatch_input_sha256`
  - `combined_input_sha256`
- In every `outgoing_input.json`:
  - `schema_version == "answer_outgoing_input_v1"`
  - `stage == "02_answer"`
  - `agent_instructions_path == "workspace/AGENTS.md"`
  - `prompt_path == "input/prompt.txt"`
  - `skill_path` is repo-relative and never an isolated temp path
- For each authoritative persona, `skill_sha256` equals the SHA-256 of the exact bytes copied to `<isolated_root>/skill/SKILL.md`.
- For each authoritative persona, `combined_input_sha256` is exactly the SHA-256 of the fixed length-prefixed byte stream:
  - `uint64_be(len(<isolated_root>/workspace/AGENTS.md bytes)) + AGENTS bytes`
  - `uint64_be(len(<isolated_root>/input/prompt.txt bytes)) + prompt bytes`
  - `uint64_be(len(<isolated_root>/skill/SKILL.md bytes)) + skill bytes`
- `combined_input_sha256` excludes `dispatch_input_v1.json`, absolute temp-directory paths, repo-root paths, and HOME/cwd strings.
- The isolated execution root for every persona is outside the repo tree, outside `<run_root>`, and outside the current user HOME tree.
- The isolated execution layout is exact for every persona:
  - `<isolated_root>/workspace/AGENTS.md`
  - `<isolated_root>/input/prompt.txt`
  - `<isolated_root>/skill/SKILL.md`
  - `<isolated_root>/home/`
- P07 returns `cwd = <isolated_root>/workspace` and `HOME = <isolated_root>/home` for later P08 worker launch.
- The isolated workspace uses only copied regular files and real directories; no symlink, hard-link, or in-place file reference back to the repo tree, `<run_root>`, or user home is allowed.
- Repo-root `AGENTS.md` contamination is blocked because the worker `cwd` is outside the repo tree.
- Home-tree `AGENTS.md` contamination is blocked because ancestor traversal from the worker `cwd` cannot enter the current user HOME tree.
- `~/.codex/AGENTS.md` contamination is blocked because the worker `HOME` is the isolated `home/` directory rather than the user home.
- Within the isolated execution root, P07 does not create any additional parent-level `AGENTS.md` above `<isolated_root>/workspace/AGENTS.md`.
- `outgoing_input.json` is the stable run-tree seal record even if the isolated execution workspace is later cleaned up or recreated.
- P07 does not launch a worker, open an app-server session, write result artifacts, or redefine P08/P09 ownership.

# Handoff

- P08 consumes the sealed Stage 2 contract from P07:
  - stable audit artifacts under `<run_root>/02_answer/personas/<persona_id>/...`
  - returned isolated execution metadata (`cwd`, `HOME`, copied skill path, copied prompt path, copied AGENTS path)
- P08 must execute the worker from the isolated workspace prepared by P07 and must not reconstruct Stage 2 inputs by rereading `dispatch_input_v1.json`.
- P09 may orchestrate multiple sealed persona workspaces later, but it must not redefine the P07 seal-chain fields or the isolated-workspace contamination boundary.
