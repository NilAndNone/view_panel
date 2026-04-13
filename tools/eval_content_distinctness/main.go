package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"view_panel/internal/content"
)

type stringSliceFlag []string

func (f *stringSliceFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringSliceFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

type evalCLIConfig struct {
	bundleRoot           string
	questionsPath        string
	outRoot              string
	panelBinary          string
	model                string
	concurrency          int
	reviewEnabled        bool
	workerTimeoutMS      int
	maxAttemptsPerPersona int
	forbiddenToolNames   []string
}

var newPanelRunExecutor = func(workDir, panelBinary string) content.PanelRunExecutor {
	return &commandPanelExecutor{
		workDir:     workDir,
		panelBinary: strings.TrimSpace(panelBinary),
	}
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	return runWithIO(args, os.Stdout, os.Stderr)
}

func runWithIO(args []string, stdout, stderr io.Writer) int {
	flagSet := flag.NewFlagSet("eval_content_distinctness", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var forbiddenToolNames stringSliceFlag
	bundleRoot := flagSet.String("bundle", "content/build/default", "compiled content bundle root containing bundle-manifest.json and runtime/")
	questionsPath := flagSet.String("questions", "content/src/evals/distinctness/questions.json", "distinctness question set JSON")
	outRoot := flagSet.String("out", "", "output root for distinctness-report.json and distinctness-report.md; required for direct CLI use")
	panelBinary := flagSet.String("panel-binary", "", "optional worldview-panel binary path; default runs `go run ./cmd/worldview-panel` from the current working tree")
	model := flagSet.String("model", "", "deprecated and unsupported by the current worldview-panel runtime contract; leave empty")
	concurrency := flagSet.Int("concurrency", 1, "panel worker concurrency for each evaluation run")
	reviewEnabled := flagSet.Bool("review-enabled", true, "whether evaluation runs should keep prepare review enabled")
	workerTimeoutMS := flagSet.Int("worker-timeout-ms", 0, "per-worker timeout in milliseconds passed through to worldview-panel")
	maxAttemptsPerPersona := flagSet.Int("max-attempts-per-persona", 1, "maximum attempts per persona passed through to worldview-panel")
	flagSet.Var(&forbiddenToolNames, "forbidden-tool-name", "forbidden tool name forwarded to worldview-panel; repeat or comma-separate upstream if needed")

	if err := flagSet.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stdout, usageText(flagSet))
			return 0
		}
		fmt.Fprintf(stderr, "%v\n\n%s", err, usageText(flagSet))
		return 2
	}
	if extras := flagSet.Args(); len(extras) > 0 {
		fmt.Fprintf(stderr, "unexpected positional arguments: %v\n\n%s", extras, usageText(flagSet))
		return 2
	}
	if modelValue := strings.TrimSpace(*model); modelValue != "" {
		fmt.Fprintf(stderr, "%s\n\n%s", unsupportedModelError(), usageText(flagSet))
		return 2
	}
	if strings.TrimSpace(*outRoot) == "" {
		fmt.Fprintf(stderr, "-out is required for direct CLI use; pass an explicit output root\n\n%s", usageText(flagSet))
		return 2
	}

	config := evalCLIConfig{
		bundleRoot:           *bundleRoot,
		questionsPath:        *questionsPath,
		outRoot:              *outRoot,
		panelBinary:          *panelBinary,
		model:                strings.TrimSpace(*model),
		concurrency:          *concurrency,
		reviewEnabled:        *reviewEnabled,
		workerTimeoutMS:      *workerTimeoutMS,
		maxAttemptsPerPersona: *maxAttemptsPerPersona,
		forbiddenToolNames:   append([]string(nil), forbiddenToolNames...),
	}

	if err := runEvalContentDistinctness(context.Background(), config, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func usageText(flagSet *flag.FlagSet) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Usage: %s [flags]\n\n", flagSet.Name())
	fmt.Fprintf(&buf, "Evaluate compiled content distinctness by running the existing worldview-panel CLI against a question set and writing report artifacts.\n\n")
	fmt.Fprintf(&buf, "Flags:\n")
	flagSet.SetOutput(&buf)
	flagSet.PrintDefaults()
	return buf.String()
}

func unsupportedModelError() error {
	return errors.New("-model is unsupported by the current worldview-panel runtime contract; leave it unset")
}

func runEvalContentDistinctness(ctx context.Context, cfg evalCLIConfig, stdout io.Writer) error {
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	report, err := content.EvaluateDistinctness(ctx, cfg.bundleRoot, cfg.questionsPath, cfg.outRoot, newPanelRunExecutor(workDir, cfg.panelBinary), content.DistinctnessExecOptions{
		Model:                 cfg.model,
		Concurrency:           cfg.concurrency,
		ReviewEnabled:         cfg.reviewEnabled,
		WorkerTimeoutMS:       cfg.workerTimeoutMS,
		MaxAttemptsPerPersona: cfg.maxAttemptsPerPersona,
		ForbiddenToolNames:    append([]string(nil), cfg.forbiddenToolNames...),
	})
	if err != nil {
		return fmt.Errorf("evaluate content distinctness: %w", err)
	}
	if err := content.WriteDistinctnessReport(cfg.outRoot, report); err != nil {
		return fmt.Errorf("write content distinctness report: %w", err)
	}

	reportJSONPath := filepath.Join(report.OutputRoot, "distinctness-report.json")
	reportMarkdownPath := filepath.Join(report.OutputRoot, "distinctness-report.md")
	_, _ = fmt.Fprintf(stdout, "wrote %s and %s\n", reportJSONPath, reportMarkdownPath)
	return nil
}

