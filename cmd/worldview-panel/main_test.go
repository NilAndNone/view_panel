package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"view_panel/internal/appserver"
	"view_panel/internal/config"
	"view_panel/internal/orchestrator"
	"view_panel/internal/runtimecontract"
	"view_panel/internal/stage/answer"
	"view_panel/internal/stage/prepare"
	"view_panel/internal/storage"
)

func TestMainStartupBlocksWhenRuntimeContractFails(t *testing.T) {
	blockedResult := runtimecontract.Result{
		Status:         runtimecontract.StatusBlocked,
		BlockingErrors: []string{"observed codex version does not match pinned version"},
	}

	result, err := runStartupForTest(t, blockedResult)
	if err == nil {
		t.Fatalf("expected startup error")
	}
	if got, want := failureStatus(err), "startup_failed"; got != want {
		t.Fatalf("expected failure status %q, got %q", want, got)
	}
	if !strings.Contains(failureMessage(err), blockedResult.BlockingErrors[0]) {
		t.Fatalf("expected failure message to contain %q, got %q", blockedResult.BlockingErrors[0], failureMessage(err))
	}
	if result.OrchestratorCalled {
		t.Fatalf("orchestrator must not run when runtime contract blocks startup")
	}

	artifact := readRuntimeContractStatusForTest(t, result.ArtifactPath)
	if artifact.Status != runtimecontract.StatusBlocked {
		t.Fatalf("expected blocked runtime contract artifact, got %q", artifact.Status)
	}
	if !reflect.DeepEqual(artifact.BlockingErrors, blockedResult.BlockingErrors) {
		t.Fatalf("expected blocking errors %v, got %v", blockedResult.BlockingErrors, artifact.BlockingErrors)
	}
}

func TestMainStartupWritesRuntimeContractStatusArtifact(t *testing.T) {
	expected := runtimecontract.Result{
		Status:             runtimecontract.StatusWarning,
		PinnedCLIVersion:   runtimecontract.DefaultPinnedCLIVersion,
		ObservedCLIVersion: runtimecontract.DefaultPinnedCLIVersion,
		LauncherForm:       runtimecontract.CanonicalLauncherForm,
		ProtocolBundlePath: filepath.Join("/repo", "third_party", "codex-protocol", runtimecontract.DefaultPinnedCLIVersion),
		Warnings: []string{
			runtimecontract.WarningPlaceholderBundle,
			runtimecontract.WarningSequentialJSONTransport,
		},
	}

	result, err := runStartupForTest(t, expected)
	if err != nil {
		t.Fatalf("unexpected startup error: %v", err)
	}
	if !result.OrchestratorCalled {
		t.Fatalf("expected orchestrator to run when runtime contract does not block startup")
	}

	artifact := readRuntimeContractStatusForTest(t, result.ArtifactPath)
	if !reflect.DeepEqual(artifact, expected) {
		t.Fatalf("expected runtime contract artifact %#v, got %#v", expected, artifact)
	}
}

func TestMainStartupReturnsRuntimeFailedWhenOrchestratorFails(t *testing.T) {
	outDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), "home")
	runID := "test-run"
	orchestratorErr := errors.New("orchestrator exploded")

	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home dir: %v", err)
	}

	err := handoffValidatedStartupWithDeps(context.Background(), config.StartupConfig{
		OutDir:      outDir,
		Concurrency: 1,
	}, startupDeps{
		newRunID:        func() string { return runID },
		locateRepoRoot:  func() (string, error) { return "/repo", nil },
		ensureRunLayout: storage.EnsureRunLayout,
		checkRuntimeContract: func(string) runtimecontract.Result {
			return runtimecontract.Result{Status: runtimecontract.StatusOK}
		},
		userHomeDir: func() (string, error) { return homeDir, nil },
		chooseIsolatedBaseDir: func(string, string, string) (string, error) {
			return filepath.Join(t.TempDir(), "isolated"), nil
		},
		runOrchestrator: func(context.Context, orchestrator.RunRequest) (orchestrator.RunResult, error) {
			return orchestrator.RunResult{}, orchestratorErr
		},
	})

	if err == nil {
		t.Fatalf("expected error")
	}
	if got, want := failureStatus(err), "runtime_failed"; got != want {
		t.Fatalf("expected failure status %q, got %q", want, got)
	}
	if !strings.Contains(failureMessage(err), orchestratorErr.Error()) {
		t.Fatalf("expected failure message to contain %q, got %q", orchestratorErr.Error(), failureMessage(err))
	}
}

