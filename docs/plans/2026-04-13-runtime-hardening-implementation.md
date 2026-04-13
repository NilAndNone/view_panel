# Runtime Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden the current runtime prototype so the transport contract is evidence-based, the pinned protocol bundle is live rather than placeholder, Stage 2 retries are real, and multi-material input is deterministic and explicit.

**Architecture:** Keep the existing three-stage runtime and artifact names intact. Add hardening in four thin slices: transport decoding inside `internal/appserver`, protocol-bundle capture plus stronger P01 enforcement, additive Stage 2 retry artifacts inside `internal/stage/answer`, and a deterministic multi-material merge layer inside `internal/materials` consumed by startup/prepare.

**Tech Stack:** Go, stdlib `os/exec`, JSON/JSON-RPC, existing `internal/appserver`, existing `internal/runtimecontract`, existing `internal/stage/answer`, existing `internal/materials`, `make smoke`, `go test`

---

## File Structure

### New files

- `internal/appserver/stream.go`
  - transport stream detection and decoding (`framed stdio` vs sequential JSON fallback)
- `internal/appserver/stream_test.go`
  - transport decoding coverage with frozen fixtures
- `internal/appserver/testdata/framed_initialize.txt`
  - captured minimal framed response sample
- `internal/appserver/testdata/sequential_initialize.json`
  - sequential JSON sample for backward-compat fallback
- `tools/capture_protocol_bundle/main.go`
  - captures a live protocol bundle and smoke transcript for the pinned CLI version
- `internal/materials/merge.go`
  - deterministic merge contract over multiple canonical material files
- `internal/materials/merge_test.go`
  - merge semantics and conflict coverage

### Existing files to modify

- `internal/appserver/client.go`
  - switch read loop to stream decoder abstraction
- `internal/appserver/messages.go`
  - only if decoding helpers need small type support
- `internal/runtimecontract/check.go`
  - strengthen bundle checks against live-captured bundle expectations
- `internal/runtimecontract/check_test.go`
  - live-bundle file expectations and contract warnings
- `third_party/codex-protocol/0.0.0/openapi.json`
- `third_party/codex-protocol/0.0.0/messages.json`
- `third_party/codex-protocol/0.0.0/schema-index.json`
- `third_party/codex-protocol/0.0.0/smoke-transcript.jsonl`
  - replace placeholder payloads with captured content
- `internal/stage/answer/batch.go`
  - real multi-attempt execution with additive per-attempt artifacts
- `internal/stage/answer/batch_test.go`
  - retry semantics, timeout semantics, forbidden-tool behavior over retries
- `internal/config/config.go`
  - relax `max_attempts_per_persona` from fixed `1` to real supported range
- `internal/schema/validate.go`
  - validate the supported retry range and merge-related startup assumptions
- `internal/schema/validate_test.go`
  - startup validation coverage
- `internal/materials/loader.go`
  - consume merge contract instead of “pick one canonical file”
- `cmd/worldview-panel/main.go`
  - wire material merge path and retry policy into the runtime
- `Makefile`
  - add capture/refresh helpers if needed
- `README.md`
  - document transport support, live bundle status, retry semantics, and material merge rules
- `docs/operations/codex-runtime-contract.md`
  - update from placeholder language to captured-bundle language
- `docs/operations/runtime-smoke-and-release-checklist.md`
  - update smoke/refresh instructions
- `AGENTS.md`
  - update Codex maintainer guidance for transport, retry, and materials merge

### Files intentionally not restructured

- `internal/orchestrator/run.go`
- `internal/storage/*`
- `internal/hash/*`
- `internal/stage/render/*`
- existing Stage 1/2/3 canonical artifact filenames

---

### Task 1: Capture transport evidence and add a real stream decoder

**Files:**
- Create: `internal/appserver/stream.go`
- Create: `internal/appserver/stream_test.go`
- Create: `internal/appserver/testdata/framed_initialize.txt`
- Create: `internal/appserver/testdata/sequential_initialize.json`
- Modify: `internal/appserver/client.go`

- [ ] **Step 1: Write the failing decoder tests**

