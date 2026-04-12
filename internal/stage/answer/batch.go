package answer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	panelhash "view_panel/internal/hash"
	"view_panel/internal/storage"
)

type SingleWorkerExecutor interface {
	Execute(ctx context.Context, req ExecuteWorkerRequest) (ExecuteWorkerResult, error)
}

type BatchRunner struct {
	Runner SingleWorkerExecutor
}

type AnswerBatchRequest struct {
	RunID                 string
	AnswerRoot            string
	Workspaces            []SealedPersonaWorkspace
	MaxConcurrency        int
	WorkerTimeoutMS       int
	MaxAttemptsPerPersona int
	ForbiddenToolNames    []string
	ExtraArgs             []string
}

type AnswerBatchResult struct {
	ArtifactPath string
	Certified    []certifiedBatchEntry
	Failed       []failedBatchEntry
	Rejected     []rejectedBatchEntry
}

type answerBatchArtifact struct {
	SchemaVersion        string               `json:"schema_version"`
	Stage                string               `json:"stage"`
	RunID                string               `json:"run_id"`
	MaxConcurrency       int                  `json:"max_concurrency"`
	WorkerTimeoutMS      int                  `json:"worker_timeout_ms"`
	MaxAttemptsPerPersona int                 `json:"max_attempts_per_persona"`
	ForbiddenToolNames   []string             `json:"forbidden_tool_names"`
	Counts               answerBatchCounts    `json:"counts"`
	Certified            []certifiedBatchEntry `json:"certified"`
	Failed               []failedBatchEntry    `json:"failed"`
	Rejected             []rejectedBatchEntry  `json:"rejected"`
}

type answerBatchCounts struct {
	TotalPersonas int `json:"total_personas"`
	Certified     int `json:"certified"`
	Failed        int `json:"failed"`
	Rejected      int `json:"rejected"`
}

type certifiedBatchEntry struct {
	PersonaID             string `json:"persona_id"`
	Outcome               string `json:"outcome"`
	AttemptCount          int    `json:"attempt_count"`
	StatusPath            string `json:"status_path"`
	AttestationPath       string `json:"attestation_path"`
	ResultJSONPath        string `json:"result_json_path"`
	ResultJSONSHA256      string `json:"result_json_sha256"`
	AuthoritativeTextSHA256 string `json:"authoritative_text_sha256"`
}

type failedBatchEntry struct {
	PersonaID            string  `json:"persona_id"`
	Outcome              string  `json:"outcome"`
	AttemptCount         int     `json:"attempt_count"`
	FailureReason        string  `json:"failure_reason"`
	StatusPath           *string `json:"status_path"`
	AttestationPath      *string `json:"attestation_path"`
	ResultJSONPath       *string `json:"result_json_path"`
	RunnerTerminalOutcome *string `json:"runner_terminal_outcome"`
	LastReachedPhase     *string `json:"last_reached_phase"`
	CloseOutcomeKind     *string `json:"close_outcome_kind"`
}

type rejectedBatchEntry struct {
	PersonaID               string             `json:"persona_id"`
	Outcome                 string             `json:"outcome"`
	AttemptCount            int                `json:"attempt_count"`
	RejectionReason         string             `json:"rejection_reason"`
	StatusPath              string             `json:"status_path"`
	AttestationPath         string             `json:"attestation_path"`
	ResultJSONPath          string             `json:"result_json_path"`
	ResultJSONSHA256        string             `json:"result_json_sha256"`
	AuthoritativeTextSHA256 string             `json:"authoritative_text_sha256"`
	ForbiddenToolHits       []forbiddenToolHit `json:"forbidden_tool_hits"`
}

type forbiddenToolHit struct {
	ToolName           string  `json:"tool_name"`
	ToolNameNormalized string  `json:"tool_name_normalized"`
	CallID             *string `json:"call_id"`
}

type workerArtifactBundle struct {
	StatusPath       string
	AttestationPath  string
	ResultJSONPath   string
	Status           *workerStatusArtifact
	Attestation      *workerAttestationArtifact
	Result           *workerResultArtifact
	ResultJSONSHA256 *string
	StatusMissing    bool
	AttestationMissing bool
	ResultMissing    bool
	StatusInvalid    bool
	AttestationInvalid bool
	ResultInvalid    bool
	AnyArtifact      bool
}

