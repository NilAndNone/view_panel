package answer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	panelhash "view_panel/internal/hash"
	"view_panel/internal/storage"
)

type AppServerLaunchContext struct {
	ExtraArgs               []string          `json:"extra_args,omitempty"`
	Environment             map[string]string `json:"environment,omitempty"`
	CurrentWorkingDirectory string            `json:"current_working_directory"`
	HomeDir                 string            `json:"home_dir"`
	CodexHomeDir            string            `json:"codex_home_dir"`
}

type RunSingleTurnOptions struct {
	Input string `json:"input"`
}

type StartedAppServer interface{}

type StartAppServerFunc func(ctx context.Context, launchContext AppServerLaunchContext) (StartedAppServer, error)
type RunSingleTurnFunc func(ctx context.Context, client StartedAppServer, options RunSingleTurnOptions) (SingleTurnRunResult, error)

type Runner struct {
	StartAppServer StartAppServerFunc
	RunSingleTurn  RunSingleTurnFunc
}

type ExecuteWorkerRequest struct {
	Workspace SealedPersonaWorkspace
	ExtraArgs []string
}

type ExecuteWorkerResult struct {
	PersonaID          string
	Status             *workerStatusArtifact
	Attestation        *workerAttestationArtifact
	Result             *workerResultArtifact
	ObservedToolCalls  []observedToolCall
	Launched           bool
	StatusArtifactPath string
	AttestationPath    string
	ResultJSONPath     string
	ResultRawPath      string
}

type SingleTurnRunResult struct {
	TerminalOutcome  string                 `json:"terminal_outcome"`
	LastReachedPhase string                 `json:"last_reached_phase,omitempty"`
	CloseOutcomeKind string                 `json:"close_outcome_kind,omitempty"`
	ThreadID         string                 `json:"thread_id,omitempty"`
	TurnID           string                 `json:"turn_id,omitempty"`
	CompletedItem    *CompletedAgentItem    `json:"completed_item,omitempty"`
	Transcript       []DiagnosticStreamItem `json:"transcript,omitempty"`
}

type CompletedAgentItem struct {
	ItemType string `json:"item_type"`
	ItemID   string `json:"item_id"`
	Text     string `json:"text"`
	Payload  any    `json:"payload,omitempty"`
}

type DiagnosticStreamItem struct {
	ItemType string `json:"item_type,omitempty"`
	ItemID   string `json:"item_id,omitempty"`
	Text     string `json:"text,omitempty"`
	ToolName string `json:"tool_name,omitempty"`
	CallID   string `json:"call_id,omitempty"`
	Payload  any    `json:"payload,omitempty"`
}

type workerResultArtifact struct {
	SchemaVersion             string `json:"schema_version"`
	Stage                     string `json:"stage"`
	PersonaID                 string `json:"persona_id"`
	SourceOutgoingInputPath   string `json:"source_outgoing_input_path"`
	SourceOutgoingInputSHA256 string `json:"source_outgoing_input_sha256"`
	CombinedInputSHA256       string `json:"combined_input_sha256"`
	AuthoritativeItemType     string `json:"authoritative_item_type"`
	AuthoritativeItemID       string `json:"authoritative_item_id"`
	Text                      string `json:"text"`
	TextSHA256                string `json:"text_sha256"`
	AuthoritativeItem         any    `json:"authoritative_item"`
}

