package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	panelstorage "view_panel/internal/storage"
)

const (
	renderBranchStateRendered = "rendered"
	renderBranchStateSkipped  = "skipped"
	renderBranchStateFailed   = "failed"

	renderSkipReasonInputUnavailable    = "input_unavailable"
	renderSkipReasonUpstreamGateSkipped = "upstream_gate_not_passed"

	renderErrorPhaseInputLoad      = "input_load"
	renderErrorPhaseRenderTurn     = "render_turn"
	renderErrorPhaseSchemaValidate = "schema_validate"
	renderErrorPhaseWriteOutput    = "write_output"

	renderOverallCompleted = "completed"
	renderOverallFailed    = "failed"
)

type RunRenderStageRequest struct {
	RunID                   string
	RenderRoot              string
	RawRenderInputPath      string
	CertifiedRenderInputPath string
	SkillPath               string
	AppServer               AppServerRenderRunner
	LaunchOptions           map[string]any
	RuntimeOptions          map[string]any
	BranchTimeout           time.Duration
}

type AppServerRenderRunner interface {
	ExecuteRenderTurn(ctx context.Context, req AppServerRenderRequest) (AppServerRenderResponse, error)
}

type AppServerRenderRequest struct {
	RunID          string
	Variant        string
	WorkingDirectory string
	SkillPath      string
	RenderInputPath string
	LaunchOptions  map[string]any
	RuntimeOptions map[string]any
}

type AppServerRenderResponse struct {
	PanelJSON json.RawMessage
}

type RunRenderStageResult struct {
	StatusPath string
	Raw        renderBranchStatus
	Certified  renderBranchStatus
	Overall    renderOverallStatus
}

type renderStatusArtifact struct {
	SchemaVersion string             `json:"schema_version"`
	Stage         string             `json:"stage"`
	RunID         string             `json:"run_id"`
	Raw           renderBranchStatus `json:"raw"`
	Certified     renderBranchStatus `json:"certified"`
	Overall       renderOverallStatus `json:"overall"`
}

type renderBranchStatus struct {
	State         string             `json:"state"`
	InputPath     string             `json:"input_path"`
	InputAvailable *bool             `json:"input_available"`
	SkipReason    *string            `json:"skip_reason"`
	PanelJSONPath *string            `json:"panel_json_path"`
	PanelMDPath   *string            `json:"panel_md_path"`
	CardsJSONPath *string            `json:"cards_json_path"`
	Error         *renderBranchError `json:"error"`
}

type renderBranchError struct {
	Phase   string `json:"phase"`
	Message string `json:"message"`
}

type renderOverallStatus struct {
	StageOutcome    string   `json:"stage_outcome"`
	RenderedVariants []string `json:"rendered_variants"`
}

type renderBranchSpec struct {
	Variant            string
	InputPath          string
	StatusInputPath    string
	PanelJSONPath      string
	StatusPanelJSONPath string
	PanelMDPath        string
	StatusPanelMDPath  string
	CardsJSONPath      string
	StatusCardsJSONPath string
	SkipReason         string
}

type validatedPanelResult struct {
	SchemaVersion string       `json:"schema_version"`
	Stage         string       `json:"stage"`
	Variant       string       `json:"variant"`
	RunID         string       `json:"run_id"`
	Title         string       `json:"title"`
	Summary       string       `json:"summary"`
	Sections      []panelSection `json:"sections"`
	Cards         []panelCard  `json:"cards"`
}

type panelSection struct {
	Heading string `json:"heading"`
	Body    string `json:"body"`
}

type panelCard struct {
	PersonaID     string `json:"persona_id"`
	SourceOutcome string `json:"source_outcome"`
	Headline      string `json:"headline"`
	Body          string `json:"body"`
}

