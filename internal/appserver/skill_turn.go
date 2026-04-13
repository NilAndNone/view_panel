package appserver

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type SkillTurnRequest struct {
	Operation     string
	SkillPath     string
	Payload       []byte
	LaunchContext AppServerLaunchContext
}

func RunSkillTurn(ctx context.Context, req SkillTurnRequest) (string, error) {
	operation := strings.TrimSpace(req.Operation)
	if operation == "" {
		operation = "skill"
	}

	skillPath := strings.TrimSpace(req.SkillPath)
	if skillPath == "" {
		return "", fmt.Errorf("%s skill path is required", operation)
	}

	skillBytes, err := os.ReadFile(skillPath)
	if err != nil {
		return "", fmt.Errorf("read %s skill: %w", operation, err)
	}

	client, err := StartAppServer(ctx, req.LaunchContext)
	if err != nil {
		return "", fmt.Errorf("start %s app-server: %w", operation, err)
	}

	result, err := RunSingleTurn(ctx, client, RunSingleTurnOptions{
		Input: string(skillBytes) + "\n\n" + string(req.Payload),
	})
	if err != nil {
		return "", fmt.Errorf("run %s turn: %w", operation, err)
	}

	return authoritativeTurnText(result, operation)
}

func authoritativeTurnText(result RunSingleTurnResult, operation string) (string, error) {
	if strings.TrimSpace(operation) == "" {
		operation = "skill"
	}
	if result.CompletedItem == nil {
		return "", fmt.Errorf("%s turn completed without an authoritative agent message", operation)
	}

	text := strings.TrimSpace(result.CompletedItem.Item.PlainText())
	if text == "" {
		return "", fmt.Errorf("%s turn returned an empty authoritative agent message", operation)
	}

	return text, nil
}
