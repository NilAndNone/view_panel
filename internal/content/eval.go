package content

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	distinctnessQuestionSetSchemaVersion = "distinctness_questions_v1"
	distinctnessReportSchemaVersion      = "distinctness_report_v1"

	nearDuplicatePenalty  = 25
	missingAnchorPenalty  = 15
	genericFramingPenalty = 10

	nearDuplicateSimilarityThreshold = 0.9
	nearDuplicateMinTokenCount       = 8
	outputPreviewRuneLimit           = 160
)

var (
	distinctnessNormalizePattern = regexp.MustCompile(`[^a-z0-9]+`)
	genericPhrases               = []string{
		"it depends",
		"balanced perspective",
		"both sides",
		"on the other hand",
		"need more information",
		"need more context",
		"there are valid points on both sides",
	}
	distinctnessStopwords = map[string]struct{}{
		"about": {}, "after": {}, "again": {}, "along": {}, "also": {}, "although": {},
		"among": {}, "because": {}, "before": {}, "between": {}, "brief": {}, "change": {},
		"clear": {}, "concrete": {}, "could": {}, "decision": {}, "details": {}, "doing": {},
		"every": {}, "focus": {}, "good": {}, "have": {}, "into": {}, "limited": {},
		"look": {}, "more": {}, "need": {}, "operating": {}, "other": {}, "paths": {},
		"people": {}, "possible": {}, "rational": {}, "should": {}, "still": {}, "team": {},
		"that": {}, "their": {}, "there": {}, "these": {}, "thing": {}, "under": {},
		"using": {}, "valid": {}, "what": {}, "when": {}, "where": {}, "which": {},
		"while": {}, "would": {},
	}
)

type DistinctnessQuestionSet struct {
	SchemaVersion string                 `json:"schema_version"`
	Questions     []DistinctnessQuestion `json:"questions"`
}

type DistinctnessQuestion struct {
	QuestionID   string `json:"question_id"`
	MaterialID   string `json:"material_id"`
	AnchorDomain string `json:"anchor_domain"`
	Question     string `json:"question"`
}

type DistinctnessExecOptions struct {
	Model                 string
	Concurrency           int
	ReviewEnabled         bool
	WorkerTimeoutMS       int
	MaxAttemptsPerPersona int
	ForbiddenToolNames    []string
}

type PanelRunExecutor interface {
	RunPanel(context.Context, PanelRunRequest) (PanelRunResult, error)
}

type PanelRunRequest struct {
	BundleID              string
	BundleRoot            string
	PersonaSetPath        string
	MaterialPath          string
	Question              DistinctnessQuestion
	OutputRoot            string
	Model                 string
	Concurrency           int
	ReviewEnabled         bool
	WorkerTimeoutMS       int
	MaxAttemptsPerPersona int
	ForbiddenToolNames    []string
}

type PanelRunResult struct {
	QuestionID string             `json:"question_id"`
	MaterialID string             `json:"material_id"`
	RunID      string             `json:"run_id"`
	Variant    string             `json:"variant"`
	Cards      []DistinctnessCard `json:"cards"`
}

type DistinctnessCard struct {
	PersonaID     string `json:"persona_id"`
	SourceOutcome string `json:"source_outcome"`
	Headline      string `json:"headline"`
	Body          string `json:"body"`
}

type DistinctnessReport struct {
	SchemaVersion       string                  `json:"schema_version"`
	BundleID            string                  `json:"bundle_id"`
	BundlePath          string                  `json:"bundle_path"`
	QuestionSetPath     string                  `json:"question_set_path"`
	OutputRoot          string                  `json:"output_root"`
	RunCount            int                     `json:"run_count"`
	PersonaCount        int                     `json:"persona_count"`
	OverallScore        int                     `json:"overall_score"`
	NearDuplicateCount  int                     `json:"near_duplicate_count"`
	MissingAnchorCount  int                     `json:"missing_anchor_count"`
	GenericFramingCount int                     `json:"generic_framing_count"`
	Runs                []DistinctnessRunReport `json:"runs"`
}

