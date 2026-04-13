package appserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	requestTimeout = 30 * time.Second
	closeTimeout   = 10 * time.Second
	eventBufferCap = 64
)

type AppServerLaunchContext struct {
	ExtraArgs               []string
	Environment             map[string]string
	CurrentWorkingDirectory string
	HomeDir                 string
	CodexHomeDir            string
}

type Client struct {
	process *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser

	encoder *json.Encoder

	nextRequestID int64

	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[RequestID]chan responseResult

	events chan ClientMessage

	stateMu     sync.Mutex
	started     bool
	closing     bool
	closed      bool
	stdinClosed bool

	readerOnce sync.Once
	readerDone chan struct{}
	readerErr  error

	waitOnce sync.Once
	waitDone chan struct{}
	waitErr  error
	exitCode int

	stderrDone chan struct{}
	stderrText stderrBuffer
}

type ClientMessage struct {
	Event                      JSONRPCEventEnvelope
	AgentMessageCompletedEvent *AgentMessageCompletedEventParams
}

type ProcessStopKind string

const (
	ProcessStopKindGraceful      ProcessStopKind = "graceful"
	ProcessStopKindAlreadyExited ProcessStopKind = "alreadyExited"
	ProcessStopKindForcedKill    ProcessStopKind = "forcedKill"
	ProcessStopKindCloseFailed   ProcessStopKind = "closeFailed"
	ProcessStopKindNotStarted    ProcessStopKind = "notStarted"
)

type ProcessStopResult struct {
	Kind            ProcessStopKind
	ProcessExitCode int
	Error           error
}

type responseResult struct {
	response JSONRPCResponseEnvelope
	err      error
}

type rawEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type RequestCallError struct {
	Method string
	ID     RequestID
	Err    error
}

func (e *RequestCallError) Error() string {
	if e == nil {
		return ""
	}
	if e.ID == 0 {
		return fmt.Sprintf("%s: %v", e.Method, e.Err)
	}
	return fmt.Sprintf("%s (id=%d): %v", e.Method, e.ID, e.Err)
}

func (e *RequestCallError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type RequestTimeoutError struct {
	Method  string
	ID      RequestID
	Timeout time.Duration
}

func (e *RequestTimeoutError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s (id=%d) timed out after %s", e.Method, e.ID, e.Timeout)
}

type stderrBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *stderrBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *stderrBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func StartAppServer(ctx context.Context, launchContext AppServerLaunchContext) (*Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(launchContext.CurrentWorkingDirectory) == "" {
		return nil, errors.New("app-server launch requires CurrentWorkingDirectory")
	}

	args := []string{"app-server", "--listen", "stdio://"}
	args = append(args, launchContext.ExtraArgs...)

	cmd := exec.Command("codex", args...)
	cmd.Dir = launchContext.CurrentWorkingDirectory
	cmd.Env = buildSealedLaunchEnvironment(launchContext)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open app-server stdin: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("open app-server stdout: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("open app-server stderr: %w", err)
	}

	if err := ctx.Err(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("start codex app-server: %w", err)
	}

	client := &Client{
		process:    cmd,
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		encoder:    json.NewEncoder(stdin),
		pending:    make(map[RequestID]chan responseResult),
		events:     make(chan ClientMessage, eventBufferCap),
		started:    true,
		readerDone: make(chan struct{}),
		waitDone:   make(chan struct{}),
		stderrDone: make(chan struct{}),
		exitCode:   -1,
	}

	go client.readLoop()
	go client.captureStderr()
	go client.waitLoop()

	return client, nil
}

func (c *Client) sendRequest(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if c == nil || !c.isStarted() {
		return nil, &RequestCallError{Method: method, Err: errors.New("client not started")}
	}

	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	id := RequestID(atomic.AddInt64(&c.nextRequestID, 1))
	responseCh := make(chan responseResult, 1)

	c.pendingMu.Lock()
	c.pending[id] = responseCh
	c.pendingMu.Unlock()

	defer c.removePending(id)

	envelope := NewJSONRPCRequestEnvelope(id, method, params)
	if err := c.writeEnvelope(requestCtx, envelope); err != nil {
		return nil, &RequestCallError{Method: method, ID: id, Err: err}
	}

	select {
	case response := <-responseCh:
		if response.err != nil {
			return nil, &RequestCallError{Method: method, ID: id, Err: response.err}
		}
		if response.response.Error != nil {
			return nil, &RequestCallError{Method: method, ID: id, Err: response.response.Error}
		}
		return cloneRawMessage(response.response.Result), nil
	case <-requestCtx.Done():
		if errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return nil, &RequestTimeoutError{Method: method, ID: id, Timeout: requestTimeout}
		}
		return nil, &RequestCallError{Method: method, ID: id, Err: requestCtx.Err()}
	}
}

