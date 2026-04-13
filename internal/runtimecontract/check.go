package runtimecontract

import (
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
	ErrBlocked    = errors.New("runtime contract blocked startup")
	versionRegexp = regexp.MustCompile(`\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?`)
)

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