type DistinctnessRunReport struct {
	QuestionID         string                       `json:"question_id"`
	MaterialID         string                       `json:"material_id"`
	AnchorDomain       string                       `json:"anchor_domain"`
	Question           string                       `json:"question"`
	RunID              string                       `json:"run_id"`
	Variant            string                       `json:"variant"`
	OutputCount        int                          `json:"output_count"`
	Score              int                          `json:"score"`
	NearDuplicatePairs []DistinctnessDuplicatePair  `json:"near_duplicate_pairs"`
	PersonaFindings    []DistinctnessPersonaFinding `json:"persona_findings"`
}

type DistinctnessDuplicatePair struct {
	PersonaIDLeft  string  `json:"persona_id_left"`
	PersonaIDRight string  `json:"persona_id_right"`
	Similarity     float64 `json:"similarity"`
}

type DistinctnessPersonaFinding struct {
	PersonaID          string   `json:"persona_id"`
	SourceOutcome      string   `json:"source_outcome"`
	MissingAnchors     bool     `json:"missing_anchors"`
	MissingAnchorsFrom []string `json:"missing_anchors_from,omitempty"`
	GenericFraming     bool     `json:"generic_framing"`
	AnchorHits         []string `json:"anchor_hits,omitempty"`
	GenericPhrases     []string `json:"generic_phrases,omitempty"`
	OutputPreview      string   `json:"output_preview"`
}

type distinctnessBundle struct {
	Root           string
	PersonaSetPath string
	MaterialPaths  map[string]string
	Bundle         Bundle
}

type personaAnchorSet struct {
	Phrases []string
	Tokens  []string
}

func EvaluateDistinctness(ctx context.Context, bundleRoot, questionSetPath, outRoot string, executor PanelRunExecutor, options DistinctnessExecOptions) (DistinctnessReport, error) {
	if executor == nil {
		return DistinctnessReport{}, fmt.Errorf("panel run executor is required")
	}
	if err := ctx.Err(); err != nil {
		return DistinctnessReport{}, err
	}

	loadedBundle, err := loadDistinctnessBundle(bundleRoot)
	if err != nil {
		return DistinctnessReport{}, err
	}
	questionSet, err := LoadDistinctnessQuestionSet(questionSetPath, loadedBundle.MaterialPaths)
	if err != nil {
		return DistinctnessReport{}, err
	}
	if err := validateDistinctnessQuestionSetAgainstBundle(questionSet, loadedBundle.Bundle); err != nil {
		return DistinctnessReport{}, err
	}

	reportRoot, err := absoluteCleanPath(outRoot)
	if err != nil {
		return DistinctnessReport{}, err
	}

	results := make([]PanelRunResult, 0, len(questionSet.Questions))
	for _, question := range questionSet.Questions {
		if err := ctx.Err(); err != nil {
			return DistinctnessReport{}, err
		}

		result, err := executor.RunPanel(ctx, PanelRunRequest{
			BundleID:              loadedBundle.Bundle.BundleID,
			BundleRoot:            loadedBundle.Root,
			PersonaSetPath:        loadedBundle.PersonaSetPath,
			MaterialPath:          loadedBundle.MaterialPaths[question.MaterialID],
			Question:              question,
			OutputRoot:            reportRoot,
			Model:                 strings.TrimSpace(options.Model),
			Concurrency:           options.Concurrency,
			ReviewEnabled:         options.ReviewEnabled,
			WorkerTimeoutMS:       options.WorkerTimeoutMS,
			MaxAttemptsPerPersona: options.MaxAttemptsPerPersona,
			ForbiddenToolNames:    append([]string(nil), options.ForbiddenToolNames...),
		})
		if err != nil {
			return DistinctnessReport{}, fmt.Errorf("run panel for question %q: %w", question.QuestionID, err)
		}
		if strings.TrimSpace(result.QuestionID) == "" {
			result.QuestionID = question.QuestionID
		}
		if strings.TrimSpace(result.MaterialID) == "" {
			result.MaterialID = question.MaterialID
		}
		results = append(results, result)
	}

	return BuildDistinctnessReport(loadedBundle.Bundle, loadedBundle.Root, questionSetPath, reportRoot, questionSet, results)
}

