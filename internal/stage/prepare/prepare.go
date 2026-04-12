package prepare

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	panelhash "view_panel/internal/hash"
	"view_panel/internal/storage"
)

const (
	prepareStageName              = "01_prepare"
	dispatchInputArtifactName     = "dispatch_input_v1.json"
	agentsArtifactName            = "agents.md"
	promptArtifactName            = "prompt.txt"
	manifestArtifactName          = "manifest.json"
	hashesArtifactName            = "hashes.json"
	prepareReviewArtifactName     = "prepare_review_v1.json"
	prepareGateStatusArtifactName = "prepare_gate_status_v1.json"
	prepareManifestSchemaVersion  = "prepare_manifest_v1"
	prepareGateSchemaVersion      = "prepare_gate_status_v1"
	prepareReviewSchemaVersion    = "prepare_review_v1"
	reviewModeAdvisory            = "advisory"
	reviewStatusCompletedClean    = "completed_clean"
	reviewStatusCompletedWithWarn = "completed_with_warnings"
	reviewStatusUnavailable       = "review_unavailable"
	statusPass                    = "pass"
	statusFail                    = "fail"
)

var canonicalSendableArtifacts = [...]string{
	dispatchInputArtifactName,
	agentsArtifactName,
	promptArtifactName,
}

// DispatchInputV1 is the only sendable Stage 1 structured input.
type DispatchInputV1 struct {
	RoleplayPrompt            string `json:"roleplay_prompt"`
	DiscussionQuestion        string `json:"discussion_question"`
	SupplementaryMaterials    string `json:"supplementary_materials"`
	OutputContract            string `json:"output_contract"`
	AssumptionsAndConstraints string `json:"assumptions_and_constraints"`
}

// PersonaRecord is the prepare-stage persona identity resolved from the
// configured persona-set index.
type PersonaRecord struct {
	PersonaID  string         `json:"persona_id"`
	SourcePath string         `json:"source_path,omitempty"`
	Fields     map[string]any `json:"fields,omitempty"`
}

// PrepareManifest is the immutable Stage 1 artifact inventory.
type PrepareManifest struct {
	SchemaVersion string                  `json:"schema_version"`
	Stage         string                  `json:"stage"`
	PersonaID     string                  `json:"persona_id"`
	Artifacts     []PrepareManifestMember `json:"artifacts"`
}

// PrepareManifestMember is one sendable bundle member recorded in manifest.json.
type PrepareManifestMember struct {
	Path      string `json:"path"`
	HashRef   string `json:"hash_ref"`
	SizeBytes int64  `json:"size_bytes"`
}

// PrepareHashes is the immutable Stage 1 hash ledger.
type PrepareHashes struct {
	DispatchInputSHA256 string `json:"dispatch_input_sha256"`
	AgentsSHA256        string `json:"agents_sha256"`
	PromptSHA256        string `json:"prompt_sha256"`
	BundleSHA256        string `json:"bundle_sha256"`
}

// AdvisoryReviewer is the runtime seam for the P06 advisory review session.
type AdvisoryReviewer interface {
	ReviewPrepare(ctx context.Context, request AdvisoryReviewRequest) ([]byte, error)
}

// PrepareRequest is the Stage 1 input required to assemble canonical per-persona
// bundles. The runtime may provide already-loaded materials/personas directly or
// inject loader callbacks while higher-level orchestration remains in flux.
type PrepareRequest struct {
	RunRoot string

	Materials     DispatchInputV1
	Personas      []PersonaRecord
	LoadMaterials func(context.Context) (DispatchInputV1, error)
	LoadPersonas  func(context.Context) ([]PersonaRecord, error)

	Reviewer AdvisoryReviewer
}

// PrepareResult captures the prepare-stage outputs needed by downstream stages.
type PrepareResult struct {
	PersonaIDs []string
	Review     PrepareReviewArtifact
	Gate       PrepareGateStatus
}

// Run assembles the canonical Stage 1 artifacts, emits advisory review evidence,
// then emits the hard gate status artifact that owns Stage 2 admission.
func Run(ctx context.Context, request PrepareRequest) (PrepareResult, error) {
	if strings.TrimSpace(request.RunRoot) == "" {
		return PrepareResult{}, fmt.Errorf("prepare run_root must not be empty")
	}
	if err := assertCanonicalSendableContract(); err != nil {
		return PrepareResult{}, err
	}

	materials, err := resolveMaterials(ctx, request)
	if err != nil {
		return PrepareResult{}, err
	}

	personas, err := resolvePersonas(ctx, request)
	if err != nil {
		return PrepareResult{}, err
	}

	for _, persona := range sortPersonasLex(personas) {
		if err := writePersonaArtifacts(request.RunRoot, materials, persona); err != nil {
			return PrepareResult{}, fmt.Errorf("prepare persona %q: %w", persona.PersonaID, err)
		}
	}

	reviewArtifact, err := runAdvisoryReview(ctx, request.Reviewer, request.RunRoot)
	if err != nil {
		return PrepareResult{}, err
	}

	gateArtifact, err := runHardGate(ctx, request.RunRoot)
	if err != nil {
		return PrepareResult{
			PersonaIDs: extractPersonaIDs(sortPersonasLex(personas)),
			Review:     reviewArtifact,
		}, err
	}

	return PrepareResult{
		PersonaIDs: gateArtifact.PersonaIDs,
		Review:     reviewArtifact,
		Gate:       gateArtifact,
	}, nil
}

