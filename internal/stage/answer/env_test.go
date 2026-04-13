package answer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	panelhash "view_panel/internal/hash"
	"view_panel/internal/storage"
)

func TestSealWorkspacesSealsExecutionEnv(t *testing.T) {
	workspace := sealWorkspaceForTest(t)

	wantExecutionEnv := sealedExecutionEnvForRoot(workspace.IsolatedRoot)
	if workspace.ExecutionEnv != wantExecutionEnv {
		t.Fatalf("execution env = %#v, want %#v", workspace.ExecutionEnv, wantExecutionEnv)
	}

	wantHomeDir := filepath.Join(workspace.IsolatedRoot, "home")
	wantCodexHomeDir := filepath.Join(wantHomeDir, ".codex")

	if info, err := os.Stat(wantCodexHomeDir); err != nil {
		t.Fatalf("stat codex home dir: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("codex home path %q is not a directory", wantCodexHomeDir)
	}
}

func TestSealWorkspacesCopiesAllowlistedParentCodexFilesFromCODEX_HOME(t *testing.T) {
	workspace := sealWorkspaceForTestWithParentCodex(t, func(parentHomeDir string, parentCodexHomeDir string) {
		writeTestFile(t, filepath.Join(parentCodexHomeDir, "config.toml"), []byte("fallback-config = true\n"))
		writeTestFile(t, filepath.Join(parentCodexHomeDir, "auth.json"), []byte(`{"token":"fallback"}`))

		overrideCodexHomeDir := filepath.Join(parentHomeDir, "override-codex-home")
		t.Setenv("CODEX_HOME", overrideCodexHomeDir)
		writeTestFile(t, filepath.Join(overrideCodexHomeDir, "config.toml"), []byte("model = \"gpt-5\"\n"))
		writeTestFile(t, filepath.Join(overrideCodexHomeDir, "auth.json"), []byte(`{"token":"override"}`))
		writeTestFile(t, filepath.Join(overrideCodexHomeDir, "sessions", "session.json"), []byte(`{"id":"session-1"}`))
		writeTestFile(t, filepath.Join(overrideCodexHomeDir, "cache.db"), []byte("cache"))
	})

	configBytes, err := os.ReadFile(filepath.Join(workspace.ExecutionEnv.CodexHomeDir, "config.toml"))
	if err != nil {
		t.Fatalf("read isolated config.toml: %v", err)
	}
	if string(configBytes) != "model = \"gpt-5\"\n" {
		t.Fatalf("isolated config.toml = %q, want override content", string(configBytes))
	}

	authBytes, err := os.ReadFile(filepath.Join(workspace.ExecutionEnv.CodexHomeDir, "auth.json"))
	if err != nil {
		t.Fatalf("read isolated auth.json: %v", err)
	}
	if string(authBytes) != `{"token":"override"}` {
		t.Fatalf("isolated auth.json = %q, want override content", string(authBytes))
	}

	for _, unexpectedPath := range []string{
		filepath.Join(workspace.ExecutionEnv.CodexHomeDir, "sessions"),
		filepath.Join(workspace.ExecutionEnv.CodexHomeDir, "cache.db"),
	} {
		if _, err := os.Stat(unexpectedPath); err == nil {
			t.Fatalf("unexpected isolated Codex runtime artifact at %q", unexpectedPath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %q: %v", unexpectedPath, err)
		}
	}
}

func TestSealWorkspacesCopiesAllowlistedParentCodexFilesFromUserHomeFallback(t *testing.T) {
	workspace := sealWorkspaceForTestWithParentCodex(t, func(_ string, parentCodexHomeDir string) {
		writeTestFile(t, filepath.Join(parentCodexHomeDir, "auth.json"), []byte(`{"token":"home-fallback"}`))
		writeTestFile(t, filepath.Join(parentCodexHomeDir, "logs", "latest.log"), []byte("log"))
	})

	authBytes, err := os.ReadFile(filepath.Join(workspace.ExecutionEnv.CodexHomeDir, "auth.json"))
	if err != nil {
		t.Fatalf("read isolated auth.json: %v", err)
	}
	if string(authBytes) != `{"token":"home-fallback"}` {
		t.Fatalf("isolated auth.json = %q, want fallback content", string(authBytes))
	}

	for _, unexpectedPath := range []string{
		filepath.Join(workspace.ExecutionEnv.CodexHomeDir, "config.toml"),
		filepath.Join(workspace.ExecutionEnv.CodexHomeDir, "logs"),
	} {
		if _, err := os.Stat(unexpectedPath); err == nil {
			t.Fatalf("unexpected isolated Codex runtime artifact at %q", unexpectedPath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %q: %v", unexpectedPath, err)
		}
	}
}

func TestSealWorkspacesWritesIsolatedAuthJSONWithOwnerOnlyPermissions(t *testing.T) {
	workspace := sealWorkspaceForTestWithParentCodex(t, func(_ string, parentCodexHomeDir string) {
		writeTestFile(t, filepath.Join(parentCodexHomeDir, "auth.json"), []byte(`{"token":"secret"}`))
	})

	authPath := filepath.Join(workspace.ExecutionEnv.CodexHomeDir, "auth.json")
	info, err := os.Stat(authPath)
	if err != nil {
		t.Fatalf("stat isolated auth.json: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("isolated auth.json perm = %#o, want %#o", got, 0o600)
	}
}

func TestSealWorkspacesRejectsEscapingPersonaIDBeforeClearingIsolatedRoot(t *testing.T) {
	testRoot := t.TempDir()
	parentHomeDir := filepath.Join(testRoot, "parent-home")
	repoRoot := filepath.Join(testRoot, "repo")
	runRoot := filepath.Join(testRoot, "runs", "run-test")
	isolatedBaseDir := filepath.Join(testRoot, "isolated")
	personaID := filepath.Join("..", "escape")
	skillRelativePath := filepath.Join("runtime", "skills", "answer", "SKILL.md")
	skillSourcePath := filepath.Join(repoRoot, skillRelativePath)

	if err := os.MkdirAll(parentHomeDir, 0o755); err != nil {
		t.Fatalf("create parent home: %v", err)
	}
	t.Setenv("HOME", parentHomeDir)
	t.Setenv("CODEX_HOME", "")

	if err := os.MkdirAll(filepath.Dir(skillSourcePath), 0o755); err != nil {
		t.Fatalf("create skill directory: %v", err)
	}
	if err := os.WriteFile(skillSourcePath, []byte("test skill instructions"), 0o644); err != nil {
		t.Fatalf("write skill source: %v", err)
	}

	gatePath, err := storage.ResolvePath(storage.PrepareRoot(runRoot), prepareGateStatusArtifactName)
	if err != nil {
		t.Fatalf("resolve gate path: %v", err)
	}
	if err := storage.WriteJSON(gatePath, prepareGateStatus{
		CanProceedToStage2: true,
		PersonaIDs:         []string{personaID},
	}); err != nil {
		t.Fatalf("write gate artifact: %v", err)
	}

	agentsPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "agents.md")
	if err != nil {
		t.Fatalf("resolve agents path: %v", err)
	}
	promptPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "prompt.txt")
	if err != nil {
		t.Fatalf("resolve prompt path: %v", err)
	}
	hashesPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "hashes.json")
	if err != nil {
		t.Fatalf("resolve hashes path: %v", err)
	}

	agentsBytes := []byte("agent instructions")
	promptBytes := []byte("discussion prompt")
	if err := storage.WriteText(agentsPath, string(agentsBytes)); err != nil {
		t.Fatalf("write agents artifact: %v", err)
	}
	if err := storage.WriteText(promptPath, string(promptBytes)); err != nil {
		t.Fatalf("write prompt artifact: %v", err)
	}
	if err := storage.WriteJSON(hashesPath, prepareHashLedger{
		DispatchInputSHA256: panelhash.SHA256Hex([]byte("dispatch input")),
		AgentsSHA256:        panelhash.SHA256Hex(agentsBytes),
		PromptSHA256:        panelhash.SHA256Hex(promptBytes),
		BundleSHA256:        panelhash.SHA256Hex([]byte("bundle")),
	}); err != nil {
		t.Fatalf("write hashes artifact: %v", err)
	}

	markerPath := filepath.Join(isolatedBaseDir, "escape", "marker.txt")
	writeTestFile(t, markerPath, []byte("do not delete"))

	_, err = SealWorkspaces(SealWorkspacesRequest{
		RepoRoot:        repoRoot,
		RunRoot:         runRoot,
		SkillSourcePath: skillRelativePath,
		IsolatedBaseDir: isolatedBaseDir,
	})
	if err == nil {
		t.Fatalf("expected SealWorkspaces to reject escaping persona_id %q", personaID)
	}

	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("marker under escaped isolated path was touched: %v", err)
	}
}

