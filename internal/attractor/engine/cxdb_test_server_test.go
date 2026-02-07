package engine

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type cxdbTestServer struct {
	srv *httptest.Server

	mu sync.Mutex

	nextContextID int
	nextTurnID    int

	contexts map[string]*cxdbContextState
	bundles  map[string]any
}

type cxdbContextState struct {
	ContextID  string
	HeadTurnID string
	HeadDepth  int
	Turns      []map[string]any
}

func newCXDBTestServer(t *testing.T) *cxdbTestServer {
	t.Helper()

	s := &cxdbTestServer{
		nextContextID: 1,
		nextTurnID:    1,
		contexts:      map[string]*cxdbContextState{},
		bundles:       map[string]any{},
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
		id := strings.TrimPrefix(r.URL.Path, "/v1/registry/bundles/")
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		var body any
		_ = json.Unmarshal(b, &body)
		s.mu.Lock()
		s.bundles[id] = body
		s.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	})

	mux.HandleFunc("/v1/contexts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			b, _ := io.ReadAll(r.Body)
			_ = r.Body.Close()
			baseTurnID := "0"
			if strings.TrimSpace(string(b)) != "" {
				var req map[string]any
				_ = json.Unmarshal(b, &req)
				if v, ok := req["base_turn_id"]; ok {
					baseTurnID = strings.TrimSpace(anyToString(v))
					if baseTurnID == "" {
						baseTurnID = "0"
					}
				}
			}

			s.mu.Lock()
			id := strconv.Itoa(s.nextContextID)
			s.nextContextID++
			s.contexts[id] = &cxdbContextState{
				ContextID:  id,
				HeadTurnID: baseTurnID,
				HeadDepth:  0,
				Turns:      []map[string]any{},
			}
			ci := s.contexts[id]
			resp := map[string]any{
				"context_id":  ci.ContextID,
				"head_turn_id": ci.HeadTurnID,
				"head_depth":  ci.HeadDepth,
			}
			s.mu.Unlock()

			_ = json.NewEncoder(w).Encode(resp)
		case http.MethodGet:
			s.mu.Lock()
			out := make([]map[string]any, 0, len(s.contexts))
			for _, c := range s.contexts {
				out = append(out, map[string]any{
					"context_id":  c.ContextID,
					"head_turn_id": c.HeadTurnID,
					"head_depth":  c.HeadDepth,
				})
			}
			s.mu.Unlock()
			_ = json.NewEncoder(w).Encode(out)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/contexts/", func(w http.ResponseWriter, r *http.Request) {
		// /v1/contexts/{id} or /v1/contexts/{id}/turns
		rest := strings.TrimPrefix(r.URL.Path, "/v1/contexts/")
		parts := strings.Split(rest, "/")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		ctxID := parts[0]
		if len(parts) == 1 {
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			s.mu.Lock()
			ci := s.contexts[ctxID]
			resp := map[string]any(nil)
			if ci != nil {
				resp = map[string]any{
					"context_id":  ci.ContextID,
					"head_turn_id": ci.HeadTurnID,
					"head_depth":  ci.HeadDepth,
				}
			}
			s.mu.Unlock()
			if resp == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if len(parts) == 2 && parts[1] == "turns" {
			switch r.Method {
			case http.MethodPost:
				b, _ := io.ReadAll(r.Body)
				_ = r.Body.Close()
				var req map[string]any
				_ = json.Unmarshal(b, &req)
				typeID := strings.TrimSpace(anyToString(req["type_id"]))
				typeVer := anyToInt(req["type_version"])
				parent := strings.TrimSpace(anyToString(req["parent_turn_id"]))
				payload, _ := req["payload"].(map[string]any)
				if payload == nil {
					payload = map[string]any{}
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
				m := map[string]any{
					"turn_id":        turnID,
					"parent_turn_id": parent,
					"depth":          depth,
					"type_id":        typeID,
					"type_version":   typeVer,
					"payload":        payload,
					"payload_hash":   "h" + turnID,
				}
				ci.Turns = append(ci.Turns, m)
				s.mu.Unlock()

				_ = json.NewEncoder(w).Encode(map[string]any{
					"context_id":   ctxID,
					"turn_id":      turnID,
					"depth":        depth,
					"payload_hash": "h" + turnID,
				})
			case http.MethodGet:
				s.mu.Lock()
				ci := s.contexts[ctxID]
				var out []map[string]any
				if ci != nil {
					out = append([]map[string]any{}, ci.Turns...)
				}
				s.mu.Unlock()
				_ = json.NewEncoder(w).Encode(out)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

func (s *cxdbTestServer) URL() string { return s.srv.URL }

func (s *cxdbTestServer) Turns(contextID string) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	ci := s.contexts[contextID]
	if ci == nil {
		return nil
	}
	return append([]map[string]any{}, ci.Turns...)
}

func (s *cxdbTestServer) ContextIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.contexts))
	for id := range s.contexts {
		out = append(out, id)
	}
	return out
}

func anyToString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

func anyToInt(v any) int {
	switch x := v.(type) {
	case nil:
		return 0
	case int:
		return x
	case float64:
		return int(x)
	default:
		return 0
	}
}
