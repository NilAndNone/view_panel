package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"view_panel/internal/appserver"
	"view_panel/internal/config"
	"view_panel/internal/materials"
	"view_panel/internal/orchestrator"
	"view_panel/internal/persona"
	"view_panel/internal/schema"
	"view_panel/internal/stage/answer"
	"view_panel/internal/stage/prepare"
	"view_panel/internal/stage/render"
	"view_panel/internal/storage"
)

const (
	modulePath                = "view_panel"
	answerSkillPath           = "runtime/skills/wv-answer-stage/SKILL.md"
	renderSkillPath           = "runtime/skills/wv-render-stage/SKILL.md"
	renderMinSuccessRate      = 0.8
	isolatedWorkspaceBaseName = "view_panel-isolated"
)

type stringSliceFlag []string

func (f *stringSliceFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringSliceFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

type cliErrorEnvelope struct {
	Status      string              `json:"status"`
	Error       string              `json:"error,omitempty"`
	Diagnostics []schema.Diagnostic `json:"diagnostics,omitempty"`
}

type statusError struct {
	status string
	err    error
}

func (e *statusError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *statusError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

type renderAppServerAdapter struct {
	repoRoot string
	extraArgs []string
	homeDir  string
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cfg, diagnostics, err := parseStartup(args)
	if err != nil {
		emitJSON(os.Stderr, cliErrorEnvelope{
			Status: "invalid_cli_usage",
			Error:  err.Error(),
		})
		return 2
	}

	if len(diagnostics) > 0 {
		emitJSON(os.Stderr, cliErrorEnvelope{
			Status:      "invalid_startup_config",
			Diagnostics: diagnostics,
		})
		return 1
	}

	if err := handoffValidatedStartup(context.Background(), cfg); err != nil {
		emitJSON(os.Stderr, cliErrorEnvelope{
			Status: failureStatus(err),
			Error:  failureMessage(err),
		})
		return 1
	}

	return 0
}

func parseStartup(args []string) (config.StartupConfig, []schema.Diagnostic, error) {
	flagSet := flag.NewFlagSet("worldview-panel", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var materials stringSliceFlag
	question := flagSet.String("question", "", "")
	personaSet := flagSet.String("persona-set", "", "")
	outdir := flagSet.String("outdir", "", "")
	concurrency := flagSet.Int("concurrency", 0, "")
	model := flagSet.String("model", "", "")
	flagSet.Var(&materials, "materials", "")

	if err := flagSet.Parse(args); err != nil {
		return config.StartupConfig{}, nil, err
	}
	if extras := flagSet.Args(); len(extras) > 0 {
		return config.StartupConfig{}, nil, fmt.Errorf("unexpected positional arguments: %v", extras)
	}

	provided := make(map[string]bool)
	flagSet.Visit(func(current *flag.Flag) {
		provided[current.Name] = true
	})

	raw := config.RawStartupInput{
		Question:            *question,
		Materials:           append([]string(nil), materials...),
		PersonaSet:          *personaSet,
		OutDir:              *outdir,
		Concurrency:         *concurrency,
		Model:               *model,
		QuestionProvided:    provided["question"],
		MaterialsProvided:   provided["materials"],
		PersonaSetProvided:  provided["persona-set"],
		OutDirProvided:      provided["outdir"],
		ConcurrencyProvided: provided["concurrency"],
		ModelProvided:       provided["model"],
	}

	cfg := config.NormalizeStartupConfig(raw)
	diagnostics := schema.ValidateStartupConfig(raw, cfg)
	return cfg, diagnostics, nil
}

func handoffValidatedStartup(ctx context.Context, cfg config.StartupConfig) error {
	runID := newRunID()
	runRoot := storage.RunRoot(cfg.OutDir, runID)

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return withStatus("startup_failed", err)
	}
	if err := storage.EnsureRunLayout(runRoot); err != nil {
		return withStatus("startup_failed", fmt.Errorf("ensure run layout: %w", err))
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return withStatus("startup_failed", fmt.Errorf("resolve current user home: %w", err))
	}
	isolatedBaseDir, err := chooseIsolatedBaseDir(repoRoot, runRoot, homeDir)
	if err != nil {
		return withStatus("startup_failed", err)
	}

	modelArgs := modelExtraArgs(cfg.Model)
	renderAdapter := renderAppServerAdapter{
		repoRoot: repoRoot,
		extraArgs: modelArgs,
		homeDir:  homeDir,
	}

	_, err = orchestrator.Run(ctx, orchestrator.RunRequest{
		RunID: runID,
		Prepare: orchestrator.PrepareStageInvocation{
			Run: func(ctx context.Context, req orchestrator.PrepareStageRequest) (orchestrator.PrepareStageResult, error) {
				result, err := runPrepareStage(ctx, cfg, runRoot)
				if err != nil {
					return orchestrator.PrepareStageResult{}, err
				}
				return orchestrator.PrepareStageResult{
					RunID:   req.RunID,
					Payload: result,
				}, nil
			},
			Request: orchestrator.PrepareStageRequest{RunID: runID},
		},
		Answer: orchestrator.AnswerStageInvocation{
			Run: func(ctx context.Context, req orchestrator.AnswerStageRequest) (orchestrator.AnswerStageResult, error) {
				result, err := runAnswerStage(ctx, cfg, repoRoot, runRoot, isolatedBaseDir, req.RunID, modelArgs)
				if err != nil {
					return orchestrator.AnswerStageResult{}, err
				}
				return orchestrator.AnswerStageResult{
					RunID:          req.RunID,
					AnswerBatchPath: result.ArtifactPath,
					Payload:        result,
				}, nil
			},
			Request: orchestrator.AnswerStageRequest{RunID: runID},
		},
		Render: orchestrator.RenderStageInvocation{
			Aggregate: func(ctx context.Context, req orchestrator.RenderAggregateRequest) (orchestrator.RenderAggregateResult, error) {
				result, err := runRenderAggregateStage(ctx, req.RunID, runRoot, req.AnswerBatchPath)
				if err != nil {
					return orchestrator.RenderAggregateResult{}, err
				}
				return orchestrator.RenderAggregateResult{
					RunID:                   req.RunID,
					RawRenderInputPath:      result.RawRenderInputPath,
					CertifiedRenderInputPath: result.CertifiedRenderInputPath,
					RawAvailable:            result.RawAvailable,
					CertifiedAvailable:      result.CertifiedAvailable,
					Payload:                 result,
				}, nil
			},
			AggregateRequest: orchestrator.RenderAggregateRequest{RunID: runID},
			Run: func(ctx context.Context, req orchestrator.RenderStageRequest) (orchestrator.RenderStageResult, error) {
				result, err := runRenderStage(ctx, runID, runRoot, req.RawRenderInputPath, req.CertifiedRenderInputPath, renderAdapter)
				if err != nil {
					return orchestrator.RenderStageResult{}, err
				}
				return orchestrator.RenderStageResult{
					RunID:     req.RunID,
					StatusPath: result.StatusPath,
					Payload:   result,
				}, nil
			},
			Request: orchestrator.RenderStageRequest{RunID: runID},
		},
	})
	if err != nil {
		return withStatus("runtime_failed", err)
	}

	return nil
}

func emitJSON(writer io.Writer, value any) {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
}

func withStatus(status string, err error) error {
	if err == nil {
		return nil
	}
	return &statusError{status: status, err: err}
}

func failureStatus(err error) string {
	var wrapped *statusError
	if errors.As(err, &wrapped) && strings.TrimSpace(wrapped.status) != "" {
		return wrapped.status
	}
	return "runtime_failed"
}

func failureMessage(err error) string {
	var wrapped *statusError
	if errors.As(err, &wrapped) && wrapped.err != nil {
		return wrapped.err.Error()
	}
	return err.Error()
}

func newRunID() string {
	return fmt.Sprintf("%s-p%d", time.Now().UTC().Format("20060102T150405.000000000Z"), os.Getpid())
}

func runPrepareStage(ctx context.Context, cfg config.StartupConfig, runRoot string) (prepare.PrepareResult, error) {
	if err := ctx.Err(); err != nil {
		return prepare.PrepareResult{}, err
	}

	materialFields, err := loadPrepareMaterials(ctx, cfg.Materials)
	if err != nil {
		return prepare.PrepareResult{}, err
	}
	personas, err := loadPreparePersonas(ctx, cfg.PersonaSet)
	if err != nil {
		return prepare.PrepareResult{}, err
	}

	return prepare.Run(ctx, prepare.PrepareRequest{
		RunRoot: runRoot,
		Materials: prepare.DispatchInputV1{
			RoleplayPrompt:            materialFields.RoleplayPrompt,
			DiscussionQuestion:        materialFields.DiscussionQuestion,
			SupplementaryMaterials:    materialFields.SupplementaryMaterials,
			OutputContract:            materialFields.OutputContract,
			AssumptionsAndConstraints: materialFields.AssumptionsAndConstraints,
		},
		Personas: personas,
	})
}

func loadPrepareMaterials(ctx context.Context, paths []string) (materials.CanonicalFields, error) {
	if err := ctx.Err(); err != nil {
		return materials.CanonicalFields{}, err
	}

	selectedPath, err := selectMaterialsPath(paths)
	if err != nil {
		return materials.CanonicalFields{}, err
	}

	loader := materials.Loader{}
	fields, err := loader.LoadFile(selectedPath)
	if err != nil {
		return materials.CanonicalFields{}, err
	}
	return fields, nil
}

func selectMaterialsPath(paths []string) (string, error) {
	candidates := make([]string, 0, len(paths))
	for _, input := range paths {
		if strings.TrimSpace(input) == "" {
			return "", fmt.Errorf("materials path must not be empty")
		}

		info, err := os.Stat(input)
		if err != nil {
			return "", fmt.Errorf("inspect materials path %q: %w", input, err)
		}

		if info.IsDir() {
			files, err := collectMaterialFiles(input)
			if err != nil {
				return "", fmt.Errorf("expand materials directory %q: %w", input, err)
			}
			candidates = append(candidates, files...)
			continue
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("materials path %q must resolve to a regular file or directory", input)
		}

		absolute, err := filepath.Abs(input)
		if err != nil {
			return "", fmt.Errorf("canonicalize materials path %q: %w", input, err)
		}
		candidates = append(candidates, absolute)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no materials file candidates found")
	}

	// The current materials loader consumes exactly one canonical document.
	// Until a higher-level merger exists, preserve CLI order for explicit paths,
	// expand directories in lexical order, and take the first resulting file.
	return candidates[0], nil
}

func collectMaterialFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		files = append(files, absolute)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func loadPreparePersonas(ctx context.Context, personaSetPath string) ([]prepare.PersonaRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	library := persona.Library{}
	resolved, err := library.Resolve(personaSetPath)
	if err != nil {
		return nil, err
	}

	personas := make([]prepare.PersonaRecord, 0, len(resolved))
	for _, item := range resolved {
		personas = append(personas, prepare.PersonaRecord{
			PersonaID:  item.PersonaID,
			SourcePath: item.SourcePath,
			Fields:     item.Fields,
		})
	}
	return personas, nil
}

func runAnswerStage(ctx context.Context, cfg config.StartupConfig, repoRoot, runRoot, isolatedBaseDir, runID string, extraArgs []string) (answer.AnswerBatchResult, error) {
	sealed, err := answer.SealWorkspaces(answer.SealWorkspacesRequest{
		RepoRoot:        repoRoot,
		RunRoot:         runRoot,
		SkillSourcePath: answerSkillPath,
		IsolatedBaseDir: isolatedBaseDir,
	})
	if err != nil {
		return answer.AnswerBatchResult{}, err
	}

	runner := answer.Runner{
		StartAppServer: startAnswerAppServer,
		RunSingleTurn:  runAnswerSingleTurn,
	}
	batch := answer.BatchRunner{Runner: runner}
	workspaces := sealed.Workspaces
	if sealed.Blocked {
		workspaces = nil
	}
	return batch.Execute(ctx, answer.AnswerBatchRequest{
		RunID:                 runID,
		AnswerRoot:            storage.AnswerRoot(runRoot),
		Workspaces:            workspaces,
		MaxConcurrency:        cfg.Concurrency,
		WorkerTimeoutMS:       0,
		MaxAttemptsPerPersona: 1,
		ExtraArgs:             append([]string(nil), extraArgs...),
	})
}

func startAnswerAppServer(ctx context.Context, launchContext answer.AppServerLaunchContext) (answer.StartedAppServer, error) {
	return appserver.StartAppServer(ctx, appserver.AppServerLaunchContext{
		ExtraArgs:               append([]string(nil), launchContext.ExtraArgs...),
		Environment:             cloneStringMap(launchContext.Environment),
		CurrentWorkingDirectory: launchContext.CurrentWorkingDirectory,
		HomeDir:                 launchContext.HomeDir,
	})
}

func runAnswerSingleTurn(ctx context.Context, client answer.StartedAppServer, options answer.RunSingleTurnOptions) (answer.SingleTurnRunResult, error) {
	startedClient, ok := client.(*appserver.Client)
	if !ok {
		return answer.SingleTurnRunResult{}, fmt.Errorf("unexpected appserver client type %T", client)
	}

	result, err := appserver.RunSingleTurn(ctx, startedClient, appserver.RunSingleTurnOptions{
		Input: options.Input,
	})
	return translateAnswerSingleTurnResult(result), err
}

func translateAnswerSingleTurnResult(result appserver.RunSingleTurnResult) answer.SingleTurnRunResult {
	transcript := make([]answer.DiagnosticStreamItem, 0, len(result.Transcript))
	for _, item := range result.Transcript {
		transcript = append(transcript, translateAnswerDiagnosticItem(item))
	}

	return answer.SingleTurnRunResult{
		TerminalOutcome: string(result.TerminalOutcome),
		LastReachedPhase: string(result.LastReachedPhase),
		CloseOutcomeKind: string(result.CloseOutcome.Kind),
		ThreadID:         result.ThreadID,
		TurnID:           result.TurnID,
		CompletedItem:    translateAnswerCompletedItem(result.CompletedItem),
		Transcript:       transcript,
	}
}

func translateAnswerCompletedItem(item *appserver.AgentMessageCompletedEventParams) *answer.CompletedAgentItem {
	if item == nil {
		return nil
	}
	return &answer.CompletedAgentItem{
		ItemType: appserver.EventItemCompletedAgentMessage,
		ItemID:   item.Item.ID,
		Text:     item.Item.PlainText(),
		Payload:  item,
	}
}

func translateAnswerDiagnosticItem(item appserver.ClientMessage) answer.DiagnosticStreamItem {
	diagnostic := answer.DiagnosticStreamItem{
		ItemType: item.Event.Method,
		Payload:  item.Event,
	}
	if item.AgentMessageCompletedEvent != nil {
		diagnostic.ItemType = appserver.EventItemCompletedAgentMessage
		diagnostic.ItemID = item.AgentMessageCompletedEvent.Item.ID
		diagnostic.Text = item.AgentMessageCompletedEvent.Item.PlainText()
		diagnostic.Payload = item.AgentMessageCompletedEvent
	}
	return diagnostic
}

func runRenderAggregateStage(ctx context.Context, runID, runRoot, answerBatchPath string) (render.AggregateResult, error) {
	return render.AggregateStage3Inputs(ctx, render.AggregateRequest{
		RunID:           runID,
		AnswerBatchPath: answerBatchPath,
		RenderRoot:      storage.RenderRoot(runRoot),
		MinSuccessRate:  renderMinSuccessRate,
	})
}

func runRenderStage(ctx context.Context, runID, runRoot, rawRenderInputPath, certifiedRenderInputPath string, adapter renderAppServerAdapter) (render.RunRenderStageResult, error) {
	return render.RunRenderStage(ctx, render.RunRenderStageRequest{
		RunID:                   runID,
		RenderRoot:              storage.RenderRoot(runRoot),
		RawRenderInputPath:      rawRenderInputPath,
		CertifiedRenderInputPath: certifiedRenderInputPath,
		SkillPath:               renderSkillPath,
		AppServer:               adapter,
	})
}

func (a renderAppServerAdapter) ExecuteRenderTurn(ctx context.Context, req render.AppServerRenderRequest) (render.AppServerRenderResponse, error) {
	skillPath, err := a.resolveRepoPath(req.SkillPath)
	if err != nil {
		return render.AppServerRenderResponse{}, fmt.Errorf("resolve render skill: %w", err)
	}
	skillBytes, err := os.ReadFile(skillPath)
	if err != nil {
		return render.AppServerRenderResponse{}, fmt.Errorf("read render skill: %w", err)
	}
	renderInputBytes, err := os.ReadFile(req.RenderInputPath)
	if err != nil {
		return render.AppServerRenderResponse{}, fmt.Errorf("read render input: %w", err)
	}

	client, err := appserver.StartAppServer(ctx, appserver.AppServerLaunchContext{
		ExtraArgs:               append([]string(nil), a.extraArgs...),
		Environment:             map[string]string{"HOME": a.homeDir},
		CurrentWorkingDirectory: req.WorkingDirectory,
		HomeDir:                 a.homeDir,
	})
	if err != nil {
		return render.AppServerRenderResponse{}, fmt.Errorf("start render app-server: %w", err)
	}

	result, err := appserver.RunSingleTurn(ctx, client, appserver.RunSingleTurnOptions{
		Input: string(skillBytes) + "\n\n" + string(renderInputBytes),
	})
	if err != nil {
		return render.AppServerRenderResponse{}, fmt.Errorf("run render turn: %w", err)
	}

	completed := result.CompletedItem
	if completed == nil {
		for _, item := range result.Transcript {
			if item.AgentMessageCompletedEvent != nil {
				completed = item.AgentMessageCompletedEvent
				break
			}
		}
	}
	if completed == nil {
		return render.AppServerRenderResponse{}, fmt.Errorf("render turn completed without an authoritative agent message")
	}

	panelJSON := strings.TrimSpace(completed.Item.PlainText())
	if panelJSON == "" {
		return render.AppServerRenderResponse{}, fmt.Errorf("render turn returned an empty authoritative agent message")
	}

	return render.AppServerRenderResponse{
		PanelJSON: json.RawMessage(panelJSON),
	}, nil
}

func (a renderAppServerAdapter) resolveRepoPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(a.repoRoot, filepath.Clean(path)), nil
}

func modelExtraArgs(model string) []string {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil
	}
	return []string{"--model", model}
}