func resolveMaterials(ctx context.Context, request PrepareRequest) (DispatchInputV1, error) {
	if request.LoadMaterials != nil {
		return request.LoadMaterials(ctx)
	}
	return request.Materials, nil
}

func resolvePersonas(ctx context.Context, request PrepareRequest) ([]PersonaRecord, error) {
	var personas []PersonaRecord
	if request.LoadPersonas != nil {
		resolved, err := request.LoadPersonas(ctx)
		if err != nil {
			return nil, err
		}
		personas = resolved
	} else {
		personas = request.Personas
	}

	seenIDs := make(map[string]struct{}, len(personas))
	for _, persona := range personas {
		if strings.TrimSpace(persona.PersonaID) == "" {
			return nil, fmt.Errorf("persona_id must not be empty")
		}
		if strings.ContainsRune(persona.PersonaID, filepath.Separator) {
			return nil, fmt.Errorf("persona_id %q must be a single path segment", persona.PersonaID)
		}
		if _, exists := seenIDs[persona.PersonaID]; exists {
			return nil, fmt.Errorf("duplicate persona_id %q", persona.PersonaID)
		}
		seenIDs[persona.PersonaID] = struct{}{}
	}

	return append([]PersonaRecord(nil), personas...), nil
}

func writePersonaArtifacts(runRoot string, materials DispatchInputV1, persona PersonaRecord) error {
	dispatchInput, err := renderDispatchInput(materials, persona)
	if err != nil {
		return err
	}

	dispatchBytes, err := panelhash.CanonicalJSONBytes(dispatchInput)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", dispatchInputArtifactName, err)
	}
	dispatchHash, err := panelhash.SHA256HexJSON(dispatchInput)
	if err != nil {
		return fmt.Errorf("hash %s: %w", dispatchInputArtifactName, err)
	}

	agentsText := dispatchInput.RoleplayPrompt
	promptText := strings.Join([]string{
		dispatchInput.DiscussionQuestion,
		dispatchInput.SupplementaryMaterials,
		dispatchInput.OutputContract,
		dispatchInput.AssumptionsAndConstraints,
	}, "\n\n")

	agentsBytes := []byte(agentsText)
	promptBytes := []byte(promptText)

	dispatchPath, err := storage.PreparePersonaArtifactPath(runRoot, persona.PersonaID, dispatchInputArtifactName)
	if err != nil {
		return fmt.Errorf("resolve %s path: %w", dispatchInputArtifactName, err)
	}
	agentsPath, err := storage.PreparePersonaArtifactPath(runRoot, persona.PersonaID, agentsArtifactName)
	if err != nil {
		return fmt.Errorf("resolve %s path: %w", agentsArtifactName, err)
	}
	promptPath, err := storage.PreparePersonaArtifactPath(runRoot, persona.PersonaID, promptArtifactName)
	if err != nil {
		return fmt.Errorf("resolve %s path: %w", promptArtifactName, err)
	}
	manifestPath, err := storage.PreparePersonaArtifactPath(runRoot, persona.PersonaID, manifestArtifactName)
	if err != nil {
		return fmt.Errorf("resolve %s path: %w", manifestArtifactName, err)
	}
	hashesPath, err := storage.PreparePersonaArtifactPath(runRoot, persona.PersonaID, hashesArtifactName)
	if err != nil {
		return fmt.Errorf("resolve %s path: %w", hashesArtifactName, err)
	}

	hashes := PrepareHashes{
		DispatchInputSHA256: dispatchHash,
		AgentsSHA256:        panelhash.SHA256Hex(agentsBytes),
		PromptSHA256:        panelhash.SHA256Hex(promptBytes),
		BundleSHA256:        bundleSHA256(dispatchBytes, agentsBytes, promptBytes),
	}

	manifest := PrepareManifest{
		SchemaVersion: prepareManifestSchemaVersion,
		Stage:         prepareStageName,
		PersonaID:     persona.PersonaID,
		Artifacts: []PrepareManifestMember{
			{
				Path:      dispatchInputArtifactName,
				HashRef:   "dispatch_input_sha256",
				SizeBytes: int64(len(dispatchBytes)),
			},
			{
				Path:      agentsArtifactName,
				HashRef:   "agents_sha256",
				SizeBytes: int64(len(agentsBytes)),
			},
			{
				Path:      promptArtifactName,
				HashRef:   "prompt_sha256",
				SizeBytes: int64(len(promptBytes)),
			},
		},
	}

	if err := storage.WriteJSON(dispatchPath, dispatchInput); err != nil {
		return fmt.Errorf("write %s: %w", dispatchInputArtifactName, err)
	}
	if err := storage.WriteText(agentsPath, agentsText); err != nil {
		return fmt.Errorf("write %s: %w", agentsArtifactName, err)
	}
	if err := storage.WriteText(promptPath, promptText); err != nil {
		return fmt.Errorf("write %s: %w", promptArtifactName, err)
	}
	if err := storage.WriteJSON(manifestPath, manifest); err != nil {
		return fmt.Errorf("write %s: %w", manifestArtifactName, err)
	}
	if err := storage.WriteJSON(hashesPath, hashes); err != nil {
		return fmt.Errorf("write %s: %w", hashesArtifactName, err)
	}

	return nil
}

