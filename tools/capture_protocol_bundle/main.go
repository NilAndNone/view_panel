package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"view_panel/internal/runtimecontract"
)

const (
	packageName                = "@openai/codex"
	smokePrompt                = "Reply with exactly OK."
	metaTransport              = "sequential-json"
	schemaBundleFilename       = "codex_app_server_protocol.schemas.json"
	terminalCompletionMethod   = "item/completed"
	terminalCompletionItemType = "agentMessage"
	terminalCompletionPhase    = "final_answer"
)

var versionRegexp = regexp.MustCompile(`\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?`)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	observedVersion, err := observedCodexVersion(ctx)
	if err != nil {
		return err
	}
	if observedVersion != runtimecontract.DefaultPinnedCLIVersion {
		return fmt.Errorf(
			"observed codex version %q does not match pinned version %q",
			observedVersion,
			runtimecontract.DefaultPinnedCLIVersion,
		)
	}

	capturedAt := time.Now().UTC().Format(time.RFC3339)
	tempRoot, err := os.MkdirTemp(tempBaseDir(repoRoot), "view-panel-protocol-")
	if err != nil {
		return fmt.Errorf("create temp root: %w", err)
	}
	defer os.RemoveAll(tempRoot)

	schemaOutDir, err := generateSchemaBundle(ctx, tempRoot)
	if err != nil {
		return err
	}

	openapiPath := filepath.Join(schemaOutDir, schemaBundleFilename)
	openapiBytes, err := os.ReadFile(openapiPath)
	if err != nil {
		return fmt.Errorf("read schema bundle %s: %w", openapiPath, err)
	}
	openapiBytes = ensureTrailingNewline(openapiBytes)

	messagesIndex, err := captureMessagesIndex(schemaOutDir, observedVersion, capturedAt)
	if err != nil {
		return err
	}

	smokeRecords, err := captureSmokeTranscript(ctx, observedVersion, capturedAt, tempRoot)
	if err != nil {
		return err
	}

	bundleRoot := filepath.Join(repoRoot, "third_party", "codex-protocol", observedVersion)
	if err := os.MkdirAll(bundleRoot, 0o755); err != nil {
		return fmt.Errorf("create bundle root: %w", err)
	}

	openapiOutPath := filepath.Join(bundleRoot, "openapi.json")
	messagesOutPath := filepath.Join(bundleRoot, "messages.json")
	transcriptOutPath := filepath.Join(bundleRoot, "smoke-transcript.jsonl")
	schemaIndexOutPath := filepath.Join(bundleRoot, "schema-index.json")

	if err := os.WriteFile(openapiOutPath, openapiBytes, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", openapiOutPath, err)
	}
	if err := writeJSON(messagesOutPath, messagesIndex); err != nil {
		return err
	}
	if err := writeJSONL(transcriptOutPath, smokeRecords); err != nil {
		return err
	}

	schemaIndex := map[string]any{
		"bundle_type":             "codex-protocol-schema-index",
		"package":                 packageName,
		"pinned_version":          runtimecontract.DefaultPinnedCLIVersion,
		"observed_cli_version":    observedVersion,
		"captured_at_utc":         capturedAt,
		"generated_from_live_cli": true,
		"schema_capture": map[string]any{
			"command":      []string{"codex", "app-server", "generate-json-schema", "--out", "<temp>"},
			"entrypoint":   schemaBundleFilename,
			"message_index": "messages.json",
		},
		"smoke_capture": map[string]any{
			"command":       []string{"codex", "app-server", "--listen", "stdio://"},
			"transport":     metaTransport,
			"filter":        "minimal-handshake-plus-terminal-agentMessage",
			"smoke_prompt":  smokePrompt,
			"terminal_event": map[string]any{
				"method":    terminalCompletionMethod,
				"item_type": terminalCompletionItemType,
				"phase":     terminalCompletionPhase,
			},
		},
		"artifacts": []map[string]any{
			{
				"path":       "openapi.json",
				"sha256_hex": mustSHA256Hex(openapiOutPath),
			},
			{
				"path":       "messages.json",
				"sha256_hex": mustSHA256Hex(messagesOutPath),
			},
			{
				"path":       "smoke-transcript.jsonl",
				"sha256_hex": mustSHA256Hex(transcriptOutPath),
			},
		},
	}
	if err := writeJSON(schemaIndexOutPath, schemaIndex); err != nil {
		return err
	}

	fmt.Printf("captured live protocol bundle at %s\n", bundleRoot)
	return nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		if isRegularFile(filepath.Join(dir, "go.mod")) && isRegularFile(filepath.Join(dir, "AGENTS.md")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", errors.New("could not locate repository root from current working directory")
}

