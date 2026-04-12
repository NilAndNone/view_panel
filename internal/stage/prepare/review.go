package prepare

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"view_panel/internal/storage"
)

const prepareReviewSkillPath = "runtime/skills/wv-prepare-stage/SKILL.md"

// AdvisoryReviewRequest is the runtime-to-skill contract for P06.
type AdvisoryReviewRequest struct {
	SchemaVersion string                  `json:"schema_version"`
	Stage         string                  `json:"stage"`
	ReviewMode    string                  `json:"review_mode"`
	SkillPath     string                  `json:"skill_path"`
	Personas      []AdvisoryReviewPersona `json:"personas"`
}

// AdvisoryReviewPersona carries the immutable prepared bundle for one persona.
type AdvisoryReviewPersona struct {
	PersonaID string               `json:"persona_id"`
	Bundle    AdvisoryReviewBundle `json:"bundle"`
}

// AdvisoryReviewBundle preserves primary review subjects separately from
// evidence-only context.
type AdvisoryReviewBundle struct {
	DispatchInput DispatchInputV1 `json:"dispatch_input_v1"`
	Agents        string          `json:"agents_md"`
	Prompt        string          `json:"prompt_txt"`
	Manifest      PrepareManifest `json:"manifest"`
	Hashes        PrepareHashes   `json:"hashes"`
	Paths         ReviewPaths     `json:"paths"`
}

// ReviewPaths are run-root-relative canonical bundle paths.
type ReviewPaths struct {
	DispatchInput string `json:"dispatch_input"`
	Agents        string `json:"agents"`
	Prompt        string `json:"prompt"`
	Manifest      string `json:"manifest"`
	Hashes        string `json:"hashes"`
}

// PrepareReviewArtifact is the canonical advisory review artifact.
type PrepareReviewArtifact struct {
	SchemaVersion string                 `json:"schema_version"`
	Stage         string                 `json:"stage"`
	ReviewMode    string                 `json:"review_mode"`
	Status        string                 `json:"status"`
	Summary       PrepareReviewSummary   `json:"summary"`
	Findings      []PrepareReviewFinding `json:"findings"`
}

// PrepareReviewSummary is the fixed non-blocking summary object.
type PrepareReviewSummary struct {
	NonBlocking      bool                        `json:"non_blocking"`
	Stage2Dependency string                      `json:"stage2_dependency"`
	ReviewCompleted  bool                        `json:"review_completed"`
	PersonaCount     int                         `json:"persona_count"`
	FindingCount     int                         `json:"finding_count"`
	CategoryCounts   PrepareReviewCategoryCounts `json:"category_counts"`
}

// PrepareReviewCategoryCounts captures deterministic category totals.
type PrepareReviewCategoryCounts struct {
	WeakPersonaDifferentiation int `json:"weak_persona_differentiation"`
	SuspiciousMaterialAssembly int `json:"suspicious_material_assembly"`
	OutputContractIssue        int `json:"output_contract_issue"`
	ReviewExecution            int `json:"review_execution"`
}

// PrepareReviewFinding is a single advisory warning.
type PrepareReviewFinding struct {
	FindingID         string   `json:"finding_id"`
	Severity          string   `json:"severity"`
	Category          string   `json:"category"`
	PersonaID         *string  `json:"persona_id"`
	ArtifactPath      *string  `json:"artifact_path"`
	Message           string   `json:"message"`
	Evidence          []string `json:"evidence"`
	SuggestedFollowUp *string  `json:"suggested_follow_up"`
	BlocksStage2      bool     `json:"blocks_stage2"`
}

func runAdvisoryReview(ctx context.Context, reviewer AdvisoryReviewer, runRoot string) (PrepareReviewArtifact, error) {
	artifact, err := buildOrFallbackPrepareReview(ctx, reviewer, runRoot)
	if err != nil {
		return PrepareReviewArtifact{}, err
	}

	reviewPath, err := storage.ResolvePath(storage.PrepareRoot(runRoot), prepareReviewArtifactName)
	if err != nil {
		return PrepareReviewArtifact{}, fmt.Errorf("resolve %s path: %w", prepareReviewArtifactName, err)
	}
	if err := storage.WriteJSON(reviewPath, artifact); err != nil {
		return PrepareReviewArtifact{}, fmt.Errorf("write %s: %w", prepareReviewArtifactName, err)
	}

	return artifact, nil
}

