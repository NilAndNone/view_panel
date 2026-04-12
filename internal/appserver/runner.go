package appserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	interruptGraceTimeout      = 5 * time.Second
	defaultInitializePeerName  = "view_panel"
	interruptReasonCanceled    = "canceled"
	interruptReasonTimedOut    = "timeout"
	defaultTurnInputItemType   = "message"
	defaultTurnInputItemRole   = "user"
)

type LifecyclePhase string

const (
	LifecyclePhaseNotStarted                  LifecyclePhase = "NotStarted"
	LifecyclePhaseInitializeResponseReceived  LifecyclePhase = "InitializeResponseReceived"
	LifecyclePhaseInitializedNotificationSent LifecyclePhase = "InitializedNotificationSent"
	LifecyclePhaseRequirementsRead            LifecyclePhase = "RequirementsRead"
	LifecyclePhaseThreadStarted               LifecyclePhase = "ThreadStarted"
	LifecyclePhaseTurnStarted                 LifecyclePhase = "TurnStarted"
)

type RunSingleTurnTerminalOutcome string

const (
	RunSingleTurnTerminalOutcomeCompleted   RunSingleTurnTerminalOutcome = "Completed"
	RunSingleTurnTerminalOutcomeInterrupted RunSingleTurnTerminalOutcome = "Interrupted"
	RunSingleTurnTerminalOutcomeFailed      RunSingleTurnTerminalOutcome = "Failed"
)

type RunSingleTurnCloseOutcome struct {
	Kind            ProcessStopKind `json:"kind"`
	ProcessExitCode int             `json:"process_exit_code,omitempty"`
	Error           string          `json:"error,omitempty"`
}

type RunSingleTurnError struct {
	Phase     LifecyclePhase `json:"phase,omitempty"`
	Method    string         `json:"method,omitempty"`
	RequestID RequestID      `json:"request_id,omitempty"`
	Message   string         `json:"message"`
}

func (e *RunSingleTurnError) Error() string {
	if e == nil {
		return ""
	}

	parts := make([]string, 0, 3)
	if e.Phase != "" {
		parts = append(parts, string(e.Phase))
	}
	if e.Method != "" {
		parts = append(parts, e.Method)
	}
	if e.RequestID != 0 {
		parts = append(parts, fmt.Sprintf("id=%d", e.RequestID))
	}

	if len(parts) == 0 {
		return e.Message
	}
	return strings.Join(parts, " ") + ": " + e.Message
}

type RunSingleTurnResult struct {
	TerminalOutcome RunSingleTurnTerminalOutcome `json:"terminal_outcome"`
	LastReachedPhase LifecyclePhase              `json:"last_reached_phase"`
	ThreadID         string                      `json:"thread_id,omitempty"`
	TurnID           string                      `json:"turn_id,omitempty"`
	CompletedItem    *AgentMessageCompletedEventParams `json:"completed_item,omitempty"`
	Transcript       []ClientMessage                   `json:"transcript,omitempty"`
	CloseOutcome     RunSingleTurnCloseOutcome         `json:"close_outcome"`
	RunError         *RunSingleTurnError               `json:"error,omitempty"`
}

type RunSingleTurnOptions struct {
	Input          string         `json:"input,omitempty"`
	InputItems     []TurnInputItem `json:"input_items,omitempty"`
	ThreadMetadata map[string]any `json:"thread_metadata,omitempty"`
	TurnMetadata   map[string]any `json:"turn_metadata,omitempty"`
	RunTimeout     time.Duration  `json:"run_timeout,omitempty"`
}

