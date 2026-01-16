package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileType represents the type of a netroub configuration file.
type FileType int

const (
	FileTypeUnknown FileType = iota
	FileTypePlan
	FileTypeScenario
)

// Plan represents a batch execution plan loaded from a YAML or JSON file.
type Plan struct {
	Parallel  int             `yaml:"parallel" json:"parallel"`
	Scenarios []ScenarioEntry `yaml:"scenarios" json:"scenarios"`
}

// ScenarioEntry represents a single scenario entry in the plan.
type ScenarioEntry struct {
	Pattern string `yaml:"pattern" json:"pattern"` // File path or glob pattern
	Repeat  int    `yaml:"repeat" json:"repeat"`   // Number of repetitions
	YAML    bool   `yaml:"yaml" json:"yaml"`       // Use YAML format (default: false, JSON)
}

// DetectFileType detects whether a file is a Plan or Scenario based on its content.
// Returns FileTypePlan if the file contains "scenarios" key,
// FileTypeScenario if it contains "event" or "scenarioName" key,
// FileTypeUnknown otherwise.
func DetectFileType(path string) (FileType, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileTypeUnknown, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse as generic map to detect keys
	var content map[string]any

	// Try YAML first (YAML is a superset of JSON, so this handles both)
	if err := yaml.Unmarshal(data, &content); err != nil {
		return FileTypeUnknown, fmt.Errorf("failed to parse file as YAML/JSON: %w", err)
	}

	// Check for Plan-specific keys
	if _, ok := content["scenarios"]; ok {
		return FileTypePlan, nil
	}

	// Check for Scenario-specific keys
	if _, ok := content["event"]; ok {
		return FileTypeScenario, nil
	}
	if _, ok := content["scenarioName"]; ok {
		return FileTypeScenario, nil
	}

	return FileTypeUnknown, nil
}

// IsYAMLExtension returns true if the file extension indicates YAML format.
func IsYAMLExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

// LoadPlan loads a plan from a YAML or JSON file.
// The file format is automatically detected based on content.
func LoadPlan(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var plan Plan

	// Try YAML first (handles both YAML and JSON since YAML is a superset)
	if err := yaml.Unmarshal(data, &plan); err != nil {
		// If YAML fails, try explicit JSON parsing for better error messages
		if jsonErr := json.Unmarshal(data, &plan); jsonErr != nil {
			return nil, fmt.Errorf("failed to parse plan file (tried YAML and JSON): YAML error: %v, JSON error: %v", err, jsonErr)
		}
	}

	// Validate that this looks like a plan file
	if len(plan.Scenarios) == 0 {
		return nil, fmt.Errorf("invalid plan file: no scenarios defined")
	}

	// Set defaults
	if plan.Parallel < 1 {
		plan.Parallel = 1
	}

	for i := range plan.Scenarios {
		if plan.Scenarios[i].Repeat < 1 {
			plan.Scenarios[i].Repeat = 1
		}
	}

	return &plan, nil
}

// ExpandScenarios expands glob patterns in the plan and returns all matching files.
// The baseDir is used as the base directory for relative patterns.
func (p *Plan) ExpandScenarios(baseDir string) ([]ScenarioEntry, error) {
	var expanded []ScenarioEntry

	for _, entry := range p.Scenarios {
		pattern := entry.Pattern

		// Make pattern absolute if relative
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(baseDir, pattern)
		}

		// Expand glob pattern
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", entry.Pattern, err)
		}

		if len(matches) == 0 {
			// If no matches and pattern contains no wildcards, treat as literal path
			if !containsGlobChar(entry.Pattern) {
				expanded = append(expanded, entry)
			} else {
				return nil, fmt.Errorf("no files match pattern %q", entry.Pattern)
			}
		} else {
			// Create an entry for each matched file
			for _, match := range matches {
				expanded = append(expanded, ScenarioEntry{
					Pattern: match,
					Repeat:  entry.Repeat,
					YAML:    entry.YAML,
				})
			}
		}
	}

	return expanded, nil
}

// GenerateTasksFromPlan generates tasks from a plan.
// Returns all tasks for all scenarios with their repetitions.
func GenerateTasksFromPlan(plan *Plan, baseDir string) ([]*Task, error) {
	expanded, err := plan.ExpandScenarios(baseDir)
	if err != nil {
		return nil, err
	}

	var allTasks []*Task
	for _, entry := range expanded {
		tasks := GenerateTasks(entry.Pattern, entry.Repeat, entry.YAML)
		allTasks = append(allTasks, tasks...)
	}

	return allTasks, nil
}

// containsGlobChar checks if the pattern contains glob special characters.
func containsGlobChar(pattern string) bool {
	for _, c := range pattern {
		if c == '*' || c == '?' || c == '[' {
			return true
		}
	}
	return false
}

// PlanSummary returns a summary of the plan.
func (p *Plan) Summary() (scenarios, totalRuns int) {
	scenarios = len(p.Scenarios)
	for _, entry := range p.Scenarios {
		totalRuns += entry.Repeat
	}
	return
}
