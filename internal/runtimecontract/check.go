package runtimecontract

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ErrBlocked     = errors.New("runtime contract blocked startup")
	versionRegexp  = regexp.MustCompile(`\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?`)
	sha256Regexp   = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

const pinnedProtocolPackage = "@openai/codex"

type Checker struct {
	ExecutableName            string
	PinnedCLIVersion          string
	PinnedLauncherForm        string
	LauncherForm              string
	ProtocolBundlePath        string
	RepositoryRoot            string
	TransportFramingVerified  bool
	LookPath                  func(string) (string, error)
	ReadVersion               func(string) (string, error)
	PathExists                func(string) bool
	RegularFileExists         func(string) bool
	BundleIsPlaceholder       func(string) bool
	ValidateProtocolBundle    func(string) []string
}

func (c Checker) Check() Result {
	pinnedVersion := normalizeVersion(c.PinnedCLIVersion)
	if pinnedVersion == "" {
		pinnedVersion = DefaultPinnedCLIVersion
	}

	pinnedLauncherForm := normalizeSpace(c.PinnedLauncherForm)
	if pinnedLauncherForm == "" {
		pinnedLauncherForm = CanonicalLauncherForm
	}

	launcherForm := normalizeSpace(c.LauncherForm)
	if launcherForm == "" {
		launcherForm = pinnedLauncherForm
	}

	protocolBundlePath := strings.TrimSpace(c.ProtocolBundlePath)
	if protocolBundlePath == "" {
		protocolBundlePath = DefaultProtocolBundlePath(c.RepositoryRoot, pinnedVersion)
	}

	result := Result{
		PinnedCLIVersion:   pinnedVersion,
		LauncherForm:       launcherForm,
		ProtocolBundlePath: protocolBundlePath,
	}

	lookPath := c.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}

	readVersion := c.ReadVersion
	if readVersion == nil {
		readVersion = defaultReadVersion
	}

	pathExists := c.PathExists
	if pathExists == nil {
		pathExists = defaultPathExists
	}

	regularFileExists := c.RegularFileExists
	if regularFileExists == nil {
		regularFileExists = defaultRegularFileExists
	}

	bundleIsPlaceholder := c.BundleIsPlaceholder
	if bundleIsPlaceholder == nil {
		bundleIsPlaceholder = defaultBundleIsPlaceholder
	}

	validateProtocolBundle := c.ValidateProtocolBundle
	if validateProtocolBundle == nil && c.PathExists == nil && c.RegularFileExists == nil {
		validateProtocolBundle = func(bundlePath string) []string {
			return defaultValidateProtocolBundle(bundlePath, pinnedVersion)
		}
	}

	executableName := strings.TrimSpace(c.ExecutableName)
	if executableName == "" {
		executableName = "codex"
	}

	codexPath, err := lookPath(executableName)
	if err != nil {
		result.BlockingErrors = append(result.BlockingErrors, "codex executable could not be found")
	} else {
		observedVersion, readErr := readVersion(codexPath)
		if readErr != nil {
			result.BlockingErrors = append(result.BlockingErrors, "codex version could not be observed")
		} else {
			result.ObservedCLIVersion = normalizeVersion(observedVersion)
			if result.ObservedCLIVersion == "" {
				result.ObservedCLIVersion = strings.TrimSpace(observedVersion)
			}
			if result.ObservedCLIVersion != pinnedVersion {
				result.BlockingErrors = append(result.BlockingErrors, "observed codex version does not match pinned version")
			}
		}
	}

	bundleExists := false
	bundleComplete := false
	switch {
	case protocolBundlePath == "":
		result.BlockingErrors = append(result.BlockingErrors, "protocol bundle path is not configured")
	default:
		bundleExists = pathExists(protocolBundlePath)
		if !bundleExists {
			result.BlockingErrors = append(result.BlockingErrors, "protocol bundle path is missing")
			break
		}

		missingBundleFiles := missingProtocolBundleFiles(protocolBundlePath, regularFileExists)
		if len(missingBundleFiles) > 0 {
			result.BlockingErrors = append(
				result.BlockingErrors,
				fmt.Sprintf(
					"protocol bundle is incomplete; missing required files: %s",
					strings.Join(missingBundleFiles, ", "),
				),
			)
			break
		}

		bundleComplete = true
	}

	if bundleComplete && validateProtocolBundle != nil {
		result.BlockingErrors = append(result.BlockingErrors, validateProtocolBundle(protocolBundlePath)...)
	}

	if !matchesLauncherForm(pinnedLauncherForm, launcherForm) {
		result.BlockingErrors = append(result.BlockingErrors, "runtime launcher form does not match pinned contract")
	}

	if bundleComplete && bundleIsPlaceholder(protocolBundlePath) {
		result.Warnings = append(result.Warnings, WarningPlaceholderBundle)
	}

	if !c.TransportFramingVerified {
		result.Warnings = append(result.Warnings, WarningSequentialJSONTransport)
	}

	switch {
	case len(result.BlockingErrors) > 0:
		result.Status = StatusBlocked
	case len(result.Warnings) > 0:
		result.Status = StatusWarning
	default:
		result.Status = StatusOK
	}

	return result
}

