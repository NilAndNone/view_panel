package content

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const bundleManifestFileName = "bundle-manifest.json"

var (
	mkdirTempFunc = os.MkdirTemp
	removeAllFunc = os.RemoveAll
	renameFunc    = os.Rename
)

func WriteBundle(outRoot string, bundle Bundle) error {
	outputRoot := filepath.Clean(strings.TrimSpace(outRoot))
	if outputRoot == "" {
		return fmt.Errorf("bundle output root must not be empty")
	}

	parentDir := filepath.Dir(outputRoot)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("create bundle output parent %q: %w", parentDir, err)
	}

	stagingRoot, err := mkdirTempFunc(parentDir, filepath.Base(outputRoot)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create staged bundle root for %q: %w", outputRoot, err)
	}
	keepStaging := true
	defer func() {
		if keepStaging {
			_ = removeAllFunc(stagingRoot)
		}
	}()

	if err := writeBundleContents(stagingRoot, bundle); err != nil {
		return err
	}

	if err := promoteBundleRoot(stagingRoot, outputRoot); err != nil {
		return err
	}
	keepStaging = false

	return nil
}

func writeBundleContents(outRoot string, bundle Bundle) error {
	if err := writeBundleJSON(bundleOutputPath(outRoot, "runtime/persona-index.json"), bundle.PersonaIndex); err != nil {
		return fmt.Errorf("write runtime persona index: %w", err)
	}

	for _, personaID := range bundle.PersonaOrder {
		persona, ok := bundle.Personas[personaID]
		if !ok {
			return fmt.Errorf("bundle missing compiled persona for %q", personaID)
		}
		if err := writeBundleJSON(bundleOutputPath(outRoot, runtimePersonaArtifact(personaID)), persona.RuntimeDocument()); err != nil {
			return fmt.Errorf("write runtime persona %q: %w", personaID, err)
		}
	}

	for _, materialID := range bundle.MaterialOrder {
		material, ok := bundle.Materials[materialID]
		if !ok {
			return fmt.Errorf("bundle missing compiled material for %q", materialID)
		}
		if err := writeBundleJSON(bundleOutputPath(outRoot, runtimeMaterialArtifact(materialID)), material.RuntimeDocument()); err != nil {
			return fmt.Errorf("write runtime material %q: %w", materialID, err)
		}
	}

	if err := writeBundleJSON(filepath.Join(outRoot, bundleManifestFileName), bundle.Manifest); err != nil {
		return fmt.Errorf("write bundle manifest: %w", err)
	}

	return nil
}

func promoteBundleRoot(stagingRoot, outputRoot string) error {
	if _, err := os.Lstat(outputRoot); err != nil {
		if os.IsNotExist(err) {
			if err := renameFunc(stagingRoot, outputRoot); err != nil {
				return fmt.Errorf("promote staged bundle root %q to %q: %w", stagingRoot, outputRoot, err)
			}
			return nil
		}
		return fmt.Errorf("inspect bundle output root %q: %w", outputRoot, err)
	}

	parentDir := filepath.Dir(outputRoot)
	backupRoot, err := mkdirTempFunc(parentDir, filepath.Base(outputRoot)+".swap-old-*")
	if err != nil {
		return fmt.Errorf("reserve swap path for %q: %w", outputRoot, err)
	}
	if err := removeAllFunc(backupRoot); err != nil {
		return fmt.Errorf("clear reserved swap path %q: %w", backupRoot, err)
	}

	if err := renameFunc(outputRoot, backupRoot); err != nil {
		return fmt.Errorf("move previous bundle root %q aside: %w", outputRoot, err)
	}
	if err := renameFunc(stagingRoot, outputRoot); err != nil {
		restoreErr := renameFunc(backupRoot, outputRoot)
		if restoreErr != nil {
			return fmt.Errorf("promote staged bundle root %q to %q: %w (restore previous bundle failed: %v)", stagingRoot, outputRoot, err, restoreErr)
		}
		return fmt.Errorf("promote staged bundle root %q to %q: %w", stagingRoot, outputRoot, err)
	}
	if err := removeAllFunc(backupRoot); err != nil {
		return fmt.Errorf("remove previous bundle root %q: %w", backupRoot, err)
	}

	return nil
}

func bundleOutputPath(outRoot, relativePath string) string {
	return filepath.Join(outRoot, filepath.FromSlash(relativePath))
}

func writeBundleJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", filepath.Dir(path), err)
	}

	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json for %q: %w", path, err)
	}
	payload = append(payload, '\n')

	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write file %q: %w", path, err)
	}

	return nil
}
