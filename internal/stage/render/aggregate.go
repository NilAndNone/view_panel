package render

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	panelhash "view_panel/internal/hash"
	panelstorage "view_panel/internal/storage"
)

const (
	answerBatchSchemaVersion = "worldview_answer_batch_v1"
	renderInputSchemaVersion = "worldview_render_input_v1"
	panelResultSchemaVersion = "worldview_panel_result_v1"
	renderStatusSchemaVersion = "worldview_render_status_v1"

	answerStageName = "02_answer"
	renderStageName = "03_render"

	rawVariant       = "raw"
	certifiedVariant = "certified"

	sourceOutcomeCertified = "certified"
	sourceOutcomeRejected  = "rejected"

	omissionReasonResultJSONPathMissing   = "result_json_path_missing"
	omissionReasonResultJSONPathInvalid   = "result_json_path_invalid"
	omissionReasonResultJSONArtifactMiss  = "result_json_artifact_missing"
	omissionReasonResultJSONInvalid       = "result_json_invalid"
	omissionReasonResultJSONSHA256Mismatch = "result_json_sha256_mismatch"

	certifiedGateReasonNoCandidates       = "no_certified_candidates"
	certifiedGateReasonNoDisplayable      = "no_certified_displayable_entries"
	certifiedGateReasonBelowMinSuccess    = "below_min_success_rate"
)

type AggregateRequest struct {
	RunID                   string
	AnswerBatchPath         string
	RenderRoot              string
	RawRenderInputPath      string
	CertifiedRenderInputPath string
	MinSuccessRate          float64
}

type AggregateResult struct {
	RawRenderInputPath       string
	CertifiedRenderInputPath string
	RawAvailable             bool
	CertifiedAvailable       bool
	CertifiedGate            CertifiedGateSummary
}

type CertifiedGateSummary struct {
	T                   int
	C                   int
	MinSuccessRate      float64
	ObservedSuccessRate *float64
	Passed              bool
	GateReason          *string
}

type ForbiddenToolHit struct {
	ToolName           string  `json:"tool_name"`
	ToolNameNormalized string  `json:"tool_name_normalized"`
	CallID             *string `json:"call_id"`
}

type renderInputEntry struct {
	PersonaID              string            `json:"persona_id"`
	SourceOutcome          string            `json:"source_outcome"`
	ResultJSONPath         string            `json:"result_json_path"`
	ResultJSONSHA256       string            `json:"result_json_sha256"`
	AuthoritativeTextSHA256 *string          `json:"authoritative_text_sha256"`
	Result                 json.RawMessage   `json:"result"`
	RejectionReason        *string           `json:"rejection_reason"`
	ForbiddenToolHits      []ForbiddenToolHit `json:"forbidden_tool_hits"`
}

type renderInputOmission struct {
	PersonaID      string  `json:"persona_id"`
	SourceOutcome  string  `json:"source_outcome"`
	ResultJSONPath *string `json:"result_json_path"`
	OmissionReason string  `json:"omission_reason"`
}

type rawRenderInputCounts struct {
	DisplayableTotal     int `json:"displayable_total"`
	CertifiedDisplayable int `json:"certified_displayable"`
	RejectedDisplayable  int `json:"rejected_displayable"`
	OmittedTotal         int `json:"omitted_total"`
}

type rawRenderInputArtifact struct {
	SchemaVersion string                `json:"schema_version"`
	Stage         string                `json:"stage"`
	Variant       string                `json:"variant"`
	RunID         string                `json:"run_id"`
	Available     bool                  `json:"available"`
	Counts        rawRenderInputCounts  `json:"counts"`
	Entries       []renderInputEntry    `json:"entries"`
	Omitted       []renderInputOmission `json:"omitted"`
}

type certifiedGateArtifact struct {
	T                   int      `json:"T"`
	C                   int      `json:"C"`
	ObservedSuccessRate *float64 `json:"observed_success_rate"`
	Passed              bool     `json:"passed"`
	GateReason          *string  `json:"gate_reason"`
}

