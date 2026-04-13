# Content Product Line Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a separate content workflow that authors richer persona/domain content, compiles it into runtime-ready bundles, and evaluates persona distinctness without reopening the runtime architecture.

**Architecture:** Keep the current runtime unchanged as the execution engine. Add a new `content/src` authoring tree, a compiler in `internal/content` plus a small CLI tool in `tools/build_content_bundle`, and an evaluator in `tools/eval_content_distinctness` that runs the existing CLI against compiled bundles and writes quality reports.

**Tech Stack:** Go, stdlib JSON/Markdown file handling, existing `cmd/worldview-panel`, existing runtime five-field material contract, existing `internal/persona`, `make`, `go test`

---

## File Structure

### New files

- `content/src/personas/index.json`
- `content/src/personas/analyst/meta.json`
- `content/src/personas/analyst/profile.md`
- `content/src/personas/analyst/psychology.md`
- `content/src/personas/analyst/anti_patterns.md`
- `content/src/personas/analyst/domains/technology.md`
- `content/src/personas/analyst/domains/operations.md`
- `content/src/personas/builder/meta.json`
- `content/src/personas/builder/profile.md`
- `content/src/personas/builder/psychology.md`
- `content/src/personas/builder/anti_patterns.md`
- `content/src/personas/builder/domains/product.md`
- `content/src/personas/builder/domains/execution.md`
- `content/src/personas/historian/meta.json`
- `content/src/personas/historian/profile.md`
- `content/src/personas/historian/psychology.md`
- `content/src/personas/historian/anti_patterns.md`
- `content/src/personas/historian/domains/history.md`
- `content/src/personas/historian/domains/politics.md`
- `content/src/personas/operator/meta.json`
- `content/src/personas/operator/profile.md`
- `content/src/personas/operator/psychology.md`
- `content/src/personas/operator/anti_patterns.md`
- `content/src/personas/operator/domains/operations.md`
- `content/src/personas/operator/domains/risk.md`
- `content/src/personas/skeptic/meta.json`
- `content/src/personas/skeptic/profile.md`
- `content/src/personas/skeptic/psychology.md`
- `content/src/personas/skeptic/anti_patterns.md`
- `content/src/personas/skeptic/domains/critique.md`
- `content/src/personas/skeptic/domains/science.md`
- `content/src/personas/policy/meta.json`
- `content/src/personas/policy/profile.md`
- `content/src/personas/policy/psychology.md`
- `content/src/personas/policy/anti_patterns.md`
- `content/src/personas/policy/domains/governance.md`
- `content/src/personas/policy/domains/institutions.md`
- `content/src/materials/technology.json`
- `content/src/materials/operations.json`
- `content/src/materials/policy.json`
- `content/src/evals/distinctness/questions.json`
- `schemas/persona-source-v1.schema.json`
- `schemas/domain-material-source-v1.schema.json`
- `schemas/content-bundle-manifest-v1.schema.json`
- `internal/content/catalog.go`
- `internal/content/catalog_test.go`
- `internal/content/compile.go`
- `internal/content/compile_test.go`
- `tools/build_content_bundle/main.go`
- `tools/eval_content_distinctness/main.go`
- `docs/operations/content-authoring-guide.md`

### Existing files to modify

- `Makefile`
- `README.md`
- `AGENTS.md`
- `testdata/smoke/runtime/personas/analyst.json`
- `testdata/smoke/runtime/personas/builder.json`

### Files intentionally not modified

- `internal/stage/*`
- `internal/orchestrator/run.go`
- `internal/storage/*`
- `internal/hash/*`
- `internal/appserver/*`

---

### Task 1: Define the content source schema and authoring tree

**Files:**
- Create: `schemas/persona-source-v1.schema.json`
- Create: `schemas/domain-material-source-v1.schema.json`
- Create: `content/src/personas/index.json`
- Create: `content/src/personas/analyst/meta.json`
- Create: `content/src/personas/analyst/profile.md`
- Create: `content/src/personas/analyst/psychology.md`
- Create: `content/src/personas/analyst/anti_patterns.md`
- Create: `content/src/personas/analyst/domains/technology.md`
- Create: `content/src/personas/analyst/domains/operations.md`
- Create: `content/src/materials/technology.json`
- Create: `docs/operations/content-authoring-guide.md`

- [ ] **Step 1: Write the failing schema/fixture tests**

