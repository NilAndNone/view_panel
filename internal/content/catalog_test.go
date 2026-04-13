package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCatalogLoadsAnalystSourceFixture(t *testing.T) {
	catalog, err := LoadCatalog(testSourceRoot())
	if err != nil {
		t.Fatalf("LoadCatalog returned error: %v", err)
	}

	if got := strings.Join(catalog.PersonaOrder, ","); got != "analyst,builder,historian,operator,skeptic,policy" {
		t.Fatalf("persona_order = %#v, want [\"analyst\", \"builder\", \"historian\", \"operator\", \"skeptic\", \"policy\"]", catalog.PersonaOrder)
	}
	if got := strings.Join(catalog.MaterialOrder, ","); got != "operations,policy,technology" {
		t.Fatalf("material_order = %#v, want [\"operations\", \"policy\", \"technology\"]", catalog.MaterialOrder)
	}

	analyst, ok := catalog.Personas["analyst"]
	if !ok {
		t.Fatalf("expected analyst persona in catalog")
	}
	if analyst.Meta.PersonaID != "analyst" {
		t.Fatalf("persona_id = %q, want analyst", analyst.Meta.PersonaID)
	}
	if strings.TrimSpace(analyst.ProfileBody) == "" {
		t.Fatalf("expected non-empty analyst profile body")
	}
	if strings.TrimSpace(analyst.DomainBodies["technology"]) == "" {
		t.Fatalf("expected non-empty technology domain body")
	}
	if strings.TrimSpace(analyst.DomainBodies["operations"]) == "" {
		t.Fatalf("expected non-empty operations domain body")
	}

	technology, ok := catalog.Materials["technology"]
	if !ok {
		t.Fatalf("expected technology material in catalog")
	}
	if technology.MaterialID != "technology" {
		t.Fatalf("material_id = %q, want technology", technology.MaterialID)
	}
	if strings.TrimSpace(technology.RoleplayPrompt) == "" {
		t.Fatalf("expected non-empty roleplay_prompt")
	}
}

func TestLoadCatalogRejectsMissingDeclaredDomainFile(t *testing.T) {
	root := t.TempDir()

	writeTextFile(t, filepath.Join(root, "personas", "index.json"), "{\n  \"personas\": [\"analyst\"]\n}\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "meta.json"), "{\n  \"persona_id\": \"analyst\",\n  \"display_name\": \"Analyst\",\n  \"core_lens\": \"risk\",\n  \"decision_style\": \"decompose\",\n  \"tone\": \"clear\",\n  \"default_bias\": \"contain downside\",\n  \"priority_stack\": [\"clarity\", \"evidence\", \"containment\"],\n  \"strong_domains\": [\"technology\", \"operations\"],\n  \"reference_anchors\": [\"postmortems\", \"decision memos\"]\n}\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "profile.md"), "profile\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "psychology.md"), "psychology\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "anti_patterns.md"), "anti patterns\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "domains", "technology.md"), "technology domain\n")
	writeTextFile(t, filepath.Join(root, "materials", "technology.json"), "{\n  \"material_id\": \"technology\",\n  \"title\": \"Technology\",\n  \"domain\": \"technology\",\n  \"roleplay_prompt\": \"prompt\",\n  \"discussion_question\": \"question\",\n  \"supplementary_materials\": \"materials\",\n  \"output_contract\": \"contract\",\n  \"assumptions_and_constraints\": \"constraints\"\n}\n")

	_, err := LoadCatalog(root)
	if err == nil {
		t.Fatalf("expected LoadCatalog to fail when a declared domain file is missing")
	}
	if !strings.Contains(err.Error(), "operations.md") {
		t.Fatalf("expected error to mention missing operations.md, got %q", err)
	}
}

func TestLoadCatalogRejectsDomainIdentifierUnsafeForTemplateFields(t *testing.T) {
	root := t.TempDir()

	writeTextFile(t, filepath.Join(root, "personas", "index.json"), "{\n  \"personas\": [\"analyst\"]\n}\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "meta.json"), "{\n  \"persona_id\": \"analyst\",\n  \"display_name\": \"Analyst\",\n  \"core_lens\": \"risk\",\n  \"decision_style\": \"decompose\",\n  \"tone\": \"clear\",\n  \"default_bias\": \"contain downside\",\n  \"priority_stack\": [\"clarity\", \"evidence\", \"containment\"],\n  \"strong_domains\": [\"technology-risk\", \"operations\"],\n  \"reference_anchors\": [\"postmortems\", \"decision memos\"]\n}\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "profile.md"), "profile\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "psychology.md"), "psychology\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "anti_patterns.md"), "anti patterns\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "domains", "technology-risk.md"), "technology risk domain\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "domains", "operations.md"), "operations domain\n")
	writeTextFile(t, filepath.Join(root, "materials", "technology.json"), "{\n  \"material_id\": \"technology\",\n  \"title\": \"Technology\",\n  \"domain\": \"technology\",\n  \"roleplay_prompt\": \"prompt\",\n  \"discussion_question\": \"question\",\n  \"supplementary_materials\": \"materials\",\n  \"output_contract\": \"contract\",\n  \"assumptions_and_constraints\": \"constraints\"\n}\n")

	_, err := LoadCatalog(root)
	if err == nil {
		t.Fatalf("expected LoadCatalog to reject template-unsafe strong_domains entry")
	}
	if !strings.Contains(err.Error(), "strong_domains") {
		t.Fatalf("expected error to mention strong_domains, got %q", err)
	}
}