type batchPersonaOutcome struct {
	Certified *certifiedBatchEntry
	Failed    *failedBatchEntry
	Rejected  *rejectedBatchEntry
}

func (b BatchRunner) Execute(ctx context.Context, req AnswerBatchRequest) (AnswerBatchResult, error) {
	if b.Runner == nil {
		return AnswerBatchResult{}, errors.New("single-worker runner is required")
	}
	if strings.TrimSpace(req.AnswerRoot) == "" {
		return AnswerBatchResult{}, errors.New("answer_root is required")
	}

	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		runID = filepath.Base(filepath.Dir(filepath.Clean(req.AnswerRoot)))
	}
	maxConcurrency := req.MaxConcurrency
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	maxAttempts := 1
	normalizedForbiddenToolNames := normalizeForbiddenToolNames(req.ForbiddenToolNames)

	outcomes := make([]batchPersonaOutcome, len(req.Workspaces))
	if len(req.Workspaces) == 0 {
		return b.persistAnswerBatch(req.AnswerRoot, answerBatchArtifact{
			SchemaVersion:         answerBatchSchemaV1,
			Stage:                 answerStage,
			RunID:                 runID,
			MaxConcurrency:        maxConcurrency,
			WorkerTimeoutMS:       req.WorkerTimeoutMS,
			MaxAttemptsPerPersona: maxAttempts,
			ForbiddenToolNames:    normalizedForbiddenToolNames,
			Counts:                answerBatchCounts{TotalPersonas: 0, Certified: 0, Failed: 0, Rejected: 0},
			Certified:             []certifiedBatchEntry{},
			Failed:                []failedBatchEntry{},
			Rejected:              []rejectedBatchEntry{},
		})
	}

	workerCount := maxConcurrency
	if workerCount > len(req.Workspaces) {
		workerCount = len(req.Workspaces)
	}

	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				outcomes[index] = b.executeOne(ctx, req, req.Workspaces[index], normalizedForbiddenToolNames)
			}
		}()
	}

	dispatchStopped := false
	for index := range req.Workspaces {
		if ctx.Err() != nil {
			dispatchStopped = true
			for remaining := index; remaining < len(req.Workspaces); remaining++ {
				outcomes[remaining] = batchPersonaOutcome{
					Failed: cancelledBeforeStartFailure(req.Workspaces[remaining].PersonaID),
				}
			}
			break
		}
		jobs <- index
	}
	close(jobs)
	wg.Wait()

	if !dispatchStopped {
		for index := range outcomes {
			if outcomes[index].Certified == nil && outcomes[index].Failed == nil && outcomes[index].Rejected == nil {
				outcomes[index] = batchPersonaOutcome{
					Failed: launchFailure(req.Workspaces[index].PersonaID),
				}
			}
		}
	}

	artifact := answerBatchArtifact{
		SchemaVersion:         answerBatchSchemaV1,
		Stage:                 answerStage,
		RunID:                 runID,
		MaxConcurrency:        maxConcurrency,
		WorkerTimeoutMS:       req.WorkerTimeoutMS,
		MaxAttemptsPerPersona: maxAttempts,
		ForbiddenToolNames:    normalizedForbiddenToolNames,
		Certified:             make([]certifiedBatchEntry, 0),
		Failed:                make([]failedBatchEntry, 0),
		Rejected:              make([]rejectedBatchEntry, 0),
	}
	for _, outcome := range outcomes {
		switch {
		case outcome.Certified != nil:
			artifact.Certified = append(artifact.Certified, *outcome.Certified)
		case outcome.Rejected != nil:
			artifact.Rejected = append(artifact.Rejected, *outcome.Rejected)
		case outcome.Failed != nil:
			artifact.Failed = append(artifact.Failed, *outcome.Failed)
		}
	}
	artifact.Counts = answerBatchCounts{
		TotalPersonas: len(req.Workspaces),
		Certified:     len(artifact.Certified),
		Failed:        len(artifact.Failed),
		Rejected:      len(artifact.Rejected),
	}

	return b.persistAnswerBatch(req.AnswerRoot, artifact)
}

