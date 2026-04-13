package config

import "testing"

func TestNormalizeStartupConfigDefaultsForbiddenToolNamesToEmptySlice(t *testing.T) {
	cfg := NormalizeStartupConfig(RawStartupInput{})

	if cfg.ForbiddenToolNames == nil {
		t.Fatalf("expected empty forbidden_tool_names slice, got nil")
	}
	if len(cfg.ForbiddenToolNames) != 0 {
		t.Fatalf("expected empty forbidden_tool_names slice, got %v", cfg.ForbiddenToolNames)
	}
}

func TestNormalizeStartupConfigKeepsEmptyForbiddenToolNamesAsEmptySlice(t *testing.T) {
	cfg := NormalizeStartupConfig(RawStartupInput{
		ForbiddenToolNames:         []string{"", " , "},
		ForbiddenToolNamesProvided: true,
	})

	if cfg.ForbiddenToolNames == nil {
		t.Fatalf("expected empty forbidden_tool_names slice, got nil")
	}
	if len(cfg.ForbiddenToolNames) != 0 {
		t.Fatalf("expected empty forbidden_tool_names slice, got %v", cfg.ForbiddenToolNames)
	}
}