func TestLoadCatalogRejectsPersonaIdentifierUnsafeForArtifacts(t *testing.T) {
	root := t.TempDir()

	writeTextFile(t, filepath.Join(root, "personas", "index.json"), "{\n  \"personas\": [\"analyst-v2\"]\n}\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst-v2", "meta.json"), "{\n  \"persona_id\": \"analyst-v2\",\n  \"display_name\": \"Analyst\",\n  \"core_lens\": \"risk\",\n  \"decision_style\": \"decompose\",\n  \"tone\": \"clear\",\n  \"default_bias\": \"contain downside\",\n  \"priority_stack\": [\"clarity\", \"evidence\", \"containment\"],\n  \"strong_domains\": [\"technology\", \"operations\"],\n  \"reference_anchors\": [\"postmortems\", \"decision memos\"]\n}\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst-v2", "profile.md"), "profile\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst-v2", "psychology.md"), "psychology\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst-v2", "anti_patterns.md"), "anti patterns\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst-v2", "domains", "technology.md"), "technology domain\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst-v2", "domains", "operations.md"), "operations domain\n")
	writeTextFile(t, filepath.Join(root, "materials", "technology.json"), "{\n  \"material_id\": \"technology\",\n  \"title\": \"Technology\",\n  \"domain\": \"technology\",\n  \"roleplay_prompt\": \"prompt\",\n  \"discussion_question\": \"question\",\n  \"supplementary_materials\": \"materials\",\n  \"output_contract\": \"contract\",\n  \"assumptions_and_constraints\": \"constraints\"\n}\n")

	_, err := LoadCatalog(root)
	if err == nil {
		t.Fatalf("expected LoadCatalog to reject artifact-unsafe persona id")
	}
	if !strings.Contains(err.Error(), "persona id") && !strings.Contains(err.Error(), "persona_id") {
		t.Fatalf("expected error to mention persona id, got %q", err)
	}
}

func TestLoadCatalogRejectsMaterialIdentifierUnsafeForArtifacts(t *testing.T) {
	root := t.TempDir()

	writeTextFile(t, filepath.Join(root, "personas", "index.json"), "{\n  \"personas\": [\"analyst\"]\n}\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "meta.json"), "{\n  \"persona_id\": \"analyst\",\n  \"display_name\": \"Analyst\",\n  \"core_lens\": \"risk\",\n  \"decision_style\": \"decompose\",\n  \"tone\": \"clear\",\n  \"default_bias\": \"contain downside\",\n  \"priority_stack\": [\"clarity\", \"evidence\", \"containment\"],\n  \"strong_domains\": [\"technology\", \"operations\"],\n  \"reference_anchors\": [\"postmortems\", \"decision memos\"]\n}\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "profile.md"), "profile\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "psychology.md"), "psychology\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "anti_patterns.md"), "anti patterns\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "domains", "technology.md"), "technology domain\n")
	writeTextFile(t, filepath.Join(root, "personas", "analyst", "domains", "operations.md"), "operations domain\n")
	writeTextFile(t, filepath.Join(root, "materials", "technology-risk.json"), "{\n  \"material_id\": \"technology-risk\",\n  \"title\": \"Technology\",\n  \"domain\": \"technology\",\n  \"roleplay_prompt\": \"prompt\",\n  \"discussion_question\": \"question\",\n  \"supplementary_materials\": \"materials\",\n  \"output_contract\": \"contract\",\n  \"assumptions_and_constraints\": \"constraints\"\n}\n")

	_, err := LoadCatalog(root)
	if err == nil {
		t.Fatalf("expected LoadCatalog to reject artifact-unsafe material id")
	}
	if !strings.Contains(err.Error(), "material id") {
		t.Fatalf("expected error to mention material id, got %q", err)
	}
}

func testSourceRoot() string {
	return filepath.Clean(filepath.Join("..", "..", "content", "src"))
}

func writeTextFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