func RunSingleTurn(ctx context.Context, client *Client, options RunSingleTurnOptions) (RunSingleTurnResult, error) {
	if options.RunTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.RunTimeout)
		defer cancel()
	}

	result := RunSingleTurnResult{
		LastReachedPhase: LifecyclePhaseNotStarted,
		CloseOutcome: RunSingleTurnCloseOutcome{
			Kind: ProcessStopKindNotStarted,
		},
	}

	if client == nil || !client.isStarted() {
		return failWithoutClient(&result, "client not started")
	}

	if err := ctx.Err(); err != nil {
		result.TerminalOutcome = RunSingleTurnTerminalOutcomeFailed
		result.RunError = newRunSingleTurnError(result.LastReachedPhase, "", err)
		result.CloseOutcome = closeOutcomeFromProcessStop(client.Close(context.Background()))
		return result, result.RunError
	}

	initializeRaw, err := client.sendRequest(ctx, MethodInitialize, InitializeRequestParams{
		ClientInfo: PeerInfo{Name: defaultInitializePeerName},
	})
	if err != nil {
		return finalizePreTurnFailure(ctx, client, &result, MethodInitialize, err)
	}
	if _, err := decodeResult[InitializeResult](initializeRaw); err != nil {
		return finalizeFailureWithClose(client, &result, MethodInitialize, fmt.Errorf("decode initialize result: %w", err))
	}
	result.LastReachedPhase = LifecyclePhaseInitializeResponseReceived

	if err := client.sendNotification(ctx, MethodInitialized, InitializedParams{}); err != nil {
		return finalizePreTurnFailure(ctx, client, &result, MethodInitialized, err)
	}
	result.LastReachedPhase = LifecyclePhaseInitializedNotificationSent

	requirementsRaw, err := client.sendRequest(ctx, MethodConfigRequirementsRead, ConfigRequirementsReadParams{})
	if err != nil {
		return finalizePreTurnFailure(ctx, client, &result, MethodConfigRequirementsRead, err)
	}

	requirements, err := decodeResult[ConfigRequirementsReadResult](requirementsRaw)
	if err != nil {
		return finalizeFailureWithClose(client, &result, MethodConfigRequirementsRead, fmt.Errorf("decode configRequirements/read result: %w", err))
	}
	if err := validateConfigRequirements(requirements); err != nil {
		return finalizeFailureWithClose(client, &result, MethodConfigRequirementsRead, err)
	}
	result.LastReachedPhase = LifecyclePhaseRequirementsRead

	threadRaw, err := client.sendRequest(ctx, MethodThreadStart, ThreadStartParams{
		Metadata: cloneMetadata(options.ThreadMetadata),
	})
	if err != nil {
		return finalizePreTurnFailure(ctx, client, &result, MethodThreadStart, err)
	}

	threadResult, err := decodeResult[ThreadStartResult](threadRaw)
	if err != nil {
		return finalizeFailureWithClose(client, &result, MethodThreadStart, fmt.Errorf("decode thread/start result: %w", err))
	}
	if strings.TrimSpace(threadResult.ThreadID) == "" {
		return finalizeFailureWithClose(client, &result, MethodThreadStart, errors.New("thread/start returned empty threadId"))
	}
	result.ThreadID = threadResult.ThreadID
	result.LastReachedPhase = LifecyclePhaseThreadStarted

	turnRaw, err := client.sendRequest(ctx, MethodTurnStart, TurnStartParams{
		ThreadID: result.ThreadID,
		Input:    buildTurnInputItems(options),
		Metadata: cloneMetadata(options.TurnMetadata),
	})
	if err != nil {
		return finalizePreTurnFailure(ctx, client, &result, MethodTurnStart, err)
	}

	turnResult, err := decodeResult[TurnStartResult](turnRaw)
	if err != nil {
		return finalizeFailureWithClose(client, &result, MethodTurnStart, fmt.Errorf("decode turn/start result: %w", err))
	}
	if strings.TrimSpace(turnResult.TurnID) == "" {
		return finalizeFailureWithClose(client, &result, MethodTurnStart, errors.New("turn/start returned empty turnId"))
	}
	result.TurnID = turnResult.TurnID
	result.LastReachedPhase = LifecyclePhaseTurnStarted

	completedItem, err := waitForAuthoritativeCompletion(ctx, client, &result)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return finalizePostTurnInterrupt(client, &result, ctx.Err())
		}
		return finalizeFailureWithClose(client, &result, "", err)
	}

	return finalizeCompleted(client, &result, completedItem)
}

