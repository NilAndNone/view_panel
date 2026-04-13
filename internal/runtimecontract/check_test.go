package runtimecontract

import (
	"errors"
	"os"
	"path/filepath"
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