type commandPanelExecutor struct {
	workDir     string
	panelBinary string
}

func (executor *commandPanelExecutor) RunPanel(ctx context.Context, req content.PanelRunRequest) (content.PanelRunResult, error) {
	if strings.TrimSpace(req.Model) != "" {
		return content.PanelRunResult{}, unsupportedModelError()
	}

	questionRoot := filepath.Join(req.OutputRoot, "panel_runs", req.Question.QuestionID)
	if err := os.RemoveAll(questionRoot); err != nil {
		return content.PanelRunResult{}, fmt.Errorf("clear evaluation output for question %q: %w", req.Question.QuestionID, err)
	}
	if err := os.MkdirAll(questionRoot, 0o755); err != nil {
		return content.PanelRunResult{}, fmt.Errorf("create evaluation output for question %q: %w", req.Question.QuestionID, err)
	}

	commandName, commandArgs := executor.commandForRequest(req, questionRoot)
	cmd := exec.CommandContext(ctx, commandName, commandArgs...)
	cmd.Dir = executor.workDir

	var commandStdout bytes.Buffer
	var commandStderr bytes.Buffer
	cmd.Stdout = &commandStdout
	cmd.Stderr = &commandStderr

	if err := cmd.Run(); err != nil {
		output := strings.TrimSpace(commandStderr.String())
		if output == "" {
			output = strings.TrimSpace(commandStdout.String())
		}
		if output != "" {
			return content.PanelRunResult{}, fmt.Errorf("%w: %s", err, output)
		}
		return content.PanelRunResult{}, err
	}

	runRoot, err := discoverRunRoot(filepath.Join(questionRoot, "runs"))
	if err != nil {
		return content.PanelRunResult{}, err
	}
	variant, cards, err := loadCardsFromRunRoot(runRoot)
	if err != nil {
		return content.PanelRunResult{}, err
	}

	return content.PanelRunResult{
		QuestionID: req.Question.QuestionID,
		MaterialID: req.Question.MaterialID,
		RunID:      filepath.Base(runRoot),
		Variant:    variant,
		Cards:      cards,
	}, nil
}

func (executor *commandPanelExecutor) commandForRequest(req content.PanelRunRequest, outRoot string) (string, []string) {
	args := []string{
		"-question", req.Question.Question,
		"-materials", req.MaterialPath,
		"-persona-set", req.PersonaSetPath,
		"-outdir", outRoot,
		"-concurrency", strconv.Itoa(req.Concurrency),
		"-review-enabled=" + strconv.FormatBool(req.ReviewEnabled),
		"-worker-timeout-ms", strconv.Itoa(req.WorkerTimeoutMS),
		"-max-attempts-per-persona", strconv.Itoa(req.MaxAttemptsPerPersona),
	}
	for _, name := range req.ForbiddenToolNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		args = append(args, "-forbidden-tool-name", name)
	}

	if executor.panelBinary != "" {
		return executor.panelBinary, args
	}
	return "go", append([]string{"run", "./cmd/worldview-panel"}, args...)
}

func discoverRunRoot(runsDir string) (string, error) {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return "", fmt.Errorf("read evaluation run root %q: %w", runsDir, err)
	}

	runIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		runIDs = append(runIDs, entry.Name())
	}
	if len(runIDs) != 1 {
		return "", fmt.Errorf("expected exactly one run directory under %q, found %d", runsDir, len(runIDs))
	}

	return filepath.Join(runsDir, runIDs[0]), nil
}

func loadCardsFromRunRoot(runRoot string) (string, []content.DistinctnessCard, error) {
	candidates := []struct {
		variant string
		path    string
	}{
		{variant: "certified", path: filepath.Join(runRoot, "03_render", "certified_cards.json")},
		{variant: "raw", path: filepath.Join(runRoot, "03_render", "raw_cards.json")},
	}

	for _, candidate := range candidates {
		payload, err := os.ReadFile(candidate.path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", nil, fmt.Errorf("read render cards %q: %w", candidate.path, err)
		}

		var cards []content.DistinctnessCard
		if err := json.Unmarshal(payload, &cards); err != nil {
			return "", nil, fmt.Errorf("decode render cards %q: %w", candidate.path, err)
		}
		return candidate.variant, cards, nil
	}

	return "", nil, fmt.Errorf("run root %q does not contain certified_cards.json or raw_cards.json", runRoot)
}