func TestSealWorkspacesClearsIsolatedRootOnReseal(t *testing.T) {
	testRoot := t.TempDir()
	parentHomeDir := filepath.Join(testRoot, "parent-home")
	repoRoot := filepath.Join(testRoot, "repo")
	runRoot := filepath.Join(testRoot, "runs", "run-test")
	isolatedBaseDir := filepath.Join(testRoot, "isolated")
	personaID := "persona-test"
	skillRelativePath := filepath.Join("runtime", "skills", "answer", "SKILL.md")
	skillSourcePath := filepath.Join(repoRoot, skillRelativePath)

	if err := os.MkdirAll(parentHomeDir, 0o755); err != nil {
		t.Fatalf("create parent home: %v", err)
	}
	t.Setenv("HOME", parentHomeDir)
	t.Setenv("CODEX_HOME", "")
	writeTestFile(t, filepath.Join(parentHomeDir, ".codex", "config.toml"), []byte("model = \"gpt-5\"\n"))

	if err := os.MkdirAll(filepath.Dir(skillSourcePath), 0o755); err != nil {
		t.Fatalf("create skill directory: %v", err)
	}
	if err := os.WriteFile(skillSourcePath, []byte("test skill instructions"), 0o644); err != nil {
		t.Fatalf("write skill source: %v", err)
	}

	gatePath, err := storage.ResolvePath(storage.PrepareRoot(runRoot), prepareGateStatusArtifactName)
	if err != nil {
		t.Fatalf("resolve gate path: %v", err)
	}
	if err := storage.WriteJSON(gatePath, prepareGateStatus{
		CanProceedToStage2: true,
		PersonaIDs:         []string{personaID},
	}); err != nil {
		t.Fatalf("write gate artifact: %v", err)
	}

	agentsPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "agents.md")
	if err != nil {
		t.Fatalf("resolve agents path: %v", err)
	}
	promptPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "prompt.txt")
	if err != nil {
		t.Fatalf("resolve prompt path: %v", err)
	}
	hashesPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "hashes.json")
	if err != nil {
		t.Fatalf("resolve hashes path: %v", err)
	}

	agentsBytes := []byte("agent instructions")
	promptBytes := []byte("discussion prompt")
	if err := storage.WriteText(agentsPath, string(agentsBytes)); err != nil {
		t.Fatalf("write agents artifact: %v", err)
	}
	if err := storage.WriteText(promptPath, string(promptBytes)); err != nil {
		t.Fatalf("write prompt artifact: %v", err)
	}
	if err := storage.WriteJSON(hashesPath, prepareHashLedger{
		DispatchInputSHA256: panelhash.SHA256Hex([]byte("dispatch input")),
		AgentsSHA256:        panelhash.SHA256Hex(agentsBytes),
		PromptSHA256:        panelhash.SHA256Hex(promptBytes),
		BundleSHA256:        panelhash.SHA256Hex([]byte("bundle")),
	}); err != nil {
		t.Fatalf("write hashes artifact: %v", err)
	}

	req := SealWorkspacesRequest{
		RepoRoot:        repoRoot,
		RunRoot:         runRoot,
		SkillSourcePath: skillRelativePath,
		IsolatedBaseDir: isolatedBaseDir,
	}

	firstSealed, err := SealWorkspaces(req)
	if err != nil {
		t.Fatalf("first SealWorkspaces returned error: %v", err)
	}
	if len(firstSealed.Workspaces) != 1 {
		t.Fatalf("first sealed workspace count = %d, want 1", len(firstSealed.Workspaces))
	}

	staleCodexPath := filepath.Join(firstSealed.Workspaces[0].ExecutionEnv.CodexHomeDir, "sessions", "stale-session.json")
	writeTestFile(t, staleCodexPath, []byte(`{"id":"stale-session"}`))

	secondSealed, err := SealWorkspaces(req)
	if err != nil {
		t.Fatalf("second SealWorkspaces returned error: %v", err)
	}
	if len(secondSealed.Workspaces) != 1 {
		t.Fatalf("second sealed workspace count = %d, want 1", len(secondSealed.Workspaces))
	}

	if _, err := os.Stat(staleCodexPath); err == nil {
		t.Fatalf("stale isolated Codex file %q survived reseal", staleCodexPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat stale isolated Codex file %q: %v", staleCodexPath, err)
	}

	configBytes, err := os.ReadFile(filepath.Join(secondSealed.Workspaces[0].ExecutionEnv.CodexHomeDir, "config.toml"))
	if err != nil {
		t.Fatalf("read resealed isolated config.toml: %v", err)
	}
	if string(configBytes) != "model = \"gpt-5\"\n" {
		t.Fatalf("resealed isolated config.toml = %q, want parent allowlisted content", string(configBytes))
	}
}