func defaultReadVersion(executablePath string) (string, error) {
	output, err := exec.Command(executablePath, "--version").CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, trimmed)
	}
	if trimmed == "" {
		return "", errors.New("empty version output")
	}
	return trimmed, nil
}

func defaultPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func defaultRegularFileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

func defaultBundleIsPlaceholder(bundlePath string) bool {
	type placeholderEnvelope struct {
		Placeholder      bool `json:"placeholder"`
		XRuntimeContract struct {
			Placeholder bool `json:"placeholder"`
		} `json:"x-runtime-contract"`
	}

	candidates := []string{
		filepath.Join(bundlePath, "schema-index.json"),
		filepath.Join(bundlePath, "messages.json"),
		filepath.Join(bundlePath, "openapi.json"),
	}

	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}

		var envelope placeholderEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			continue
		}
		if envelope.Placeholder || envelope.XRuntimeContract.Placeholder {
			return true
		}
	}

	return false
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

func normalizeSpace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func matchesLauncherForm(pinnedLauncherForm, launcherForm string) bool {
	pinnedFields := strings.Fields(pinnedLauncherForm)
	launcherFields := strings.Fields(launcherForm)
	if len(launcherFields) < len(pinnedFields) {
		return false
	}

	for index, field := range pinnedFields {
		if launcherFields[index] != field {
			return false
		}
	}

	for _, field := range launcherFields[len(pinnedFields):] {
		if field == "--listen" || strings.HasPrefix(field, "--listen=") {
			return false
		}
	}

	return true
}

func missingProtocolBundleFiles(bundlePath string, regularFileExists func(string) bool) []string {
	missing := make([]string, 0, len(RequiredProtocolBundleFiles))
	for _, name := range RequiredProtocolBundleFiles {
		if regularFileExists(filepath.Join(bundlePath, name)) {
			continue
		}
		missing = append(missing, name)
	}

	return missing
}

type protocolBundleSchemaIndex struct {
	BundleType          string                        `json:"bundle_type"`
	Package             string                        `json:"package"`
	PinnedVersion       string                        `json:"pinned_version"`
	ObservedCLIVersion  string                        `json:"observed_cli_version"`
	GeneratedFromLiveCLI bool                         `json:"generated_from_live_cli"`
	Artifacts           []protocolBundleSchemaArtifact `json:"artifacts"`
}

type protocolBundleSchemaArtifact struct {
	Path      string `json:"path"`
	SHA256Hex string `json:"sha256_hex"`
}

type protocolBundleMessagesIndex struct {
	BundleType          string                         `json:"bundle_type"`
	Package             string                         `json:"package"`
	PinnedVersion       string                         `json:"pinned_version"`
	ObservedCLIVersion  string                         `json:"observed_cli_version"`
	GeneratedFromLiveCLI bool                          `json:"generated_from_live_cli"`
	Requests            []protocolBundleMethodEntry    `json:"requests"`
	ClientNotifications []protocolBundleMethodEntry    `json:"client_notifications"`
	ServerNotifications []protocolBundleMethodEntry    `json:"server_notifications"`
	TerminalCompletion  protocolBundleTerminalCompletion `json:"terminal_completion"`
}