func observedCodexVersion(ctx context.Context) (string, error) {
	output, err := exec.CommandContext(ctx, "codex", "--version").CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		return "", fmt.Errorf("observe codex version: %w: %s", err, trimmed)
	}

	version := normalizeVersion(trimmed)
	if version == "" {
		return "", fmt.Errorf("could not normalize codex version output %q", trimmed)
	}
	return version, nil
}

func generateSchemaBundle(ctx context.Context, tempRoot string) (string, error) {
	schemaOutDir := filepath.Join(tempRoot, "schema-out")
	cmd := exec.CommandContext(ctx, "codex", "app-server", "generate-json-schema", "--out", schemaOutDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("generate schema bundle: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return schemaOutDir, nil
}

func captureMessagesIndex(schemaOutDir, observedVersion, capturedAt string) (map[string]any, error) {
	requests, err := extractMethodEntries(filepath.Join(schemaOutDir, "ClientRequest.json"))
	if err != nil {
		return nil, err
	}
	clientNotifications, err := extractMethodEntries(filepath.Join(schemaOutDir, "ClientNotification.json"))
	if err != nil {
		return nil, err
	}
	serverNotifications, err := extractMethodEntries(filepath.Join(schemaOutDir, "ServerNotification.json"))
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"bundle_type":              "codex-protocol-message-index",
		"package":                  packageName,
		"pinned_version":           runtimecontract.DefaultPinnedCLIVersion,
		"observed_cli_version":     observedVersion,
		"captured_at_utc":          capturedAt,
		"generated_from_live_cli":  true,
		"requests":                 requests,
		"client_notifications":     clientNotifications,
		"server_notifications":     serverNotifications,
		"terminal_completion": map[string]any{
			"method":    terminalCompletionMethod,
			"item_type": terminalCompletionItemType,
			"phase":     terminalCompletionPhase,
		},
	}, nil
}

func extractMethodEntries(path string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	rawEntries, _ := root["oneOf"].([]any)
	entries := make([]map[string]any, 0, len(rawEntries))
	for _, rawEntry := range rawEntries {
		entryMap, ok := rawEntry.(map[string]any)
		if !ok {
			continue
		}
		properties, ok := entryMap["properties"].(map[string]any)
		if !ok {
			continue
		}
		methodProperty, ok := properties["method"].(map[string]any)
		if !ok {
			continue
		}
		methodValues, ok := methodProperty["enum"].([]any)
		if !ok || len(methodValues) == 0 {
			continue
		}
		method, _ := methodValues[0].(string)
		if method == "" {
			continue
		}
		entry := map[string]any{"method": method}
		if title, _ := entryMap["title"].(string); title != "" {
			entry["title"] = title
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(left, right int) bool {
		return entries[left]["method"].(string) < entries[right]["method"].(string)
	})

	return entries, nil
}

func captureSmokeTranscript(ctx context.Context, observedVersion, capturedAt, tempRoot string) ([]any, error) {
	smokeCWD, err := os.MkdirTemp(tempBaseDir(tempRoot), "view-panel-capture-")
	if err != nil {
		return nil, fmt.Errorf("create smoke cwd: %w", err)
	}
	defer os.RemoveAll(smokeCWD)

	client, err := startAppServer(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	initializeRequest, initializeID, err := client.SendRequest("initialize", map[string]any{
		"clientInfo": map[string]any{
			"name":    "view-panel-capture",
			"version": observedVersion,
		},
	})
	if err != nil {
		return nil, err
	}
	initializeMessages, err := client.ReadUntil(5*time.Second, func(message map[string]any) bool {
		return messageIDEquals(message, initializeID)
	})
	if err != nil {
		return nil, err
	}
	initializeResponse, err := findSuccessfulResponse(initializeMessages, initializeID, "initialize")
	if err != nil {
		return nil, err
	}

	initializedNotification, err := client.SendNotification("initialized", map[string]any{})
	if err != nil {
		return nil, err
	}

	configRequest, configID, err := client.SendRequest("configRequirements/read", map[string]any{})
	if err != nil {
		return nil, err
	}
	configMessages, err := client.ReadUntil(5*time.Second, func(message map[string]any) bool {
		return messageIDEquals(message, configID)
	})
	if err != nil {
		return nil, err
	}
	configResponse, err := findSuccessfulResponse(configMessages, configID, "configRequirements/read")
	if err != nil {
		return nil, err
	}

	threadRequest, threadID, err := client.SendRequest("thread/start", map[string]any{
		"approvalPolicy": "never",
		"cwd":            smokeCWD,
		"ephemeral":      true,
	})
	if err != nil {
		return nil, err
	}
	threadMessages, err := client.ReadUntil(5*time.Second, func(message map[string]any) bool {
		return messageIDEquals(message, threadID)
	})
	if err != nil {
		return nil, err
	}
	threadResponse, err := findSuccessfulResponse(threadMessages, threadID, "thread/start")
	if err != nil {
		return nil, err
	}
	threadIdentifier, err := extractThreadID(threadResponse)
	if err != nil {
		return nil, err
	}

	turnRequest, turnID, err := client.SendRequest("turn/start", map[string]any{
		"threadId": threadIdentifier,
		"input": []map[string]any{
			{
				"type": "text",
				"text": smokePrompt,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	turnMessages, err := client.ReadUntil(20*time.Second, func(message map[string]any) bool {
		return isTerminalAgentCompletion(message)
	})
	if err != nil {
		return nil, err
	}
	turnResponse, err := findSuccessfulResponse(turnMessages, turnID, "turn/start")
	if err != nil {
		return nil, err
	}
	terminalCompletion, err := findTerminalAgentCompletion(turnMessages)
	if err != nil {
		return nil, err
	}

	return []any{
		map[string]any{
			"record_type":             "meta",
			"package":                 packageName,
			"pinned_version":          runtimecontract.DefaultPinnedCLIVersion,
			"observed_cli_version":    observedVersion,
			"captured_at_utc":         capturedAt,
			"generated_from_live_cli": true,
			"transport":               metaTransport,
			"smoke_prompt":            smokePrompt,
			"terminal_completion": map[string]any{
				"method":    terminalCompletionMethod,
				"item_type": terminalCompletionItemType,
				"phase":     terminalCompletionPhase,
			},
		},
		map[string]any{"direction": "client->server", "message": initializeRequest},
		map[string]any{"direction": "server->client", "message": initializeResponse},
		map[string]any{"direction": "client->server", "message": initializedNotification},
		map[string]any{"direction": "client->server", "message": configRequest},
		map[string]any{"direction": "server->client", "message": configResponse},
		map[string]any{"direction": "client->server", "message": threadRequest},
		map[string]any{"direction": "server->client", "message": threadResponse},
		map[string]any{"direction": "client->server", "message": turnRequest},
		map[string]any{"direction": "server->client", "message": turnResponse},
		map[string]any{"direction": "server->client", "message": terminalCompletion},
	}, nil
}

type appServerClient struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	lines      chan appServerLine
	stderr     bytes.Buffer
	nextID     int
}

type appServerLine struct {
	message map[string]any
	err     error
}

func startAppServer(ctx context.Context) (*appServerClient, error) {
	cmd := exec.CommandContext(ctx, "codex", "app-server", "--listen", "stdio://")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open app-server stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open app-server stdout: %w", err)
	}

	client := &appServerClient{
		cmd:    cmd,
		stdin:  stdin,
		lines:  make(chan appServerLine, 16),
		nextID: 1,
	}
	cmd.Stderr = &client.stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start codex app-server: %w", err)
	}

	go client.scan(stdout)
	return client, nil
}

func (c *appServerClient) scan(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var message map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			c.lines <- appServerLine{err: fmt.Errorf("decode app-server line: %w", err)}
			close(c.lines)
			return
		}
		c.lines <- appServerLine{message: message}
	}
	if err := scanner.Err(); err != nil {
		c.lines <- appServerLine{err: fmt.Errorf("scan app-server stdout: %w", err)}
	}
	close(c.lines)
}

func (c *appServerClient) SendRequest(method string, params any) (map[string]any, int, error) {
	id := c.nextID
	c.nextID++

	message := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	if err := c.writeMessage(message); err != nil {
		return nil, 0, err
	}
	return message, id, nil
}

func (c *appServerClient) SendNotification(method string, params any) (map[string]any, error) {
	message := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	if err := c.writeMessage(message); err != nil {
		return nil, err
	}
	return message, nil
}

func (c *appServerClient) writeMessage(message map[string]any) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal %v: %w", message["method"], err)
	}
	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write %v: %w", message["method"], err)
	}
	return nil
}

