// Package pi configures the Pi subprocess harness.
package pi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultModel is the initial engineering agent model. Availability depends on
// the installed Pi provider catalog and credentials.
const DefaultModel = "gpt-5.6-terra"

// Config describes one Pi invocation. It does not launch a process.
// Provider may be empty to let Pi resolve Model using its configured providers.
// Tools must explicitly list the built-in tools available to the model.
// WorkDir must be an absolute path to an existing directory.
type Config struct {
	WorkDir  string
	Provider string
	Model    string
	Tools    []string
}

// ImplementationConfig permits editing and shell commands. It is not a sandbox:
// shell access can bypass prompt-level restrictions on Git publication.
func ImplementationConfig(workDir string) Config {
	return Config{
		WorkDir: workDir,
		Model:   DefaultModel,
		Tools:   []string{"read", "grep", "find", "ls", "edit", "write", "bash"},
	}
}

// ReviewConfig exposes only read-only built-in tools, with no shell.
// This is not filesystem isolation; readable files are not confined to WorkDir.
func ReviewConfig(workDir string) Config {
	return Config{
		WorkDir: workDir,
		Model:   DefaultModel,
		Tools:   []string{"read", "grep", "find", "ls"},
	}
}

// Args validates configuration and returns arguments for a fresh RPC session.
// The process adapter separately sets exec.Cmd.Dir to WorkDir and
// sends the prepared request over stdin. Arguments must not be joined into a shell command.
// No resume flags or arbitrary additional arguments are accepted here.
func (c Config) Args() ([]string, error) {
	if !filepath.IsAbs(c.WorkDir) {
		return nil, errors.New("pi working directory must be absolute")
	}
	info, err := os.Stat(c.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("pi working directory: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("pi working directory is not a directory")
	}
	if strings.TrimSpace(c.Model) == "" {
		return nil, errors.New("pi model is required")
	}
	if len(c.Tools) == 0 {
		return nil, errors.New("pi tool allowlist is required")
	}
	seen := make(map[string]bool)
	for _, tool := range c.Tools {
		switch tool {
		case "read", "grep", "find", "ls", "edit", "write", "bash":
		default:
			return nil, fmt.Errorf("unsupported pi tool %q", tool)
		}
		if seen[tool] {
			return nil, fmt.Errorf("duplicate pi tool %q", tool)
		}
		seen[tool] = true
	}

	args := []string{
		"--mode", "rpc",
		"--no-session",
		"--no-approve",
		"--no-extensions",
		"--no-skills",
		"--no-prompt-templates",
		"--no-context-files",
		"--model", c.Model,
		"--tools", strings.Join(c.Tools, ","),
	}
	if c.Provider != "" {
		if strings.TrimSpace(c.Provider) == "" {
			return nil, errors.New("pi provider must not be whitespace")
		}
		args = append(args, "--provider", c.Provider)
	}
	return args, nil
}
