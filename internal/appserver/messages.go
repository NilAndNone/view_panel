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

	EventItemCompletedAgentMessage = "item/completed.agentMessage"
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
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ThreadStartResult struct {
	ThreadID string `json:"threadId"`
}

type TurnStartParams struct {
	ThreadID string           `json:"threadId,omitempty"`
	Input    []TurnInputItem  `json:"input,omitempty"`
	Metadata map[string]any   `json:"metadata,omitempty"`
}

type TurnStartResult struct {
	TurnID string `json:"turnId"`
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

type AgentMessageCompletedEventParams struct {
	Item       AgentMessageItem          `json:"item"`
	Completion *AgentMessageCompletion   `json:"completion,omitempty"`
}

type AgentMessageItem struct {
	ID      string                    `json:"id,omitempty"`
	Type    string                    `json:"type,omitempty"`
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
	Model        string     `json:"model,omitempty"`
	FinishReason string     `json:"finishReason,omitempty"`
	StopReason   string     `json:"stopReason,omitempty"`
	Usage        *TokenUsage `json:"usage,omitempty"`
}

type TokenUsage struct {
	InputTokens  int `json:"inputTokens,omitempty"`
	OutputTokens int `json:"outputTokens,omitempty"`
	TotalTokens  int `json:"totalTokens,omitempty"`
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
