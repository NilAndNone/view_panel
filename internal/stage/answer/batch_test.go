package answer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	panelhash "view_panel/internal/hash"
)

func TestNormalizeForbiddenToolNamesReturnsEmptySliceForNilInput(t *testing.T) {
	got := normalizeForbiddenToolNames(nil)

	if got == nil {
		t.Fatalf("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestBatchRunnerRetriesUntilSecondAttemptCertifies(t *testing.T) {
	answerRoot := t.TempDir()
	runner := &stubExecutor{
		AnswerRoot: answerRoot,
		Attempts: []stubAttempt{
			{Outcome: stubAttemptFailed},
			{Outcome: stubAttemptCertified},
		},
	}

	batch := BatchRunner{Runner: runner}
	result, err := batch.Execute(context.Background(), AnswerBatchRequest{
		RunID:                 "run-test",
		AnswerRoot:            answerRoot,
		Workspaces:            []SealedPersonaWorkspace{{PersonaID: "persona-a"}},
		MaxConcurrency:        1,
		MaxAttemptsPerPersona: 2,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if runner.Calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", runner.Calls)
	}
	if len(result.Certified) != 1 || result.Certified[0].AttemptCount != 2 {
		t.Fatalf("expected certified second attempt, got %#v", result)
	}

	artifact := readAnswerBatchArtifact(t, result.ArtifactPath)
	if artifact.MaxAttemptsPerPersona != 2 {
		t.Fatalf("expected max_attempts_per_persona 2 in artifact, got %d", artifact.MaxAttemptsPerPersona)
	}
}

func TestBatchRunnerStopsAfterConfiguredAttempts(t *testing.T) {
	answerRoot := t.TempDir()
	runner := &stubExecutor{
		AnswerRoot: answerRoot,
		Attempts: []stubAttempt{
			{Outcome: stubAttemptFailed},
			{Outcome: stubAttemptFailed},
		},
	}

	batch := BatchRunner{Runner: runner}
	result, err := batch.Execute(context.Background(), AnswerBatchRequest{
		RunID:                 "run-test",
		AnswerRoot:            answerRoot,
		Workspaces:            []SealedPersonaWorkspace{{PersonaID: "persona-a"}},
		MaxConcurrency:        1,
		MaxAttemptsPerPersona: 2,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if runner.Calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", runner.Calls)
	}
	if len(result.Failed) != 1 || result.Failed[0].AttemptCount != 2 {
		t.Fatalf("expected failed second attempt summary, got %#v", result)
	}
}

func TestBatchRunnerStopsRetryingAfterRejection(t *testing.T) {
	answerRoot := t.TempDir()
	runner := &stubExecutor{
		AnswerRoot: answerRoot,
		Attempts: []stubAttempt{
			{
				Outcome:           stubAttemptCertified,
				ObservedToolCalls: []observedToolCall{{ToolName: "shell"}},
			},
			{Outcome: stubAttemptCertified},
		},
	}

	batch := BatchRunner{Runner: runner}
	result, err := batch.Execute(context.Background(), AnswerBatchRequest{
		RunID:                 "run-test",
		AnswerRoot:            answerRoot,
		Workspaces:            []SealedPersonaWorkspace{{PersonaID: "persona-a"}},
		MaxConcurrency:        1,
		MaxAttemptsPerPersona: 2,
		ForbiddenToolNames:    []string{"shell"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if runner.Calls != 1 {
		t.Fatalf("expected retry loop to stop after rejection, got %d calls", runner.Calls)
	}
	if len(result.Rejected) != 1 || result.Rejected[0].AttemptCount != 1 {
		t.Fatalf("expected rejected first attempt summary, got %#v", result)
	}
}

func TestBatchRunnerDoesNotDispatchNewPersonaAfterCancellation(t *testing.T) {
	answerRoot := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := &stubExecutor{
		AnswerRoot: answerRoot,
		Attempts: []stubAttempt{
			{Outcome: stubAttemptBlockUntilCancel},
		},
		Started: make(chan int, 2),
	}

	type executeResult struct {
		result AnswerBatchResult
		err    error
	}
	done := make(chan executeResult, 1)
	go func() {
		result, err := (BatchRunner{Runner: runner}).Execute(ctx, AnswerBatchRequest{
			RunID:                 "run-test",
			AnswerRoot:            answerRoot,
			Workspaces:            []SealedPersonaWorkspace{{PersonaID: "persona-a"}, {PersonaID: "persona-b"}},
			MaxConcurrency:        1,
			MaxAttemptsPerPersona: 2,
		})
		done <- executeResult{result: result, err: err}
	}()

	select {
	case callNumber := <-runner.Started:
		if callNumber != 1 {
			t.Fatalf("expected first started call to be 1, got %d", callNumber)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for first worker start")
	}

	time.Sleep(50 * time.Millisecond)
	cancel()

	var execution executeResult
	select {
	case execution = <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for batch execution to finish")
	}
	if execution.err != nil {
		t.Fatalf("Execute returned error: %v", execution.err)
	}
	if runner.Calls != 1 {
		t.Fatalf("expected exactly 1 worker call after cancellation, got %d", runner.Calls)
	}
	if len(execution.result.Failed) != 2 {
		t.Fatalf("expected 2 failed entries, got %#v", execution.result)
	}
	if execution.result.Failed[0].PersonaID != "persona-a" || execution.result.Failed[0].FailureReason != failureReasonBatchCancelledRun || execution.result.Failed[0].AttemptCount != 1 {
		t.Fatalf("expected persona-a to fail in flight on attempt 1, got %#v", execution.result.Failed[0])
	}
	if execution.result.Failed[1].PersonaID != "persona-b" || execution.result.Failed[1].FailureReason != failureReasonBatchCancelledWait || execution.result.Failed[1].AttemptCount != 0 {
		t.Fatalf("expected persona-b to be cancelled before start, got %#v", execution.result.Failed[1])
	}
}

func TestBatchRunnerDoesNotStartRetryAfterCancellation(t *testing.T) {
	answerRoot := t.TempDir()
	runner := &stubExecutor{
		AnswerRoot: answerRoot,
		Attempts: []stubAttempt{
			{Outcome: stubAttemptFailed},
		},
	}

	outcome := (BatchRunner{Runner: runner}).executeOne(&scriptedContext{
		errs: []error{nil, nil, nil, nil, context.Canceled},
	}, AnswerBatchRequest{
		AnswerRoot:            answerRoot,
		MaxAttemptsPerPersona: 2,
	}, SealedPersonaWorkspace{
		PersonaID: "persona-a",
	}, nil)

	if runner.Calls != 1 {
		t.Fatalf("expected exactly 1 worker call before retry cancellation, got %d", runner.Calls)
	}
	if outcome.Failed == nil {
		t.Fatalf("expected failed outcome, got %#v", outcome)
	}
	if outcome.Failed.FailureReason != failureReasonBatchCancelledWait {
		t.Fatalf("expected cancellation-before-start failure, got %#v", outcome.Failed)
	}
	if outcome.Failed.AttemptCount != 1 {
		t.Fatalf("expected attempt_count 1 when retry is skipped after first attempt, got %#v", outcome.Failed)
	}
}

func TestNextReadyWorkerReturnsCancelledWhenReadyAndCanceledSimultaneously(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	readyWorkers := make(chan int, 1)
	readyWorkers <- 7
	cancel()

	worker, ok := nextReadyWorker(ctx, readyWorkers)
	if ok {
		t.Fatalf("expected cancellation to win ready boundary, got worker %d", worker)
	}
}

const (
	stubAttemptFailed           = "failed"
	stubAttemptCertified        = "certified"
	stubAttemptBlockUntilCancel = "block_until_cancel"
)

type stubAttempt struct {
	Outcome           string
	ObservedToolCalls []observedToolCall
}

type stubExecutor struct {
	AnswerRoot string
	Attempts   []stubAttempt
	Started    chan int

	mu    sync.Mutex
	Calls int
}

func (s *stubExecutor) Execute(ctx context.Context, req ExecuteWorkerRequest) (ExecuteWorkerResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Calls >= len(s.Attempts) {
		return ExecuteWorkerResult{}, fmt.Errorf("unexpected extra execute call %d", s.Calls+1)
	}

	attempt := s.Attempts[s.Calls]
	s.Calls++
	callNumber := s.Calls
	if s.Started != nil {
		select {
		case s.Started <- callNumber:
		default:
		}
	}

	switch attempt.Outcome {
	case stubAttemptFailed:
		if err := writeFailedArtifacts(s.AnswerRoot, req.Workspace.PersonaID); err != nil {
			return ExecuteWorkerResult{}, err
		}
	case stubAttemptCertified:
		if err := writeCertifiedArtifacts(s.AnswerRoot, req.Workspace.PersonaID, attempt.ObservedToolCalls); err != nil {
			return ExecuteWorkerResult{}, err
		}
	case stubAttemptBlockUntilCancel:
		<-ctx.Done()
		return ExecuteWorkerResult{
			PersonaID: req.Workspace.PersonaID,
		}, ctx.Err()
	default:
		return ExecuteWorkerResult{}, fmt.Errorf("unknown stub outcome %q", attempt.Outcome)
	}

	return ExecuteWorkerResult{
		PersonaID:         req.Workspace.PersonaID,
		ObservedToolCalls: append([]observedToolCall(nil), attempt.ObservedToolCalls...),
	}, nil
}

func writeFailedArtifacts(answerRoot, personaID string) error {
	personaDir := filepath.Join(answerRoot, "personas", personaID)
	if err := os.MkdirAll(personaDir, 0o755); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(personaDir, resultJSONArtifactName))

	lastReachedPhase := "runner_failed"
	closeOutcomeKind := "error"
	status := workerStatusArtifact{
		SchemaVersion:              workerStatusSchemaV1,
		Stage:                      answerStage,
		PersonaID:                  personaID,
		Outcome:                    statusOutcomeFailed,
		LastReachedPhase:           &lastReachedPhase,
		CloseOutcomeKind:           &closeOutcomeKind,
		AuthoritativeOutputPresent: false,
		ResultRawPresent:           false,
		ResultJSONPresent:          false,
		AttestationPresent:         true,
		SourceOutgoingInputPath:    outgoingInputPathLiteral,
		AttestationPath:            attestationArtifactName,
	}
	attestation := workerAttestationArtifact{
		SchemaVersion:              workerAttestationSchemaV1,
		Stage:                      answerStage,
		PersonaID:                  personaID,
		SourceOutgoingInputPath:    outgoingInputPathLiteral,
		RunnerTerminalOutcome:      &status.Outcome,
		LastReachedPhase:           &lastReachedPhase,
		CloseOutcomeKind:           &closeOutcomeKind,
		AuthoritativeOutputPresent: false,
		DiagnosticStreamItemCount:  0,
	}

	if err := writeJSONFile(filepath.Join(personaDir, attestationArtifactName), attestation); err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(personaDir, statusArtifactName), status)
}

func writeCertifiedArtifacts(answerRoot, personaID string, observedToolCalls []observedToolCall) error {
	personaDir := filepath.Join(answerRoot, "personas", personaID)
	if err := os.MkdirAll(personaDir, 0o755); err != nil {
		return err
	}

	text := "authoritative answer"
	textSHA := panelhash.SHA256Hex([]byte(text))
	itemID := "item-1"
	lastReachedPhase := "completed"
	closeOutcomeKind := "completed"
	result := workerResultArtifact{
		SchemaVersion:             workerResultSchemaV1,
		Stage:                     answerStage,
		PersonaID:                 personaID,
		SourceOutgoingInputPath:   outgoingInputPathLiteral,
		SourceOutgoingInputSHA256: "source-sha",
		CombinedInputSHA256:       "combined-sha",
		AuthoritativeItemType:     authoritativeCompletedItemType,
		AuthoritativeItemID:       itemID,
		Text:                      text,
		TextSHA256:                textSHA,
		AuthoritativeItem: map[string]any{
			"id":   itemID,
			"text": text,
			"type": authoritativeCompletedItemType,
		},
	}
	resultPath := filepath.Join(personaDir, resultJSONArtifactName)
	if err := writeJSONFile(resultPath, result); err != nil {
		return err
	}
	resultSHA, err := panelhash.SHA256HexFile(resultPath)
	if err != nil {
		return err
	}

	status := workerStatusArtifact{
		SchemaVersion:              workerStatusSchemaV1,
		Stage:                      answerStage,
		PersonaID:                  personaID,
		Outcome:                    statusOutcomeCompleted,
		LastReachedPhase:           &lastReachedPhase,
		CloseOutcomeKind:           &closeOutcomeKind,
		AuthoritativeOutputPresent: true,
		AuthoritativeItemType:      stringPtr(authoritativeCompletedItemType),
		AuthoritativeItemID:        &itemID,
		ResultRawPresent:           false,
		ResultJSONPresent:          true,
		AttestationPresent:         true,
		SourceOutgoingInputPath:    outgoingInputPathLiteral,
		ResultJSONPath:             stringPtr(resultJSONArtifactName),
		AttestationPath:            attestationArtifactName,
	}
	attestation := workerAttestationArtifact{
		SchemaVersion:              workerAttestationSchemaV1,
		Stage:                      answerStage,
		PersonaID:                  personaID,
		SourceOutgoingInputPath:    outgoingInputPathLiteral,
		RunnerTerminalOutcome:      stringPtr(statusOutcomeCompleted),
		LastReachedPhase:           &lastReachedPhase,
		CloseOutcomeKind:           &closeOutcomeKind,
		AuthoritativeOutputPresent: true,
		AuthoritativeItemType:      stringPtr(authoritativeCompletedItemType),
		AuthoritativeItemID:        &itemID,
		AuthoritativeTextSHA256:    &textSHA,
		ResultJSONSHA256:           &resultSHA,
		ObservedToolCalls:          append([]observedToolCall(nil), observedToolCalls...),
		DiagnosticStreamItemCount:  0,
	}

	if err := writeJSONFile(filepath.Join(personaDir, attestationArtifactName), attestation); err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(personaDir, statusArtifactName), status)
}

func writeJSONFile(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func readAnswerBatchArtifact(t *testing.T, path string) answerBatchArtifact {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read answer batch artifact: %v", err)
	}

	var artifact answerBatchArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("unmarshal answer batch artifact: %v", err)
	}
	return artifact
}

type scriptedContext struct {
	errs  []error
	mu    sync.Mutex
	calls int
}

func (c *scriptedContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (c *scriptedContext) Done() <-chan struct{} {
	return nil
}

func (c *scriptedContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.errs) == 0 {
		return nil
	}
	if c.calls >= len(c.errs) {
		return c.errs[len(c.errs)-1]
	}
	err := c.errs[c.calls]
	c.calls++
	return err
}

func (c *scriptedContext) Value(key any) any {
	return nil
}

func TestNormalizeForbiddenToolNamesReturnsEmptySliceWhenValuesNormalizeAway(t *testing.T) {
	got := normalizeForbiddenToolNames([]string{"", "   "})

	if got == nil {
		t.Fatalf("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}
