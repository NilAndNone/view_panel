package content

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestCompileBundleEmitsRuntimeCompatiblePersonaAndMaterialFields(t *testing.T) {
	catalog, err := LoadCatalog(testSourceRoot())
	if err != nil {
		t.Fatalf("LoadCatalog returned error: %v", err)
	}

	bundle, err := CompileBundle(catalog, CompileOptions{BundleID: "default"})
	if err != nil {
		t.Fatalf("CompileBundle returned error: %v", err)
	}

	if bundle.BundleID != "default" {
		t.Fatalf("bundle_id = %q, want default", bundle.BundleID)
	}
	expectedPersonaRefs := []string{
		"personas/analyst.json",
		"personas/builder.json",
		"personas/historian.json",
		"personas/operator.json",
		"personas/skeptic.json",
		"personas/policy.json",
	}
	if len(bundle.PersonaIndex.Personas) != len(expectedPersonaRefs) {
		t.Fatalf("persona_index = %#v, want %d entries", bundle.PersonaIndex.Personas, len(expectedPersonaRefs))
	}
	for index, expectedRef := range expectedPersonaRefs {
		if bundle.PersonaIndex.Personas[index] != expectedRef {
			t.Fatalf("persona_index[%d] = %q, want %q", index, bundle.PersonaIndex.Personas[index], expectedRef)
		}
	}
	if len(bundle.MaterialOrder) != 3 {
		t.Fatalf("material_order = %#v, want 3 entries", bundle.MaterialOrder)
	}
	if bundle.MaterialOrder[0] != "operations" || bundle.MaterialOrder[1] != "policy" || bundle.MaterialOrder[2] != "technology" {
		t.Fatalf("material_order = %#v, want operations, policy, technology", bundle.MaterialOrder)
	}

	analyst, ok := bundle.Personas["analyst"]
	if !ok {
		t.Fatalf("missing compiled analyst persona")
	}
	if analyst.Fields["profile_long"] == "" {
		t.Fatalf("expected profile_long field")
	}
	if analyst.Fields["domain_technology"] == "" {
		t.Fatalf("expected domain_technology field")
	}
	if analyst.Fields["domain_operations"] == "" {
		t.Fatalf("expected domain_operations field")
	}
	if analyst.Fields["strong_domains"] != "technology | operations" {
		t.Fatalf("strong_domains = %q", analyst.Fields["strong_domains"])
	}

	builder, ok := bundle.Personas["builder"]
	if !ok {
		t.Fatalf("missing compiled builder persona")
	}
	for _, field := range []string{
		"display_name",
		"core_lens",
		"decision_style",
		"tone",
		"default_bias",
		"priority_stack",
		"strong_domains",
		"reference_anchors",
		"profile_long",
		"psychology_summary",
		"anti_patterns",
		"domain_technology",
		"domain_product",
		"domain_execution",
	} {
		if builder.Fields[field] == "" {
			t.Fatalf("expected non-empty builder field %q", field)
		}
	}
	if !strings.Contains(builder.Fields["strong_domains"], "technology") {
		t.Fatalf("builder strong_domains = %q, want technology entry", builder.Fields["strong_domains"])
	}

	for _, personaID := range []string{"historian", "operator", "skeptic", "policy"} {
		persona, ok := bundle.Personas[personaID]
		if !ok {
			t.Fatalf("missing compiled %s persona", personaID)
		}
		for _, field := range []string{
			"display_name",
			"core_lens",
			"decision_style",
			"tone",
			"default_bias",
			"priority_stack",
			"strong_domains",
			"reference_anchors",
			"profile_long",
			"psychology_summary",
			"anti_patterns",
			"domain_technology",
		} {
			if persona.Fields[field] == "" {
				t.Fatalf("expected non-empty %s field %q", personaID, field)
			}
		}
	}

	personaDoc := builder.RuntimeDocument()
	if got, ok := personaDoc["persona_id"].(string); !ok || got != "builder" {
		t.Fatalf("runtime persona_id = %#v, want builder", personaDoc["persona_id"])
	}
	if got, ok := personaDoc["profile_long"].(string); !ok || got == "" {
		t.Fatalf("runtime profile_long = %#v, want non-empty string", personaDoc["profile_long"])
	}

	for _, materialID := range []string{"operations", "policy", "technology"} {
		material, ok := bundle.Materials[materialID]
		if !ok {
			t.Fatalf("missing compiled %s material", materialID)
		}
		if material.Fields.RoleplayPrompt == "" || material.Fields.DiscussionQuestion == "" || material.Fields.SupplementaryMaterials == "" || material.Fields.OutputContract == "" || material.Fields.AssumptionsAndConstraints == "" {
			t.Fatalf("compiled material %q has empty canonical field: %#v", materialID, material.Fields)
		}

		materialDoc := material.RuntimeDocument()
		if got, ok := materialDoc["roleplay_prompt"].(string); !ok || got == "" {
			t.Fatalf("runtime roleplay_prompt for %q = %#v, want non-empty string", materialID, materialDoc["roleplay_prompt"])
		}
	}

	if bundle.Manifest.SchemaVersion != "content_bundle_manifest_v1" {
		t.Fatalf("manifest schema_version = %q", bundle.Manifest.SchemaVersion)
	}
	if bundle.Manifest.PersonaCount != 6 {
		t.Fatalf("manifest persona_count = %d, want 6", bundle.Manifest.PersonaCount)
	}
	if bundle.Manifest.MaterialCount != 3 {
		t.Fatalf("manifest material_count = %d, want 3", bundle.Manifest.MaterialCount)
	}
	if len(bundle.Manifest.Artifacts) != 10 {
		t.Fatalf("manifest artifacts = %#v, want 10 entries", bundle.Manifest.Artifacts)
	}
	expectedArtifacts := []string{
		"runtime/persona-index.json",
		"runtime/personas/analyst.json",
		"runtime/personas/builder.json",
		"runtime/personas/historian.json",
		"runtime/personas/operator.json",
		"runtime/personas/skeptic.json",
		"runtime/personas/policy.json",
		"runtime/materials/operations.json",
		"runtime/materials/policy.json",
		"runtime/materials/technology.json",
	}
	for index, expectedArtifact := range expectedArtifacts {
		if bundle.Manifest.Artifacts[index] != expectedArtifact {
			t.Fatalf("artifact[%d] = %q, want %q", index, bundle.Manifest.Artifacts[index], expectedArtifact)
		}
	}
}