type protocolBundleMethodEntry struct {
	Method string `json:"method"`
}

type protocolBundleTerminalCompletion struct {
	Method   string `json:"method"`
	ItemType string `json:"item_type"`
	Phase    string `json:"phase"`
}

type protocolBundleTranscriptMeta struct {
	RecordType          string                         `json:"record_type"`
	Package             string                         `json:"package"`
	PinnedVersion       string                         `json:"pinned_version"`
	GeneratedFromLiveCLI bool                          `json:"generated_from_live_cli"`
	TerminalCompletion  protocolBundleTerminalCompletion `json:"terminal_completion"`
}

func defaultValidateProtocolBundle(bundlePath string, pinnedVersion string) []string {
	blocking := make([]string, 0)

	schemaIndex, err := readProtocolBundleSchemaIndex(filepath.Join(bundlePath, "schema-index.json"))
	if err != nil {
		return []string{fmt.Sprintf("protocol bundle schema-index.json could not be read: %v", err)}
	}

	if schemaIndex.BundleType != "codex-protocol-schema-index" {
		blocking = append(blocking, "protocol bundle schema-index.json has unexpected bundle_type")
	}
	if schemaIndex.Package != pinnedProtocolPackage {
		blocking = append(blocking, "protocol bundle metadata package does not match pinned package in schema-index.json")
	}
	if !schemaIndex.GeneratedFromLiveCLI {
		blocking = append(blocking, "protocol bundle schema-index.json is not marked as live-captured")
	}
	if normalizeVersion(schemaIndex.PinnedVersion) != normalizeVersion(pinnedVersion) {
		blocking = append(blocking, "protocol bundle metadata version does not match checker pinned version in schema-index.json")
	}
	if normalizeVersion(schemaIndex.ObservedCLIVersion) != normalizeVersion(schemaIndex.PinnedVersion) {
		blocking = append(blocking, "protocol bundle schema-index.json observed version does not match pinned version")
	}

	artifactByPath := make(map[string]protocolBundleSchemaArtifact, len(schemaIndex.Artifacts))
	for _, artifact := range schemaIndex.Artifacts {
		artifactByPath[artifact.Path] = artifact
	}

	for _, name := range []string{"openapi.json", "messages.json", "smoke-transcript.jsonl"} {
		artifact, ok := artifactByPath[name]
		if !ok {
			blocking = append(blocking, fmt.Sprintf("protocol bundle schema-index.json is missing artifact hash for %s", name))
			continue
		}
		if !sha256Regexp.MatchString(artifact.SHA256Hex) {
			blocking = append(blocking, fmt.Sprintf("protocol bundle schema-index.json has invalid sha256 for %s", name))
			continue
		}
		sum, err := fileSHA256Hex(filepath.Join(bundlePath, name))
		if err != nil {
			blocking = append(blocking, fmt.Sprintf("protocol bundle %s could not be hashed: %v", name, err))
			continue
		}
		if sum != artifact.SHA256Hex {
			blocking = append(blocking, fmt.Sprintf("protocol bundle hash mismatch for %s", name))
		}
	}

	messagesIndex, messageErrors := readProtocolBundleMessagesIndex(filepath.Join(bundlePath, "messages.json"), pinnedVersion)
	blocking = append(blocking, messageErrors...)

	if messagesIndex != nil {
		blocking = append(
			blocking,
			validateSmokeTranscript(
				filepath.Join(bundlePath, "smoke-transcript.jsonl"),
				pinnedVersion,
				messagesIndex.TerminalCompletion,
			)...,
		)
	}

	return blocking
}

func readProtocolBundleSchemaIndex(path string) (protocolBundleSchemaIndex, error) {
	var schemaIndex protocolBundleSchemaIndex

	data, err := os.ReadFile(path)
	if err != nil {
		return schemaIndex, err
	}
	if err := json.Unmarshal(data, &schemaIndex); err != nil {
		return schemaIndex, err
	}

	return schemaIndex, nil
}