func RunRenderStage(ctx context.Context, req RunRenderStageRequest) (RunRenderStageResult, error) {
	if strings.TrimSpace(req.RunID) == "" {
		return RunRenderStageResult{}, fmt.Errorf("run_id is required")
	}
	if err := ctx.Err(); err != nil {
		return RunRenderStageResult{}, err
	}

	resolved, err := resolveRenderStagePaths(req)
	if err != nil {
		return RunRenderStageResult{}, err
	}

	if strings.TrimSpace(req.SkillPath) == "" {
		req.SkillPath = filepath.Join("runtime", "skills", "wv-render-stage", "SKILL.md")
	}

	rawSpec := renderBranchSpec{
		Variant:             rawVariant,
		InputPath:           resolved.rawInputPath,
		StatusInputPath:     path.Join("runs", req.RunID, renderStageName, "raw_render_input.json"),
		PanelJSONPath:       resolved.rawPanelJSONPath,
		StatusPanelJSONPath: path.Join("runs", req.RunID, renderStageName, "raw_panel.json"),
		PanelMDPath:         resolved.rawPanelMDPath,
		StatusPanelMDPath:   path.Join("runs", req.RunID, renderStageName, "raw_panel.md"),
		CardsJSONPath:       resolved.rawCardsJSONPath,
		StatusCardsJSONPath: path.Join("runs", req.RunID, renderStageName, "raw_cards.json"),
		SkipReason:          renderSkipReasonInputUnavailable,
	}

	certifiedSpec := renderBranchSpec{
		Variant:             certifiedVariant,
		InputPath:           resolved.certifiedInputPath,
		StatusInputPath:     path.Join("runs", req.RunID, renderStageName, "certified_render_input.json"),
		PanelJSONPath:       resolved.certifiedPanelJSONPath,
		StatusPanelJSONPath: path.Join("runs", req.RunID, renderStageName, "certified_panel.json"),
		PanelMDPath:         resolved.certifiedPanelMDPath,
		StatusPanelMDPath:   path.Join("runs", req.RunID, renderStageName, "certified_panel.md"),
		CardsJSONPath:       resolved.certifiedCardsJSONPath,
		StatusCardsJSONPath: path.Join("runs", req.RunID, renderStageName, "certified_cards.json"),
		SkipReason:          renderSkipReasonUpstreamGateSkipped,
	}

	rawStatus := executeRenderBranch(ctx, req, resolved.renderRoot, rawSpec)
	certifiedStatus := executeRenderBranch(ctx, req, resolved.renderRoot, certifiedSpec)

	renderedVariants := make([]string, 0, 2)
	if rawStatus.State == renderBranchStateRendered {
		renderedVariants = append(renderedVariants, rawVariant)
	}
	if certifiedStatus.State == renderBranchStateRendered {
		renderedVariants = append(renderedVariants, certifiedVariant)
	}

	overall := renderOverallStatus{
		StageOutcome:    renderOverallCompleted,
		RenderedVariants: renderedVariants,
	}
	if rawStatus.State == renderBranchStateFailed || certifiedStatus.State == renderBranchStateFailed {
		overall.StageOutcome = renderOverallFailed
	}

	statusArtifact := renderStatusArtifact{
		SchemaVersion: renderStatusSchemaVersion,
		Stage:         renderStageName,
		RunID:         req.RunID,
		Raw:           rawStatus,
		Certified:     certifiedStatus,
		Overall:       overall,
	}

	if err := writeJSONFile(resolved.statusPath, statusArtifact); err != nil {
		return RunRenderStageResult{}, fmt.Errorf("write render status: %w", err)
	}

	return RunRenderStageResult{
		StatusPath: resolved.statusPath,
		Raw:        rawStatus,
		Certified:  certifiedStatus,
		Overall:    overall,
	}, nil
}

type resolvedRenderStagePaths struct {
	renderRoot             string
	rawInputPath           string
	certifiedInputPath     string
	rawPanelJSONPath       string
	rawPanelMDPath         string
	rawCardsJSONPath       string
	certifiedPanelJSONPath string
	certifiedPanelMDPath   string
	certifiedCardsJSONPath string
	statusPath             string
}

