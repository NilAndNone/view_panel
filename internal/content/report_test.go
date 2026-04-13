package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteDistinctnessReportWritesJSONAndMarkdown(t *testing.T) {
	outDir := t.TempDir()
	report := DistinctnessReport{
		SchemaVersion:       distinctnessReportSchemaVersion,
		BundleID:            "default",
		BundlePath:          "/tmp/bundle",
		QuestionSetPath:     "/tmp/questions.json",
		OutputRoot:          outDir,
		RunCount:            1,
		PersonaCount:        1,
		OverallScore:        90,
		NearDuplicateCount:  0,
		MissingAnchorCount:  1,
		GenericFramingCount: 0,
		Runs: []DistinctnessRunReport{
			{
				QuestionID:  "technology_fragility",
				MaterialID:  "technology",
				AnchorDomain: "technology",
				Question:    "What should the team do about rising fragility?",
				RunID:       "run-1",
				Variant:     "raw",
				OutputCount: 1,
				Score:       90,
				PersonaFindings: []DistinctnessPersonaFinding{
					{
						PersonaID:       "analyst",
						SourceOutcome:   "certified",
						MissingAnchors:  true,
						OutputPreview:   "Use incident postmortems before increasing release cadence.",
						GenericPhrases:  nil,
						AnchorHits:      nil,
						MissingAnchorsFrom: []string{
							"postmortems",
						},
					},
				},
			},
		},
	}

	if err := WriteDistinctnessReport(outDir, report); err != nil {
		t.Fatalf("WriteDistinctnessReport returned error: %v", err)
	}

	requireFile(t, filepath.Join(outDir, distinctnessReportJSONFileName))
	requireFile(t, filepath.Join(outDir, distinctnessReportMarkdownFileName))

	var decoded DistinctnessReport
	readJSONFile(t, filepath.Join(outDir, distinctnessReportJSONFileName), &decoded)
	if decoded.BundleID != "default" {
		t.Fatalf("bundle_id = %q, want default", decoded.BundleID)
	}
	if decoded.OverallScore != 90 {
		t.Fatalf("overall_score = %d, want 90", decoded.OverallScore)
	}

	markdownBytes, err := os.ReadFile(filepath.Join(outDir, distinctnessReportMarkdownFileName))
	if err != nil {
		t.Fatalf("ReadFile(markdown): %v", err)
	}
	markdown := string(markdownBytes)
	for _, snippet := range []string{
		"# Content Distinctness Report",
		"Overall score: 90",
		"technology_fragility",
		"Anchor domain: technology",
		"analyst",
		"Missing anchors: 1",
	} {
		if !strings.Contains(markdown, snippet) {
			t.Fatalf("markdown missing %q: %q", snippet, markdown)
		}
	}
}