```go
package appserver

import (
    "bytes"
    "os"
    "path/filepath"
    "testing"
)

func TestDecodeStreamHandlesFramedJSONRPC(t *testing.T) {
    data, err := os.ReadFile(filepath.Join("testdata", "framed_initialize.txt"))
    if err != nil {
        t.Fatal(err)
    }

    decoder, err := NewStreamDecoder(bytes.NewReader(data))
    if err != nil {
        t.Fatalf("NewStreamDecoder returned error: %v", err)
    }

    var env rawEnvelope
    if err := decoder.Decode(&env); err != nil {
        t.Fatalf("Decode returned error: %v", err)
    }
    if env.JSONRPC != "2.0" {
        t.Fatalf("jsonrpc = %q, want %q", env.JSONRPC, "2.0")
    }
}

func TestDecodeStreamFallsBackToSequentialJSON(t *testing.T) {
    data, err := os.ReadFile(filepath.Join("testdata", "sequential_initialize.json"))
    if err != nil {
        t.Fatal(err)
    }

    decoder, err := NewStreamDecoder(bytes.NewReader(data))
    if err != nil {
        t.Fatalf("NewStreamDecoder returned error: %v", err)
    }

    var env rawEnvelope
    if err := decoder.Decode(&env); err != nil {
        t.Fatalf("Decode returned error: %v", err)
    }
    if env.Method == "" && len(env.Result) == 0 {
        t.Fatalf("decoded envelope is empty")
    }
}
```

- [ ] **Step 2: Run the decoder tests to verify they fail**

Run: `go test ./internal/appserver -run 'TestDecodeStreamHandlesFramedJSONRPC|TestDecodeStreamFallsBackToSequentialJSON' -v`
Expected: FAIL because `NewStreamDecoder` does not exist yet.

- [ ] **Step 3: Implement stream detection and decoding**

```go
package appserver

import (
    "bufio"
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "strconv"
    "strings"
)

type StreamDecoder interface {
    Decode(v any) error
}

type streamDecoder struct {
    sequential *json.Decoder
    framed     *framedDecoder
}

func NewStreamDecoder(r io.Reader) (StreamDecoder, error) {
    buffered := bufio.NewReader(r)
    peek, err := buffered.Peek(32)
    if err != nil && err != io.EOF {
        return nil, err
    }
    if looksLikeFramedHeader(peek) {
        return &streamDecoder{framed: newFramedDecoder(buffered)}, nil
    }
    dec := json.NewDecoder(buffered)
    dec.UseNumber()
    return &streamDecoder{sequential: dec}, nil
}

func (d *streamDecoder) Decode(v any) error {
    switch {
    case d.framed != nil:
        return d.framed.Decode(v)
    default:
        return d.sequential.Decode(v)
    }
}

func looksLikeFramedHeader(peek []byte) bool {
    trimmed := strings.TrimLeft(string(peek), "\r\n\t ")
    return strings.HasPrefix(strings.ToLower(trimmed), "content-length:")
}

type framedDecoder struct {
    reader *bufio.Reader
}

func newFramedDecoder(reader *bufio.Reader) *framedDecoder {
    return &framedDecoder{reader: reader}
}

func (d *framedDecoder) Decode(v any) error {
    payload, err := d.readFrame()
    if err != nil {
        return err
    }
    return json.Unmarshal(payload, v)
}

func (d *framedDecoder) readFrame() ([]byte, error) {
    contentLength := -1
    for {
        line, err := d.reader.ReadString('\n')
        if err != nil {
            return nil, err
        }
        line = strings.TrimRight(line, "\r\n")
        if line == "" {
            break
        }
        key, value, found := strings.Cut(line, ":")
        if !found {
            return nil, fmt.Errorf("invalid frame header %q", line)
        }
        if strings.EqualFold(strings.TrimSpace(key), "Content-Length") {
            parsed, err := strconv.Atoi(strings.TrimSpace(value))
            if err != nil {
                return nil, fmt.Errorf("invalid content length %q: %w", value, err)
            }
            contentLength = parsed
        }
    }
    if contentLength < 0 {
        return nil, fmt.Errorf("missing Content-Length header")
    }
    payload := make([]byte, contentLength)
    if _, err := io.ReadFull(d.reader, payload); err != nil {
        return nil, err
    }
    return bytes.Clone(payload), nil
}
```

- [ ] **Step 4: Integrate the stream decoder into the client read loop**

