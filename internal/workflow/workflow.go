// Package workflow orchestrates local engineering stages in ordinary Go.
package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	stall "github.com/bboykarlito/libstall"
	"stall-engineer/internal/behaviour"
)

// Config selects the repository, protected base branch, and timeout per stage.
type Config struct {
	Repo         string
	BaseBranch   string
	StageTimeout time.Duration
}

// Outcome retains completed and partial stage results on failure.
type Outcome struct {
	Implementation stall.Result
	Review         stall.Result
}

// Run performs preflight, implementation, and review. It never commits, pushes,
// retries, or rolls back changes. Harnesses must target Config.Repo and enforce
// their own tool policies; the review harness must be read-only.
func Run(ctx context.Context, config Config, task stall.Task, implementation, review stall.Harness) (out Outcome, err error) {
	if strings.TrimSpace(task.Input) == "" {
		return out, errors.New("task is required")
	}
	if config.Repo == "" || config.BaseBranch == "" || config.StageTimeout <= 0 {
		return out, errors.New("repository, base branch, and positive stage timeout are required")
	}
	if implementation == nil || review == nil {
		return out, errors.New("implementation and review harnesses are required")
	}
	repo, err := inspect(ctx, config.Repo, config.BaseBranch)
	if err != nil {
		return out, fmt.Errorf("preflight: %w", err)
	}
	var runner stall.Runner
	runStage := func(b stall.Behaviour, h stall.Harness, input stall.Task) (stall.Result, error) {
		stageCtx, cancel := context.WithTimeout(ctx, config.StageTimeout)
		defer cancel()
		return runner.Run(stageCtx, b, h, input)
	}
	out.Implementation, err = runStage(behaviour.Implement{}, implementation, task)
	if err != nil {
		return out, fmt.Errorf("implementation: %w", err)
	}
	evidence, err := repo.evidence(ctx)
	if err != nil {
		return out, fmt.Errorf("capture changes: %w", err)
	}
	if evidence == "" {
		return out, errors.New("implementation produced no reviewable changes")
	}
	input, err := json.Marshal(struct {
		Task    string `json:"original_task"`
		Summary string `json:"implementation_summary"`
		Diff    string `json:"diff"`
	}{task.Input, out.Implementation.Output, evidence})
	if err != nil {
		return out, err
	}
	out.Review, err = runStage(behaviour.Review{}, review, stall.Task{Input: string(input)})
	if err != nil {
		return out, fmt.Errorf("review: %w", err)
	}
	after, err := repo.evidence(ctx)
	if err != nil {
		return out, fmt.Errorf("verify reviewed changes: %w", err)
	}
	if after != evidence {
		return out, errors.New("changes differ from the reviewed snapshot; approval is no longer valid")
	}
	return out, nil
}