func renderDispatchInput(materials DispatchInputV1, persona PersonaRecord) (DispatchInputV1, error) {
	roleplayPrompt, err := renderPersonaAwareSection(materials.RoleplayPrompt, persona)
	if err != nil {
		return DispatchInputV1{}, fmt.Errorf("render roleplay_prompt: %w", err)
	}
	discussionQuestion, err := renderPersonaAwareSection(materials.DiscussionQuestion, persona)
	if err != nil {
		return DispatchInputV1{}, fmt.Errorf("render discussion_question: %w", err)
	}
	supplementaryMaterials, err := renderPersonaAwareSection(materials.SupplementaryMaterials, persona)
	if err != nil {
		return DispatchInputV1{}, fmt.Errorf("render supplementary_materials: %w", err)
	}
	outputContract, err := renderPersonaAwareSection(materials.OutputContract, persona)
	if err != nil {
		return DispatchInputV1{}, fmt.Errorf("render output_contract: %w", err)
	}
	assumptionsAndConstraints, err := renderPersonaAwareSection(materials.AssumptionsAndConstraints, persona)
	if err != nil {
		return DispatchInputV1{}, fmt.Errorf("render assumptions_and_constraints: %w", err)
	}

	return DispatchInputV1{
		RoleplayPrompt:            roleplayPrompt,
		DiscussionQuestion:        discussionQuestion,
		SupplementaryMaterials:    supplementaryMaterials,
		OutputContract:            outputContract,
		AssumptionsAndConstraints: assumptionsAndConstraints,
	}, nil
}

type personaTemplateContext struct {
	PersonaID  string
	SourcePath string
	Fields     map[string]any
}

func renderPersonaAwareSection(source string, persona PersonaRecord) (string, error) {
	if !strings.Contains(source, "{{") {
		return source, nil
	}

	tmpl, err := template.New("prepare-section").Option("missingkey=error").Parse(source)
	if err != nil {
		return "", err
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, personaTemplateContext{
		PersonaID:  persona.PersonaID,
		SourcePath: persona.SourcePath,
		Fields:     persona.Fields,
	}); err != nil {
		return "", err
	}

	return buffer.String(), nil
}

func extractPersonaIDs(personas []PersonaRecord) []string {
	ids := make([]string, 0, len(personas))
	for _, persona := range personas {
		ids = append(ids, persona.PersonaID)
	}
	return ids
}

func sortPersonasLex(personas []PersonaRecord) []PersonaRecord {
	sorted := append([]PersonaRecord(nil), personas...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PersonaID < sorted[j].PersonaID
	})
	return sorted
}

func assertCanonicalSendableContract() error {
	expected := []string{
		dispatchInputArtifactName,
		agentsArtifactName,
		promptArtifactName,
	}
	actual := canonicalSendableArtifacts[:]
	if len(actual) != len(expected) {
		return fmt.Errorf("prepare sendable artifact contract mismatch")
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return fmt.Errorf("prepare sendable artifact contract mismatch")
		}
	}
	return nil
}

func bundleSHA256(payloads ...[]byte) string {
	var framed bytes.Buffer
	for _, payload := range payloads {
		var lengthPrefix [8]byte
		binary.BigEndian.PutUint64(lengthPrefix[:], uint64(len(payload)))
		framed.Write(lengthPrefix[:])
		framed.Write(payload)
	}
	return panelhash.SHA256Hex(framed.Bytes())
}
