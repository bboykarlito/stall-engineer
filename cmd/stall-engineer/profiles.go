package main

import (
	"fmt"

	pi "github.com/bboykarlito/pi-in-stall"
)

// Loading belongs to the adapter; the read-only review requirement belongs to
// this application. Validate both profiles before starting either model stage.
func loadConfigs(implementationPath, reviewPath, workDir string, overrides map[string]string) (pi.Config, pi.Config, error) {
	load := func(path string) (pi.Config, error) {
		profile, err := pi.LoadProfile(path)
		if err != nil {
			return pi.Config{}, fmt.Errorf("profile %s: %w", path, err)
		}
		if model, ok := overrides["model"]; ok {
			profile.Model = model
		}
		if provider, ok := overrides["provider"]; ok {
			profile.Provider = provider
		}
		config, err := profile.Config(workDir)
		if err != nil {
			return pi.Config{}, fmt.Errorf("profile %s: %w", path, err)
		}
		return config, nil
	}
	implementation, err := load(implementationPath)
	if err != nil {
		return pi.Config{}, pi.Config{}, err
	}
	review, err := load(reviewPath)
	if err != nil {
		return pi.Config{}, pi.Config{}, err
	}
	for _, tool := range review.Tools {
		switch tool {
		case "read", "grep", "find", "ls":
		default:
			return pi.Config{}, pi.Config{}, fmt.Errorf("review profile must be read-only: tool %q is forbidden", tool)
		}
	}
	return implementation, review, nil
}
