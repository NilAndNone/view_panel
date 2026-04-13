package answer

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	panelhash "view_panel/internal/hash"
	"view_panel/internal/storage"
)

const (
	answerStage                     = "02_answer"
	answerOutgoingInputSchemaV1     = "answer_outgoing_input_v1"
	answerOutgoingInputSchemaV2     = "answer_outgoing_input_v2"
	workerResultSchemaV1            = "worldview_worker_result_v1"
	workerAttestationSchemaV1       = "worldview_worker_attestation_v1"
	workerStatusSchemaV1            = "worldview_worker_status_v1"
	answerBatchSchemaV1             = "worldview_answer_batch_v1"
	authoritativeCompletedItemType  = "item/completed.agentMessage"
	prelaunchVerificationPhase      = "prelaunch_verification"
	rejectedBeforeLaunchOutcome     = "rejected_before_launch"
	batchPreDispatchPhase           = "batch_pre_dispatch"
	notInvokedSentinel              = "not_invoked"
	resultRawModeAuthoritativeText  = "authoritative_text"
	resultRawModeDiagnosticText     = "diagnostic_text"
	resultRawModeEmpty              = "empty"
	prepareGateStatusArtifactName   = "prepare_gate_status_v1.json"
	outgoingInputArtifactName       = "outgoing_input.json"
	resultRawArtifactName           = "result.raw.txt"
	resultJSONArtifactName          = "result.json"
	attestationArtifactName         = "attestation.json"
	statusArtifactName              = "status.json"
	answerBatchArtifactName         = "answer_batch.json"
	workspaceAgentsArtifactName     = "workspace/AGENTS.md"
	inputPromptArtifactName         = "input/prompt.txt"
	isolatedSkillArtifactName       = "skill/SKILL.md"
	outgoingInputPathLiteral        = "outgoing_input.json"
	statusOutcomeCompleted          = "completed"
	statusOutcomeInterrupted        = "interrupted"
	statusOutcomeFailed             = "failed"
	batchOutcomeCertified           = "certified"
	batchOutcomeRejected            = "rejected"
	batchOutcomeFailed              = "failed"
	rejectionReasonForbiddenTool    = "forbidden_tool"
	failureReasonWorkerTimeout      = "worker_timeout"
	failureReasonBatchTimeout       = "batch_timeout"
	failureReasonBatchCancelledRun  = "batch_cancelled_in_flight"
	failureReasonBatchCancelledWait = "batch_cancelled_before_start"
	failureReasonLaunchError        = "launch_error"
	failureReasonMissingArtifact    = "missing_worker_artifact"
	failureReasonInvalidArtifact    = "invalid_worker_artifact"
	failureReasonWorkerFailed       = "worker_failed"
	failureReasonWorkerInterrupted  = "worker_interrupted"
	failureReasonAuthoritativeMiss  = "authoritative_output_missing"
	failureReasonResultJSONMissing  = "result_json_missing"
	failureReasonResultArtifactMiss = "result_json_artifact_missing"
)

var inheritedCodexRuntimeFiles = [...]string{"config.toml", "auth.json"}

type SealWorkspacesRequest struct {
	RepoRoot        string
	RunRoot         string
	SkillSourcePath string
	IsolatedBaseDir string
}

type SealWorkspacesResult struct {
	Blocked    bool
	PersonaIDs []string
	Workspaces []SealedPersonaWorkspace
}

type SealedPersonaWorkspace struct {
	PersonaID           string             `json:"persona_id"`
	RunRoot             string             `json:"run_root"`
	RunPersonaDir       string             `json:"run_persona_dir"`
	OutgoingInputPath   string             `json:"outgoing_input_path"`
	IsolatedRoot        string             `json:"isolated_root"`
	ExecutionEnv        SealedExecutionEnv `json:"execution_env"`
	WorkspaceAgentsPath string             `json:"workspace_agents_path"`
	PromptPath          string             `json:"prompt_path"`
	SkillPath           string             `json:"skill_path"`
}