type certifiedRenderInputArtifact struct {
	SchemaVersion  string                `json:"schema_version"`
	Stage          string                `json:"stage"`
	Variant        string                `json:"variant"`
	RunID          string                `json:"run_id"`
	Available      bool                  `json:"available"`
	MinSuccessRate float64               `json:"min_success_rate"`
	Gate           certifiedGateArtifact `json:"gate"`
	Entries        []renderInputEntry    `json:"entries"`
	Omitted        []renderInputOmission `json:"omitted"`
}

type answerBatchArtifact struct {
	SchemaVersion string                     `json:"schema_version"`
	Stage         string                     `json:"stage"`
	RunID         string                     `json:"run_id"`
	Certified     []answerBatchCertifiedEntry `json:"certified"`
	Rejected      []answerBatchRejectedEntry `json:"rejected"`
}

type answerBatchCertifiedEntry struct {
	PersonaID               string  `json:"persona_id"`
	Outcome                 string  `json:"outcome"`
	ResultJSONPath          *string `json:"result_json_path"`
	ResultJSONSHA256        *string `json:"result_json_sha256"`
	AuthoritativeTextSHA256 *string `json:"authoritative_text_sha256"`
}

type answerBatchRejectedEntry struct {
	PersonaID               string             `json:"persona_id"`
	Outcome                 string             `json:"outcome"`
	ResultJSONPath          *string            `json:"result_json_path"`
	ResultJSONSHA256        *string            `json:"result_json_sha256"`
	AuthoritativeTextSHA256 *string            `json:"authoritative_text_sha256"`
	RejectionReason         *string            `json:"rejection_reason"`
	ForbiddenToolHits       []ForbiddenToolHit `json:"forbidden_tool_hits"`
}

type renderCandidate struct {
	PersonaID               string
	SourceOutcome           string
	ResultJSONPath          *string
	ResultJSONSHA256        *string
	AuthoritativeTextSHA256 *string
	RejectionReason         *string
	ForbiddenToolHits       []ForbiddenToolHit
}

type classifiedCandidate struct {
	entry    *renderInputEntry
	omission *renderInputOmission
}

