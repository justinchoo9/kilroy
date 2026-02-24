package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func loadCreateRunfileSkill(t *testing.T) string {
	t.Helper()
	p := filepath.Join(findRepoRoot(t), "skills", "create-runfile", "SKILL.md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read create-runfile skill: %v", err)
	}
	return string(b)
}

func loadCreateRunfileTemplateMap(t *testing.T) map[string]any {
	t.Helper()
	p := filepath.Join(findRepoRoot(t), "skills", "create-runfile", "reference_run_template.yaml")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read create-runfile template: %v", err)
	}
	var data map[string]any
	if err := yaml.Unmarshal(b, &data); err != nil {
		t.Fatalf("parse create-runfile template: %v", err)
	}
	return data
}

func asMap(t *testing.T, v any, path string) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%s: expected map[string]any, got %T", path, v)
	}
	return m
}

func asSlice(t *testing.T, v any, path string) []any {
	t.Helper()
	s, ok := v.([]any)
	if !ok {
		t.Fatalf("%s: expected []any, got %T", path, v)
	}
	return s
}

func TestCreateRunfileSkill_RequiresSchemaBackedFieldsOnly(t *testing.T) {
	skill := loadCreateRunfileSkill(t)
	required := []string{
		"Emit only fields supported by `internal/attractor/engine/config.go`.",
		"do not emit unsupported keys",
		"`artifact_policy.checkpoint.exclude_globs`",
		"`git.checkpoint_exclude_globs`",
	}
	for _, needle := range required {
		if !strings.Contains(skill, needle) {
			t.Fatalf("create-runfile skill missing required guardrail text: %q", needle)
		}
	}
}

func TestCreateRunfileTemplate_UsesArtifactPolicyCheckpointExcludes(t *testing.T) {
	template := loadCreateRunfileTemplateMap(t)

	gitMap := asMap(t, template["git"], "git")
	if _, ok := gitMap["checkpoint_exclude_globs"]; ok {
		t.Fatal("template must not use deprecated git.checkpoint_exclude_globs")
	}

	artifactPolicy := asMap(t, template["artifact_policy"], "artifact_policy")
	checkpoint := asMap(t, artifactPolicy["checkpoint"], "artifact_policy.checkpoint")
	excludes := asSlice(t, checkpoint["exclude_globs"], "artifact_policy.checkpoint.exclude_globs")
	if len(excludes) == 0 {
		t.Fatal("artifact_policy.checkpoint.exclude_globs must not be empty")
	}
}
