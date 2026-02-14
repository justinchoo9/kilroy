package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLaunchDetached_SetsCmdDirToLogsRoot(t *testing.T) {
	logsRoot := t.TempDir()
	cwdPath := filepath.Join(logsRoot, "cwd.txt")

	oldExec := detachedExecCommand
	t.Cleanup(func() { detachedExecCommand = oldExec })
	detachedExecCommand = func(name string, args ...string) *exec.Cmd {
		_ = name
		_ = args
		return exec.Command("bash", "-c", fmt.Sprintf("pwd > %q", cwdPath))
	}

	if err := launchDetached([]string{"attractor", "run"}, logsRoot); err != nil {
		t.Fatalf("launchDetached: %v", err)
	}

	waitForFile(t, cwdPath, 5*time.Second)
	gotRaw, err := os.ReadFile(cwdPath)
	if err != nil {
		t.Fatalf("read cwd file: %v", err)
	}
	got := strings.TrimSpace(string(gotRaw))
	if got != logsRoot {
		t.Fatalf("child cwd mismatch: got %q want %q", got, logsRoot)
	}
}