func AggregateStage3Inputs(ctx context.Context, req AggregateRequest) (AggregateResult, error) {
	if strings.TrimSpace(req.RunID) == "" {
		return AggregateResult{}, fmt.Errorf("run_id is required")
	}
	if strings.TrimSpace(req.AnswerBatchPath) == "" {
		return AggregateResult{}, fmt.Errorf("answer_batch_path is required")
	}
	if req.MinSuccessRate < 0 || req.MinSuccessRate > 1 {
		return AggregateResult{}, fmt.Errorf("min_success_rate must be within [0,1]")
	}
	if err := ctx.Err(); err != nil {
		return AggregateResult{}, err
	}

	rawPath, certifiedPath, err := resolveAggregateOutputPaths(req)
	if err != nil {
		return AggregateResult{}, err
	}

	answerBatchPath := filepath.Clean(req.AnswerBatchPath)
	answerBatchBytes, err := os.ReadFile(answerBatchPath)
	if err != nil {
		return AggregateResult{}, fmt.Errorf("read answer batch: %w", err)
	}

	var answerBatch answerBatchArtifact
	if err := json.Unmarshal(answerBatchBytes, &answerBatch); err != nil {
		return AggregateResult{}, fmt.Errorf("parse answer batch: %w", err)
	}
	if answerBatch.SchemaVersion != answerBatchSchemaVersion {
		return AggregateResult{}, fmt.Errorf("answer batch schema_version must be %q", answerBatchSchemaVersion)
	}
	if answerBatch.Stage != answerStageName {
		return AggregateResult{}, fmt.Errorf("answer batch stage must be %q", answerStageName)
	}
	if answerBatch.RunID != req.RunID {
		return AggregateResult{}, fmt.Errorf("answer batch run_id %q does not match request run_id %q", answerBatch.RunID, req.RunID)
	}

	answerRoot, err := filepath.Abs(filepath.Dir(answerBatchPath))
	if err != nil {
		return AggregateResult{}, fmt.Errorf("resolve answer batch root: %w", err)
	}

	rawEntries := make([]renderInputEntry, 0, len(answerBatch.Certified)+len(answerBatch.Rejected))
	rawOmitted := make([]renderInputOmission, 0, len(answerBatch.Certified)+len(answerBatch.Rejected))
	certifiedEntries := make([]renderInputEntry, 0, len(answerBatch.Certified))
	certifiedOmitted := make([]renderInputOmission, 0, len(answerBatch.Certified))

	for _, batchEntry := range answerBatch.Certified {
		if err := ctx.Err(); err != nil {
			return AggregateResult{}, err
		}
		classified, err := classifyCandidate(answerRoot, newCertifiedCandidate(batchEntry))
		if err != nil {
			return AggregateResult{}, err
		}
		if classified.entry != nil {
			rawEntries = append(rawEntries, *classified.entry)
			certifiedEntries = append(certifiedEntries, *classified.entry)
			continue
		}
		rawOmitted = append(rawOmitted, *classified.omission)
		certifiedOmitted = append(certifiedOmitted, *classified.omission)
	}

	for _, batchEntry := range answerBatch.Rejected {
		if err := ctx.Err(); err != nil {
			return AggregateResult{}, err
		}
		classified, err := classifyCandidate(answerRoot, newRejectedCandidate(batchEntry))
		if err != nil {
			return AggregateResult{}, err
		}
		if classified.entry != nil {
			rawEntries = append(rawEntries, *classified.entry)
			continue
		}
		rawOmitted = append(rawOmitted, *classified.omission)
	}

	rawCounts := rawRenderInputCounts{
		DisplayableTotal:     len(rawEntries),
		CertifiedDisplayable: len(certifiedEntries),
		RejectedDisplayable:  len(rawEntries) - len(certifiedEntries),
		OmittedTotal:         len(rawOmitted),
	}

	gateSummary := buildCertifiedGateSummary(len(answerBatch.Certified), len(certifiedEntries), req.MinSuccessRate)

	rawArtifact := rawRenderInputArtifact{
		SchemaVersion: renderInputSchemaVersion,
		Stage:         renderStageName,
		Variant:       rawVariant,
		RunID:         req.RunID,
		Available:     len(rawEntries) > 0,
		Counts:        rawCounts,
		Entries:       rawEntries,
		Omitted:       rawOmitted,
	}

	certifiedArtifact := certifiedRenderInputArtifact{
		SchemaVersion:  renderInputSchemaVersion,
		Stage:          renderStageName,
		Variant:        certifiedVariant,
		RunID:          req.RunID,
		Available:      gateSummary.Passed,
		MinSuccessRate: req.MinSuccessRate,
		Gate: certifiedGateArtifact{
			T:                   gateSummary.T,
			C:                   gateSummary.C,
			ObservedSuccessRate: gateSummary.ObservedSuccessRate,
			Passed:              gateSummary.Passed,
			GateReason:          gateSummary.GateReason,
		},
		Entries: certifiedEntries,
		Omitted: certifiedOmitted,
	}

	if err := writeJSONFile(rawPath, rawArtifact); err != nil {
		return AggregateResult{}, fmt.Errorf("write raw render input: %w", err)
	}
	if err := writeJSONFile(certifiedPath, certifiedArtifact); err != nil {
		return AggregateResult{}, fmt.Errorf("write certified render input: %w", err)
	}

	return AggregateResult{
		RawRenderInputPath:       rawPath,
		CertifiedRenderInputPath: certifiedPath,
		RawAvailable:             rawArtifact.Available,
		CertifiedAvailable:       certifiedArtifact.Available,
		CertifiedGate:            gateSummary,
	}, nil
}

