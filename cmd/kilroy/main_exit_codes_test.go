package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type cxdbTestServer struct {
	srv *httptest.Server

	mu sync.Mutex

	nextContextID int
	nextTurnID    int
	contexts      map[string]*cxdbContextState
}

type cxdbContextState struct {
	ContextID  string
	HeadTurnID string
	HeadDepth  int
}

func newCXDBTestServer(t *testing.T) *cxdbTestServer {
	t.Helper()

	s := &cxdbTestServer{
		nextContextID: 1,
		nextTurnID:    1,
		contexts:      map[string]*cxdbContextState{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/v1/registry/bundles/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/v1/contexts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		baseTurnID := "0"
		b, _ := ioReadAll(r.Body)
		_ = r.Body.Close()
		if strings.TrimSpace(string(b)) != "" {
			var req map[string]any
			_ = json.Unmarshal(b, &req)
			if v, ok := req["base_turn_id"]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					baseTurnID = strings.TrimSpace(s)
				}
			}
		}

		s.mu.Lock()
		id := strconv.Itoa(s.nextContextID)
		s.nextContextID++
		s.contexts[id] = &cxdbContextState{ContextID: id, HeadTurnID: baseTurnID, HeadDepth: 0}
		ci := *s.contexts[id]
		s.mu.Unlock()

		_ = json.NewEncoder(w).Encode(map[string]any{
			"context_id":   ci.ContextID,
			"head_turn_id": ci.HeadTurnID,
			"head_depth":   ci.HeadDepth,
		})
	})
	mux.HandleFunc("/v1/contexts/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/v1/contexts/")
		parts := strings.Split(rest, "/")
		if len(parts) < 2 || parts[1] != "turns" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		ctxID := strings.TrimSpace(parts[0])
		if ctxID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		s.mu.Lock()
		ci := s.contexts[ctxID]
		if ci == nil {
			s.mu.Unlock()
			w.WriteHeader(http.StatusNotFound)
			return
		}
		turnID := strconv.Itoa(s.nextTurnID)
		s.nextTurnID++
		ci.HeadDepth++
		ci.HeadTurnID = turnID
		depth := ci.HeadDepth
		s.mu.Unlock()

		_ = json.NewEncoder(w).Encode(map[string]any{
			"context_id":   ctxID,
			"turn_id":      turnID,
			"depth":        depth,
			"payload_hash": "h" + turnID,
			"content_hash": "h" + turnID,
		})
	})

	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

func (s *cxdbTestServer) URL() string { return s.srv.URL }

func ioReadAll(r io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	return buf.Bytes(), err
}

func buildKilroyBinary(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	// wd is .../cmd/kilroy
	root := filepath.Dir(filepath.Dir(wd))
	bin := filepath.Join(t.TempDir(), "kilroy")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/kilroy")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, string(out))
	}
	return bin
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s failed: %v\n%s", strings.Join(args, " "), err, string(out))
		}
	}
	run("git", "init")
	run("git", "config", "user.name", "tester")
	run("git", "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644)
	run("git", "add", "-A")
	run("git", "commit", "-m", "init")
	return repo
}

func writePinnedCatalog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model_prices_and_context_window.json")
	// Minimal LiteLLM catalog shape (object map).
	_ = os.WriteFile(path, []byte(`{
  "gpt-5.2": {
    "litellm_provider": "openai",
    "mode": "chat",
    "max_input_tokens": 1000,
    "max_output_tokens": 1000
  }
}`), 0o644)
	return path
}

func writeRunConfig(t *testing.T, repo string, cxdbURL string, catalogPath string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "run.yaml")
	b := []byte("version: 1\n" +
		"repo:\n" +
		"  path: " + repo + "\n" +
		"cxdb:\n" +
		"  binary_addr: 127.0.0.1:9009\n" +
		"  http_base_url: " + cxdbURL + "\n" +
		"modeldb:\n" +
		"  litellm_catalog_path: " + catalogPath + "\n" +
		"  litellm_catalog_update_policy: pinned\n")
	_ = os.WriteFile(path, b, 0o644)
	return path
}

func runKilroy(t *testing.T, bin string, args ...string) (exitCode int, stdoutStderr string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("kilroy timed out\n%s", string(out))
	}
	if err == nil {
		return 0, string(out)
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("kilroy failed: %v\n%s", err, string(out))
	}
	return ee.ExitCode(), string(out)
}

func TestKilroyAttractorExitCodes(t *testing.T) {
	cxdbSrv := newCXDBTestServer(t)
	bin := buildKilroyBinary(t)
	repo := initTestRepo(t)
	catalog := writePinnedCatalog(t)
	cfg := writeRunConfig(t, repo, cxdbSrv.URL(), catalog)

	// Success -> exit code 0.
	successGraph := filepath.Join(t.TempDir(), "success.dot")
	_ = os.WriteFile(successGraph, []byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  start -> exit
}
`), 0o644)
	logsRoot1 := filepath.Join(t.TempDir(), "logs-success")
	code, out := runKilroy(t, bin, "attractor", "run", "--graph", successGraph, "--config", cfg, "--run-id", "cli-success", "--logs-root", logsRoot1)
	if code != 0 {
		t.Fatalf("success exit code: got %d want 0\n%s", code, out)
	}

	// Failure -> exit code 1.
	failGraph := filepath.Join(t.TempDir(), "fail.dot")
	_ = os.WriteFile(failGraph, []byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  t [shape=parallelogram, tool_command="exit 1"]
  start -> t -> exit [condition="outcome=success"]
}
`), 0o644)
	logsRoot2 := filepath.Join(t.TempDir(), "logs-fail")
	code, out = runKilroy(t, bin, "attractor", "run", "--graph", failGraph, "--config", cfg, "--run-id", "cli-fail", "--logs-root", logsRoot2)
	if code != 1 {
		t.Fatalf("fail exit code: got %d want 1\n%s", code, out)
	}
}
