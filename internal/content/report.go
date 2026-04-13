package content

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	distinctnessReportJSONFileName     = "distinctness-report.json"
	distinctnessReportMarkdownFileName = "distinctness-report.md"
)

func WriteDistinctnessReport(outRoot string, report DistinctnessReport) error {
	reportRoot := filepath.Clean(strings.TrimSpace(outRoot))
	if reportRoot == "" {
		return fmt.Errorf("distinctness report output root must not be empty")
	}
	if err := os.MkdirAll(reportRoot, 0o755); err != nil {
		return fmt.Errorf("create distinctness report output root %q: %w", reportRoot, err)
	}

	if err := writeBundleJSON(filepath.Join(reportRoot, distinctnessReportJSONFileName), report); err != nil {
		return fmt.Errorf("write distinctness report json: %w", err)
	}

	markdownPath := filepath.Join(reportRoot, distinctnessReportMarkdownFileName)
	if err := os.WriteFile(markdownPath, []byte(renderDistinctnessReportMarkdown(report)), 0o644); err != nil {
		return fmt.Errorf("write distinctness report markdown: %w", err)
	}

	return nil
}

func renderDistinctnessReportMarkdown(report DistinctnessReport) string {
	var builder strings.Builder

	builder.WriteString("# Content Distinctness Report\n\n")
	fmt.Fprintf(&builder, "- Bundle ID: %s\n", report.BundleID)
	fmt.Fprintf(&builder, "- Bundle path: %s\n", report.BundlePath)
	fmt.Fprintf(&builder, "- Question set: %s\n", report.QuestionSetPath)
	fmt.Fprintf(&builder, "- Output root: %s\n", report.OutputRoot)
	fmt.Fprintf(&builder, "- Run count: %d\n", report.RunCount)
	fmt.Fprintf(&builder, "- Persona count: %d\n", report.PersonaCount)
	fmt.Fprintf(&builder, "- Overall score: %d\n", report.OverallScore)
	fmt.Fprintf(&builder, "- Near duplicates: %d\n", report.NearDuplicateCount)
	fmt.Fprintf(&builder, "- Missing anchors: %d\n", report.MissingAnchorCount)
	fmt.Fprintf(&builder, "- Generic framing: %d\n", report.GenericFramingCount)

	for _, run := range report.Runs {
		builder.WriteString("\n\n## ")
		builder.WriteString(run.QuestionID)
		builder.WriteString("\n\n")
		fmt.Fprintf(&builder, "- Material ID: %s\n", run.MaterialID)
		fmt.Fprintf(&builder, "- Anchor domain: %s\n", run.AnchorDomain)
		fmt.Fprintf(&builder, "- Run ID: %s\n", run.RunID)
		fmt.Fprintf(&builder, "- Variant: %s\n", run.Variant)
		fmt.Fprintf(&builder, "- Output count: %d\n", run.OutputCount)
		fmt.Fprintf(&builder, "- Score: %d\n", run.Score)
		builder.WriteString("\nQuestion:\n")
		builder.WriteString(run.Question)

		builder.WriteString("\n\n### Near-duplicate pairs\n")
		if len(run.NearDuplicatePairs) == 0 {
			builder.WriteString("None")
		} else {
			for _, pair := range run.NearDuplicatePairs {
				fmt.Fprintf(&builder, "- %s vs %s (similarity %.4f)\n", pair.PersonaIDLeft, pair.PersonaIDRight, pair.Similarity)
			}
		}

		builder.WriteString("\n\n### Persona findings\n")
		if len(run.PersonaFindings) == 0 {
			builder.WriteString("None")
			continue
		}

		for _, finding := range run.PersonaFindings {
			builder.WriteString("\n\n#### ")
			builder.WriteString(finding.PersonaID)
			builder.WriteString("\n")
			fmt.Fprintf(&builder, "- Source outcome: %s\n", finding.SourceOutcome)
			fmt.Fprintf(&builder, "- Missing anchors: %t\n", finding.MissingAnchors)
			fmt.Fprintf(&builder, "- Generic framing: %t\n", finding.GenericFraming)
			builder.WriteString("- Anchor hits: ")
			builder.WriteString(renderListOrNone(finding.AnchorHits))
			builder.WriteString("\n")
			builder.WriteString("- Missing anchor examples: ")
			builder.WriteString(renderListOrNone(finding.MissingAnchorsFrom))
			builder.WriteString("\n")
			builder.WriteString("- Generic phrases: ")
			builder.WriteString(renderListOrNone(finding.GenericPhrases))
			builder.WriteString("\n")
			builder.WriteString("- Output preview: ")
			builder.WriteString(finding.OutputPreview)
		}
	}

	builder.WriteString("\n")
	return builder.String()
}

func renderListOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}