func (c *appServerClient) ReadUntil(timeout time.Duration, predicate func(map[string]any) bool) ([]map[string]any, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	messages := make([]map[string]any, 0)
	for {
		select {
		case line, ok := <-c.lines:
			if !ok {
				if len(messages) > 0 {
					return messages, errors.New("app-server stdout closed before expected message arrived")
				}
				return nil, fmt.Errorf("app-server stdout closed unexpectedly: %s", strings.TrimSpace(c.stderr.String()))
			}
			if line.err != nil {
				return messages, line.err
			}
			messages = append(messages, line.message)
			if predicate(line.message) {
				return messages, nil
			}
		case <-timer.C:
			return messages, fmt.Errorf("timeout waiting for app-server message: %s", strings.TrimSpace(c.stderr.String()))
		}
	}
}

func (c *appServerClient) Close() {
	if c.stdin != nil {
		_ = c.stdin.Close()
	}

	done := make(chan struct{})
	go func() {
		_ = c.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		<-done
	}
}

func findSuccessfulResponse(messages []map[string]any, requestID int, method string) (map[string]any, error) {
	for _, message := range messages {
		if messageIDEquals(message, requestID) {
			if _, hasError := message["error"]; hasError {
				return nil, fmt.Errorf("%s returned JSON-RPC error response", method)
			}
			if _, hasResult := message["result"]; !hasResult {
				return nil, fmt.Errorf("%s response is missing result", method)
			}
			return message, nil
		}
	}
	return nil, fmt.Errorf("successful %s response with id %d not found", method, requestID)
}