func readProtocolBundleMessagesIndex(path string, pinnedVersion string) (*protocolBundleMessagesIndex, []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{fmt.Sprintf("protocol bundle messages.json could not be read: %v", err)}
	}

	var messagesIndex protocolBundleMessagesIndex
	if err := json.Unmarshal(data, &messagesIndex); err != nil {
		return nil, []string{fmt.Sprintf("protocol bundle messages.json could not be parsed: %v", err)}
	}

	blocking := make([]string, 0)
	if messagesIndex.BundleType != "codex-protocol-message-index" {
		blocking = append(blocking, "protocol bundle messages.json has unexpected bundle_type")
	}
	if messagesIndex.Package != pinnedProtocolPackage {
		blocking = append(blocking, "protocol bundle metadata package does not match pinned package in messages.json")
	}
	if !messagesIndex.GeneratedFromLiveCLI {
		blocking = append(blocking, "protocol bundle messages.json is not marked as live-captured")
	}
	if normalizeVersion(messagesIndex.PinnedVersion) != normalizeVersion(pinnedVersion) {
		blocking = append(blocking, "protocol bundle metadata version does not match checker pinned version in messages.json")
	}
	if normalizeVersion(messagesIndex.ObservedCLIVersion) != normalizeVersion(messagesIndex.PinnedVersion) {
		blocking = append(blocking, "protocol bundle messages.json observed version does not match pinned version")
	}

	for _, method := range []string{"initialize", "configRequirements/read", "thread/start", "turn/start"} {
		if !containsMethodEntry(messagesIndex.Requests, method) {
			blocking = append(blocking, fmt.Sprintf("protocol bundle messages.json is missing required request method %s", method))
		}
	}
	if !containsMethodEntry(messagesIndex.ClientNotifications, "initialized") {
		blocking = append(blocking, "protocol bundle messages.json is missing required client notification initialized")
	}
	if messagesIndex.TerminalCompletion.Method == "" || messagesIndex.TerminalCompletion.ItemType == "" {
		blocking = append(blocking, "protocol bundle messages.json is missing terminal completion metadata")
	}

	return &messagesIndex, blocking
}