func (b BatchRunner) executeOne(parent context.Context, req AnswerBatchRequest, workspace SealedPersonaWorkspace, forbiddenToolNames []string) batchPersonaOutcome {
	attemptCount := 1
	attemptCtx := parent
	cancel := func() {}
	if req.WorkerTimeoutMS > 0 {
		attemptCtx, cancel = context.WithTimeout(parent, time.Duration(req.WorkerTimeoutMS)*time.Millisecond)
	}
	defer cancel()

	execution, runErr := b.Runner.Execute(attemptCtx, ExecuteWorkerRequest{
		Workspace: workspace,
		ExtraArgs: req.ExtraArgs,
	})

	bundle := loadWorkerArtifactBundle(req.AnswerRoot, workspace.PersonaID)
	if parent.Err() != nil {
		reason := failureReasonBatchCancelledRun
		if errors.Is(parent.Err(), context.DeadlineExceeded) {
			reason = failureReasonBatchTimeout
		}
		return batchPersonaOutcome{
			Failed: failedEntryFromBundle(workspace.PersonaID, attemptCount, reason, bundle, false),
		}
	}
	if req.WorkerTimeoutMS > 0 && errors.Is(attemptCtx.Err(), context.DeadlineExceeded) {
		return batchPersonaOutcome{
			Failed: failedEntryFromBundle(workspace.PersonaID, attemptCount, failureReasonWorkerTimeout, bundle, false),
		}
	}

	if runErr != nil {
		return batchPersonaOutcome{
			Failed: failedEntryForRunnerError(workspace.PersonaID, attemptCount, bundle),
		}
	}

	if outcome := classifyStructuralOutcome(workspace.PersonaID, attemptCount, execution, bundle, forbiddenToolNames); outcome != nil {
		switch {
		case outcome.Certified != nil:
			return batchPersonaOutcome{Certified: outcome.Certified}
		case outcome.Rejected != nil:
			return batchPersonaOutcome{Rejected: outcome.Rejected}
		case outcome.Failed != nil:
			return batchPersonaOutcome{Failed: outcome.Failed}
		}
	}

	return batchPersonaOutcome{
		Failed: failedEntryFromBundle(workspace.PersonaID, attemptCount, failureReasonInvalidArtifact, bundle, true),
	}
}

func (b BatchRunner) persistAnswerBatch(answerRoot string, artifact answerBatchArtifact) (AnswerBatchResult, error) {
	artifactPath, err := storage.ResolvePath(answerRoot, answerBatchArtifactName)
	if err != nil {
		return AnswerBatchResult{}, fmt.Errorf("resolve answer_batch.json path: %w", err)
	}
	if err := storage.WriteJSON(artifactPath, artifact); err != nil {
		return AnswerBatchResult{}, fmt.Errorf("write answer_batch.json: %w", err)
	}
	return AnswerBatchResult{
		ArtifactPath: artifactPath,
		Certified:    artifact.Certified,
		Failed:       artifact.Failed,
		Rejected:     artifact.Rejected,
	}, nil
}