func resolveRenderStagePaths(req RunRenderStageRequest) (resolvedRenderStagePaths, error) {
	renderRoot := strings.TrimSpace(req.RenderRoot)
	if renderRoot == "" {
		switch {
		case strings.TrimSpace(req.RawRenderInputPath) != "":
			renderRoot = filepath.Dir(req.RawRenderInputPath)
		case strings.TrimSpace(req.CertifiedRenderInputPath) != "":
			renderRoot = filepath.Dir(req.CertifiedRenderInputPath)
		default:
			return resolvedRenderStagePaths{}, fmt.Errorf("render_root or render input paths are required")
		}
	}

	rawInputPath := strings.TrimSpace(req.RawRenderInputPath)
	if rawInputPath == "" {
		var err error
		rawInputPath, err = panelstorage.ResolvePath(renderRoot, "raw_render_input.json")
		if err != nil {
			return resolvedRenderStagePaths{}, fmt.Errorf("resolve raw render input path: %w", err)
		}
	} else {
		rawInputPath = filepath.Clean(rawInputPath)
	}

	certifiedInputPath := strings.TrimSpace(req.CertifiedRenderInputPath)
	if certifiedInputPath == "" {
		var err error
		certifiedInputPath, err = panelstorage.ResolvePath(renderRoot, "certified_render_input.json")
		if err != nil {
			return resolvedRenderStagePaths{}, fmt.Errorf("resolve certified render input path: %w", err)
		}
	} else {
		certifiedInputPath = filepath.Clean(certifiedInputPath)
	}

	rawPanelJSONPath, err := panelstorage.ResolvePath(renderRoot, "raw_panel.json")
	if err != nil {
		return resolvedRenderStagePaths{}, fmt.Errorf("resolve raw panel json path: %w", err)
	}
	rawPanelMDPath, err := panelstorage.ResolvePath(renderRoot, "raw_panel.md")
	if err != nil {
		return resolvedRenderStagePaths{}, fmt.Errorf("resolve raw panel markdown path: %w", err)
	}
	rawCardsJSONPath, err := panelstorage.ResolvePath(renderRoot, "raw_cards.json")
	if err != nil {
		return resolvedRenderStagePaths{}, fmt.Errorf("resolve raw cards json path: %w", err)
	}
	certifiedPanelJSONPath, err := panelstorage.ResolvePath(renderRoot, "certified_panel.json")
	if err != nil {
		return resolvedRenderStagePaths{}, fmt.Errorf("resolve certified panel json path: %w", err)
	}
	certifiedPanelMDPath, err := panelstorage.ResolvePath(renderRoot, "certified_panel.md")
	if err != nil {
		return resolvedRenderStagePaths{}, fmt.Errorf("resolve certified panel markdown path: %w", err)
	}
	certifiedCardsJSONPath, err := panelstorage.ResolvePath(renderRoot, "certified_cards.json")
	if err != nil {
		return resolvedRenderStagePaths{}, fmt.Errorf("resolve certified cards json path: %w", err)
	}
	statusPath, err := panelstorage.ResolvePath(renderRoot, "status.json")
	if err != nil {
		return resolvedRenderStagePaths{}, fmt.Errorf("resolve render status path: %w", err)
	}

	return resolvedRenderStagePaths{
		renderRoot:             filepath.Clean(renderRoot),
		rawInputPath:           rawInputPath,
		certifiedInputPath:     certifiedInputPath,
		rawPanelJSONPath:       rawPanelJSONPath,
		rawPanelMDPath:         rawPanelMDPath,
		rawCardsJSONPath:       rawCardsJSONPath,
		certifiedPanelJSONPath: certifiedPanelJSONPath,
		certifiedPanelMDPath:   certifiedPanelMDPath,
		certifiedCardsJSONPath: certifiedCardsJSONPath,
		statusPath:             statusPath,
	}, nil
}

