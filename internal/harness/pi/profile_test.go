package pi

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const validProfile = `harness: pi
version: 1
model: gpt-5.6-terra
tools:
  allow: [read, grep]
skills:
  discover: false
  paths: []
`

func writeProfile(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "profile.yaml")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadProfile(t *testing.T) {
	path := writeProfile(t, validProfile)
	p, err := LoadProfile(path)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	c, err := p.Config(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Model != DefaultModel || c.WorkDir != dir || !reflect.DeepEqual(c.Tools, []string{"read", "grep"}) {
		t.Fatalf("config=%+v", c)
	}
	p.Tools.Allow[0] = "bash"
	if c.Tools[0] != "read" {
		t.Fatal("config shares profile tools")
	}
}

func TestProfileRejections(t *testing.T) {
	cases := map[string]string{
		"empty":               "",
		"null":                "null",
		"unknown root":        validProfile + "api_key: secret\n",
		"unknown nested":      strings.Replace(validProfile, "  allow:", "  alow:", 1),
		"duplicate key":       validProfile + "model: another\n",
		"multiple docs":       validProfile + "---\n" + validProfile,
		"trailing empty doc":  validProfile + "---\n",
		"unsupported harness": strings.Replace(validProfile, "harness: pi", "harness: codex", 1),
		"unsupported version": strings.Replace(validProfile, "version: 1", "version: 2", 1),
		"missing version":     strings.Replace(validProfile, "version: 1\n", "", 1),
		"missing model":       strings.Replace(validProfile, "model: gpt-5.6-terra\n", "", 1),
		"empty tools":         strings.Replace(validProfile, "[read, grep]", "[]", 1),
		"unknown tool":        strings.Replace(validProfile, "[read, grep]", "[custom]", 1),
		"duplicate tools":     strings.Replace(validProfile, "[read, grep]", "[read, read]", 1),
		"tool type":           strings.Replace(validProfile, "[read, grep]", "read", 1),
		"discovery":           strings.Replace(validProfile, "discover: false", "discover: true", 1),
		"missing skill":       strings.Replace(validProfile, "paths: []", "paths: [missing]", 1),
		"empty skill":         strings.Replace(validProfile, "paths: []", "paths: ['']", 1),
		"runtime path":        validProfile + "workdir: /tmp\n",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadProfile(writeProfile(t, text)); err == nil {
				t.Fatal("invalid profile accepted")
			}
		})
	}
}

func TestExplicitSkills(t *testing.T) {
	path := writeProfile(t, strings.Replace(validProfile, "paths: []", "paths: [skills/test]", 1))
	dir := filepath.Join(filepath.Dir(path), "skills", "test")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(skill, []byte("---\nname: test\ndescription: Test skill\n---\nInstructions\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProfile(path)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(skill)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Skills.Paths, []string{resolved}) {
		t.Fatalf("skill paths=%v", p.Skills.Paths)
	}
	c, err := p.Config(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	args, err := c.Args()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args[len(args)-2:], []string{"--skill", resolved}) || !strings.Contains(strings.Join(args, " "), "--no-skills") {
		t.Fatalf("args=%v", args)
	}
	h, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	p.Skills.Paths[0] = "changed-profile"
	c.Skills[0] = "changed-config"
	if h.config.Skills[0] != resolved {
		t.Fatal("harness did not snapshot skills")
	}
}

func TestSkillValidation(t *testing.T) {
	for name, body := range map[string]string{
		"no frontmatter":      "plain text",
		"missing description": "---\nname: test\n---\n",
		"unclosed":            "---\nname: test\n",
		"malformed":           "---\nname: [\n---\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "SKILL.md")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			c := ReviewConfig(t.TempDir())
			c.Skills = []string{path}
			if _, err := c.Args(); err == nil {
				t.Fatal("invalid skill accepted")
			}
		})
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte("---\nname: duplicate\ndescription: Test\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c := ReviewConfig(dir)
	c.Skills = []string{path, path}
	if _, err := c.Args(); err == nil {
		t.Fatal("duplicate skill names accepted")
	}
}