type prepareGateStatus struct {
	CanProceedToStage2 bool     `json:"can_proceed_to_stage2"`
	PersonaIDs         []string `json:"persona_ids"`
}

type prepareHashLedger struct {
	DispatchInputSHA256 string `json:"dispatch_input_sha256"`
	AgentsSHA256        string `json:"agents_sha256"`
	PromptSHA256        string `json:"prompt_sha256"`
	BundleSHA256        string `json:"bundle_sha256"`
}

type outgoingInputRecord struct {
	SchemaVersion             string `json:"schema_version"`
	Stage                     string `json:"stage"`
	PersonaID                 string `json:"persona_id"`
	ExecutionCWD              string `json:"execution_cwd"`
	ExecutionHomeDir          string `json:"execution_home_dir"`
	ExecutionCodexHomeDir     string `json:"execution_codex_home_dir"`
	AgentInstructionsPath     string `json:"agent_instructions_path"`
	AgentInstructionsSHA256   string `json:"agent_instructions_sha256"`
	PromptPath                string `json:"prompt_path"`
	PromptSHA256              string `json:"prompt_sha256"`
	SkillPath                 string `json:"skill_path"`
	SkillSHA256               string `json:"skill_sha256"`
	CodexRuntimeSHA256        string `json:"codex_runtime_sha256"`
	SourceDispatchInputSHA256 string `json:"source_dispatch_input_sha256"`
	CombinedInputSHA256       string `json:"combined_input_sha256"`
}