func classifyStructuralOutcome(personaID string, attemptCount int, execution ExecuteWorkerResult, bundle workerArtifactBundle, forbiddenToolNames []string) *batchPersonaOutcome {
	if bundle.StatusMissing || bundle.AttestationMissing {
		return &batchPersonaOutcome{
			Failed: failedEntryFromBundle(personaID, attemptCount, failureReasonMissingArtifact, bundle, false),
		}
	}
	if bundle.StatusInvalid || bundle.AttestationInvalid {
		return &batchPersonaOutcome{
			Failed: failedEntryFromBundle(personaID, attemptCount, failureReasonInvalidArtifact, bundle, true),
		}
	}
	if bundle.Status == nil || bundle.Attestation == nil {
		return &batchPersonaOutcome{
			Failed: failedEntryFromBundle(personaID, attemptCount, failureReasonInvalidArtifact, bundle, true),
		}
	}

	switch bundle.Status.Outcome {
	case statusOutcomeInterrupted:
		return &batchPersonaOutcome{
			Failed: failedEntryFromBundle(personaID, attemptCount, failureReasonWorkerInterrupted, bundle, false),
		}
	case statusOutcomeFailed:
		return &batchPersonaOutcome{
			Failed: failedEntryFromBundle(personaID, attemptCount, failureReasonWorkerFailed, bundle, false),
		}
	case statusOutcomeCompleted:
	default:
		return &batchPersonaOutcome{
			Failed: failedEntryFromBundle(personaID, attemptCount, failureReasonInvalidArtifact, bundle, true),
		}
	}

	if !bundle.Status.AuthoritativeOutputPresent {
		return &batchPersonaOutcome{
			Failed: failedEntryFromBundle(personaID, attemptCount, failureReasonAuthoritativeMiss, bundle, false),
		}
	}
	if !bundle.Status.ResultJSONPresent {
		return &batchPersonaOutcome{
			Failed: failedEntryFromBundle(personaID, attemptCount, failureReasonResultJSONMissing, bundle, false),
		}
	}
	if bundle.Status.ResultJSONPath == nil {
		return &batchPersonaOutcome{
			Failed: failedEntryFromBundle(personaID, attemptCount, failureReasonInvalidArtifact, bundle, true),
		}
	}

	if bundle.ResultMissing {
		return &batchPersonaOutcome{
			Failed: failedEntryFromBundle(personaID, attemptCount, failureReasonResultArtifactMiss, bundle, false),
		}
	}
	if bundle.ResultInvalid || bundle.Result == nil || bundle.ResultJSONSHA256 == nil {
		return &batchPersonaOutcome{
			Failed: failedEntryFromBundle(personaID, attemptCount, failureReasonInvalidArtifact, bundle, true),
		}
	}

	forbiddenToolHits := forbiddenToolHits(execution.ObservedToolCalls, bundle.Attestation.ObservedToolCalls, forbiddenToolNames)
	if len(forbiddenToolHits) > 0 {
		return &batchPersonaOutcome{
			Rejected: &rejectedBatchEntry{
				PersonaID:               personaID,
				Outcome:                 batchOutcomeRejected,
				AttemptCount:            attemptCount,
				RejectionReason:         rejectionReasonForbiddenTool,
				StatusPath:              bundle.StatusPath,
				AttestationPath:         bundle.AttestationPath,
				ResultJSONPath:          bundle.ResultJSONPath,
				ResultJSONSHA256:        derefString(bundle.ResultJSONSHA256),
				AuthoritativeTextSHA256: bundle.Result.TextSHA256,
				ForbiddenToolHits:       forbiddenToolHits,
			},
		}
	}

	return &batchPersonaOutcome{
		Certified: &certifiedBatchEntry{
			PersonaID:               personaID,
			Outcome:                 batchOutcomeCertified,
			AttemptCount:            attemptCount,
			StatusPath:              bundle.StatusPath,
			AttestationPath:         bundle.AttestationPath,
			ResultJSONPath:          bundle.ResultJSONPath,
			ResultJSONSHA256:        derefString(bundle.ResultJSONSHA256),
			AuthoritativeTextSHA256: bundle.Result.TextSHA256,
		},
	}
}

func failedEntryForRunnerError(personaID string, attemptCount int, bundle workerArtifactBundle) *failedBatchEntry {
	switch {
	case bundle.StatusInvalid || bundle.AttestationInvalid || bundle.ResultInvalid:
		return failedEntryFromBundle(personaID, attemptCount, failureReasonInvalidArtifact, bundle, true)
	case bundle.Status != nil:
		switch bundle.Status.Outcome {
		case statusOutcomeInterrupted:
			return failedEntryFromBundle(personaID, attemptCount, failureReasonWorkerInterrupted, bundle, false)
		case statusOutcomeFailed:
			return failedEntryFromBundle(personaID, attemptCount, failureReasonWorkerFailed, bundle, false)
		}
	case bundle.AnyArtifact:
		return failedEntryFromBundle(personaID, attemptCount, failureReasonMissingArtifact, bundle, false)
	}

	statusPath, attestationPath, _ := expectedBatchRelativePaths(personaID)
	return &failedBatchEntry{
		PersonaID:             personaID,
		Outcome:               batchOutcomeFailed,
		AttemptCount:          attemptCount,
		FailureReason:         failureReasonLaunchError,
		StatusPath:            stringPtr(statusPath),
		AttestationPath:       stringPtr(attestationPath),
		ResultJSONPath:        nil,
		RunnerTerminalOutcome: nil,
		LastReachedPhase:      nil,
		CloseOutcomeKind:      nil,
	}
}

