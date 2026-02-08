package model

import "testing"

func TestNode_Prompt_FallsBackToLLMPrompt(t *testing.T) {
	n := NewNode("test")
	n.Attrs["llm_prompt"] = "Do the thing"

	if got := n.Prompt(); got != "Do the thing" {
		t.Errorf("Prompt() = %q, want %q", got, "Do the thing")
	}
}

func TestNode_Prompt_PrefersPromptOverLLMPrompt(t *testing.T) {
	n := NewNode("test")
	n.Attrs["prompt"] = "Canonical prompt"
	n.Attrs["llm_prompt"] = "Alternate prompt"

	if got := n.Prompt(); got != "Canonical prompt" {
		t.Errorf("Prompt() = %q, want %q", got, "Canonical prompt")
	}
}

func TestNode_Prompt_EmptyWhenNeitherSet(t *testing.T) {
	n := NewNode("test")
	if got := n.Prompt(); got != "" {
		t.Errorf("Prompt() = %q, want empty", got)
	}
}
