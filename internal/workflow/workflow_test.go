package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	stall "github.com/bboykarlito/libstall"
)

type harnessFunc func(context.Context, stall.Request) (stall.Result, error)

func (f harnessFunc) Execute(ctx context.Context, req stall.Request) (stall.Result, error) {
	return f(ctx, req)
}

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func fixture(t *testing.T) Config {
	t.Helper()
	dir := t.TempDir()
	gitTest(t, dir, "init", "-b", "main")
	gitTest(t, dir, "config", "user.name", "Test")
	gitTest(t, dir, "config", "user.email", "test@example.invalid")
	gitTest(t, dir, "config", "commit.gpgsign", "false")
	gitTest(t, dir, "config", "core.hooksPath", filepath.Join(dir, "no-hooks"))
	writeFile(t, dir, "tracked.txt", "before\n")
	gitTest(t, dir, "add", "tracked.txt")
	gitTest(t, dir, "commit", "-m", "initial")
	gitTest(t, dir, "switch", "-c", "feature/test")
	return Config{Repo: dir, BaseBranch: "main", StageTimeout: time.Second}
}
func writeFile(t *testing.T, dir, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflow(t *testing.T) {
	for _, scenario := range []string{"success", "implementation-error", "review-rejected", "review-mutates", "no-changes", "commits"} {
		t.Run(scenario, func(t *testing.T) {
			config := fixture(t)
			before := gitTest(t, config.Repo, "rev-parse", "HEAD")
			calls := 0
			impl := harnessFunc(func(ctx context.Context, req stall.Request) (stall.Result, error) {
				calls++
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("missing stage deadline")
				}
				if req.Task.Input != "fix task" {
					t.Fatal("task lost")
				}
				if scenario != "no-changes" {
					writeFile(t, config.Repo, "tracked.txt", "after\n")
					writeFile(t, config.Repo, "new file.txt", "new evidence\n")
				}
				result := stall.Result{Output: "implementation summary", Usage: stall.Usage{InputTokens: 2}}
				if scenario == "implementation-error" {
					return result, errors.New("provider failed")
				}
				if scenario == "commits" {
					gitTest(t, config.Repo, "add", ".")
					gitTest(t, config.Repo, "commit", "-m", "unexpected")
				}
				return result, nil
			})
			review := harnessFunc(func(ctx context.Context, req stall.Request) (stall.Result, error) {
				calls++
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("missing review deadline")
				}
				var data map[string]string
				if err := json.Unmarshal([]byte(req.Task.Input), &data); err != nil {
					t.Fatal(err)
				}
				if data["original_task"] != "fix task" || data["implementation_summary"] != "implementation summary" || !strings.Contains(data["diff"], "+new evidence") || !strings.Contains(data["diff"], "+after") {
					t.Fatalf("missing evidence: %v", data)
				}
				if scenario == "review-rejected" {
					return stall.Result{Output: `{"approved":false,"summary":"bug","findings":["bug"]}`}, nil
				}
				if scenario == "review-mutates" {
					writeFile(t, config.Repo, "tracked.txt", "different\n")
				}
				return stall.Result{Output: `{"approved":true,"summary":"looks sound","findings":[]}`}, nil
			})
			result, err := Run(context.Background(), config, stall.Task{Input: "fix task"}, impl, review)
			if (err == nil) != (scenario == "success") {
				t.Fatalf("unexpected error: %v", err)
			}
			wantCalls := 2
			if scenario == "implementation-error" || scenario == "no-changes" || scenario == "commits" {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("calls=%d want=%d", calls, wantCalls)
			}
			if result.Implementation.Output != "implementation summary" {
				t.Fatal("lost implementation result")
			}
			if scenario != "commits" && gitTest(t, config.Repo, "rev-parse", "HEAD") != before {
				t.Fatal("application created a commit")
			}
		})
	}
}

func TestPreflight(t *testing.T) {
	for _, scenario := range []string{"dirty", "untracked", "staged", "main", "detached", "custom-base", "remote-default"} {
		t.Run(scenario, func(t *testing.T) {
			config := fixture(t)
			switch scenario {
			case "dirty":
				writeFile(t, config.Repo, "tracked.txt", "dirty")
			case "untracked":
				writeFile(t, config.Repo, "new.txt", "dirty")
			case "staged":
				writeFile(t, config.Repo, "tracked.txt", "dirty")
				gitTest(t, config.Repo, "add", ".")
			case "main":
				gitTest(t, config.Repo, "switch", "main")
			case "detached":
				gitTest(t, config.Repo, "checkout", "--detach")
			case "custom-base":
				config.BaseBranch = "feature/test"
			case "remote-default":
				gitTest(t, config.Repo, "update-ref", "refs/remotes/origin/feature/test", "HEAD")
				gitTest(t, config.Repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/feature/test")
			}
			called := false
			h := harnessFunc(func(context.Context, stall.Request) (stall.Result, error) { called = true; return stall.Result{}, nil })
			_, err := Run(context.Background(), config, stall.Task{Input: "task"}, h, h)
			if err == nil || called {
				t.Fatalf("preflight bypassed: err=%v called=%v", err, called)
			}
		})
	}
}

func TestStageTimeout(t *testing.T) {
	config := fixture(t)
	config.StageTimeout = 20 * time.Millisecond
	impl := harnessFunc(func(ctx context.Context, _ stall.Request) (stall.Result, error) {
		<-ctx.Done()
		return stall.Result{Output: "partial"}, ctx.Err()
	})
	review := harnessFunc(func(context.Context, stall.Request) (stall.Result, error) {
		t.Fatal("review ran after timeout")
		return stall.Result{}, nil
	})
	out, err := Run(context.Background(), config, stall.Task{Input: "task"}, impl, review)
	if !errors.Is(err, context.DeadlineExceeded) || out.Implementation.Output != "partial" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
}
