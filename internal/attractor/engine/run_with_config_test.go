package engine

import (
	"context"
	"testing"
	"time"
)

func TestRunWithConfig_FailsFastWhenProviderBackendMissing(t *testing.T) {
	dot := []byte(`
digraph G {
  graph [goal="test"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="hi"]
  start -> a -> exit
}
`)
	cfg := &RunConfigFile{}
	cfg.Version = 1
	cfg.Repo.Path = "/tmp/repo"
	cfg.CXDB.BinaryAddr = "127.0.0.1:9009"
	cfg.CXDB.HTTPBaseURL = "http://127.0.0.1:9010"
	cfg.ModelDB.LiteLLMCatalogPath = "/tmp/catalog.json"
	// Intentionally omit llm.providers.openai.backend

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := RunWithConfig(ctx, dot, cfg, RunOptions{RunID: "r1", LogsRoot: t.TempDir()})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