func failWithoutClient(result *RunSingleTurnResult, message string) (RunSingleTurnResult, error) {
	result.TerminalOutcome = RunSingleTurnTerminalOutcomeFailed
	result.RunError = newRunSingleTurnError(result.LastReachedPhase, "", errors.New(message))
	return *result, result.RunError
}

func finalizePreTurnFailure(ctx context.Context, client *Client, result *RunSingleTurnResult, method string, err error) (RunSingleTurnResult, error) {
	if shouldHardStopBeforeTurn(ctx, err) {
		return finalizeFailureWithHardStop(client, result, method, err)
	}
	return finalizeFailureWithClose(client, result, method, err)
}

func finalizeFailureWithHardStop(client *Client, result *RunSingleTurnResult, method string, err error) (RunSingleTurnResult, error) {
	result.TerminalOutcome = RunSingleTurnTerminalOutcomeFailed
	result.RunError = newRunSingleTurnError(result.LastReachedPhase, method, err)
	result.CloseOutcome = closeOutcomeFromProcessStop(client.ForceKill())
	return *result, result.RunError
}

func finalizeFailureWithClose(client *Client, result *RunSingleTurnResult, method string, err error) (RunSingleTurnResult, error) {
	result.TerminalOutcome = RunSingleTurnTerminalOutcomeFailed
	result.RunError = newRunSingleTurnError(result.LastReachedPhase, method, err)
	result.CloseOutcome = closeOutcomeFromProcessStop(client.Close(context.Background()))
	return *result, result.RunError
}

func finalizeCompleted(client *Client, result *RunSingleTurnResult, completedItem *AgentMessageCompletedEventParams) (RunSingleTurnResult, error) {
	result.TerminalOutcome = RunSingleTurnTerminalOutcomeCompleted
	result.CompletedItem = completedItem
	result.RunError = nil
	result.CloseOutcome = closeOutcomeFromProcessStop(client.Close(context.Background()))
	return *result, nil
}

func finalizePostTurnInterrupt(client *Client, result *RunSingleTurnResult, cause error) (RunSingleTurnResult, error) {
	completedItem, err := waitForInterruptGrace(client, result, cause)
	if err == nil {
		return finalizeCompleted(client, result, completedItem)
	}

	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, io.EOF) {
		return finalizeFailureWithClose(client, result, MethodTurnInterrupt, err)
	}

	result.TerminalOutcome = RunSingleTurnTerminalOutcomeInterrupted
	result.RunError = newRunSingleTurnError(result.LastReachedPhase, MethodTurnInterrupt, fmt.Errorf("turn did not complete after interrupt attempt: %w", cause))
	result.CloseOutcome = closeOutcomeFromProcessStop(client.ForceKill())
	return *result, result.RunError
}

func waitForAuthoritativeCompletion(ctx context.Context, client *Client, result *RunSingleTurnResult) (*AgentMessageCompletedEventParams, error) {
	for {
		message, err := client.ReadMessage(ctx)
		if err != nil {
			return nil, err
		}

		result.Transcript = append(result.Transcript, message)

		if message.AgentMessageCompletedEvent != nil {
			return message.AgentMessageCompletedEvent, nil
		}
		if isCompletionLikeEventMethod(message.Event.Method) {
			return nil, unexpectedTerminalEventError(result.LastReachedPhase, message.Event.Method)
		}
	}
}

