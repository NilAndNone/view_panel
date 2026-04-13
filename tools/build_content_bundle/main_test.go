package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildContentBundleWritesRuntimeOutputs(t *testing.T) {
	outDir := t.TempDir()
	sourceRoot := testBuilderSourceRoot()

	if err := runBuildContentBundle(sourceRoot, outDir, "default"); err != nil {
		t.Fatalf("runBuildContentBundle returned error: %v", err)
	}

	requireFile(t, filepath.Join(outDir, "bundle-manifest.json"))
	requireFile(t, filepath.Join(outDir, "runtime", "persona-index.json"))
	requireFile(t, filepath.Join(outDir, "runtime", "personas", "analyst.json"))
	requireFile(t, filepath.Join(outDir, "runtime", "materials", "technology.json"))

	var index struct {
		Personas []string `json:"personas"`
	}
	readJSONFile(t, filepath.Join(outDir, "runtime", "persona-index.json"), &index)
	if len(index.Personas) != 1 || index.Personas[0] != "personas/analyst.json" {
		t.Fatalf("persona_index = %#v, want [\"personas/analyst.json\"]", index.Personas)
	}
}

func TestRunHelpPrintsUsage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runWithIO([]string{"-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runWithIO returned %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	output := stdout.String()
	for _, snippet := range []string{
		"Usage:",
		"Build a runtime-ready content bundle",
		"-source",
		"-out",
		"-bundle-id",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("help output missing %q: %q", snippet, output)
		}
	}
}

func TestRunInvalidArgumentsPrintsUsageToStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runWithIO([]string{"unexpected-positional"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runWithIO returned %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}

	output := stderr.String()
	for _, snippet := range []string{
		"unexpected positional arguments",
		"Usage:",
		"-source",
		"-out",
		"-bundle-id",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("stderr missing %q: %q", snippet, output)
		}
	}
}

func testBuilderSourceRoot() string {
	return filepath.Clean(filepath.Join("..", "..", "testdata", "content", "builder_fixture", "src"))
}

func requireFile(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("%q is a directory, want file", path)
	}
}

func readJSONFile(t *testing.T, path string, target any) {
	t.Helper()

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		t.Fatalf("Unmarshal(%q): %v", path, err)
	}
}
