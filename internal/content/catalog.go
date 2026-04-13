package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	personasDirName      = "personas"
	materialsDirName     = "materials"
	personaIndexFileName = "index.json"
	metaFileName         = "meta.json"
	profileFileName      = "profile.md"
	psychologyFileName   = "psychology.md"
	antiPatternsFileName = "anti_patterns.md"
	domainsDirName       = "domains"
)

const artifactIdentifierPatternText = `^[a-z][a-z0-9_]*$`

var artifactIdentifierPattern = regexp.MustCompile(artifactIdentifierPatternText)

type Catalog struct {
	SourceRoot    string
	PersonaOrder  []string
	Personas      map[string]PersonaSource
	MaterialOrder []string
	Materials     map[string]MaterialSource
}

type PersonaMeta struct {
	PersonaID        string   `json:"persona_id"`
	DisplayName      string   `json:"display_name"`
	CoreLens         string   `json:"core_lens"`
	DecisionStyle    string   `json:"decision_style"`
	Tone             string   `json:"tone"`
	DefaultBias      string   `json:"default_bias"`
	PriorityStack    []string `json:"priority_stack"`
	StrongDomains    []string `json:"strong_domains"`
	ReferenceAnchors []string `json:"reference_anchors"`
}

type PersonaSource struct {
	Meta         PersonaMeta
	ProfileBody  string
	Psychology   string
	AntiPatterns string
	DomainBodies map[string]string
}

type MaterialSource struct {
	MaterialID                string `json:"material_id"`
	Title                     string `json:"title"`
	Domain                    string `json:"domain"`
	RoleplayPrompt            string `json:"roleplay_prompt"`
	DiscussionQuestion        string `json:"discussion_question"`
	SupplementaryMaterials    string `json:"supplementary_materials"`
	OutputContract            string `json:"output_contract"`
	AssumptionsAndConstraints string `json:"assumptions_and_constraints"`
}

type personaIndexSource struct {
	Personas []string `json:"personas"`
}

func LoadCatalog(sourceRoot string) (Catalog, error) {
	if strings.TrimSpace(sourceRoot) == "" {
		return Catalog{}, fmt.Errorf("content source root must not be empty")
	}

	absoluteRoot, err := filepath.Abs(sourceRoot)
	if err != nil {
		return Catalog{}, fmt.Errorf("resolve content source root %q: %w", sourceRoot, err)
	}

	personasRoot := filepath.Join(absoluteRoot, personasDirName)
	index, err := loadPersonaIndex(filepath.Join(personasRoot, personaIndexFileName))
	if err != nil {
		return Catalog{}, err
	}
	if err := validatePersonaRootLayout(personasRoot, index.Personas); err != nil {
		return Catalog{}, err
	}

	catalog := Catalog{
		SourceRoot:    absoluteRoot,
		PersonaOrder:  append([]string(nil), index.Personas...),
		Personas:      make(map[string]PersonaSource, len(index.Personas)),
		MaterialOrder: nil,
		Materials:     nil,
	}

	for _, personaID := range index.Personas {
		persona, err := loadPersonaSource(personasRoot, personaID)
		if err != nil {
			return Catalog{}, err
		}
		catalog.Personas[personaID] = persona
	}

	materialOrder, materialMap, err := loadMaterialSources(filepath.Join(absoluteRoot, materialsDirName))
	if err != nil {
		return Catalog{}, err
	}
	catalog.MaterialOrder = materialOrder
	catalog.Materials = materialMap

	return catalog, nil
}

func loadPersonaIndex(path string) (personaIndexSource, error) {
	index, err := decodeJSONFile[personaIndexSource](path)
	if err != nil {
		return personaIndexSource{}, err
	}

	personas, err := normalizeUniqueStrings("personas index", index.Personas, 1)
	if err != nil {
		return personaIndexSource{}, err
	}
	for _, personaID := range personas {
		if err := validateSlug("persona id", personaID); err != nil {
			return personaIndexSource{}, err
		}
	}
	index.Personas = personas

	return index, nil
}

