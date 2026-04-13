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

func TestValidateStartupConfigAllowsMultipleAttempts(t *testing.T) {
	raw := validRawStartupInput(t)
	raw.MaxAttemptsPerPersona = 2
	raw.MaxAttemptsPerPersonaProvided = true

	diagnostics := ValidateStartupConfig(raw, config.NormalizeStartupConfig(raw))
	requireNoDiagnostic(t, diagnostics, "max_attempts_per_persona")
}

func TestValidateStartupConfigRejectsMalformedCanonicalMaterials(t *testing.T) {
	raw := validRawStartupInput(t)
	baseDir := t.TempDir()
	materialPath := filepath.Join(baseDir, "bad_materials.json")
	if err := os.WriteFile(materialPath, []byte(`{"roleplay_prompt": 1}`), 0o644); err != nil {
		t.Fatalf("write malformed materials file: %v", err)
	}
	raw.Materials = []string{materialPath}

	diagnostics := ValidateStartupConfig(raw, config.NormalizeStartupConfig(raw))
	requireDiagnostic(t, diagnostics, "materials", "materials_invalid")
}

func TestValidateStartupConfigRejectsNullCanonicalMaterialsField(t *testing.T) {
	raw := validRawStartupInput(t)
	baseDir := t.TempDir()
	materialPath := filepath.Join(baseDir, "null_materials.json")
	content := `{
  "roleplay_prompt": "roleplay",
  "discussion_question": null,
  "supplementary_materials": "materials",
  "output_contract": "contract",
  "assumptions_and_constraints": "constraints"
}`
	if err := os.WriteFile(materialPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write null materials file: %v", err)
	}
	raw.Materials = []string{materialPath}

	diagnostics := ValidateStartupConfig(raw, config.NormalizeStartupConfig(raw))
	requireDiagnostic(t, diagnostics, "materials", "materials_invalid")
}

func TestValidateStartupConfigRejectsMaterialsWhitespaceDriftConflict(t *testing.T) {
	raw := validRawStartupInput(t)
	baseDir := t.TempDir()
	firstPath := filepath.Join(baseDir, "01.json")
	secondPath := filepath.Join(baseDir, "02.json")
	first := `{
  "roleplay_prompt": "roleplay",
  "discussion_question": "question",
  "supplementary_materials": "first",
  "output_contract": "contract",
  "assumptions_and_constraints": "constraints"
}`
	second := `{
  "roleplay_prompt": "",
  "discussion_question": " question ",
  "supplementary_materials": "second",
  "output_contract": "",
  "assumptions_and_constraints": ""
}`
	if err := os.WriteFile(firstPath, []byte(first), 0o644); err != nil {
		t.Fatalf("write first materials file: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte(second), 0o644); err != nil {
		t.Fatalf("write second materials file: %v", err)
	}
	raw.Materials = []string{firstPath, secondPath}

	diagnostics := ValidateStartupConfig(raw, config.NormalizeStartupConfig(raw))
	requireDiagnostic(t, diagnostics, "materials", "materials_invalid")
}

func validRawStartupInput(t *testing.T) config.RawStartupInput {
	t.Helper()

	baseDir := t.TempDir()
	materialPath := filepath.Join(baseDir, "materials.json")
	content := `{
  "roleplay_prompt": "roleplay",
  "discussion_question": "question",
  "supplementary_materials": "materials",
  "output_contract": "contract",
  "assumptions_and_constraints": "constraints"
}`
	if err := os.WriteFile(materialPath, []byte(content), 0o644); err != nil {
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

func requireNoDiagnostic(t *testing.T, diagnostics []Diagnostic, field string) {
	t.Helper()

	for _, diagnostic := range diagnostics {
		if diagnostic.Field == field {
			t.Fatalf("expected no diagnostics for field=%q, got %v", field, diagnostics)
		}
	}
}
