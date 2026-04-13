package appserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestBuildTurnInputItemsUsesTextUserInputShape(t *testing.T) {
	t.Parallel()

	items := buildTurnInputItems(RunSingleTurnOptions{
		Input: "Reply with exactly OK.",
	})
	if len(items) != 1 {
		t.Fatalf("expected 1 turn input item, got %d", len(items))
	}

	item := items[0]
	if got, want := item.Type, "text"; got != want {
		t.Fatalf("expected turn input type %q, got %q", want, got)
	}
	if got := item.Role; got != "" {
		t.Fatalf("expected turn input role to be empty, got %q", got)
	}
	if got, want := item.Text, "Reply with exactly OK."; got != want {
		t.Fatalf("expected turn input text %q, got %q", want, got)
	}
}

func TestRunSingleTurnUsesPinnedFinalAnswerItemCompletedShape(t *testing.T) {
	t.Parallel()

	stdoutReader, stdoutWriter := io.Pipe()
	writer := &protocolTestClientWriter{
		handleWrite: func(request rawEnvelope, client *Client) error {
		switch request.Method {
		case MethodInitialize:
			var params InitializeRequestParams
			if err := json.Unmarshal(request.Params, &params); err != nil {
				return err
			}
			if got, want := params.ClientInfo.Name, defaultInitializePeerName; got != want {
				return errors.New("unexpected initialize clientInfo.name")
			}
			if strings.TrimSpace(params.ClientInfo.Version) == "" {
				return errors.New("initialize clientInfo.version must be non-empty")
			}
			return protocolTestDispatchResult(client, request.ID, `{"platformFamily":"unix","platformOs":"android","userAgent":"view_panel/0.0.0"}`)
		case MethodInitialized:
			return nil
		case MethodConfigRequirementsRead:
			return protocolTestDispatchResult(client, request.ID, `{"requirements":null}`)
		case MethodThreadStart:
			var params struct {
				ApprovalPolicy string `json:"approvalPolicy"`
				CWD            string `json:"cwd"`
				Ephemeral      bool   `json:"ephemeral"`
				Metadata       map[string]any `json:"metadata"`
			}
			if err := json.Unmarshal(request.Params, &params); err != nil {
				return err
			}
			if got, want := params.ApprovalPolicy, "never"; got != want {
				return errors.New("unexpected thread/start approvalPolicy")
			}
			if got, want := params.CWD, "/tmp/view-panel"; got != want {
				return errors.New("unexpected thread/start cwd")
			}
			if !params.Ephemeral {
				return errors.New("thread/start ephemeral must be true")
			}
			if got, want := params.Metadata["origin"], "protocol-drift"; got != want {
				return errors.New("thread/start metadata.origin was not preserved")
			}
			reviewEnabled, ok := params.Metadata["review_enabled"].(bool)
			if !ok || !reviewEnabled {
				return errors.New("thread/start metadata.review_enabled was not preserved")
			}
			return protocolTestDispatchResult(client, request.ID, `{
				"approvalPolicy":"never",
				"approvalsReviewer":"user",
				"cwd":"/tmp/view-panel",
				"model":"gpt-5.4",
				"modelProvider":"openai",
				"reasoningEffort":"xhigh",
				"sandbox":{"type":"dangerFullAccess"},
				"serviceTier":"fast",
				"thread":{"id":"thread-1"}
			}`)
		case MethodTurnStart:
			if err := protocolTestDispatchResult(client, request.ID, `{
				"turn":{"error":null,"id":"turn-1","items":[],"status":"inProgress"}
			}`); err != nil {
				return err
			}
			if _, err := io.WriteString(stdoutWriter, `{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thread-1","turn":{"id":"turn-1","items":[],"status":"completed"}}}`+"\n"); err != nil {
				return err
			}
			if _, err := io.WriteString(stdoutWriter, `{"jsonrpc":"2.0","method":"item/completed","params":{"item":{"id":"msg-commentary","phase":"commentary","text":"Working","type":"agentMessage"},"threadId":"thread-1","turnId":"turn-1"}}`+"\n"); err != nil {
				return err
			}
			if _, err := io.WriteString(stdoutWriter, `{"jsonrpc":"2.0","method":"item/completed","params":{"item":{"id":"msg-1","phase":"final_answer","text":"OK","type":"agentMessage"},"threadId":"thread-1","turnId":"turn-1"}}`+"\n"); err != nil {
				return err
			}
			return stdoutWriter.Close()
		default:
			return errors.New("unexpected method")
		}
		},
	}
	client := &Client{
		stdout:                  stdoutReader,
		encoder:                 json.NewEncoder(writer),
		pending:                 make(map[RequestID]chan responseResult),
		events:                  make(chan ClientMessage, eventBufferCap),
		currentWorkingDirectory: "/tmp/view-panel",
		started:                 true,
		readerDone:              make(chan struct{}),
		waitDone:                make(chan struct{}),
		stderrDone:              make(chan struct{}),
		exitCode:                -1,
	}
	writer.client = client
	go client.readLoop()
	t.Cleanup(func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	})

	result, err := RunSingleTurn(context.Background(), client, RunSingleTurnOptions{
		Input: "Reply with exactly OK.",
		ThreadMetadata: map[string]any{
			"origin":         "protocol-drift",
			"review_enabled": true,
		},
	})
	if err != nil {
		t.Fatalf("expected RunSingleTurn success, got error: %v", err)
	}
	if got, want := result.TerminalOutcome, RunSingleTurnTerminalOutcomeCompleted; got != want {
		t.Fatalf("expected terminal outcome %q, got %q", want, got)
	}
	if result.CompletedItem == nil {
		t.Fatalf("expected completed item to be present")
	}
	if got, want := result.CompletedItem.Item.ID, "msg-1"; got != want {
		t.Fatalf("expected completed item id %q, got %q", want, got)
	}
	if got, want := len(result.Transcript), 3; got != want {
		t.Fatalf("expected transcript length %d, got %d", want, got)
	}
	if result.Transcript[0].TurnCompletedEvent == nil {
		t.Fatalf("expected transcript to include decoded turn/completed event")
	}
	if got, want := result.Transcript[0].TurnCompletedEvent.NormalizedStatus(), TurnStatusCompleted; got != want {
		t.Fatalf("expected turn/completed status %q, got %q", want, got)
	}
	if result.Transcript[1].AgentMessageCompletedEvent != nil {
		t.Fatalf("expected commentary item/completed event to remain non-authoritative")
	}
}