type workerAttestationArtifact struct {
	SchemaVersion              string             `json:"schema_version"`
	Stage                      string             `json:"stage"`
	PersonaID                  string             `json:"persona_id"`
	SourceOutgoingInputPath    string             `json:"source_outgoing_input_path"`
	SourceOutgoingInputSHA256  *string            `json:"source_outgoing_input_sha256"`
	CombinedInputSHA256        *string            `json:"combined_input_sha256"`
	AgentInstructionsSHA256    *string            `json:"agent_instructions_sha256"`
	PromptSHA256               *string            `json:"prompt_sha256"`
	SkillPath                  *string            `json:"skill_path"`
	SkillSHA256                *string            `json:"skill_sha256"`
	RunnerTerminalOutcome      *string            `json:"runner_terminal_outcome"`
	LastReachedPhase           *string            `json:"last_reached_phase"`
	CloseOutcomeKind           *string            `json:"close_outcome_kind"`
	AuthoritativeOutputPresent bool               `json:"authoritative_output_present"`
	AuthoritativeItemType      *string            `json:"authoritative_item_type"`
	AuthoritativeItemID        *string            `json:"authoritative_item_id"`
	AuthoritativeTextSHA256    *string            `json:"authoritative_text_sha256"`
	ResultJSONSHA256           *string            `json:"result_json_sha256"`
	ResultRawTXTSHA256         *string            `json:"result_raw_txt_sha256"`
	ResultRawMode              *string            `json:"result_raw_mode"`
	ObservedToolCalls          []observedToolCall `json:"observed_tool_calls"`
	DiagnosticStreamItemCount  int                `json:"diagnostic_stream_item_count"`
}

type workerStatusArtifact struct {
	SchemaVersion              string  `json:"schema_version"`
	Stage                      string  `json:"stage"`
	PersonaID                  string  `json:"persona_id"`
	Outcome                    string  `json:"outcome"`
	LastReachedPhase           *string `json:"last_reached_phase"`
	CloseOutcomeKind           *string `json:"close_outcome_kind"`
	AuthoritativeOutputPresent bool    `json:"authoritative_output_present"`
	AuthoritativeItemType      *string `json:"authoritative_item_type"`
	AuthoritativeItemID        *string `json:"authoritative_item_id"`
	ResultRawPresent           bool    `json:"result_raw_present"`
	ResultJSONPresent          bool    `json:"result_json_present"`
	AttestationPresent         bool    `json:"attestation_present"`
	SourceOutgoingInputPath    string  `json:"source_outgoing_input_path"`
	ResultRawPath              *string `json:"result_raw_path"`
	ResultJSONPath             *string `json:"result_json_path"`
	AttestationPath            string  `json:"attestation_path"`
}

type observedToolCall struct {
	ToolName string  `json:"tool_name"`
	CallID   *string `json:"call_id"`
}

type verifiedSealedInputs struct {
	OutgoingRecord     *outgoingInputRecord
	OutgoingSHA256     *string
	AgentsBytes        []byte
	PromptBytes        []byte
	SkillBytes         []byte
	VerificationFailed bool
}