func validatePersonaRootLayout(personasRoot string, expected []string) error {
	entries, err := os.ReadDir(personasRoot)
	if err != nil {
		return fmt.Errorf("read personas root %q: %w", personasRoot, err)
	}

	expectedSet := make(map[string]struct{}, len(expected))
	for _, personaID := range expected {
		expectedSet[personaID] = struct{}{}
	}

	seen := make(map[string]struct{}, len(expected))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() {
			if _, ok := expectedSet[name]; !ok {
				return fmt.Errorf("unexpected persona directory %q under %q", name, personasRoot)
			}
			seen[name] = struct{}{}
			continue
		}
		if name != personaIndexFileName {
			return fmt.Errorf("unexpected file %q under %q", name, personasRoot)
		}
	}

	for _, personaID := range expected {
		if _, ok := seen[personaID]; !ok {
			return fmt.Errorf("persona directory %q missing under %q", personaID, personasRoot)
		}
	}

	return nil
}

func loadPersonaSource(personasRoot, personaID string) (PersonaSource, error) {
	personaRoot := filepath.Join(personasRoot, personaID)
	if err := validatePersonaPackLayout(personaRoot); err != nil {
		return PersonaSource{}, err
	}

	meta, err := decodeJSONFile[PersonaMeta](filepath.Join(personaRoot, metaFileName))
	if err != nil {
		return PersonaSource{}, err
	}
	meta, err = normalizePersonaMeta(meta)
	if err != nil {
		return PersonaSource{}, fmt.Errorf("validate persona meta %q: %w", filepath.Join(personaRoot, metaFileName), err)
	}
	if meta.PersonaID != personaID {
		return PersonaSource{}, fmt.Errorf("persona meta %q declares persona_id %q but directory is %q", filepath.Join(personaRoot, metaFileName), meta.PersonaID, personaID)
	}

	profileBody, err := readRequiredText(filepath.Join(personaRoot, profileFileName))
	if err != nil {
		return PersonaSource{}, err
	}
	psychology, err := readRequiredText(filepath.Join(personaRoot, psychologyFileName))
	if err != nil {
		return PersonaSource{}, err
	}
	antiPatterns, err := readRequiredText(filepath.Join(personaRoot, antiPatternsFileName))
	if err != nil {
		return PersonaSource{}, err
	}
	domainBodies, err := loadDomainBodies(filepath.Join(personaRoot, domainsDirName), meta.StrongDomains)
	if err != nil {
		return PersonaSource{}, err
	}

	return PersonaSource{
		Meta:         meta,
		ProfileBody:  profileBody,
		Psychology:   psychology,
		AntiPatterns: antiPatterns,
		DomainBodies: domainBodies,
	}, nil
}

func validatePersonaPackLayout(personaRoot string) error {
	entries, err := os.ReadDir(personaRoot)
	if err != nil {
		return fmt.Errorf("read persona pack %q: %w", personaRoot, err)
	}

	allowed := map[string]bool{
		metaFileName:         false,
		profileFileName:      false,
		psychologyFileName:   false,
		antiPatternsFileName: false,
		domainsDirName:       true,
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		expectDir, ok := allowed[name]
		if !ok {
			return fmt.Errorf("unexpected entry %q in persona pack %q", name, personaRoot)
		}
		if entry.IsDir() != expectDir {
			return fmt.Errorf("entry %q in persona pack %q has unexpected kind", name, personaRoot)
		}
	}

	for name, expectDir := range allowed {
		path := filepath.Join(personaRoot, name)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("required persona pack entry %q missing in %q: %w", name, personaRoot, err)
		}
		if info.IsDir() != expectDir {
			return fmt.Errorf("required persona pack entry %q in %q has unexpected kind", name, personaRoot)
		}
	}

	return nil
}