```go
func (c *Client) readLoop() {
    defer close(c.readerDone)

    decoder, err := NewStreamDecoder(c.stdout)
    if err != nil {
        c.finishReader(err)
        return
    }

    for {
        var envelope rawEnvelope
        if err := decoder.Decode(&envelope); err != nil {
            c.finishReader(err)
            return
        }
        // existing routing logic stays unchanged
    }
}
```

- [ ] **Step 5: Run the appserver package tests**

Run: `go test ./internal/appserver -v`
Expected: PASS, including the new stream-decoder tests.

- [ ] **Step 6: Commit**

```bash
git add internal/appserver/stream.go internal/appserver/stream_test.go internal/appserver/testdata/framed_initialize.txt internal/appserver/testdata/sequential_initialize.json internal/appserver/client.go
git commit -m "feat: support framed app-server transport"
```

### Task 2: Replace the placeholder protocol bundle with a live captured bundle

**Files:**
- Create: `tools/capture_protocol_bundle/main.go`
- Modify: `third_party/codex-protocol/0.0.0/openapi.json`
- Modify: `third_party/codex-protocol/0.0.0/messages.json`
- Modify: `third_party/codex-protocol/0.0.0/schema-index.json`
- Modify: `third_party/codex-protocol/0.0.0/smoke-transcript.jsonl`
- Modify: `internal/runtimecontract/check.go`
- Modify: `internal/runtimecontract/check_test.go`
- Modify: `docs/operations/codex-runtime-contract.md`
- Modify: `Makefile`

- [ ] **Step 1: Write the failing runtime-contract expectation tests**

```go
func TestCheckDoesNotWarnWhenBundleIsLiveCaptured(t *testing.T) {
    checker := Checker{
        RepositoryRoot:           t.TempDir(),
        PinnedCLIVersion:         DefaultPinnedCLIVersion,
        PinnedLauncherForm:       CanonicalLauncherForm,
        LauncherForm:             CanonicalLauncherForm,
        LookPathFunc:             func(string) (string, error) { return "/usr/bin/codex", nil },
        ReadVersionFunc:          func(string) (string, error) { return DefaultPinnedCLIVersion, nil },
        TransportFramingVerified: true,
        IsPlaceholderBundleFunc:  func(string) bool { return false },
        BundleFileExistsFunc:     func(string) bool { return true },
    }

    result := checker.Check()
    for _, warning := range result.Warnings {
        if warning == WarningPlaceholderBundle {
            t.Fatalf("unexpected placeholder warning in live bundle path")
        }
    }
}
```

- [ ] **Step 2: Run the runtime-contract tests to verify the new live-bundle expectation fails**

Run: `go test ./internal/runtimecontract -run 'TestCheckDoesNotWarnWhenBundleIsLiveCaptured' -v`
Expected: FAIL until the live-bundle helpers exist.

- [ ] **Step 3: Add a bundle-capture tool**

```go
package main

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"

    "view_panel/internal/appserver"
)

func main() {
    repoRoot := mustRepoRoot()
    version := mustObservedCodexVersion()
    bundleRoot := filepath.Join(repoRoot, "third_party", "codex-protocol", version)
    must(os.MkdirAll(bundleRoot, 0o755))

    capture := mustCaptureProtocol(context.Background())
    must(writeJSON(filepath.Join(bundleRoot, "messages.json"), capture.Messages))
    must(writeJSON(filepath.Join(bundleRoot, "schema-index.json"), capture.SchemaIndex))
    must(writeJSON(filepath.Join(bundleRoot, "openapi.json"), capture.OpenAPI))
    must(writeJSONL(filepath.Join(bundleRoot, "smoke-transcript.jsonl"), capture.Transcript))
}
```

- [ ] **Step 4: Strengthen runtime-contract checks and Makefile helpers**

```go
func (c Checker) Check() Result {
    // existing blocking checks stay
    if c.IsPlaceholderBundleFunc(c.ProtocolBundlePath) {
        result.Warnings = append(result.Warnings, WarningPlaceholderBundle)
    }
    if !c.TransportFramingVerified {
        result.Warnings = append(result.Warnings, WarningSequentialJSONTransport)
    }
    // once capture tool exists, the default checked-in path should make placeholder warning disappear
}
```