func SealWorkspaces(req SealWorkspacesRequest) (SealWorkspacesResult, error) {
	repoRoot, err := resolvePathLoose(req.RepoRoot)
	if err != nil {
		return SealWorkspacesResult{}, fmt.Errorf("resolve repo root: %w", err)
	}

	runRoot, err := resolvePathLoose(req.RunRoot)
	if err != nil {
		return SealWorkspacesResult{}, fmt.Errorf("resolve run root: %w", err)
	}

	gatePath, err := storage.ResolvePath(storage.PrepareRoot(runRoot), prepareGateStatusArtifactName)
	if err != nil {
		return SealWorkspacesResult{}, fmt.Errorf("resolve gate artifact path: %w", err)
	}
	gateBytes, err := readRegularFile(gatePath)
	if err != nil {
		return SealWorkspacesResult{}, fmt.Errorf("read gate artifact: %w", err)
	}

	gate, err := parsePrepareGateStatus(gateBytes)
	if err != nil {
		return SealWorkspacesResult{}, fmt.Errorf("parse gate artifact: %w", err)
	}

	result := SealWorkspacesResult{
		Blocked:    !gate.CanProceedToStage2,
		PersonaIDs: append([]string(nil), gate.PersonaIDs...),
		Workspaces: make([]SealedPersonaWorkspace, 0, len(gate.PersonaIDs)),
	}
	if result.Blocked {
		return result, nil
	}

	skillSourcePath, skillRepoRelativePath, err := resolveFileWithinRoot(repoRoot, req.SkillSourcePath)
	if err != nil {
		return SealWorkspacesResult{}, fmt.Errorf("resolve answer skill: %w", err)
	}

	skillBytes, err := readRegularFile(skillSourcePath)
	if err != nil {
		return SealWorkspacesResult{}, fmt.Errorf("read answer skill: %w", err)
	}
	skillSHA := panelhash.SHA256Hex(skillBytes)

	isolatedBaseDir, err := resolvePathLoose(req.IsolatedBaseDir)
	if err != nil {
		return SealWorkspacesResult{}, fmt.Errorf("resolve isolated base dir: %w", err)
	}

	homeDir, err := currentUserHomeDir()
	if err != nil {
		return SealWorkspacesResult{}, fmt.Errorf("resolve current user home: %w", err)
	}
	parentCodexHomeDir, err := currentUserCodexHomeDir(homeDir)
	if err != nil {
		return SealWorkspacesResult{}, fmt.Errorf("resolve current user codex home: %w", err)
	}
	if err := ensureOutsideRestrictedTrees(isolatedBaseDir, repoRoot, runRoot, homeDir, parentCodexHomeDir); err != nil {
		return SealWorkspacesResult{}, fmt.Errorf("validate isolated base dir: %w", err)
	}

	runID, err := runIDFromRunRoot(runRoot)
	if err != nil {
		return SealWorkspacesResult{}, err
	}
	isolatedRunRoot, err := resolvePathLoose(filepath.Join(isolatedBaseDir, runID))
	if err != nil {
		return SealWorkspacesResult{}, fmt.Errorf("resolve isolated run root: %w", err)
	}

	for _, personaID := range gate.PersonaIDs {
		if err := validatePersonaIDForIsolation(personaID); err != nil {
			return SealWorkspacesResult{}, err
		}

		agentsPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "agents.md")
		if err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("resolve agents path for %s: %w", personaID, err)
		}
		promptPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "prompt.txt")
		if err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("resolve prompt path for %s: %w", personaID, err)
		}
		hashesPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "hashes.json")
		if err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("resolve hashes path for %s: %w", personaID, err)
		}

		agentsBytes, err := readRegularFile(agentsPath)
		if err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("read agents for %s: %w", personaID, err)
		}
		promptBytes, err := readRegularFile(promptPath)
		if err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("read prompt for %s: %w", personaID, err)
		}
		hashesBytes, err := readRegularFile(hashesPath)
		if err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("read hashes for %s: %w", personaID, err)
		}

		hashLedger, err := parsePrepareHashLedger(hashesBytes)
		if err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("parse hashes for %s: %w", personaID, err)
		}

		agentsSHA := panelhash.SHA256Hex(agentsBytes)
		if agentsSHA != hashLedger.AgentsSHA256 {
			return SealWorkspacesResult{}, fmt.Errorf("agents hash mismatch for %s", personaID)
		}
		promptSHA := panelhash.SHA256Hex(promptBytes)
		if promptSHA != hashLedger.PromptSHA256 {
			return SealWorkspacesResult{}, fmt.Errorf("prompt hash mismatch for %s", personaID)
		}

		answerPersonaDir := storage.AnswerPersonaDir(runRoot, personaID)
		snapshotAgentsPath, err := storage.AnswerPersonaArtifactPath(runRoot, personaID, workspaceAgentsArtifactName)
		if err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("resolve stage2 agents snapshot path for %s: %w", personaID, err)
		}
		snapshotPromptPath, err := storage.AnswerPersonaArtifactPath(runRoot, personaID, inputPromptArtifactName)
		if err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("resolve stage2 prompt snapshot path for %s: %w", personaID, err)
		}
		outgoingInputPath, err := storage.AnswerPersonaArtifactPath(runRoot, personaID, outgoingInputArtifactName)
		if err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("resolve outgoing_input path for %s: %w", personaID, err)
		}

		if err := storage.WriteText(snapshotAgentsPath, string(agentsBytes)); err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("write stage2 agents snapshot for %s: %w", personaID, err)
		}
		if err := storage.WriteText(snapshotPromptPath, string(promptBytes)); err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("write stage2 prompt snapshot for %s: %w", personaID, err)
		}
		agentsSHA, err = panelhash.SHA256HexFile(snapshotAgentsPath)
		if err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("hash stage2 agents snapshot for %s: %w", personaID, err)
		}
		if agentsSHA != hashLedger.AgentsSHA256 {
			return SealWorkspacesResult{}, fmt.Errorf("stage2 agents snapshot hash mismatch for %s", personaID)
		}
		promptSHA, err = panelhash.SHA256HexFile(snapshotPromptPath)
		if err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("hash stage2 prompt snapshot for %s: %w", personaID, err)
		}
		if promptSHA != hashLedger.PromptSHA256 {
			return SealWorkspacesResult{}, fmt.Errorf("stage2 prompt snapshot hash mismatch for %s", personaID)
		}

		isolatedRoot, err := resolvePathLoose(filepath.Join(isolatedRunRoot, personaID))
		if err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("resolve isolated root for %s: %w", personaID, err)
		}
		if !pathWithinRoot(isolatedRunRoot, isolatedRoot) {
			return SealWorkspacesResult{}, fmt.Errorf("isolated root for %s escapes %q", personaID, isolatedRunRoot)
		}
		if err := ensureOutsideRestrictedTrees(isolatedRoot, repoRoot, runRoot, homeDir, parentCodexHomeDir); err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("validate isolated root for %s: %w", personaID, err)
		}
		if err := os.RemoveAll(isolatedRoot); err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("clear isolated root for %s: %w", personaID, err)
		}

		isolatedWorkspaceDir := filepath.Join(isolatedRoot, "workspace")
		isolatedInputDir := filepath.Join(isolatedRoot, "input")
		isolatedSkillDir := filepath.Join(isolatedRoot, "skill")
		isolatedHomeDir := filepath.Join(isolatedRoot, "home")
		isolatedCodexHomeDir := filepath.Join(isolatedHomeDir, ".codex")
		for _, dirPath := range []string{isolatedWorkspaceDir, isolatedInputDir, isolatedSkillDir, isolatedHomeDir, isolatedCodexHomeDir} {
			if err := os.MkdirAll(dirPath, 0o755); err != nil {
				return SealWorkspacesResult{}, fmt.Errorf("create isolated directory %s for %s: %w", dirPath, personaID, err)
			}
		}
		if err := copyAllowlistedCodexRuntimeFiles(parentCodexHomeDir, isolatedCodexHomeDir); err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("copy isolated codex runtime for %s: %w", personaID, err)
		}
		codexRuntimeSHA, err := allowlistedCodexRuntimeSHA256(isolatedCodexHomeDir)
		if err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("hash isolated codex runtime for %s: %w", personaID, err)
		}

		isolatedAgentsPath := filepath.Join(isolatedWorkspaceDir, "AGENTS.md")
		isolatedPromptPath := filepath.Join(isolatedInputDir, "prompt.txt")
		isolatedSkillPath := filepath.Join(isolatedSkillDir, "SKILL.md")
		if err := writeFileAtomic(isolatedAgentsPath, agentsBytes); err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("write isolated agents for %s: %w", personaID, err)
		}
		if err := writeFileAtomic(isolatedPromptPath, promptBytes); err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("write isolated prompt for %s: %w", personaID, err)
		}
		if err := writeFileAtomic(isolatedSkillPath, skillBytes); err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("write isolated skill for %s: %w", personaID, err)
		}

		combinedSHA := lengthPrefixedSHA256Hex(agentsBytes, promptBytes, skillBytes)
		executionEnv := sealedExecutionEnvForRoot(isolatedRoot)
		outgoing := outgoingInputRecord{
			SchemaVersion:             answerOutgoingInputSchemaV2,
			Stage:                     answerStage,
			PersonaID:                 personaID,
			ExecutionCWD:              executionEnv.CWD,
			ExecutionHomeDir:          executionEnv.HomeDir,
			ExecutionCodexHomeDir:     executionEnv.CodexHomeDir,
			AgentInstructionsPath:     workspaceAgentsArtifactName,
			AgentInstructionsSHA256:   agentsSHA,
			PromptPath:                inputPromptArtifactName,
			PromptSHA256:              promptSHA,
			SkillPath:                 skillRepoRelativePath,
			SkillSHA256:               skillSHA,
			CodexRuntimeSHA256:        codexRuntimeSHA,
			SourceDispatchInputSHA256: hashLedger.DispatchInputSHA256,
			CombinedInputSHA256:       combinedSHA,
		}

		if err := storage.WriteJSON(outgoingInputPath, outgoing); err != nil {
			return SealWorkspacesResult{}, fmt.Errorf("write outgoing_input.json for %s: %w", personaID, err)
		}

		result.Workspaces = append(result.Workspaces, SealedPersonaWorkspace{
			PersonaID:           personaID,
			RunRoot:             runRoot,
			RunPersonaDir:       answerPersonaDir,
			OutgoingInputPath:   outgoingInputPath,
			IsolatedRoot:        isolatedRoot,
			ExecutionEnv:        executionEnv,
			WorkspaceAgentsPath: isolatedAgentsPath,
			PromptPath:          isolatedPromptPath,
			SkillPath:           isolatedSkillPath,
		})
	}

	return result, nil
}