```go
func TestPersonaSourceFixtureMatchesSchemaExpectations(t *testing.T) {
    catalog, err := content.LoadCatalog("content/src")
    if err != nil {
        t.Fatalf("LoadCatalog returned error: %v", err)
    }
    if len(catalog.Personas) == 0 {
        t.Fatalf("expected at least one persona source pack")
    }
    analyst, ok := catalog.Personas["analyst"]
    if !ok {
        t.Fatalf("expected analyst persona in source catalog")
    }
    if analyst.Meta.PersonaID != "analyst" {
        t.Fatalf("persona_id = %q, want %q", analyst.Meta.PersonaID, "analyst")
    }
    if len(analyst.DomainBodies) == 0 {
        t.Fatalf("expected analyst domain bodies to be loaded")
    }
}
```

- [ ] **Step 2: Run the content loader test to verify it fails**

Run: `go test ./internal/content -run TestPersonaSourceFixtureMatchesSchemaExpectations -v`
Expected: FAIL because `internal/content` and the source tree do not exist yet.

- [ ] **Step 3: Add source schema files and the first authoring fixtures**

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "persona-source-v1",
  "type": "object",
  "additionalProperties": false,
  "required": [
    "persona_id",
    "display_name",
    "core_lens",
    "decision_style",
    "tone",
    "default_bias",
    "priority_stack",
    "strong_domains",
    "reference_anchors"
  ],
  "properties": {
    "persona_id": {"type": "string", "minLength": 1},
    "display_name": {"type": "string", "minLength": 1},
    "core_lens": {"type": "string", "minLength": 1},
    "decision_style": {"type": "string", "minLength": 1},
    "tone": {"type": "string", "minLength": 1},
    "default_bias": {"type": "string", "minLength": 1},
    "priority_stack": {"type": "array", "minItems": 3, "items": {"type": "string", "minLength": 1}},
    "strong_domains": {"type": "array", "minItems": 2, "items": {"type": "string", "minLength": 1}},
    "reference_anchors": {"type": "array", "minItems": 2, "items": {"type": "string", "minLength": 1}}
  }
}
```

```json
{
  "personas": [
    "analyst",
    "builder",
    "historian",
    "operator",
    "skeptic",
    "policy"
  ]
}
```

```json
{
  "persona_id": "analyst",
  "display_name": "Analyst",
  "core_lens": "structured risk analysis under uncertainty",
  "decision_style": "decompose claims, compare scenarios, rank failure modes",
  "tone": "clear, sober, operationally precise",
  "default_bias": "prefer preventing avoidable downside over chasing speculative upside",
  "priority_stack": ["clarity", "evidence", "containment"],
  "strong_domains": ["technology", "operations"],
  "reference_anchors": ["postmortems", "operational risk reviews", "decision memos"]
}
```

```md
# Analyst profile

You reason by decomposing a messy question into tractable risks, assumptions, and operational branches.

You distrust hand-wavy optimism and vague “strategy” language unless it resolves into concrete decisions.
```

- [ ] **Step 4: Implement the authoring guide with a strict template**

```md
# Content Authoring Guide

Every persona source pack must contain:
- `meta.json`
- `profile.md`
- `psychology.md`
- `anti_patterns.md`
- `domains/<domain>.md` for every declared `strong_domains` entry

