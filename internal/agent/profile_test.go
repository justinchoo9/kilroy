package agent

import (
	"strings"
	"testing"
)

func TestProviderProfiles_ToolsetsAndDocSelection(t *testing.T) {
	openai := NewOpenAIProfile("gpt-5.2")
	if openai.ID() != "openai" {
		t.Fatalf("openai id: %q", openai.ID())
	}
		if openai.SupportsParallelToolCalls() {
			t.Fatalf("openai should not support parallel tool calls by default")
		}
		if got := strings.Join(openai.ProjectDocFiles(), ","); got != "AGENTS.md,.codex/instructions.md" {
			t.Fatalf("openai docs: %q", got)
		}
	assertHasTool(t, openai, "apply_patch")
	assertMissingTool(t, openai, "edit_file")

	anthropic := NewAnthropicProfile("claude-test")
	if anthropic.ID() != "anthropic" {
		t.Fatalf("anthropic id: %q", anthropic.ID())
	}
	if !anthropic.SupportsParallelToolCalls() {
		t.Fatalf("anthropic should support parallel tool calls")
	}
	assertHasTool(t, anthropic, "edit_file")
	assertMissingTool(t, anthropic, "apply_patch")

	gemini := NewGeminiProfile("gemini-test")
	if gemini.ID() != "google" {
		t.Fatalf("gemini id: %q", gemini.ID())
	}
	if !gemini.SupportsParallelToolCalls() {
		t.Fatalf("gemini should support parallel tool calls")
	}
	assertHasTool(t, gemini, "edit_file")
	assertHasTool(t, gemini, "read_many_files")
	assertHasTool(t, gemini, "list_dir")
	assertMissingTool(t, gemini, "apply_patch")
}

func TestProviderProfiles_BuildSystemPrompt_IncludesProviderSpecificBaseInstructions(t *testing.T) {
	env := EnvironmentInfo{
		WorkingDir:      "/tmp",
		Platform:        "linux",
		OSVersion:       "test",
		Today:           "2026-02-07",
		KnowledgeCutoff: "2024-06-01",
	}

	openai := NewOpenAIProfile("gpt-5.2")
	sysO := openai.BuildSystemPrompt(env, nil)
	if !strings.Contains(sysO, "OpenAI profile") || !strings.Contains(sysO, "apply_patch") {
		t.Fatalf("openai system prompt missing expected base instructions:\n%s", sysO)
	}
	if strings.Contains(sysO, "edit_file") {
		t.Fatalf("openai system prompt should not focus on edit_file:\n%s", sysO)
	}

	anthropic := NewAnthropicProfile("claude-test")
	sysA := anthropic.BuildSystemPrompt(env, nil)
	if !strings.Contains(sysA, "Anthropic profile") || !strings.Contains(sysA, "edit_file") {
		t.Fatalf("anthropic system prompt missing expected base instructions:\n%s", sysA)
	}
	if strings.Contains(sysA, "apply_patch") {
		t.Fatalf("anthropic system prompt should not focus on apply_patch:\n%s", sysA)
	}

	gemini := NewGeminiProfile("gemini-test")
	sysG := gemini.BuildSystemPrompt(env, nil)
	if !strings.Contains(sysG, "Gemini profile") || !strings.Contains(sysG, "edit_file") {
		t.Fatalf("gemini system prompt missing expected base instructions:\n%s", sysG)
	}
}

func assertHasTool(t *testing.T, p ProviderProfile, name string) {
	t.Helper()
	for _, td := range p.ToolDefinitions() {
		if td.Name == name {
			return
		}
	}
	t.Fatalf("expected tool %q in profile %q tool defs", name, p.ID())
}

func assertMissingTool(t *testing.T, p ProviderProfile, name string) {
	t.Helper()
	for _, td := range p.ToolDefinitions() {
		if td.Name == name {
			t.Fatalf("did not expect tool %q in profile %q tool defs", name, p.ID())
		}
	}
}