func (r Runner) Execute(ctx context.Context, req ExecuteWorkerRequest) (ExecuteWorkerResult, error) {
	statusPath, err := storage.AnswerPersonaArtifactPath(req.Workspace.RunRoot, req.Workspace.PersonaID, statusArtifactName)
	if err != nil {
		return ExecuteWorkerResult{}, fmt.Errorf("resolve status.json path: %w", err)
	}
	attestationPath, err := storage.AnswerPersonaArtifactPath(req.Workspace.RunRoot, req.Workspace.PersonaID, attestationArtifactName)
	if err != nil {
		return ExecuteWorkerResult{}, fmt.Errorf("resolve attestation.json path: %w", err)
	}
	resultJSONPath, err := storage.AnswerPersonaArtifactPath(req.Workspace.RunRoot, req.Workspace.PersonaID, resultJSONArtifactName)
	if err != nil {
		return ExecuteWorkerResult{}, fmt.Errorf("resolve result.json path: %w", err)
	}
	resultRawPath, err := storage.AnswerPersonaArtifactPath(req.Workspace.RunRoot, req.Workspace.PersonaID, resultRawArtifactName)
	if err != nil {
		return ExecuteWorkerResult{}, fmt.Errorf("resolve result.raw.txt path: %w", err)
	}

	result := ExecuteWorkerResult{
		PersonaID:          req.Workspace.PersonaID,
		StatusArtifactPath: statusPath,
		AttestationPath:    attestationPath,
		ResultJSONPath:     resultJSONPath,
		ResultRawPath:      resultRawPath,
	}

	if r.StartAppServer == nil {
		return result, errors.New("StartAppServer is required")
	}
	if r.RunSingleTurn == nil {
		return result, errors.New("RunSingleTurn is required")
	}

	verified, verificationErr := verifySealedInputs(req.Workspace)
	if verificationErr != nil {
		attestation := workerAttestationArtifact{
			SchemaVersion:              workerAttestationSchemaV1,
			Stage:                      answerStage,
			PersonaID:                  req.Workspace.PersonaID,
			SourceOutgoingInputPath:    outgoingInputPathLiteral,
			SourceOutgoingInputSHA256:  verified.OutgoingSHA256,
			CombinedInputSHA256:        outgoingFieldPtr(verified.OutgoingRecord, func(record *outgoingInputRecord) string { return record.CombinedInputSHA256 }),
			AgentInstructionsSHA256:    outgoingFieldPtr(verified.OutgoingRecord, func(record *outgoingInputRecord) string { return record.AgentInstructionsSHA256 }),
			PromptSHA256:               outgoingFieldPtr(verified.OutgoingRecord, func(record *outgoingInputRecord) string { return record.PromptSHA256 }),
			SkillPath:                  outgoingFieldPtr(verified.OutgoingRecord, func(record *outgoingInputRecord) string { return record.SkillPath }),
			SkillSHA256:                outgoingFieldPtr(verified.OutgoingRecord, func(record *outgoingInputRecord) string { return record.SkillSHA256 }),
			RunnerTerminalOutcome:      stringPtr(statusOutcomeFailed),
			LastReachedPhase:           stringPtr(prelaunchVerificationPhase),
			CloseOutcomeKind:           stringPtr(rejectedBeforeLaunchOutcome),
			AuthoritativeOutputPresent: false,
			AuthoritativeItemType:      nil,
			AuthoritativeItemID:        nil,
			AuthoritativeTextSHA256:    nil,
			ResultJSONSHA256:           nil,
			ResultRawTXTSHA256:         nil,
			ResultRawMode:              nil,
			ObservedToolCalls:          []observedToolCall{},
			DiagnosticStreamItemCount:  0,
		}
		if err := storage.WriteJSON(attestationPath, attestation); err != nil {
			return result, fmt.Errorf("write attestation after prelaunch rejection: %w", err)
		}

		status := workerStatusArtifact{
			SchemaVersion:              workerStatusSchemaV1,
			Stage:                      answerStage,
			PersonaID:                  req.Workspace.PersonaID,
			Outcome:                    statusOutcomeFailed,
			LastReachedPhase:           stringPtr(prelaunchVerificationPhase),
			CloseOutcomeKind:           stringPtr(rejectedBeforeLaunchOutcome),
			AuthoritativeOutputPresent: false,
			AuthoritativeItemType:      nil,
			AuthoritativeItemID:        nil,
			ResultRawPresent:           false,
			ResultJSONPresent:          false,
			AttestationPresent:         true,
			SourceOutgoingInputPath:    outgoingInputPathLiteral,
			ResultRawPath:              nil,
			ResultJSONPath:             nil,
			AttestationPath:            attestationArtifactName,
		}
		if err := storage.WriteJSON(statusPath, status); err != nil {
			return result, fmt.Errorf("write status after prelaunch rejection: %w", err)
		}

		result.Status = &status
		result.Attestation = &attestation
		result.ObservedToolCalls = attestation.ObservedToolCalls
		return result, nil
	}

	turnInput := string(verified.SkillBytes) + "\n\n" + string(verified.PromptBytes)
	startedClient, launchErr := r.StartAppServer(ctx, req.Workspace.ExecutionEnv.LaunchContext(req.ExtraArgs))

	terminalOutcome := (*string)(nil)
	lastReachedPhase := (*string)(nil)
	closeOutcomeKind := (*string)(nil)
	observedToolCalls := []observedToolCall{}
	var authoritative *CompletedAgentItem
	var runResult SingleTurnRunResult
	launched := launchErr == nil
	if launched {
		result.Launched = true
		var runErr error
		runResult, runErr = r.RunSingleTurn(ctx, startedClient, RunSingleTurnOptions{Input: turnInput})
		if normalized := normalizeTerminalOutcome(runResult.TerminalOutcome); normalized != "" {
			terminalOutcome = stringPtr(normalized)
		}
		if strings.TrimSpace(runResult.LastReachedPhase) != "" {
			lastReachedPhase = stringPtr(runResult.LastReachedPhase)
		}
		if strings.TrimSpace(runResult.CloseOutcomeKind) != "" {
			closeOutcomeKind = stringPtr(runResult.CloseOutcomeKind)
		}
		observedToolCalls = collectObservedToolCalls(runResult.Transcript)
		authoritative = extractAuthoritativeCompletedItem(runResult)
		if runErr != nil && terminalOutcome == nil {
			launchErr = runErr
		}
	}

	if err := os.Remove(resultJSONPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("delete stale result.json: %w", err)
	}

	rawPresent := false
	var rawSHA *string
	var rawMode *string
	if launched {
		rawPresent = true
		rawBytes, mode := diagnosticRawOutput(runResult.Transcript, authoritative)
		if err := storage.WriteText(resultRawPath, string(rawBytes)); err != nil {
			return result, fmt.Errorf("write result.raw.txt: %w", err)
		}
		rawHash, err := panelhash.SHA256HexFile(resultRawPath)
		if err != nil {
			return result, fmt.Errorf("hash result.raw.txt: %w", err)
		}
		rawSHA = &rawHash
		rawMode = &mode
	}

	var resultArtifact *workerResultArtifact
	var resultJSONSHA *string
	authoritativeOutputPresent := terminalOutcome != nil && *terminalOutcome == statusOutcomeCompleted && authoritative != nil
	var authoritativeItemType *string
	var authoritativeItemID *string
	var authoritativeTextSHA *string
	if authoritativeOutputPresent {
		authoritativeItemType = stringPtr(authoritativeCompletedItemType)
		authoritativeItemID = stringPtr(authoritative.ItemID)
		textSHA := panelhash.SHA256Hex([]byte(authoritative.Text))
		authoritativeTextSHA = &textSHA
		resultArtifact = &workerResultArtifact{
			SchemaVersion:             workerResultSchemaV1,
			Stage:                     answerStage,
			PersonaID:                 req.Workspace.PersonaID,
			SourceOutgoingInputPath:   outgoingInputPathLiteral,
			SourceOutgoingInputSHA256: derefString(verified.OutgoingSHA256),
			CombinedInputSHA256:       verified.OutgoingRecord.CombinedInputSHA256,
			AuthoritativeItemType:     authoritativeCompletedItemType,
			AuthoritativeItemID:       authoritative.ItemID,
			Text:                      authoritative.Text,
			TextSHA256:                textSHA,
			AuthoritativeItem:         completedItemPayload(authoritative),
		}
		if err := storage.WriteJSON(resultJSONPath, resultArtifact); err != nil {
			return result, fmt.Errorf("write result.json: %w", err)
		}
		sum, err := panelhash.SHA256HexFile(resultJSONPath)
		if err != nil {
			return result, fmt.Errorf("hash result.json: %w", err)
		}
		resultJSONSHA = &sum
	}

	statusOutcome := statusOutcomeFailed
	if terminalOutcome != nil {
		statusOutcome = *terminalOutcome
	}

	if launchErr != nil && !launched {
		statusOutcome = statusOutcomeFailed
		terminalOutcome = nil
		lastReachedPhase = nil
		closeOutcomeKind = nil
	}

	attestation := workerAttestationArtifact{
		SchemaVersion:              workerAttestationSchemaV1,
		Stage:                      answerStage,
		PersonaID:                  req.Workspace.PersonaID,
		SourceOutgoingInputPath:    outgoingInputPathLiteral,
		SourceOutgoingInputSHA256:  verified.OutgoingSHA256,
		CombinedInputSHA256:        stringPtr(verified.OutgoingRecord.CombinedInputSHA256),
		AgentInstructionsSHA256:    stringPtr(verified.OutgoingRecord.AgentInstructionsSHA256),
		PromptSHA256:               stringPtr(verified.OutgoingRecord.PromptSHA256),
		SkillPath:                  stringPtr(verified.OutgoingRecord.SkillPath),
		SkillSHA256:                stringPtr(verified.OutgoingRecord.SkillSHA256),
		RunnerTerminalOutcome:      terminalOutcome,
		LastReachedPhase:           lastReachedPhase,
		CloseOutcomeKind:           closeOutcomeKind,
		AuthoritativeOutputPresent: authoritativeOutputPresent,
		AuthoritativeItemType:      authoritativeItemType,
		AuthoritativeItemID:        authoritativeItemID,
		AuthoritativeTextSHA256:    authoritativeTextSHA,
		ResultJSONSHA256:           resultJSONSHA,
		ResultRawTXTSHA256:         rawSHA,
		ResultRawMode:              rawMode,
		ObservedToolCalls:          observedToolCalls,
		DiagnosticStreamItemCount:  len(runResult.Transcript),
	}
	if err := storage.WriteJSON(attestationPath, attestation); err != nil {
		return result, fmt.Errorf("write attestation.json: %w", err)
	}

	status := workerStatusArtifact{
		SchemaVersion:              workerStatusSchemaV1,
		Stage:                      answerStage,
		PersonaID:                  req.Workspace.PersonaID,
		Outcome:                    statusOutcome,
		LastReachedPhase:           lastReachedPhase,
		CloseOutcomeKind:           closeOutcomeKind,
		AuthoritativeOutputPresent: authoritativeOutputPresent,
		AuthoritativeItemType:      authoritativeItemType,
		AuthoritativeItemID:        authoritativeItemID,
		ResultRawPresent:           rawPresent,
		ResultJSONPresent:          resultArtifact != nil,
		AttestationPresent:         true,
		SourceOutgoingInputPath:    outgoingInputPathLiteral,
		ResultRawPath:              nullableArtifactPath(rawPresent, resultRawArtifactName),
		ResultJSONPath:             nullableArtifactPath(resultArtifact != nil, resultJSONArtifactName),
		AttestationPath:            attestationArtifactName,
	}
	if err := storage.WriteJSON(statusPath, status); err != nil {
		return result, fmt.Errorf("write status.json: %w", err)
	}

	result.Status = &status
	result.Attestation = &attestation
	result.Result = resultArtifact
	result.ObservedToolCalls = observedToolCalls

	if launchErr != nil {
		return result, launchErr
	}

	return result, nil
}

