package engine

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/strongdm/kilroy/internal/attractor/runtime"
)

func TestRunWithConfig_CLIBackend_OpenAIIdleTimeoutKillsProcessGroup(t *testing.T) {
	repo := initTestRepo(t)
	logsRoot := t.TempDir()

	pinned := writePinnedCatalog(t)
	cxdbSrv := newCXDBTestServer(t)

	cli := filepath.Join(t.TempDir(), "codex")
	childPIDFile := filepath.Join(t.TempDir(), "watchdog-child.pid")
	if err := os.WriteFile(cli, []byte(`#!/usr/bin/env bash
set -euo pipefail

pidfile="${KILROY_WATCHDOG_CHILD_PID_FILE:?missing pidfile}"
bash -c 'while true; do sleep 1; done' &
child="$!"
echo "$child" > "$pidfile"
echo "codex started" >&2

while true; do
  sleep 60
done
`), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KILROY_CODEX_PATH", cli)
	t.Setenv("KILROY_WATCHDOG_CHILD_PID_FILE", childPIDFile)
	t.Setenv("KILROY_CODEX_IDLE_TIMEOUT", "2s")
	t.Setenv("KILROY_CODEX_KILL_GRACE", "200ms")

	cfg := &RunConfigFile{Version: 1}
	cfg.Repo.Path = repo
	cfg.CXDB.BinaryAddr = cxdbSrv.BinaryAddr()
	cfg.CXDB.HTTPBaseURL = cxdbSrv.URL()
	cfg.LLM.Providers = map[string]struct {
		Backend BackendKind `json:"backend" yaml:"backend"`
	}{
		"openai": {Backend: BackendCLI},
	}
	cfg.ModelDB.LiteLLMCatalogPath = pinned
	cfg.ModelDB.LiteLLMCatalogUpdatePolicy = "pinned"
	cfg.Git.RunBranchPrefix = "attractor/run"

	dot := []byte(`
digraph G {
  graph [goal="test idle timeout watchdog"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="say hi"]
  start -> a -> exit
}
`)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	res, err := RunWithConfig(ctx, dot, cfg, RunOptions{RunID: "watchdog-timeout", LogsRoot: logsRoot})
	if err != nil {
		t.Fatalf("RunWithConfig: %v", err)
	}

	statusBytes, err := os.ReadFile(filepath.Join(res.LogsRoot, "a", "status.json"))
	if err != nil {
		t.Fatalf("read a/status.json: %v", err)
	}
	outcome, err := runtime.DecodeOutcomeJSON(statusBytes)
	if err != nil {
		t.Fatalf("decode a/status.json: %v", err)
	}
	if outcome.Status != runtime.StatusFail {
		t.Fatalf("a status: got %q want %q (out=%+v)", outcome.Status, runtime.StatusFail, outcome)
	}
	if !strings.Contains(strings.ToLower(outcome.FailureReason), "idle timeout") {
		t.Fatalf("a failure_reason: got %q want idle timeout", outcome.FailureReason)
	}

	pidBytes, err := os.ReadFile(childPIDFile)
	if err != nil {
		t.Fatalf("read child pid file: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		t.Fatalf("parse child pid: %v (raw=%q)", err, string(pidBytes))
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("watchdog child process pid=%d still exists", pid)
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
