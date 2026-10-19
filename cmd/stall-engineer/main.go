// Command stall-engineer implements and reviews a task, leaving changes local.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	stall "github.com/bboykarlito/libstall"
	"stall-engineer/internal/harness/pi"
	"stall-engineer/internal/workflow"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("stall-engineer", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repo := flags.String("repo", "", "absolute or relative path to a clean repository root on a feature branch")
	task := flags.String("task", "", "engineering task")
	model := flags.String("model", pi.DefaultModel, "Pi model ID for both stages")
	provider := flags.String("provider", "", "optional Pi provider ID")
	base := flags.String("base-branch", "main", "additional protected base branch (main, master, trunk also protected)")
	timeout := flags.Duration("timeout", 10*time.Minute, "timeout for each model stage")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}
	if *repo == "" || strings.TrimSpace(*task) == "" {
		return fmt.Errorf("-repo and -task are required")
	}
	if *timeout <= 0 || strings.TrimSpace(*base) == "" {
		return fmt.Errorf("-timeout must be positive and -base-branch must be nonempty")
	}
	absolute, err := filepath.Abs(*repo)
	if err != nil {
		return err
	}
	implConfig := pi.ImplementationConfig(absolute)
	reviewConfig := pi.ReviewConfig(absolute)
	implConfig.Model, reviewConfig.Model = *model, *model
	implConfig.Provider, reviewConfig.Provider = *provider, *provider
	implementation, err := pi.New(implConfig)
	if err != nil {
		return err
	}
	review, err := pi.New(reviewConfig)
	if err != nil {
		return err
	}
	fmt.Fprintln(stderr, "Implementation has shell access: this is not a sandbox. The application will not commit or push.")
	out, runErr := workflow.Run(ctx, workflow.Config{Repo: absolute, BaseBranch: *base, StageTimeout: *timeout}, stall.Task{Input: *task}, implementation, review)
	report(stdout, "Implementation", out.Implementation)
	report(stdout, "Review", out.Review)
	if runErr != nil {
		return runErr
	}
	fmt.Fprintln(stdout, "Review approved. Changes remain local; inspect the diff and verify tests before publishing.")
	return nil
}

func report(w io.Writer, stage string, result stall.Result) {
	fmt.Fprintf(w, "\n%s\n%s\nReported tokens: input=%d output=%d (zero may mean unavailable; not billing data)\n", stage, result.Output, result.Usage.InputTokens, result.Usage.OutputTokens)
}