func failedEntryFromBundle(personaID string, attemptCount int, reason string, bundle workerArtifactBundle, forceNullCarryThrough bool) *failedBatchEntry {
	runnerTerminalOutcome, lastReachedPhase, closeOutcomeKind := carryThroughFields(bundle, forceNullCarryThrough)
	statusPath := stringPtr(bundle.StatusPath)
	attestationPath := stringPtr(bundle.AttestationPath)
	resultJSONPath := (*string)(nil)
	if reason == failureReasonBatchCancelledWait {
		statusPath = nil
		attestationPath = nil
		runnerTerminalOutcome = stringPtr(notInvokedSentinel)
		lastReachedPhase = stringPtr(batchPreDispatchPhase)
		closeOutcomeKind = stringPtr(notInvokedSentinel)
	}
	return &failedBatchEntry{
		PersonaID:             personaID,
		Outcome:               batchOutcomeFailed,
		AttemptCount:          attemptCount,
		FailureReason:         reason,
		StatusPath:            statusPath,
		AttestationPath:       attestationPath,
		ResultJSONPath:        resultJSONPath,
		RunnerTerminalOutcome: runnerTerminalOutcome,
		LastReachedPhase:      lastReachedPhase,
		CloseOutcomeKind:      closeOutcomeKind,
	}
}

func cancelledBeforeStartFailure(personaID string) *failedBatchEntry {
	return &failedBatchEntry{
		PersonaID:             personaID,
		Outcome:               batchOutcomeFailed,
		AttemptCount:          0,
		FailureReason:         failureReasonBatchCancelledWait,
		StatusPath:            nil,
		AttestationPath:       nil,
		ResultJSONPath:        nil,
		RunnerTerminalOutcome: stringPtr(notInvokedSentinel),
		LastReachedPhase:      stringPtr(batchPreDispatchPhase),
		CloseOutcomeKind:      stringPtr(notInvokedSentinel),
	}
}

func launchFailure(personaID string) *failedBatchEntry {
	statusPath, attestationPath, _ := expectedBatchRelativePaths(personaID)
	return &failedBatchEntry{
		PersonaID:             personaID,
		Outcome:               batchOutcomeFailed,
		AttemptCount:          1,
		FailureReason:         failureReasonLaunchError,
		StatusPath:            stringPtr(statusPath),
		AttestationPath:       stringPtr(attestationPath),
		ResultJSONPath:        nil,
		RunnerTerminalOutcome: nil,
		LastReachedPhase:      nil,
		CloseOutcomeKind:      nil,
	}
}

func carryThroughFields(bundle workerArtifactBundle, forceNull bool) (*string, *string, *string) {
	if forceNull {
		return nil, nil, nil
	}
	if bundle.Attestation != nil {
		return cloneStringPtr(bundle.Attestation.RunnerTerminalOutcome), cloneStringPtr(bundle.Attestation.LastReachedPhase), cloneStringPtr(bundle.Attestation.CloseOutcomeKind)
	}
	if bundle.Status != nil {
		return nil, cloneStringPtr(bundle.Status.LastReachedPhase), cloneStringPtr(bundle.Status.CloseOutcomeKind)
	}
	return nil, nil, nil
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	return stringPtr(*value)
}

func expectedBatchRelativePaths(personaID string) (string, string, string) {
	statusPath := path.Join("personas", personaID, statusArtifactName)
	attestationPath := path.Join("personas", personaID, attestationArtifactName)
	resultJSONPath := path.Join("personas", personaID, resultJSONArtifactName)
	return statusPath, attestationPath, resultJSONPath
}