func TestCompileBundleEmitsRenderCompatibleRoleplayPrompt(t *testing.T) {
	catalog, err := LoadCatalog(testSourceRoot())
	if err != nil {
		t.Fatalf("LoadCatalog returned error: %v", err)
	}

	bundle, err := CompileBundle(catalog, CompileOptions{BundleID: "default"})
	if err != nil {
		t.Fatalf("CompileBundle returned error: %v", err)
	}

	for _, materialID := range bundle.MaterialOrder {
		material := bundle.Materials[materialID]
		tpl, err := template.New("roleplay_prompt").Option("missingkey=error").Parse(material.Fields.RoleplayPrompt)
		if err != nil {
			t.Fatalf("Parse roleplay_prompt for %q: %v", materialID, err)
		}

		for _, personaID := range bundle.PersonaOrder {
			persona := bundle.Personas[personaID]

			var rendered bytes.Buffer
			err = tpl.Execute(&rendered, prepareTemplateContextForPersona(persona, "/tmp/"+personaID+".json"))
			if err != nil {
				t.Fatalf("Execute roleplay_prompt for %q/%q: %v", materialID, personaID, err)
			}

			output := rendered.String()
			if !strings.Contains(output, persona.Fields["display_name"]) {
				t.Fatalf("rendered roleplay_prompt for %q/%q missing display name: %q", materialID, personaID, output)
			}
			if !strings.Contains(output, persona.Fields["domain_technology"]) {
				t.Fatalf("rendered roleplay_prompt for %q/%q missing domain_technology body", materialID, personaID)
			}
		}
	}
}