func LoadDistinctnessQuestionSet(path string, materialPaths map[string]string) (DistinctnessQuestionSet, error) {
	absolutePath, err := absoluteCleanPath(path)
	if err != nil {
		return DistinctnessQuestionSet{}, err
	}

	questionSet, err := decodeJSONFile[DistinctnessQuestionSet](absolutePath)
	if err != nil {
		return DistinctnessQuestionSet{}, fmt.Errorf("load distinctness question set: %w", err)
	}
	if questionSet.SchemaVersion != distinctnessQuestionSetSchemaVersion {
		return DistinctnessQuestionSet{}, fmt.Errorf("question set schema_version must be %q", distinctnessQuestionSetSchemaVersion)
	}
	if len(questionSet.Questions) == 0 {
		return DistinctnessQuestionSet{}, fmt.Errorf("distinctness question set must contain at least one question")
	}

	seen := make(map[string]struct{}, len(questionSet.Questions))
	normalized := make([]DistinctnessQuestion, 0, len(questionSet.Questions))
	for index, question := range questionSet.Questions {
		question.QuestionID = strings.TrimSpace(question.QuestionID)
		question.MaterialID = strings.TrimSpace(question.MaterialID)
		question.AnchorDomain = strings.TrimSpace(question.AnchorDomain)
		question.Question = strings.TrimSpace(question.Question)

		if err := validateSlug("question_id", question.QuestionID); err != nil {
			return DistinctnessQuestionSet{}, fmt.Errorf("validate distinctness question %d: %w", index, err)
		}
		if err := validateSlug("material_id", question.MaterialID); err != nil {
			return DistinctnessQuestionSet{}, fmt.Errorf("validate distinctness question %q: %w", question.QuestionID, err)
		}
		if err := validateSlug("anchor_domain", question.AnchorDomain); err != nil {
			return DistinctnessQuestionSet{}, fmt.Errorf("validate distinctness question %q: %w", question.QuestionID, err)
		}
		if question.Question == "" {
			return DistinctnessQuestionSet{}, fmt.Errorf("validate distinctness question %q: question must not be empty", question.QuestionID)
		}
		if _, ok := seen[question.QuestionID]; ok {
			return DistinctnessQuestionSet{}, fmt.Errorf("distinctness question set contains duplicate question_id %q", question.QuestionID)
		}
		if len(materialPaths) > 0 {
			if _, ok := materialPaths[question.MaterialID]; !ok {
				return DistinctnessQuestionSet{}, fmt.Errorf("distinctness question %q references unknown material_id %q", question.QuestionID, question.MaterialID)
			}
		}

		seen[question.QuestionID] = struct{}{}
		normalized = append(normalized, question)
	}
	questionSet.Questions = normalized
	return questionSet, nil
}

