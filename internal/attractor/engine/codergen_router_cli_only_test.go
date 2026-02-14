package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/danshapiro/kilroy/internal/attractor/model"
)

func TestCLIOnlyModelOverride_SwitchesBackendAndWarns(t *testing.T) {
	// Set up router with openai configured as API backend.
	runtimes := map[string]ProviderRuntime{
		"openai": {Key: "openai", Backend: BackendAPI},
	}
	router := NewCodergenRouterWithRuntimes(nil, nil, runtimes)

	// Confirm baseline: openai backend is API.
	if got := router.backendForProvider("openai"); got != BackendAPI {
		t.Fatalf("backendForProvider(openai) = %q, want %q", got, BackendAPI)
	}

	// Create a node using the CLI-only model.
	node := model.NewNode("spark-test")
	node.Attrs["llm_provider"] = "openai"
	node.Attrs["llm_model"] = "gpt-5.3-codex-spark"
	node.Attrs["shape"] = "box"

	// Create a minimal execution with an Engine to capture warnings.
	eng := &Engine{}
	exec := &Execution{Engine: eng}

	// Run will likely fail (no real CLI binary), but the override should fire
	// first. We don't check the error — runCLI may return a failure outcome
	// instead of an error.
	_, _, _ = router.Run(context.Background(), exec, node, "test prompt")

	// Verify the CLI-only override warning was emitted.
	found := false
	for _, w := range eng.Warnings {
		if strings.Contains(w, "cli-only model override") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning containing 'cli-only model override', got warnings: %v", eng.Warnings)
	}
}

func TestCLIOnlyModelOverride_RegularModelNoOverride(t *testing.T) {
	// Set up router with openai configured as API backend.
	runtimes := map[string]ProviderRuntime{
		"openai": {Key: "openai", Backend: BackendAPI},
	}
	router := NewCodergenRouterWithRuntimes(nil, nil, runtimes)

	// Create a node using a regular (non-CLI-only) model.
	node := model.NewNode("regular-test")
	node.Attrs["llm_provider"] = "openai"
	node.Attrs["llm_model"] = "gpt-5.3-codex"
	node.Attrs["shape"] = "box"

	eng := &Engine{}
	exec := &Execution{Engine: eng}

	// Run will fail (no API client), but no CLI-only override should fire.
	_, _, _ = router.Run(context.Background(), exec, node, "test prompt")

	for _, w := range eng.Warnings {
		if strings.Contains(w, "cli-only model override") {
			t.Errorf("unexpected CLI-only override warning for regular model: %s", w)
		}
	}
}