func locateRepoRoot() (string, error) {
	start, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}

	current, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("canonicalize working directory %q: %w", start, err)
	}

	for {
		goModPath := filepath.Join(current, "go.mod")
		data, err := os.ReadFile(goModPath)
		if err == nil {
			if parseModulePath(data) == modulePath {
				return current, nil
			}
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("read %q: %w", goModPath, err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", fmt.Errorf("could not locate repo root containing module %q from current working directory", modulePath)
}

func parseModulePath(goMod []byte) string {
	for _, line := range strings.Split(string(goMod), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, "module "))
	}
	return ""
}

func chooseIsolatedBaseDir(repoRoot, runRoot, homeDir string) (string, error) {
	candidate, err := filepath.Abs(filepath.Join(os.TempDir(), isolatedWorkspaceBaseName))
	if err != nil {
		return "", fmt.Errorf("canonicalize isolated base dir: %w", err)
	}
	if pathWithinRoot(repoRoot, candidate) || pathWithinRoot(runRoot, candidate) || pathWithinRoot(homeDir, candidate) {
		return "", fmt.Errorf("isolated base dir %q must be outside repo root, run root, and home", candidate)
	}
	return candidate, nil
}

func pathWithinRoot(root, candidate string) bool {
	root = strings.TrimSpace(root)
	candidate = strings.TrimSpace(candidate)
	if root == "" || candidate == "" {
		return false
	}

	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	if relative == "." {
		return true
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