func TestSealWorkspacesRejectsIsolatedBaseUnderOverriddenParentCodexHome(t *testing.T) {
	testRoot := t.TempDir()
	parentHomeDir := filepath.Join(testRoot, "parent-home")
	overrideCodexHomeDir := filepath.Join(testRoot, "live-codex-home")
	repoRoot := filepath.Join(testRoot, "repo")
	runRoot := filepath.Join(testRoot, "runs", "run-test")
	isolatedBaseDir := filepath.Join(overrideCodexHomeDir, "isolated")
	personaID := "persona-test"
	skillRelativePath := filepath.Join("runtime", "skills", "answer", "SKILL.md")
	skillSourcePath := filepath.Join(repoRoot, skillRelativePath)

	if err := os.MkdirAll(parentHomeDir, 0o755); err != nil {
		t.Fatalf("create parent home: %v", err)
	}
	t.Setenv("HOME", parentHomeDir)
	t.Setenv("CODEX_HOME", overrideCodexHomeDir)

	if err := os.MkdirAll(filepath.Dir(skillSourcePath), 0o755); err != nil {
		t.Fatalf("create skill directory: %v", err)
	}
	if err := os.WriteFile(skillSourcePath, []byte("test skill instructions"), 0o644); err != nil {
		t.Fatalf("write skill source: %v", err)
	}

	gatePath, err := storage.ResolvePath(storage.PrepareRoot(runRoot), prepareGateStatusArtifactName)
	if err != nil {
		t.Fatalf("resolve gate path: %v", err)
	}
	if err := storage.WriteJSON(gatePath, prepareGateStatus{
		CanProceedToStage2: true,
		PersonaIDs:         []string{personaID},
	}); err != nil {
		t.Fatalf("write gate artifact: %v", err)
	}

	agentsPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "agents.md")
	if err != nil {
		t.Fatalf("resolve agents path: %v", err)
	}
	promptPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "prompt.txt")
	if err != nil {
		t.Fatalf("resolve prompt path: %v", err)
	}
	hashesPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "hashes.json")
	if err != nil {
		t.Fatalf("resolve hashes path: %v", err)
	}

	agentsBytes := []byte("agent instructions")
	promptBytes := []byte("discussion prompt")
	if err := storage.WriteText(agentsPath, string(agentsBytes)); err != nil {
		t.Fatalf("write agents artifact: %v", err)
	}
	if err := storage.WriteText(promptPath, string(promptBytes)); err != nil {
		t.Fatalf("write prompt artifact: %v", err)
	}
	if err := storage.WriteJSON(hashesPath, prepareHashLedger{
		DispatchInputSHA256: panelhash.SHA256Hex([]byte("dispatch input")),
		AgentsSHA256:        panelhash.SHA256Hex(agentsBytes),
		PromptSHA256:        panelhash.SHA256Hex(promptBytes),
		BundleSHA256:        panelhash.SHA256Hex([]byte("bundle")),
	}); err != nil {
		t.Fatalf("write hashes artifact: %v", err)
	}

	markerPath := filepath.Join(isolatedBaseDir, "run-test", personaID, "marker.txt")
	writeTestFile(t, markerPath, []byte("do not delete"))

	_, err = SealWorkspaces(SealWorkspacesRequest{
		RepoRoot:        repoRoot,
		RunRoot:         runRoot,
		SkillSourcePath: skillRelativePath,
		IsolatedBaseDir: isolatedBaseDir,
	})
	if err == nil {
		t.Fatalf("expected SealWorkspaces to reject isolated base under overridden parent CODEX_HOME")
	}
	if !strings.Contains(err.Error(), "restricted tree") {
		t.Fatalf("unexpected SealWorkspaces error: %v", err)
	}
	if !strings.Contains(err.Error(), overrideCodexHomeDir) {
		t.Fatalf("SealWorkspaces error %q does not mention overridden parent CODEX_HOME %q", err.Error(), overrideCodexHomeDir)
	}
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("marker under overridden parent CODEX_HOME was touched: %v", err)
	}
}