func verifySealedInputs(workspace SealedPersonaWorkspace) (verifiedSealedInputs, error) {
	verified := verifiedSealedInputs{}
	outgoingBytes, err := readRegularFile(workspace.OutgoingInputPath)
	if err != nil {
		return verified, err
	}
	outgoingSHA := panelhash.SHA256Hex(outgoingBytes)
	verified.OutgoingSHA256 = &outgoingSHA

	outgoingRecord, err := parseOutgoingInputRecord(outgoingBytes)
	if err != nil {
		return verified, err
	}
	verified.OutgoingRecord = &outgoingRecord
	if err := verifySealedWorkspaceContract(workspace, outgoingRecord); err != nil {
		return verified, err
	}

	agentsBytes, err := readPrelaunchVerifiedIsolatedFile(workspace.IsolatedRoot, workspace.WorkspaceAgentsPath)
	if err != nil {
		return verified, err
	}
	promptBytes, err := readPrelaunchVerifiedIsolatedFile(workspace.IsolatedRoot, workspace.PromptPath)
	if err != nil {
		return verified, err
	}
	skillBytes, err := readPrelaunchVerifiedIsolatedFile(workspace.IsolatedRoot, workspace.SkillPath)
	if err != nil {
		return verified, err
	}

	if panelhash.SHA256Hex(agentsBytes) != outgoingRecord.AgentInstructionsSHA256 {
		return verified, errors.New("isolated AGENTS.md hash mismatch")
	}
	if panelhash.SHA256Hex(promptBytes) != outgoingRecord.PromptSHA256 {
		return verified, errors.New("isolated prompt hash mismatch")
	}
	if panelhash.SHA256Hex(skillBytes) != outgoingRecord.SkillSHA256 {
		return verified, errors.New("isolated skill hash mismatch")
	}
	if err := ensurePathHasNoSymlinkComponentsWithinRoot(workspace.IsolatedRoot, workspace.ExecutionEnv.CodexHomeDir); err != nil {
		return verified, err
	}
	codexRuntimeSHA, err := allowlistedCodexRuntimeSHA256(workspace.ExecutionEnv.CodexHomeDir)
	if err != nil {
		return verified, err
	}
	if strings.TrimSpace(outgoingRecord.CodexRuntimeSHA256) != "" && codexRuntimeSHA != outgoingRecord.CodexRuntimeSHA256 {
		return verified, errors.New("isolated codex runtime hash mismatch")
	}
	if lengthPrefixedSHA256Hex(agentsBytes, promptBytes, skillBytes) != outgoingRecord.CombinedInputSHA256 {
		return verified, errors.New("combined input hash mismatch")
	}

	verified.AgentsBytes = agentsBytes
	verified.PromptBytes = promptBytes
	verified.SkillBytes = skillBytes
	return verified, nil
}

