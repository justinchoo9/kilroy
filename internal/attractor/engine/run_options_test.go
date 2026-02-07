package engine

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunOptions_ApplyDefaults_DefaultLogsRootUsesXDGStateHomeAndIsOutsideRepo(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	repo := t.TempDir()
	opts := RunOptions{
		RepoPath: repo,
		RunID:    "01HZZZZZZZZZZZZZZZZZZZZZZZZ", // stable, filesystem-safe
	}
	if err := opts.applyDefaults(); err != nil {
		t.Fatalf("applyDefaults: %v", err)
	}

	wantPrefix := filepath.Join(state, "kilroy", "attractor", "runs", opts.RunID)
	if !strings.HasPrefix(opts.LogsRoot, wantPrefix) {
		t.Fatalf("LogsRoot: got %q want prefix %q", opts.LogsRoot, wantPrefix)
	}
	if strings.HasPrefix(opts.LogsRoot, repo+string(filepath.Separator)) || opts.LogsRoot == repo {
		t.Fatalf("LogsRoot should be outside repo: logs_root=%q repo=%q", opts.LogsRoot, repo)
	}
	if opts.WorktreeDir != filepath.Join(opts.LogsRoot, "worktree") {
		t.Fatalf("WorktreeDir: got %q want %q", opts.WorktreeDir, filepath.Join(opts.LogsRoot, "worktree"))
	}
}
