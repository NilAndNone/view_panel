package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type prepareGateStatus struct {
	Status             string   `json:"status"`
	CanProceedToStage2 bool     `json:"can_proceed_to_stage2"`
	PersonaIDs         []string `json:"persona_ids"`
}

type answerBatch struct {
	Counts answerBatchCounts `json:"counts"`
}

type answerBatchCounts struct {
	TotalPersonas int `json:"total_personas"`
	Certified     int `json:"certified"`
	Failed        int `json:"failed"`
	Rejected      int `json:"rejected"`
}

type renderInput struct {
	Available bool `json:"available"`
}

type renderStatus struct {
	Raw       renderBranch `json:"raw"`
	Certified renderBranch `json:"certified"`
}

type renderBranch struct {
	State string `json:"state"`
}

func main() {
	var runRoot string
	flag.StringVar(&runRoot, "run-root", "", "absolute run root path")
	flag.Parse()

	if runRoot == "" {
		fail("missing --run-root")
	}

	runRoot = filepath.Clean(runRoot)
	mustBeDir(runRoot)

	prepare := mustLoadJSON[prepareGateStatus](filepath.Join(runRoot, "01_prepare", "prepare_gate_status_v1.json"))
	if prepare.Status != "pass" {
		fail("prepare gate status must be pass, got %q", prepare.Status)
	}
	if !prepare.CanProceedToStage2 {
		fail("prepare gate must allow stage2 in smoke run")
	}
	if len(prepare.PersonaIDs) != 2 {
		fail("prepare persona_ids must contain exactly 2 entries, got %d", len(prepare.PersonaIDs))
	}

	answer := mustLoadJSON[answerBatch](filepath.Join(runRoot, "02_answer", "answer_batch.json"))
	if answer.Counts.TotalPersonas != 2 {
		fail("answer_batch counts.total_personas must be 2, got %d", answer.Counts.TotalPersonas)
	}
	if answer.Counts.Certified+answer.Counts.Failed+answer.Counts.Rejected != 2 {
		fail("answer_batch counts do not add up to 2")
	}

	rawInputPath := filepath.Join(runRoot, "03_render", "raw_render_input.json")
	certifiedInputPath := filepath.Join(runRoot, "03_render", "certified_render_input.json")
	renderStatusPath := filepath.Join(runRoot, "03_render", "status.json")

	rawInput := mustLoadJSON[renderInput](rawInputPath)
	certifiedInput := mustLoadJSON[renderInput](certifiedInputPath)
	status := mustLoadJSON[renderStatus](renderStatusPath)

	validateBranchState("raw", rawInput.Available, status.Raw.State)
	validateBranchState("certified", certifiedInput.Available, status.Certified.State)

	fmt.Printf("smoke check passed: %s\n", runRoot)
}

func validateBranchState(name string, available bool, state string) {
	switch state {
	case "rendered", "skipped", "failed":
	default:
		fail("%s branch state must be rendered/skipped/failed, got %q", name, state)
	}

	if name == "raw" {
		if available && state == "skipped" {
			fail("raw branch is available but status.raw.state is skipped")
		}
		if !available && state == "rendered" {
			fail("raw branch is unavailable but status.raw.state is rendered")
		}
		return
	}

	if available && state != "rendered" {
		fail("certified branch is available but status.certified.state is %q", state)
	}
	if !available && state == "rendered" {
		fail("certified branch is unavailable but status.certified.state is rendered")
	}
}

func mustBeDir(path string) {
	info, err := os.Stat(path)
	if err != nil {
		fail("stat %s: %v", path, err)
	}
	if !info.IsDir() {
		fail("expected directory: %s", path)
	}
}

func mustLoadJSON[T any](path string) T {
	data, err := os.ReadFile(path)
	if err != nil {
		fail("read %s: %v", path, err)
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		fail("decode %s: %v", path, err)
	}
	return value
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
