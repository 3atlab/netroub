package executor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPlan(t *testing.T) {
	// Create a temporary plan file
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.yaml")

	planContent := `
parallel: 4

scenarios:
  - pattern: "baseline.json"
    repeat: 50
  - pattern: "A*_*.json"
    repeat: 110
  - pattern: "scenario.yaml"
    repeat: 10
    yaml: true
`
	err := os.WriteFile(planPath, []byte(planContent), 0644)
	require.NoError(t, err)

	plan, err := LoadPlan(planPath)
	require.NoError(t, err)

	assert.Equal(t, 4, plan.Parallel)
	assert.Equal(t, 3, len(plan.Scenarios))

	assert.Equal(t, "baseline.json", plan.Scenarios[0].Pattern)
	assert.Equal(t, 50, plan.Scenarios[0].Repeat)
	assert.False(t, plan.Scenarios[0].YAML)

	assert.Equal(t, "A*_*.json", plan.Scenarios[1].Pattern)
	assert.Equal(t, 110, plan.Scenarios[1].Repeat)

	assert.Equal(t, "scenario.yaml", plan.Scenarios[2].Pattern)
	assert.Equal(t, 10, plan.Scenarios[2].Repeat)
	assert.True(t, plan.Scenarios[2].YAML)
}

func TestLoadPlan_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.yaml")

	// Minimal plan with defaults
	planContent := `
scenarios:
  - pattern: "test.json"
`
	err := os.WriteFile(planPath, []byte(planContent), 0644)
	require.NoError(t, err)

	plan, err := LoadPlan(planPath)
	require.NoError(t, err)

	// Default parallel should be 1
	assert.Equal(t, 1, plan.Parallel)
	// Default repeat should be 1
	assert.Equal(t, 1, plan.Scenarios[0].Repeat)
}

func TestLoadPlan_FileNotFound(t *testing.T) {
	_, err := LoadPlan("/nonexistent/plan.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read plan file")
}

func TestLoadPlan_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.yaml")

	err := os.WriteFile(planPath, []byte("invalid: yaml: content:"), 0644)
	require.NoError(t, err)

	_, err = LoadPlan(planPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse plan file")
}

func TestPlan_ExpandScenarios(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test scenario files
	testFiles := []string{
		"A1_delay.json",
		"A2_loss.json",
		"B1_corrupt.json",
		"baseline.json",
	}
	for _, f := range testFiles {
		err := os.WriteFile(filepath.Join(tmpDir, f), []byte("{}"), 0644)
		require.NoError(t, err)
	}

	plan := &Plan{
		Parallel: 2,
		Scenarios: []ScenarioEntry{
			{Pattern: "A*.json", Repeat: 10},
			{Pattern: "baseline.json", Repeat: 5},
		},
	}

	expanded, err := plan.ExpandScenarios(tmpDir)
	require.NoError(t, err)

	// A*.json should match A1_delay.json and A2_loss.json
	// baseline.json is a literal match
	assert.Equal(t, 3, len(expanded))

	// Check that repeats are preserved
	for _, e := range expanded {
		if filepath.Base(e.Pattern) == "baseline.json" {
			assert.Equal(t, 5, e.Repeat)
		} else {
			assert.Equal(t, 10, e.Repeat)
		}
	}
}

func TestPlan_ExpandScenarios_NoMatch(t *testing.T) {
	tmpDir := t.TempDir()

	plan := &Plan{
		Scenarios: []ScenarioEntry{
			{Pattern: "nonexistent*.json", Repeat: 10},
		},
	}

	_, err := plan.ExpandScenarios(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no files match pattern")
}

func TestPlan_ExpandScenarios_LiteralPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.json")
	err := os.WriteFile(testFile, []byte("{}"), 0644)
	require.NoError(t, err)

	plan := &Plan{
		Scenarios: []ScenarioEntry{
			{Pattern: "test.json", Repeat: 5},
		},
	}

	expanded, err := plan.ExpandScenarios(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 1, len(expanded))
	assert.Equal(t, testFile, expanded[0].Pattern)
	assert.Equal(t, 5, expanded[0].Repeat)
}

func TestGenerateTasksFromPlan(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test scenario files
	testFiles := []string{"A1.json", "A2.json"}
	for _, f := range testFiles {
		err := os.WriteFile(filepath.Join(tmpDir, f), []byte("{}"), 0644)
		require.NoError(t, err)
	}

	plan := &Plan{
		Parallel: 4,
		Scenarios: []ScenarioEntry{
			{Pattern: "A*.json", Repeat: 3},
		},
	}

	tasks, err := GenerateTasksFromPlan(plan, tmpDir)
	require.NoError(t, err)

	// 2 files × 3 repeats = 6 tasks
	assert.Equal(t, 6, len(tasks))

	// Check task IDs
	expectedIDs := map[string]bool{
		"A1_001": true, "A1_002": true, "A1_003": true,
		"A2_001": true, "A2_002": true, "A2_003": true,
	}
	for _, task := range tasks {
		assert.True(t, expectedIDs[task.RunID], "unexpected task ID: %s", task.RunID)
	}
}

func TestPlan_Summary(t *testing.T) {
	plan := &Plan{
		Parallel: 4,
		Scenarios: []ScenarioEntry{
			{Pattern: "A1.json", Repeat: 100},
			{Pattern: "A2.json", Repeat: 50},
			{Pattern: "B1.json", Repeat: 10},
		},
	}

	scenarios, totalRuns := plan.Summary()
	assert.Equal(t, 3, scenarios)
	assert.Equal(t, 160, totalRuns)
}