func TestMainStartupReturnsStartupFailedWhenUserHomeLookupFails(t *testing.T) {
	outDir := t.TempDir()
	runID := "test-run"
	homeErr := errors.New("home lookup failed")

	err := handoffValidatedStartupWithDeps(context.Background(), config.StartupConfig{
		OutDir:      outDir,
		Concurrency: 1,
	}, startupDeps{
		newRunID:        func() string { return runID },
		locateRepoRoot:  func() (string, error) { return "/repo", nil },
		ensureRunLayout: storage.EnsureRunLayout,
		checkRuntimeContract: func(string) runtimecontract.Result {
			return runtimecontract.Result{Status: runtimecontract.StatusOK}
		},
		userHomeDir: func() (string, error) { return "", homeErr },
		chooseIsolatedBaseDir: func(string, string, string) (string, error) {
			return filepath.Join(t.TempDir(), "isolated"), nil
		},
		runOrchestrator: func(context.Context, orchestrator.RunRequest) (orchestrator.RunResult, error) {
			t.Fatalf("orchestrator must not run when startup fails before orchestration")
			return orchestrator.RunResult{}, nil
		},
	})

	if err == nil {
		t.Fatalf("expected error")
	}
	if got, want := failureStatus(err), "startup_failed"; got != want {
		t.Fatalf("expected failure status %q, got %q", want, got)
	}
	if !strings.Contains(failureMessage(err), homeErr.Error()) {
		t.Fatalf("expected failure message to contain %q, got %q", homeErr.Error(), failureMessage(err))
	}
}

func TestTranslateAnswerLaunchContextPreservesCodexHome(t *testing.T) {
	input := answer.AppServerLaunchContext{
		ExtraArgs:               []string{"--sandbox", "workspace-write"},
		Environment:             map[string]string{"FOO": "bar"},
		CurrentWorkingDirectory: "/tmp/isolated/workspace",
		HomeDir:                 "/tmp/isolated/home",
		CodexHomeDir:            "/tmp/isolated/home/.codex",
	}

	got := translateAnswerLaunchContext(input)
	want := appserver.AppServerLaunchContext{
		ExtraArgs:               []string{"--sandbox", "workspace-write"},
		Environment:             map[string]string{"FOO": "bar"},
		CurrentWorkingDirectory: "/tmp/isolated/workspace",
		HomeDir:                 "/tmp/isolated/home",
		CodexHomeDir:            "/tmp/isolated/home/.codex",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("translated launch context = %#v, want %#v", got, want)
	}
}

func TestParseStartupParsesPolicyFlags(t *testing.T) {
	baseDir := t.TempDir()
	materialPath := filepath.Join(baseDir, "materials.json")
	content := `{
  "roleplay_prompt": "roleplay",
  "discussion_question": "question",
  "supplementary_materials": "materials",
  "output_contract": "contract",
  "assumptions_and_constraints": "constraints"
}`
	if err := os.WriteFile(materialPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write materials file: %v", err)
	}

	cfg, diagnostics, err := parseStartup([]string{
		"-question", "What should happen?",
		"-materials", materialPath,
		"-persona-set", "default",
		"-outdir", filepath.Join(baseDir, "out"),
		"-review-enabled=false",
		"-worker-timeout-ms", "2500",
		"-max-attempts-per-persona", "1",
		"-forbidden-tool-name", "Shell,Web",
		"-forbidden-tool-name", "web",
	})
	if err != nil {
		t.Fatalf("parseStartup returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diagnostics)
	}
	if cfg.Model != "" {
		t.Fatalf("expected empty model when -model is omitted, got %q", cfg.Model)
	}
	if cfg.ReviewEnabled {
		t.Fatalf("expected review_enabled=false")
	}
	if cfg.WorkerTimeoutMS != 2500 {
		t.Fatalf("expected worker_timeout_ms 2500, got %d", cfg.WorkerTimeoutMS)
	}
	if cfg.MaxAttemptsPerPersona != 1 {
		t.Fatalf("expected max_attempts_per_persona 1, got %d", cfg.MaxAttemptsPerPersona)
	}
	if got, want := cfg.ForbiddenToolNames, []string{"shell", "web"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected forbidden_tool_names %v, got %v", want, got)
	}
}

func TestRunEmitsInvalidCLIUsageEnvelopeForUnsupportedModel(t *testing.T) {
	baseDir := t.TempDir()
	materialPath := filepath.Join(baseDir, "materials.json")
	content := `{
  "roleplay_prompt": "roleplay",
  "discussion_question": "question",
  "supplementary_materials": "materials",
  "output_contract": "contract",
  "assumptions_and_constraints": "constraints"
}`
	if err := os.WriteFile(materialPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write materials file: %v", err)
	}

	var exitCode int
	stderr := captureStderrForTest(t, func() {
		exitCode = run([]string{
			"-question", "What should happen?",
			"-materials", materialPath,
			"-persona-set", "default",
			"-outdir", filepath.Join(baseDir, "out"),
			"-model", "gpt-5.4",
		})
	})

	if exitCode != 2 {
		t.Fatalf("run returned %d, want 2", exitCode)
	}

	var envelope cliErrorEnvelope
	if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
		t.Fatalf("decode stderr envelope: %v; stderr=%q", err, stderr)
	}
	if envelope.Status != "invalid_cli_usage" {
		t.Fatalf("status = %q, want invalid_cli_usage", envelope.Status)
	}
	if len(envelope.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %v, want none", envelope.Diagnostics)
	}
	if !strings.Contains(envelope.Error, "-model") || !strings.Contains(envelope.Error, "unsupported") {
		t.Fatalf("error = %q, want unsupported -model message", envelope.Error)
	}
}

