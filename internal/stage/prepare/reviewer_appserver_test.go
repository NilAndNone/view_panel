package prepare

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"view_panel/internal/appserver"
	"view_panel/internal/storage"
)

func TestAppServerReviewerReviewPrepareUsesSkillAndAuthoritativeMessage(t *testing.T) {
	env := newFakeCodexEnv(t)

	expected := `{"schema_version":"prepare_review_v1","stage":"01_prepare","review_mode":"advisory","status":"completed_clean","summary":{"non_blocking":true,"stage2_dependency":"none","review_completed":true,"persona_count":1,"finding_count":0,"category_counts":{"weak_persona_differentiation":0,"suspicious_material_assembly":0,"output_contract_issue":0,"review_execution":0}},"findings":[]}`
	if err := os.WriteFile(env.responsePath, []byte(expected), 0o644); err != nil {
		t.Fatalf("write fake response: %v", err)
	}

	reviewer := NewAppServerReviewer(env.repoRoot, []string{"--model", "test-model"}, env.homeDir)
	response, err := reviewer.ReviewPrepare(context.Background(), AdvisoryReviewRequest{
		SchemaVersion: prepareReviewSchemaVersion,
		Stage:         prepareStageName,
		ReviewMode:    reviewModeAdvisory,
		SkillPath:     prepareReviewSkillPath,
		Personas: []AdvisoryReviewPersona{
			{
				PersonaID: "alpha",
				Bundle: AdvisoryReviewBundle{
					Agents: "persona alpha",
					Prompt: "question",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ReviewPrepare returned error: %v", err)
	}
	if got := string(response); got != expected {
		t.Fatalf("ReviewPrepare response mismatch\nwant: %s\ngot:  %s", expected, got)
	}

	turnStart := env.loggedTurnStartRequest(t)
	var envelope struct {
		Method string                    `json:"method"`
		Params appserver.TurnStartParams `json:"params"`
	}
	if err := json.Unmarshal([]byte(turnStart), &envelope); err != nil {
		t.Fatalf("decode logged turn/start request: %v", err)
	}
	if envelope.Method != appserver.MethodTurnStart {
		t.Fatalf("expected logged method %q, got %q", appserver.MethodTurnStart, envelope.Method)
	}
	if len(envelope.Params.Input) != 1 {
		t.Fatalf("expected exactly one turn input item, got %d", len(envelope.Params.Input))
	}

	inputText := envelope.Params.Input[0].Text
	if !strings.Contains(inputText, "# worldview-panel prepare-stage advisory review") {
		t.Fatalf("turn input did not include advisory skill content: %q", inputText)
	}
	if !strings.Contains(inputText, `"review_mode":"advisory"`) {
		t.Fatalf("turn input did not include serialized review request: %q", inputText)
	}
	if !strings.Contains(inputText, `"persona_id":"alpha"`) {
		t.Fatalf("turn input did not include persona payload: %q", inputText)
	}
}

func TestBuildOrFallbackPrepareReviewSynthesizesReviewUnavailableOnInvalidReviewerOutput(t *testing.T) {
	env := newFakeCodexEnv(t)

	if err := os.WriteFile(env.responsePath, []byte("not-json"), 0o644); err != nil {
		t.Fatalf("write fake response: %v", err)
	}

	runRoot := filepath.Join(env.baseDir, "out", "runs", "run-1")
	writePreparedPersonaBundle(t, runRoot, "alpha")

	artifact, err := buildOrFallbackPrepareReview(context.Background(), NewAppServerReviewer(env.repoRoot, nil, env.homeDir), runRoot)
	if err != nil {
		t.Fatalf("buildOrFallbackPrepareReview returned error: %v", err)
	}
	if artifact.Status != reviewStatusUnavailable {
		t.Fatalf("expected fallback status %q, got %q", reviewStatusUnavailable, artifact.Status)
	}
	if artifact.Summary.ReviewCompleted {
		t.Fatalf("expected fallback review_completed=false")
	}
	if artifact.Summary.PersonaCount != 1 {
		t.Fatalf("expected persona_count 1, got %d", artifact.Summary.PersonaCount)
	}
	if artifact.Summary.CategoryCounts.ReviewExecution != 1 {
		t.Fatalf("expected review_execution count 1, got %d", artifact.Summary.CategoryCounts.ReviewExecution)
	}
	if len(artifact.Findings) != 1 {
		t.Fatalf("expected one fallback finding, got %d", len(artifact.Findings))
	}
	if artifact.Findings[0].Category != "review_execution" {
		t.Fatalf("expected fallback category review_execution, got %q", artifact.Findings[0].Category)
	}
	if !strings.Contains(artifact.Findings[0].Message, "review response is not valid JSON") {
		t.Fatalf("expected invalid JSON fallback message, got %q", artifact.Findings[0].Message)
	}
}

func TestBuildOrFallbackPrepareReviewSynthesizesReviewUnavailableWhenReviewTurnHasNoAuthoritativeCompletion(t *testing.T) {
	env := newFakeCodexEnv(t)
	t.Setenv("CODEX_FAKE_MODE", "no-completion")

	runRoot := filepath.Join(env.baseDir, "out", "runs", "run-1")
	writePreparedPersonaBundle(t, runRoot, "alpha")

	artifact, err := buildOrFallbackPrepareReview(context.Background(), NewAppServerReviewer(env.repoRoot, nil, env.homeDir), runRoot)
	if err != nil {
		t.Fatalf("buildOrFallbackPrepareReview returned error: %v", err)
	}

	assertReviewUnavailable(t, artifact, 1, "review session failed: run review turn:")
}

func TestBuildOrFallbackPrepareReviewSynthesizesReviewUnavailableWhenAuthoritativeMessageIsEmpty(t *testing.T) {
	env := newFakeCodexEnv(t)
	t.Setenv("CODEX_FAKE_MODE", "empty-completed")

	runRoot := filepath.Join(env.baseDir, "out", "runs", "run-1")
	writePreparedPersonaBundle(t, runRoot, "alpha")

	artifact, err := buildOrFallbackPrepareReview(context.Background(), NewAppServerReviewer(env.repoRoot, nil, env.homeDir), runRoot)
	if err != nil {
		t.Fatalf("buildOrFallbackPrepareReview returned error: %v", err)
	}

	assertReviewUnavailable(t, artifact, 1, "review turn returned an empty authoritative agent message")
}

func assertReviewUnavailable(t *testing.T, artifact PrepareReviewArtifact, personaCount int, findingMessageSubstring string) {
	t.Helper()

	if artifact.Status != reviewStatusUnavailable {
		t.Fatalf("expected fallback status %q, got %q", reviewStatusUnavailable, artifact.Status)
	}
	if !artifact.Summary.NonBlocking {
		t.Fatalf("expected fallback review to remain non-blocking")
	}
	if artifact.Summary.Stage2Dependency != "none" {
		t.Fatalf("expected stage2_dependency %q, got %q", "none", artifact.Summary.Stage2Dependency)
	}
	if artifact.Summary.ReviewCompleted {
		t.Fatalf("expected fallback review_completed=false")
	}
	if artifact.Summary.PersonaCount != personaCount {
		t.Fatalf("expected persona_count %d, got %d", personaCount, artifact.Summary.PersonaCount)
	}
	if artifact.Summary.CategoryCounts.ReviewExecution != 1 {
		t.Fatalf("expected review_execution count 1, got %d", artifact.Summary.CategoryCounts.ReviewExecution)
	}
	if len(artifact.Findings) != 1 {
		t.Fatalf("expected one fallback finding, got %d", len(artifact.Findings))
	}
	if artifact.Findings[0].Category != "review_execution" {
		t.Fatalf("expected fallback category review_execution, got %q", artifact.Findings[0].Category)
	}
	if artifact.Findings[0].BlocksStage2 {
		t.Fatalf("expected fallback finding to remain non-blocking")
	}
	if !strings.Contains(artifact.Findings[0].Message, findingMessageSubstring) {
		t.Fatalf("expected fallback message to contain %q, got %q", findingMessageSubstring, artifact.Findings[0].Message)
	}
}

type fakeCodexEnv struct {
	baseDir      string
	repoRoot     string
	homeDir      string
	responsePath string
	logPath      string
}

func newFakeCodexEnv(t *testing.T) fakeCodexEnv {
	t.Helper()

	baseDir := t.TempDir()
	repoRoot := filepath.Join(baseDir, "repo")
	homeDir := filepath.Join(baseDir, "home")
	responsePath := filepath.Join(baseDir, "fake-response.txt")
	logPath := filepath.Join(baseDir, "codex.log")

	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "skills", "wv-prepare-stage"), 0o755); err != nil {
		t.Fatalf("create fake repo skill directory: %v", err)
	}
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("create fake home directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, prepareReviewSkillPath), []byte("# worldview-panel prepare-stage advisory review"), 0o644); err != nil {
		t.Fatalf("write fake skill file: %v", err)
	}

	codexPath := filepath.Join(baseDir, "codex")
	if err := os.WriteFile(codexPath, []byte(fakeCodexScript), 0o755); err != nil {
		t.Fatalf("write fake codex executable: %v", err)
	}

	t.Setenv("PATH", baseDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CODEX_FAKE_RESPONSE_FILE", responsePath)
	t.Setenv("CODEX_FAKE_LOG", logPath)

	return fakeCodexEnv{
		baseDir:      baseDir,
		repoRoot:     repoRoot,
		homeDir:      homeDir,
		responsePath: responsePath,
		logPath:      logPath,
	}
}

func (e fakeCodexEnv) loggedTurnStartRequest(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(e.logPath)
	if err != nil {
		t.Fatalf("read fake codex log: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.Contains(line, `"method":"turn/start"`) {
			return line
		}
	}
	t.Fatalf("turn/start request not found in log %q", string(data))
	return ""
}

func writePreparedPersonaBundle(t *testing.T, runRoot, personaID string) {
	t.Helper()

	dispatchPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, dispatchInputArtifactName)
	if err != nil {
		t.Fatalf("resolve dispatch input path: %v", err)
	}
	agentsPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, agentsArtifactName)
	if err != nil {
		t.Fatalf("resolve agents path: %v", err)
	}
	promptPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, promptArtifactName)
	if err != nil {
		t.Fatalf("resolve prompt path: %v", err)
	}
	manifestPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, manifestArtifactName)
	if err != nil {
		t.Fatalf("resolve manifest path: %v", err)
	}
	hashesPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, hashesArtifactName)
	if err != nil {
		t.Fatalf("resolve hashes path: %v", err)
	}

	if err := storage.WriteJSON(dispatchPath, DispatchInputV1{
		RoleplayPrompt:            "persona alpha",
		DiscussionQuestion:        "What should happen?",
		SupplementaryMaterials:    "materials",
		OutputContract:            "return json",
		AssumptionsAndConstraints: "none",
	}); err != nil {
		t.Fatalf("write dispatch input: %v", err)
	}
	if err := storage.WriteText(agentsPath, "persona alpha"); err != nil {
		t.Fatalf("write agents.md: %v", err)
	}
	if err := storage.WriteText(promptPath, "What should happen?\n\nmaterials\n\nreturn json\n\nnone"); err != nil {
		t.Fatalf("write prompt.txt: %v", err)
	}
	if err := storage.WriteJSON(manifestPath, PrepareManifest{
		SchemaVersion: prepareManifestSchemaVersion,
		Stage:         prepareStageName,
		PersonaID:     personaID,
	}); err != nil {
		t.Fatalf("write manifest.json: %v", err)
	}
	if err := storage.WriteJSON(hashesPath, PrepareHashes{
		DispatchInputSHA256: "dispatch",
		AgentsSHA256:        "agents",
		PromptSHA256:        "prompt",
		BundleSHA256:        "bundle",
	}); err != nil {
		t.Fatalf("write hashes.json: %v", err)
	}
}

