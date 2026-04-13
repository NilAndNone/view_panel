package appserver

import (
	"encoding/json"
	"strings"
)

const (
	JSONRPCVersion = "2.0"

	MethodInitialize             = "initialize"
	MethodInitialized            = "initialized"
	MethodConfigRequirementsRead = "configRequirements/read"
	MethodThreadStart            = "thread/start"
	MethodTurnStart              = "turn/start"
	MethodTurnInterrupt          = "turn/interrupt"

	EventTurnCompleted             = "turn/completed"
	EventItemCompleted             = "item/completed"
	EventItemCompletedAgentMessage = "item/completed.agentMessage"

	TurnStatusCompleted   = "completed"
	TurnStatusInterrupted = "interrupted"
	TurnStatusFailed      = "failed"
)

type RequestID int64

type JSONRPCRequestEnvelope struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      RequestID `json:"id"`
	Method  string    `json:"method"`
	Params  any       `json:"params,omitempty"`
}

type JSONRPCResponseEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      RequestID       `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCNotificationEnvelope struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type JSONRPCEventEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *JSONRPCError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func NewJSONRPCRequestEnvelope(id RequestID, method string, params any) JSONRPCRequestEnvelope {
	return JSONRPCRequestEnvelope{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

func NewJSONRPCNotificationEnvelope(method string, params any) JSONRPCNotificationEnvelope {
	return JSONRPCNotificationEnvelope{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  params,
	}
}

type PeerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type InitializeRequestParams struct {
	ProtocolVersion string         `json:"protocolVersion,omitempty"`
	ClientInfo      PeerInfo       `json:"clientInfo"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion string         `json:"protocolVersion,omitempty"`
	ServerInfo      *PeerInfo      `json:"serverInfo,omitempty"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
}

type InitializedParams struct{}

type ConfigRequirementsReadParams struct{}

type ConfigRequirementsReadResult struct {
	Requirements []ConfigRequirement `json:"requirements,omitempty"`
}

type ConfigRequirement struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Optional    bool   `json:"optional,omitempty"`
	Satisfied   bool   `json:"satisfied,omitempty"`
}

type ThreadStartParams struct {
	ApprovalPolicy string         `json:"approvalPolicy,omitempty"`
	CWD            string         `json:"cwd,omitempty"`
	Ephemeral      bool           `json:"ephemeral"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type ThreadStartResult struct {
	ThreadID string `json:"threadId"`
}

func (r *ThreadStartResult) UnmarshalJSON(data []byte) error {
	type rawThreadStartResult struct {
		ThreadID string `json:"threadId"`
		Thread   struct {
			ID string `json:"id"`
		} `json:"thread"`
	}

	var raw rawThreadStartResult
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	r.ThreadID = strings.TrimSpace(raw.ThreadID)
	if r.ThreadID == "" {
		r.ThreadID = strings.TrimSpace(raw.Thread.ID)
	}
	return nil
}

type TurnStartParams struct {
	ThreadID string          `json:"threadId,omitempty"`
	Input    []TurnInputItem `json:"input,omitempty"`
	Metadata map[string]any  `json:"metadata,omitempty"`
}

type TurnStartResult struct {
	TurnID string `json:"turnId"`
}

func (r *TurnStartResult) UnmarshalJSON(data []byte) error {
	type rawTurnStartResult struct {
		TurnID string `json:"turnId"`
		Turn   struct {
			ID string `json:"id"`
		} `json:"turn"`
	}

	var raw rawTurnStartResult
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	r.TurnID = strings.TrimSpace(raw.TurnID)
	if r.TurnID == "" {
		r.TurnID = strings.TrimSpace(raw.Turn.ID)
	}
	return nil
}

type TurnInterruptParams struct {
	ThreadID string `json:"threadId,omitempty"`
	TurnID   string `json:"turnId,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type TurnInputItem struct {
	Type    string                 `json:"type,omitempty"`
	Role    string                 `json:"role,omitempty"`
	Text    string                 `json:"text,omitempty"`
	Content []TurnInputContentPart `json:"content,omitempty"`
}

type TurnInputContentPart struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

type TurnCompletedEventParams struct {
	ThreadID string            `json:"threadId,omitempty"`
	TurnID   string            `json:"turnId,omitempty"`
	Turn     TurnCompletedTurn `json:"turn"`
}

type TurnCompletedTurn struct {
	ID     string            `json:"id,omitempty"`
	Status string            `json:"status,omitempty"`
	Error  *TurnErrorDetails `json:"error,omitempty"`
}

type TurnErrorDetails struct {
	Message           string `json:"message,omitempty"`
	AdditionalDetails string `json:"additionalDetails,omitempty"`
}

func (params TurnCompletedEventParams) EffectiveTurnID() string {
	if turnID := strings.TrimSpace(params.Turn.ID); turnID != "" {
		return turnID
	}
	return strings.TrimSpace(params.TurnID)
}

func (params TurnCompletedEventParams) NormalizedStatus() string {
	return strings.TrimSpace(params.Turn.Status)
}

func (params TurnCompletedEventParams) IsFailed() bool {
	return strings.EqualFold(params.NormalizedStatus(), TurnStatusFailed)
}

func (params TurnCompletedEventParams) IsInterrupted() bool {
	return strings.EqualFold(params.NormalizedStatus(), TurnStatusInterrupted)
}

func (params TurnCompletedEventParams) ErrorMessage() string {
	if params.Turn.Error == nil {
		return ""
	}
	return strings.TrimSpace(params.Turn.Error.Message)
}

type AgentMessageCompletedEventParams struct {
	Item       AgentMessageItem        `json:"item"`
	Completion *AgentMessageCompletion `json:"completion,omitempty"`
}

type AgentMessageItem struct {
	ID      string                    `json:"id,omitempty"`
	Type    string                    `json:"type,omitempty"`
	Phase   string                    `json:"phase,omitempty"`
	Role    string                    `json:"role,omitempty"`
	Status  string                    `json:"status,omitempty"`
	Text    string                    `json:"text,omitempty"`
	Content []AgentMessageContentPart `json:"content,omitempty"`
}

type AgentMessageContentPart struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

type AgentMessageCompletion struct {
	Model        string      `json:"model,omitempty"`
	FinishReason string      `json:"finishReason,omitempty"`
	StopReason   string      `json:"stopReason,omitempty"`
	Usage        *TokenUsage `json:"usage,omitempty"`
}

type TokenUsage struct {
	InputTokens  int `json:"inputTokens,omitempty"`
	OutputTokens int `json:"outputTokens,omitempty"`
	TotalTokens  int `json:"totalTokens,omitempty"`
}

func (params AgentMessageCompletedEventParams) IsAuthoritativeFinalAnswer() bool {
	return strings.EqualFold(strings.TrimSpace(params.Item.Type), "agentMessage") &&
		strings.EqualFold(strings.TrimSpace(params.Item.Phase), "final_answer")
}

func (item AgentMessageItem) PlainText() string {
	if item.Text != "" {
		return item.Text
	}

	var builder strings.Builder
	for _, part := range item.Content {
		if part.Text == "" {
			continue
		}
		builder.WriteString(part.Text)
	}

	return builder.String()
}