Do not write generic self-help language.
Do not write “balanced” summaries that erase the persona's bias.
Every domain file must contain concrete anchors: examples, thinkers, incidents, or institutions.
```

- [ ] **Step 5: Run the loader test and verify the fixture tree passes**

Run: `go test ./internal/content -run TestPersonaSourceFixtureMatchesSchemaExpectations -v`
Expected: PASS after Task 2 adds `LoadCatalog`.

- [ ] **Step 6: Commit**

```bash
git add schemas/persona-source-v1.schema.json schemas/domain-material-source-v1.schema.json content/src docs/operations/content-authoring-guide.md internal/content
git commit -m "feat: add content source schema and authoring fixtures"
```

### Task 2: Implement the content catalog loader and compiler

**Files:**
- Create: `internal/content/catalog.go`
- Create: `internal/content/catalog_test.go`
- Create: `internal/content/compile.go`
- Create: `internal/content/compile_test.go`
- Create: `schemas/content-bundle-manifest-v1.schema.json`

- [ ] **Step 1: Write the failing catalog/compiler tests**

```go
func TestCompileBundleEmitsRuntimeCompatiblePersonaSet(t *testing.T) {
    catalog, err := content.LoadCatalog("content/src")
    if err != nil {
        t.Fatal(err)
    }

    bundle, err := content.CompileBundle(catalog, content.CompileOptions{BundleID: "default"})
    if err != nil {
        t.Fatalf("CompileBundle returned error: %v", err)
    }

    if len(bundle.Personas) == 0 {
        t.Fatalf("expected compiled personas")
    }
    analyst, ok := bundle.Personas["analyst"]
    if !ok {
        t.Fatalf("missing compiled analyst persona")
    }
    if analyst.Fields["profile_long"] == "" {
        t.Fatalf("expected profile_long field")
    }
    if analyst.Fields["domain_technology"] == "" {
        t.Fatalf("expected domain_technology field")
    }
}
```

- [ ] **Step 2: Run the compiler tests to verify they fail**

Run: `go test ./internal/content -run 'TestCompileBundleEmitsRuntimeCompatiblePersonaSet' -v`
Expected: FAIL because the compiler does not exist yet.

- [ ] **Step 3: Implement the catalog loader with strict validation**

```go
type PersonaMeta struct {
    PersonaID        string   `json:"persona_id"`
    DisplayName      string   `json:"display_name"`
    CoreLens         string   `json:"core_lens"`
    DecisionStyle    string   `json:"decision_style"`
    Tone             string   `json:"tone"`
    DefaultBias      string   `json:"default_bias"`
    PriorityStack    []string `json:"priority_stack"`
    StrongDomains    []string `json:"strong_domains"`
    ReferenceAnchors []string `json:"reference_anchors"`
}

type PersonaSource struct {
    Meta          PersonaMeta
    ProfileBody   string
    Psychology    string
    AntiPatterns  string
    DomainBodies  map[string]string
}
```

- [ ] **Step 4: Implement deterministic compilation to runtime-compatible JSON**

```go
func compilePersona(source PersonaSource) map[string]any {
    fields := map[string]any{
        "persona_id":          source.Meta.PersonaID,
        "display_name":        source.Meta.DisplayName,
        "core_lens":           source.Meta.CoreLens,
        "decision_style":      source.Meta.DecisionStyle,
        "tone":                source.Meta.Tone,
        "default_bias":        source.Meta.DefaultBias,
        "priority_stack":      strings.Join(source.Meta.PriorityStack, " | "),
        "reference_anchors":   strings.Join(source.Meta.ReferenceAnchors, " | "),
        "profile_long":        strings.TrimSpace(source.ProfileBody),
        "psychology_summary":  strings.TrimSpace(source.Psychology),
        "anti_patterns":       strings.TrimSpace(source.AntiPatterns),
    }
    for _, domain := range sortedKeys(source.DomainBodies) {
        fields["domain_"+domain] = strings.TrimSpace(source.DomainBodies[domain])
    }
    return fields
}
```

- [ ] **Step 5: Emit a bundle manifest that records inputs and outputs**

```json
{
  "schema_version": "content_bundle_manifest_v1",
  "bundle_id": "default",
  "persona_count": 6,
  "material_count": 3,
  "artifacts": [
    "runtime/persona-index.json",
    "runtime/personas/analyst.json",
    "runtime/materials/technology.json"
  ]
}
```

- [ ] **Step 6: Run the content package tests**

Run: `go test ./internal/content -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/content schemas/content-bundle-manifest-v1.schema.json
git commit -m "feat: compile rich content packs into runtime bundles"
```

### Task 3: Add the bundle builder CLI and build outputs

**Files:**
- Create: `tools/build_content_bundle/main.go`
- Modify: `Makefile`
- Modify: `README.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: Write the failing builder smoke test**

```go
func TestBuildContentBundleWritesRuntimeOutputs(t *testing.T) {
    outDir := t.TempDir()
    if err := runBuildContentBundle("content/src", outDir, "default"); err != nil {
        t.Fatalf("runBuildContentBundle returned error: %v", err)
    }
    requireFile(t, filepath.Join(outDir, "runtime", "persona-index.json"))
    requireFile(t, filepath.Join(outDir, "runtime", "personas", "analyst.json"))
    requireFile(t, filepath.Join(outDir, "runtime", "materials", "technology.json"))
}
```

- [ ] **Step 2: Run the bundle builder test to verify it fails**