func readPrelaunchVerifiedIsolatedFile(isolatedRoot, path string) ([]byte, error) {
	if err := ensurePathHasNoSymlinkComponentsWithinRoot(isolatedRoot, filepath.Dir(path)); err != nil {
		return nil, err
	}
	return readRegularFile(path)
}

func verifySealedWorkspaceContract(workspace SealedPersonaWorkspace, outgoingRecord outgoingInputRecord) error {
	if err := workspace.ExecutionEnv.ValidateForRoot(workspace.IsolatedRoot); err != nil {
		return err
	}
	if workspace.ExecutionEnv.CWD != outgoingRecord.ExecutionCWD ||
		workspace.ExecutionEnv.HomeDir != outgoingRecord.ExecutionHomeDir ||
		workspace.ExecutionEnv.CodexHomeDir != outgoingRecord.ExecutionCodexHomeDir {
		return errors.New("sealed execution environment record mismatch")
	}

	expectedRunPersonaDir := storage.AnswerPersonaDir(workspace.RunRoot, workspace.PersonaID)
	if !sameCleanPath(workspace.RunPersonaDir, expectedRunPersonaDir) {
		return errors.New("sealed run persona directory mismatch")
	}

	expectedOutgoingInputPath, err := storage.AnswerPersonaArtifactPath(workspace.RunRoot, workspace.PersonaID, outgoingInputArtifactName)
	if err != nil {
		return fmt.Errorf("resolve expected outgoing_input.json path: %w", err)
	}
	if !sameCleanPath(workspace.OutgoingInputPath, expectedOutgoingInputPath) {
		return errors.New("sealed outgoing_input path mismatch")
	}

	expectedAgentsPath := filepath.Join(workspace.IsolatedRoot, "workspace", "AGENTS.md")
	if !sameCleanPath(workspace.WorkspaceAgentsPath, expectedAgentsPath) {
		return errors.New("sealed isolated AGENTS path mismatch")
	}

	expectedPromptPath := filepath.Join(workspace.IsolatedRoot, "input", "prompt.txt")
	if !sameCleanPath(workspace.PromptPath, expectedPromptPath) {
		return errors.New("sealed isolated prompt path mismatch")
	}

	expectedSkillPath := filepath.Join(workspace.IsolatedRoot, "skill", "SKILL.md")
	if !sameCleanPath(workspace.SkillPath, expectedSkillPath) {
		return errors.New("sealed isolated skill path mismatch")
	}

	return nil
}