func TestRunSingleTurnReturnsFailedWhenTurnCompletedReportsTerminalFailure(t *testing.T) {
	t.Parallel()

	client := newProtocolTestClient(t, func(request rawEnvelope, client *Client) error {
		switch request.Method {
		case MethodInitialize:
			return protocolTestDispatchResult(client, request.ID, `{"platformFamily":"unix","platformOs":"android","userAgent":"view_panel/0.0.0"}`)
		case MethodInitialized:
			return nil
		case MethodConfigRequirementsRead:
			return protocolTestDispatchResult(client, request.ID, `{"requirements":null}`)
		case MethodThreadStart:
			return protocolTestDispatchResult(client, request.ID, `{
				"approvalPolicy":"never",
				"approvalsReviewer":"user",
				"cwd":"/tmp/view-panel",
				"model":"gpt-5.4",
				"modelProvider":"openai",
				"reasoningEffort":"xhigh",
				"sandbox":{"type":"dangerFullAccess"},
				"serviceTier":"fast",
				"thread":{"id":"thread-1"}
			}`)
		case MethodTurnStart:
			if err := protocolTestDispatchResult(client, request.ID, `{
				"turn":{"error":null,"id":"turn-1","items":[],"status":"inProgress"}
			}`); err != nil {
				return err
			}
			client.events <- mustProtocolTestMessage(t, client, rawEnvelope{
				JSONRPC: JSONRPCVersion,
				Method:  "turn/completed",
				Params: json.RawMessage(`{
					"threadId":"thread-1",
					"turn":{"error":{"message":"worker execution failed"},"id":"turn-1","items":[],"status":"failed"}
				}`),
			})
			return nil
		default:
			return errors.New("unexpected method")
		}
	})

	result, err := RunSingleTurn(context.Background(), client, RunSingleTurnOptions{
		Input:      "Reply with exactly OK.",
		RunTimeout: 100 * time.Millisecond,
	})
	if err == nil {
		t.Fatalf("expected RunSingleTurn failure")
	}
	if got, want := result.TerminalOutcome, RunSingleTurnTerminalOutcomeFailed; got != want {
		t.Fatalf("expected terminal outcome %q, got %q", want, got)
	}
	if result.CompletedItem != nil {
		t.Fatalf("expected completed item to be absent")
	}
	if result.RunError == nil {
		t.Fatalf("expected run error to be present")
	}
	if got := result.RunError.Error(); !strings.Contains(got, "worker execution failed") {
		t.Fatalf("expected run error to mention turn failure, got %q", got)
	}
	if got, want := len(result.Transcript), 1; got != want {
		t.Fatalf("expected transcript length %d, got %d", want, got)
	}
}

func TestDecodeThreadStartResultSupportsNestedThreadID(t *testing.T) {
	t.Parallel()

	result, err := decodeResult[ThreadStartResult](json.RawMessage(`{"thread":{"id":"thread-1"}}`))
	if err != nil {
		t.Fatalf("expected decode success, got error: %v", err)
	}
	if got, want := result.ThreadID, "thread-1"; got != want {
		t.Fatalf("expected nested thread id %q, got %q", want, got)
	}
}

