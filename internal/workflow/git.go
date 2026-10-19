package workflow

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const maxEvidenceBytes = 4 * 1024 * 1024

type repository struct{ dir, branch, head string }

// cappedBuffer drains output but retains only a bounded prefix.
type cappedBuffer struct {
	bytes.Buffer
	exceeded bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := maxEvidenceBytes - b.Len()
	if len(p) > remaining {
		b.exceeded = true
		p = p[:remaining]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var out, stderr cappedBuffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if out.exceeded || stderr.exceeded {
		return "", errors.New("git output exceeds 4 MiB limit")
	}
	if err != nil {
		return out.String(), fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return out.String(), nil
}

func inspect(ctx context.Context, dir, baseBranch string) (repository, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return repository{}, err
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return repository{}, err
	}
	top, err := git(ctx, absolute, "rev-parse", "--show-toplevel")
	if err != nil {
		return repository{}, err
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(top))
	if err != nil {
		return repository{}, err
	}
	if root != absolute {
		return repository{}, errors.New("repository path must be the worktree root")
	}
	branch, err := git(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return repository{}, errors.New("a named feature branch is required (not detached HEAD)")
	}
	branch = strings.TrimSpace(branch)
	if branch == baseBranch || branch == "main" || branch == "master" || branch == "trunk" {
		return repository{}, fmt.Errorf("refusing protected branch %q; select a feature branch", branch)
	}
	remoteHead, err := git(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil && strings.TrimSpace(remoteHead) == "origin/"+branch {
		return repository{}, fmt.Errorf("refusing origin's default branch %q", branch)
	}
	if ctx.Err() != nil {
		return repository{}, ctx.Err()
	}
	status, err := git(ctx, root, "status", "--porcelain", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return repository{}, err
	}
	if status != "" {
		return repository{}, errors.New("repository must have a clean index and worktree, including untracked files")
	}
	entries, err := git(ctx, root, "ls-files", "--stage")
	if err != nil {
		return repository{}, err
	}
	for _, entry := range strings.Split(entries, "\n") {
		if strings.HasPrefix(entry, "160000 ") {
			return repository{}, errors.New("repositories with submodules are not supported by this initial review workflow")
		}
	}
	head, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return repository{}, err
	}
	return repository{dir: root, branch: branch, head: strings.TrimSpace(head)}, nil
}

func (r repository) unchangedHead(ctx context.Context) error {
	head, err := git(ctx, r.dir, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	branch, err := git(ctx, r.dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(head) != r.head || strings.TrimSpace(branch) != r.branch {
		return errors.New("HEAD or branch changed during execution; refusing to continue")
	}
	return nil
}

func (r repository) evidence(ctx context.Context) (string, error) {
	if err := r.unchangedHead(ctx); err != nil {
		return "", err
	}
	// Disable repository-selected external diff commands and text converters.
	diff, err := git(ctx, r.dir, "diff", "--no-ext-diff", "--no-textconv", "--binary", "--submodule=short", "HEAD", "--")
	if err != nil {
		return "", err
	}
	files, err := git(ctx, r.dir, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", err
	}
	var result strings.Builder
	result.WriteString(diff)
	for _, file := range strings.Split(files, "\x00") {
		if file == "" {
			continue
		}
		added, err := git(ctx, r.dir, "diff", "--no-index", "--no-ext-diff", "--no-textconv", "--binary", "--", "/dev/null", file)
		// --no-index returns 1 when differences exist.
		var exit *exec.ExitError
		if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
			return "", err
		}
		if result.Len()+len(added) > maxEvidenceBytes {
			return "", errors.New("change evidence exceeds 4 MiB limit")
		}
		result.WriteString(added)
	}
	return result.String(), nil
}
