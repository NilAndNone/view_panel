package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	panelhash "view_panel/internal/hash"
)

const (
	runsDirName       = "runs"
	requestDirName    = "request"
	prepareDirName    = "01_prepare"
	answerDirName     = "02_answer"
	renderDirName     = "03_render"
	auditDirName      = "audit"
	personasDirName   = "personas"
	requestLogFile    = "log.jsonl"
	auditEventsFile   = "events.jsonl"
	auditErrorsFile   = "errors.log"
	defaultDirMode    = 0o755
	defaultFileMode   = 0o644
)

var (
	ErrRelativePathRequired = errors.New("relative path is required")
	ErrPathEscape           = errors.New("resolved path escapes base directory")
)

// RunRoot returns the canonical run root for a run under outdir.
func RunRoot(outdir, runID string) string {
	return filepath.Join(outdir, runsDirName, runID)
}

// RequestDir returns the request-area directory for a run.
func RequestDir(runRoot string) string {
	return filepath.Join(runRoot, requestDirName)
}

// PrepareRoot returns the prepare-stage root for a run.
func PrepareRoot(runRoot string) string {
	return filepath.Join(runRoot, prepareDirName)
}

// AnswerRoot returns the answer-stage root for a run.
func AnswerRoot(runRoot string) string {
	return filepath.Join(runRoot, answerDirName)
}

// RenderRoot returns the render-stage root for a run.
func RenderRoot(runRoot string) string {
	return filepath.Join(runRoot, renderDirName)
}

// AuditRoot returns the audit root for a run.
func AuditRoot(runRoot string) string {
	return filepath.Join(runRoot, auditDirName)
}

// PreparePersonaDir returns the canonical prepare persona directory.
func PreparePersonaDir(runRoot, personaID string) string {
	return filepath.Join(PrepareRoot(runRoot), personasDirName, personaID)
}

// AnswerPersonaDir returns the canonical answer persona directory.
func AnswerPersonaDir(runRoot, personaID string) string {
	return filepath.Join(AnswerRoot(runRoot), personasDirName, personaID)
}

// PreparePersonaArtifactPath returns a boundary-checked artifact path under the
// prepare persona directory.
func PreparePersonaArtifactPath(runRoot, personaID, artifactName string) (string, error) {
	return ResolvePath(PreparePersonaDir(runRoot, personaID), artifactName)
}

// AnswerPersonaArtifactPath returns a boundary-checked artifact path under the
// answer persona directory.
func AnswerPersonaArtifactPath(runRoot, personaID, artifactName string) (string, error) {
	return ResolvePath(AnswerPersonaDir(runRoot, personaID), artifactName)
}

// RequestLogPath returns the canonical request log path.
func RequestLogPath(runRoot string) string {
	return filepath.Join(RequestDir(runRoot), requestLogFile)
}

// AuditEventsPath returns the canonical audit events JSONL path.
func AuditEventsPath(runRoot string) string {
	return filepath.Join(AuditRoot(runRoot), auditEventsFile)
}

// AuditErrorsPath returns the canonical audit error log path.
func AuditErrorsPath(runRoot string) string {
	return filepath.Join(AuditRoot(runRoot), auditErrorsFile)
}

// ResolvePath joins rel under base after lexical boundary checks and rejects any
// existing segment under base that is a symlink.
func ResolvePath(base, rel string) (string, error) {
	baseAbs, err := filepath.Abs(filepath.Clean(base))
	if err != nil {
		return "", fmt.Errorf("canonicalize base path: %w", err)
	}

	if err := validateBasePath(baseAbs); err != nil {
		return "", err
	}

	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", ErrRelativePathRequired
	}
	if volume := filepath.VolumeName(rel); volume != "" {
		return "", fmt.Errorf("relative path %q must not include volume name %q", rel, volume)
	}

	cleaned := filepath.Clean(rel)
	if filepath.IsAbs(cleaned) || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: %q", ErrPathEscape, rel)
	}

	target := filepath.Join(baseAbs, cleaned)
	if !pathWithinBase(target, baseAbs) {
		return "", fmt.Errorf("%w: %q", ErrPathEscape, rel)
	}
	if err := validateExistingSegments(baseAbs, cleaned); err != nil {
		return "", err
	}

	return target, nil
}