```make
capture-protocol:
	go run ./tools/capture_protocol_bundle
```

- [ ] **Step 5: Run the capture tool and refresh the checked-in bundle**

Run: `make capture-protocol`
Expected: refreshed files under `third_party/codex-protocol/0.0.0/` with non-placeholder content.

- [ ] **Step 6: Run verification**

Run: `go test ./internal/runtimecontract -v`
Expected: PASS, with the placeholder warning now only appearing in tests that intentionally inject placeholder bundles.

- [ ] **Step 7: Commit**

```bash
git add tools/capture_protocol_bundle/main.go third_party/codex-protocol/0.0.0 internal/runtimecontract/check.go internal/runtimecontract/check_test.go docs/operations/codex-runtime-contract.md Makefile
git commit -m "feat: capture live codex protocol bundle"
```

### Task 3: Implement real Stage 2 retry semantics

**Files:**
- Modify: `internal/stage/answer/batch.go`
- Modify: `internal/stage/answer/batch_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/schema/validate.go`
- Modify: `internal/schema/validate_test.go`
- Modify: `cmd/worldview-panel/main.go`
- Modify: `README.md`

- [ ] **Step 1: Write the failing retry tests**

```go
func TestBatchRunnerRetriesUntilSecondAttemptCertifies(t *testing.T) {
    runner := stubExecutor{
        Results: []ExecuteWorkerResult{
            failedResult("persona-a"),
            certifiedResult("persona-a"),
        },
    }

    batch := BatchRunner{Runner: runner}
    result, err := batch.Execute(context.Background(), AnswerBatchRequest{
        RunID:                 "run-test",
        AnswerRoot:            t.TempDir(),
        Workspaces:            []SealedPersonaWorkspace{{PersonaID: "persona-a"}},
        MaxConcurrency:        1,
        MaxAttemptsPerPersona: 2,
    })
    if err != nil {
        t.Fatalf("Execute returned error: %v", err)
    }
    if len(result.Certified) != 1 || result.Certified[0].AttemptCount != 2 {
        t.Fatalf("expected certified second attempt, got %#v", result)
    }
}

func TestBatchRunnerStopsAfterConfiguredAttempts(t *testing.T) {
    runner := stubExecutor{
        Results: []ExecuteWorkerResult{
            failedResult("persona-a"),
            failedResult("persona-a"),
        },
    }

    batch := BatchRunner{Runner: runner}
    result, err := batch.Execute(context.Background(), AnswerBatchRequest{
        RunID:                 "run-test",
        AnswerRoot:            t.TempDir(),
        Workspaces:            []SealedPersonaWorkspace{{PersonaID: "persona-a"}},
        MaxConcurrency:        1,
        MaxAttemptsPerPersona: 2,
    })
    if err != nil {
        t.Fatalf("Execute returned error: %v", err)
    }
    if len(result.Failed) != 1 || result.Failed[0].AttemptCount != 2 {
        t.Fatalf("expected failed second attempt summary, got %#v", result)
    }
}
```

- [ ] **Step 2: Run the answer-batch tests to verify they fail**

Run: `go test ./internal/stage/answer -run 'TestBatchRunnerRetriesUntilSecondAttemptCertifies|TestBatchRunnerStopsAfterConfiguredAttempts' -v`
Expected: FAIL because the batch runner hardcodes a single attempt.

- [ ] **Step 3: Implement additive retry execution and attempt artifacts**

```go
func (b BatchRunner) executeOne(parent context.Context, req AnswerBatchRequest, workspace SealedPersonaWorkspace, forbiddenToolNames []string) batchPersonaOutcome {
    maxAttempts := req.MaxAttemptsPerPersona
    if maxAttempts < 1 {
        maxAttempts = 1
    }

    var lastOutcome batchPersonaOutcome
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        outcome := b.executeAttempt(parent, req, workspace, forbiddenToolNames, attempt)
        lastOutcome = outcome
        if outcome.Certified != nil || outcome.Rejected != nil {
            return outcome
        }
        if parent.Err() != nil {
            return outcome
        }
    }
    return lastOutcome
}
```

- [ ] **Step 4: Relax startup validation to support real retry values**