func BuildDistinctnessReport(bundle Bundle, bundlePath, questionSetPath, outRoot string, questionSet DistinctnessQuestionSet, runs []PanelRunResult) (DistinctnessReport, error) {
	if strings.TrimSpace(bundle.BundleID) == "" {
		return DistinctnessReport{}, fmt.Errorf("bundle_id must not be empty")
	}
	if len(bundle.PersonaOrder) == 0 {
		return DistinctnessReport{}, fmt.Errorf("bundle must contain at least one persona")
	}
	if questionSet.SchemaVersion != distinctnessQuestionSetSchemaVersion {
		return DistinctnessReport{}, fmt.Errorf("question set schema_version must be %q", distinctnessQuestionSetSchemaVersion)
	}
	if len(questionSet.Questions) == 0 {
		return DistinctnessReport{}, fmt.Errorf("question set must contain at least one question")
	}

	runByQuestionID := make(map[string]PanelRunResult, len(runs))
	for _, run := range runs {
		questionID := strings.TrimSpace(run.QuestionID)
		if questionID == "" {
			return DistinctnessReport{}, fmt.Errorf("panel run result missing question_id")
		}
		if _, exists := runByQuestionID[questionID]; exists {
			return DistinctnessReport{}, fmt.Errorf("duplicate panel run result for question_id %q", questionID)
		}
		runByQuestionID[questionID] = run
	}

	report := DistinctnessReport{
		SchemaVersion:   distinctnessReportSchemaVersion,
		BundleID:        bundle.BundleID,
		BundlePath:      filepath.Clean(bundlePath),
		QuestionSetPath: filepath.Clean(questionSetPath),
		OutputRoot:      filepath.Clean(outRoot),
		RunCount:        len(questionSet.Questions),
		PersonaCount:    len(bundle.PersonaOrder),
		Runs:            make([]DistinctnessRunReport, 0, len(questionSet.Questions)),
	}

	totalScore := 0
	for _, question := range questionSet.Questions {
		run, ok := runByQuestionID[question.QuestionID]
		if !ok {
			return DistinctnessReport{}, fmt.Errorf("missing panel run result for question_id %q", question.QuestionID)
		}
		runReport, err := buildDistinctnessRunReport(bundle, question, run)
		if err != nil {
			return DistinctnessReport{}, err
		}

		report.Runs = append(report.Runs, runReport)
		report.NearDuplicateCount += len(runReport.NearDuplicatePairs)
		totalScore += runReport.Score
		for _, finding := range runReport.PersonaFindings {
			if finding.MissingAnchors {
				report.MissingAnchorCount++
			}
			if finding.GenericFraming {
				report.GenericFramingCount++
			}
		}
	}

	report.OverallScore = clampScore(totalScore / len(report.Runs))
	return report, nil
}

func validateDistinctnessQuestionSetAgainstBundle(questionSet DistinctnessQuestionSet, bundle Bundle) error {
	for _, question := range questionSet.Questions {
		fieldName := "domain_" + question.AnchorDomain
		missingPersonaIDs := make([]string, 0)
		for _, personaID := range bundle.PersonaOrder {
			persona, ok := bundle.Personas[personaID]
			if !ok {
				missingPersonaIDs = append(missingPersonaIDs, personaID)
				continue
			}
			if strings.TrimSpace(persona.Fields[fieldName]) == "" {
				missingPersonaIDs = append(missingPersonaIDs, personaID)
			}
		}
		if len(missingPersonaIDs) > 0 {
			return fmt.Errorf("distinctness question %q has unsupported anchor_domain %q for bundle %q: every persona must expose %q; missing personas: %s", question.QuestionID, question.AnchorDomain, bundle.BundleID, fieldName, strings.Join(missingPersonaIDs, ", "))
		}
	}
	return nil
}