func sameCleanPath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func extractAuthoritativeCompletedItem(result SingleTurnRunResult) *CompletedAgentItem {
	if result.CompletedItem != nil && result.CompletedItem.ItemType == authoritativeCompletedItemType {
		return result.CompletedItem
	}
	for i := len(result.Transcript) - 1; i >= 0; i-- {
		item := result.Transcript[i]
		if item.ItemType != authoritativeCompletedItemType {
			continue
		}
		return &CompletedAgentItem{
			ItemType: item.ItemType,
			ItemID:   item.ItemID,
			Text:     item.Text,
			Payload:  item.Payload,
		}
	}
	return nil
}

func collectObservedToolCalls(transcript []DiagnosticStreamItem) []observedToolCall {
	calls := make([]observedToolCall, 0)
	for _, item := range transcript {
		if strings.TrimSpace(item.ToolName) == "" {
			continue
		}
		calls = append(calls, observedToolCall{
			ToolName: item.ToolName,
			CallID:   stringPtrIfNotEmpty(item.CallID),
		})
	}
	return calls
}

func diagnosticRawOutput(transcript []DiagnosticStreamItem, authoritative *CompletedAgentItem) ([]byte, string) {
	if authoritative != nil {
		return []byte(authoritative.Text), resultRawModeAuthoritativeText
	}

	var builder strings.Builder
	for _, item := range transcript {
		if !looksLikeDiagnosticText(item) {
			continue
		}
		builder.WriteString(item.Text)
	}

	if builder.Len() == 0 {
		return []byte{}, resultRawModeEmpty
	}
	return []byte(builder.String()), resultRawModeDiagnosticText
}

