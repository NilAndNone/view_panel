package materials

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	fieldRoleplayPrompt         = "roleplay_prompt"
	fieldDiscussionQuestion     = "discussion_question"
	fieldSupplementaryMaterials = "supplementary_materials"
	fieldOutputContract         = "output_contract"
	fieldAssumptionsConstraints = "assumptions_and_constraints"
	fieldIncident               = "incident"
)

// Incident captures the canonical dispatch sections from the upstream incident
// model. The prepare stage treats these five fields as the sole authoritative
// Stage 1 materials inputs.
type Incident struct {
	RoleplayPrompt            string `json:"roleplay_prompt"`
	DiscussionQuestion        string `json:"discussion_question"`
	SupplementaryMaterials    string `json:"supplementary_materials"`
	OutputContract            string `json:"output_contract"`
	AssumptionsAndConstraints string `json:"assumptions_and_constraints"`
}

// CanonicalFields is the deterministic Stage 1 materials model consumed by the
// prepare slice.
type CanonicalFields struct {
	RoleplayPrompt            string `json:"roleplay_prompt"`
	DiscussionQuestion        string `json:"discussion_question"`
	SupplementaryMaterials    string `json:"supplementary_materials"`
	OutputContract            string `json:"output_contract"`
	AssumptionsAndConstraints string `json:"assumptions_and_constraints"`
}

// Loader reads canonical Stage 1 materials files from disk.
type Loader struct{}

// LoadFromIncident performs the plan-mandated one-to-one mapping from the
// upstream incident model into the canonical prepare fields.
func LoadFromIncident(incident Incident) CanonicalFields {
	return CanonicalFields{
		RoleplayPrompt:            incident.RoleplayPrompt,
		DiscussionQuestion:        incident.DiscussionQuestion,
		SupplementaryMaterials:    incident.SupplementaryMaterials,
		OutputContract:            incident.OutputContract,
		AssumptionsAndConstraints: incident.AssumptionsAndConstraints,
	}
}

// LoadFile reads a JSON document from disk and extracts the canonical incident
// fields. The loader accepts either a direct object containing the five fields
// or a broader document with an "incident" object containing those fields.
func (Loader) LoadFile(path string) (CanonicalFields, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CanonicalFields{}, fmt.Errorf("read materials file %q: %w", path, err)
	}
	return LoadBytes(data)
}

// LoadFiles reads multiple canonical materials files and merges them with the
// deterministic multi-material contract.
func (loader Loader) LoadFiles(paths []string) (CanonicalFields, error) {
	loaded := make([]CanonicalFields, 0, len(paths))
	for _, path := range paths {
		fields, err := loader.LoadFile(path)
		if err != nil {
			return CanonicalFields{}, err
		}
		loaded = append(loaded, fields)
	}
	return MergeCanonicalFields(loaded)
}

// LoadBytes extracts the canonical fields from a JSON payload.
func LoadBytes(data []byte) (CanonicalFields, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return CanonicalFields{}, fmt.Errorf("decode materials payload: %w", err)
	}
	if root == nil {
		return CanonicalFields{}, fmt.Errorf("decode materials payload: top-level value must be an object")
	}

	if incidentRaw, ok := root[fieldIncident]; ok {
		var incidentObject map[string]json.RawMessage
		if err := json.Unmarshal(incidentRaw, &incidentObject); err != nil {
			return CanonicalFields{}, fmt.Errorf("decode materials payload %q object: %w", fieldIncident, err)
		}
		if incidentObject == nil {
			return CanonicalFields{}, fmt.Errorf("decode materials payload %q object: top-level value must be an object", fieldIncident)
		}
		return extractCanonicalFields(incidentObject)
	}

	return extractCanonicalFields(root)
}

// ResolveFiles expands explicit material inputs into the deterministic file
// list consumed by startup and prepare. Explicit files preserve CLI order;
// directories expand to regular files in lexical order.
func ResolveFiles(paths []string) ([]string, error) {
	candidates := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, input := range paths {
		if strings.TrimSpace(input) == "" {
			return nil, fmt.Errorf("materials path must not be empty")
		}

		info, err := os.Stat(input)
		if err != nil {
			return nil, fmt.Errorf("inspect materials path %q: %w", input, err)
		}

		if info.IsDir() {
			files, err := collectMaterialFiles(input)
			if err != nil {
				return nil, fmt.Errorf("expand materials directory %q: %w", input, err)
			}
			for _, file := range files {
				if _, ok := seen[file]; ok {
					continue
				}
				seen[file] = struct{}{}
				candidates = append(candidates, file)
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("materials path %q must resolve to a regular file or directory", input)
		}

		absolute, err := filepath.Abs(input)
		if err != nil {
			return nil, fmt.Errorf("canonicalize materials path %q: %w", input, err)
		}
		if _, ok := seen[absolute]; ok {
			continue
		}
		seen[absolute] = struct{}{}
		candidates = append(candidates, absolute)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no materials file candidates found")
	}

	return candidates, nil
}

func collectMaterialFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		files = append(files, absolute)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func extractCanonicalFields(raw map[string]json.RawMessage) (CanonicalFields, error) {
	roleplayPrompt, err := readRequiredString(raw, fieldRoleplayPrompt)
	if err != nil {
		return CanonicalFields{}, err
	}
	discussionQuestion, err := readRequiredString(raw, fieldDiscussionQuestion)
	if err != nil {
		return CanonicalFields{}, err
	}
	supplementaryMaterials, err := readRequiredString(raw, fieldSupplementaryMaterials)
	if err != nil {
		return CanonicalFields{}, err
	}
	outputContract, err := readRequiredString(raw, fieldOutputContract)
	if err != nil {
		return CanonicalFields{}, err
	}
	assumptionsAndConstraints, err := readRequiredString(raw, fieldAssumptionsConstraints)
	if err != nil {
		return CanonicalFields{}, err
	}

	return CanonicalFields{
		RoleplayPrompt:            roleplayPrompt,
		DiscussionQuestion:        discussionQuestion,
		SupplementaryMaterials:    supplementaryMaterials,
		OutputContract:            outputContract,
		AssumptionsAndConstraints: assumptionsAndConstraints,
	}, nil
}

func readRequiredString(raw map[string]json.RawMessage, key string) (string, error) {
	value, ok := raw[key]
	if !ok {
		return "", fmt.Errorf("materials payload missing required field %q", key)
	}

	var decoded *string
	if err := json.Unmarshal(value, &decoded); err != nil {
		return "", fmt.Errorf("materials payload field %q must be a JSON string: %w", key, err)
	}
	if decoded == nil {
		return "", fmt.Errorf("materials payload field %q must be a JSON string and must not be null", key)
	}
	return *decoded, nil
}
