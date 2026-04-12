package materials

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	fieldRoleplayPrompt           = "roleplay_prompt"
	fieldDiscussionQuestion       = "discussion_question"
	fieldSupplementaryMaterials   = "supplementary_materials"
	fieldOutputContract           = "output_contract"
	fieldAssumptionsConstraints   = "assumptions_and_constraints"
	fieldIncident                 = "incident"
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

// Loader reads the canonical Stage 1 materials fields from disk.
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

	var decoded string
	if err := json.Unmarshal(value, &decoded); err != nil {
		return "", fmt.Errorf("materials payload field %q must be a JSON string: %w", key, err)
	}
	return decoded, nil
}
