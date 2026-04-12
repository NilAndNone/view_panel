package persona

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	indexKeyPersonas = "personas"
	entryKeyRef      = "ref"
	entryKeyPath     = "path"
	entryKeyPersona  = "persona_id"
)

// ResolvedPersona is the canonical prepare-stage persona record resolved from
// the persona-set index.
type ResolvedPersona struct {
	PersonaID  string         `json:"persona_id"`
	SourcePath string         `json:"source_path"`
	Fields     map[string]any `json:"fields,omitempty"`
}

// Library resolves the prepare-stage persona set from the configured index.
type Library struct{}

// Resolve reads the configured persona-set index, preserves its declared
// ordering exactly, and resolves persona backing files strictly through the
// referenced paths. Membership is never inferred by scanning runtime/personas.
func (Library) Resolve(indexPath string) ([]ResolvedPersona, error) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("read persona index %q: %w", indexPath, err)
	}

	entries, err := parseIndexEntries(data)
	if err != nil {
		return nil, fmt.Errorf("parse persona index %q: %w", indexPath, err)
	}

	indexDir := filepath.Dir(indexPath)
	backingRoot := filepath.Join(indexDir, "personas")
	backingRootAbs, err := filepath.Abs(backingRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve persona backing root %q: %w", backingRoot, err)
	}

	seenIDs := make(map[string]string, len(entries))
	resolved := make([]ResolvedPersona, 0, len(entries))
	for position, entry := range entries {
		record, err := resolveEntry(indexDir, backingRootAbs, entry)
		if err != nil {
			return nil, fmt.Errorf("resolve persona index entry %d: %w", position, err)
		}
		if priorPath, exists := seenIDs[record.PersonaID]; exists {
			return nil, fmt.Errorf("duplicate persona_id %q resolved from %q and %q", record.PersonaID, priorPath, record.SourcePath)
		}
		seenIDs[record.PersonaID] = record.SourcePath
		resolved = append(resolved, record)
	}

	return resolved, nil
}

type rawIndexEntry struct {
	Ref       string `json:"ref"`
	Path      string `json:"path"`
	PersonaID string `json:"persona_id"`
}

func parseIndexEntries(data []byte) ([]rawIndexEntry, error) {
	var arrayEntries []json.RawMessage
	if err := json.Unmarshal(data, &arrayEntries); err == nil && arrayEntries != nil {
		return decodeIndexEntryArray(arrayEntries)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("persona index must be either an array or an object with a %q array: %w", indexKeyPersonas, err)
	}
	if envelope == nil {
		return nil, fmt.Errorf("persona index must be either an array or an object with a %q array", indexKeyPersonas)
	}
	if len(envelope) != 1 {
		return nil, fmt.Errorf("persona index object must contain exactly one key %q", indexKeyPersonas)
	}

	personasRaw, ok := envelope[indexKeyPersonas]
	if !ok {
		return nil, fmt.Errorf("persona index object missing required key %q", indexKeyPersonas)
	}

	if err := json.Unmarshal(personasRaw, &arrayEntries); err != nil {
		return nil, fmt.Errorf("persona index key %q must be an array: %w", indexKeyPersonas, err)
	}

	return decodeIndexEntryArray(arrayEntries)
}

