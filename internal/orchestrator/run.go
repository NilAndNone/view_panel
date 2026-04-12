package orchestrator

import (
	"context"
	"fmt"
	"strings"
)

type RunRequest struct {
	RunID   string
	Prepare PrepareStageInvocation
	Answer  AnswerStageInvocation
	Render  RenderStageInvocation
}

type RunResult struct {
	Prepare        PrepareStageResult
	Answer         AnswerStageResult
	RenderAggregate RenderAggregateResult
	Render         RenderStageResult
}

type PrepareStageInvocation struct {
	Run     func(context.Context, PrepareStageRequest) (PrepareStageResult, error)
	Request PrepareStageRequest
}

type PrepareStageRequest struct {
	RunID   string
	Payload any
}

type PrepareStageResult struct {
	RunID   string
	Payload any
}

type AnswerStageInvocation struct {
	Run     func(context.Context, AnswerStageRequest) (AnswerStageResult, error)
	Request AnswerStageRequest
}

type AnswerStageRequest struct {
	RunID   string
	Prepare PrepareStageResult
	Payload any
}

type AnswerStageResult struct {
	RunID          string
	AnswerBatchPath string
	Payload        any
}

type RenderStageInvocation struct {
	Aggregate        func(context.Context, RenderAggregateRequest) (RenderAggregateResult, error)
	AggregateRequest RenderAggregateRequest
	Run              func(context.Context, RenderStageRequest) (RenderStageResult, error)
	Request          RenderStageRequest
}

type RenderAggregateRequest struct {
	RunID          string
	AnswerBatchPath string
	Payload        any
}

type RenderAggregateResult struct {
	RunID                   string
	RawRenderInputPath      string
	CertifiedRenderInputPath string
	RawAvailable            bool
	CertifiedAvailable      bool
	Payload                 any
}

type RenderStageRequest struct {
	RunID                   string
	RawRenderInputPath      string
	CertifiedRenderInputPath string
	Payload                 any
}

type RenderStageResult struct {
	RunID     string
	StatusPath string
	Payload   any
}

func Run(ctx context.Context, req RunRequest) (RunResult, error) {
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		return RunResult{}, fmt.Errorf("run_id is required")
	}
	if req.Prepare.Run == nil {
		return RunResult{}, fmt.Errorf("prepare stage runner is required")
	}
	if req.Answer.Run == nil {
		return RunResult{}, fmt.Errorf("answer stage runner is required")
	}
	if req.Render.Aggregate == nil {
		return RunResult{}, fmt.Errorf("render aggregate runner is required")
	}
	if req.Render.Run == nil {
		return RunResult{}, fmt.Errorf("render stage runner is required")
	}

	prepareRequest := req.Prepare.Request
	if strings.TrimSpace(prepareRequest.RunID) == "" {
		prepareRequest.RunID = runID
	}
	prepareResult, err := req.Prepare.Run(ctx, prepareRequest)
	if err != nil {
		return RunResult{}, fmt.Errorf("prepare stage failed: %w", err)
	}

	answerRequest := req.Answer.Request
	if strings.TrimSpace(answerRequest.RunID) == "" {
		answerRequest.RunID = runID
	}
	answerRequest.Prepare = prepareResult
	answerResult, err := req.Answer.Run(ctx, answerRequest)
	if err != nil {
		return RunResult{}, fmt.Errorf("answer stage failed: %w", err)
	}
	if strings.TrimSpace(answerResult.AnswerBatchPath) == "" {
		return RunResult{}, fmt.Errorf("answer stage did not return answer_batch.json path")
	}

	renderAggregateRequest := req.Render.AggregateRequest
	if strings.TrimSpace(renderAggregateRequest.RunID) == "" {
		renderAggregateRequest.RunID = runID
	}
	renderAggregateRequest.AnswerBatchPath = answerResult.AnswerBatchPath
	renderAggregateResult, err := req.Render.Aggregate(ctx, renderAggregateRequest)
	if err != nil {
		return RunResult{}, fmt.Errorf("render input aggregation failed: %w", err)
	}
	if strings.TrimSpace(renderAggregateResult.RawRenderInputPath) == "" {
		return RunResult{}, fmt.Errorf("render aggregation did not return raw_render_input.json path")
	}
	if strings.TrimSpace(renderAggregateResult.CertifiedRenderInputPath) == "" {
		return RunResult{}, fmt.Errorf("render aggregation did not return certified_render_input.json path")
	}

	renderRequest := req.Render.Request
	if strings.TrimSpace(renderRequest.RunID) == "" {
		renderRequest.RunID = runID
	}
	renderRequest.RawRenderInputPath = renderAggregateResult.RawRenderInputPath
	renderRequest.CertifiedRenderInputPath = renderAggregateResult.CertifiedRenderInputPath
	renderResult, err := req.Render.Run(ctx, renderRequest)
	if err != nil {
		return RunResult{}, fmt.Errorf("render stage failed: %w", err)
	}

	return RunResult{
		Prepare:        prepareResult,
		Answer:         answerResult,
		RenderAggregate: renderAggregateResult,
		Render:         renderResult,
	}, nil
}