func TestMainStartupRejectsUnsupportedModelBeforeOrchestration(t *testing.T) {
	outDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), "home")
	runID := "test-run"

	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home dir: %v", err)
	}

	cfg := config.StartupConfig{
		OutDir:        outDir,
		Concurrency:   1,
		Model:         "gpt-5.4",
		ReviewEnabled: true,
	}

	err := handoffValidatedStartupWithDeps(context.Background(), cfg, startupDeps{
		newRunID:        func() string { return runID },
		locateRepoRoot:  func() (string, error) { return "/repo", nil },
		ensureRunLayout: storage.EnsureRunLayout,
		checkRuntimeContract: func(string) runtimecontract.Result {
			return runtimecontract.Result{Status: runtimecontract.StatusOK}
		},
		userHomeDir: func() (string, error) { return homeDir, nil },
		chooseIsolatedBaseDir: func(string, string, string) (string, error) {
			return filepath.Join(t.TempDir(), "isolated"), nil
		},
		newPrepareReviewer: func(string, []string, string) prepare.AdvisoryReviewer {
			t.Fatalf("prepare reviewer must not be constructed when model is unsupported")
			return nil
		},
		runPrepareStage: func(context.Context, config.StartupConfig, string, prepare.AdvisoryReviewer) (prepare.PrepareResult, error) {
			t.Fatalf("prepare stage must not run when model is unsupported")
			return prepare.PrepareResult{}, nil
		},
		runAnswerStage: func(context.Context, config.StartupConfig, string, string, string, string, []string) (answer.AnswerBatchResult, error) {
			t.Fatalf("answer stage must not run when model is unsupported")
			return answer.AnswerBatchResult{ArtifactPath: "answer_batch.json"}, nil
		},
		runOrchestrator: func(context.Context, orchestrator.RunRequest) (orchestrator.RunResult, error) {
			t.Fatalf("orchestrator must not run when model is unsupported")
			return orchestrator.RunResult{}, nil
		},
	})
	if err == nil {
		t.Fatalf("expected startup error for unsupported model")
	}
	message := failureMessage(err)
	if !strings.Contains(message, "-model") || !strings.Contains(message, "unsupported") {
		t.Fatalf("expected unsupported model error, got %q", message)
	}
}

