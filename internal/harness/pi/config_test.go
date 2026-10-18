package pi

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestImplementationArgs(t *testing.T) {
	c := ImplementationConfig(t.TempDir())
	args, err := c.Args()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--mode", "rpc", "--no-session", "--no-approve",
		"--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files",
		"--model", "gpt-5.6-terra", "--tools", "read,grep,find,ls,edit,write,bash",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %q, want %q", args, want)
	}
}

func TestReviewToolsAndOverrides(t *testing.T) {
	c := ReviewConfig(t.TempDir())
	if !reflect.DeepEqual(c.Tools, []string{"read", "grep", "find", "ls"}) {
		t.Fatalf("unexpected review tools: %v", c.Tools)
	}
	c.Provider = "configured-provider"
	c.Model = "another-model"
	args, err := c.Args()
	if err != nil {
		t.Fatal(err)
	}
	wantTail := []string{"--model", "another-model", "--tools", "read,grep,find,ls", "--provider", "configured-provider"}
	if !reflect.DeepEqual(args[len(args)-len(wantTail):], wantTail) {
		t.Fatalf("overrides missing: %q", args)
	}
}

func TestInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*Config){
		"empty directory":    func(c *Config) { c.WorkDir = "" },
		"relative directory": func(c *Config) { c.WorkDir = "." },
		"missing directory":  func(c *Config) { c.WorkDir = filepath.Join(dir, "missing") },
		"file as directory":  func(c *Config) { c.WorkDir = file },
		"empty model":        func(c *Config) { c.Model = " " },
		"blank provider":     func(c *Config) { c.Provider = " " },
		"no tools":           func(c *Config) { c.Tools = nil },
		"unknown tool":       func(c *Config) { c.Tools = []string{"custom"} },
		"duplicate tool":     func(c *Config) { c.Tools = []string{"read", "read"} },
		"combined tools":     func(c *Config) { c.Tools = []string{"read,bash"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := ImplementationConfig(dir)
			mutate(&c)
			if args, err := c.Args(); err == nil || args != nil {
				t.Fatalf("expected rejection, got args %q, err %v", args, err)
			}
		})
	}
}

func TestConfigsDoNotShareTools(t *testing.T) {
	a, b := ReviewConfig(t.TempDir()), ReviewConfig(t.TempDir())
	a.Tools[0] = "bash"
	if b.Tools[0] != "read" {
		t.Fatal("tool allowlists share mutable state")
	}
}
