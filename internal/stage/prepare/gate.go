package prepare

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	panelhash "view_panel/internal/hash"
	"view_panel/internal/storage"
)

const (
	checkPersonaDiscovery      = "prepare-persona-discovery"
	checkRequiredArtifacts     = "prepare-required-artifacts"
	checkDispatchSchema        = "prepare-dispatch-input-schema"
	checkHashAndBundle         = "prepare-hash-and-bundle"
	checkAuditWritability      = "prepare-audit-writability"
	errorPersonaSetEmpty       = "P05_E_PERSONA_SET_EMPTY"
	errorPersonaEntryNonDir    = "P05_E_PERSONA_ENTRY_NON_DIRECTORY"
	errorPersonaIO             = "P05_E_PERSONA_IO_ERROR"
	errorDispatchInvalid       = "P05_E_DISPATCH_INPUT_INVALID"
	errorDispatchFieldsInvalid = "P05_E_DISPATCH_INPUT_FIELDS_INVALID"
	errorManifestInvalid       = "P05_E_MANIFEST_INVALID"
	errorManifestPersonaID     = "P05_E_MANIFEST_PERSONA_ID_MISMATCH"
	errorHashEvidenceRead      = "P05_E_HASH_EVIDENCE_UNREADABLE"
	errorHashLedgerInvalid     = "P05_E_HASH_LEDGER_INVALID"
	errorHashLedgerMissing     = "P05_E_HASH_LEDGER_KEY_MISSING"
	errorHashFormatInvalid     = "P05_E_HASH_FORMAT_INVALID"
	errorHashMismatch          = "P05_E_HASH_MISMATCH"
	errorHashPayloadRead       = "P05_E_HASH_PAYLOAD_UNREADABLE"
	errorBundleMismatch        = "P05_E_BUNDLE_MISMATCH"
	errorAuditNotWritable      = "P05_E_AUDIT_PATH_NOT_WRITABLE"
	errorMissingDispatch       = "P05_E_REQUIRED_ARTIFACT_MISSING_DISPATCH_INPUT"
	errorMissingAgents         = "P05_E_REQUIRED_ARTIFACT_MISSING_AGENTS"
	errorMissingPrompt         = "P05_E_REQUIRED_ARTIFACT_MISSING_PROMPT"
	errorMissingManifest       = "P05_E_REQUIRED_ARTIFACT_MISSING_MANIFEST"
	errorMissingHashes         = "P05_E_REQUIRED_ARTIFACT_MISSING_HASHES"
	errorArtifactUnreadable    = "P05_E_REQUIRED_ARTIFACT_UNREADABLE"
	errorArtifactPathInvalid   = "P05_E_REQUIRED_ARTIFACT_PATH_INVALID"
)

var approvedDispatchKeys = []string{
	"roleplay_prompt",
	"discussion_question",
	"supplementary_materials",
	"output_contract",
	"assumptions_and_constraints",
}

var hashRefByArtifact = map[string]string{
	dispatchInputArtifactName: "dispatch_input_sha256",
	agentsArtifactName:        "agents_sha256",
	promptArtifactName:        "prompt_sha256",
}

var lowercaseSHA256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// PrepareGateStatus is the canonical hard-gate artifact emitted under 01_prepare.
type PrepareGateStatus struct {
	SchemaVersion      string             `json:"schema_version"`
	Stage              string             `json:"stage"`
	Status             string             `json:"status"`
	CanProceedToStage2 bool               `json:"can_proceed_to_stage2"`
	PersonaIDs         []string           `json:"persona_ids"`
	Checks             []PrepareGateCheck `json:"checks"`
}

// PrepareGateCheck is one deterministic gate check result.
type PrepareGateCheck struct {
	CheckID        string               `json:"check_id"`
	Status         string               `json:"status"`
	ErrorCode      string               `json:"error_code,omitempty"`
	ErrorMessage   string               `json:"error_message,omitempty"`
	PersonaResults []PrepareGatePersona `json:"persona_results"`
}