func looksLikeDiagnosticText(item DiagnosticStreamItem) bool {
	if item.ItemType == authoritativeCompletedItemType || strings.TrimSpace(item.Text) == "" {
		return false
	}
	lowerType := strings.ToLower(strings.TrimSpace(item.ItemType))
	return lowerType == "" ||
		strings.Contains(lowerType, "assistant") ||
		strings.Contains(lowerType, "message") ||
		strings.Contains(lowerType, "delta") ||
		strings.Contains(lowerType, "text")
}

func completedItemPayload(item *CompletedAgentItem) any {
	if item == nil {
		return nil
	}
	if item.Payload != nil {
		return item.Payload
	}
	return map[string]any{
		"item_type": item.ItemType,
		"item_id":   item.ItemID,
		"text":      item.Text,
	}
}

func normalizeTerminalOutcome(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case statusOutcomeCompleted:
		return statusOutcomeCompleted
	case statusOutcomeInterrupted:
		return statusOutcomeInterrupted
	case statusOutcomeFailed:
		return statusOutcomeFailed
	default:
		return ""
	}
}

func outgoingFieldPtr(record *outgoingInputRecord, selector func(*outgoingInputRecord) string) *string {
	if record == nil {
		return nil
	}
	return stringPtr(selector(record))
}

func nullableArtifactPath(present bool, artifact string) *string {
	if !present {
		return nil
	}
	return stringPtr(artifact)
}

func stringPtr(value string) *string {
	copyValue := value
	return &copyValue
}

func stringPtrIfNotEmpty(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return stringPtr(value)
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func parseWorkerStatusArtifact(data []byte) (*workerStatusArtifact, error) {
	var artifact workerStatusArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, err
	}
	if artifact.SchemaVersion != workerStatusSchemaV1 || artifact.Stage != answerStage {
		return nil, errors.New("worker status schema mismatch")
	}
	if artifact.SourceOutgoingInputPath != outgoingInputPathLiteral {
		return nil, errors.New("worker status outgoing_input path mismatch")
	}
	switch artifact.Outcome {
	case statusOutcomeCompleted, statusOutcomeInterrupted, statusOutcomeFailed:
	default:
		return nil, fmt.Errorf("invalid worker status outcome %q", artifact.Outcome)
	}
	if !artifact.AttestationPresent {
		return nil, errors.New("worker status attestation_present must be true")
	}
	if artifact.AttestationPath != attestationArtifactName {
		return nil, errors.New("worker status attestation_path mismatch")
	}
	if !artifact.AuthoritativeOutputPresent {
		if artifact.AuthoritativeItemType != nil || artifact.AuthoritativeItemID != nil {
			return nil, errors.New("worker status authoritative sentinels mismatch")
		}
	}
	if artifact.AuthoritativeOutputPresent {
		if artifact.AuthoritativeItemType == nil || *artifact.AuthoritativeItemType != authoritativeCompletedItemType {
			return nil, errors.New("worker status authoritative_item_type mismatch")
		}
		if artifact.AuthoritativeItemID == nil || strings.TrimSpace(*artifact.AuthoritativeItemID) == "" {
			return nil, errors.New("worker status missing authoritative_item_id")
		}
	}
	if artifact.ResultRawPresent != (artifact.ResultRawPath != nil) {
		return nil, errors.New("worker status result_raw path mismatch")
	}
	if artifact.ResultJSONPresent != (artifact.ResultJSONPath != nil) {
		return nil, errors.New("worker status result_json path mismatch")
	}
	return &artifact, nil
}