func validateSmokeTranscript(path string, pinnedVersion string, terminalCompletion protocolBundleTerminalCompletion) []string {
	file, err := os.Open(path)
	if err != nil {
		return []string{fmt.Sprintf("protocol bundle smoke transcript could not be read: %v", err)}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	lineNumber := 0
	sawMeta := false
	sawInitialize := false
	sawConfigRequirements := false
	sawThreadStart := false
	sawTurnStart := false
	sawTerminalCompletion := false
	requestStates := map[string]*jsonRPCRequestState{
		"initialize":              {},
		"configRequirements/read": {},
		"thread/start":            {},
		"turn/start":              {},
	}
	requestMethodByID := make(map[string]string)
	blocking := make([]string, 0)

	for scanner.Scan() {
		lineNumber++
		line := scanner.Bytes()

		if lineNumber == 1 {
			var meta protocolBundleTranscriptMeta
			if err := json.Unmarshal(line, &meta); err != nil {
				return []string{fmt.Sprintf("protocol bundle smoke transcript line 1 could not be parsed: %v", err)}
			}
			if meta.RecordType != "meta" {
				blocking = append(blocking, "protocol bundle smoke transcript line 1 is not a meta record")
			}
			if meta.Package != pinnedProtocolPackage {
				blocking = append(blocking, "protocol bundle metadata package does not match pinned package in smoke transcript meta")
			}
			if !meta.GeneratedFromLiveCLI {
				blocking = append(blocking, "protocol bundle smoke transcript meta is not marked as live-captured")
			}
			if normalizeVersion(meta.PinnedVersion) != "" && normalizeVersion(meta.PinnedVersion) != normalizeVersion(pinnedVersion) {
				blocking = append(blocking, "protocol bundle metadata version does not match checker pinned version in smoke transcript meta")
			}
			sawMeta = true
			continue
		}

		var record struct {
			Direction string                 `json:"direction"`
			Message   map[string]any         `json:"message"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			return []string{fmt.Sprintf("protocol bundle smoke transcript line %d could not be parsed: %v", lineNumber, err)}
		}

		method, _ := record.Message["method"].(string)
		if record.Direction == "client->server" {
			switch method {
			case "initialize":
				sawInitialize = true
			case "configRequirements/read":
				sawConfigRequirements = true
			case "thread/start":
				sawThreadStart = true
			case "turn/start":
				sawTurnStart = true
			}
			if state, ok := requestStates[method]; ok {
				state.requestSeen = true
				if idKey, ok := messageIDKey(record.Message); ok {
					requestMethodByID[idKey] = method
				}
			}
		}

		if record.Direction == "server->client" {
			if idKey, ok := messageIDKey(record.Message); ok {
				if method, ok := requestMethodByID[idKey]; ok {
					state := requestStates[method]
					if hasJSONRPCError(record.Message) || !hasJSONRPCResult(record.Message) {
						state.nonSuccessResponseSeen = true
					} else {
						state.successResponseSeen = true
					}
				}
			}
		}

		if record.Direction == "server->client" && matchesTerminalCompletion(record.Message, terminalCompletion) {
			sawTerminalCompletion = true
		}
	}

	if err := scanner.Err(); err != nil {
		return []string{fmt.Sprintf("protocol bundle smoke transcript could not be scanned: %v", err)}
	}

	if !sawMeta {
		blocking = append(blocking, "protocol bundle smoke transcript is missing meta record")
	}
	if !sawInitialize {
		blocking = append(blocking, "protocol bundle smoke transcript is missing initialize request")
	}
	if !sawConfigRequirements {
		blocking = append(blocking, "protocol bundle smoke transcript is missing configRequirements/read request")
	}
	if !sawThreadStart {
		blocking = append(blocking, "protocol bundle smoke transcript is missing thread/start request")
	}
	if !sawTurnStart {
		blocking = append(blocking, "protocol bundle smoke transcript is missing turn/start request")
	}
	if !sawTerminalCompletion {
		blocking = append(blocking, "protocol bundle smoke transcript does not contain terminal agentMessage completion")
	}
	for method, state := range requestStates {
		if !state.requestSeen {
			continue
		}
		if state.successResponseSeen {
			continue
		}
		if state.nonSuccessResponseSeen {
			blocking = append(blocking, fmt.Sprintf("protocol bundle smoke transcript %s response is not a successful JSON-RPC result", method))
			continue
		}
		blocking = append(blocking, fmt.Sprintf("protocol bundle smoke transcript is missing successful %s response", method))
	}

	return blocking
}

type jsonRPCRequestState struct {
	requestSeen            bool
	successResponseSeen    bool
	nonSuccessResponseSeen bool
}

func messageIDKey(message map[string]any) (string, bool) {
	value, ok := message["id"]
	if !ok {
		return "", false
	}

	switch typed := value.(type) {
	case string:
		if typed == "" {
			return "", false
		}
		return "s:" + typed, true
	case float64:
		return fmt.Sprintf("n:%.0f", typed), true
	default:
		return "", false
	}
}

func hasJSONRPCError(message map[string]any) bool {
	_, ok := message["error"]
	return ok
}

func hasJSONRPCResult(message map[string]any) bool {
	_, ok := message["result"]
	return ok
}

func extractNormalizedVersionField(line []byte, field string) (string, bool) {
	var envelope map[string]any
	if err := json.Unmarshal(line, &envelope); err != nil {
		return "", false
	}
	value, _ := envelope[field].(string)
	return normalizeVersion(value), value != ""
}

func containsMethodEntry(entries []protocolBundleMethodEntry, want string) bool {
	for _, entry := range entries {
		if entry.Method == want {
			return true
		}
	}
	return false
}

func matchesTerminalCompletion(message map[string]any, terminalCompletion protocolBundleTerminalCompletion) bool {
	method, _ := message["method"].(string)
	if method != terminalCompletion.Method {
		return false
	}

	params, _ := message["params"].(map[string]any)
	item, _ := params["item"].(map[string]any)
	itemType, _ := item["type"].(string)
	if itemType != terminalCompletion.ItemType {
		return false
	}

	if terminalCompletion.Phase == "" {
		return true
	}
	phase, _ := item["phase"].(string)
	return phase == terminalCompletion.Phase
}

func fileSHA256Hex(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