func loadDomainBodies(domainsRoot string, strongDomains []string) (map[string]string, error) {
	entries, err := os.ReadDir(domainsRoot)
	if err != nil {
		return nil, fmt.Errorf("read domains root %q: %w", domainsRoot, err)
	}

	expected := make(map[string]struct{}, len(strongDomains))
	for _, domain := range strongDomains {
		expected[domain+".md"] = struct{}{}
	}

	seen := make(map[string]struct{}, len(strongDomains))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() {
			return nil, fmt.Errorf("unexpected domain subdirectory %q in %q", name, domainsRoot)
		}
		if _, ok := expected[name]; !ok {
			return nil, fmt.Errorf("unexpected domain file %q in %q", name, domainsRoot)
		}
		seen[name] = struct{}{}
	}

	domainBodies := make(map[string]string, len(strongDomains))
	for _, domain := range strongDomains {
		filename := domain + ".md"
		if _, ok := seen[filename]; !ok {
			return nil, fmt.Errorf("missing required domain file %q in %q", filename, domainsRoot)
		}
		body, err := readRequiredText(filepath.Join(domainsRoot, filename))
		if err != nil {
			return nil, err
		}
		domainBodies[domain] = body
	}

	return domainBodies, nil
}

func loadMaterialSources(materialsRoot string) ([]string, map[string]MaterialSource, error) {
	entries, err := os.ReadDir(materialsRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("read materials root %q: %w", materialsRoot, err)
	}

	order := make([]string, 0, len(entries))
	materialMap := make(map[string]MaterialSource, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() {
			return nil, nil, fmt.Errorf("unexpected directory %q in materials root %q", name, materialsRoot)
		}
		if filepath.Ext(name) != ".json" {
			return nil, nil, fmt.Errorf("unexpected non-JSON material file %q in %q", name, materialsRoot)
		}

		source, err := loadMaterialSource(filepath.Join(materialsRoot, name))
		if err != nil {
			return nil, nil, err
		}
		if _, exists := materialMap[source.MaterialID]; exists {
			return nil, nil, fmt.Errorf("duplicate material_id %q in %q", source.MaterialID, materialsRoot)
		}
		order = append(order, source.MaterialID)
		materialMap[source.MaterialID] = source
	}

	if len(order) == 0 {
		return nil, nil, fmt.Errorf("materials root %q does not contain any source files", materialsRoot)
	}

	return order, materialMap, nil
}

func loadMaterialSource(path string) (MaterialSource, error) {
	source, err := decodeJSONFile[MaterialSource](path)
	if err != nil {
		return MaterialSource{}, err
	}

	source.MaterialID = strings.TrimSpace(source.MaterialID)
	source.Title = strings.TrimSpace(source.Title)
	source.Domain = strings.TrimSpace(source.Domain)
	source.RoleplayPrompt = strings.TrimSpace(source.RoleplayPrompt)
	source.DiscussionQuestion = strings.TrimSpace(source.DiscussionQuestion)
	source.SupplementaryMaterials = strings.TrimSpace(source.SupplementaryMaterials)
	source.OutputContract = strings.TrimSpace(source.OutputContract)
	source.AssumptionsAndConstraints = strings.TrimSpace(source.AssumptionsAndConstraints)

	if err := validateSlug("material id", source.MaterialID); err != nil {
		return MaterialSource{}, fmt.Errorf("validate material source %q: %w", path, err)
	}
	if source.Title == "" {
		return MaterialSource{}, fmt.Errorf("validate material source %q: title must not be empty", path)
	}
	if err := validateSlug("material domain", source.Domain); err != nil {
		return MaterialSource{}, fmt.Errorf("validate material source %q: %w", path, err)
	}
	if source.RoleplayPrompt == "" || source.DiscussionQuestion == "" || source.SupplementaryMaterials == "" || source.OutputContract == "" || source.AssumptionsAndConstraints == "" {
		return MaterialSource{}, fmt.Errorf("validate material source %q: canonical runtime fields must all be non-empty", path)
	}

	expectedFileName := source.MaterialID + ".json"
	if filepath.Base(path) != expectedFileName {
		return MaterialSource{}, fmt.Errorf("validate material source %q: filename must match material_id %q", path, expectedFileName)
	}

	return source, nil
}