// EnsureRunLayout is the single directory materializer for the canonical run
// layout. Downstream stages must not create ad hoc run_root paths themselves.
func EnsureRunLayout(runRoot string) error {
	directories := []string{
		runRoot,
		RequestDir(runRoot),
		PrepareRoot(runRoot),
		filepath.Join(PrepareRoot(runRoot), personasDirName),
		AnswerRoot(runRoot),
		filepath.Join(AnswerRoot(runRoot), personasDirName),
		RenderRoot(runRoot),
		AuditRoot(runRoot),
	}

	for _, directory := range directories {
		if err := os.MkdirAll(directory, defaultDirMode); err != nil {
			return fmt.Errorf("ensure run layout directory %q: %w", directory, err)
		}
	}

	if err := ensureRegularStub(AuditEventsPath(runRoot)); err != nil {
		return err
	}
	if err := ensureRegularStub(AuditErrorsPath(runRoot)); err != nil {
		return err
	}

	return nil
}

// WriteJSON writes canonical JSON atomically and shares its byte contract with
// hash.SHA256HexJSON.
func WriteJSON(path string, value any) error {
	payload, err := panelhash.CanonicalJSONBytes(value)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, payload, defaultFileMode)
}

// WriteText writes plain text atomically with no implicit trailing newline.
func WriteText(path, content string) error {
	return atomicWriteFile(path, []byte(content), defaultFileMode)
}

// AppendJSONL appends exactly one canonical JSON object plus one trailing
// newline without rewriting prior content.
func AppendJSONL(path string, value any) error {
	payload, err := panelhash.CanonicalJSONBytes(value)
	if err != nil {
		return err
	}
	if len(payload) == 0 || payload[0] != '{' {
		return fmt.Errorf("append jsonl requires a top-level JSON object")
	}
	payload = append(payload, '\n')

	target, err := prepareWriteTarget(path)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_APPEND, defaultFileMode)
	if err != nil {
		return fmt.Errorf("open jsonl append target %q: %w", target, err)
	}
	defer file.Close()

	if _, err := file.Write(payload); err != nil {
		return fmt.Errorf("append jsonl payload to %q: %w", target, err)
	}

	return nil
}

func validateBasePath(base string) error {
	info, err := os.Lstat(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect base path %q: %w", base, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("base path %q must not be a symlink", base)
	}
	if !info.IsDir() {
		return fmt.Errorf("base path %q must be a directory", base)
	}
	return nil
}

func validateExistingSegments(base, rel string) error {
	current := base
	for _, segment := range strings.Split(rel, string(os.PathSeparator)) {
		if segment == "" {
			continue
		}
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("inspect path segment %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path segment %q must not be a symlink", current)
		}
	}
	return nil
}

func pathWithinBase(target, base string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func ensureRegularStub(path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("layout stub %q must not be a symlink", path)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("layout stub %q must be a regular file", path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("inspect layout stub %q: %w", path, err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, defaultFileMode)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return fmt.Errorf("create layout stub %q: %w", path, err)
	}
	return file.Close()
}

func prepareWriteTarget(path string) (string, error) {
	target, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("canonicalize write target: %w", err)
	}

	if runRoot, ok := inferRunRoot(target); ok {
		if err := EnsureRunLayout(runRoot); err != nil {
			return "", err
		}
	}

	if err := os.MkdirAll(filepath.Dir(target), defaultDirMode); err != nil {
		return "", fmt.Errorf("ensure write target parent %q: %w", filepath.Dir(target), err)
	}

	return target, nil
}

func inferRunRoot(path string) (string, bool) {
	cleaned := filepath.Clean(path)
	parts := strings.Split(cleaned, string(os.PathSeparator))
	for index := 0; index+1 < len(parts); index++ {
		if parts[index] != runsDirName || parts[index+1] == "" {
			continue
		}
		if index == 0 {
			return string(os.PathSeparator) + filepath.Join(parts[1:index+2]...), true
		}
		return filepath.Join(parts[:index+2]...), true
	}
	return "", false
}

func atomicWriteFile(path string, payload []byte, mode os.FileMode) error {
	target, err := prepareWriteTarget(path)
	if err != nil {
		return err
	}

	directory := filepath.Dir(target)
	tempFile, err := os.CreateTemp(directory, "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file for %q: %w", target, err)
	}
	tempName := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempName)
	}()

	if _, err := tempFile.Write(payload); err != nil {
		return fmt.Errorf("write temp file for %q: %w", target, err)
	}
	if err := tempFile.Chmod(mode); err != nil {
		return fmt.Errorf("chmod temp file for %q: %w", target, err)
	}
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("sync temp file for %q: %w", target, err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp file for %q: %w", target, err)
	}
	if err := os.Rename(tempName, target); err != nil {
		return fmt.Errorf("replace %q atomically: %w", target, err)
	}

	tempName = ""
	return nil
}