func TestContainsGlobChar(t *testing.T) {
	tests := []struct {
		pattern string
		want    bool
	}{
		{"test.json", false},
		{"test*.json", true},
		{"test?.json", true},
		{"test[1-3].json", true},
		{"/path/to/file.json", false},
		{"A*_*.json", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got := containsGlobChar(tt.pattern)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectFileType_Plan(t *testing.T) {
	tmpDir := t.TempDir()

	// Test YAML plan
	yamlPlanPath := filepath.Join(tmpDir, "plan.yaml")
	yamlContent := `
parallel: 4
scenarios:
  - pattern: "test.json"
    repeat: 10
`
	err := os.WriteFile(yamlPlanPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	fileType, err := DetectFileType(yamlPlanPath)
	require.NoError(t, err)
	assert.Equal(t, FileTypePlan, fileType)

	// Test JSON plan
	jsonPlanPath := filepath.Join(tmpDir, "plan.json")
	jsonContent := `{
  "parallel": 4,
  "scenarios": [
    {"pattern": "test.json", "repeat": 10}
  ]
}`
	err = os.WriteFile(jsonPlanPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	fileType, err = DetectFileType(jsonPlanPath)
	require.NoError(t, err)
	assert.Equal(t, FileTypePlan, fileType)
}

func TestDetectFileType_Scenario(t *testing.T) {
	tmpDir := t.TempDir()

	// Test JSON scenario with "event" key
	jsonScenarioPath := filepath.Join(tmpDir, "scenario.json")
	jsonContent := `{
  "scenarioName": "test_scenario",
  "duration": "60s",
  "topo": "test.yaml",
  "event": [
    {"beginTime": "0s", "type": "delay"}
  ]
}`
	err := os.WriteFile(jsonScenarioPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	fileType, err := DetectFileType(jsonScenarioPath)
	require.NoError(t, err)
	assert.Equal(t, FileTypeScenario, fileType)

	// Test YAML scenario with "scenarioName" key
	yamlScenarioPath := filepath.Join(tmpDir, "scenario.yaml")
	yamlContent := `
scenarioName: test_scenario
duration: 60s
topo: test.yaml
event:
  - beginTime: 0s
    type: delay
`
	err = os.WriteFile(yamlScenarioPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	fileType, err = DetectFileType(yamlScenarioPath)
	require.NoError(t, err)
	assert.Equal(t, FileTypeScenario, fileType)
}

func TestDetectFileType_Unknown(t *testing.T) {
	tmpDir := t.TempDir()

	// Test file with no recognizable keys
	unknownPath := filepath.Join(tmpDir, "unknown.yaml")
	content := `
foo: bar
baz: 123
`
	err := os.WriteFile(unknownPath, []byte(content), 0644)
	require.NoError(t, err)

	fileType, err := DetectFileType(unknownPath)
	require.NoError(t, err)
	assert.Equal(t, FileTypeUnknown, fileType)
}

func TestDetectFileType_FileNotFound(t *testing.T) {
	_, err := DetectFileType("/nonexistent/file.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestDetectFileType_InvalidContent(t *testing.T) {
	tmpDir := t.TempDir()

	invalidPath := filepath.Join(tmpDir, "invalid.yaml")
	err := os.WriteFile(invalidPath, []byte("invalid: yaml: : content"), 0644)
	require.NoError(t, err)

	_, err = DetectFileType(invalidPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse file")
}

func TestIsYAMLExtension(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"test.yaml", true},
		{"test.yml", true},
		{"test.YAML", true},
		{"test.YML", true},
		{"test.json", false},
		{"test.JSON", false},
		{"/path/to/scenario.yaml", true},
		{"/path/to/scenario.json", false},
		{"no_extension", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsYAMLExtension(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLoadPlan_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")

	// JSON format plan
	planContent := `{
  "parallel": 2,
  "scenarios": [
    {"pattern": "baseline.json", "repeat": 50},
    {"pattern": "A*.json", "repeat": 110},
    {"pattern": "scenario.yaml", "repeat": 10, "yaml": true}
  ]
}`
	err := os.WriteFile(planPath, []byte(planContent), 0644)
	require.NoError(t, err)

	plan, err := LoadPlan(planPath)
	require.NoError(t, err)

	assert.Equal(t, 2, plan.Parallel)
	assert.Equal(t, 3, len(plan.Scenarios))

	assert.Equal(t, "baseline.json", plan.Scenarios[0].Pattern)
	assert.Equal(t, 50, plan.Scenarios[0].Repeat)
	assert.False(t, plan.Scenarios[0].YAML)

	assert.Equal(t, "A*.json", plan.Scenarios[1].Pattern)
	assert.Equal(t, 110, plan.Scenarios[1].Repeat)

	assert.Equal(t, "scenario.yaml", plan.Scenarios[2].Pattern)
	assert.Equal(t, 10, plan.Scenarios[2].Repeat)
	assert.True(t, plan.Scenarios[2].YAML)
}

func TestLoadPlan_EmptyScenarios(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.yaml")

	// Plan with no scenarios
	planContent := `
parallel: 4
scenarios: []
`
	err := os.WriteFile(planPath, []byte(planContent), 0644)
	require.NoError(t, err)

	_, err = LoadPlan(planPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no scenarios defined")
}
