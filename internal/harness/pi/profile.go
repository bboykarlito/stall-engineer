package pi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Profile is the versioned Pi-specific YAML schema, independent of a repository.
// It is not a shared schema for other stall adapters. Credentials are intentionally
// absent: authentication remains owned by Pi.
type Profile struct {
	Harness  string `yaml:"harness"`
	Version  int    `yaml:"version"`
	Model    string `yaml:"model"`
	Provider string `yaml:"provider,omitempty"`
	Tools    struct {
		Allow []string `yaml:"allow"`
	} `yaml:"tools"`
	Skills struct {
		Discover bool     `yaml:"discover"`
		Paths    []string `yaml:"paths"`
	} `yaml:"skills"`
}

// LoadProfile reads exactly one YAML document, rejecting unknown/duplicate keys
// and unsupported settings. Relative skill paths are resolved against the profile
// file's directory (not the target repository). A skill directory selects only
// its SKILL.md, never a recursively discovered pool. Paths are not shell-expanded.
func LoadProfile(path string) (Profile, error) {
	var p Profile
	absolute, err := filepath.Abs(path)
	if err != nil {
		return p, err
	}
	data, err := readLimited(absolute)
	if err != nil {
		return p, fmt.Errorf("read pi profile: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&p); err != nil {
		return Profile{}, fmt.Errorf("decode pi profile: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Profile{}, errors.New("pi profile must contain exactly one YAML document")
	}
	for i, path := range p.Skills.Paths {
		if strings.TrimSpace(path) == "" {
			return Profile{}, errors.New("skill path must not be empty")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(absolute), path)
		}
		info, err := os.Stat(path)
		if err != nil {
			return Profile{}, fmt.Errorf("skill path: %w", err)
		}
		if info.IsDir() {
			path = filepath.Join(path, "SKILL.md")
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return Profile{}, fmt.Errorf("skill path: %w", err)
		}
		p.Skills.Paths[i] = resolved
	}
	if err := p.validate(); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// Config combines reusable profile settings with a per-invocation working directory.
// Slices are copied so subsequent profile mutation does not change the config.
func (p Profile) Config(workDir string) (Config, error) {
	if err := p.validate(); err != nil {
		return Config{}, err
	}
	c := p.config(workDir)
	if _, err := c.Args(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (p Profile) config(workDir string) Config {
	return Config{WorkDir: workDir, Model: p.Model, Provider: p.Provider,
		Tools: append([]string(nil), p.Tools.Allow...), Skills: append([]string(nil), p.Skills.Paths...)}
}
func (p Profile) validate() error {
	if p.Harness != "pi" {
		return fmt.Errorf("unsupported harness %q: expected pi", p.Harness)
	}
	if p.Version != 1 {
		return fmt.Errorf("unsupported pi profile version %d: expected 1", p.Version)
	}
	if p.Skills.Discover {
		return errors.New("ambient skill discovery is unsupported; select explicit skills.paths")
	}
	return p.config("").validateOptions()
}

func readLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file", path)
	}
	const limit = 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("%s exceeds 1 MiB limit", path)
	}
	return data, nil
}

func validateSkills(paths []string) error {
	names := make(map[string]bool)
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("skill file must be absolute: %q", path)
		}
		if filepath.Ext(path) != ".md" {
			return fmt.Errorf("skill must be a Markdown file: %s", path)
		}
		data, err := readLimited(path)
		if err != nil {
			return fmt.Errorf("read skill: %w", err)
		}
		text := strings.ReplaceAll(string(data), "\r\n", "\n")
		lines := strings.Split(text, "\n")
		if len(lines) < 3 || lines[0] != "---" {
			return fmt.Errorf("skill %s requires YAML frontmatter", path)
		}
		end := 1
		for end < len(lines) && lines[end] != "---" {
			end++
		}
		if end == len(lines) {
			return fmt.Errorf("skill %s has unclosed frontmatter", path)
		}
		var meta struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
		}
		if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &meta); err != nil {
			return fmt.Errorf("skill %s: %w", path, err)
		}
		if strings.TrimSpace(meta.Name) == "" || strings.TrimSpace(meta.Description) == "" {
			return fmt.Errorf("skill %s requires name and description", path)
		}
		if names[meta.Name] {
			return fmt.Errorf("duplicate selected skill name %q", meta.Name)
		}
		names[meta.Name] = true
	}
	return nil
}
