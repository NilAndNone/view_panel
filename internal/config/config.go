package config

import (
	"path/filepath"
	"strings"
)

const (
	DefaultOutDir      = "."
	DefaultConcurrency = 1
	DefaultModel       = "gpt-5.3-codex-spark"
)

// StartupConfig is the canonical startup contract handed to later runtime layers
// after defaults, normalization, and validation have completed.
type StartupConfig struct {
	Question    string   `json:"question"`
	Materials   []string `json:"materials"`
	PersonaSet  string   `json:"persona_set"`
	OutDir      string   `json:"outdir"`
	Concurrency int      `json:"concurrency"`
	Model       string   `json:"model"`
}

// RawStartupInput preserves explicit CLI state so validation can distinguish an
// omitted optional field from an explicitly blank value.
type RawStartupInput struct {
	Question            string
	Materials           []string
	PersonaSet          string
	OutDir              string
	Concurrency         int
	Model               string
	QuestionProvided    bool
	MaterialsProvided   bool
	PersonaSetProvided  bool
	OutDirProvided      bool
	ConcurrencyProvided bool
	ModelProvided       bool
}

// NormalizeStartupConfig applies deterministic trimming, list splitting,
// defaulting, and path canonicalization. Filesystem validation remains owned by
// internal/schema.
func NormalizeStartupConfig(raw RawStartupInput) StartupConfig {
	return StartupConfig{
		Question:    strings.TrimSpace(raw.Question),
		Materials:   normalizeMaterials(raw.Materials),
		PersonaSet:  strings.TrimSpace(raw.PersonaSet),
		OutDir:      normalizeOutDir(raw.OutDir),
		Concurrency: normalizeConcurrency(raw.Concurrency, raw.ConcurrencyProvided),
		Model:       normalizeModel(raw.Model, raw.ModelProvided),
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

func canonicalizePathLike(value string) string {
	cleaned := filepath.Clean(value)
	absolute, err := filepath.Abs(cleaned)
	if err != nil {
		return cleaned
	}
	return absolute
}
