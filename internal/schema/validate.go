package schema

import (
	"fmt"
	"os"
	"strings"

	"view_panel/internal/config"
)

// Diagnostic is the machine-readable startup validation surface consumed by the
// CLI entrypoint.
type Diagnostic struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidateStartupConfig is the single startup validator for the P00 contract.
// It validates only CLI/config startup concerns and intentionally stops before
// any later-stage runtime behavior.
func ValidateStartupConfig(raw config.RawStartupInput, cfg config.StartupConfig) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)

	if cfg.Question == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "required",
			Field:   "question",
			Message: "question is required and must be non-blank",
		})
	}

	if !raw.MaterialsProvided || len(cfg.Materials) == 0 {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "required",
			Field:   "materials",
			Message: "materials is required and must include at least one path",
		})
	} else {
		seen := make(map[string]int, len(cfg.Materials))
		for index, material := range cfg.Materials {
			field := fmt.Sprintf("materials[%d]", index)
			if material == "" {
				diagnostics = append(diagnostics, Diagnostic{
					Code:    "required",
					Field:   field,
					Message: "material path is required and must be non-blank",
				})
				continue
			}

			if firstIndex, exists := seen[material]; exists {
				diagnostics = append(diagnostics, Diagnostic{
					Code:    "duplicate",
					Field:   field,
					Message: fmt.Sprintf("material path duplicates canonical entry materials[%d]", firstIndex),
				})
				continue
			}
			seen[material] = index

			info, err := os.Stat(material)
			if err != nil {
				code := "path_invalid"
				message := fmt.Sprintf("material path %q could not be inspected: %v", material, err)
				if os.IsNotExist(err) {
					code = "path_missing"
					message = fmt.Sprintf("material path %q does not exist", material)
				}
				diagnostics = append(diagnostics, Diagnostic{
					Code:    code,
					Field:   field,
					Message: message,
				})
				continue
			}

			if !info.IsDir() && !info.Mode().IsRegular() {
				diagnostics = append(diagnostics, Diagnostic{
					Code:    "path_kind",
					Field:   field,
					Message: fmt.Sprintf("material path %q must resolve to a file or directory", material),
				})
			}
		}
	}

	if cfg.PersonaSet == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "required",
			Field:   "persona_set",
			Message: "persona_set is required and must be non-blank",
		})
	}

	if cfg.Concurrency < 1 {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "range",
			Field:   "concurrency",
			Message: "concurrency must be >= 1",
		})
	}

	if cfg.WorkerTimeoutMS < 0 {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "range",
			Field:   "worker_timeout_ms",
			Message: "worker_timeout_ms must be >= 0",
		})
	}

	if cfg.MaxAttemptsPerPersona < 1 {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "range",
			Field:   "max_attempts_per_persona",
			Message: "max_attempts_per_persona must be >= 1",
		})
	} else if cfg.MaxAttemptsPerPersona != 1 {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "unsupported_value",
			Field:   "max_attempts_per_persona",
			Message: "max_attempts_per_persona currently supports only 1",
		})
	}

	if raw.ModelProvided && cfg.Model == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "required",
			Field:   "model",
			Message: "model must be non-blank when explicitly provided",
		})
	}

	if strings.TrimSpace(cfg.OutDir) == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "required",
			Field:   "outdir",
			Message: "outdir must resolve to a directory path",
		})
		return diagnostics
	}

	info, err := os.Stat(cfg.OutDir)
	if err != nil {
		if os.IsNotExist(err) {
			if mkdirErr := os.MkdirAll(cfg.OutDir, 0o755); mkdirErr != nil {
				diagnostics = append(diagnostics, Diagnostic{
					Code:    "path_create_failed",
					Field:   "outdir",
					Message: fmt.Sprintf("outdir %q could not be created: %v", cfg.OutDir, mkdirErr),
				})
				return diagnostics
			}
			info, err = os.Stat(cfg.OutDir)
		}
		if err != nil {
			code := "path_invalid"
			message := fmt.Sprintf("outdir %q could not be inspected: %v", cfg.OutDir, err)
			if os.IsNotExist(err) {
				code = "path_missing"
				message = fmt.Sprintf("outdir %q does not exist", cfg.OutDir)
			}
			diagnostics = append(diagnostics, Diagnostic{
				Code:    code,
				Field:   "outdir",
				Message: message,
			})
			return diagnostics
		}
	}

	if !info.IsDir() {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "path_kind",
			Field:   "outdir",
			Message: fmt.Sprintf("outdir %q must resolve to a directory", cfg.OutDir),
		})
	}

	return diagnostics
}