func parsePrepareGateStatus(data []byte) (prepareGateStatus, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return prepareGateStatus{}, err
	}

	canProceedRaw, ok := raw["can_proceed_to_stage2"]
	if !ok {
		return prepareGateStatus{}, errors.New("missing can_proceed_to_stage2")
	}
	personaIDsRaw, ok := raw["persona_ids"]
	if !ok {
		return prepareGateStatus{}, errors.New("missing persona_ids")
	}

	var canProceed bool
	if err := json.Unmarshal(canProceedRaw, &canProceed); err != nil {
		return prepareGateStatus{}, fmt.Errorf("decode can_proceed_to_stage2: %w", err)
	}

	var personaIDs []string
	if err := json.Unmarshal(personaIDsRaw, &personaIDs); err != nil {
		return prepareGateStatus{}, fmt.Errorf("decode persona_ids: %w", err)
	}

	seen := make(map[string]struct{}, len(personaIDs))
	for _, personaID := range personaIDs {
		if strings.TrimSpace(personaID) == "" {
			return prepareGateStatus{}, errors.New("blank persona_id in gate artifact")
		}
		if err := validatePersonaIDForIsolation(personaID); err != nil {
			return prepareGateStatus{}, err
		}
		if _, ok := seen[personaID]; ok {
			return prepareGateStatus{}, fmt.Errorf("duplicate persona_id %q in gate artifact", personaID)
		}
		seen[personaID] = struct{}{}
	}

	return prepareGateStatus{
		CanProceedToStage2: canProceed,
		PersonaIDs:         personaIDs,
	}, nil
}