func buildDistinctnessRunReport(bundle Bundle, question DistinctnessQuestion, run PanelRunResult) (DistinctnessRunReport, error) {
	orderedCards, err := validateAndOrderRunCards(bundle, question, run)
	if err != nil {
		return DistinctnessRunReport{}, err
	}

	runReport := DistinctnessRunReport{
		QuestionID:         question.QuestionID,
		MaterialID:         question.MaterialID,
		AnchorDomain:       question.AnchorDomain,
		Question:           question.Question,
		RunID:              strings.TrimSpace(run.RunID),
		Variant:            strings.TrimSpace(run.Variant),
		OutputCount:        len(orderedCards),
		NearDuplicatePairs: make([]DistinctnessDuplicatePair, 0),
		PersonaFindings:    make([]DistinctnessPersonaFinding, 0, len(orderedCards)),
	}

	if runReport.RunID == "" {
		return DistinctnessRunReport{}, fmt.Errorf("panel run %q missing run_id", question.QuestionID)
	}
	if runReport.Variant == "" {
		return DistinctnessRunReport{}, fmt.Errorf("panel run %q missing variant", question.QuestionID)
	}

	for index, card := range orderedCards {
		personaID := bundle.PersonaOrder[index]
		persona := bundle.Personas[personaID]
		anchors, err := buildQuestionAnchorSet(persona, question.AnchorDomain)
		if err != nil {
			return DistinctnessRunReport{}, fmt.Errorf("question %q persona %q: %w", question.QuestionID, personaID, err)
		}

		outputText := cardText(card)
		anchorHits := findAnchorHits(outputText, anchors)
		genericHits := findGenericPhrases(outputText)
		finding := DistinctnessPersonaFinding{
			PersonaID:      personaID,
			SourceOutcome:  strings.TrimSpace(card.SourceOutcome),
			MissingAnchors: len(anchorHits) == 0,
			GenericFraming: len(genericHits) > 0 && len(anchorHits) == 0,
			AnchorHits:     anchorHits,
			GenericPhrases: genericHits,
			OutputPreview:  outputPreview(outputText),
		}
		if finding.MissingAnchors {
			finding.MissingAnchorsFrom = missingAnchorExamples(anchors)
		}
		runReport.PersonaFindings = append(runReport.PersonaFindings, finding)
	}

	for leftIndex := 0; leftIndex < len(orderedCards); leftIndex++ {
		for rightIndex := leftIndex + 1; rightIndex < len(orderedCards); rightIndex++ {
			isDuplicate, similarity := nearDuplicate(cardText(orderedCards[leftIndex]), cardText(orderedCards[rightIndex]))
			if !isDuplicate {
				continue
			}
			runReport.NearDuplicatePairs = append(runReport.NearDuplicatePairs, DistinctnessDuplicatePair{
				PersonaIDLeft:  bundle.PersonaOrder[leftIndex],
				PersonaIDRight: bundle.PersonaOrder[rightIndex],
				Similarity:     similarity,
			})
		}
	}

	missingAnchorCount := 0
	genericFramingCount := 0
	for _, finding := range runReport.PersonaFindings {
		if finding.MissingAnchors {
			missingAnchorCount++
		}
		if finding.GenericFraming {
			genericFramingCount++
		}
	}
	runReport.Score = clampScore(100 - len(runReport.NearDuplicatePairs)*nearDuplicatePenalty - missingAnchorCount*missingAnchorPenalty - genericFramingCount*genericFramingPenalty)
	return runReport, nil
}

func validateAndOrderRunCards(bundle Bundle, question DistinctnessQuestion, run PanelRunResult) ([]DistinctnessCard, error) {
	if strings.TrimSpace(run.QuestionID) != question.QuestionID {
		return nil, fmt.Errorf("panel run question_id %q does not match requested question_id %q", run.QuestionID, question.QuestionID)
	}
	if strings.TrimSpace(run.MaterialID) != question.MaterialID {
		return nil, fmt.Errorf("panel run material_id %q does not match requested material_id %q", run.MaterialID, question.MaterialID)
	}

	cardsByPersona := make(map[string]DistinctnessCard, len(run.Cards))
	for _, card := range run.Cards {
		personaID := strings.TrimSpace(card.PersonaID)
		if _, ok := bundle.Personas[personaID]; !ok {
			return nil, fmt.Errorf("question %q references unknown persona_id %q", question.QuestionID, personaID)
		}
		if _, exists := cardsByPersona[personaID]; exists {
			return nil, fmt.Errorf("question %q has duplicate rendered card for persona %q", question.QuestionID, personaID)
		}
		cardsByPersona[personaID] = card
	}

	if len(cardsByPersona) != len(bundle.PersonaOrder) {
		for _, personaID := range bundle.PersonaOrder {
			if _, ok := cardsByPersona[personaID]; !ok {
				return nil, fmt.Errorf("question %q is missing rendered card for persona %q", question.QuestionID, personaID)
			}
		}
		return nil, fmt.Errorf("question %q expected %d rendered cards but got %d", question.QuestionID, len(bundle.PersonaOrder), len(cardsByPersona))
	}

	ordered := make([]DistinctnessCard, 0, len(bundle.PersonaOrder))
	for _, personaID := range bundle.PersonaOrder {
		card, ok := cardsByPersona[personaID]
		if !ok {
			return nil, fmt.Errorf("question %q is missing rendered card for persona %q", question.QuestionID, personaID)
		}
		ordered = append(ordered, card)
	}
	return ordered, nil
}