func (c *Client) sendNotification(ctx context.Context, method string, params any) error {
	if c == nil || !c.isStarted() {
		return &RequestCallError{Method: method, Err: errors.New("client not started")}
	}

	envelope := NewJSONRPCNotificationEnvelope(method, params)
	if err := c.writeEnvelope(ctx, envelope); err != nil {
		return &RequestCallError{Method: method, Err: err}
	}

	return nil
}

func (c *Client) ReadMessage(ctx context.Context) (ClientMessage, error) {
	if c == nil || !c.isStarted() {
		return ClientMessage{}, io.EOF
	}

	select {
	case <-ctx.Done():
		return ClientMessage{}, ctx.Err()
	case message, ok := <-c.events:
		if ok {
			return message, nil
		}

		readerErr := c.readerError()
		if readerErr == nil || errors.Is(readerErr, io.EOF) {
			return ClientMessage{}, io.EOF
		}
		return ClientMessage{}, readerErr
	}
}

func (c *Client) ForceKill() ProcessStopResult {
	if c == nil || !c.isStarted() || c.process == nil || c.process.Process == nil {
		return ProcessStopResult{Kind: ProcessStopKindNotStarted}
	}

	c.beginShutdown()
	_ = c.closeStdin()

	select {
	case <-c.waitDone:
		c.awaitDrains()
		c.markClosed()
		return ProcessStopResult{
			Kind:            ProcessStopKindAlreadyExited,
			ProcessExitCode: c.processExitCode(),
			Error:           c.waitError(),
		}
	default:
	}

	killErr := c.process.Process.Kill()
	if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
		return ProcessStopResult{
			Kind:            ProcessStopKindCloseFailed,
			ProcessExitCode: c.processExitCode(),
			Error:           killErr,
		}
	}

	<-c.waitDone
	c.awaitDrains()
	c.markClosed()

	kind := ProcessStopKindForcedKill
	if errors.Is(killErr, os.ErrProcessDone) {
		kind = ProcessStopKindAlreadyExited
	}

	return ProcessStopResult{
		Kind:            kind,
		ProcessExitCode: c.processExitCode(),
		Error:           c.waitError(),
	}
}

func (c *Client) Close(_ context.Context) ProcessStopResult {
	if c == nil || !c.isStarted() || c.process == nil || c.process.Process == nil {
		return ProcessStopResult{Kind: ProcessStopKindNotStarted}
	}

	c.beginShutdown()
	closeErr := c.closeStdin()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()

	select {
	case <-c.waitDone:
		c.awaitDrains()
		c.markClosed()
		return c.normalizeCloseResult(closeErr)
	case <-shutdownCtx.Done():
		result := c.ForceKill()
		if result.Kind == ProcessStopKindAlreadyExited {
			return c.normalizeCloseResult(closeErr)
		}
		return result
	}
}

func decodeResult[T any](raw json.RawMessage) (T, error) {
	var result T
	if len(raw) == 0 || string(raw) == "null" {
		return result, nil
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, err
	}
	return result, nil
}

func (c *Client) readLoop() {
	defer close(c.readerDone)

	decoder, err := NewStreamDecoder(c.stdout)
	if err != nil {
		c.finishReader(err)
		return
	}

	for {
		var envelope rawEnvelope
		if err := decoder.Decode(&envelope); err != nil {
			c.finishReader(err)
			return
		}

		if len(envelope.ID) != 0 && (len(envelope.Result) != 0 || envelope.Error != nil || envelope.Method == "") {
			responseID, err := parseRequestID(envelope.ID)
			if err != nil {
				c.finishReader(fmt.Errorf("decode response id: %w", err))
				return
			}

			c.dispatchResponse(JSONRPCResponseEnvelope{
				JSONRPC: envelope.JSONRPC,
				ID:      responseID,
				Result:  cloneRawMessage(envelope.Result),
				Error:   envelope.Error,
			})
			continue
		}

		if envelope.Method == "" {
			c.finishReader(errors.New("json-rpc message missing method"))
			return
		}

		message, err := c.makeClientMessage(envelope)
		if err != nil {
			c.finishReader(err)
			return
		}

		c.events <- message
	}
}

func (c *Client) captureStderr() {
	defer close(c.stderrDone)
	_, _ = io.Copy(&c.stderrText, c.stderr)
}

func (c *Client) waitLoop() {
	defer close(c.waitDone)

	err := c.process.Wait()
	c.waitOnce.Do(func() {
		c.waitErr = err
		c.exitCode = exitCodeFromWait(err)
	})
}

