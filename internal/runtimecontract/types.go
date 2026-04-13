package runtimecontract

import (
	"path/filepath"
	"strings"
)

type Status string

const (
	StatusOK      Status = "ok"
	StatusWarning Status = "warning"
	StatusBlocked Status = "blocked"
)

const (
	DefaultPinnedCLIVersion       = "0.0.0"
	CanonicalLauncherForm         = "codex app-server --listen stdio://"
	WarningPlaceholderBundle      = "protocol bundle is placeholder content"
	WarningSequentialJSONTransport = "transport framing is not strongly verified beyond sequential JSON assumption"
)

var RequiredProtocolBundleFiles = []string{
	"openapi.json",
	"messages.json",
	"schema-index.json",
	"smoke-transcript.jsonl",
}

type Result struct {
	Status             Status   `json:"status"`
	ObservedCLIVersion string   `json:"observed_cli_version"`
	PinnedCLIVersion   string   `json:"pinned_cli_version"`
	LauncherForm       string   `json:"launcher_form"`
	ProtocolBundlePath string   `json:"protocol_bundle_path"`
	BlockingErrors     []string `json:"blocking_errors"`
	Warnings           []string `json:"warnings"`
}

func DefaultProtocolBundlePath(repositoryRoot, version string) string {
	repositoryRoot = strings.TrimSpace(repositoryRoot)
	if repositoryRoot == "" {
		return ""
	}
	version = normalizeVersion(version)
	if version == "" {
		version = DefaultPinnedCLIVersion
	}
	return filepath.Join(repositoryRoot, "third_party", "codex-protocol", version)
}