func TestDecodeTurnStartResultSupportsNestedTurnID(t *testing.T) {
	t.Parallel()

	result, err := decodeResult[TurnStartResult](json.RawMessage(`{"turn":{"id":"turn-1"}}`))
	if err != nil {
		t.Fatalf("expected decode success, got error: %v", err)
	}
	if got, want := result.TurnID, "turn-1"; got != want {
		t.Fatalf("expected nested turn id %q, got %q", want, got)
	}
}

func TestWaitForAuthoritativeCompletionIgnoresNonFinalItemCompletedUntilFinalAnswer(t *testing.T) {
	t.Parallel()

	client := &Client{
		events:   make(chan ClientMessage, eventBufferCap),
		started:  true,
		exitCode: -1,
	}

	commentaryMessage, err := client.makeClientMessage(rawEnvelope{
		JSONRPC: JSONRPCVersion,
		Method:  "item/completed",
		Params: json.RawMessage(`{
			"item":{"id":"msg-commentary","phase":"commentary","text":"Working","type":"agentMessage"},
			"threadId":"thread-1",
			"turnId":"turn-1"
		}`),
	})
	if err != nil {
		t.Fatalf("expected commentary event decode success, got error: %v", err)
	}

	finalMessage, err := client.makeClientMessage(rawEnvelope{
		JSONRPC: JSONRPCVersion,
		Method:  "item/completed",
		Params: json.RawMessage(`{
			"item":{"id":"msg-final","phase":"final_answer","text":"OK","type":"agentMessage"},
			"threadId":"thread-1",
			"turnId":"turn-1"
		}`),
	})
	if err != nil {
		t.Fatalf("expected final event decode success, got error: %v", err)
	}

	client.events <- ClientMessage{
		Event: JSONRPCEventEnvelope{
			JSONRPC: JSONRPCVersion,
			Method:  "turn/completed",
			Params:  json.RawMessage(`{"turnId":"turn-1"}`),
		},
	}
	client.events <- commentaryMessage
	client.events <- finalMessage

	result := RunSingleTurnResult{
		LastReachedPhase: LifecyclePhaseTurnStarted,
	}
	completed, err := waitForAuthoritativeCompletion(context.Background(), client, &result)
	if err != nil {
		t.Fatalf("expected authoritative completion, got error: %v", err)
	}
	if completed == nil {
		t.Fatalf("expected completed item")
	}
	if got, want := completed.Item.ID, "msg-final"; got != want {
		t.Fatalf("expected final completed item id %q, got %q", want, got)
	}
	if got, want := len(result.Transcript), 3; got != want {
		t.Fatalf("expected transcript length %d, got %d", want, got)
	}
}

func TestWaitForAuthoritativeCompletionStopsOnInterruptedTurnCompleted(t *testing.T) {
	t.Parallel()

	client := &Client{
		events:   make(chan ClientMessage, eventBufferCap),
		started:  true,
		exitCode: -1,
	}

	interruptedMessage, err := client.makeClientMessage(rawEnvelope{
		JSONRPC: JSONRPCVersion,
		Method:  "turn/completed",
		Params: json.RawMessage(`{
			"threadId":"thread-1",
			"turn":{"error":null,"id":"turn-1","items":[],"status":"interrupted"}
		}`),
	})
	if err != nil {
		t.Fatalf("expected interrupted event decode success, got error: %v", err)
	}

	client.events <- interruptedMessage

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := RunSingleTurnResult{
		ThreadID:         "thread-1",
		TurnID:           "turn-1",
		LastReachedPhase: LifecyclePhaseTurnStarted,
	}
	completed, err := waitForAuthoritativeCompletion(ctx, client, &result)
	if err == nil {
		t.Fatalf("expected interrupted turn to stop authoritative wait")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected interrupted turn to stop without waiting for context timeout")
	}
	if completed != nil {
		t.Fatalf("expected completed item to be absent")
	}
	if got := err.Error(); !strings.Contains(got, "interrupted") {
		t.Fatalf("expected interrupted error, got %q", got)
	}
	if got, want := len(result.Transcript), 1; got != want {
		t.Fatalf("expected transcript length %d, got %d", want, got)
	}
}

