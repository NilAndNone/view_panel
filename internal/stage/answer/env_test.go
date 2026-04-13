package answer

import (
	"context"
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

func sealWorkspaceForTest(t *testing.T) SealedPersonaWorkspace {
	t.Helper()

	testRoot := t.TempDir()
	repoRoot := filepath.Join(testRoot, "repo")
	runRoot := filepath.Join(testRoot, "runs", "run-test")
	isolatedBaseDir := filepath.Join(testRoot, "isolated")
	personaID := "persona-test"
	skillRelativePath := filepath.Join("runtime", "skills", "answer", "SKILL.md")
	skillSourcePath := filepath.Join(repoRoot, skillRelativePath)

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