func TestMainStartupDisablesPrepareReviewerAndWiresAnswerPolicy(t *testing.T) {
	outDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), "home")
	runID := "test-run"
	reviewerFactoryCalled := false
	prepareCalled := false
	answerCalled := false

	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home dir: %v", err)
	}

	cfg := config.StartupConfig{
		OutDir:                outDir,
		Concurrency:           1,
		ReviewEnabled:         false,
		WorkerTimeoutMS:       2500,
		MaxAttemptsPerPersona: 1,
		ForbiddenToolNames:    []string{"shell", "web"},
	}

	err := handoffValidatedStartupWithDeps(context.Background(), cfg, startupDeps{
		newRunID:        func() string { return runID },
		locateRepoRoot:  func() (string, error) { return "/repo", nil },
		ensureRunLayout: storage.EnsureRunLayout,
		checkRuntimeContract: func(string) runtimecontract.Result {
			return runtimecontract.Result{Status: runtimecontract.StatusOK}
		},
		userHomeDir: func() (string, error) { return homeDir, nil },
		chooseIsolatedBaseDir: func(string, string, string) (string, error) {
			return filepath.Join(t.TempDir(), "isolated"), nil
		},
		newPrepareReviewer: func(string, []string, string) prepare.AdvisoryReviewer {
			reviewerFactoryCalled = true
			return &stubReviewer{}
		},
		runPrepareStage: func(_ context.Context, gotCfg config.StartupConfig, _ string, reviewer prepare.AdvisoryReviewer) (prepare.PrepareResult, error) {
			prepareCalled = true
			if reviewer != nil {
				t.Fatalf("expected nil reviewer when review is disabled, got %v", reviewer)
			}
			if gotCfg.ReviewEnabled {
				t.Fatalf("expected review_enabled=false")
			}
			return prepare.PrepareResult{}, nil
		},
		runAnswerStage: func(_ context.Context, gotCfg config.StartupConfig, _, _, _, _ string, _ []string) (answer.AnswerBatchResult, error) {
			answerCalled = true
			if gotCfg.WorkerTimeoutMS != cfg.WorkerTimeoutMS {
				t.Fatalf("expected worker_timeout_ms %d, got %d", cfg.WorkerTimeoutMS, gotCfg.WorkerTimeoutMS)
			}
			if gotCfg.MaxAttemptsPerPersona != cfg.MaxAttemptsPerPersona {
				t.Fatalf("expected max_attempts_per_persona %d, got %d", cfg.MaxAttemptsPerPersona, gotCfg.MaxAttemptsPerPersona)
			}
			if !reflect.DeepEqual(gotCfg.ForbiddenToolNames, cfg.ForbiddenToolNames) {
				t.Fatalf("expected forbidden_tool_names %v, got %v", cfg.ForbiddenToolNames, gotCfg.ForbiddenToolNames)
			}
			return answer.AnswerBatchResult{ArtifactPath: "answer_batch.json"}, nil
		},
		runOrchestrator: func(ctx context.Context, req orchestrator.RunRequest) (orchestrator.RunResult, error) {
			if _, err := req.Prepare.Run(ctx, req.Prepare.Request); err != nil {
				return orchestrator.RunResult{}, err
			}
			if _, err := req.Answer.Run(ctx, req.Answer.Request); err != nil {
				return orchestrator.RunResult{}, err
			}
			return orchestrator.RunResult{}, nil
		},
	})
	if err != nil {
		t.Fatalf("handoffValidatedStartupWithDeps returned error: %v", err)
	}
	if reviewerFactoryCalled {
		t.Fatalf("prepare reviewer must not be constructed when review is disabled")
	}
	if !prepareCalled {
		t.Fatalf("expected prepare stage to run")
	}
	if !answerCalled {
		t.Fatalf("expected answer stage to run")
	}
}