func (c *Client) makeClientMessage(envelope rawEnvelope) (ClientMessage, error) {
	message := ClientMessage{
		Event: JSONRPCEventEnvelope{
			JSONRPC: envelope.JSONRPC,
			Method:  envelope.Method,
			Params:  cloneRawMessage(envelope.Params),
		},
	}

	if envelope.Method == EventItemCompletedAgentMessage {
		var completed AgentMessageCompletedEventParams
		if err := json.Unmarshal(envelope.Params, &completed); err != nil {
			return ClientMessage{}, fmt.Errorf("decode %s event: %w", envelope.Method, err)
		}
		message.AgentMessageCompletedEvent = &completed
	}

	return message, nil
}

func (c *Client) finishReader(err error) {
	c.readerOnce.Do(func() {
		if err == nil {
			err = io.EOF
		}
		c.readerErr = err
		c.failPending(err)
		close(c.events)
	})
}

func (c *Client) failPending(err error) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()

	for _, responseCh := range c.pending {
		select {
		case responseCh <- responseResult{err: err}:
		default:
		}
	}
}

func (c *Client) dispatchResponse(response JSONRPCResponseEnvelope) {
	c.pendingMu.Lock()
	responseCh, ok := c.pending[response.ID]
	c.pendingMu.Unlock()
	if !ok {
		return
	}

	select {
	case responseCh <- responseResult{response: response}:
	default:
	}
}

func (c *Client) removePending(id RequestID) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	delete(c.pending, id)
}

func (c *Client) writeEnvelope(ctx context.Context, envelope any) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if err := c.rejectWrites(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := c.encoder.Encode(envelope); err != nil {
		return fmt.Errorf("write json-rpc message: %w", err)
	}

	return nil
}

func (c *Client) rejectWrites() error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	switch {
	case !c.started:
		return errors.New("client not started")
	case c.closed:
		return errors.New("client closed")
	case c.closing:
		return errors.New("client shutting down")
	default:
		return nil
	}
}

func (c *Client) beginShutdown() {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.closing = true
}

func (c *Client) closeStdin() error {
	c.stateMu.Lock()
	if c.stdinClosed || c.stdin == nil {
		c.stateMu.Unlock()
		return nil
	}
	c.stdinClosed = true
	stdin := c.stdin
	c.stateMu.Unlock()

	err := stdin.Close()
	if err != nil && !errors.Is(err, os.ErrClosed) {
		return err
	}
	return nil
}

func (c *Client) markClosed() {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.closed = true
}

func (c *Client) awaitDrains() {
	<-c.readerDone
	<-c.stderrDone
}

func (c *Client) normalizeCloseResult(closeErr error) ProcessStopResult {
	waitErr := c.waitError()
	exitCode := c.processExitCode()
	if closeErr == nil && waitErr == nil && exitCode == 0 {
		return ProcessStopResult{
			Kind:            ProcessStopKindGraceful,
			ProcessExitCode: exitCode,
		}
	}

	combinedErr := firstError(closeErr, waitErr)
	return ProcessStopResult{
		Kind:            ProcessStopKindCloseFailed,
		ProcessExitCode: exitCode,
		Error:           combinedErr,
	}
}

func (c *Client) isStarted() bool {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	return c.started
}

func (c *Client) readerError() error {
	return c.readerErr
}

func (c *Client) waitError() error {
	return c.waitErr
}

func (c *Client) processExitCode() int {
	return c.exitCode
}

func buildSealedLaunchEnvironment(launchContext AppServerLaunchContext) []string {
	envMap := make(map[string]string, len(launchContext.Environment)+8)
	for _, entry := range os.Environ() {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		if key == "HOME" || key == "CODEX_HOME" {
			continue
		}
		envMap[key] = value
	}

	for key, value := range launchContext.Environment {
		envMap[key] = value
	}

	if launchContext.HomeDir != "" {
		envMap["HOME"] = launchContext.HomeDir
	}
	if launchContext.CodexHomeDir != "" {
		envMap["CODEX_HOME"] = launchContext.CodexHomeDir
	}

	keys := make([]string, 0, len(envMap))
	for key := range envMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+envMap[key])
	}

	return env
}

func parseRequestID(raw json.RawMessage) (RequestID, error) {
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, err
	}

	value, err := number.Int64()
	if err != nil {
		return 0, err
	}

	return RequestID(value), nil
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}

	cloned := make(json.RawMessage, len(raw))
	copy(cloned, raw)
	return cloned
}

func exitCodeFromWait(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return -1
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