func executeRenderBranch(ctx context.Context, req RunRenderStageRequest, renderRoot string, spec renderBranchSpec) renderBranchStatus {
	inputAvailable, loadErr := loadRenderInputAvailability(spec.InputPath, req.RunID, spec.Variant)
	if loadErr != nil {
		return failedRenderBranchStatus(spec, nil, renderErrorPhaseInputLoad, loadErr)
	}
	if !inputAvailable {
		return skippedRenderBranchStatus(spec, inputAvailable)
	}
	if req.AppServer == nil {
		return failedRenderBranchStatus(spec, boolPtr(inputAvailable), renderErrorPhaseRenderTurn, fmt.Errorf("appserver render runner is nil"))
	}

	branchCtx := ctx
	cancel := func() {}
	if req.BranchTimeout > 0 {
		branchCtx, cancel = context.WithTimeout(ctx, req.BranchTimeout)
	}
	defer cancel()

	response, err := req.AppServer.ExecuteRenderTurn(branchCtx, AppServerRenderRequest{
		RunID:           req.RunID,
		Variant:         spec.Variant,
		WorkingDirectory: renderRoot,
		SkillPath:       req.SkillPath,
		RenderInputPath: spec.InputPath,
		LaunchOptions:   req.LaunchOptions,
		RuntimeOptions:  req.RuntimeOptions,
	})
	if err != nil {
		return failedRenderBranchStatus(spec, boolPtr(inputAvailable), renderErrorPhaseRenderTurn, err)
	}

	panel, err := validatePanelResult(response.PanelJSON, spec.Variant, req.RunID)
	if err != nil {
		return failedRenderBranchStatus(spec, boolPtr(inputAvailable), renderErrorPhaseSchemaValidate, err)
	}

	if err := writeBranchOutputs(spec, response.PanelJSON, panel); err != nil {
		return failedRenderBranchStatus(spec, boolPtr(inputAvailable), renderErrorPhaseWriteOutput, err)
	}

	return renderedRenderBranchStatus(spec, inputAvailable)
}

func loadRenderInputAvailability(filePath string, expectedRunID string, expectedVariant string) (bool, error) {
	inputBytes, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(inputBytes, &envelope); err != nil {
		return false, err
	}

	requiredKeys := []string{"schema_version", "stage", "variant", "run_id", "available", "entries", "omitted"}
	switch expectedVariant {
	case rawVariant:
		requiredKeys = append(requiredKeys, "counts")
	case certifiedVariant:
		requiredKeys = append(requiredKeys, "min_success_rate", "gate")
	default:
		return false, fmt.Errorf("unsupported render variant %q", expectedVariant)
	}

	if err := requireExactJSONKeys(envelope, requiredKeys); err != nil {
		return false, err
	}

	var header struct {
		SchemaVersion string `json:"schema_version"`
		Stage         string `json:"stage"`
		Variant       string `json:"variant"`
		RunID         string `json:"run_id"`
		Available     bool   `json:"available"`
	}
	if err := json.Unmarshal(inputBytes, &header); err != nil {
		return false, err
	}
	if header.SchemaVersion != renderInputSchemaVersion {
		return false, fmt.Errorf("render input schema_version must be %q", renderInputSchemaVersion)
	}
	if header.Stage != renderStageName {
		return false, fmt.Errorf("render input stage must be %q", renderStageName)
	}
	if header.Variant != expectedVariant {
		return false, fmt.Errorf("render input variant must be %q", expectedVariant)
	}
	if header.RunID != expectedRunID {
		return false, fmt.Errorf("render input run_id %q does not match request run_id %q", header.RunID, expectedRunID)
	}

	return header.Available, nil
}

func renderedRenderBranchStatus(spec renderBranchSpec, inputAvailable bool) renderBranchStatus {
	return renderBranchStatus{
		State:          renderBranchStateRendered,
		InputPath:      spec.StatusInputPath,
		InputAvailable: boolPtr(inputAvailable),
		SkipReason:     nil,
		PanelJSONPath:  stringPtr(spec.StatusPanelJSONPath),
		PanelMDPath:    stringPtr(spec.StatusPanelMDPath),
		CardsJSONPath:  stringPtr(spec.StatusCardsJSONPath),
		Error:          nil,
	}
}