Run: `go test ./tools/build_content_bundle -run TestBuildContentBundleWritesRuntimeOutputs -v`
Expected: FAIL because the tool does not exist yet.

- [ ] **Step 3: Implement the bundle builder CLI**

```go
func main() {
    source := flag.String("source", "content/src", "content source root")
    out := flag.String("out", "content/build/default", "bundle output root")
    bundleID := flag.String("bundle-id", "default", "bundle identifier")
    flag.Parse()

    catalog, err := content.LoadCatalog(*source)
    must(err)

    bundle, err := content.CompileBundle(catalog, content.CompileOptions{BundleID: *bundleID})
    must(err)

    must(content.WriteBundle(*out, bundle))
}
```

- [ ] **Step 4: Add Makefile entrypoints**

```make
build-content:
	go run ./tools/build_content_bundle -source content/src -out content/build/default -bundle-id default

content-smoke-build:
	$(MAKE) build-content
	@test -f content/build/default/runtime/persona-index.json
```

- [ ] **Step 5: Document how compiled bundles feed the current runtime**

```md
Build the content bundle:

```sh
make build-content
```

Run the panel with compiled content:

```sh
./go_bin -persona-set ./content/build/default/runtime/persona-index.json -materials ./content/build/default/runtime/materials/technology.json -question "..."
```
```

- [ ] **Step 6: Run the builder tests**

Run: `go test ./tools/build_content_bundle ./internal/content -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add tools/build_content_bundle Makefile README.md AGENTS.md
git commit -m "feat: add content bundle build tool"
```

### Task 4: Add a distinctness evaluator over real panel runs

**Files:**
- Create: `content/src/evals/distinctness/questions.json`
- Create: `tools/eval_content_distinctness/main.go`
- Modify: `Makefile`
- Modify: `README.md`
- Modify: `docs/operations/content-authoring-guide.md`

- [ ] **Step 1: Write the failing evaluator test**

```go
func TestScoreRunFlagsNearDuplicatePersonaOutputs(t *testing.T) {
    report := scoreOutputs([]personaOutput{
        {PersonaID: "analyst", Text: "We should reduce downside by sequencing rollout carefully."},
        {PersonaID: "builder", Text: "We should reduce downside by sequencing rollout carefully."},
    })
    if report.NearDuplicateCount == 0 {
        t.Fatalf("expected near-duplicate outputs to be flagged")
    }
}
```

- [ ] **Step 2: Run the evaluator test to verify it fails**

Run: `go test ./tools/eval_content_distinctness -run TestScoreRunFlagsNearDuplicatePersonaOutputs -v`
Expected: FAIL because the evaluator does not exist yet.

- [ ] **Step 3: Implement a first-pass distinctness scorer**

```go
type DistinctnessReport struct {
    BundleID            string `json:"bundle_id"`
    QuestionSetPath     string `json:"question_set_path"`
    RunCount            int    `json:"run_count"`
    NearDuplicateCount  int    `json:"near_duplicate_count"`
    MissingAnchorCount  int    `json:"missing_anchor_count"`
    GenericFramingCount int    `json:"generic_framing_count"`
}
```

Use simple deterministic heuristics first:
- normalized exact-match or high-overlap text => near duplicate
- missing explicit anchors from `reference_anchors` / domain bodies => missing anchor
- stock phrases like `it depends`, `balanced perspective`, `both sides` without concrete detail => generic framing

- [ ] **Step 4: Run the real evaluator against compiled content**

```sh
make build-content
go run ./tools/eval_content_distinctness \
  -bundle ./content/build/default \
  -questions ./content/src/evals/distinctness/questions.json \
  -out ./content/build/default/evals/latest
```

Expected: writes `distinctness-report.json` and `distinctness-report.md`.

- [ ] **Step 5: Add Makefile entrypoint**

```make
content-eval:
	go run ./tools/eval_content_distinctness -bundle ./content/build/default -questions ./content/src/evals/distinctness/questions.json -out ./content/build/default/evals/latest
```

- [ ] **Step 6: Run evaluator tests**

Run: `go test ./tools/eval_content_distinctness -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add content/src/evals tools/eval_content_distinctness Makefile README.md docs/operations/content-authoring-guide.md
git commit -m "feat: add content distinctness evaluator"
```

### Task 5: Author the initial production cohort and align smoke fixtures