func buildOrFallbackPrepareReview(ctx context.Context, reviewer AdvisoryReviewer, runRoot string) (PrepareReviewArtifact, error) {
	request, personaCount, err := loadReviewRequest(runRoot)
	if err != nil {
		return synthesizeReviewUnavailable(personaCount, err.Error()), nil
	}

	if reviewer == nil {
		return synthesizeReviewUnavailable(personaCount, "review session runner unavailable"), nil
	}

	response, err := reviewer.ReviewPrepare(ctx, request)
	if err != nil {
		return synthesizeReviewUnavailable(personaCount, fmt.Sprintf("review session failed: %v", err)), nil
	}
	if len(response) == 0 {
		return synthesizeReviewUnavailable(personaCount, "review session returned an empty response"), nil
	}

	artifact, err := validateCompletedReviewResponse(response, personaCount)
	if err != nil {
		return synthesizeReviewUnavailable(personaCount, err.Error()), nil
	}
	return artifact, nil
}

func loadReviewRequest(runRoot string) (AdvisoryReviewRequest, int, error) {
	personasRoot := filepath.Join(storage.PrepareRoot(runRoot), "personas")
	entries, err := os.ReadDir(personasRoot)
	if err != nil {
		return AdvisoryReviewRequest{}, 0, fmt.Errorf("enumerate review personas: %w", err)
	}

	personaIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			personaIDs = append(personaIDs, entry.Name())
		}
	}
	sort.Strings(personaIDs)

	personas := make([]AdvisoryReviewPersona, 0, len(personaIDs))
	for _, personaID := range personaIDs {
		dispatchPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, dispatchInputArtifactName)
		if err != nil {
			return AdvisoryReviewRequest{}, len(personaIDs), fmt.Errorf("resolve %s for %s: %w", dispatchInputArtifactName, personaID, err)
		}
		dispatchBytes, err := os.ReadFile(dispatchPath)
		if err != nil {
			return AdvisoryReviewRequest{}, len(personaIDs), fmt.Errorf("read %s for %s: %w", dispatchInputArtifactName, personaID, err)
		}
		var dispatch DispatchInputV1
		if err := json.Unmarshal(dispatchBytes, &dispatch); err != nil {
			return AdvisoryReviewRequest{}, len(personaIDs), fmt.Errorf("decode %s for %s: %w", dispatchInputArtifactName, personaID, err)
		}

		agentsPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, agentsArtifactName)
		if err != nil {
			return AdvisoryReviewRequest{}, len(personaIDs), fmt.Errorf("resolve %s for %s: %w", agentsArtifactName, personaID, err)
		}
		agentsBytes, err := os.ReadFile(agentsPath)
		if err != nil {
			return AdvisoryReviewRequest{}, len(personaIDs), fmt.Errorf("read %s for %s: %w", agentsArtifactName, personaID, err)
		}

		promptPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, promptArtifactName)
		if err != nil {
			return AdvisoryReviewRequest{}, len(personaIDs), fmt.Errorf("resolve %s for %s: %w", promptArtifactName, personaID, err)
		}
		promptBytes, err := os.ReadFile(promptPath)
		if err != nil {
			return AdvisoryReviewRequest{}, len(personaIDs), fmt.Errorf("read %s for %s: %w", promptArtifactName, personaID, err)
		}

		manifestPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, manifestArtifactName)
		if err != nil {
			return AdvisoryReviewRequest{}, len(personaIDs), fmt.Errorf("resolve %s for %s: %w", manifestArtifactName, personaID, err)
		}
		manifestBytes, err := os.ReadFile(manifestPath)
		if err != nil {
			return AdvisoryReviewRequest{}, len(personaIDs), fmt.Errorf("read %s for %s: %w", manifestArtifactName, personaID, err)
		}
		var manifest PrepareManifest
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			return AdvisoryReviewRequest{}, len(personaIDs), fmt.Errorf("decode %s for %s: %w", manifestArtifactName, personaID, err)
		}

		hashesPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, hashesArtifactName)
		if err != nil {
			return AdvisoryReviewRequest{}, len(personaIDs), fmt.Errorf("resolve %s for %s: %w", hashesArtifactName, personaID, err)
		}
		hashesBytes, err := os.ReadFile(hashesPath)
		if err != nil {
			return AdvisoryReviewRequest{}, len(personaIDs), fmt.Errorf("read %s for %s: %w", hashesArtifactName, personaID, err)
		}
		var hashes PrepareHashes
		if err := json.Unmarshal(hashesBytes, &hashes); err != nil {
			return AdvisoryReviewRequest{}, len(personaIDs), fmt.Errorf("decode %s for %s: %w", hashesArtifactName, personaID, err)
		}

		personas = append(personas, AdvisoryReviewPersona{
			PersonaID: personaID,
			Bundle: AdvisoryReviewBundle{
				DispatchInput: dispatch,
				Agents:        string(agentsBytes),
				Prompt:        string(promptBytes),
				Manifest:      manifest,
				Hashes:        hashes,
				Paths: ReviewPaths{
					DispatchInput: filepath.ToSlash(filepath.Join(prepareStageName, "personas", personaID, dispatchInputArtifactName)),
					Agents:        filepath.ToSlash(filepath.Join(prepareStageName, "personas", personaID, agentsArtifactName)),
					Prompt:        filepath.ToSlash(filepath.Join(prepareStageName, "personas", personaID, promptArtifactName)),
					Manifest:      filepath.ToSlash(filepath.Join(prepareStageName, "personas", personaID, manifestArtifactName)),
					Hashes:        filepath.ToSlash(filepath.Join(prepareStageName, "personas", personaID, hashesArtifactName)),
				},
			},
		})
	}

	return AdvisoryReviewRequest{
		SchemaVersion: prepareReviewSchemaVersion,
		Stage:         prepareStageName,
		ReviewMode:    reviewModeAdvisory,
		SkillPath:     prepareReviewSkillPath,
		Personas:      personas,
	}, len(personas), nil
}

