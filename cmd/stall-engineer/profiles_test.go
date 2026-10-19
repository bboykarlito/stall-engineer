package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func profileFile(t *testing.T, model, tools string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "profile.yaml")
	text := "harness: pi\nversion: 1\nmodel: " + model + "\nprovider: profile-provider\ntools:\n  allow: [" + tools + "]\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestProfileSelectionAndOverrides(t *testing.T) {
	implPath := profileFile(t, "implement-model", "read,write,bash")
	reviewPath := profileFile(t, "review-model", "read,grep")
	repo := t.TempDir()
	impl, review, err := loadConfigs(implPath, reviewPath, repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if impl.Model != "implement-model" || review.Model != "review-model" || impl.Provider != "profile-provider" {
		t.Fatal("profile values overwritten by defaults")
	}
	impl, review, err = loadConfigs(implPath, reviewPath, repo, map[string]string{"model": "override", "provider": ""})
	if err != nil {
		t.Fatal(err)
	}
	if impl.Model != "override" || review.Model != "override" || impl.Provider != "" || review.Provider != "" {
		t.Fatal("explicit overrides not applied")
	}
	if _, _, err := loadConfigs(implPath, reviewPath, repo, map[string]string{"model": ""}); err == nil {
		t.Fatal("empty model override accepted")
	}
}

func TestUnsafeReviewRejectedBeforeExecution(t *testing.T) {
	for _, tool := range []string{"bash", "write", "edit"} {
		t.Run(tool, func(t *testing.T) {
			implPath := profileFile(t, "model", "read,bash")
			reviewPath := profileFile(t, "model", "read,"+tool)
			// Neither Git nor Pi is available: review validation must fail before either runs.
			t.Setenv("PATH", "")
			var output bytes.Buffer
			err := run(context.Background(), []string{"-repo", t.TempDir(), "-task", "fix", "-implement-profile", implPath, "-review-profile", reviewPath}, &output, &output)
			if err == nil || !strings.Contains(err.Error(), "review profile must be read-only") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestShippedProfiles(t *testing.T) {
	impl, review, err := loadConfigs("../../harnesses/implement.yaml", "../../harnesses/review.yaml", t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if impl.Model != "gpt-5.6-terra" || review.Model != "gpt-5.6-terra" || len(impl.Tools) != 7 || len(review.Tools) != 4 {
		t.Fatal("unexpected shipped defaults")
	}
}
