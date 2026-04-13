package prepare

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"view_panel/internal/appserver"
)

type appServerReviewer struct {
	repoRoot  string
	extraArgs []string
	homeDir   string
}

// NewAppServerReviewer returns the default P06 advisory reviewer backed by the
// existing Codex app-server single-turn runtime flow.
func NewAppServerReviewer(repoRoot string, extraArgs []string, homeDir string) AdvisoryReviewer {
	return &appServerReviewer{
		repoRoot:  repoRoot,
		extraArgs: append([]string(nil), extraArgs...),
		homeDir:   homeDir,
	}
}

func (r *appServerReviewer) ReviewPrepare(ctx context.Context, request AdvisoryReviewRequest) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("reviewer is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	skillPath := strings.TrimSpace(request.SkillPath)
	if skillPath == "" {
		skillPath = prepareReviewSkillPath
	}
	resolvedSkillPath, err := r.resolveRepoPath(skillPath)
	if err != nil {
		return nil, fmt.Errorf("resolve review skill: %w", err)
	}

	request.SkillPath = skillPath
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal review request: %w", err)
	}

	environment := map[string]string{}
	if strings.TrimSpace(r.homeDir) != "" {
		environment["HOME"] = r.homeDir
	}

	response, err := appserver.RunSkillTurn(ctx, appserver.SkillTurnRequest{
		Operation: "review",
		SkillPath: resolvedSkillPath,
		Payload:   requestBytes,
		LaunchContext: appserver.AppServerLaunchContext{
			ExtraArgs:               append([]string(nil), r.extraArgs...),
			Environment:             environment,
			CurrentWorkingDirectory: r.repoRoot,
			HomeDir:                 r.homeDir,
		},
	})
	if err != nil {
		return nil, err
	}

	return []byte(response), nil
}

func (r *appServerReviewer) resolveRepoPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	if strings.TrimSpace(r.repoRoot) == "" {
		return "", fmt.Errorf("repository root must not be empty for relative path %q", path)
	}
	return filepath.Join(r.repoRoot, filepath.Clean(path)), nil
}