func TestCompiledLivePersonasStayRicherThanSmokeFixtures(t *testing.T) {
	catalog, err := LoadCatalog(testSourceRoot())
	if err != nil {
		t.Fatalf("LoadCatalog returned error: %v", err)
	}

	bundle, err := CompileBundle(catalog, CompileOptions{BundleID: "default"})
	if err != nil {
		t.Fatalf("CompileBundle returned error: %v", err)
	}

	requiredFields := []string{
		"display_name",
		"core_lens",
		"decision_style",
		"tone",
		"default_bias",
		"priority_stack",
		"strong_domains",
		"reference_anchors",
		"profile_long",
		"psychology_summary",
		"anti_patterns",
		"domain_technology",
	}

	for _, personaID := range []string{"analyst", "builder"} {
		smoke := readSmokeRuntimePersonaFixture(t, personaID)
		compiled, ok := bundle.Personas[personaID]
		if !ok {
			t.Fatalf("missing compiled persona %q", personaID)
		}
		if smoke["persona_id"] != personaID {
			t.Fatalf("smoke persona_id = %q, want %q", smoke["persona_id"], personaID)
		}

		for _, field := range requiredFields {
			if strings.TrimSpace(smoke[field]) == "" {
				t.Fatalf("smoke fixture %q missing field %q", personaID, field)
			}
			if strings.TrimSpace(compiled.Fields[field]) == "" {
				t.Fatalf("compiled persona %q missing field %q", personaID, field)
			}
		}

		if len(compiled.Fields["profile_long"]) <= len(smoke["profile_long"]) {
			t.Fatalf("compiled %q profile_long should be richer than smoke fixture", personaID)
		}
		if len(compiled.Fields["psychology_summary"]) <= len(smoke["psychology_summary"]) {
			t.Fatalf("compiled %q psychology_summary should be richer than smoke fixture", personaID)
		}
		if len(compiled.Fields["anti_patterns"]) <= len(smoke["anti_patterns"]) {
			t.Fatalf("compiled %q anti_patterns should be richer than smoke fixture", personaID)
		}
	}
}

func TestCompileBundleRejectsTemplateOutsidePrepareContext(t *testing.T) {
	catalog := Catalog{
		PersonaOrder: []string{"analyst"},
		Personas: map[string]PersonaSource{
			"analyst": {
				Meta: PersonaMeta{
					PersonaID:        "analyst",
					DisplayName:      "Analyst",
					CoreLens:         "risk",
					DecisionStyle:    "decompose",
					Tone:             "clear",
					DefaultBias:      "contain downside",
					PriorityStack:    []string{"clarity", "evidence", "containment"},
					StrongDomains:    []string{"technology", "operations"},
					ReferenceAnchors: []string{"postmortems", "decision memos"},
				},
				ProfileBody:  "profile",
				Psychology:   "psychology",
				AntiPatterns: "anti patterns",
				DomainBodies: map[string]string{
					"technology": "technology domain",
					"operations": "operations domain",
				},
			},
		},
		MaterialOrder: []string{"technology"},
		Materials: map[string]MaterialSource{
			"technology": {
				MaterialID:                "technology",
				Title:                     "Technology",
				Domain:                    "technology",
				RoleplayPrompt:            "You are {{.display_name}}.",
				DiscussionQuestion:        "question",
				SupplementaryMaterials:    "materials",
				OutputContract:            "contract",
				AssumptionsAndConstraints: "constraints",
			},
		},
	}

	_, err := CompileBundle(catalog, CompileOptions{BundleID: "default"})
	if err == nil {
		t.Fatalf("expected CompileBundle to reject template outside prepare context")
	}
	if !strings.Contains(err.Error(), "display_name") {
		t.Fatalf("expected error to mention display_name placeholder, got %q", err)
	}
}

func readSmokeRuntimePersonaFixture(t *testing.T, personaID string) map[string]string {
	t.Helper()

	path := filepath.Clean(filepath.Join("..", "..", "testdata", "smoke", "runtime", "personas", personaID+".json"))
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}

	var persona map[string]string
	if err := json.Unmarshal(payload, &persona); err != nil {
		t.Fatalf("Unmarshal(%q): %v", path, err)
	}

	return persona
}
