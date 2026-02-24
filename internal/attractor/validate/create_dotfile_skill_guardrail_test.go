package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadCreateDotfileSkill(t *testing.T) string {
	t.Helper()
	p := filepath.Join(findRepoRoot(t), "skills", "create-dotfile", "SKILL.md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read create-dotfile skill: %v", err)
	}
	return string(b)
}

func TestCreateDotfileSkill_UsesCanonicalFailureClasses(t *testing.T) {
	skill := loadCreateDotfileSkill(t)
	required := []string{
		"`transient_infra`",
		"`budget_exhausted`",
		"`compilation_loop`",
		"`deterministic`",
		"`canceled`",
		"`structural`",
	}
	for _, class := range required {
		if !strings.Contains(skill, class) {
			t.Fatalf("create-dotfile skill missing canonical failure_class %s", class)
		}
	}
}

func TestCreateDotfileSkill_RejectsNonCanonicalFailureClassLabels(t *testing.T) {
	skill := loadCreateDotfileSkill(t)
	if !strings.Contains(skill, "Do not emit non-canonical `failure_class` values") {
		t.Fatal("create-dotfile skill must explicitly reject non-canonical failure_class values")
	}
	if !strings.Contains(skill, "`semantic`") {
		t.Fatal("create-dotfile skill must include semantic as an example invalid failure_class")
	}
}

func TestCreateDotfileSkill_RequiresGenericArtifactHygieneChecks(t *testing.T) {
	skill := loadCreateDotfileSkill(t)
	required := []string{
		"`verify_artifacts` checks must fail deterministically",
		"exact offending paths",
	}
	for _, needle := range required {
		if !strings.Contains(skill, needle) {
			t.Fatalf("create-dotfile skill missing artifact-hygiene guardrail text: %q", needle)
		}
	}
}