func waitForInterruptGrace(client *Client, result *RunSingleTurnResult, cause error) (*AgentMessageCompletedEventParams, error) {
	deadline := time.Now().Add(interruptGraceTimeout)
	interruptCtx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	go func() {
		_ = client.sendNotification(interruptCtx, MethodTurnInterrupt, TurnInterruptParams{
			ThreadID: result.ThreadID,
			TurnID:   result.TurnID,
			Reason:   interruptReason(cause),
		})
	}()

	for {
		readCtx, readCancel := context.WithDeadline(context.Background(), deadline)
		message, err := client.ReadMessage(readCtx)
		readCancel()
		if err != nil {
			return nil, err
		}

		result.Transcript = append(result.Transcript, message)

		if message.AgentMessageCompletedEvent != nil {
			return message.AgentMessageCompletedEvent, nil
		}
		if isCompletionLikeEventMethod(message.Event.Method) {
			return nil, unexpectedTerminalEventError(result.LastReachedPhase, message.Event.Method)
		}
	}
}

func buildTurnInputItems(options RunSingleTurnOptions) []TurnInputItem {
	if len(options.InputItems) != 0 {
		items := make([]TurnInputItem, len(options.InputItems))
		copy(items, options.InputItems)
		return items
	}
	if options.Input == "" {
		return nil
	}

	return []TurnInputItem{
		{
			Type: defaultTurnInputItemType,
			Role: defaultTurnInputItemRole,
			Text: options.Input,
		},
	}
}

func validateConfigRequirements(result ConfigRequirementsReadResult) error {
	unsatisfied := make([]string, 0)
	for _, requirement := range result.Requirements {
		if requirement.Optional || requirement.Satisfied {
			continue
		}

		name := strings.TrimSpace(requirement.Name)
		if name == "" {
			name = "unnamed requirement"
		}
		unsatisfied = append(unsatisfied, name)
	}

	if len(unsatisfied) == 0 {
		return nil
	}
	return fmt.Errorf("unsatisfied config requirements: %s", strings.Join(unsatisfied, ", "))
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}

	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func shouldHardStopBeforeTurn(ctx context.Context, err error) bool {
	var timeoutErr *RequestTimeoutError
	if errors.As(err, &timeoutErr) {
		return true
	}

	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func closeOutcomeFromProcessStop(stop ProcessStopResult) RunSingleTurnCloseOutcome {
	outcome := RunSingleTurnCloseOutcome{
		Kind:            stop.Kind,
		ProcessExitCode: stop.ProcessExitCode,
	}
	if stop.Error != nil {
		outcome.Error = stop.Error.Error()
	}
	return outcome
}

func newRunSingleTurnError(phase LifecyclePhase, method string, err error) *RunSingleTurnError {
	if err == nil {
		return nil
	}

	var existing *RunSingleTurnError
	if errors.As(err, &existing) {
		copy := *existing
		if copy.Phase == "" {
			copy.Phase = phase
		}
		if copy.Method == "" {
			copy.Method = method
		}
		return &copy
	}

	runErr := &RunSingleTurnError{
		Phase:   phase,
		Method:  method,
		Message: err.Error(),
	}

	var requestCallErr *RequestCallError
	if errors.As(err, &requestCallErr) {
		if runErr.Method == "" {
			runErr.Method = requestCallErr.Method
		}
		runErr.RequestID = requestCallErr.ID
	}

	var requestTimeoutErr *RequestTimeoutError
	if errors.As(err, &requestTimeoutErr) {
		if runErr.Method == "" {
			runErr.Method = requestTimeoutErr.Method
		}
		runErr.RequestID = requestTimeoutErr.ID
	}

	return runErr
}

func unexpectedTerminalEventError(phase LifecyclePhase, method string) *RunSingleTurnError {
	return &RunSingleTurnError{
		Phase:   phase,
		Method:  method,
		Message: fmt.Sprintf("unexpected terminal event %q", method),
	}
}

func interruptReason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return interruptReasonTimedOut
	}
	return interruptReasonCanceled
}

func isCompletionLikeEventMethod(method string) bool {
	if method == "" {
		return false
	}
	if method == EventItemCompletedAgentMessage {
		return true
	}

	normalized := strings.ToLower(method)
	return strings.Contains(normalized, "completed") ||
		strings.Contains(normalized, "finished") ||
		strings.Contains(normalized, "failed") ||
		strings.Contains(normalized, "terminated")
}