func TestSealWorkspacesWritesOutgoingInputSchemaV2(t *testing.T) {
	workspace := sealWorkspaceForTest(t)

	outgoingBytes, err := os.ReadFile(workspace.OutgoingInputPath)
	if err != nil {
		t.Fatalf("read outgoing_input.json: %v", err)
	}

	var record outgoingInputRecord
	if err := json.Unmarshal(outgoingBytes, &record); err != nil {
		t.Fatalf("decode outgoing_input.json: %v", err)
	}
	if record.SchemaVersion != "answer_outgoing_input_v2" {
		t.Fatalf("outgoing_input schema_version = %q, want %q", record.SchemaVersion, "answer_outgoing_input_v2")
	}
	if strings.TrimSpace(record.CodexRuntimeSHA256) == "" {
		t.Fatalf("expected outgoing_input v2 to include codex_runtime_sha256")
	}
}

func TestVerifySealedInputsAcceptsLegacyOutgoingInputV1WithoutCodexRuntimeHash(t *testing.T) {
	workspace := sealWorkspaceForTest(t)

	outgoingBytes, err := os.ReadFile(workspace.OutgoingInputPath)
	if err != nil {
		t.Fatalf("read outgoing_input.json before legacy rewrite: %v", err)
	}

	var current outgoingInputRecord
	if err := json.Unmarshal(outgoingBytes, &current); err != nil {
		t.Fatalf("decode outgoing_input.json before legacy rewrite: %v", err)
	}

	legacy := map[string]string{
		"schema_version":              "answer_outgoing_input_v1",
		"stage":                       current.Stage,
		"persona_id":                  current.PersonaID,
		"execution_cwd":               current.ExecutionCWD,
		"execution_home_dir":          current.ExecutionHomeDir,
		"execution_codex_home_dir":    current.ExecutionCodexHomeDir,
		"agent_instructions_path":     current.AgentInstructionsPath,
		"agent_instructions_sha256":   current.AgentInstructionsSHA256,
		"prompt_path":                 current.PromptPath,
		"prompt_sha256":               current.PromptSHA256,
		"skill_path":                  current.SkillPath,
		"skill_sha256":                current.SkillSHA256,
		"source_dispatch_input_sha256": current.SourceDispatchInputSHA256,
		"combined_input_sha256":       current.CombinedInputSHA256,
	}
	if err := storage.WriteJSON(workspace.OutgoingInputPath, legacy); err != nil {
		t.Fatalf("rewrite outgoing_input.json as legacy v1: %v", err)
	}

	verified, err := verifySealedInputs(workspace)
	if err != nil {
		t.Fatalf("verifySealedInputs returned error for legacy outgoing_input v1: %v", err)
	}
	if verified.OutgoingRecord == nil {
		t.Fatalf("expected verified outgoing record for legacy outgoing_input v1")
	}
	if verified.OutgoingRecord.SchemaVersion != "answer_outgoing_input_v1" {
		t.Fatalf("verified outgoing_input schema_version = %q, want %q", verified.OutgoingRecord.SchemaVersion, "answer_outgoing_input_v1")
	}
	if verified.OutgoingRecord.CodexRuntimeSHA256 != "" {
		t.Fatalf("legacy outgoing_input v1 codex_runtime_sha256 = %q, want empty", verified.OutgoingRecord.CodexRuntimeSHA256)
	}
}