```go
if cfg.MaxAttemptsPerPersona < 1 {
    diagnostics = append(diagnostics, Diagnostic{
        Code: "range",
        Field: "max_attempts_per_persona",
        Message: "max_attempts_per_persona must be >= 1",
    })
}
```

- [ ] **Step 5: Run verification**

Run: `go test ./internal/stage/answer ./internal/schema ./cmd/worldview-panel -v`
Expected: PASS, including retry coverage and startup validation updates.

- [ ] **Step 6: Commit**

```bash
git add internal/stage/answer/batch.go internal/stage/answer/batch_test.go internal/config/config.go internal/schema/validate.go internal/schema/validate_test.go cmd/worldview-panel/main.go README.md
git commit -m "feat: add stage2 retry semantics"
```

### Task 4: Replace “pick one material” with a deterministic merge contract

**Files:**
- Create: `internal/materials/merge.go`
- Create: `internal/materials/merge_test.go`
- Modify: `internal/materials/loader.go`
- Modify: `cmd/worldview-panel/main.go`
- Modify: `internal/schema/validate.go`
- Modify: `README.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: Write the failing merge tests**

```go
func TestMergeCanonicalFieldsConcatenatesSupplementaryMaterialsInInputOrder(t *testing.T) {
    merged, err := MergeCanonicalFields([]CanonicalFields{
        {
            RoleplayPrompt:            "r",
            DiscussionQuestion:        "q",
            SupplementaryMaterials:    "a",
            OutputContract:            "o",
            AssumptionsAndConstraints: "c",
        },
        {
            SupplementaryMaterials: "b",
        },
    })
    if err != nil {
        t.Fatalf("MergeCanonicalFields returned error: %v", err)
    }
    if merged.SupplementaryMaterials != "a\n\n"+"b" {
        t.Fatalf("supplementary_materials = %q", merged.SupplementaryMaterials)
    }
}

func TestMergeCanonicalFieldsRejectsConflictingDiscussionQuestion(t *testing.T) {
    _, err := MergeCanonicalFields([]CanonicalFields{{DiscussionQuestion: "q1"}, {DiscussionQuestion: "q2"}})
    if err == nil {
        t.Fatalf("expected conflict error")
    }
}
```

- [ ] **Step 2: Run the materials tests to verify they fail**

Run: `go test ./internal/materials -run 'TestMergeCanonicalFields' -v`
Expected: FAIL because the merge layer does not exist yet.

- [ ] **Step 3: Implement explicit merge semantics**

```go
package materials

import (
    "fmt"
    "strings"
)

func MergeCanonicalFields(items []CanonicalFields) (CanonicalFields, error) {
    var merged CanonicalFields
    var supplementary []string

    setSingle := func(name string, current *string, next string) error {
        next = strings.TrimSpace(next)
        if next == "" {
            return nil
        }
        if *current == "" {
            *current = next
            return nil
        }
        if *current != next {
            return fmt.Errorf("conflicting %s", name)
        }
        return nil
    }

    for _, item := range items {
        if err := setSingle("roleplay_prompt", &merged.RoleplayPrompt, item.RoleplayPrompt); err != nil {
            return CanonicalFields{}, err
        }
        if err := setSingle("discussion_question", &merged.DiscussionQuestion, item.DiscussionQuestion); err != nil {
            return CanonicalFields{}, err
        }
        if err := setSingle("output_contract", &merged.OutputContract, item.OutputContract); err != nil {
            return CanonicalFields{}, err
        }
        if err := setSingle("assumptions_and_constraints", &merged.AssumptionsAndConstraints, item.AssumptionsAndConstraints); err != nil {
            return CanonicalFields{}, err
        }
        if trimmed := strings.TrimSpace(item.SupplementaryMaterials); trimmed != "" {
            supplementary = append(supplementary, trimmed)
        }
    }

    merged.SupplementaryMaterials = strings.Join(supplementary, "\n\n")
    return merged, nil
}
```

- [ ] **Step 4: Wire merged materials into startup/prepare**

```go
func loadPrepareMaterials(ctx context.Context, paths []string) (materials.CanonicalFields, error) {
    resolvedPaths, err := selectMaterialsPaths(paths)
    if err != nil {
        return materials.CanonicalFields{}, err
    }

    loader := materials.Loader{}
    loaded := make([]materials.CanonicalFields, 0, len(resolvedPaths))
    for _, path := range resolvedPaths {
        fields, err := loader.LoadFile(path)
        if err != nil {
            return materials.CanonicalFields{}, err
        }
        loaded = append(loaded, fields)
    }
    return materials.MergeCanonicalFields(loaded)
}
```

- [ ] **Step 5: Run verification**

Run: `go test ./internal/materials ./cmd/worldview-panel -v`
Expected: PASS, with the merged-material path exercised.

- [ ] **Step 6: Commit**

```bash
git add internal/materials/merge.go internal/materials/merge_test.go internal/materials/loader.go cmd/worldview-panel/main.go internal/schema/validate.go README.md AGENTS.md
git commit -m "feat: add deterministic materials merge"
```

### Task 5: Update smoke, docs, and retained plan docs for the hardened runtime

**Files:**
- Modify: `README.md`
- Modify: `AGENTS.md`
- Modify: `docs/operations/codex-runtime-contract.md`
- Modify: `docs/operations/runtime-smoke-and-release-checklist.md`
- Modify: `docs/plans/tech-plan-runtime/01-codex-runtime-contract.md`
- Modify: `docs/plans/tech-plan-runtime/02-app-server-client-lifecycle.md`
- Modify: `docs/plans/tech-plan-runtime/09-answer-batch-orchestration.md`
- Modify: `docs/plans/tech-plan-runtime/12-end-to-end-smoke-and-makefile-flow.md`
- Modify: `Makefile`

- [ ] **Step 1: Update runtime docs to reflect the hardened transport/bundle/retry/materials behavior**

```md
## Runtime Contract

