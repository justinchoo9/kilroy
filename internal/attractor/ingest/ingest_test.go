package ingest

import (
	"context"
	"testing"
)

func TestBuildCLIArgs(t *testing.T) {
	tests := []struct {
		name      string
		opts      Options
		wantExe   string
		checkArgs func(*testing.T, []string)
	}{
		{
			name: "basic invocation",
			opts: Options{
				Model:        "claude-sonnet-4-5",
				SkillPath:    "/repo/skills/english-to-dotfile/SKILL.md",
				Requirements: "Build a solitaire game",
			},
			wantExe: "claude",
			checkArgs: func(t *testing.T, args []string) {
				assertContains(t, args, "-p")
				assertContains(t, args, "--output-format")
				assertContains(t, args, "text")
				assertContains(t, args, "--model")
				assertContains(t, args, "claude-sonnet-4-5")
				assertContains(t, args, "--append-system-prompt-file")
				assertContains(t, args, "/repo/skills/english-to-dotfile/SKILL.md")
				assertContains(t, args, "--max-turns")
				assertContains(t, args, "--dangerously-skip-permissions")
			},
		},
		{
			name: "custom model",
			opts: Options{
				Model:        "claude-opus-4-6",
				SkillPath:    "/repo/skills/english-to-dotfile/SKILL.md",
				Requirements: "Build DTTF",
			},
			checkArgs: func(t *testing.T, args []string) {
				assertContains(t, args, "claude-opus-4-6")
			},
		},
		{
			name: "custom max turns",
			opts: Options{
				Model:        "claude-sonnet-4-5",
				SkillPath:    "/repo/skills/english-to-dotfile/SKILL.md",
				Requirements: "Build something",
				MaxTurns:     5,
			},
			checkArgs: func(t *testing.T, args []string) {
				assertContains(t, args, "5")
			},
		},
		{
			name: "no skill path omits flag",
			opts: Options{
				Model:        "claude-sonnet-4-5",
				Requirements: "Build something",
			},
			checkArgs: func(t *testing.T, args []string) {
				assertNotContains(t, args, "--append-system-prompt-file")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exe, args := buildCLIArgs(tt.opts)
			if tt.wantExe != "" && exe != tt.wantExe {
				t.Errorf("exe = %q, want %q", exe, tt.wantExe)
			}
			if tt.checkArgs != nil {
				tt.checkArgs(t, args)
			}
		})
	}
}

func TestRunIngestRequiresSkill(t *testing.T) {
	_, err := Run(context.Background(), Options{
		Requirements: "Build something",
		SkillPath:    "/nonexistent/SKILL.md",
		Model:        "claude-sonnet-4-5",
	})
	if err == nil {
		t.Fatal("expected error for missing skill file")
	}
}

func assertContains(t *testing.T, slice []string, want string) {
	t.Helper()
	for _, s := range slice {
		if s == want {
			return
		}
	}
	t.Errorf("args %v does not contain %q", slice, want)
}

func assertNotContains(t *testing.T, slice []string, unwanted string) {
	t.Helper()
	for _, s := range slice {
		if s == unwanted {
			t.Errorf("args %v should not contain %q", slice, unwanted)
			return
		}
	}
}