func TestRunnerUsesSealedExecutionEnvForLaunch(t *testing.T) {
	workspace := sealWorkspaceForTest(t)

	t.Setenv("HOME", filepath.Join(t.TempDir(), "parent-home"))
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "parent-codex-home"))

	var capturedLaunchContext AppServerLaunchContext
	runner := Runner{
		StartAppServer: func(ctx context.Context, launchContext AppServerLaunchContext) (StartedAppServer, error) {
			capturedLaunchContext = launchContext
			return struct{}{}, nil
		},
		RunSingleTurn: func(ctx context.Context, client StartedAppServer, options RunSingleTurnOptions) (SingleTurnRunResult, error) {
			return SingleTurnRunResult{
				TerminalOutcome: statusOutcomeCompleted,
				CompletedItem: &CompletedAgentItem{
					ItemType: authoritativeCompletedItemType,
					ItemID:   "completed-item-1",
					Text:     "sealed answer",
				},
			}, nil
		},
	}

	result, err := runner.Execute(context.Background(), ExecuteWorkerRequest{
		Workspace: workspace,
		ExtraArgs: []string{"--sandbox", "workspace-write"},
	})
	if err != nil {
		t.Fatalf("Runner.Execute returned error: %v", err)
	}
	if !result.Launched {
		t.Fatalf("expected worker launch to occur")
	}

	wantLaunchContext := workspace.ExecutionEnv.LaunchContext([]string{"--sandbox", "workspace-write"})
	if !reflect.DeepEqual(capturedLaunchContext.ExtraArgs, wantLaunchContext.ExtraArgs) {
		t.Fatalf("launch extra args = %#v, want %#v", capturedLaunchContext.ExtraArgs, wantLaunchContext.ExtraArgs)
	}
	if capturedLaunchContext.CurrentWorkingDirectory != wantLaunchContext.CurrentWorkingDirectory {
		t.Fatalf("launch cwd = %q, want %q", capturedLaunchContext.CurrentWorkingDirectory, wantLaunchContext.CurrentWorkingDirectory)
	}
	if capturedLaunchContext.HomeDir != wantLaunchContext.HomeDir {
		t.Fatalf("launch home dir = %q, want %q", capturedLaunchContext.HomeDir, wantLaunchContext.HomeDir)
	}
	if capturedLaunchContext.CodexHomeDir != wantLaunchContext.CodexHomeDir {
		t.Fatalf("launch codex home dir = %q, want %q", capturedLaunchContext.CodexHomeDir, wantLaunchContext.CodexHomeDir)
	}
	if !reflect.DeepEqual(capturedLaunchContext.Environment, wantLaunchContext.Environment) {
		t.Fatalf("launch environment = %#v, want %#v", capturedLaunchContext.Environment, wantLaunchContext.Environment)
	}
	if len(capturedLaunchContext.Environment) != 0 {
		t.Fatalf("launch environment should be empty for sealed execution env, got %#v", capturedLaunchContext.Environment)
	}
}

