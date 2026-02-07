package modeldb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveLiteLLMCatalog_PinnedOnly(t *testing.T) {
	dir := t.TempDir()
	pinned := filepath.Join(dir, "pinned.json")
	if err := os.WriteFile(pinned, []byte(`{"m":{"litellm_provider":"openai","mode":"chat"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := ResolveLiteLLMCatalog(context.Background(), pinned, dir, CatalogPinnedOnly, "", 0)
	if err != nil {
		t.Fatalf("ResolveLiteLLMCatalog: %v", err)
	}
	if res.SnapshotPath == "" || res.SHA256 == "" {
		t.Fatalf("missing snapshot metadata: %+v", res)
	}
	if res.Source != pinned {
		t.Fatalf("source: got %q want %q", res.Source, pinned)
	}
}

func TestResolveLiteLLMCatalog_OnRunStartFetch(t *testing.T) {
	dir := t.TempDir()
	pinned := filepath.Join(dir, "pinned.json")
	if err := os.WriteFile(pinned, []byte(`{"m":{"litellm_provider":"openai","mode":"chat"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"m2":{"litellm_provider":"anthropic","mode":"chat"}}`))
	}))
	defer srv.Close()

	res, err := ResolveLiteLLMCatalog(context.Background(), pinned, dir, CatalogOnRunStart, srv.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("ResolveLiteLLMCatalog: %v", err)
	}
	if res.Source != srv.URL {
		t.Fatalf("source: got %q want %q", res.Source, srv.URL)
	}
}
