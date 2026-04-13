package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"view_panel/internal/content"
)

type stubPanelExecutor struct {
	requests []content.PanelRunRequest
}

func (s *stubPanelExecutor) RunPanel(ctx context.Context, req content.PanelRunRequest) (content.PanelRunResult, error) {
	s.requests = append(s.requests, req)
	return content.PanelRunResult{
		QuestionID: req.Question.QuestionID,
		MaterialID: req.Question.MaterialID,
		RunID:      "run-123",
		Variant:    "raw",
		Cards: []content.DistinctnessCard{
			{
				PersonaID:     "analyst",
				SourceOutcome: "certified",
				Headline:      "Use anchors",
				Body:          "Use incident postmortems and rollback drills before increasing release cadence.",
			},
		},
	}, nil
}

func TestRunWritesDistinctnessReports(t *testing.T) {
	bundleRoot := writeFixtureBundle(t)
	questionsPath := writeQuestionSet(t)
	outDir := t.TempDir()

	stub := &stubPanelExecutor{}
	original := newPanelRunExecutor
	newPanelRunExecutor = func(workDir, panelBinary string) content.PanelRunExecutor {
		return stub
	}
	defer func() {
		newPanelRunExecutor = original
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runWithIO([]string{
		"-bundle", bundleRoot,
		"-questions", questionsPath,
		"-out", outDir,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runWithIO returned %d, want 0; stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if len(stub.requests) != 1 {
		t.Fatalf("executor requests = %d, want 1", len(stub.requests))
	}
	if stub.requests[0].Question.MaterialID != "technology" {
		t.Fatalf("material_id = %q, want technology", stub.requests[0].Question.MaterialID)
	}
	if stub.requests[0].Model != "" {
		t.Fatalf("default model = %q, want empty", stub.requests[0].Model)
	}

	requireFile(t, filepath.Join(outDir, "distinctness-report.json"))
	requireFile(t, filepath.Join(outDir, "distinctness-report.md"))

	var report content.DistinctnessReport
	readJSONFile(t, filepath.Join(outDir, "distinctness-report.json"), &report)
	if report.BundleID != "default" {
		t.Fatalf("bundle_id = %q, want default", report.BundleID)
	}
	if report.RunCount != 1 {
		t.Fatalf("run_count = %d, want 1", report.RunCount)
	}
	if !strings.Contains(stdout.String(), "distinctness-report.json") {
		t.Fatalf("stdout missing report path: %q", stdout.String())
	}
}

func TestRunRequiresExplicitOutFlag(t *testing.T) {
	bundleRoot := writeFixtureBundle(t)
	questionsPath := writeQuestionSet(t)

	testCases := []struct {
		name     string
		extraArgs []string
	}{
		{
			name: "missing",
		},
		{
			name:      "explicit_empty",
			extraArgs: []string{"-out="},
		},
		{
			name:      "whitespace_only",
			extraArgs: []string{"-out", " \t \n "},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubPanelExecutor{}
			original := newPanelRunExecutor
			newPanelRunExecutor = func(workDir, panelBinary string) content.PanelRunExecutor {
				return stub
			}
			defer func() {
				newPanelRunExecutor = original
			}()

			args := []string{
				"-bundle", bundleRoot,
				"-questions", questionsPath,
			}
			args = append(args, tc.extraArgs...)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := runWithIO(args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("runWithIO returned %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), "-out is required for direct CLI use") {
				t.Fatalf("stderr = %q, want explicit -out validation error", stderr.String())
			}
			if !strings.Contains(stderr.String(), "Usage:") {
				t.Fatalf("stderr = %q, want usage output", stderr.String())
			}
			if len(stub.requests) != 0 {
				t.Fatalf("executor requests = %d, want 0", len(stub.requests))
			}
		})
	}
}

func TestRunForwardsExecutionFlagsIntoPanelExecution(t *testing.T) {
	bundleRoot := writeFixtureBundle(t)
	questionsPath := writeQuestionSet(t)
	outDir := t.TempDir()
	panelBinary := filepath.Join(t.TempDir(), "worldview-panel")

	stub := &stubPanelExecutor{}
	var capturedPanelBinary string
	original := newPanelRunExecutor
	newPanelRunExecutor = func(workDir, binary string) content.PanelRunExecutor {
		capturedPanelBinary = binary
		return stub
	}
	defer func() {
		newPanelRunExecutor = original
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runWithIO([]string{
		"-bundle", bundleRoot,
		"-questions", questionsPath,
		"-out", outDir,
		"-concurrency", "3",
		"-review-enabled=false",
		"-worker-timeout-ms", "2500",
		"-max-attempts-per-persona", "4",
		"-forbidden-tool-name", "shell.exec",
		"-forbidden-tool-name", "web.search",
		"-panel-binary", panelBinary,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runWithIO returned %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if capturedPanelBinary != panelBinary {
		t.Fatalf("panelBinary = %q, want %q", capturedPanelBinary, panelBinary)
	}
	if len(stub.requests) != 1 {
		t.Fatalf("executor requests = %d, want 1", len(stub.requests))
	}

	request := stub.requests[0]
	if request.Concurrency != 3 {
		t.Fatalf("concurrency = %d, want 3", request.Concurrency)
	}
	if request.ReviewEnabled {
		t.Fatalf("review_enabled = %t, want false", request.ReviewEnabled)
	}
	if request.WorkerTimeoutMS != 2500 {
		t.Fatalf("worker_timeout_ms = %d, want 2500", request.WorkerTimeoutMS)
	}
	if request.MaxAttemptsPerPersona != 4 {
		t.Fatalf("max_attempts_per_persona = %d, want 4", request.MaxAttemptsPerPersona)
	}
	if got := strings.Join(request.ForbiddenToolNames, ","); got != "shell.exec,web.search" {
		t.Fatalf("forbidden_tool_names = %q, want %q", got, "shell.exec,web.search")
	}
}

func TestCommandForRequestForwardsExecutionFlagsIntoArgs(t *testing.T) {
	panelBinary := filepath.Join(t.TempDir(), "worldview-panel")
	outRoot := filepath.Join(t.TempDir(), "panel-run")

	executor := &commandPanelExecutor{
		panelBinary: panelBinary,
	}
	req := content.PanelRunRequest{
		MaterialPath:   filepath.Join("content", "build", "default", "runtime", "materials", "technology.json"),
		PersonaSetPath: filepath.Join("content", "build", "default", "runtime", "persona_sets", "default.json"),
		Question: content.DistinctnessQuestion{
			QuestionID: "question-1",
			MaterialID: "technology",
			Question:   "How should teams change release cadence?",
		},
		Concurrency:           3,
		ReviewEnabled:         false,
		WorkerTimeoutMS:       2500,
		MaxAttemptsPerPersona: 4,
		ForbiddenToolNames: []string{
			"shell.exec",
			"web.search",
		},
	}

	commandName, commandArgs := executor.commandForRequest(req, outRoot)
	if commandName != panelBinary {
		t.Fatalf("commandName = %q, want %q", commandName, panelBinary)
	}

	wantArgs := []string{
		"-question", "How should teams change release cadence?",
		"-materials", filepath.Join("content", "build", "default", "runtime", "materials", "technology.json"),
		"-persona-set", filepath.Join("content", "build", "default", "runtime", "persona_sets", "default.json"),
		"-outdir", outRoot,
		"-concurrency", "3",
		"-review-enabled=false",
		"-worker-timeout-ms", "2500",
		"-max-attempts-per-persona", "4",
		"-forbidden-tool-name", "shell.exec",
		"-forbidden-tool-name", "web.search",
	}
	if !reflect.DeepEqual(commandArgs, wantArgs) {
		t.Fatalf("commandArgs = %#v, want %#v", commandArgs, wantArgs)
	}
}

func TestRunHelpPrintsUsage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runWithIO([]string{"-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runWithIO returned %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	output := stdout.String()
	for _, snippet := range []string{
		"Usage:",
		"Evaluate compiled content distinctness",
		"-bundle",
		"-model",
		"-questions",
		"-out",
		"unsupported",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("help output missing %q: %q", snippet, output)
		}
	}
	if strings.Contains(output, "passed through to worldview-panel runs") {
		t.Fatalf("help output still presents -model as supported passthrough: %q", output)
	}
}

func TestRunInvalidArgumentsPrintsUsageToStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runWithIO([]string{"unexpected-positional"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runWithIO returned %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}

	output := stderr.String()
	for _, snippet := range []string{
		"unexpected positional arguments",
		"Usage:",
		"-bundle",
		"-questions",
		"-out",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("stderr missing %q: %q", snippet, output)
		}
	}
}

func TestRunRejectsUnsupportedAnchorDomainBeforeExecution(t *testing.T) {
	bundleRoot := writeFixtureBundle(t)
	questionsPath := writeQuestionSetForDomain(t, "finance")
	outDir := t.TempDir()

	stub := &stubPanelExecutor{}
	original := newPanelRunExecutor
	newPanelRunExecutor = func(workDir, panelBinary string) content.PanelRunExecutor {
		return stub
	}
	defer func() {
		newPanelRunExecutor = original
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runWithIO([]string{
		"-bundle", bundleRoot,
		"-questions", questionsPath,
		"-out", outDir,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runWithIO returned %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "anchor_domain") || !strings.Contains(stderr.String(), "domain_finance") {
		t.Fatalf("stderr = %q, want anchor_domain validation error", stderr.String())
	}
	if len(stub.requests) != 0 {
		t.Fatalf("executor requests = %d, want 0", len(stub.requests))
	}
}

func TestRunRejectsUnsupportedModelFlagBeforeEvaluation(t *testing.T) {
	bundleRoot := writeFixtureBundle(t)
	questionsPath := writeQuestionSet(t)
	outDir := t.TempDir()

	stub := &stubPanelExecutor{}
	original := newPanelRunExecutor
	newPanelRunExecutor = func(workDir, panelBinary string) content.PanelRunExecutor {
		return stub
	}
	defer func() {
		newPanelRunExecutor = original
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runWithIO([]string{
		"-bundle", bundleRoot,
		"-questions", questionsPath,
		"-out", outDir,
		"-model", "gpt-5.4",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runWithIO returned %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "-model") || !strings.Contains(stderr.String(), "unsupported") {
		t.Fatalf("stderr = %q, want unsupported -model error", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stderr = %q, want usage output", stderr.String())
	}
	if len(stub.requests) != 0 {
		t.Fatalf("executor requests = %d, want 0", len(stub.requests))
	}
}

func TestCommandForRequestOmitsModelFlagWhenModelEmpty(t *testing.T) {
	executor := &commandPanelExecutor{
		panelBinary: "worldview-panel",
	}

	_, args := executor.commandForRequest(content.PanelRunRequest{
		Question: content.DistinctnessQuestion{
			QuestionID: "technology_fragility",
			MaterialID: "technology",
			Question:   "What should a team do when release speed is outrunning reliability?",
		},
		MaterialPath:          "/tmp/material.json",
		PersonaSetPath:        "/tmp/personas.json",
		OutputRoot:            "/tmp/out",
		Concurrency:           1,
		ReviewEnabled:         true,
		MaxAttemptsPerPersona: 1,
	}, "/tmp/out/panel_runs/technology_fragility")

	for _, arg := range args {
		if arg == "-model" {
			t.Fatalf("command args unexpectedly included -model: %v", args)
		}
	}
}

func TestCommandPanelExecutorRejectsUnsupportedModelBeforeExecution(t *testing.T) {
	executor := &commandPanelExecutor{
		workDir:     t.TempDir(),
		panelBinary: filepath.Join(t.TempDir(), "missing-worldview-panel"),
	}
	outputRoot := t.TempDir()
	questionID := "technology_fragility"

	_, err := executor.RunPanel(context.Background(), content.PanelRunRequest{
		Question: content.DistinctnessQuestion{
			QuestionID: questionID,
			MaterialID: "technology",
			Question:   "What should a team do when release speed is outrunning reliability?",
		},
		MaterialPath:          "/tmp/material.json",
		PersonaSetPath:        "/tmp/personas.json",
		OutputRoot:            outputRoot,
		Model:                 "gpt-5.4",
		Concurrency:           1,
		ReviewEnabled:         true,
		MaxAttemptsPerPersona: 1,
	})
	if err == nil {
		t.Fatalf("expected unsupported model error")
	}
	if !strings.Contains(err.Error(), "-model") || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error = %q, want unsupported -model message", err)
	}
	questionRoot := filepath.Join(outputRoot, "panel_runs", questionID)
	if _, statErr := os.Stat(questionRoot); !os.IsNotExist(statErr) {
		t.Fatalf("question root %q exists or returned unexpected stat error: %v", questionRoot, statErr)
	}
}

func writeFixtureBundle(t *testing.T) string {
	t.Helper()

	sourceRoot := filepath.Clean(filepath.Join("..", "..", "testdata", "content", "builder_fixture", "src"))
	catalog, err := content.LoadCatalog(sourceRoot)
	if err != nil {
		t.Fatalf("LoadCatalog returned error: %v", err)
	}
	bundle, err := content.CompileBundle(catalog, content.CompileOptions{BundleID: "default"})
	if err != nil {
		t.Fatalf("CompileBundle returned error: %v", err)
	}

	outDir := t.TempDir()
	if err := content.WriteBundle(outDir, bundle); err != nil {
		t.Fatalf("WriteBundle returned error: %v", err)
	}
	return outDir
}

func writeQuestionSet(t *testing.T) string {
	return writeQuestionSetForDomain(t, "technology")
}

func writeQuestionSetForDomain(t *testing.T, anchorDomain string) string {
	t.Helper()

	outPath := filepath.Join(t.TempDir(), "questions.json")
	payload := map[string]any{
		"schema_version": "distinctness_questions_v1",
		"questions": []map[string]string{
			{
				"question_id":   "technology_fragility",
				"material_id":   "technology",
				"anchor_domain": anchorDomain,
				"question":      "What should a team do when release speed is outrunning reliability?",
			},
		},
	}
	bytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent returned error: %v", err)
	}
	bytes = append(bytes, '\n')
	if err := os.WriteFile(outPath, bytes, 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", outPath, err)
	}
	return outPath
}

func requireFile(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("%q is a directory, want file", path)
	}
}

func readJSONFile(t *testing.T, path string, target any) {
	t.Helper()

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		t.Fatalf("Unmarshal(%q): %v", path, err)
	}
}
