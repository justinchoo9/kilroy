package engine

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/strongdm/kilroy/internal/cxdb"
)

// CXDBSink appends normalized Attractor events to a CXDB context via the HTTP API.
//
// v1 implementation notes:
// - Uses the HTTP JSON append endpoint for simplicity.
// - Serializes appends to maintain a linear head within a context.
type CXDBSink struct {
	Client *cxdb.Client

	RunID      string
	ContextID  string
	HeadTurnID string
	BundleID   string

	mu sync.Mutex
}

func NewCXDBSink(client *cxdb.Client, runID, contextID, headTurnID, bundleID string) *CXDBSink {
	return &CXDBSink{
		Client:     client,
		RunID:      runID,
		ContextID:  contextID,
		HeadTurnID: headTurnID,
		BundleID:   bundleID,
	}
}

func (s *CXDBSink) Append(ctx context.Context, typeID string, typeVersion int, data map[string]any) (turnID string, contentHash string, err error) {
	if s == nil || s.Client == nil {
		return "", "", fmt.Errorf("cxdb sink is nil")
	}
	if data == nil {
		data = map[string]any{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	resp, err := s.Client.AppendTurn(ctx, s.ContextID, cxdb.AppendTurnRequest{
		TypeID:       typeID,
		TypeVersion:  typeVersion,
		Payload:      data,
		ParentTurnID: s.HeadTurnID,
	})
	if err != nil {
		return "", "", err
	}
	s.HeadTurnID = resp.TurnID
	return resp.TurnID, resp.ContentHash, nil
}

func (s *CXDBSink) ForkFromHead(ctx context.Context) (*CXDBSink, error) {
	if s == nil || s.Client == nil {
		return nil, fmt.Errorf("cxdb sink is nil")
	}
	base := s.HeadTurnID
	if strings.TrimSpace(base) == "" {
		base = "0"
	}
	ci, err := s.Client.ForkContext(ctx, base)
	if err != nil {
		return nil, err
	}
	return NewCXDBSink(s.Client, s.RunID, ci.ContextID, ci.HeadTurnID, s.BundleID), nil
}

func (s *CXDBSink) PutArtifactFile(ctx context.Context, nodeID, logicalName, path string) (artifactTurnID string, err error) {
	if s == nil {
		return "", fmt.Errorf("cxdb sink is nil")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if mimeType == "" {
		// best-effort fallbacks
		switch strings.ToLower(filepath.Ext(path)) {
		case ".md":
			mimeType = "text/markdown"
		case ".json":
			mimeType = "application/json"
		case ".ndjson":
			mimeType = "application/x-ndjson"
		case ".tgz", ".tar.gz":
			mimeType = "application/gzip"
		default:
			mimeType = "application/octet-stream"
		}
	}

	_, blobHash, err := s.Append(ctx, "com.kilroy.attractor.Blob", 1, map[string]any{
		"bytes": base64.StdEncoding.EncodeToString(b),
	})
	if err != nil {
		return "", err
	}
	turnID, _, err := s.Append(ctx, "com.kilroy.attractor.Artifact", 1, map[string]any{
		"run_id":       s.RunID,
		"node_id":      nodeID,
		"name":         logicalName,
		"mime":         mimeType,
		"content_hash": blobHash,
		"bytes_len":    uint64(len(b)),
		"local_path":   path,
	})
	if err != nil {
		return "", err
	}
	return turnID, nil
}

func nowMS() uint64 { return uint64(time.Now().UTC().UnixNano() / int64(time.Millisecond)) }