// PrepareGatePersona is one persona-scoped check result.
type PrepareGatePersona struct {
	PersonaID    string `json:"persona_id"`
	Status       string `json:"status"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type requiredArtifactState struct {
	personaID        string
	firstFailure     string
	firstMessage     string
	blockingArtifact string
	artifacts        map[string]resolvedArtifact
}

type resolvedArtifact struct {
	path     string
	readable bool
}

func runHardGate(ctx context.Context, runRoot string) (PrepareGateStatus, error) {
	discoveryCheck, discoveredIDs := gateCheckPersonaDiscovery(runRoot)
	requiredCheck, requiredState := gateCheckRequiredArtifacts(runRoot, discoveredIDs)
	dispatchSchemaCheck := gateCheckDispatchSchema(discoveredIDs, requiredState)
	hashBundleCheck := gateCheckHashAndBundle(discoveredIDs, requiredState)
	auditCheck := gateCheckAuditWritability(runRoot)

	checks := []PrepareGateCheck{
		discoveryCheck,
		requiredCheck,
		dispatchSchemaCheck,
		hashBundleCheck,
		auditCheck,
	}

	overallStatus := deriveOverallStatus(checks)
	status := PrepareGateStatus{
		SchemaVersion:      prepareGateSchemaVersion,
		Stage:              prepareStageName,
		Status:             overallStatus,
		CanProceedToStage2: overallStatus == statusPass,
		PersonaIDs:         append([]string(nil), discoveredIDs...),
		Checks:             checks,
	}

	gateStatusPath, err := storage.ResolvePath(storage.PrepareRoot(runRoot), prepareGateStatusArtifactName)
	if err != nil {
		return PrepareGateStatus{}, fmt.Errorf("resolve %s path: %w", prepareGateStatusArtifactName, err)
	}
	if err := storage.WriteJSON(gateStatusPath, status); err != nil {
		return PrepareGateStatus{}, fmt.Errorf("write %s: %w", prepareGateStatusArtifactName, err)
	}

	if err := ctx.Err(); err != nil {
		return status, err
	}
	return status, nil
}

func gateCheckPersonaDiscovery(runRoot string) (PrepareGateCheck, []string) {
	personasRoot := filepath.Join(storage.PrepareRoot(runRoot), "personas")
	entries, err := os.ReadDir(personasRoot)
	if err != nil {
		return PrepareGateCheck{
			CheckID:        checkPersonaDiscovery,
			Status:         statusFail,
			ErrorCode:      errorPersonaIO,
			ErrorMessage:   fmt.Sprintf("unable to enumerate %s: %v", personasRoot, err),
			PersonaResults: []PrepareGatePersona{},
		}, nil
	}

	personaIDs := make([]string, 0, len(entries))
	hasNonDirectoryEntry := false
	for _, entry := range entries {
		name := entry.Name()
		if name == "" {
			return PrepareGateCheck{
				CheckID:        checkPersonaDiscovery,
				Status:         statusFail,
				ErrorCode:      errorPersonaIO,
				ErrorMessage:   fmt.Sprintf("unable to extract persona_id from %s", personasRoot),
				PersonaResults: []PrepareGatePersona{},
			}, nil
		}
		if !entry.IsDir() {
			hasNonDirectoryEntry = true
			continue
		}
		personaIDs = append(personaIDs, name)
	}

	sort.Strings(personaIDs)
	personaRows := make([]PrepareGatePersona, 0, len(personaIDs))
	for _, personaID := range personaIDs {
		personaRows = append(personaRows, PrepareGatePersona{
			PersonaID: personaID,
			Status:    statusPass,
		})
	}

	switch {
	case hasNonDirectoryEntry:
		return PrepareGateCheck{
			CheckID:        checkPersonaDiscovery,
			Status:         statusFail,
			ErrorCode:      errorPersonaEntryNonDir,
			ErrorMessage:   fmt.Sprintf("non-directory entry present under %s", personasRoot),
			PersonaResults: personaRows,
		}, personaIDs
	case len(personaIDs) == 0:
		return PrepareGateCheck{
			CheckID:        checkPersonaDiscovery,
			Status:         statusFail,
			ErrorCode:      errorPersonaSetEmpty,
			ErrorMessage:   fmt.Sprintf("no persona directories found under %s", personasRoot),
			PersonaResults: []PrepareGatePersona{},
		}, nil
	default:
		return PrepareGateCheck{
			CheckID:        checkPersonaDiscovery,
			Status:         statusPass,
			PersonaResults: personaRows,
		}, personaIDs
	}
}

func gateCheckRequiredArtifacts(runRoot string, personaIDs []string) (PrepareGateCheck, map[string]requiredArtifactState) {
	check := PrepareGateCheck{
		CheckID:        checkRequiredArtifacts,
		Status:         statusPass,
		PersonaResults: []PrepareGatePersona{},
	}
	state := make(map[string]requiredArtifactState, len(personaIDs))
	if len(personaIDs) == 0 {
		return check, state
	}

	artifactsInOrder := []string{
		dispatchInputArtifactName,
		agentsArtifactName,
		promptArtifactName,
		manifestArtifactName,
		hashesArtifactName,
	}

	for _, personaID := range personaIDs {
		artifactState := requiredArtifactState{
			personaID: personaID,
			artifacts: make(map[string]resolvedArtifact, len(artifactsInOrder)),
		}
		row := PrepareGatePersona{
			PersonaID: personaID,
			Status:    statusPass,
		}

		for _, artifactName := range artifactsInOrder {
			path, err := storage.PreparePersonaArtifactPath(runRoot, personaID, artifactName)
			if err != nil {
				if row.Status == statusPass {
					row.Status = statusFail
					row.ErrorCode = errorArtifactPathInvalid
					row.ErrorMessage = fmt.Sprintf("resolve %s path: %v", artifactName, err)
					artifactState.firstFailure = errorArtifactPathInvalid
					artifactState.firstMessage = row.ErrorMessage
					artifactState.blockingArtifact = artifactName
				}
				continue
			}

			info, err := os.Stat(path)
			if err != nil {
				if os.IsNotExist(err) {
					if row.Status == statusPass {
						row.Status = statusFail
						row.ErrorCode = missingArtifactCode(artifactName)
						row.ErrorMessage = fmt.Sprintf("missing required artifact %s", artifactName)
						artifactState.firstFailure = row.ErrorCode
						artifactState.firstMessage = row.ErrorMessage
						artifactState.blockingArtifact = artifactName
					}
					continue
				}
				if row.Status == statusPass {
					row.Status = statusFail
					row.ErrorCode = errorArtifactUnreadable
					row.ErrorMessage = fmt.Sprintf("stat required artifact %s: %v", artifactName, err)
					artifactState.firstFailure = row.ErrorCode
					artifactState.firstMessage = row.ErrorMessage
					artifactState.blockingArtifact = artifactName
				}
				continue
			}
			if info.IsDir() {
				if row.Status == statusPass {
					row.Status = statusFail
					row.ErrorCode = errorArtifactUnreadable
					row.ErrorMessage = fmt.Sprintf("required artifact %s is a directory", artifactName)
					artifactState.firstFailure = row.ErrorCode
					artifactState.firstMessage = row.ErrorMessage
					artifactState.blockingArtifact = artifactName
				}
				continue
			}

			if _, err := os.ReadFile(path); err != nil {
				if row.Status == statusPass {
					row.Status = statusFail
					row.ErrorCode = errorArtifactUnreadable
					row.ErrorMessage = fmt.Sprintf("read required artifact %s: %v", artifactName, err)
					artifactState.firstFailure = row.ErrorCode
					artifactState.firstMessage = row.ErrorMessage
					artifactState.blockingArtifact = artifactName
				}
				continue
			}

			artifactState.artifacts[artifactName] = resolvedArtifact{
				path:     path,
				readable: true,
			}
		}

		state[personaID] = artifactState
		check.PersonaResults = append(check.PersonaResults, row)
	}

	check.Status = derivePersonaScopedCheckStatus(check.PersonaResults)
	return check, state
}

func gateCheckDispatchSchema(personaIDs []string, required map[string]requiredArtifactState) PrepareGateCheck {
	check := PrepareGateCheck{
		CheckID:        checkDispatchSchema,
		Status:         statusPass,
		PersonaResults: []PrepareGatePersona{},
	}
	if len(personaIDs) == 0 {
		return check
	}

	for _, personaID := range personaIDs {
		requiredState := required[personaID]
		row := PrepareGatePersona{
			PersonaID: personaID,
			Status:    statusPass,
		}

		if dispatchBlockedByRequiredArtifacts(requiredState) {
			row.Status = statusFail
			row.ErrorMessage = fmt.Sprintf("blocked by %s: %s", checkRequiredArtifacts, requiredState.firstFailure)
			check.PersonaResults = append(check.PersonaResults, row)
			continue
		}

		dispatchPath := requiredState.artifacts[dispatchInputArtifactName].path
		data, err := os.ReadFile(dispatchPath)
		if err != nil {
			row.Status = statusFail
			row.ErrorCode = errorDispatchInvalid
			row.ErrorMessage = fmt.Sprintf("read %s during schema check: %v", dispatchInputArtifactName, err)
			check.PersonaResults = append(check.PersonaResults, row)
			continue
		}

		var object map[string]json.RawMessage
		if err := json.Unmarshal(data, &object); err != nil || object == nil {
			row.Status = statusFail
			row.ErrorCode = errorDispatchInvalid
			row.ErrorMessage = fmt.Sprintf("%s is not a valid top-level object", dispatchInputArtifactName)
			check.PersonaResults = append(check.PersonaResults, row)
			continue
		}

		if !matchesApprovedDispatchFieldSet(object) {
			row.Status = statusFail
			row.ErrorCode = errorDispatchFieldsInvalid
			row.ErrorMessage = fmt.Sprintf("%s field set does not match the canonical five-field contract", dispatchInputArtifactName)
			check.PersonaResults = append(check.PersonaResults, row)
			continue
		}

		if !dispatchValuesAreAllStrings(object) {
			row.Status = statusFail
			row.ErrorCode = errorDispatchInvalid
			row.ErrorMessage = fmt.Sprintf("%s contains a non-string canonical field value", dispatchInputArtifactName)
		}
		check.PersonaResults = append(check.PersonaResults, row)
	}

	check.Status = derivePersonaScopedCheckStatus(check.PersonaResults)
	return check
}

func gateCheckHashAndBundle(personaIDs []string, required map[string]requiredArtifactState) PrepareGateCheck {
	check := PrepareGateCheck{
		CheckID:        checkHashAndBundle,
		Status:         statusPass,
		PersonaResults: []PrepareGatePersona{},
	}
	if len(personaIDs) == 0 {
		return check
	}

	for _, personaID := range personaIDs {
		requiredState := required[personaID]
		row := PrepareGatePersona{
			PersonaID: personaID,
			Status:    statusPass,
		}

		if requiredState.firstFailure != "" {
			row.Status = statusFail
			row.ErrorMessage = fmt.Sprintf("blocked by %s: %s", checkRequiredArtifacts, requiredState.firstFailure)
			check.PersonaResults = append(check.PersonaResults, row)
			continue
		}

		ledger, errCode, errMessage := readAndValidateHashLedger(requiredState.artifacts[hashesArtifactName].path)
		if errCode != "" {
			row.Status = statusFail
			row.ErrorCode = errCode
			row.ErrorMessage = errMessage
			check.PersonaResults = append(check.PersonaResults, row)
			continue
		}

		skipPersona := false
		for _, key := range []string{"dispatch_input_sha256", "agents_sha256", "prompt_sha256", "bundle_sha256"} {
			if !lowercaseSHA256Pattern.MatchString(ledger[key]) {
				row.Status = statusFail
				row.ErrorCode = errorHashFormatInvalid
				row.ErrorMessage = fmt.Sprintf("hash key %s is not a lowercase sha256 digest", key)
				check.PersonaResults = append(check.PersonaResults, row)
				skipPersona = true
				break
			}
		}
		if skipPersona {
			continue
		}

		manifest, manifestCode, manifestMessage := readAndValidateManifest(requiredState.artifacts[manifestArtifactName].path, personaID)
		if manifestCode != "" {
			row.Status = statusFail
			row.ErrorCode = manifestCode
			row.ErrorMessage = manifestMessage
			check.PersonaResults = append(check.PersonaResults, row)
			continue
		}

		payloads := make([][]byte, 0, len(canonicalSendableArtifacts))
		for _, artifactName := range canonicalSendableArtifacts {
			artifactPath := requiredState.artifacts[artifactName].path
			payload, err := os.ReadFile(artifactPath)
				if err != nil {
					row.Status = statusFail
					row.ErrorCode = errorHashPayloadRead
					row.ErrorMessage = fmt.Sprintf("read payload %s: %v", artifactName, err)
					check.PersonaResults = append(check.PersonaResults, row)
					skipPersona = true
					break
				}
				payloads = append(payloads, payload)
			}
			if skipPersona {
				continue
			}

			if manifestViolation := validateManifestAgainstLedgerAndPayloads(manifest, ledger, payloads); manifestViolation != "" {
				row.Status = statusFail
				row.ErrorCode = errorManifestInvalid
				row.ErrorMessage = manifestViolation
			check.PersonaResults = append(check.PersonaResults, row)
			continue
		}

		for index, artifactName := range canonicalSendableArtifacts {
			actualDigest := panelhash.SHA256Hex(payloads[index])
			if actualDigest != ledger[hashRefByArtifact[artifactName]] {
				row.Status = statusFail
				row.ErrorCode = errorHashMismatch
				row.ErrorMessage = fmt.Sprintf("payload digest mismatch for %s", artifactName)
				check.PersonaResults = append(check.PersonaResults, row)
				skipPersona = true
				break
			}
		}
		if skipPersona {
			continue
		}

		if bundleSHA256(payloads...) != ledger["bundle_sha256"] {
			row.Status = statusFail
			row.ErrorCode = errorBundleMismatch
			row.ErrorMessage = "bundle_sha256 does not match canonical payload framing"
		}
		check.PersonaResults = append(check.PersonaResults, row)
	}

	check.Status = derivePersonaScopedCheckStatus(check.PersonaResults)
	return check
}

func gateCheckAuditWritability(runRoot string) PrepareGateCheck {
	check := PrepareGateCheck{
		CheckID:        checkAuditWritability,
		Status:         statusPass,
		PersonaResults: []PrepareGatePersona{},
	}

	for _, auditPath := range []string{storage.AuditEventsPath(runRoot), storage.AuditErrorsPath(runRoot)} {
		file, err := os.OpenFile(auditPath, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			check.Status = statusFail
			check.ErrorCode = errorAuditNotWritable
			check.ErrorMessage = fmt.Sprintf("audit path is not writable: %s", auditPath)
			return check
		}
		_ = file.Close()
	}

	return check
}

func readAndValidateHashLedger(path string) (map[string]string, string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errorHashEvidenceRead, fmt.Sprintf("read %s: %v", hashesArtifactName, err)
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return nil, errorHashLedgerInvalid, fmt.Sprintf("%s is not a valid top-level object", hashesArtifactName)
	}

	allowedKeys := map[string]struct{}{
		"dispatch_input_sha256": {},
		"agents_sha256":         {},
		"prompt_sha256":         {},
		"bundle_sha256":         {},
	}
	for key := range object {
		if _, ok := allowedKeys[key]; !ok {
			return nil, errorHashLedgerInvalid, fmt.Sprintf("%s contains unsupported key %q", hashesArtifactName, key)
		}
	}

	ledger := make(map[string]string, len(allowedKeys))
	for _, key := range []string{"dispatch_input_sha256", "agents_sha256", "prompt_sha256", "bundle_sha256"} {
		rawValue, ok := object[key]
		if !ok {
			return nil, errorHashLedgerMissing, fmt.Sprintf("%s is missing required key %q", hashesArtifactName, key)
		}
		var decoded string
		if err := json.Unmarshal(rawValue, &decoded); err != nil {
			return nil, errorHashLedgerInvalid, fmt.Sprintf("%s key %q must be a string", hashesArtifactName, key)
		}
		ledger[key] = decoded
	}

	return ledger, "", ""
}

func readAndValidateManifest(path, personaID string) (PrepareManifest, string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PrepareManifest{}, errorHashEvidenceRead, fmt.Sprintf("read %s: %v", manifestArtifactName, err)
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return PrepareManifest{}, errorManifestInvalid, fmt.Sprintf("%s is not a valid top-level object", manifestArtifactName)
	}

	requiredKeys := map[string]struct{}{
		"schema_version": {},
		"stage":          {},
		"persona_id":     {},
		"artifacts":      {},
	}
	if len(object) != len(requiredKeys) {
		return PrepareManifest{}, errorManifestInvalid, fmt.Sprintf("%s must contain exactly schema_version, stage, persona_id, and artifacts", manifestArtifactName)
	}
	for key := range object {
		if _, ok := requiredKeys[key]; !ok {
			return PrepareManifest{}, errorManifestInvalid, fmt.Sprintf("%s contains unsupported key %q", manifestArtifactName, key)
		}
	}

	var manifest PrepareManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return PrepareManifest{}, errorManifestInvalid, fmt.Sprintf("decode %s: %v", manifestArtifactName, err)
	}

	if manifest.SchemaVersion != prepareManifestSchemaVersion || manifest.Stage != prepareStageName {
		return PrepareManifest{}, errorManifestInvalid, fmt.Sprintf("%s literals must be schema_version=%q and stage=%q", manifestArtifactName, prepareManifestSchemaVersion, prepareStageName)
	}
	if manifest.PersonaID == "" {
		return PrepareManifest{}, errorManifestInvalid, fmt.Sprintf("%s persona_id must be a non-empty string", manifestArtifactName)
	}
	if manifest.PersonaID != personaID {
		return PrepareManifest{}, errorManifestPersonaID, fmt.Sprintf("%s persona_id %q does not match directory persona_id %q", manifestArtifactName, manifest.PersonaID, personaID)
	}
	if len(manifest.Artifacts) != len(canonicalSendableArtifacts) {
		return PrepareManifest{}, errorManifestInvalid, fmt.Sprintf("%s must contain exactly three artifact entries", manifestArtifactName)
	}

	expectedEntries := []PrepareManifestMember{
		{Path: dispatchInputArtifactName, HashRef: "dispatch_input_sha256"},
		{Path: agentsArtifactName, HashRef: "agents_sha256"},
		{Path: promptArtifactName, HashRef: "prompt_sha256"},
	}
	for index, expected := range expectedEntries {
		member := manifest.Artifacts[index]
		if member.Path != expected.Path || member.HashRef != expected.HashRef || member.SizeBytes < 0 {
			return PrepareManifest{}, errorManifestInvalid, fmt.Sprintf("%s artifact entry %d violates canonical path/hash_ref ordering", manifestArtifactName, index)
		}
	}

	return manifest, "", ""
}

func validateManifestAgainstLedgerAndPayloads(manifest PrepareManifest, ledger map[string]string, payloads [][]byte) string {
	for index, member := range manifest.Artifacts {
		if _, ok := ledger[member.HashRef]; !ok {
			return fmt.Sprintf("%s hash_ref %q is not present in %s", manifestArtifactName, member.HashRef, hashesArtifactName)
		}
		if int64(len(payloads[index])) != member.SizeBytes {
			return fmt.Sprintf("%s size_bytes for %s does not match payload size", manifestArtifactName, member.Path)
		}
	}
	return ""
}

func dispatchBlockedByRequiredArtifacts(state requiredArtifactState) bool {
	switch state.blockingArtifact {
	case dispatchInputArtifactName:
		return state.firstFailure == errorArtifactPathInvalid ||
			state.firstFailure == errorMissingDispatch ||
			state.firstFailure == errorArtifactUnreadable
	default:
		return false
	}
}

func matchesApprovedDispatchFieldSet(object map[string]json.RawMessage) bool {
	if len(object) != len(approvedDispatchKeys) {
		return false
	}
	for _, key := range approvedDispatchKeys {
		if _, ok := object[key]; !ok {
			return false
		}
	}
	return true
}

func dispatchValuesAreAllStrings(object map[string]json.RawMessage) bool {
	for _, key := range approvedDispatchKeys {
		var value string
		if err := json.Unmarshal(object[key], &value); err != nil {
			return false
		}
	}
	return true
}

func missingArtifactCode(artifactName string) string {
	switch artifactName {
	case dispatchInputArtifactName:
		return errorMissingDispatch
	case agentsArtifactName:
		return errorMissingAgents
	case promptArtifactName:
		return errorMissingPrompt
	case manifestArtifactName:
		return errorMissingManifest
	case hashesArtifactName:
		return errorMissingHashes
	default:
		return errorArtifactUnreadable
	}
}

func derivePersonaScopedCheckStatus(rows []PrepareGatePersona) string {
	for _, row := range rows {
		if row.Status == statusFail {
			return statusFail
		}
	}
	return statusPass
}

func deriveOverallStatus(checks []PrepareGateCheck) string {
	for _, check := range checks {
		if check.Status == statusFail {
			return statusFail
		}
	}
	return statusPass
}
