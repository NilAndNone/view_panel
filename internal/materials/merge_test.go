package materials

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeCanonicalFieldsConcatenatesSupplementaryMaterialsInInputOrder(t *testing.T) {
	merged, err := MergeCanonicalFields([]CanonicalFields{
		{
			RoleplayPrompt:            "r",
			DiscussionQuestion:        "q",
			SupplementaryMaterials:    "a",
			OutputContract:            "o",
			AssumptionsAndConstraints: "c",
		},
		{
			SupplementaryMaterials: "b",
		},
	})
	if err != nil {
		t.Fatalf("MergeCanonicalFields returned error: %v", err)
	}
	if merged.SupplementaryMaterials != "a\n\n"+"b" {
		t.Fatalf("supplementary_materials = %q", merged.SupplementaryMaterials)
	}
}

func TestMergeCanonicalFieldsRejectsConflictingDiscussionQuestion(t *testing.T) {
	_, err := MergeCanonicalFields([]CanonicalFields{
		{DiscussionQuestion: "q1"},
		{DiscussionQuestion: "q2"},
	})
	if err == nil {
		t.Fatalf("expected conflict error")
	}
}

func TestMergeCanonicalFieldsRejectsWhitespaceDriftOnSingleValue(t *testing.T) {
	_, err := MergeCanonicalFields([]CanonicalFields{
		{DiscussionQuestion: "question"},
		{DiscussionQuestion: " question "},
	})
	if err == nil {
		t.Fatalf("expected conflict error")
	}
	if !strings.Contains(err.Error(), "discussion_question") {
		t.Fatalf("expected error to mention discussion_question, got %q", err)
	}
}

func TestMergeCanonicalFieldsRejectsConflictingSingleValueFields(t *testing.T) {
	tests := []struct {
		name  string
		items []CanonicalFields
	}{
		{
			name: "roleplay_prompt",
			items: []CanonicalFields{
				{RoleplayPrompt: "r1"},
				{RoleplayPrompt: "r2"},
			},
		},
		{
			name: "output_contract",
			items: []CanonicalFields{
				{OutputContract: "o1"},
				{OutputContract: "o2"},
			},
		},
		{
			name: "assumptions_and_constraints",
			items: []CanonicalFields{
				{AssumptionsAndConstraints: "c1"},
				{AssumptionsAndConstraints: "c2"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := MergeCanonicalFields(tc.items)
			if err == nil {
				t.Fatalf("expected conflict error")
			}
			if !strings.Contains(err.Error(), tc.name) {
				t.Fatalf("expected error to mention %q, got %q", tc.name, err)
			}
		})
	}
}

func TestMergeCanonicalFieldsIgnoresBlankValues(t *testing.T) {
	merged, err := MergeCanonicalFields([]CanonicalFields{
		{
			RoleplayPrompt:         "roleplay",
			DiscussionQuestion:     "question",
			OutputContract:         "contract",
			SupplementaryMaterials: "first",
		},
		{
			RoleplayPrompt:         " ",
			DiscussionQuestion:     "\n\t",
			OutputContract:         "",
			SupplementaryMaterials: " \n ",
		},
		{
			AssumptionsAndConstraints: "constraints",
			SupplementaryMaterials:    "second",
		},
	})
	if err != nil {
		t.Fatalf("MergeCanonicalFields returned error: %v", err)
	}
	if merged.RoleplayPrompt != "roleplay" {
		t.Fatalf("roleplay_prompt = %q", merged.RoleplayPrompt)
	}
	if merged.DiscussionQuestion != "question" {
		t.Fatalf("discussion_question = %q", merged.DiscussionQuestion)
	}
	if merged.OutputContract != "contract" {
		t.Fatalf("output_contract = %q", merged.OutputContract)
	}
	if merged.AssumptionsAndConstraints != "constraints" {
		t.Fatalf("assumptions_and_constraints = %q", merged.AssumptionsAndConstraints)
	}
	if merged.SupplementaryMaterials != "first\n\nsecond" {
		t.Fatalf("supplementary_materials = %q", merged.SupplementaryMaterials)
	}
}

func TestResolveFilesPreservesInputOrderAndExpandsDirectoriesLexically(t *testing.T) {
	baseDir := t.TempDir()
	firstPath := filepath.Join(baseDir, "first.json")
	dirPath := filepath.Join(baseDir, "dir")
	if err := os.WriteFile(firstPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("mkdir dir: %v", err)
	}
	bPath := filepath.Join(dirPath, "b.json")
	aPath := filepath.Join(dirPath, "a.json")
	if err := os.WriteFile(bPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write b file: %v", err)
	}
	if err := os.WriteFile(aPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write a file: %v", err)
	}

	got, err := ResolveFiles([]string{firstPath, dirPath})
	if err != nil {
		t.Fatalf("ResolveFiles returned error: %v", err)
	}

	want := []string{firstPath, aPath, bPath}
	for index := range want {
		absolute, err := filepath.Abs(want[index])
		if err != nil {
			t.Fatalf("filepath.Abs(%q): %v", want[index], err)
		}
		want[index] = absolute
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("resolved files = %v, want %v", got, want)
	}
}

func TestResolveFilesDeduplicatesOverlappingExpandedPaths(t *testing.T) {
	baseDir := t.TempDir()
	dirPath := filepath.Join(baseDir, "dir")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("mkdir dir: %v", err)
	}
	filePath := filepath.Join(dirPath, "only.json")
	if err := os.WriteFile(filePath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := ResolveFiles([]string{filePath, dirPath})
	if err != nil {
		t.Fatalf("ResolveFiles returned error: %v", err)
	}

	absolute, err := filepath.Abs(filePath)
	if err != nil {
		t.Fatalf("filepath.Abs(%q): %v", filePath, err)
	}
	want := []string{absolute}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("resolved files = %v, want %v", got, want)
	}
}

func TestLoaderLoadFilesMergesMultipleFiles(t *testing.T) {
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
  "discussion_question": "",
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

	loader := Loader{}
	merged, err := loader.LoadFiles([]string{firstPath, secondPath})
	if err != nil {
		t.Fatalf("LoadFiles returned error: %v", err)
	}
	if merged.SupplementaryMaterials != "first\n\nsecond" {
		t.Fatalf("supplementary_materials = %q", merged.SupplementaryMaterials)
	}
}

func TestLoadBytesRejectsNullCanonicalField(t *testing.T) {
	_, err := LoadBytes([]byte(`{
  "roleplay_prompt": "roleplay",
  "discussion_question": null,
  "supplementary_materials": "materials",
  "output_contract": "contract",
  "assumptions_and_constraints": "constraints"
}`))
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "discussion_question") {
		t.Fatalf("expected error to mention discussion_question, got %q", err)
	}
}