func TestRunnerRejectsPostSealMutationInIsolatedCodexHome(t *testing.T) {
	workspace := sealWorkspaceForTestWithParentCodex(t, func(_ string, parentCodexHomeDir string) {
		writeTestFile(t, filepath.Join(parentCodexHomeDir, "config.toml"), []byte("model = \"gpt-5\"\n"))
	})

	writeTestFile(t, filepath.Join(workspace.ExecutionEnv.CodexHomeDir, "config.toml"), []byte("model = \"tampered\"\n"))

	startCalled := false
	runCalled := false
	runner := Runner{
		StartAppServer: func(ctx context.Context, launchContext AppServerLaunchContext) (StartedAppServer, error) {
			startCalled = true
			return struct{}{}, nil
		},
		RunSingleTurn: func(ctx context.Context, client StartedAppServer, options RunSingleTurnOptions) (SingleTurnRunResult, error) {
			runCalled = true
			return SingleTurnRunResult{
				TerminalOutcome: statusOutcomeCompleted,
				CompletedItem: &CompletedAgentItem{
					ItemType: authoritativeCompletedItemType,
					ItemID:   "completed-item-1",
					Text:     "sealed answer",
				},
			}, nil
		},
	}

	result, err := runner.Execute(context.Background(), ExecuteWorkerRequest{
		Workspace: workspace,
	})
	if err != nil {
		t.Fatalf("Runner.Execute returned error: %v", err)
	}
	if startCalled {
		t.Fatalf("worker launch occurred despite post-seal isolated Codex mutation")
	}
	if runCalled {
		t.Fatalf("single-turn execution occurred despite post-seal isolated Codex mutation")
	}
	if result.Launched {
		t.Fatalf("worker launch should be rejected before launch")
	}
	if result.Status == nil {
		t.Fatalf("expected status artifact for prelaunch rejection")
	}
	if result.Status.Outcome != statusOutcomeFailed {
		t.Fatalf("status outcome = %q, want %q", result.Status.Outcome, statusOutcomeFailed)
	}
	if result.Attestation == nil {
		t.Fatalf("expected attestation artifact for prelaunch rejection")
	}
	if result.Attestation.CloseOutcomeKind == nil || *result.Attestation.CloseOutcomeKind != rejectedBeforeLaunchOutcome {
		t.Fatalf("close outcome kind = %#v, want %q", result.Attestation.CloseOutcomeKind, rejectedBeforeLaunchOutcome)
	}
}

func TestReadRegularFileAllowsSymlinkedAncestorOutsidePrelaunchScope(t *testing.T) {
	realDir := filepath.Join(t.TempDir(), "real")
	targetPath := filepath.Join(realDir, "artifact.txt")
	writeTestFile(t, targetPath, []byte("artifact payload"))

	symlinkedParent := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(realDir, symlinkedParent); err != nil {
		t.Fatalf("symlink parent dir: %v", err)
	}

	got, err := readRegularFile(filepath.Join(symlinkedParent, "artifact.txt"))
	if err != nil {
		t.Fatalf("readRegularFile returned error for symlinked ancestor outside prelaunch scope: %v", err)
	}
	if string(got) != "artifact payload" {
		t.Fatalf("readRegularFile payload = %q, want %q", string(got), "artifact payload")
	}
}

func TestVerifySealedInputsRejectsMutatedExecutionEnv(t *testing.T) {
	workspace := sealWorkspaceForTest(t)
	workspace.ExecutionEnv.CodexHomeDir = filepath.Join(t.TempDir(), ".codex")

	_, err := verifySealedInputs(workspace)
	if err == nil {
		t.Fatalf("expected verifySealedInputs to reject mutated execution env")
	}
	if !strings.Contains(err.Error(), "sealed execution environment mismatch") {
		t.Fatalf("unexpected verifySealedInputs error: %v", err)
	}
}

func TestVerifySealedInputsRejectsCoordinatedWorkspaceMutation(t *testing.T) {
	workspace := sealWorkspaceForTest(t)
	mutatedRoot := filepath.Join(t.TempDir(), "alternate-isolated-root")
	workspace.IsolatedRoot = mutatedRoot
	workspace.ExecutionEnv = sealedExecutionEnvForRoot(mutatedRoot)
	workspace.WorkspaceAgentsPath = filepath.Join(mutatedRoot, "workspace", "AGENTS.md")
	workspace.PromptPath = filepath.Join(mutatedRoot, "input", "prompt.txt")
	workspace.SkillPath = filepath.Join(mutatedRoot, "skill", "SKILL.md")

	_, err := verifySealedInputs(workspace)
	if err == nil {
		t.Fatalf("expected verifySealedInputs to reject coordinated workspace mutation")
	}
	if !strings.Contains(err.Error(), "sealed execution environment record mismatch") {
		t.Fatalf("unexpected verifySealedInputs error: %v", err)
	}
}

func TestVerifySealedInputsRejectsSymlinkedIsolatedAgentsFile(t *testing.T) {
	workspace := sealWorkspaceForTest(t)

	originalBytes, err := os.ReadFile(workspace.WorkspaceAgentsPath)
	if err != nil {
		t.Fatalf("read isolated AGENTS.md before symlink swap: %v", err)
	}

	replacementPath := filepath.Join(t.TempDir(), "replacement-agents.md")
	writeTestFile(t, replacementPath, originalBytes)

	if err := os.Remove(workspace.WorkspaceAgentsPath); err != nil {
		t.Fatalf("remove isolated AGENTS.md before symlink swap: %v", err)
	}
	if err := os.Symlink(replacementPath, workspace.WorkspaceAgentsPath); err != nil {
		t.Fatalf("symlink isolated AGENTS.md: %v", err)
	}

	_, err = verifySealedInputs(workspace)
	if err == nil {
		t.Fatalf("expected verifySealedInputs to reject symlinked isolated AGENTS.md")
	}
	if !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("unexpected verifySealedInputs error: %v", err)
	}
}