func normalizePersonaMeta(meta PersonaMeta) (PersonaMeta, error) {
	meta.PersonaID = strings.TrimSpace(meta.PersonaID)
	meta.DisplayName = strings.TrimSpace(meta.DisplayName)
	meta.CoreLens = strings.TrimSpace(meta.CoreLens)
	meta.DecisionStyle = strings.TrimSpace(meta.DecisionStyle)
	meta.Tone = strings.TrimSpace(meta.Tone)
	meta.DefaultBias = strings.TrimSpace(meta.DefaultBias)

	if err := validateSlug("persona_id", meta.PersonaID); err != nil {
		return PersonaMeta{}, err
	}
	if meta.DisplayName == "" {
		return PersonaMeta{}, fmt.Errorf("display_name must not be empty")
	}
	if meta.CoreLens == "" {
		return PersonaMeta{}, fmt.Errorf("core_lens must not be empty")
	}
	if meta.DecisionStyle == "" {
		return PersonaMeta{}, fmt.Errorf("decision_style must not be empty")
	}
	if meta.Tone == "" {
		return PersonaMeta{}, fmt.Errorf("tone must not be empty")
	}
	if meta.DefaultBias == "" {
		return PersonaMeta{}, fmt.Errorf("default_bias must not be empty")
	}

	var err error
	meta.PriorityStack, err = normalizeUniqueStrings("priority_stack", meta.PriorityStack, 3)
	if err != nil {
		return PersonaMeta{}, err
	}
	meta.StrongDomains, err = normalizeUniqueStrings("strong_domains", meta.StrongDomains, 2)
	if err != nil {
		return PersonaMeta{}, err
	}
	for _, domain := range meta.StrongDomains {
		if err := validateTemplateSafeIdentifier("strong_domains entry", domain); err != nil {
			return PersonaMeta{}, err
		}
	}
	meta.ReferenceAnchors, err = normalizeUniqueStrings("reference_anchors", meta.ReferenceAnchors, 2)
	if err != nil {
		return PersonaMeta{}, err
	}

	return meta, nil
}

func normalizeUniqueStrings(field string, values []string, minimum int) ([]string, error) {
	if len(values) < minimum {
		return nil, fmt.Errorf("%s must contain at least %d entries", field, minimum)
	}

	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for index, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("%s[%d] must not be empty", field, index)
		}
		if _, exists := seen[value]; exists {
			return nil, fmt.Errorf("%s contains duplicate value %q", field, value)
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}

	return normalized, nil
}

func validateSlug(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	if value != filepath.Base(value) {
		return fmt.Errorf("%s %q must be a single path segment", field, value)
	}
	if !artifactIdentifierPattern.MatchString(value) {
		return fmt.Errorf("%s %q must match %s", field, value, artifactIdentifierPatternText)
	}
	return nil
}

func validateTemplateSafeIdentifier(field, value string) error {
	if err := validateSlug(field, value); err != nil {
		return err
	}
	return nil
}

func readRequiredText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", path, err)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", fmt.Errorf("text file %q must not be empty", path)
	}
	return text, nil
}

func decodeJSONFile[T any](path string) (T, error) {
	var zero T

	data, err := os.ReadFile(path)
	if err != nil {
		return zero, fmt.Errorf("read %q: %w", path, err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var value T
	if err := decoder.Decode(&value); err != nil {
		return zero, fmt.Errorf("decode %q: %w", path, err)
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return zero, fmt.Errorf("decode %q: unexpected trailing JSON content", path)
		}
		return zero, fmt.Errorf("decode %q: %w", path, err)
	}

	return value, nil
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