func decodeIndexEntryArray(arrayEntries []json.RawMessage) ([]rawIndexEntry, error) {
	entries := make([]rawIndexEntry, 0, len(arrayEntries))
	for i, rawEntry := range arrayEntries {
		entry, err := parseIndexEntry(rawEntry)
		if err != nil {
			return nil, fmt.Errorf("invalid index entry %d: %w", i, err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func parseIndexEntry(raw json.RawMessage) (rawIndexEntry, error) {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		if strings.TrimSpace(asString) == "" {
			return rawIndexEntry{}, fmt.Errorf("string reference must not be empty")
		}
		return rawIndexEntry{Ref: asString}, nil
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return rawIndexEntry{}, fmt.Errorf("entry must be a string reference or an object: %w", err)
	}
	if object == nil {
		return rawIndexEntry{}, fmt.Errorf("entry must be an object")
	}

	for key := range object {
		switch key {
		case entryKeyRef, entryKeyPath, entryKeyPersona:
		default:
			return rawIndexEntry{}, fmt.Errorf("unsupported entry key %q", key)
		}
	}

	var entry rawIndexEntry
	if rawValue, ok := object[entryKeyRef]; ok {
		if err := json.Unmarshal(rawValue, &entry.Ref); err != nil {
			return rawIndexEntry{}, fmt.Errorf("entry key %q must be a string: %w", entryKeyRef, err)
		}
	}
	if rawValue, ok := object[entryKeyPath]; ok {
		if err := json.Unmarshal(rawValue, &entry.Path); err != nil {
			return rawIndexEntry{}, fmt.Errorf("entry key %q must be a string: %w", entryKeyPath, err)
		}
	}
	if rawValue, ok := object[entryKeyPersona]; ok {
		if err := json.Unmarshal(rawValue, &entry.PersonaID); err != nil {
			return rawIndexEntry{}, fmt.Errorf("entry key %q must be a string: %w", entryKeyPersona, err)
		}
	}

	refsSet := 0
	if strings.TrimSpace(entry.Ref) != "" {
		refsSet++
	}
	if strings.TrimSpace(entry.Path) != "" {
		refsSet++
	}
	if refsSet != 1 {
		return rawIndexEntry{}, fmt.Errorf("entry must set exactly one of %q or %q", entryKeyRef, entryKeyPath)
	}

	return entry, nil
}

func resolveEntry(indexDir, backingRootAbs string, entry rawIndexEntry) (ResolvedPersona, error) {
	ref := entry.Ref
	if ref == "" {
		ref = entry.Path
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ResolvedPersona{}, fmt.Errorf("persona reference must not be empty")
	}

	candidatePath := ref
	if !filepath.IsAbs(candidatePath) {
		candidatePath = filepath.Join(indexDir, candidatePath)
	}
	candidatePathAbs, err := filepath.Abs(candidatePath)
	if err != nil {
		return ResolvedPersona{}, fmt.Errorf("resolve persona reference %q: %w", ref, err)
	}
	if !pathWithinRoot(backingRootAbs, candidatePathAbs) {
		return ResolvedPersona{}, fmt.Errorf("persona reference %q resolves outside backing root %q", ref, backingRootAbs)
	}

	info, err := os.Stat(candidatePathAbs)
	if err != nil {
		return ResolvedPersona{}, fmt.Errorf("persona reference %q is unresolved: %w", ref, err)
	}
	if info.IsDir() {
		return ResolvedPersona{}, fmt.Errorf("persona reference %q resolved to a directory, not a file", ref)
	}

	data, err := os.ReadFile(candidatePathAbs)
	if err != nil {
		return ResolvedPersona{}, fmt.Errorf("read persona definition %q: %w", candidatePathAbs, err)
	}

	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		return ResolvedPersona{}, fmt.Errorf("decode persona definition %q: %w", candidatePathAbs, err)
	}
	if document == nil {
		return ResolvedPersona{}, fmt.Errorf("decode persona definition %q: top-level value must be an object", candidatePathAbs)
	}

	personaIDRaw, ok := document[entryKeyPersona]
	if !ok {
		return ResolvedPersona{}, fmt.Errorf("persona definition %q missing required field %q", candidatePathAbs, entryKeyPersona)
	}

	var personaID string
	if err := json.Unmarshal(personaIDRaw, &personaID); err != nil {
		return ResolvedPersona{}, fmt.Errorf("persona definition %q field %q must be a string: %w", candidatePathAbs, entryKeyPersona, err)
	}
	if strings.TrimSpace(personaID) == "" {
		return ResolvedPersona{}, fmt.Errorf("persona definition %q field %q must not be empty", candidatePathAbs, entryKeyPersona)
	}

	if expected := strings.TrimSpace(entry.PersonaID); expected != "" && expected != personaID {
		return ResolvedPersona{}, fmt.Errorf("persona definition %q resolved persona_id %q but index expected %q", candidatePathAbs, personaID, expected)
	}

	fields := make(map[string]any, len(document)-1)
	for key, rawValue := range document {
		if key == entryKeyPersona {
			continue
		}
		var decoded any
		if err := json.Unmarshal(rawValue, &decoded); err != nil {
			return ResolvedPersona{}, fmt.Errorf("decode persona definition %q field %q: %w", candidatePathAbs, key, err)
		}
		fields[key] = decoded
	}

	return ResolvedPersona{
		PersonaID:  personaID,
		SourcePath: candidatePathAbs,
		Fields:     fields,
	}, nil
}

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	if relative == "." {
		return true
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