func loadDistinctnessBundle(bundleRoot string) (distinctnessBundle, error) {
	absoluteRoot, err := absoluteCleanPath(bundleRoot)
	if err != nil {
		return distinctnessBundle{}, err
	}

	manifestPath := filepath.Join(absoluteRoot, bundleManifestFileName)
	manifest, err := decodeJSONFile[BundleManifest](manifestPath)
	if err != nil {
		return distinctnessBundle{}, fmt.Errorf("load bundle manifest: %w", err)
	}
	if manifest.SchemaVersion != contentBundleManifestSchemaVersion {
		return distinctnessBundle{}, fmt.Errorf("bundle manifest schema_version must be %q", contentBundleManifestSchemaVersion)
	}

	personaSetPath := filepath.Join(absoluteRoot, "runtime", "persona-index.json")
	personaIndex, err := decodeJSONFile[RuntimePersonaIndex](personaSetPath)
	if err != nil {
		return distinctnessBundle{}, fmt.Errorf("load runtime persona index: %w", err)
	}
	if len(personaIndex.Personas) == 0 {
		return distinctnessBundle{}, fmt.Errorf("runtime persona index must contain at least one persona")
	}

	loaded := distinctnessBundle{
		Root:           absoluteRoot,
		PersonaSetPath: personaSetPath,
		MaterialPaths:  make(map[string]string),
		Bundle: Bundle{
			BundleID:     manifest.BundleID,
			Manifest:     manifest,
			PersonaIndex: personaIndex,
			PersonaOrder: make([]string, 0, len(personaIndex.Personas)),
			Personas:     make(map[string]CompiledPersona, len(personaIndex.Personas)),
			Materials:    make(map[string]CompiledMaterial),
		},
	}

	for _, ref := range personaIndex.Personas {
		personaPath := filepath.Join(absoluteRoot, "runtime", filepath.FromSlash(strings.TrimSpace(ref)))
		document, err := decodeJSONFile[map[string]string](personaPath)
		if err != nil {
			return distinctnessBundle{}, fmt.Errorf("load runtime persona %q: %w", ref, err)
		}
		personaID := strings.TrimSpace(document["persona_id"])
		if personaID == "" {
			return distinctnessBundle{}, fmt.Errorf("runtime persona %q missing persona_id", ref)
		}
		delete(document, "persona_id")
		loaded.Bundle.PersonaOrder = append(loaded.Bundle.PersonaOrder, personaID)
		loaded.Bundle.Personas[personaID] = CompiledPersona{
			PersonaID: personaID,
			Fields:    document,
		}
	}

	for _, artifact := range manifest.Artifacts {
		if !strings.HasPrefix(artifact, "runtime/materials/") || !strings.HasSuffix(artifact, ".json") {
			continue
		}
		materialID := strings.TrimSuffix(filepath.Base(artifact), ".json")
		materialPath := filepath.Join(absoluteRoot, filepath.FromSlash(artifact))
		if _, err := os.Stat(materialPath); err != nil {
			return distinctnessBundle{}, fmt.Errorf("stat runtime material %q: %w", materialID, err)
		}
		loaded.MaterialPaths[materialID] = materialPath
		loaded.Bundle.MaterialOrder = append(loaded.Bundle.MaterialOrder, materialID)
	}
	sort.Strings(loaded.Bundle.MaterialOrder)
	if len(loaded.MaterialPaths) == 0 {
		return distinctnessBundle{}, fmt.Errorf("bundle manifest does not reference any runtime materials")
	}

	return loaded, nil
}

func buildQuestionAnchorSet(persona CompiledPersona, anchorDomain string) (personaAnchorSet, error) {
	domainField := "domain_" + anchorDomain
	domainBody, ok := persona.Fields[domainField]
	if !ok {
		return personaAnchorSet{}, fmt.Errorf("missing scoped anchor field %q", domainField)
	}

	phrases := uniqueStringsSorted(splitJoinedField(persona.Fields["reference_anchors"]))
	tokens := uniqueStringsSorted(extractAnchorTokens(domainBody))
	return personaAnchorSet{
		Phrases: phrases,
		Tokens:  tokens,
	}, nil
}

func splitJoinedField(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, "|")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		values = append(values, part)
	}
	return values
}

