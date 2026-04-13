package content

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteBundleWritesRuntimeBundleArtifacts(t *testing.T) {
	catalog, err := LoadCatalog(testBuilderSourceRoot())
	if err != nil {
		t.Fatalf("LoadCatalog returned error: %v", err)
	}

	bundle, err := CompileBundle(catalog, CompileOptions{BundleID: "default"})
	if err != nil {
		t.Fatalf("CompileBundle returned error: %v", err)
	}

	outDir := t.TempDir()
	if err := WriteBundle(outDir, bundle); err != nil {
		t.Fatalf("WriteBundle returned error: %v", err)
	}

	requireFile(t, filepath.Join(outDir, "bundle-manifest.json"))
	requireFile(t, filepath.Join(outDir, "runtime", "persona-index.json"))
	requireFile(t, filepath.Join(outDir, "runtime", "personas", "analyst.json"))
	requireFile(t, filepath.Join(outDir, "runtime", "materials", "technology.json"))

	var manifest BundleManifest
	readJSONFile(t, filepath.Join(outDir, "bundle-manifest.json"), &manifest)
	if manifest.BundleID != "default" {
		t.Fatalf("bundle_id = %q, want default", manifest.BundleID)
	}

	var index RuntimePersonaIndex
	readJSONFile(t, filepath.Join(outDir, "runtime", "persona-index.json"), &index)
	if len(index.Personas) != 1 || index.Personas[0] != "personas/analyst.json" {
		t.Fatalf("persona_index = %#v, want [\"personas/analyst.json\"]", index.Personas)
	}

	var persona map[string]string
	readJSONFile(t, filepath.Join(outDir, "runtime", "personas", "analyst.json"), &persona)
	if persona["persona_id"] != "analyst" {
		t.Fatalf("persona_id = %q, want analyst", persona["persona_id"])
	}
	if persona["profile_long"] == "" {
		t.Fatalf("expected profile_long in compiled persona output")
	}

	var material map[string]string
	readJSONFile(t, filepath.Join(outDir, "runtime", "materials", "technology.json"), &material)
	if material["roleplay_prompt"] == "" {
		t.Fatalf("expected roleplay_prompt in compiled material output")
	}
}

func TestWriteBundleReplacesExistingOutputWithoutLeavingStaleArtifacts(t *testing.T) {
	catalog, err := LoadCatalog(testBuilderSourceRoot())
	if err != nil {
		t.Fatalf("LoadCatalog returned error: %v", err)
	}

	firstBundle, err := CompileBundle(catalog, CompileOptions{BundleID: "first"})
	if err != nil {
		t.Fatalf("CompileBundle returned error: %v", err)
	}

	outDir := filepath.Join(t.TempDir(), "bundle")
	if err := WriteBundle(outDir, firstBundle); err != nil {
		t.Fatalf("WriteBundle returned error: %v", err)
	}

	stalePath := filepath.Join(outDir, "runtime", "personas", "stale.json")
	writeTextFile(t, stalePath, "{\n  \"persona_id\": \"stale\"\n}\n")

	secondBundle, err := CompileBundle(catalog, CompileOptions{BundleID: "second"})
	if err != nil {
		t.Fatalf("CompileBundle returned error: %v", err)
	}
	if err := WriteBundle(outDir, secondBundle); err != nil {
		t.Fatalf("WriteBundle returned error: %v", err)
	}

	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("stale artifact still present after rewrite: %v", err)
	}

	var manifest BundleManifest
	readJSONFile(t, filepath.Join(outDir, "bundle-manifest.json"), &manifest)
	if manifest.BundleID != "second" {
		t.Fatalf("bundle_id = %q, want second", manifest.BundleID)
	}
}

func TestWriteBundleKeepsPreviousOutputWhenStagingFails(t *testing.T) {
	catalog, err := LoadCatalog(testBuilderSourceRoot())
	if err != nil {
		t.Fatalf("LoadCatalog returned error: %v", err)
	}

	previousBundle, err := CompileBundle(catalog, CompileOptions{BundleID: "previous"})
	if err != nil {
		t.Fatalf("CompileBundle returned error: %v", err)
	}

	parentDir := t.TempDir()
	outDir := filepath.Join(parentDir, "bundle")
	if err := WriteBundle(outDir, previousBundle); err != nil {
		t.Fatalf("WriteBundle returned error: %v", err)
	}

	originalMkdirTemp := mkdirTempFunc
	mkdirTempFunc = func(dir, pattern string) (string, error) {
		return "", errors.New("forced staging failure")
	}
	defer func() {
		mkdirTempFunc = originalMkdirTemp
	}()

	nextBundle, err := CompileBundle(catalog, CompileOptions{BundleID: "next"})
	if err != nil {
		t.Fatalf("CompileBundle returned error: %v", err)
	}

	err = WriteBundle(outDir, nextBundle)
	if err == nil {
		t.Fatalf("expected WriteBundle to fail when staging root cannot be created")
	}

	var manifest BundleManifest
	readJSONFile(t, filepath.Join(outDir, "bundle-manifest.json"), &manifest)
	if manifest.BundleID != "previous" {
		t.Fatalf("bundle_id = %q, want previous", manifest.BundleID)
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