func parseWorkerAttestationArtifact(data []byte) (*workerAttestationArtifact, error) {
	var artifact workerAttestationArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, err
	}
	if artifact.SchemaVersion != workerAttestationSchemaV1 || artifact.Stage != answerStage {
		return nil, errors.New("worker attestation schema mismatch")
	}
	if artifact.SourceOutgoingInputPath != outgoingInputPathLiteral {
		return nil, errors.New("worker attestation outgoing_input path mismatch")
	}
	if !artifact.AuthoritativeOutputPresent {
		if artifact.AuthoritativeItemType != nil || artifact.AuthoritativeItemID != nil || artifact.AuthoritativeTextSHA256 != nil {
			return nil, errors.New("worker attestation authoritative sentinels mismatch")
		}
	}
	if artifact.AuthoritativeOutputPresent {
		if artifact.AuthoritativeItemType == nil || *artifact.AuthoritativeItemType != authoritativeCompletedItemType {
			return nil, errors.New("worker attestation authoritative_item_type mismatch")
		}
		if artifact.AuthoritativeItemID == nil || artifact.AuthoritativeTextSHA256 == nil {
			return nil, errors.New("worker attestation missing authoritative output fields")
		}
	}
	if artifact.ResultRawTXTSHA256 == nil && artifact.ResultRawMode != nil {
		return nil, errors.New("worker attestation result_raw_mode must be null when result_raw_txt_sha256 is null")
	}
	if artifact.ResultRawMode != nil {
		switch *artifact.ResultRawMode {
		case resultRawModeAuthoritativeText, resultRawModeDiagnosticText, resultRawModeEmpty:
		default:
			return nil, fmt.Errorf("invalid result_raw_mode %q", *artifact.ResultRawMode)
		}
	}
	if artifact.DiagnosticStreamItemCount < 0 {
		return nil, errors.New("diagnostic_stream_item_count must be non-negative")
	}
	return &artifact, nil
}

func parseWorkerResultArtifact(data []byte) (*workerResultArtifact, error) {
	var artifact workerResultArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, err
	}
	if artifact.SchemaVersion != workerResultSchemaV1 || artifact.Stage != answerStage {
		return nil, errors.New("worker result schema mismatch")
	}
	if artifact.SourceOutgoingInputPath != outgoingInputPathLiteral {
		return nil, errors.New("worker result outgoing_input path mismatch")
	}
	if artifact.AuthoritativeItemType != authoritativeCompletedItemType {
		return nil, errors.New("worker result authoritative_item_type mismatch")
	}
	if strings.TrimSpace(artifact.AuthoritativeItemID) == "" {
		return nil, errors.New("worker result missing authoritative_item_id")
	}
	if panelhash.SHA256Hex([]byte(artifact.Text)) != artifact.TextSHA256 {
		return nil, errors.New("worker result text_sha256 mismatch")
	}
	if artifact.AuthoritativeItem == nil {
		return nil, errors.New("worker result missing authoritative_item payload")
	}
	return &artifact, nil
}