func parsePrepareHashLedger(data []byte) (prepareHashLedger, error) {
	expectedKeys := map[string]struct{}{
		"dispatch_input_sha256": {},
		"agents_sha256":         {},
		"prompt_sha256":         {},
		"bundle_sha256":         {},
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return prepareHashLedger{}, err
	}
	if len(raw) != len(expectedKeys) {
		return prepareHashLedger{}, errors.New("hash ledger key set mismatch")
	}
	for key := range raw {
		if _, ok := expectedKeys[key]; !ok {
			return prepareHashLedger{}, fmt.Errorf("unexpected hash ledger key %q", key)
		}
	}

	var ledger prepareHashLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		return prepareHashLedger{}, err
	}
	if ledger.DispatchInputSHA256 == "" || ledger.AgentsSHA256 == "" || ledger.PromptSHA256 == "" || ledger.BundleSHA256 == "" {
		return prepareHashLedger{}, errors.New("hash ledger contains empty value")
	}

	return ledger, nil
}

func parseOutgoingInputRecord(data []byte) (outgoingInputRecord, error) {
	var record outgoingInputRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return outgoingInputRecord{}, err
	}
	switch record.SchemaVersion {
	case answerOutgoingInputSchemaV1, answerOutgoingInputSchemaV2:
	default:
		return outgoingInputRecord{}, fmt.Errorf("unexpected schema_version %q", record.SchemaVersion)
	}
	if record.Stage != answerStage {
		return outgoingInputRecord{}, fmt.Errorf("unexpected stage %q", record.Stage)
	}
	if strings.TrimSpace(record.ExecutionCWD) == "" {
		return outgoingInputRecord{}, errors.New("missing execution_cwd")
	}
	if strings.TrimSpace(record.ExecutionHomeDir) == "" {
		return outgoingInputRecord{}, errors.New("missing execution_home_dir")
	}
	if strings.TrimSpace(record.ExecutionCodexHomeDir) == "" {
		return outgoingInputRecord{}, errors.New("missing execution_codex_home_dir")
	}
	if record.AgentInstructionsPath != workspaceAgentsArtifactName {
		return outgoingInputRecord{}, fmt.Errorf("unexpected agent_instructions_path %q", record.AgentInstructionsPath)
	}
	if record.PromptPath != inputPromptArtifactName {
		return outgoingInputRecord{}, fmt.Errorf("unexpected prompt_path %q", record.PromptPath)
	}
	if strings.TrimSpace(record.PersonaID) == "" {
		return outgoingInputRecord{}, errors.New("missing persona_id")
	}
	if strings.TrimSpace(record.SkillPath) == "" {
		return outgoingInputRecord{}, errors.New("missing skill_path")
	}
	if strings.TrimSpace(record.AgentInstructionsSHA256) == "" ||
		strings.TrimSpace(record.PromptSHA256) == "" ||
		strings.TrimSpace(record.SkillSHA256) == "" ||
		strings.TrimSpace(record.SourceDispatchInputSHA256) == "" ||
		strings.TrimSpace(record.CombinedInputSHA256) == "" {
		return outgoingInputRecord{}, errors.New("missing seal-chain hash")
	}
	if record.SchemaVersion == answerOutgoingInputSchemaV2 && strings.TrimSpace(record.CodexRuntimeSHA256) == "" {
		return outgoingInputRecord{}, errors.New("missing seal-chain hash")
	}
	return record, nil
}

