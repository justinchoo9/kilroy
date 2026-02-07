package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRun_LoopRestartEdgeErrorsInV1(t *testing.T) {
	repo := t.TempDir()
	runCmd(t, repo, "git", "init")
	runCmd(t, repo, "git", "config", "user.name", "tester")
	runCmd(t, repo, "git", "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644)
	runCmd(t, repo, "git", "add", "-A")
	runCmd(t, repo, "git", "commit", "-m", "init")

	dot := []byte(`
digraph G {
  graph [goal="test"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="hi"]
  start -> a
  a -> exit [loop_restart=true]
}
`)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := Run(ctx, dot, RunOptions{RepoPath: repo})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

