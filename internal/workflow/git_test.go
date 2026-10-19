package workflow

import (
	"context"
	"strings"
	"testing"
)

func TestEvidenceIncludesStagedBinaryAndNewFiles(t *testing.T) {
	config := fixture(t)
	repo, err := inspect(context.Background(), config.Repo, config.BaseBranch)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, config.Repo, "tracked.txt", "staged content\n")
	gitTest(t, config.Repo, "add", "tracked.txt")
	writeFile(t, config.Repo, "binary.dat", "\x00\x01\x02\xff")
	writeFile(t, config.Repo, "empty.txt", "")
	evidence, err := repo.evidence(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"+staged content", "GIT binary patch", "empty.txt"} {
		if !strings.Contains(evidence, part) {
			t.Fatalf("missing %q in evidence: %s", part, evidence)
		}
	}
}

func TestRejectSubmoduleRepository(t *testing.T) {
	config := fixture(t)
	head := gitTest(t, config.Repo, "rev-parse", "HEAD")
	gitTest(t, config.Repo, "update-index", "--add", "--cacheinfo", "160000,"+head+",nested")
	gitTest(t, config.Repo, "commit", "-m", "add gitlink")
	if _, err := inspect(context.Background(), config.Repo, config.BaseBranch); err == nil {
		t.Fatal("accepted submodule repository")
	}
}

func TestCappedBuffer(t *testing.T) {
	var b cappedBuffer
	data := []byte(strings.Repeat("x", maxEvidenceBytes+1))
	n, err := b.Write(data)
	if err != nil || n != len(data) || !b.exceeded || b.Len() != maxEvidenceBytes {
		t.Fatal("output cap failed")
	}
	n, err = b.Write([]byte("more"))
	if err != nil || n != 4 || b.Len() != maxEvidenceBytes {
		t.Fatal("output not drained after limit")
	}
}