func TestVerifySealedInputsRejectsSymlinkedAncestorOfIsolatedAgentsFile(t *testing.T) {
	workspace := sealWorkspaceForTest(t)

	originalBytes, err := os.ReadFile(workspace.WorkspaceAgentsPath)
	if err != nil {
		t.Fatalf("read isolated AGENTS.md before workspace symlink swap: %v", err)
	}

	replacementWorkspaceDir := filepath.Join(t.TempDir(), "replacement-workspace")
	writeTestFile(t, filepath.Join(replacementWorkspaceDir, "AGENTS.md"), originalBytes)

	originalWorkspaceDir := filepath.Dir(workspace.WorkspaceAgentsPath)
	if err := os.Remove(workspace.WorkspaceAgentsPath); err != nil {
		t.Fatalf("remove isolated AGENTS.md before workspace symlink swap: %v", err)
	}
	if err := os.Remove(originalWorkspaceDir); err != nil {
		t.Fatalf("remove isolated workspace dir before symlink swap: %v", err)
	}
	if err := os.Symlink(replacementWorkspaceDir, originalWorkspaceDir); err != nil {
		t.Fatalf("symlink isolated workspace dir: %v", err)
	}

	_, err = verifySealedInputs(workspace)
	if err == nil {
		t.Fatalf("expected verifySealedInputs to reject symlinked workspace ancestor")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("unexpected verifySealedInputs error: %v", err)
	}
}

func TestVerifySealedInputsRejectsSymlinkedAllowlistedCodexRuntimeFile(t *testing.T) {
	workspace := sealWorkspaceForTestWithParentCodex(t, func(_ string, parentCodexHomeDir string) {
		writeTestFile(t, filepath.Join(parentCodexHomeDir, "config.toml"), []byte("model = \"gpt-5\"\n"))
	})

	runtimeFilePath := filepath.Join(workspace.ExecutionEnv.CodexHomeDir, "config.toml")
	originalBytes, err := os.ReadFile(runtimeFilePath)
	if err != nil {
		t.Fatalf("read isolated config.toml before symlink swap: %v", err)
	}

	replacementPath := filepath.Join(t.TempDir(), "replacement-config.toml")
	writeTestFile(t, replacementPath, originalBytes)

	if err := os.Remove(runtimeFilePath); err != nil {
		t.Fatalf("remove isolated config.toml before symlink swap: %v", err)
	}
	if err := os.Symlink(replacementPath, runtimeFilePath); err != nil {
		t.Fatalf("symlink isolated config.toml: %v", err)
	}

	_, err = verifySealedInputs(workspace)
	if err == nil {
		t.Fatalf("expected verifySealedInputs to reject symlinked isolated Codex runtime file")
	}
	if !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("unexpected verifySealedInputs error: %v", err)
	}
}

func TestVerifySealedInputsRejectsSymlinkedIsolatedCodexHomeDir(t *testing.T) {
	workspace := sealWorkspaceForTestWithParentCodex(t, func(_ string, parentCodexHomeDir string) {
		writeTestFile(t, filepath.Join(parentCodexHomeDir, "config.toml"), []byte("model = \"gpt-5\"\n"))
	})

	runtimeFilePath := filepath.Join(workspace.ExecutionEnv.CodexHomeDir, "config.toml")
	originalBytes, err := os.ReadFile(runtimeFilePath)
	if err != nil {
		t.Fatalf("read isolated config.toml before CODEX_HOME symlink swap: %v", err)
	}

	replacementCodexHomeDir := filepath.Join(t.TempDir(), "replacement-codex-home")
	writeTestFile(t, filepath.Join(replacementCodexHomeDir, "config.toml"), originalBytes)

	if err := os.Remove(runtimeFilePath); err != nil {
		t.Fatalf("remove isolated config.toml before CODEX_HOME symlink swap: %v", err)
	}
	if err := os.Remove(workspace.ExecutionEnv.CodexHomeDir); err != nil {
		t.Fatalf("remove isolated CODEX_HOME dir before symlink swap: %v", err)
	}
	if err := os.Symlink(replacementCodexHomeDir, workspace.ExecutionEnv.CodexHomeDir); err != nil {
		t.Fatalf("symlink isolated CODEX_HOME dir: %v", err)
	}

	_, err = verifySealedInputs(workspace)
	if err == nil {
		t.Fatalf("expected verifySealedInputs to reject symlinked isolated CODEX_HOME dir")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("unexpected verifySealedInputs error: %v", err)
	}
}