func resolveAggregateOutputPaths(req AggregateRequest) (string, string, error) {
	renderRoot := strings.TrimSpace(req.RenderRoot)
	if renderRoot == "" {
		switch {
		case strings.TrimSpace(req.RawRenderInputPath) != "":
			renderRoot = filepath.Dir(req.RawRenderInputPath)
		case strings.TrimSpace(req.CertifiedRenderInputPath) != "":
			renderRoot = filepath.Dir(req.CertifiedRenderInputPath)
		default:
			return "", "", fmt.Errorf("render_root or explicit render input paths are required")
		}
	}

	rawPath := strings.TrimSpace(req.RawRenderInputPath)
	if rawPath == "" {
		var err error
		rawPath, err = panelstorage.ResolvePath(renderRoot, "raw_render_input.json")
		if err != nil {
			return "", "", fmt.Errorf("resolve raw render input path: %w", err)
		}
	} else {
		rawPath = filepath.Clean(rawPath)
	}

	certifiedPath := strings.TrimSpace(req.CertifiedRenderInputPath)
	if certifiedPath == "" {
		var err error
		certifiedPath, err = panelstorage.ResolvePath(renderRoot, "certified_render_input.json")
		if err != nil {
			return "", "", fmt.Errorf("resolve certified render input path: %w", err)
		}
	} else {
		certifiedPath = filepath.Clean(certifiedPath)
	}

	return rawPath, certifiedPath, nil
}

func newCertifiedCandidate(entry answerBatchCertifiedEntry) renderCandidate {
	return renderCandidate{
		PersonaID:               entry.PersonaID,
		SourceOutcome:           sourceOutcomeCertified,
		ResultJSONPath:          entry.ResultJSONPath,
		ResultJSONSHA256:        entry.ResultJSONSHA256,
		AuthoritativeTextSHA256: entry.AuthoritativeTextSHA256,
		RejectionReason:         nil,
		ForbiddenToolHits:       []ForbiddenToolHit{},
	}
}

func newRejectedCandidate(entry answerBatchRejectedEntry) renderCandidate {
	return renderCandidate{
		PersonaID:               entry.PersonaID,
		SourceOutcome:           sourceOutcomeRejected,
		ResultJSONPath:          entry.ResultJSONPath,
		ResultJSONSHA256:        entry.ResultJSONSHA256,
		AuthoritativeTextSHA256: entry.AuthoritativeTextSHA256,
		RejectionReason:         cloneString(entry.RejectionReason),
		ForbiddenToolHits:       cloneForbiddenToolHits(entry.ForbiddenToolHits),
	}
}

func classifyCandidate(answerRoot string, candidate renderCandidate) (classifiedCandidate, error) {
	resultJSONPath := normalizedOptionalString(candidate.ResultJSONPath)
	if resultJSONPath == nil {
		return classifiedCandidate{
			omission: buildOmission(candidate, nil, omissionReasonResultJSONPathMissing),
		}, nil
	}

	resolvedPath, err := resolveResultJSONPath(answerRoot, *resultJSONPath)
	if err != nil {
		return classifiedCandidate{
			omission: buildOmission(candidate, resultJSONPath, omissionReasonResultJSONPathInvalid),
		}, nil
	}

	resultJSONBytes, err := os.ReadFile(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return classifiedCandidate{
				omission: buildOmission(candidate, resultJSONPath, omissionReasonResultJSONArtifactMiss),
			}, nil
		}
		return classifiedCandidate{}, fmt.Errorf("read %s result.json for persona %q: %w", candidate.SourceOutcome, candidate.PersonaID, err)
	}

	if !isJSONObjectJSON(resultJSONBytes) {
		return classifiedCandidate{
			omission: buildOmission(candidate, resultJSONPath, omissionReasonResultJSONInvalid),
		}, nil
	}

	expectedHash := normalizedOptionalString(candidate.ResultJSONSHA256)
	actualHash, err := panelhash.SHA256HexFile(resolvedPath)
	if err != nil {
		return classifiedCandidate{}, fmt.Errorf("hash %s result.json for persona %q: %w", candidate.SourceOutcome, candidate.PersonaID, err)
	}
	if expectedHash == nil || actualHash != strings.ToLower(*expectedHash) {
		return classifiedCandidate{
			omission: buildOmission(candidate, resultJSONPath, omissionReasonResultJSONSHA256Mismatch),
		}, nil
	}

	return classifiedCandidate{
		entry: &renderInputEntry{
			PersonaID:               candidate.PersonaID,
			SourceOutcome:           candidate.SourceOutcome,
			ResultJSONPath:          *resultJSONPath,
			ResultJSONSHA256:        actualHash,
			AuthoritativeTextSHA256: cloneString(candidate.AuthoritativeTextSHA256),
			Result:                  json.RawMessage(resultJSONBytes),
			RejectionReason:         cloneString(candidate.RejectionReason),
			ForbiddenToolHits:       cloneForbiddenToolHits(candidate.ForbiddenToolHits),
		},
	}, nil
}