func TestMainStartupInjectsPrepareReviewerWhenReviewEnabled(t *testing.T) {
	outDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), "home")
	runID := "test-run"
	sentinelReviewer := &stubReviewer{}
	reviewerFactoryCalled := false
	prepareCalled := false
	answerCalled := false

	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home dir: %v", err)
	}

	cfg := config.StartupConfig{
		OutDir:                outDir,
		Concurrency:           1,
		ReviewEnabled:         true,
		WorkerTimeoutMS:       2500,
		MaxAttemptsPerPersona: 1,
		ForbiddenToolNames:    []string{"shell", "web"},
	}

	err := handoffValidatedStartupWithDeps(context.Background(), cfg, startupDeps{
		newRunID:        func() string { return runID },
		locateRepoRoot:  func() (string, error) { return "/repo", nil },
		ensureRunLayout: storage.EnsureRunLayout,
		checkRuntimeContract: func(string) runtimecontract.Result {
			return runtimecontract.Result{Status: runtimecontract.StatusOK}
		},
		userHomeDir: func() (string, error) { return homeDir, nil },
		chooseIsolatedBaseDir: func(string, string, string) (string, error) {
			return filepath.Join(t.TempDir(), "isolated"), nil
		},
		newPrepareReviewer: func(string, []string, string) prepare.AdvisoryReviewer {
			reviewerFactoryCalled = true
			return sentinelReviewer
		},
		runPrepareStage: func(_ context.Context, gotCfg config.StartupConfig, _ string, reviewer prepare.AdvisoryReviewer) (prepare.PrepareResult, error) {
			prepareCalled = true
			if reviewer != sentinelReviewer {
				t.Fatalf("expected injected reviewer %v, got %v", sentinelReviewer, reviewer)
			}
			if gotCfg.WorkerTimeoutMS != cfg.WorkerTimeoutMS {
				t.Fatalf("expected worker_timeout_ms %d, got %d", cfg.WorkerTimeoutMS, gotCfg.WorkerTimeoutMS)
			}
			return prepare.PrepareResult{}, nil
		},
		runAnswerStage: func(_ context.Context, gotCfg config.StartupConfig, _, _, _, _ string, _ []string) (answer.AnswerBatchResult, error) {
			answerCalled = true
			if gotCfg.WorkerTimeoutMS != cfg.WorkerTimeoutMS {
				t.Fatalf("expected worker_timeout_ms %d, got %d", cfg.WorkerTimeoutMS, gotCfg.WorkerTimeoutMS)
			}
			if gotCfg.MaxAttemptsPerPersona != cfg.MaxAttemptsPerPersona {
				t.Fatalf("expected max_attempts_per_persona %d, got %d", cfg.MaxAttemptsPerPersona, gotCfg.MaxAttemptsPerPersona)
			}
			if !reflect.DeepEqual(gotCfg.ForbiddenToolNames, cfg.ForbiddenToolNames) {
				t.Fatalf("expected forbidden_tool_names %v, got %v", cfg.ForbiddenToolNames, gotCfg.ForbiddenToolNames)
			}
			return answer.AnswerBatchResult{ArtifactPath: "answer_batch.json"}, nil
		},
		runOrchestrator: func(ctx context.Context, req orchestrator.RunRequest) (orchestrator.RunResult, error) {
			if _, err := req.Prepare.Run(ctx, req.Prepare.Request); err != nil {
				return orchestrator.RunResult{}, err
			}
			if _, err := req.Answer.Run(ctx, req.Answer.Request); err != nil {
				return orchestrator.RunResult{}, err
			}
			return orchestrator.RunResult{}, nil
		},
	})
	if err != nil {
		t.Fatalf("handoffValidatedStartupWithDeps returned error: %v", err)
	}
	if !reviewerFactoryCalled {
		t.Fatalf("expected prepare reviewer to be constructed when review is enabled")
	}
	if !prepareCalled {
		t.Fatalf("expected prepare stage to run")
	}
	if !answerCalled {
		t.Fatalf("expected answer stage to run")
	}
}

type stubReviewer struct{}

func (stubReviewer) ReviewPrepare(context.Context, prepare.AdvisoryReviewRequest) ([]byte, error) {
	return nil, nil
}

type startupTestResult struct {
	ArtifactPath       string
	OrchestratorCalled bool
}

func runStartupForTest(t *testing.T, contractResult runtimecontract.Result) (startupTestResult, error) {
	t.Helper()

	outDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), "home")
	runID := "test-run"
	orchestratorCalled := false

	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home dir: %v", err)
	}

	cfg := config.StartupConfig{
		OutDir:      outDir,
		Concurrency: 1,
	}

	err := handoffValidatedStartupWithDeps(context.Background(), cfg, startupDeps{
		newRunID:        func() string { return runID },
		locateRepoRoot:  func() (string, error) { return "/repo", nil },
		ensureRunLayout: storage.EnsureRunLayout,
		checkRuntimeContract: func(string) runtimecontract.Result {
			return contractResult
		},
		userHomeDir: func() (string, error) {
			return homeDir, nil
		},
		chooseIsolatedBaseDir: func(string, string, string) (string, error) {
			return filepath.Join(t.TempDir(), "isolated"), nil
		},
		runOrchestrator: func(context.Context, orchestrator.RunRequest) (orchestrator.RunResult, error) {
			orchestratorCalled = true
			return orchestrator.RunResult{}, nil
		},
	})

	return startupTestResult{
		ArtifactPath:       filepath.Join(outDir, "runs", runID, "audit", "runtime_contract_status.json"),
		OrchestratorCalled: orchestratorCalled,
	}, err
}

func readRuntimeContractStatusForTest(t *testing.T, artifactPath string) runtimecontract.Result {
	t.Helper()

	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read runtime contract status artifact: %v", err)
	}

	var result runtimecontract.Result
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode runtime contract status artifact: %v", err)
	}

	return result
}

func captureStderrForTest(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}

	defer func() {
		os.Stderr = original
		_ = reader.Close()
	}()

	os.Stderr = writer
	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stderr output: %v", err)
	}
	return string(output)
}