func runIDFromRunRoot(runRoot string) (string, error) {
	runID := filepath.Base(filepath.Clean(runRoot))
	if runID == "" || runID == "." || runID == string(filepath.Separator) {
		return "", fmt.Errorf("unable to derive run_id from %q", runRoot)
	}
	return runID, nil
}

func resolveFileWithinRoot(root, candidate string) (string, string, error) {
	if strings.TrimSpace(candidate) == "" {
		return "", "", errors.New("empty file path")
	}

	joined := candidate
	if !filepath.IsAbs(joined) {
		joined = filepath.Join(root, candidate)
	}

	resolved, err := resolvePathLoose(joined)
	if err != nil {
		return "", "", err
	}
	if !pathWithinRoot(root, resolved) {
		return "", "", fmt.Errorf("%q resolves outside %q", candidate, root)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("%q is not a regular file", resolved)
	}

	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", "", fmt.Errorf("%q is outside %q", resolved, root)
	}

	return resolved, filepath.ToSlash(rel), nil
}

func ensureOutsideRestrictedTrees(candidate string, restrictedRoots ...string) error {
	for _, restricted := range restrictedRoots {
		if restricted == "" {
			continue
		}
		if pathWithinRoot(restricted, candidate) {
			return fmt.Errorf("%q resolves under restricted tree %q", candidate, restricted)
		}
	}
	return nil
}