const fakeCodexScript = `#!/bin/sh
set -eu

response_file="${CODEX_FAKE_RESPONSE_FILE:-}"
log_file="${CODEX_FAKE_LOG:-}"
mode="${CODEX_FAKE_MODE:-completed}"

json_escape() {
	sed \
		-e 's/\\/\\\\/g' \
		-e 's/"/\\"/g' \
		-e ':a;N;$!ba;s/\n/\\n/g'
}

request_id() {
	printf '%s\n' "$1" | sed -n 's/.*"id":\([0-9][0-9]*\).*/\1/p'
}

while IFS= read -r line; do
	if [ -n "$log_file" ]; then
		printf '%s\n' "$line" >> "$log_file"
	fi

	case "$line" in
		*'"method":"initialize"'*)
			id=$(request_id "$line")
			printf '{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":"0.0.0","serverInfo":{"name":"fake-codex"}}}\n' "$id"
			;;
		*'"method":"initialized"'*)
			:
			;;
		*'"method":"configRequirements/read"'*)
			id=$(request_id "$line")
			printf '{"jsonrpc":"2.0","id":%s,"result":{"requirements":[]}}\n' "$id"
			;;
		*'"method":"thread/start"'*)
			id=$(request_id "$line")
			printf '{"jsonrpc":"2.0","id":%s,"result":{"threadId":"thread-1"}}\n' "$id"
			;;
		*'"method":"turn/start"'*)
			id=$(request_id "$line")
			printf '{"jsonrpc":"2.0","id":%s,"result":{"turnId":"turn-1"}}\n' "$id"
			case "$mode" in
				no-completion)
					exit 0
					;;
				empty-completed)
					response=''
					;;
				*)
					if [ -n "$response_file" ] && [ -f "$response_file" ]; then
						response=$(json_escape < "$response_file")
					else
						response='{}'
					fi
					;;
			esac
			printf '{"jsonrpc":"2.0","method":"item/completed.agentMessage","params":{"item":{"id":"msg-1","type":"message","role":"assistant","text":"%s"}}}\n' "$response"
			;;
	esac
done
`
