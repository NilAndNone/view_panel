package content

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"view_panel/internal/materials"
)

const contentBundleManifestSchemaVersion = "content_bundle_manifest_v1"

type CompileOptions struct {
	BundleID string
}

type Bundle struct {
	BundleID      string
	PersonaIndex  RuntimePersonaIndex
	PersonaOrder  []string
	Personas      map[string]CompiledPersona
	MaterialOrder []string
	Materials     map[string]CompiledMaterial
	Manifest      BundleManifest
}

type RuntimePersonaIndex struct {
	Personas []string `json:"personas"`
}

type CompiledPersona struct {
	PersonaID string            `json:"persona_id"`
	Fields    map[string]string `json:"fields"`
}

type CompiledMaterial struct {
	MaterialID string                    `json:"material_id"`
	Fields     materials.CanonicalFields `json:"fields"`
}

type BundleManifest struct {
	SchemaVersion string   `json:"schema_version"`
	BundleID      string   `json:"bundle_id"`
	PersonaCount  int      `json:"persona_count"`
	MaterialCount int      `json:"material_count"`
	Artifacts     []string `json:"artifacts"`
}

func CompileBundle(catalog Catalog, options CompileOptions) (Bundle, error) {
	bundleID := strings.TrimSpace(options.BundleID)
	if bundleID == "" {
		return Bundle{}, fmt.Errorf("bundle_id must not be empty")
	}
	if len(catalog.PersonaOrder) == 0 {
		return Bundle{}, fmt.Errorf("catalog must contain at least one persona")
	}
	if len(catalog.MaterialOrder) == 0 {
		return Bundle{}, fmt.Errorf("catalog must contain at least one material")
	}

	bundle := Bundle{
		BundleID:      bundleID,
		PersonaOrder:  append([]string(nil), catalog.PersonaOrder...),
		Personas:      make(map[string]CompiledPersona, len(catalog.PersonaOrder)),
		MaterialOrder: append([]string(nil), catalog.MaterialOrder...),
		Materials:     make(map[string]CompiledMaterial, len(catalog.MaterialOrder)),
	}

	personaRefs := make([]string, 0, len(catalog.PersonaOrder))
	artifacts := []string{"runtime/persona-index.json"}
	for _, personaID := range catalog.PersonaOrder {
		source, ok := catalog.Personas[personaID]
		if !ok {
			return Bundle{}, fmt.Errorf("catalog missing persona source for %q", personaID)
		}
		bundle.Personas[personaID] = compilePersona(source)
		personaRefs = append(personaRefs, runtimePersonaRef(personaID))
		artifacts = append(artifacts, runtimePersonaArtifact(personaID))
	}

	for _, materialID := range catalog.MaterialOrder {
		source, ok := catalog.Materials[materialID]
		if !ok {
			return Bundle{}, fmt.Errorf("catalog missing material source for %q", materialID)
		}
		compiledMaterial := compileMaterial(source)
		if err := validateCompiledMaterial(compiledMaterial, bundle.Personas, bundle.PersonaOrder); err != nil {
			return Bundle{}, fmt.Errorf("compile material %q: %w", materialID, err)
		}
		bundle.Materials[materialID] = compiledMaterial
		artifacts = append(artifacts, runtimeMaterialArtifact(materialID))
	}

	bundle.PersonaIndex = RuntimePersonaIndex{Personas: personaRefs}
	bundle.Manifest = BundleManifest{
		SchemaVersion: contentBundleManifestSchemaVersion,
		BundleID:      bundleID,
		PersonaCount:  len(bundle.PersonaOrder),
		MaterialCount: len(bundle.MaterialOrder),
		Artifacts:     artifacts,
	}

	return bundle, nil
}

func compilePersona(source PersonaSource) CompiledPersona {
	fields := map[string]string{
		"display_name":       source.Meta.DisplayName,
		"core_lens":          source.Meta.CoreLens,
		"decision_style":     source.Meta.DecisionStyle,
		"tone":               source.Meta.Tone,
		"default_bias":       source.Meta.DefaultBias,
		"priority_stack":     strings.Join(source.Meta.PriorityStack, " | "),
		"strong_domains":     strings.Join(source.Meta.StrongDomains, " | "),
		"reference_anchors":  strings.Join(source.Meta.ReferenceAnchors, " | "),
		"profile_long":       strings.TrimSpace(source.ProfileBody),
		"psychology_summary": strings.TrimSpace(source.Psychology),
		"anti_patterns":      strings.TrimSpace(source.AntiPatterns),
	}

	for _, domain := range sortedKeys(source.DomainBodies) {
		fields["domain_"+domain] = strings.TrimSpace(source.DomainBodies[domain])
	}

	return CompiledPersona{
		PersonaID: source.Meta.PersonaID,
		Fields:    fields,
	}
}

