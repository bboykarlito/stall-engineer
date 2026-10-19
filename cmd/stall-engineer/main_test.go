package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCLIArguments(t *testing.T) {
	for _, args := range [][]string{
		{}, {"-repo", "/tmp"}, {"-task", "fix"},
		{"-repo", "/tmp", "-task", "fix", "-timeout", "0s"},
		{"-repo", "/tmp", "-task", "fix", "extra"},
		{"-repo", "/tmp", "-task", "fix", "-base-branch", ""},
	} {
		var output bytes.Buffer
		if err := run(context.Background(), args, &output, &output); err == nil {
			t.Fatalf("accepted arguments %v", args)
		}
	}
}

func TestHelp(t *testing.T) {
	var output bytes.Buffer
	if err := run(context.Background(), []string{"-help"}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "gpt-5.6-terra") {
		t.Fatal("help missing model default")
	}
}

func TestReportMarksUsageUncertainty(t *testing.T) {
	// Invalid repositories fail preflight without invoking Pi, regardless of PATH.
	t.Setenv("PATH", "")
	var output bytes.Buffer
	err := run(context.Background(), []string{"-repo", t.TempDir(), "-task", "fix"}, &output, &output)
	if err == nil || !strings.Contains(err.Error(), "preflight") {
		t.Fatalf("expected preflight failure, got %v", err)
	}
	if !strings.Contains(output.String(), "zero may mean unavailable") {
		t.Fatal("missing usage caveat")
	}
}