func skippedRenderBranchStatus(spec renderBranchSpec, inputAvailable bool) renderBranchStatus {
	return renderBranchStatus{
		State:          renderBranchStateSkipped,
		InputPath:      spec.StatusInputPath,
		InputAvailable: boolPtr(inputAvailable),
		SkipReason:     stringPtr(spec.SkipReason),
		PanelJSONPath:  nil,
		PanelMDPath:    nil,
		CardsJSONPath:  nil,
		Error:          nil,
	}
}

func failedRenderBranchStatus(spec renderBranchSpec, inputAvailable *bool, phase string, err error) renderBranchStatus {
	return renderBranchStatus{
		State:          renderBranchStateFailed,
		InputPath:      spec.StatusInputPath,
		InputAvailable: inputAvailable,
		SkipReason:     nil,
		PanelJSONPath:  nil,
		PanelMDPath:    nil,
		CardsJSONPath:  nil,
		Error: &renderBranchError{
			Phase:   phase,
			Message: err.Error(),
		},
	}
}

func validatePanelResult(panelJSON []byte, expectedVariant string, expectedRunID string) (validatedPanelResult, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(panelJSON, &envelope); err != nil {
		return validatedPanelResult{}, fmt.Errorf("panel JSON is not valid JSON: %w", err)
	}
	if err := requireExactJSONKeys(envelope, []string{
		"schema_version",
		"stage",
		"variant",
		"run_id",
		"title",
		"summary",
		"sections",
		"cards",
	}); err != nil {
		return validatedPanelResult{}, err
	}

	var panel validatedPanelResult
	if err := json.Unmarshal(panelJSON, &panel); err != nil {
		return validatedPanelResult{}, fmt.Errorf("panel JSON does not match panel_result_v1: %w", err)
	}
	if panel.SchemaVersion != panelResultSchemaVersion {
		return validatedPanelResult{}, fmt.Errorf("panel schema_version must be %q", panelResultSchemaVersion)
	}
	if panel.Stage != renderStageName {
		return validatedPanelResult{}, fmt.Errorf("panel stage must be %q", renderStageName)
	}
	if panel.Variant != expectedVariant {
		return validatedPanelResult{}, fmt.Errorf("panel variant must be %q", expectedVariant)
	}
	if panel.RunID != expectedRunID {
		return validatedPanelResult{}, fmt.Errorf("panel run_id %q does not match request run_id %q", panel.RunID, expectedRunID)
	}

	sections, err := validatePanelSections(envelope["sections"])
	if err != nil {
		return validatedPanelResult{}, err
	}
	cards, err := validatePanelCards(envelope["cards"], expectedVariant)
	if err != nil {
		return validatedPanelResult{}, err
	}

	panel.Sections = sections
	panel.Cards = cards
	return panel, nil
}

func validatePanelSections(raw json.RawMessage) ([]panelSection, error) {
	if !isJSONArray(raw) {
		return nil, fmt.Errorf("sections must be an array")
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("sections must be an array of objects: %w", err)
	}

	sections := make([]panelSection, 0, len(items))
	for index, item := range items {
		if !isJSONObject(item) {
			return nil, fmt.Errorf("sections[%d] must be an object", index)
		}

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(item, &fields); err != nil {
			return nil, fmt.Errorf("parse sections[%d]: %w", index, err)
		}
		if err := requireExactJSONKeys(fields, []string{"heading", "body"}); err != nil {
			return nil, fmt.Errorf("sections[%d]: %w", index, err)
		}

		var section panelSection
		if err := json.Unmarshal(item, &section); err != nil {
			return nil, fmt.Errorf("decode sections[%d]: %w", index, err)
		}
		sections = append(sections, section)
	}

	return sections, nil
}