func TestVerifySealedInputsRejectsUnexpectedIsolatedCodexHomeEntry(t *testing.T) {
	workspace := sealWorkspaceForTest(t)

	writeTestFile(t, filepath.Join(workspace.ExecutionEnv.CodexHomeDir, "sessions", "session.json"), []byte(`{"id":"session-1"}`))

	_, err := verifySealedInputs(workspace)
	if err == nil {
		t.Fatalf("expected verifySealedInputs to reject unexpected isolated CODEX_HOME entry")
	}
	if !strings.Contains(err.Error(), "unexpected entry") {
		t.Fatalf("unexpected verifySealedInputs error: %v", err)
	}
}

func sealWorkspaceForTest(t *testing.T) SealedPersonaWorkspace {
	t.Helper()
	return sealWorkspaceForTestWithParentCodex(t, nil)
}

func sealWorkspaceForTestWithParentCodex(t *testing.T, setupParentCodex func(parentHomeDir string, parentCodexHomeDir string)) SealedPersonaWorkspace {
	t.Helper()

	testRoot := t.TempDir()
	parentHomeDir := filepath.Join(testRoot, "parent-home")
	repoRoot := filepath.Join(testRoot, "repo")
	runRoot := filepath.Join(testRoot, "runs", "run-test")
	isolatedBaseDir := filepath.Join(testRoot, "isolated")
	personaID := "persona-test"
	skillRelativePath := filepath.Join("runtime", "skills", "answer", "SKILL.md")
	skillSourcePath := filepath.Join(repoRoot, skillRelativePath)

	if err := os.MkdirAll(parentHomeDir, 0o755); err != nil {
		t.Fatalf("create parent home: %v", err)
	}
	t.Setenv("HOME", parentHomeDir)
	t.Setenv("CODEX_HOME", "")
	if setupParentCodex != nil {
		setupParentCodex(parentHomeDir, filepath.Join(parentHomeDir, ".codex"))
	}

	if err := os.MkdirAll(filepath.Dir(skillSourcePath), 0o755); err != nil {
		t.Fatalf("create skill directory: %v", err)
	}
	skillBytes := []byte("test skill instructions")
	if err := os.WriteFile(skillSourcePath, skillBytes, 0o644); err != nil {
		t.Fatalf("write skill source: %v", err)
	}

	gatePath, err := storage.ResolvePath(storage.PrepareRoot(runRoot), prepareGateStatusArtifactName)
	if err != nil {
		t.Fatalf("resolve gate path: %v", err)
	}
	if err := storage.WriteJSON(gatePath, prepareGateStatus{
		CanProceedToStage2: true,
		PersonaIDs:         []string{personaID},
	}); err != nil {
		t.Fatalf("write gate artifact: %v", err)
	}

	agentsPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "agents.md")
	if err != nil {
		t.Fatalf("resolve agents path: %v", err)
	}
	promptPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "prompt.txt")
	if err != nil {
		t.Fatalf("resolve prompt path: %v", err)
	}
	hashesPath, err := storage.PreparePersonaArtifactPath(runRoot, personaID, "hashes.json")
	if err != nil {
		t.Fatalf("resolve hashes path: %v", err)
	}

	agentsBytes := []byte("agent instructions")
	promptBytes := []byte("discussion prompt")
	if err := storage.WriteText(agentsPath, string(agentsBytes)); err != nil {
		t.Fatalf("write agents artifact: %v", err)
	}
	if err := storage.WriteText(promptPath, string(promptBytes)); err != nil {
		t.Fatalf("write prompt artifact: %v", err)
	}
	if err := storage.WriteJSON(hashesPath, prepareHashLedger{
		DispatchInputSHA256: panelhash.SHA256Hex([]byte("dispatch input")),
		AgentsSHA256:        panelhash.SHA256Hex(agentsBytes),
		PromptSHA256:        panelhash.SHA256Hex(promptBytes),
		BundleSHA256:        panelhash.SHA256Hex([]byte("bundle")),
	}); err != nil {
		t.Fatalf("write hashes artifact: %v", err)
	}

	sealed, err := SealWorkspaces(SealWorkspacesRequest{
		RepoRoot:        repoRoot,
		RunRoot:         runRoot,
		SkillSourcePath: skillRelativePath,
		IsolatedBaseDir: isolatedBaseDir,
	})
	if err != nil {
		t.Fatalf("SealWorkspaces returned error: %v", err)
	}
	if sealed.Blocked {
		t.Fatalf("SealWorkspaces unexpectedly blocked stage 2")
	}
	if len(sealed.Workspaces) != 1 {
		t.Fatalf("sealed workspace count = %d, want 1", len(sealed.Workspaces))
	}

	return sealed.Workspaces[0]
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create test file directory for %q: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write test file %q: %v", path, err)
	}
}