**Files:**
- Create: all remaining `content/src/personas/<id>/*` files listed above
- Create: `content/src/materials/operations.json`
- Create: `content/src/materials/policy.json`
- Modify: `testdata/smoke/runtime/personas/analyst.json`
- Modify: `testdata/smoke/runtime/personas/builder.json`
- Modify: `content/src/personas/index.json`

- [ ] **Step 1: Replace thin persona fixtures with compiled-output parity tests**

```go
func TestCompiledAnalystPersonaIsRicherThanSmokeFixture(t *testing.T) {
    catalog, err := content.LoadCatalog("content/src")
    if err != nil {
        t.Fatal(err)
    }
    bundle, err := content.CompileBundle(catalog, content.CompileOptions{BundleID: "default"})
    if err != nil {
        t.Fatal(err)
    }
    analyst := bundle.Personas["analyst"]
    if len(fmt.Sprint(analyst.Fields["profile_long"])) < 200 {
        t.Fatalf("expected richer profile_long field")
    }
    if _, ok := analyst.Fields["anti_patterns"]; !ok {
        t.Fatalf("expected anti_patterns field")
    }
}
```

- [ ] **Step 2: Author each persona pack with the same structure, not ad hoc fields**

Use this minimum `anti_patterns.md` template per persona:

```md
# Anti-patterns

Avoid these failure modes:
- generic “balanced” summary that removes the persona's bias
- unsupported certainty without named anchors
- copying operational language from `operator` or shipping language from `builder`
- replacing domain-specific claims with abstract motivational advice
```

- [ ] **Step 3: Add richer domain materials that still compile into the runtime five-field contract**

```json
{
  "roleplay_prompt": "You are answering as {{.Fields.display_name}}. Use {{.Fields.core_lens}} and the following long-form content:\n\n{{.Fields.profile_long}}\n\nPsychology:\n{{.Fields.psychology_summary}}\n\nAnti-patterns:\n{{.Fields.anti_patterns}}",
  "discussion_question": "{{.Fields.display_name}}, answer the user's question from your own lens: {{.PersonaID}}",
  "supplementary_materials": "Domain anchors:\n{{.Fields.domain_technology}}",
  "output_contract": "Answer in 3 sections: thesis, reasoning, concrete implications.",
  "assumptions_and_constraints": "Do not imitate other personas. Avoid your listed anti-patterns. Name concrete anchors when possible."
}
```

- [ ] **Step 4: Rebuild the bundle and run content evaluation**

Run:
- `make build-content`
- `make content-eval`

Expected:
- compiled runtime bundle exists
- distinctness report exists
- no schema/build failures

- [ ] **Step 5: Keep smoke fixtures intentionally thin but aligned**

The smoke fixture personas should remain small and deterministic. Only update them enough to keep field naming and shape aligned with compiled bundle expectations.

- [ ] **Step 6: Commit**

```bash
git add content/src testdata/smoke/runtime/personas/analyst.json testdata/smoke/runtime/personas/builder.json
git commit -m "feat: add initial rich content cohort"
```

### Task 6: Final verification and workflow documentation

**Files:**
- Modify: `README.md`
- Modify: `AGENTS.md`
- Modify: `docs/operations/content-authoring-guide.md`

- [ ] **Step 1: Add the final human workflow docs**

Document this exact sequence:

```sh
make build-content
make content-eval
./go_bin -persona-set ./content/build/default/runtime/persona-index.json -materials ./content/build/default/runtime/materials/technology.json -question "What should a small team optimize first?"
```

- [ ] **Step 2: Add the final Codex workflow docs**

Document in `AGENTS.md`:
- source content lives under `content/src`
- compiled runtime-ready artifacts live under `content/build`
- runtime code should not learn the rich authoring format directly
- content changes should prefer compiler/eval/docs, not Stage 1/2/3 edits

- [ ] **Step 3: Run the full verification set**

Run:
- `go test ./internal/content ./tools/build_content_bundle ./tools/eval_content_distinctness -v`
- `make build-content`
- `make content-eval`

Expected:
- PASS
- compiled bundle exists under `content/build/default/runtime`
- evaluation report exists under `content/build/default/evals/latest`

- [ ] **Step 4: Commit**

```bash
git add README.md AGENTS.md docs/operations/content-authoring-guide.md Makefile
git commit -m "docs: add content workflow and evaluation guidance"
```