func validateCompletedReviewResponse(data []byte, personaCount int) (PrepareReviewArtifact, error) {
	var topLevel map[string]json.RawMessage
	if err := json.Unmarshal(data, &topLevel); err != nil {
		return PrepareReviewArtifact{}, fmt.Errorf("review response is not valid JSON: %w", err)
	}
	if topLevel == nil {
		return PrepareReviewArtifact{}, fmt.Errorf("review response must be a top-level object")
	}

	requiredTopLevel := []string{"schema_version", "stage", "review_mode", "status", "summary", "findings"}
	if len(topLevel) != len(requiredTopLevel) {
		return PrepareReviewArtifact{}, fmt.Errorf("review response must contain exactly schema_version, stage, review_mode, status, summary, and findings")
	}
	for _, key := range requiredTopLevel {
		if _, ok := topLevel[key]; !ok {
			return PrepareReviewArtifact{}, fmt.Errorf("review response missing required key %q", key)
		}
	}

	var artifact PrepareReviewArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return PrepareReviewArtifact{}, fmt.Errorf("decode review response: %w", err)
	}

	if artifact.SchemaVersion != prepareReviewSchemaVersion || artifact.Stage != prepareStageName || artifact.ReviewMode != reviewModeAdvisory {
		return PrepareReviewArtifact{}, fmt.Errorf("review response literals must match schema_version=%q, stage=%q, and review_mode=%q", prepareReviewSchemaVersion, prepareStageName, reviewModeAdvisory)
	}
	switch artifact.Status {
	case reviewStatusCompletedClean, reviewStatusCompletedWithWarn:
	default:
		if artifact.Status == reviewStatusUnavailable {
			return PrepareReviewArtifact{}, fmt.Errorf("skill returned forbidden status %q; runtime must synthesize the fallback artifact itself", reviewStatusUnavailable)
		}
		return PrepareReviewArtifact{}, fmt.Errorf("review response has unsupported status %q", artifact.Status)
	}

	if !artifact.Summary.NonBlocking || artifact.Summary.Stage2Dependency != "none" {
		return PrepareReviewArtifact{}, fmt.Errorf("review summary must remain non-blocking with stage2_dependency=\"none\"")
	}
	if !artifact.Summary.ReviewCompleted {
		return PrepareReviewArtifact{}, fmt.Errorf("completed review response must set summary.review_completed=true")
	}
	if artifact.Summary.PersonaCount != personaCount {
		return PrepareReviewArtifact{}, fmt.Errorf("review summary persona_count %d does not match prepared persona count %d", artifact.Summary.PersonaCount, personaCount)
	}
	if artifact.Summary.FindingCount != len(artifact.Findings) {
		return PrepareReviewArtifact{}, fmt.Errorf("review summary finding_count %d does not match findings length %d", artifact.Summary.FindingCount, len(artifact.Findings))
	}

	counts := PrepareReviewCategoryCounts{}
	for index, finding := range artifact.Findings {
		if finding.Severity != "warning" {
			return PrepareReviewArtifact{}, fmt.Errorf("finding %d severity must be \"warning\"", index)
		}
		if finding.BlocksStage2 {
			return PrepareReviewArtifact{}, fmt.Errorf("finding %d must keep blocks_stage2=false", index)
		}
		switch finding.Category {
		case "weak_persona_differentiation":
			counts.WeakPersonaDifferentiation++
		case "suspicious_material_assembly":
			counts.SuspiciousMaterialAssembly++
		case "output_contract_issue":
			counts.OutputContractIssue++
		case "review_execution":
			return PrepareReviewArtifact{}, fmt.Errorf("completed review response must not emit review_execution findings")
		default:
			return PrepareReviewArtifact{}, fmt.Errorf("finding %d category %q is unsupported", index, finding.Category)
		}
	}

	if artifact.Summary.CategoryCounts.ReviewExecution != 0 {
		return PrepareReviewArtifact{}, fmt.Errorf("completed review response must keep summary.category_counts.review_execution=0")
	}
	if artifact.Summary.CategoryCounts.WeakPersonaDifferentiation != counts.WeakPersonaDifferentiation ||
		artifact.Summary.CategoryCounts.SuspiciousMaterialAssembly != counts.SuspiciousMaterialAssembly ||
		artifact.Summary.CategoryCounts.OutputContractIssue != counts.OutputContractIssue {
		return PrepareReviewArtifact{}, fmt.Errorf("review response category counts do not match emitted findings")
	}
	if counts.WeakPersonaDifferentiation+
		counts.SuspiciousMaterialAssembly+
		counts.OutputContractIssue+
		artifact.Summary.CategoryCounts.ReviewExecution != artifact.Summary.FindingCount {
		return PrepareReviewArtifact{}, fmt.Errorf("review response category count sum does not match finding_count")
	}

	if len(artifact.Findings) == 0 {
		if artifact.Status != reviewStatusCompletedClean {
			return PrepareReviewArtifact{}, fmt.Errorf("completed review response with zero findings must use status %q", reviewStatusCompletedClean)
		}
	} else if artifact.Status != reviewStatusCompletedWithWarn {
		return PrepareReviewArtifact{}, fmt.Errorf("completed review response with findings must use status %q", reviewStatusCompletedWithWarn)
	}

	return artifact, nil
}

func synthesizeReviewUnavailable(personaCount int, reason string) PrepareReviewArtifact {
	finding := PrepareReviewFinding{
		FindingID:    "prepare-review-runtime-fallback-001",
		Severity:     "warning",
		Category:     "review_execution",
		Message:      reason,
		Evidence:     []string{reason},
		BlocksStage2: false,
	}

	return PrepareReviewArtifact{
		SchemaVersion: prepareReviewSchemaVersion,
		Stage:         prepareStageName,
		ReviewMode:    reviewModeAdvisory,
		Status:        reviewStatusUnavailable,
		Summary: PrepareReviewSummary{
			NonBlocking:      true,
			Stage2Dependency: "none",
			ReviewCompleted:  false,
			PersonaCount:     personaCount,
			FindingCount:     1,
			CategoryCounts: PrepareReviewCategoryCounts{
				WeakPersonaDifferentiation: 0,
				SuspiciousMaterialAssembly: 0,
				OutputContractIssue:        0,
				ReviewExecution:            1,
			},
		},
		Findings: []PrepareReviewFinding{finding},
	}
}
