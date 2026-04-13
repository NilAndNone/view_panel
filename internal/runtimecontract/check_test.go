package runtimecontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckBlocksWhenExecutableCannotBeFound(t *testing.T) {
	checker := Checker{
		PinnedCLIVersion:         "0.0.0",
		LauncherForm:             CanonicalLauncherForm,
		ProtocolBundlePath:       filepath.Join("third_party", "codex-protocol", "0.0.0"),
		LookPath:                 func(string) (string, error) { return "", errors.New("missing") },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusBlocked {
		t.Fatalf("expected blocked, got %q", result.Status)
	}
	if !contains(result.BlockingErrors, "codex executable could not be found") {
		t.Fatalf("expected missing executable error, got %v", result.BlockingErrors)
	}
}

func TestCheckBlocksWhenVersionCannotBeObserved(t *testing.T) {
	checker := Checker{
		PinnedCLIVersion:         "0.0.0",
		LauncherForm:             CanonicalLauncherForm,
		ProtocolBundlePath:       filepath.Join("third_party", "codex-protocol", "0.0.0"),
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return "", errors.New("boom") },
		PathExists:               func(string) bool { return true },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusBlocked {
		t.Fatalf("expected blocked, got %q", result.Status)
	}
	if !contains(result.BlockingErrors, "codex version could not be observed") {
		t.Fatalf("expected version observability error, got %v", result.BlockingErrors)
	}
}

func TestCheckBlocksOnVersionMismatch(t *testing.T) {
	checker := Checker{
		PinnedCLIVersion:         "0.0.0",
		LauncherForm:             CanonicalLauncherForm,
		ProtocolBundlePath:       filepath.Join("third_party", "codex-protocol", "0.0.0"),
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return "codex-cli 0.0.1", nil },
		PathExists:               func(string) bool { return true },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusBlocked {
		t.Fatalf("expected blocked, got %q", result.Status)
	}
	if got, want := result.ObservedCLIVersion, "0.0.1"; got != want {
		t.Fatalf("expected observed version %q, got %q", want, got)
	}
	if !contains(result.BlockingErrors, "observed codex version does not match pinned version") {
		t.Fatalf("expected version mismatch error, got %v", result.BlockingErrors)
	}
}

func TestCheckBlocksOnMissingProtocolBundle(t *testing.T) {
	checker := Checker{
		PinnedCLIVersion:         "0.0.0",
		LauncherForm:             CanonicalLauncherForm,
		ProtocolBundlePath:       filepath.Join("third_party", "codex-protocol", "0.0.0"),
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return "0.0.0", nil },
		PathExists:               func(string) bool { return false },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusBlocked {
		t.Fatalf("expected blocked, got %q", result.Status)
	}
	if !contains(result.BlockingErrors, "protocol bundle path is missing") {
		t.Fatalf("expected missing bundle error, got %v", result.BlockingErrors)
	}
}

func TestCheckBlocksOnLauncherMismatch(t *testing.T) {
	checker := Checker{
		PinnedCLIVersion:         "0.0.0",
		LauncherForm:             "codex app-server --listen tcp://127.0.0.1:1234",
		ProtocolBundlePath:       filepath.Join("third_party", "codex-protocol", "0.0.0"),
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return "0.0.0", nil },
		PathExists:               func(string) bool { return true },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusBlocked {
		t.Fatalf("expected blocked, got %q", result.Status)
	}
	if !contains(result.BlockingErrors, "runtime launcher form does not match pinned contract") {
		t.Fatalf("expected launcher mismatch error, got %v", result.BlockingErrors)
	}
}

func TestCheckAllowsCanonicalLauncherWithSuffixArgs(t *testing.T) {
	checker := Checker{
		PinnedCLIVersion:         "0.0.0",
		LauncherForm:             "codex app-server --listen stdio:// --model gpt-5",
		ProtocolBundlePath:       filepath.Join("third_party", "codex-protocol", "0.0.0"),
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return "0.0.0", nil },
		PathExists:               func(string) bool { return true },
		RegularFileExists:        func(string) bool { return true },
		BundleIsPlaceholder:      func(string) bool { return false },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusOK {
		t.Fatalf("expected ok, got %q with blocking errors %v", result.Status, result.BlockingErrors)
	}
}

func TestCheckBlocksWhenSuffixOverridesPinnedListenFlag(t *testing.T) {
	checker := Checker{
		PinnedCLIVersion:         "0.0.0",
		LauncherForm:             "codex app-server --listen stdio:// --listen tcp://127.0.0.1:1234",
		ProtocolBundlePath:       filepath.Join("third_party", "codex-protocol", "0.0.0"),
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return "0.0.0", nil },
		PathExists:               func(string) bool { return true },
		RegularFileExists:        func(string) bool { return true },
		BundleIsPlaceholder:      func(string) bool { return false },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusBlocked {
		t.Fatalf("expected blocked, got %q", result.Status)
	}
	if !contains(result.BlockingErrors, "runtime launcher form does not match pinned contract") {
		t.Fatalf("expected launcher mismatch error, got %v", result.BlockingErrors)
	}
}

func TestCheckBlocksWhenProtocolBundlePathIsUnconfigured(t *testing.T) {
	checker := Checker{
		PinnedCLIVersion:         "0.0.0",
		LauncherForm:             CanonicalLauncherForm,
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return "0.0.0", nil },
		PathExists:               func(string) bool { return true },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusBlocked {
		t.Fatalf("expected blocked, got %q", result.Status)
	}
	if result.ProtocolBundlePath != "" {
		t.Fatalf("expected empty bundle path, got %q", result.ProtocolBundlePath)
	}
	if !contains(result.BlockingErrors, "protocol bundle path is not configured") {
		t.Fatalf("expected unconfigured bundle path error, got %v", result.BlockingErrors)
	}
}

func TestCheckUsesRepositoryRootForDefaultProtocolBundlePath(t *testing.T) {
	expectedBundlePath := filepath.Join("/repo", "third_party", "codex-protocol", "0.0.0")
	existingPaths := map[string]bool{
		expectedBundlePath: true,
	}
	for _, name := range RequiredProtocolBundleFiles {
		existingPaths[filepath.Join(expectedBundlePath, name)] = true
	}

	checker := Checker{
		PinnedCLIVersion: "0.0.0",
		RepositoryRoot:   "/repo",
		LauncherForm:     CanonicalLauncherForm,
		LookPath:         func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:      func(string) (string, error) { return "0.0.0", nil },
		PathExists: func(path string) bool {
			return existingPaths[path]
		},
		RegularFileExists: func(path string) bool {
			return existingPaths[path]
		},
		BundleIsPlaceholder:      func(string) bool { return false },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.ProtocolBundlePath != expectedBundlePath {
		t.Fatalf("expected bundle path %q, got %q", expectedBundlePath, result.ProtocolBundlePath)
	}
	if result.Status != StatusOK {
		t.Fatalf("expected ok, got %q with blocking errors %v", result.Status, result.BlockingErrors)
	}
}

func TestCheckBlocksOnIncompleteProtocolBundle(t *testing.T) {
	bundlePath := filepath.Join("third_party", "codex-protocol", "0.0.0")
	existingPaths := map[string]bool{
		bundlePath:                                        true,
		filepath.Join(bundlePath, "openapi.json"):        true,
		filepath.Join(bundlePath, "messages.json"):       true,
		filepath.Join(bundlePath, "schema-index.json"):   true,
	}

	checker := Checker{
		PinnedCLIVersion:         "0.0.0",
		LauncherForm:             CanonicalLauncherForm,
		ProtocolBundlePath:       bundlePath,
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return "0.0.0", nil },
		PathExists:               func(path string) bool { return existingPaths[path] },
		RegularFileExists:        func(path string) bool { return existingPaths[path] },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusBlocked {
		t.Fatalf("expected blocked, got %q", result.Status)
	}
	if !contains(result.BlockingErrors, "protocol bundle is incomplete; missing required files: smoke-transcript.jsonl") {
		t.Fatalf("expected incomplete bundle error, got %v", result.BlockingErrors)
	}
}

func TestCheckBlocksWhenRequiredBundleEntryIsDirectory(t *testing.T) {
	bundleRoot := t.TempDir()
	for _, name := range []string{"messages.json", "schema-index.json", "smoke-transcript.jsonl"} {
		if err := os.WriteFile(filepath.Join(bundleRoot, name), []byte("{}"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(bundleRoot, "openapi.json"), 0o755); err != nil {
		t.Fatalf("mkdir openapi.json: %v", err)
	}

	checker := Checker{
		PinnedCLIVersion:         "0.0.0",
		LauncherForm:             CanonicalLauncherForm,
		ProtocolBundlePath:       bundleRoot,
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return "0.0.0", nil },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusBlocked {
		t.Fatalf("expected blocked, got %q", result.Status)
	}
	if !contains(result.BlockingErrors, "protocol bundle is incomplete; missing required files: openapi.json") {
		t.Fatalf("expected incomplete bundle error, got %v", result.BlockingErrors)
	}
}

func TestCheckWarnsOnPlaceholderBundleAndSequentialJSONTransport(t *testing.T) {
	checker := Checker{
		PinnedCLIVersion:   "0.0.0",
		LauncherForm:       CanonicalLauncherForm,
		ProtocolBundlePath: filepath.Join("third_party", "codex-protocol", "0.0.0"),
		LookPath:           func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:        func(string) (string, error) { return "codex-cli 0.0.0", nil },
		PathExists:         func(string) bool { return true },
		RegularFileExists:  func(string) bool { return true },
		BundleIsPlaceholder: func(string) bool {
			return true
		},
	}

	result := checker.Check()

	if result.Status != StatusWarning {
		t.Fatalf("expected warning, got %q", result.Status)
	}
	if got, want := result.ObservedCLIVersion, "0.0.0"; got != want {
		t.Fatalf("expected observed version %q, got %q", want, got)
	}
	if !contains(result.Warnings, WarningPlaceholderBundle) {
		t.Fatalf("expected placeholder warning, got %v", result.Warnings)
	}
	if !contains(result.Warnings, WarningSequentialJSONTransport) {
		t.Fatalf("expected transport warning, got %v", result.Warnings)
	}
}

func TestCheckCanReturnOK(t *testing.T) {
	checker := Checker{
		PinnedCLIVersion:         "0.0.0",
		LauncherForm:             CanonicalLauncherForm,
		ProtocolBundlePath:       filepath.Join("third_party", "codex-protocol", "0.0.0"),
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return "0.0.0", nil },
		PathExists:               func(string) bool { return true },
		RegularFileExists:        func(string) bool { return true },
		BundleIsPlaceholder:      func(string) bool { return false },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusOK {
		t.Fatalf("expected ok, got %q", result.Status)
	}
	if len(result.BlockingErrors) != 0 {
		t.Fatalf("expected no blocking errors, got %v", result.BlockingErrors)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", result.Warnings)
	}
}

func TestCheckDoesNotWarnWhenBundleIsLiveCaptured(t *testing.T) {
	bundlePath := writeLiveBundleFixture(t, liveBundleFixtureOptions{
		includeTerminalAgentMessage: true,
	})

	checker := Checker{
		PinnedCLIVersion:         DefaultPinnedCLIVersion,
		LauncherForm:             CanonicalLauncherForm,
		ProtocolBundlePath:       bundlePath,
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return DefaultPinnedCLIVersion, nil },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusOK {
		t.Fatalf("expected ok, got %q with blocking errors %v and warnings %v", result.Status, result.BlockingErrors, result.Warnings)
	}
	if contains(result.Warnings, WarningPlaceholderBundle) {
		t.Fatalf("did not expect placeholder warning for live bundle: %v", result.Warnings)
	}
}

func TestCheckBlocksWhenLiveBundleTranscriptLacksTerminalAgentMessage(t *testing.T) {
	bundlePath := writeLiveBundleFixture(t, liveBundleFixtureOptions{
		includeTerminalAgentMessage: false,
	})

	checker := Checker{
		PinnedCLIVersion:         DefaultPinnedCLIVersion,
		LauncherForm:             CanonicalLauncherForm,
		ProtocolBundlePath:       bundlePath,
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return DefaultPinnedCLIVersion, nil },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusBlocked {
		t.Fatalf("expected blocked, got %q", result.Status)
	}
	if !containsSubstring(result.BlockingErrors, "smoke transcript does not contain terminal agentMessage completion") {
		t.Fatalf("expected terminal completion error, got %v", result.BlockingErrors)
	}
}

func TestCheckBlocksWhenLiveBundleMetadataVersionDriftsFromPinnedVersion(t *testing.T) {
	bundlePath := writeLiveBundleFixture(t, liveBundleFixtureOptions{
		includeTerminalAgentMessage: true,
		bundleMetadataVersion:       "0.0.1",
	})

	checker := Checker{
		PinnedCLIVersion:         DefaultPinnedCLIVersion,
		LauncherForm:             CanonicalLauncherForm,
		ProtocolBundlePath:       bundlePath,
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return DefaultPinnedCLIVersion, nil },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusBlocked {
		t.Fatalf("expected blocked, got %q", result.Status)
	}
	if !containsSubstring(result.BlockingErrors, "bundle metadata version does not match checker pinned version") {
		t.Fatalf("expected bundle metadata version drift error, got %v", result.BlockingErrors)
	}
}

func TestCheckBlocksWhenLiveBundlePackageDriftsFromPinnedPackage(t *testing.T) {
	bundlePath := writeLiveBundleFixture(t, liveBundleFixtureOptions{
		includeTerminalAgentMessage: true,
		bundleMetadataPackage:       "@wrong/codex",
	})

	checker := Checker{
		PinnedCLIVersion:         DefaultPinnedCLIVersion,
		LauncherForm:             CanonicalLauncherForm,
		ProtocolBundlePath:       bundlePath,
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return DefaultPinnedCLIVersion, nil },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusBlocked {
		t.Fatalf("expected blocked, got %q", result.Status)
	}
	if !containsSubstring(result.BlockingErrors, "bundle metadata package does not match pinned package") {
		t.Fatalf("expected bundle metadata package drift error, got %v", result.BlockingErrors)
	}
}

func TestCheckBlocksWhenLiveBundleTranscriptUsesErrorEnvelopeForInitialize(t *testing.T) {
	bundlePath := writeLiveBundleFixture(t, liveBundleFixtureOptions{
		includeTerminalAgentMessage: true,
		requestErrors: map[string]bool{
			"initialize": true,
		},
	})

	checker := Checker{
		PinnedCLIVersion:         DefaultPinnedCLIVersion,
		LauncherForm:             CanonicalLauncherForm,
		ProtocolBundlePath:       bundlePath,
		LookPath:                 func(string) (string, error) { return "/usr/bin/codex", nil },
		ReadVersion:              func(string) (string, error) { return DefaultPinnedCLIVersion, nil },
		TransportFramingVerified: true,
	}

	result := checker.Check()

	if result.Status != StatusBlocked {
		t.Fatalf("expected blocked, got %q", result.Status)
	}
	if !containsSubstring(result.BlockingErrors, "initialize response is not a successful JSON-RPC result") {
		t.Fatalf("expected initialize success-semantics error, got %v", result.BlockingErrors)
	}
}

func TestNormalizeVersion(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strips codex cli prefix",
			input: "codex-cli 0.0.0",
			want:  "0.0.0",
		},
		{
			name:  "keeps prerelease and build metadata",
			input: " 1.2.3-beta+build.5 ",
			want:  "1.2.3-beta+build.5",
		},
		{
			name:  "falls back to trimmed input without semver",
			input: " version unknown ",
			want:  "version unknown",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := normalizeVersion(testCase.input); got != testCase.want {
				t.Fatalf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func containsSubstring(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}

type liveBundleFixtureOptions struct {
	includeTerminalAgentMessage bool
	bundleMetadataVersion       string
	bundleMetadataPackage       string
	requestErrors               map[string]bool
}

func writeLiveBundleFixture(t *testing.T, options liveBundleFixtureOptions) string {
	t.Helper()

	bundleRoot := t.TempDir()
	bundleMetadataVersion := options.bundleMetadataVersion
	if bundleMetadataVersion == "" {
		bundleMetadataVersion = DefaultPinnedCLIVersion
	}
	bundleMetadataPackage := options.bundleMetadataPackage
	if bundleMetadataPackage == "" {
		bundleMetadataPackage = "@openai/codex"
	}
	openapiPath := filepath.Join(bundleRoot, "openapi.json")
	messagesPath := filepath.Join(bundleRoot, "messages.json")
	transcriptPath := filepath.Join(bundleRoot, "smoke-transcript.jsonl")
	schemaIndexPath := filepath.Join(bundleRoot, "schema-index.json")

	writeJSONFile(t, openapiPath, map[string]any{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"title":   "Captured protocol schema",
		"type":    "object",
		"generated_from_live_cli": true,
	})

	writeJSONFile(t, messagesPath, map[string]any{
		"bundle_type":           "codex-protocol-message-index",
		"package":               bundleMetadataPackage,
		"pinned_version":        bundleMetadataVersion,
		"observed_cli_version":  bundleMetadataVersion,
		"captured_at_utc":       "2026-04-13T00:00:00Z",
		"generated_from_live_cli": true,
		"requests": []map[string]any{
			{"method": "initialize"},
			{"method": "configRequirements/read"},
			{"method": "thread/start"},
			{"method": "turn/start"},
		},
		"client_notifications": []map[string]any{
			{"method": "initialized"},
		},
		"server_notifications": []map[string]any{
			{"method": "thread/started"},
			{"method": "turn/started"},
			{"method": "item/completed"},
		},
		"terminal_completion": map[string]any{
			"method":    "item/completed",
			"item_type": "agentMessage",
			"phase":     "final_answer",
		},
	})

	transcriptRecords := []map[string]any{
		{
			"record_type":            "meta",
			"package":                bundleMetadataPackage,
			"pinned_version":         bundleMetadataVersion,
			"observed_cli_version":   bundleMetadataVersion,
			"captured_at_utc":        "2026-04-13T00:00:00Z",
			"generated_from_live_cli": true,
			"transport":              "sequential-json",
			"terminal_completion": map[string]any{
				"method":    "item/completed",
				"item_type": "agentMessage",
				"phase":     "final_answer",
			},
		},
		{
			"direction": "client->server",
			"message": map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "initialize",
				"params": map[string]any{
					"clientInfo": map[string]any{
						"name":    "view-panel-capture",
						"version": DefaultPinnedCLIVersion,
					},
				},
			},
		},
		{
			"direction": "server->client",
			"message": successOrErrorResponse(
				1,
				"initialize",
				options.requestErrors,
				map[string]any{
					"userAgent":      "view-panel-capture/0.0.0",
					"platformFamily": "unix",
					"platformOs":     "android",
				},
			),
		},
		{
			"direction": "client->server",
			"message": map[string]any{
				"jsonrpc": "2.0",
				"method":  "initialized",
				"params":  map[string]any{},
			},
		},
		{
			"direction": "client->server",
			"message": map[string]any{
				"jsonrpc": "2.0",
				"id":      2,
				"method":  "configRequirements/read",
				"params":  map[string]any{},
			},
		},
		{
			"direction": "server->client",
			"message": successOrErrorResponse(
				2,
				"configRequirements/read",
				options.requestErrors,
				map[string]any{
					"requirements": nil,
				},
			),
		},
		{
			"direction": "client->server",
			"message": map[string]any{
				"jsonrpc": "2.0",
				"id":      3,
				"method":  "thread/start",
				"params": map[string]any{
					"approvalPolicy": "never",
					"cwd":            "/tmp/view-panel-capture",
				},
			},
		},
		{
			"direction": "server->client",
			"message": successOrErrorResponse(
				3,
				"thread/start",
				options.requestErrors,
				map[string]any{
					"thread": map[string]any{
						"id":  "thread-live",
						"cwd": "/tmp/view-panel-capture",
					},
				},
			),
		},
		{
			"direction": "client->server",
			"message": map[string]any{
				"jsonrpc": "2.0",
				"id":      4,
				"method":  "turn/start",
				"params": map[string]any{
					"threadId": "thread-live",
					"input": []map[string]any{
						{
							"type": "text",
							"text": "Reply with exactly OK.",
						},
					},
				},
			},
		},
		{
			"direction": "server->client",
			"message": successOrErrorResponse(
				4,
				"turn/start",
				options.requestErrors,
				map[string]any{
					"turn": map[string]any{
						"id":     "turn-live",
						"status": "inProgress",
					},
				},
			),
		},
	}

	if options.includeTerminalAgentMessage {
		transcriptRecords = append(transcriptRecords, map[string]any{
			"direction": "server->client",
			"message": map[string]any{
				"method": "item/completed",
				"params": map[string]any{
					"threadId": "thread-live",
					"turnId":   "turn-live",
					"item": map[string]any{
						"type":  "agentMessage",
						"id":    "agent-live",
						"text":  "OK",
						"phase": "final_answer",
					},
				},
			},
		})
	}

	writeJSONLFile(t, transcriptPath, transcriptRecords)

	writeJSONFile(t, schemaIndexPath, map[string]any{
		"bundle_type":            "codex-protocol-schema-index",
		"package":                bundleMetadataPackage,
		"pinned_version":         bundleMetadataVersion,
		"observed_cli_version":   bundleMetadataVersion,
		"captured_at_utc":        "2026-04-13T00:00:00Z",
		"generated_from_live_cli": true,
		"artifacts": []map[string]any{
			{"path": "openapi.json", "sha256_hex": fixtureSHA256Hex(t, openapiPath)},
			{"path": "messages.json", "sha256_hex": fixtureSHA256Hex(t, messagesPath)},
			{"path": "smoke-transcript.jsonl", "sha256_hex": fixtureSHA256Hex(t, transcriptPath)},
		},
	})

	return bundleRoot
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeJSONLFile(t *testing.T, path string, records []map[string]any) {
	t.Helper()

	lines := make([][]byte, 0, len(records))
	for _, record := range records {
		line, err := json.Marshal(record)
		if err != nil {
			t.Fatalf("marshal jsonl %s: %v", path, err)
		}
		lines = append(lines, line)
	}
	if err := os.WriteFile(path, append(bytesJoin(lines, []byte("\n")), '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func fixtureSHA256Hex(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func bytesJoin(parts [][]byte, sep []byte) []byte {
	if len(parts) == 0 {
		return nil
	}
	size := 0
	for _, part := range parts {
		size += len(part)
	}
	size += len(sep) * (len(parts) - 1)
	joined := make([]byte, 0, size)
	for index, part := range parts {
		if index > 0 {
			joined = append(joined, sep...)
		}
		joined = append(joined, part...)
	}
	return joined
}

func successOrErrorResponse(id int, method string, requestErrors map[string]bool, result map[string]any) map[string]any {
	if requestErrors != nil && requestErrors[method] {
		return map[string]any{
			"id": id,
			"error": map[string]any{
				"code":    -32603,
				"message": method + " failed",
			},
		}
	}

	return map[string]any{
		"id":     id,
		"result": result,
	}
}