func loadWorkerArtifactBundle(answerRoot string, personaID string) workerArtifactBundle {
	statusRelPath, attestationRelPath, resultRelPath := expectedBatchRelativePaths(personaID)
	bundle := workerArtifactBundle{
		StatusPath:      statusRelPath,
		AttestationPath: attestationRelPath,
		ResultJSONPath:  resultRelPath,
	}

	statusAbsPath, err := storage.ResolvePath(answerRoot, statusRelPath)
	if err != nil {
		bundle.StatusInvalid = true
		return bundle
	}
	statusBytes, statusState := readOptionalArtifact(statusAbsPath)
	switch statusState {
	case "missing":
		bundle.StatusMissing = true
	case "invalid":
		bundle.StatusInvalid = true
		bundle.AnyArtifact = true
	case "ok":
		bundle.AnyArtifact = true
		statusArtifact, err := parseWorkerStatusArtifact(statusBytes)
		if err != nil {
			bundle.StatusInvalid = true
		} else {
			bundle.Status = statusArtifact
			if statusArtifact.ResultJSONPath != nil {
				bundle.ResultJSONPath = path.Join("personas", personaID, *statusArtifact.ResultJSONPath)
			}
		}
	}

	attestationAbsPath, err := storage.ResolvePath(answerRoot, attestationRelPath)
	if err != nil {
		bundle.AttestationInvalid = true
		return bundle
	}
	attestationBytes, attestationState := readOptionalArtifact(attestationAbsPath)
	switch attestationState {
	case "missing":
		bundle.AttestationMissing = true
	case "invalid":
		bundle.AttestationInvalid = true
		bundle.AnyArtifact = true
	case "ok":
		bundle.AnyArtifact = true
		attestationArtifact, err := parseWorkerAttestationArtifact(attestationBytes)
		if err != nil {
			bundle.AttestationInvalid = true
		} else {
			bundle.Attestation = attestationArtifact
		}
	}

	resultAbsPath, err := storage.ResolvePath(answerRoot, bundle.ResultJSONPath)
	if err != nil {
		bundle.ResultInvalid = true
		return bundle
	}
	resultBytes, resultState := readOptionalArtifact(resultAbsPath)
	switch resultState {
	case "missing":
		bundle.ResultMissing = true
	case "invalid":
		bundle.ResultInvalid = true
		bundle.AnyArtifact = true
	case "ok":
		bundle.AnyArtifact = true
		resultArtifact, err := parseWorkerResultArtifact(resultBytes)
		if err != nil {
			bundle.ResultInvalid = true
		} else {
			bundle.Result = resultArtifact
			resultSHA, err := panelhash.SHA256HexFile(resultAbsPath)
			if err != nil {
				bundle.ResultInvalid = true
				break
			}
			bundle.ResultJSONSHA256 = &resultSHA
		}
	}

	return bundle
}

func readOptionalArtifact(path string) ([]byte, string) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "missing"
		}
		return nil, "invalid"
	}
	if !info.Mode().IsRegular() {
		return nil, "invalid"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "invalid"
	}
	return data, "ok"
}

func normalizeForbiddenToolNames(input []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(input))
	for _, item := range input {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func forbiddenToolHits(primary []observedToolCall, fallback []observedToolCall, forbiddenToolNames []string) []forbiddenToolHit {
	observed := primary
	if len(observed) == 0 {
		observed = fallback
	}
	if len(observed) == 0 || len(forbiddenToolNames) == 0 {
		return nil
	}

	forbidden := make(map[string]struct{}, len(forbiddenToolNames))
	for _, name := range forbiddenToolNames {
		forbidden[name] = struct{}{}
	}

	hits := make([]forbiddenToolHit, 0)
	for _, observedCall := range observed {
		normalized := strings.ToLower(strings.TrimSpace(observedCall.ToolName))
		if normalized == "" {
			continue
		}
		if _, ok := forbidden[normalized]; !ok {
			continue
		}
		hits = append(hits, forbiddenToolHit{
			ToolName:           observedCall.ToolName,
			ToolNameNormalized: normalized,
			CallID:             cloneStringPtr(observedCall.CallID),
		})
	}
	return hits
}