func resolveResultJSONPath(answerRoot string, relativePath string) (string, error) {
	if strings.TrimSpace(relativePath) == "" {
		return "", fmt.Errorf("result_json_path is empty")
	}

	if filepath.Base(filepath.Clean(filepath.FromSlash(relativePath))) != "result.json" {
		return "", fmt.Errorf("only result.json artifacts may be opened")
	}

	resolvedPath, err := panelstorage.ResolvePath(answerRoot, relativePath)
	if err != nil {
		return "", fmt.Errorf("resolve candidate path: %w", err)
	}
	if filepath.Base(resolvedPath) != "result.json" {
		return "", fmt.Errorf("only result.json artifacts may be opened")
	}

	return resolvedPath, nil
}

func buildOmission(candidate renderCandidate, resultJSONPath *string, omissionReason string) *renderInputOmission {
	return &renderInputOmission{
		PersonaID:      candidate.PersonaID,
		SourceOutcome:  candidate.SourceOutcome,
		ResultJSONPath: cloneString(resultJSONPath),
		OmissionReason: omissionReason,
	}
}

func buildCertifiedGateSummary(totalCandidates int, displayableCandidates int, minSuccessRate float64) CertifiedGateSummary {
	summary := CertifiedGateSummary{
		T:              totalCandidates,
		C:              displayableCandidates,
		MinSuccessRate: minSuccessRate,
	}

	switch {
	case totalCandidates == 0:
		summary.Passed = false
		summary.GateReason = stringPtr(certifiedGateReasonNoCandidates)
	case displayableCandidates == 0:
		observed := float64(0)
		summary.ObservedSuccessRate = &observed
		summary.Passed = false
		summary.GateReason = stringPtr(certifiedGateReasonNoDisplayable)
	default:
		observed := float64(displayableCandidates) / float64(totalCandidates)
		summary.ObservedSuccessRate = &observed
		if observed < minSuccessRate {
			summary.Passed = false
			summary.GateReason = stringPtr(certifiedGateReasonBelowMinSuccess)
		} else {
			summary.Passed = true
		}
	}

	return summary
}

func writeJSONFile(filePath string, value any) error {
	return panelstorage.WriteJSON(filePath, value)
}

func isJSONObjectJSON(data []byte) bool {
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return false
	}
	_, ok := decoded.(map[string]any)
	return ok
}

func cloneForbiddenToolHits(hits []ForbiddenToolHit) []ForbiddenToolHit {
	if len(hits) == 0 {
		return []ForbiddenToolHit{}
	}
	cloned := make([]ForbiddenToolHit, len(hits))
	copy(cloned, hits)
	return cloned
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func normalizedOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func stringPtr(value string) *string {
	return &value
}