func pathWithinRoot(root string, candidate string) bool {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func resolvePathLoose(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("empty path")
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absolutePath = filepath.Clean(absolutePath)

	current := absolutePath
	suffix := make([]string, 0, 4)
	for {
		resolvedCurrent, err := filepath.EvalSymlinks(current)
		if err == nil {
			resolved := resolvedCurrent
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return absolutePath, nil
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func currentUserHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		return resolvePathLoose(home)
	}
	if envHome := strings.TrimSpace(os.Getenv("HOME")); envHome != "" {
		return resolvePathLoose(envHome)
	}
	return "", errors.New("unable to determine current user home")
}

func currentUserCodexHomeDir(homeDir string) (string, error) {
	if codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME")); codexHome != "" {
		return resolvePathLoose(codexHome)
	}
	if strings.TrimSpace(homeDir) == "" {
		return "", errors.New("empty current user home")
	}
	return resolvePathLoose(filepath.Join(homeDir, ".codex"))
}

func copyAllowlistedCodexRuntimeFiles(parentCodexHomeDir string, isolatedCodexHomeDir string) error {
	for _, fileName := range inheritedCodexRuntimeFiles {
		sourcePath := filepath.Join(parentCodexHomeDir, fileName)
		sourceBytes, err := readRegularFile(sourcePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("read %q: %w", sourcePath, err)
		}

		targetPath := filepath.Join(isolatedCodexHomeDir, fileName)
		if err := writeFileAtomicMode(targetPath, sourceBytes, isolatedCodexRuntimeFileMode(fileName)); err != nil {
			return fmt.Errorf("write %q: %w", targetPath, err)
		}
	}
	return nil
}

func validatePersonaIDForIsolation(personaID string) error {
	switch {
	case filepath.IsAbs(personaID):
		return fmt.Errorf("invalid persona_id %q in gate artifact", personaID)
	case personaID == "." || personaID == "..":
		return fmt.Errorf("invalid persona_id %q in gate artifact", personaID)
	case filepath.Base(personaID) != personaID:
		return fmt.Errorf("invalid persona_id %q in gate artifact", personaID)
	case strings.Contains(personaID, "/"), strings.Contains(personaID, "\\"):
		return fmt.Errorf("invalid persona_id %q in gate artifact", personaID)
	default:
		return nil
	}
}

func allowlistedCodexRuntimeSHA256(codexHomeDir string) (string, error) {
	allowedEntries := make(map[string]struct{}, len(inheritedCodexRuntimeFiles))
	for _, fileName := range inheritedCodexRuntimeFiles {
		allowedEntries[fileName] = struct{}{}
	}

	entries, err := os.ReadDir(codexHomeDir)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", codexHomeDir, err)
	}
	for _, entry := range entries {
		if _, ok := allowedEntries[entry.Name()]; !ok {
			return "", fmt.Errorf("unexpected entry in %q: %q", codexHomeDir, entry.Name())
		}
	}

	parts := make([][]byte, 0, len(inheritedCodexRuntimeFiles)*3)
	for _, fileName := range inheritedCodexRuntimeFiles {
		parts = append(parts, []byte(fileName))

		filePath := filepath.Join(codexHomeDir, fileName)
		fileBytes, err := readRegularFile(filePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				parts = append(parts, []byte{0})
				continue
			}
			return "", fmt.Errorf("read %q: %w", filePath, err)
		}

		parts = append(parts, []byte{1}, fileBytes)
	}
	return lengthPrefixedSHA256Hex(parts...), nil
}

func isolatedCodexRuntimeFileMode(fileName string) os.FileMode {
	if fileName == "auth.json" {
		return 0o600
	}
	return 0o644
}

func readRegularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%q is not a regular file", path)
	}
	return os.ReadFile(path)
}

func ensurePathHasNoSymlinkComponentsWithinRoot(root string, path string) error {
	if strings.TrimSpace(root) == "" {
		return errors.New("empty root")
	}
	if strings.TrimSpace(path) == "" {
		return errors.New("empty path")
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	rootAbs = filepath.Clean(rootAbs)

	current, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	current = filepath.Clean(current)
	if !pathWithinRoot(rootAbs, current) {
		return fmt.Errorf("%q is outside root %q", path, root)
	}

	for {
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%q traverses symlink %q", path, current)
		}
		if sameCleanPath(current, rootAbs) {
			return nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return fmt.Errorf("%q is outside root %q", path, root)
		}
		current = parent
	}
}

func writeFileAtomic(path string, data []byte) error {
	return writeFileAtomicMode(path, data, 0o644)
}

func writeFileAtomicMode(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Chmod(perm); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	success = true
	return nil
}

func lengthPrefixedSHA256Hex(parts ...[]byte) string {
	buffer := bytes.NewBuffer(nil)
	lengthBuffer := make([]byte, 8)
	for _, part := range parts {
		binary.BigEndian.PutUint64(lengthBuffer, uint64(len(part)))
		buffer.Write(lengthBuffer)
		buffer.Write(part)
	}
	return panelhash.SHA256Hex(buffer.Bytes())
}
