package config

import (
	"path/filepath"
	"strings"
)

const (
	DefaultOutDir               = "."
	DefaultConcurrency          = 1
	DefaultModel                = "gpt-5.3-codex-spark"
	DefaultReviewEnabled        = true
	DefaultWorkerTimeoutMS      = 0
	DefaultMaxAttemptsPerPersona = 1
)

// StartupConfig is the canonical startup contract handed to later runtime layers
// after defaults, normalization, and validation have completed.
type StartupConfig struct {
	Question              string   `json:"question"`
	Materials             []string `json:"materials"`
	PersonaSet            string   `json:"persona_set"`
	OutDir                string   `json:"outdir"`
	Concurrency           int      `json:"concurrency"`
	Model                 string   `json:"model"`
	ReviewEnabled         bool     `json:"review_enabled"`
	WorkerTimeoutMS       int      `json:"worker_timeout_ms"`
	MaxAttemptsPerPersona int      `json:"max_attempts_per_persona"`
	ForbiddenToolNames    []string `json:"forbidden_tool_names"`
}

// RawStartupInput preserves explicit CLI state so validation can distinguish an
// omitted optional field from an explicitly blank value.
type RawStartupInput struct {
	Question                     string
	Materials                    []string
	PersonaSet                   string
	OutDir                       string
	Concurrency                  int
	Model                        string
	ReviewEnabled                bool
	WorkerTimeoutMS              int
	MaxAttemptsPerPersona        int
	ForbiddenToolNames           []string
	QuestionProvided             bool
	MaterialsProvided            bool
	PersonaSetProvided           bool
	OutDirProvided               bool
	ConcurrencyProvided          bool
	ModelProvided                bool
	ReviewEnabledProvided        bool
	WorkerTimeoutMSProvided      bool
	MaxAttemptsPerPersonaProvided bool
	ForbiddenToolNamesProvided   bool
}

// NormalizeStartupConfig applies deterministic trimming, list splitting,
// defaulting, and path canonicalization. Filesystem validation remains owned by
// internal/schema.
func NormalizeStartupConfig(raw RawStartupInput) StartupConfig {
	return StartupConfig{
		Question:              strings.TrimSpace(raw.Question),
		Materials:             normalizeMaterials(raw.Materials),
		PersonaSet:            strings.TrimSpace(raw.PersonaSet),
		OutDir:                normalizeOutDir(raw.OutDir),
		Concurrency:           normalizeConcurrency(raw.Concurrency, raw.ConcurrencyProvided),
		Model:                 normalizeModel(raw.Model, raw.ModelProvided),
		ReviewEnabled:         normalizeReviewEnabled(raw.ReviewEnabled, raw.ReviewEnabledProvided),
		WorkerTimeoutMS:       normalizeWorkerTimeoutMS(raw.WorkerTimeoutMS, raw.WorkerTimeoutMSProvided),
		MaxAttemptsPerPersona: normalizeMaxAttemptsPerPersona(raw.MaxAttemptsPerPersona, raw.MaxAttemptsPerPersonaProvided),
		ForbiddenToolNames:    normalizeForbiddenToolNames(raw.ForbiddenToolNames),
	}
}

func normalizeMaterials(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		parts := strings.Split(value, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				normalized = append(normalized, "")
				continue
			}
			normalized = append(normalized, canonicalizePathLike(part))
		}
	}

	return normalized
}

func normalizeOutDir(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = DefaultOutDir
	}
	return canonicalizePathLike(value)
}

func normalizeConcurrency(value int, provided bool) int {
	if !provided && value == 0 {
		return DefaultConcurrency
	}
	return value
}

func normalizeModel(value string, provided bool) string {
	value = strings.TrimSpace(value)
	if !provided && value == "" {
		return DefaultModel
	}
	return value
}

func normalizeReviewEnabled(value bool, provided bool) bool {
	if !provided {
		return DefaultReviewEnabled
	}
	return value
}

func normalizeWorkerTimeoutMS(value int, provided bool) int {
	if !provided {
		return DefaultWorkerTimeoutMS
	}
	return value
}

func normalizeMaxAttemptsPerPersona(value int, provided bool) int {
	if !provided {
		return DefaultMaxAttemptsPerPersona
	}
	return value
}

func normalizeForbiddenToolNames(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		parts := strings.Split(value, ",")
		for _, part := range parts {
			part = strings.ToLower(strings.TrimSpace(part))
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			normalized = append(normalized, part)
		}
	}

	if len(normalized) == 0 {
		return []string{}
	}
	return normalized
}

func canonicalizePathLike(value string) string {
	cleaned := filepath.Clean(value)
	absolute, err := filepath.Abs(cleaned)
	if err != nil {
		return cleaned
	}
	return absolute
}
