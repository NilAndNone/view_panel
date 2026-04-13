package materials

import (
	"fmt"
	"strings"
)

type mergedSingleValue struct {
	value string
	set   bool
}

func (m *mergedSingleValue) Merge(fieldName, next string) error {
	if strings.TrimSpace(next) == "" {
		return nil
	}
	if !m.set {
		m.value = next
		m.set = true
		return nil
	}
	if m.value != next {
		return fmt.Errorf("conflicting %s across materials files", fieldName)
	}
	return nil
}

// MergeCanonicalFields applies the canonical multi-material merge contract over
// Stage 1 fields.
func MergeCanonicalFields(items []CanonicalFields) (CanonicalFields, error) {
	var merged CanonicalFields
	var roleplayPrompt mergedSingleValue
	var discussionQuestion mergedSingleValue
	var outputContract mergedSingleValue
	var assumptionsAndConstraints mergedSingleValue
	supplementary := make([]string, 0, len(items))

	for _, item := range items {
		if err := roleplayPrompt.Merge(fieldRoleplayPrompt, item.RoleplayPrompt); err != nil {
			return CanonicalFields{}, err
		}
		if err := discussionQuestion.Merge(fieldDiscussionQuestion, item.DiscussionQuestion); err != nil {
			return CanonicalFields{}, err
		}
		if err := outputContract.Merge(fieldOutputContract, item.OutputContract); err != nil {
			return CanonicalFields{}, err
		}
		if err := assumptionsAndConstraints.Merge(fieldAssumptionsConstraints, item.AssumptionsAndConstraints); err != nil {
			return CanonicalFields{}, err
		}

		if strings.TrimSpace(item.SupplementaryMaterials) != "" {
			supplementary = append(supplementary, item.SupplementaryMaterials)
		}
	}

	merged.RoleplayPrompt = roleplayPrompt.value
	merged.DiscussionQuestion = discussionQuestion.value
	merged.SupplementaryMaterials = strings.Join(supplementary, "\n\n")
	merged.OutputContract = outputContract.value
	merged.AssumptionsAndConstraints = assumptionsAndConstraints.value
	return merged, nil
}