func (persona CompiledPersona) RuntimeDocument() map[string]any {
	document := make(map[string]any, len(persona.Fields)+1)
	document["persona_id"] = persona.PersonaID
	for _, key := range sortedKeys(persona.Fields) {
		document[key] = persona.Fields[key]
	}
	return document
}

func compileMaterial(source MaterialSource) CompiledMaterial {
	return CompiledMaterial{
		MaterialID: source.MaterialID,
		Fields: materials.CanonicalFields{
			RoleplayPrompt:            source.RoleplayPrompt,
			DiscussionQuestion:        source.DiscussionQuestion,
			SupplementaryMaterials:    source.SupplementaryMaterials,
			OutputContract:            source.OutputContract,
			AssumptionsAndConstraints: source.AssumptionsAndConstraints,
		},
	}
}

func validateCompiledMaterial(material CompiledMaterial, personas map[string]CompiledPersona, personaOrder []string) error {
	fieldTemplates := map[string]string{
		"roleplay_prompt":             material.Fields.RoleplayPrompt,
		"discussion_question":         material.Fields.DiscussionQuestion,
		"supplementary_materials":     material.Fields.SupplementaryMaterials,
		"output_contract":             material.Fields.OutputContract,
		"assumptions_and_constraints": material.Fields.AssumptionsAndConstraints,
	}

	for _, fieldName := range []string{
		"roleplay_prompt",
		"discussion_question",
		"supplementary_materials",
		"output_contract",
		"assumptions_and_constraints",
	} {
		renderedTemplate, err := template.New(fieldName).Option("missingkey=error").Parse(fieldTemplates[fieldName])
		if err != nil {
			return fmt.Errorf("parse %s template: %w", fieldName, err)
		}

		for _, personaID := range personaOrder {
			persona, ok := personas[personaID]
			if !ok {
				return fmt.Errorf("missing compiled persona %q during material validation", personaID)
			}

			var rendered bytes.Buffer
			err := renderedTemplate.Execute(&rendered, prepareTemplateContextForPersona(persona, runtimePersonaArtifact(persona.PersonaID)))
			if err != nil {
				return fmt.Errorf("execute %s template for persona %q: %w", fieldName, personaID, err)
			}
		}
	}

	return nil
}

func (material CompiledMaterial) RuntimeDocument() map[string]any {
	return map[string]any{
		"roleplay_prompt":             material.Fields.RoleplayPrompt,
		"discussion_question":         material.Fields.DiscussionQuestion,
		"supplementary_materials":     material.Fields.SupplementaryMaterials,
		"output_contract":             material.Fields.OutputContract,
		"assumptions_and_constraints": material.Fields.AssumptionsAndConstraints,
	}
}

func runtimePersonaRef(personaID string) string {
	return filepath.ToSlash(filepath.Join("personas", personaID+".json"))
}

func runtimePersonaArtifact(personaID string) string {
	return filepath.ToSlash(filepath.Join("runtime", "personas", personaID+".json"))
}

func runtimeMaterialArtifact(materialID string) string {
	return filepath.ToSlash(filepath.Join("runtime", "materials", materialID+".json"))
}

type prepareTemplateContext struct {
	PersonaID  string
	SourcePath string
	Fields     map[string]any
}

func prepareTemplateContextForPersona(persona CompiledPersona, sourcePath string) prepareTemplateContext {
	return prepareTemplateContext{
		PersonaID:  persona.PersonaID,
		SourcePath: sourcePath,
		Fields:     personaFieldMapAny(persona.Fields),
	}
}

func personaFieldMapAny(values map[string]string) map[string]any {
	converted := make(map[string]any, len(values))
	for key, value := range values {
		converted[key] = value
	}
	return converted
}