func extractAnchorTokens(text string) []string {
	rawTokens := normalizedTokens(text)
	anchors := make([]string, 0, len(rawTokens))
	for _, token := range rawTokens {
		if len(token) < 5 {
			continue
		}
		if _, stopword := distinctnessStopwords[token]; stopword {
			continue
		}
		anchors = append(anchors, token)
	}
	return anchors
}

func findAnchorHits(text string, anchors personaAnchorSet) []string {
	normalizedText := normalizeDistinctnessText(text)
	tokenSet := make(map[string]struct{})
	for _, token := range normalizedTokens(text) {
		tokenSet[token] = struct{}{}
	}

	hits := make([]string, 0)
	for _, phrase := range anchors.Phrases {
		if strings.Contains(normalizedText, normalizeDistinctnessText(phrase)) {
			hits = append(hits, phrase)
		}
	}
	for _, token := range anchors.Tokens {
		if _, ok := tokenSet[token]; ok {
			hits = append(hits, token)
		}
	}
	return uniqueStringsSorted(hits)
}

func findGenericPhrases(text string) []string {
	normalizedText := normalizeDistinctnessText(text)
	hits := make([]string, 0)
	for _, phrase := range genericPhrases {
		if strings.Contains(normalizedText, normalizeDistinctnessText(phrase)) {
			hits = append(hits, phrase)
		}
	}
	return hits
}

func nearDuplicate(left, right string) (bool, float64) {
	leftNormalized := normalizeDistinctnessText(left)
	rightNormalized := normalizeDistinctnessText(right)
	if leftNormalized == "" || rightNormalized == "" {
		return false, 0
	}
	if leftNormalized == rightNormalized {
		return true, 1
	}

	leftTokens := similarityTokens(left)
	rightTokens := similarityTokens(right)
	if minInt(len(leftTokens), len(rightTokens)) < nearDuplicateMinTokenCount {
		return false, 0
	}

	leftSet := make(map[string]struct{}, len(leftTokens))
	rightSet := make(map[string]struct{}, len(rightTokens))
	for _, token := range leftTokens {
		leftSet[token] = struct{}{}
	}
	for _, token := range rightTokens {
		rightSet[token] = struct{}{}
	}

	intersection := 0
	for token := range leftSet {
		if _, ok := rightSet[token]; ok {
			intersection++
		}
	}
	union := len(leftSet) + len(rightSet) - intersection
	if union == 0 {
		return false, 0
	}

	similarity := float64(intersection) / float64(union)
	return similarity >= nearDuplicateSimilarityThreshold, roundSimilarity(similarity)
}

func similarityTokens(text string) []string {
	tokens := normalizedTokens(text)
	values := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if len(token) < 3 {
			continue
		}
		if _, stopword := distinctnessStopwords[token]; stopword {
			continue
		}
		values = append(values, token)
	}
	return uniqueStringsSorted(values)
}

func cardText(card DistinctnessCard) string {
	parts := []string{
		strings.TrimSpace(card.Headline),
		strings.TrimSpace(card.Body),
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func outputPreview(text string) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) <= outputPreviewRuneLimit {
		return text
	}
	return strings.TrimSpace(string(runes[:outputPreviewRuneLimit-3])) + "..."
}

func missingAnchorExamples(anchors personaAnchorSet) []string {
	examples := append([]string(nil), anchors.Phrases...)
	examples = append(examples, anchors.Tokens...)
	examples = uniqueStringsSorted(examples)
	if len(examples) > 3 {
		examples = examples[:3]
	}
	return examples
}

func normalizeDistinctnessText(text string) string {
	normalized := distinctnessNormalizePattern.ReplaceAllString(strings.ToLower(text), " ")
	return strings.Join(strings.Fields(normalized), " ")
}

func normalizedTokens(text string) []string {
	return strings.Fields(normalizeDistinctnessText(text))
}

func uniqueStringsSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	sort.Strings(unique)
	return unique
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func roundSimilarity(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func absoluteCleanPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", path, err)
	}
	return filepath.Clean(absolutePath), nil
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
