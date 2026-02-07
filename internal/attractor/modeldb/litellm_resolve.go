package modeldb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CatalogUpdatePolicy string

const (
	CatalogPinnedOnly CatalogUpdatePolicy = "pinned"
	CatalogOnRunStart CatalogUpdatePolicy = "on_run_start"
)

type ResolvedCatalog struct {
	SnapshotPath string
	Source       string
	SHA256       string
	Warning      string
}

// ResolveLiteLLMCatalog snapshots the effective LiteLLM catalog into the run logs directory
// (for repeatability) and returns metadata about the snapshot.
//
// Policy:
// - pinned: copy the pinned file to {logs_root}/modeldb/litellm_catalog.json
// - on_run_start: attempt to fetch latest from url; on failure, warn and fall back to pinned
func ResolveLiteLLMCatalog(ctx context.Context, pinnedPath string, logsRoot string, policy CatalogUpdatePolicy, url string, timeout time.Duration) (*ResolvedCatalog, error) {
	if strings.TrimSpace(pinnedPath) == "" {
		return nil, fmt.Errorf("pinnedPath is required")
	}
	if strings.TrimSpace(logsRoot) == "" {
		return nil, fmt.Errorf("logsRoot is required")
	}
	if policy == "" {
		policy = CatalogOnRunStart
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if strings.TrimSpace(url) == "" {
		url = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	}

	dstDir := filepath.Join(logsRoot, "modeldb")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return nil, err
	}
	dstPath := filepath.Join(dstDir, "litellm_catalog.json")

	source := pinnedPath
	warn := ""
	// Best-effort pinned hash for "effective differs from pinned" warning (metaspec).
	pinnedSHA := ""
	if pb, err := os.ReadFile(pinnedPath); err == nil {
		sum := sha256.Sum256(pb)
		pinnedSHA = hex.EncodeToString(sum[:])
	}

	switch policy {
	case CatalogPinnedOnly:
		if err := copyFile(dstPath, pinnedPath); err != nil {
			return nil, err
		}
	case CatalogOnRunStart:
		b, fetchErr := fetchBytes(ctx, url, timeout)
		if fetchErr == nil && len(b) > 0 {
			if err := os.WriteFile(dstPath, b, 0o644); err != nil {
				return nil, err
			}
			source = url
		} else {
			warn = fmt.Sprintf("modeldb: fetch failed (%v); falling back to pinned snapshot", fetchErr)
			if err := copyFile(dstPath, pinnedPath); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("invalid catalog update policy: %q", policy)
	}

	b, err := os.ReadFile(dstPath)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(b)
	sha := hex.EncodeToString(sum[:])
	if warn == "" && policy == CatalogOnRunStart && pinnedSHA != "" && sha != pinnedSHA {
		warn = fmt.Sprintf("modeldb: effective catalog differs from pinned snapshot (pinned_sha256=%s effective_sha256=%s)", pinnedSHA, sha)
	}
	return &ResolvedCatalog{
		SnapshotPath: dstPath,
		Source:       source,
		SHA256:       sha,
		Warning:      warn,
	}, nil
}

func fetchBytes(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return io.ReadAll(resp.Body)
}

func copyFile(dst, src string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}