func validatePanelCards(raw json.RawMessage, expectedVariant string) ([]panelCard, error) {
	if !isJSONArray(raw) {
		return nil, fmt.Errorf("cards must be an array")
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("cards must be an array of objects: %w", err)
	}

	cards := make([]panelCard, 0, len(items))
	for index, item := range items {
		if !isJSONObject(item) {
			return nil, fmt.Errorf("cards[%d] must be an object", index)
		}

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(item, &fields); err != nil {
			return nil, fmt.Errorf("parse cards[%d]: %w", index, err)
		}
		if err := requireExactJSONKeys(fields, []string{"persona_id", "source_outcome", "headline", "body"}); err != nil {
			return nil, fmt.Errorf("cards[%d]: %w", index, err)
		}

		var card panelCard
		if err := json.Unmarshal(item, &card); err != nil {
			return nil, fmt.Errorf("decode cards[%d]: %w", index, err)
		}

		switch expectedVariant {
		case rawVariant:
			if card.SourceOutcome != sourceOutcomeCertified && card.SourceOutcome != sourceOutcomeRejected {
				return nil, fmt.Errorf("cards[%d].source_outcome must be %q or %q for raw panels", index, sourceOutcomeCertified, sourceOutcomeRejected)
			}
		case certifiedVariant:
			if card.SourceOutcome != sourceOutcomeCertified {
				return nil, fmt.Errorf("cards[%d].source_outcome must be %q for certified panels", index, sourceOutcomeCertified)
			}
		default:
			return nil, fmt.Errorf("unsupported variant %q", expectedVariant)
		}

		cards = append(cards, card)
	}

	return cards, nil
}

func writeBranchOutputs(spec renderBranchSpec, panelJSON []byte, panel validatedPanelResult) error {
	createdPaths := make([]string, 0, 3)

	if err := panelstorage.WriteText(spec.PanelJSONPath, string(panelJSON)); err != nil {
		return err
	}
	createdPaths = append(createdPaths, spec.PanelJSONPath)

	if err := panelstorage.WriteText(spec.PanelMDPath, renderPanelMarkdown(panel)); err != nil {
		for _, createdPath := range createdPaths {
			_ = os.Remove(createdPath)
		}
		return err
	}
	createdPaths = append(createdPaths, spec.PanelMDPath)

	if err := panelstorage.WriteJSON(spec.CardsJSONPath, panel.Cards); err != nil {
		for _, createdPath := range createdPaths {
			_ = os.Remove(createdPath)
		}
		return err
	}

	return nil
}

func renderPanelMarkdown(panel validatedPanelResult) string {
	var builder strings.Builder

	builder.WriteString("# ")
	builder.WriteString(panel.Title)
	builder.WriteString("\n\n")
	builder.WriteString(panel.Summary)

	for _, section := range panel.Sections {
		builder.WriteString("\n\n## ")
		builder.WriteString(section.Heading)
		builder.WriteString("\n\n")
		builder.WriteString(section.Body)
	}

	builder.WriteString("\n\n## Cards")
	for _, card := range panel.Cards {
		builder.WriteString("\n\n### ")
		builder.WriteString(card.Headline)
		builder.WriteString("\n- persona_id: ")
		builder.WriteString(card.PersonaID)
		builder.WriteString("\n- source_outcome: ")
		builder.WriteString(card.SourceOutcome)
		builder.WriteString("\n\n")
		builder.WriteString(card.Body)
	}

	return builder.String()
}

func requireExactJSONKeys(values map[string]json.RawMessage, expectedKeys []string) error {
	expectedSet := make(map[string]struct{}, len(expectedKeys))
	for _, key := range expectedKeys {
		expectedSet[key] = struct{}{}
	}

	for _, key := range expectedKeys {
		if _, ok := values[key]; !ok {
			return fmt.Errorf("missing required key %q", key)
		}
	}

	for key := range values {
		if _, ok := expectedSet[key]; !ok {
			return fmt.Errorf("unexpected key %q", key)
		}
	}

	return nil
}

func isJSONArray(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '['
}

func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

func boolPtr(value bool) *bool {
	return &value
}
