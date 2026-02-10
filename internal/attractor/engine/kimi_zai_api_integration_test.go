package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestKimiCodingAndZai_APIIntegration(t *testing.T) {
	repo := initTestRepo(t)
	logsRoot := t.TempDir()
	pinned := writeProviderCatalogForTest(t)
	cxdbSrv := newCXDBTestServer(t)

	var mu sync.Mutex
	seenPaths := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenPaths[r.URL.Path]++
		mu.Unlock()
		switch r.URL.Path {
		case "/coding/v1/messages":
			body := decodeJSONBody(t, r)
			if !isKimiCodingContractRequest(body) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"kimi requires stream=true and max_tokens>=16000"}}`))
				return
			}
			writeAnthropicStreamOK(w, "ok")
		case "/api/coding/paas/v4/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"x","model":"m","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	runCase := func(provider, model, keyEnv, baseURL string) {
		t.Helper()
		cfg := &RunConfigFile{Version: 1}
		cfg.Repo.Path = repo
		cfg.CXDB.BinaryAddr = cxdbSrv.BinaryAddr()
		cfg.CXDB.HTTPBaseURL = cxdbSrv.URL()
		cfg.ModelDB.OpenRouterModelInfoPath = pinned
		cfg.ModelDB.OpenRouterModelInfoUpdatePolicy = "pinned"
		cfg.Git.RunBranchPrefix = "attractor/run"
		cfg.LLM.Providers = map[string]ProviderConfig{
			provider: {
				Backend: BackendAPI,
				API: ProviderAPIConfig{
					APIKeyEnv: keyEnv,
					BaseURL:   baseURL,
				},
			},
		}
		t.Setenv(keyEnv, "k")

		dot := []byte(fmt.Sprintf(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=%s, llm_model=%s, codergen_mode=one_shot, auto_status=true, prompt="say hi"]
  start -> a -> exit
}
`, provider, model))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := RunWithConfig(ctx, dot, cfg, RunOptions{RunID: "kz-" + provider, LogsRoot: logsRoot})
		if err != nil {
			t.Fatalf("%s run failed: %v", provider, err)
		}
	}

	runCase("kimi", "kimi-k2.5", "KIMI_API_KEY", srv.URL+"/coding")
	runCase("zai", "glm-4.7", "ZAI_API_KEY", srv.URL)

	mu.Lock()
	defer mu.Unlock()
	if seenPaths["/coding/v1/messages"] == 0 {
		t.Fatalf("missing kimi coding messages call: %v", seenPaths)
	}
	if seenPaths["/api/coding/paas/v4/chat/completions"] == 0 {
		t.Fatalf("missing zai chat-completions call: %v", seenPaths)
	}
}

func TestKimiAgentLoop_UsesNativeKimiProviderRouting(t *testing.T) {
	repo := initTestRepo(t)
	logsRoot := t.TempDir()
	pinned := writeProviderCatalogForTest(t)
	cxdbSrv := newCXDBTestServer(t)

	var mu sync.Mutex
	seenPaths := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenPaths[r.URL.Path]++
		mu.Unlock()

		switch r.URL.Path {
		case "/coding/v1/messages":
			body := decodeJSONBody(t, r)
			if !isKimiCodingContractRequest(body) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"kimi requires stream=true and max_tokens>=16000"}}`))
				return
			}
			writeAnthropicStreamOK(w, "ok")
		case "/v1/responses":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","code":"model_not_found","message":"The requested model 'kimi-k2.5' does not exist.","param":"model"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	cfg := &RunConfigFile{Version: 1}
	cfg.Repo.Path = repo
	cfg.CXDB.BinaryAddr = cxdbSrv.BinaryAddr()
	cfg.CXDB.HTTPBaseURL = cxdbSrv.URL()
	cfg.ModelDB.OpenRouterModelInfoPath = pinned
	cfg.ModelDB.OpenRouterModelInfoUpdatePolicy = "pinned"
	cfg.Git.RunBranchPrefix = "attractor/run"
	cfg.LLM.Providers = map[string]ProviderConfig{
		"kimi": {
			Backend: BackendAPI,
			API: ProviderAPIConfig{
				APIKeyEnv: "KIMI_API_KEY",
				BaseURL:   srv.URL + "/coding",
			},
		},
	}
	cfg.LLM.CLIProfile = "real"

	t.Setenv("KIMI_API_KEY", "k")
	// Also configure OpenAI so this test catches accidental profile-family routing.
	t.Setenv("OPENAI_API_KEY", "openai-k")
	t.Setenv("OPENAI_BASE_URL", srv.URL)

	dot := []byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=kimi, llm_model=kimi-k2.5, codergen_mode=agent_loop, auto_status=true, prompt="say hi"]
  start -> a -> exit
}
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := RunWithConfig(ctx, dot, cfg, RunOptions{RunID: "kz-kimi-agent-loop", LogsRoot: logsRoot}); err != nil {
		t.Fatalf("kimi agent_loop run failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if seenPaths["/coding/v1/messages"] == 0 {
		t.Fatalf("missing kimi coding messages call: %v", seenPaths)
	}
	if seenPaths["/v1/responses"] != 0 {
		t.Fatalf("unexpected openai responses call for kimi agent_loop: %v", seenPaths)
	}
}

func TestKimiCoding_APIIntegration_EnforcesStreamingAndMinMaxTokensContract(t *testing.T) {
	repo := initTestRepo(t)
	logsRoot := t.TempDir()
	pinned := writeProviderCatalogForTest(t)
	cxdbSrv := newCXDBTestServer(t)

	var seenContract bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/coding/v1/messages" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body := decodeJSONBody(t, r)
		seenContract = isKimiCodingContractRequest(body)
		if !seenContract {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"kimi requires stream=true and max_tokens>=16000"}}`))
			return
		}
		writeAnthropicStreamOK(w, "ok")
	}))
	defer srv.Close()

	cfg := &RunConfigFile{Version: 1}
	cfg.Repo.Path = repo
	cfg.CXDB.BinaryAddr = cxdbSrv.BinaryAddr()
	cfg.CXDB.HTTPBaseURL = cxdbSrv.URL()
	cfg.ModelDB.OpenRouterModelInfoPath = pinned
	cfg.ModelDB.OpenRouterModelInfoUpdatePolicy = "pinned"
	cfg.Git.RunBranchPrefix = "attractor/run"
	cfg.LLM.Providers = map[string]ProviderConfig{
		"kimi": {
			Backend: BackendAPI,
			API: ProviderAPIConfig{
				APIKeyEnv: "KIMI_API_KEY",
				BaseURL:   srv.URL + "/coding",
			},
		},
	}
	t.Setenv("KIMI_API_KEY", "k")

	dot := []byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider="kimi", llm_model="kimi-k2.5", codergen_mode=one_shot, auto_status=true, prompt="say hi"]
  start -> a -> exit
}
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := RunWithConfig(ctx, dot, cfg, RunOptions{RunID: "kz-kimi-contract", LogsRoot: logsRoot}); err != nil {
		t.Fatalf("kimi contract run failed: %v", err)
	}
	if !seenContract {
		t.Fatalf("expected kimi request to enforce stream=true and max_tokens>=16000")
	}
}