func TestWaitForAuthoritativeCompletionPinsRawIngressToItemCompletedFinalAnswer(t *testing.T) {
	t.Parallel()

	stdoutReader, stdoutWriter := io.Pipe()
	client := &Client{
		stdout:     stdoutReader,
		events:     make(chan ClientMessage, eventBufferCap),
		started:    true,
		readerDone: make(chan struct{}),
		waitDone:   make(chan struct{}),
		stderrDone: make(chan struct{}),
		exitCode:   -1,
	}

	go client.readLoop()
	go func() {
		defer stdoutWriter.Close()
		_, _ = io.WriteString(stdoutWriter, `{"jsonrpc":"2.0","method":"turn/completed","params":{"turnId":"turn-1"}}`+"\n")
		_, _ = io.WriteString(stdoutWriter, `{"jsonrpc":"2.0","method":"item/completed.agentMessage","params":{"item":{"id":"msg-legacy","phase":"final_answer","text":"Old","type":"agentMessage"}}}`+"\n")
		_, _ = io.WriteString(stdoutWriter, `{"jsonrpc":"2.0","method":"item/completed","params":{"item":{"id":"msg-commentary","phase":"commentary","text":"Working","type":"agentMessage"},"threadId":"thread-1","turnId":"turn-1"}}`+"\n")
		_, _ = io.WriteString(stdoutWriter, `{"jsonrpc":"2.0","method":"item/completed","params":{"item":{"id":"msg-final","phase":"final_answer","text":"OK","type":"agentMessage"},"threadId":"thread-1","turnId":"turn-1"}}`+"\n")
	}()

	result := RunSingleTurnResult{
		LastReachedPhase: LifecyclePhaseTurnStarted,
	}
	completed, err := waitForAuthoritativeCompletion(context.Background(), client, &result)
	if err != nil {
		t.Fatalf("expected authoritative completion, got error: %v", err)
	}
	if completed == nil {
		t.Fatalf("expected completed item")
	}
	if got, want := completed.Item.ID, "msg-final"; got != want {
		t.Fatalf("expected final completed item id %q, got %q", want, got)
	}
	if got, want := len(result.Transcript), 4; got != want {
		t.Fatalf("expected transcript length %d, got %d", want, got)
	}
	if result.Transcript[1].AgentMessageCompletedEvent != nil {
		t.Fatalf("expected legacy item/completed.agentMessage event to remain non-authoritative at raw ingress")
	}
}

func TestMakeClientMessageLegacyAgentMessageIsNeverAuthoritative(t *testing.T) {
	t.Parallel()

	client := &Client{}

	finalMessage := mustProtocolTestMessage(t, client, rawEnvelope{
		JSONRPC: JSONRPCVersion,
		Method:  EventItemCompletedAgentMessage,
		Params: json.RawMessage(`{
			"item":{"id":"msg-final","phase":"final_answer","text":"OK","type":"agentMessage"}
		}`),
	})
	if finalMessage.AgentMessageCompletedEvent != nil {
		t.Fatalf("expected legacy item/completed.agentMessage event to remain non-authoritative")
	}
}

type protocolTestClientWriter struct {
	buffer      bytes.Buffer
	handleWrite func(rawEnvelope, *Client) error
	client      *Client
}

func newProtocolTestClient(t *testing.T, handleWrite func(rawEnvelope, *Client) error) *Client {
	t.Helper()

	writer := &protocolTestClientWriter{
		handleWrite: handleWrite,
	}
	client := &Client{
		encoder:    json.NewEncoder(writer),
		pending:    make(map[RequestID]chan responseResult),
		events:     make(chan ClientMessage, eventBufferCap),
		started:    true,
		readerDone: make(chan struct{}),
		waitDone:   make(chan struct{}),
		stderrDone: make(chan struct{}),
		exitCode:   -1,
	}
	writer.client = client
	return client
}

func (w *protocolTestClientWriter) Write(p []byte) (int, error) {
	if _, err := w.buffer.Write(p); err != nil {
		return 0, err
	}

	for {
		data := w.buffer.Bytes()
		index := bytes.IndexByte(data, '\n')
		if index < 0 {
			return len(p), nil
		}

		line := append([]byte(nil), data[:index]...)
		w.buffer.Next(index + 1)
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var envelope rawEnvelope
		if err := json.Unmarshal(line, &envelope); err != nil {
			return 0, err
		}
		if err := w.handleWrite(envelope, w.client); err != nil {
			return 0, err
		}
	}
}

func protocolTestDispatchResult(client *Client, requestID json.RawMessage, result string) error {
	id, err := parseRequestID(requestID)
	if err != nil {
		return err
	}

	client.dispatchResponse(JSONRPCResponseEnvelope{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  json.RawMessage(result),
	})
	return nil
}

func mustProtocolTestMessage(t *testing.T, client *Client, envelope rawEnvelope) ClientMessage {
	t.Helper()

	message, err := client.makeClientMessage(envelope)
	if err != nil {
		t.Fatalf("expected client message decode success, got error: %v", err)
	}
	return message
}
