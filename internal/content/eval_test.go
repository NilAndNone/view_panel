package content

import (
	"context"
	"strings"
	"testing"

	"view_panel/internal/materials"
)

func TestBuildDistinctnessReportFlagsNearDuplicatePersonaOutputs(t *testing.T) {
	bundle := Bundle{
		BundleID:     "default",
		PersonaOrder: []string{"analyst", "builder"},
		Personas: map[string]CompiledPersona{
			"analyst": {
				PersonaID: "analyst",
				Fields: map[string]string{
					"reference_anchors": "postmortems | decision memos",
					"domain_technology": "Anchor on observability, rollback paths, and incident postmortems.",
				},
			},
			"builder": {
				PersonaID: "builder",
				Fields: map[string]string{
					"reference_anchors": "launch reviews | shipping checklists",
					"domain_technology": "Anchor on release checklists, user feedback loops, and launch reviews.",
				},
			},
		},
	}

	report, err := BuildDistinctnessReport(
		bundle,
		"/tmp/bundle",
		"/tmp/questions.json",
		"/tmp/out",
		DistinctnessQuestionSet{
			SchemaVersion: distinctnessQuestionSetSchemaVersion,
			Questions: []DistinctnessQuestion{
				{
					QuestionID:   "technology_fragility",
					MaterialID:   "technology",
					AnchorDomain: "technology",
					Question:     "What should the team do about rising fragility?",
				},
			},
		},
		[]PanelRunResult{
			{
				QuestionID: "technology_fragility",
				MaterialID: "technology",
				RunID:      "run-1",
				Variant:    "raw",
				Cards: []DistinctnessCard{
					{
						PersonaID:     "analyst",
						SourceOutcome: "certified",
						Headline:      "Slow the release train",
						Body:          "Reduce downside by sequencing rollout carefully, adding rollback drills, and requiring explicit evidence before the next launch.",
					},
					{
						PersonaID:     "builder",
						SourceOutcome: "certified",
						Headline:      "Slow the release train",
						Body:          "Reduce downside by sequencing rollout carefully, adding rollback drills, and requiring explicit evidence before the next launch.",
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("BuildDistinctnessReport returned error: %v", err)
	}

	if report.RunCount != 1 {
		t.Fatalf("run_count = %d, want 1", report.RunCount)
	}
	if report.NearDuplicateCount != 1 {
		t.Fatalf("near_duplicate_count = %d, want 1", report.NearDuplicateCount)
	}
	if len(report.Runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(report.Runs))
	}

	run := report.Runs[0]
	if run.AnchorDomain != "technology" {
		t.Fatalf("anchor_domain = %q, want technology", run.AnchorDomain)
	}
	if len(run.NearDuplicatePairs) != 1 {
		t.Fatalf("near_duplicate_pairs = %d, want 1", len(run.NearDuplicatePairs))
	}
	pair := run.NearDuplicatePairs[0]
	if pair.PersonaIDLeft != "analyst" || pair.PersonaIDRight != "builder" {
		t.Fatalf("pair personas = %q/%q, want analyst/builder", pair.PersonaIDLeft, pair.PersonaIDRight)
	}
	if pair.Similarity < 0.99 {
		t.Fatalf("similarity = %.4f, want >= 0.99", pair.Similarity)
	}
	if run.Score >= 100 {
		t.Fatalf("score = %d, want penalty below 100", run.Score)
	}
}

func TestBuildDistinctnessReportFlagsMissingAnchorsAndGenericFraming(t *testing.T) {
	bundle := Bundle{
		BundleID:     "default",
		PersonaOrder: []string{"analyst"},
		Personas: map[string]CompiledPersona{
			"analyst": {
				PersonaID: "analyst",
				Fields: map[string]string{
					"reference_anchors": "postmortems | operational risk reviews",
					"domain_technology": "Anchor on observability, rollback paths, and dependency risk.",
				},
			},
		},
	}

	report, err := BuildDistinctnessReport(
		bundle,
		"/tmp/bundle",
		"/tmp/questions.json",
		"/tmp/out",
		DistinctnessQuestionSet{
			SchemaVersion: distinctnessQuestionSetSchemaVersion,
			Questions: []DistinctnessQuestion{
				{
					QuestionID:   "technology_fragility",
					MaterialID:   "technology",
					AnchorDomain: "technology",
					Question:     "What should the team do about rising fragility?",
				},
			},
		},
		[]PanelRunResult{
			{
				QuestionID: "technology_fragility",
				MaterialID: "technology",
				RunID:      "run-2",
				Variant:    "raw",
				Cards: []DistinctnessCard{
					{
						PersonaID:     "analyst",
						SourceOutcome: "certified",
						Headline:      "Stay balanced",
						Body:          "It depends. We need a balanced perspective because both sides have merit.",
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("BuildDistinctnessReport returned error: %v", err)
	}

	if report.MissingAnchorCount != 1 {
		t.Fatalf("missing_anchor_count = %d, want 1", report.MissingAnchorCount)
	}
	if report.GenericFramingCount != 1 {
		t.Fatalf("generic_framing_count = %d, want 1", report.GenericFramingCount)
	}

	run := report.Runs[0]
	if len(run.PersonaFindings) != 1 {
		t.Fatalf("persona_findings = %d, want 1", len(run.PersonaFindings))
	}
	finding := run.PersonaFindings[0]
	if !finding.MissingAnchors {
		t.Fatalf("missing_anchors = false, want true")
	}
	if !finding.GenericFraming {
		t.Fatalf("generic_framing = false, want true")
	}
	if len(finding.AnchorHits) != 0 {
		t.Fatalf("anchor_hits = %#v, want none", finding.AnchorHits)
	}
	if len(finding.GenericPhrases) == 0 {
		t.Fatalf("expected generic phrase hits")
	}
}

func TestBuildDistinctnessReportRejectsMissingPersonaOutput(t *testing.T) {
	bundle := Bundle{
		BundleID:     "default",
		PersonaOrder: []string{"analyst", "builder"},
		Personas: map[string]CompiledPersona{
			"analyst": {PersonaID: "analyst", Fields: map[string]string{"reference_anchors": "postmortems", "domain_technology": "observability"}},
			"builder": {PersonaID: "builder", Fields: map[string]string{"reference_anchors": "launch reviews", "domain_technology": "shipping"}},
		},
	}

	_, err := BuildDistinctnessReport(
		bundle,
		"/tmp/bundle",
		"/tmp/questions.json",
		"/tmp/out",
		DistinctnessQuestionSet{
			SchemaVersion: distinctnessQuestionSetSchemaVersion,
			Questions: []DistinctnessQuestion{
				{QuestionID: "technology_fragility", MaterialID: "technology", AnchorDomain: "technology", Question: "What should the team do?"},
			},
		},
		[]PanelRunResult{
			{
				QuestionID: "technology_fragility",
				MaterialID: "technology",
				RunID:      "run-3",
				Variant:    "raw",
				Cards: []DistinctnessCard{
					{PersonaID: "analyst", SourceOutcome: "certified", Headline: "A", Body: "Use observability and postmortems."},
				},
			},
		},
	)
	if err == nil {
		t.Fatalf("expected BuildDistinctnessReport to reject missing persona output")
	}
	if !strings.Contains(err.Error(), "missing rendered card for persona") {
		t.Fatalf("expected missing persona error, got %q", err)
	}
}

func TestBuildDistinctnessReportRejectsDuplicatePersonaOutput(t *testing.T) {
	bundle := Bundle{
		BundleID:     "default",
		PersonaOrder: []string{"analyst"},
		Personas: map[string]CompiledPersona{
			"analyst": {PersonaID: "analyst", Fields: map[string]string{"reference_anchors": "postmortems", "domain_technology": "observability"}},
		},
	}

	_, err := BuildDistinctnessReport(
		bundle,
		"/tmp/bundle",
		"/tmp/questions.json",
		"/tmp/out",
		DistinctnessQuestionSet{
			SchemaVersion: distinctnessQuestionSetSchemaVersion,
			Questions: []DistinctnessQuestion{
				{QuestionID: "technology_fragility", MaterialID: "technology", AnchorDomain: "technology", Question: "What should the team do?"},
			},
		},
		[]PanelRunResult{
			{
				QuestionID: "technology_fragility",
				MaterialID: "technology",
				RunID:      "run-4",
				Variant:    "raw",
				Cards: []DistinctnessCard{
					{PersonaID: "analyst", SourceOutcome: "certified", Headline: "A", Body: "Use observability and postmortems."},
					{PersonaID: "analyst", SourceOutcome: "certified", Headline: "B", Body: "Use observability and postmortems."},
				},
			},
		},
	)
	if err == nil {
		t.Fatalf("expected BuildDistinctnessReport to reject duplicate persona output")
	}
	if !strings.Contains(err.Error(), "duplicate rendered card for persona") {
		t.Fatalf("expected duplicate persona error, got %q", err)
	}
}

func TestBuildDistinctnessReportScopesAnchorsToQuestionDomainAndKeepsOrderingDeterministic(t *testing.T) {
	bundle := Bundle{
		BundleID:     "default",
		PersonaOrder: []string{"analyst"},
		Personas: map[string]CompiledPersona{
			"analyst": {
				PersonaID: "analyst",
				Fields: map[string]string{
					"reference_anchors": "zeta memos | alpha reviews",
					"domain_technology": "Observability rollback dependency review",
					"domain_operations": "Escalation staffing queueing cadence",
				},
			},
		},
	}

	report, err := BuildDistinctnessReport(
		bundle,
		"/tmp/bundle",
		"/tmp/questions.json",
		"/tmp/out",
		DistinctnessQuestionSet{
			SchemaVersion: distinctnessQuestionSetSchemaVersion,
			Questions: []DistinctnessQuestion{
				{
					QuestionID:   "operations_pressure",
					MaterialID:   "technology",
					AnchorDomain: "operations",
					Question:     "What should the team do about operational pressure?",
				},
			},
		},
		[]PanelRunResult{
			{
				QuestionID: "operations_pressure",
				MaterialID: "technology",
				RunID:      "run-5",
				Variant:    "raw",
				Cards: []DistinctnessCard{
					{
						PersonaID:     "analyst",
						SourceOutcome: "certified",
						Headline:      "Ops",
						Body:          "Queueing stress and staffing pressure matter more here than observability. " + strings.Repeat("火箭", 120),
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("BuildDistinctnessReport returned error: %v", err)
	}

	finding := report.Runs[0].PersonaFindings[0]
	wantHits := []string{"queueing", "staffing"}
	if strings.Join(finding.AnchorHits, ",") != strings.Join(wantHits, ",") {
		t.Fatalf("anchor_hits = %#v, want %#v", finding.AnchorHits, wantHits)
	}
	if strings.Contains(strings.Join(finding.AnchorHits, ","), "observability") {
		t.Fatalf("anchor_hits should not use technology-scoped anchors: %#v", finding.AnchorHits)
	}
	if strings.Contains(finding.OutputPreview, "\xef\xbf\xbd") {
		t.Fatalf("output_preview contains invalid rune replacement: %q", finding.OutputPreview)
	}
	if !strings.HasSuffix(finding.OutputPreview, "...") {
		t.Fatalf("output_preview = %q, want truncated ellipsis", finding.OutputPreview)
	}
}

func TestBuildDistinctnessReportProvidesDeterministicMissingAnchorExamples(t *testing.T) {
	bundle := Bundle{
		BundleID:     "default",
		PersonaOrder: []string{"analyst"},
		Personas: map[string]CompiledPersona{
			"analyst": {
				PersonaID: "analyst",
				Fields: map[string]string{
					"reference_anchors": "zeta memos | alpha reviews",
					"domain_technology": "rollback observability dependency postmortems",
				},
			},
		},
	}

	report, err := BuildDistinctnessReport(
		bundle,
		"/tmp/bundle",
		"/tmp/questions.json",
		"/tmp/out",
		DistinctnessQuestionSet{
			SchemaVersion: distinctnessQuestionSetSchemaVersion,
			Questions: []DistinctnessQuestion{
				{QuestionID: "technology_fragility", MaterialID: "technology", AnchorDomain: "technology", Question: "What should the team do?"},
			},
		},
		[]PanelRunResult{
			{
				QuestionID: "technology_fragility",
				MaterialID: "technology",
				RunID:      "run-6",
				Variant:    "raw",
				Cards: []DistinctnessCard{
					{PersonaID: "analyst", SourceOutcome: "certified", Headline: "A", Body: "Use judgment."},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("BuildDistinctnessReport returned error: %v", err)
	}

	finding := report.Runs[0].PersonaFindings[0]
	wantExamples := []string{"alpha reviews", "dependency", "observability"}
	if strings.Join(finding.MissingAnchorsFrom, ",") != strings.Join(wantExamples, ",") {
		t.Fatalf("missing_anchors_from = %#v, want %#v", finding.MissingAnchorsFrom, wantExamples)
	}
}

func TestEvaluateDistinctnessRejectsUnsupportedAnchorDomainBeforeExecution(t *testing.T) {
	sourceRoot := testBuilderSourceRoot()
	catalog, err := LoadCatalog(sourceRoot)
	if err != nil {
		t.Fatalf("LoadCatalog returned error: %v", err)
	}
	bundle, err := CompileBundle(catalog, CompileOptions{BundleID: "default"})
	if err != nil {
		t.Fatalf("CompileBundle returned error: %v", err)
	}

	bundleRoot := t.TempDir()
	if err := WriteBundle(bundleRoot, bundle); err != nil {
		t.Fatalf("WriteBundle returned error: %v", err)
	}

	questionsPath := t.TempDir() + "/questions.json"
	writeTextFile(t, questionsPath, "{\n  \"schema_version\": \"distinctness_questions_v1\",\n  \"questions\": [\n    {\n      \"question_id\": \"technology_fragility\",\n      \"material_id\": \"technology\",\n      \"anchor_domain\": \"finance\",\n      \"question\": \"What should the team do?\"\n    }\n  ]\n}\n")

	executor := &countingPanelExecutor{}
	_, err = EvaluateDistinctness(context.Background(), bundleRoot, questionsPath, t.TempDir(), executor, DistinctnessExecOptions{})
	if err == nil {
		t.Fatalf("expected EvaluateDistinctness to reject unsupported anchor_domain")
	}
	if !strings.Contains(err.Error(), "anchor_domain") || !strings.Contains(err.Error(), "domain_finance") {
		t.Fatalf("expected anchor_domain validation error, got %q", err)
	}
	if executor.callCount != 0 {
		t.Fatalf("executor call_count = %d, want 0", executor.callCount)
	}
}

func TestEvaluateDistinctnessRejectsPartiallySupportedAnchorDomainBeforeExecution(t *testing.T) {
	bundleRoot := t.TempDir()
	if err := WriteBundle(bundleRoot, Bundle{
		BundleID:     "default",
		PersonaIndex: RuntimePersonaIndex{Personas: []string{"personas/analyst.json", "personas/builder.json"}},
		PersonaOrder: []string{"analyst", "builder"},
		Personas: map[string]CompiledPersona{
			"analyst": {
				PersonaID: "analyst",
				Fields: map[string]string{
					"display_name":      "Analyst",
					"reference_anchors": "postmortems",
					"domain_technology": "observability rollback",
					"domain_finance":    "capital allocation downside",
				},
			},
			"builder": {
				PersonaID: "builder",
				Fields: map[string]string{
					"display_name":      "Builder",
					"reference_anchors": "launch reviews",
					"domain_technology": "shipping loops",
				},
			},
		},
		MaterialOrder: []string{"technology"},
		Materials: map[string]CompiledMaterial{
			"technology": {
				MaterialID: "technology",
				Fields: materials.CanonicalFields{
					RoleplayPrompt:            "prompt",
					DiscussionQuestion:        "question",
					SupplementaryMaterials:    "materials",
					OutputContract:            "contract",
					AssumptionsAndConstraints: "constraints",
				},
			},
		},
		Manifest: BundleManifest{
			SchemaVersion: contentBundleManifestSchemaVersion,
			BundleID:      "default",
			PersonaCount:  2,
			MaterialCount: 1,
			Artifacts: []string{
				"runtime/persona-index.json",
				"runtime/personas/analyst.json",
				"runtime/personas/builder.json",
				"runtime/materials/technology.json",
			},
		},
	}); err != nil {
		t.Fatalf("WriteBundle returned error: %v", err)
	}

	questionsPath := t.TempDir() + "/questions.json"
	writeTextFile(t, questionsPath, "{\n  \"schema_version\": \"distinctness_questions_v1\",\n  \"questions\": [\n    {\n      \"question_id\": \"technology_fragility\",\n      \"material_id\": \"technology\",\n      \"anchor_domain\": \"finance\",\n      \"question\": \"What should the team do?\"\n    }\n  ]\n}\n")

	executor := &countingPanelExecutor{}
	_, err := EvaluateDistinctness(context.Background(), bundleRoot, questionsPath, t.TempDir(), executor, DistinctnessExecOptions{})
	if err == nil {
		t.Fatalf("expected EvaluateDistinctness to reject partially supported anchor_domain")
	}
	if !strings.Contains(err.Error(), "anchor_domain") || !strings.Contains(err.Error(), "builder") || !strings.Contains(err.Error(), "domain_finance") {
		t.Fatalf("expected partial-support anchor_domain validation error, got %q", err)
	}
	if executor.callCount != 0 {
		t.Fatalf("executor call_count = %d, want 0", executor.callCount)
	}
}

type countingPanelExecutor struct {
	callCount int
}

func (executor *countingPanelExecutor) RunPanel(ctx context.Context, req PanelRunRequest) (PanelRunResult, error) {
	executor.callCount++
	return PanelRunResult{}, nil
}