Startup now enforces:
- pinned CLI version
- canonical launcher form
- presence of a live captured protocol bundle

Transport decoding supports:
- framed stdio with `Content-Length` headers
- sequential JSON fallback only for backward compatibility tests
```

- [ ] **Step 2: Extend smoke so it exercises the hardened path**

```make
smoke:
	@$(MAKE) build
	@$(MAKE) capture-protocol
	@# existing smoke flow continues unchanged
```

- [ ] **Step 3: Run smoke and docs verification**

Run: `rm -rf out/smoke && make smoke`
Expected: PASS and `smoke check passed: <run_root>`.

Run: `latest_run=$(find out/smoke/runs -mindepth 1 -maxdepth 1 -type d | sort | tail -n 1) && jq . "$latest_run/audit/runtime_contract_status.json"`
Expected: valid JSON with non-empty `observed_cli_version`, `pinned_cli_version`, and no placeholder warning once the live bundle is captured.

- [ ] **Step 4: Commit**

```bash
git add README.md AGENTS.md docs/operations docs/plans/tech-plan-runtime Makefile
git commit -m "docs: align runtime docs with hardened behavior"
```

### Task 6: Final verification and handoff

**Files:**
- Modify: none expected
- Verify: repository state only

- [ ] **Step 1: Run the full verification set**

Run: `go test ./...`
Expected: PASS across all packages.

Run: `rm -rf out/smoke && make smoke`
Expected: PASS with a fresh smoke run root.

- [ ] **Step 2: Verify retained runtime plan alignment**

Run: `rg -n 'placeholder|sequential JSON|max_attempts_per_persona currently supports only 1|pick one material' docs/plans/tech-plan-runtime README.md AGENTS.md docs/operations`
Expected: no stale wording that contradicts the hardened implementation.

- [ ] **Step 3: Inspect scope control**

Run: `git status --short && git diff --stat`
Expected: only transport, runtime contract, answer retry, materials merge, smoke, and docs files changed.

- [ ] **Step 4: Create the final commit**

```bash
git add internal/appserver internal/runtimecontract internal/stage/answer internal/materials internal/config internal/schema cmd/worldview-panel README.md AGENTS.md docs/operations docs/plans/tech-plan-runtime Makefile third_party/codex-protocol/0.0.0 tools/capture_protocol_bundle
git commit -m "feat: harden runtime transport and material semantics"
```

- [ ] **Step 5: Request review before merge**

Run a repository review focused on:
- framed transport correctness
- live protocol bundle integrity
- retry artifact semantics and backward compatibility
- deterministic materials merge behavior
- smoke stability after hardening

Expected: findings addressed before merge.