func extractThreadID(response map[string]any) (string, error) {
	result, _ := response["result"].(map[string]any)
	thread, _ := result["thread"].(map[string]any)
	threadID, _ := thread["id"].(string)
	if threadID == "" {
		return "", errors.New("thread/start response does not include thread.id")
	}
	return threadID, nil
}

func findTerminalAgentCompletion(messages []map[string]any) (map[string]any, error) {
	for _, message := range messages {
		if isTerminalAgentCompletion(message) {
			return message, nil
		}
	}
	return nil, errors.New("turn/start stream did not include terminal agentMessage completion")
}

func isTerminalAgentCompletion(message map[string]any) bool {
	method, _ := message["method"].(string)
	if method != terminalCompletionMethod {
		return false
	}

	params, _ := message["params"].(map[string]any)
	item, _ := params["item"].(map[string]any)
	itemType, _ := item["type"].(string)
	if itemType != terminalCompletionItemType {
		return false
	}

	phase, _ := item["phase"].(string)
	return phase == terminalCompletionPhase
}

func messageIDEquals(message map[string]any, want int) bool {
	switch value := message["id"].(type) {
	case float64:
		return int(value) == want
	case int:
		return value == want
	default:
		return false
	}
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	data = ensureTrailingNewline(data)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeJSONL(path string, records []any) error {
	lines := make([][]byte, 0, len(records))
	for _, record := range records {
		line, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("marshal jsonl record for %s: %w", path, err)
		}
		lines = append(lines, line)
	}

	content := bytes.Join(lines, []byte("\n"))
	content = ensureTrailingNewline(content)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func mustSHA256Hex(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	match := versionRegexp.FindString(value)
	if match != "" {
		return match
	}
	return value
}

func ensureTrailingNewline(data []byte) []byte {
	if len(data) == 0 || data[len(data)-1] == '\n' {
		return data
	}
	return append(data, '\n')
}

func tempBaseDir(fallback string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return home
	}
	return fallback
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}
