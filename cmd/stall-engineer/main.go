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
	pi "github.com/bboykarlito/pi-in-stall"
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
	implementationProfile := flags.String("implement-profile", "harnesses/implement.yaml", "Pi implementation YAML profile (relative to current directory)")
	reviewProfile := flags.String("review-profile", "harnesses/review.yaml", "Pi review YAML profile (relative to current directory)")
	model := flags.String("model", "", "override both profile models (shipped profiles: "+pi.DefaultModel+")")
	provider := flags.String("provider", "", "override both profile providers; explicit empty value uses Pi resolution")
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
	overrides := make(map[string]string)
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "model" {
			overrides["model"] = *model
		}
		if f.Name == "provider" {
			overrides["provider"] = *provider
		}
	})
	implConfig, reviewConfig, err := loadConfigs(*implementationProfile, *reviewProfile, absolute, overrides)
	if err != nil {
		return err
	}
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
