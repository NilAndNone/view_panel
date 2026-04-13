package schema

import (
	"os"
	"path/filepath"
	"testing"

	"view_panel/internal/config"
)

func TestNormalizeStartupConfigDefaultsPolicyKnobs(t *testing.T) {
	cfg := config.NormalizeStartupConfig(config.RawStartupInput{})

	if !cfg.ReviewEnabled {
		t.Fatalf("review_enabled should default to true")
	}
	if cfg.WorkerTimeoutMS != 0 {
		t.Fatalf("worker_timeout_ms should default to 0, got %d", cfg.WorkerTimeoutMS)
	}
	if cfg.MaxAttemptsPerPersona != 1 {
		t.Fatalf("max_attempts_per_persona should default to 1, got %d", cfg.MaxAttemptsPerPersona)
	}
	if len(cfg.ForbiddenToolNames) != 0 {
		t.Fatalf("forbidden_tool_names should default to empty, got %v", cfg.ForbiddenToolNames)
	}
}

func TestNormalizeStartupConfigNormalizesForbiddenToolNames(t *testing.T) {
	cfg := config.NormalizeStartupConfig(config.RawStartupInput{
		ForbiddenToolNames: []string{" Shell , Web ", "web", "", "shell,git"},
	})

	got := cfg.ForbiddenToolNames
	want := []string{"shell", "web", "git"}
	if len(got) != len(want) {
		t.Fatalf("expected %v forbidden tool names, got %v", want, got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected forbidden tool names %v, got %v", want, got)
		}
	}
}

func TestValidateStartupConfigRejectsNegativeWorkerTimeout(t *testing.T) {
	raw := validRawStartupInput(t)
	raw.WorkerTimeoutMS = -1
	raw.WorkerTimeoutMSProvided = true

	diagnostics := ValidateStartupConfig(raw, config.NormalizeStartupConfig(raw))
	requireDiagnostic(t, diagnostics, "worker_timeout_ms", "range")
}

func TestValidateStartupConfigRejectsNonPositiveMaxAttempts(t *testing.T) {
	raw := validRawStartupInput(t)
	raw.MaxAttemptsPerPersona = 0
	raw.MaxAttemptsPerPersonaProvided = true

	diagnostics := ValidateStartupConfig(raw, config.NormalizeStartupConfig(raw))
	requireDiagnostic(t, diagnostics, "max_attempts_per_persona", "range")
}

func TestValidateStartupConfigRejectsUnsupportedMaxAttempts(t *testing.T) {
	raw := validRawStartupInput(t)
	raw.MaxAttemptsPerPersona = 2
	raw.MaxAttemptsPerPersonaProvided = true

	diagnostics := ValidateStartupConfig(raw, config.NormalizeStartupConfig(raw))
	requireDiagnostic(t, diagnostics, "max_attempts_per_persona", "unsupported_value")
}

func validRawStartupInput(t *testing.T) config.RawStartupInput {
	t.Helper()

	baseDir := t.TempDir()
	materialPath := filepath.Join(baseDir, "materials.txt")
	if err := os.WriteFile(materialPath, []byte("materials"), 0o644); err != nil {
		t.Fatalf("write materials file: %v", err)
	}

	return config.RawStartupInput{
		Question:            "What should happen?",
		Materials:           []string{materialPath},
		PersonaSet:          "default",
		OutDir:              filepath.Join(baseDir, "out"),
		Concurrency:         1,
		QuestionProvided:    true,
		MaterialsProvided:   true,
		PersonaSetProvided:  true,
		OutDirProvided:      true,
		ConcurrencyProvided: true,
	}
}

func requireDiagnostic(t *testing.T, diagnostics []Diagnostic, field, code string) {
	t.Helper()

	for _, diagnostic := range diagnostics {
		if diagnostic.Field == field && diagnostic.Code == code {
			return
		}
	}

	t.Fatalf("expected diagnostic field=%q code=%q, got %v", field, code, diagnostics)
}
