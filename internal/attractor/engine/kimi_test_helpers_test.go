package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func decodeJSONBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	_ = r.Body.Close()
	out := map[string]any{}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode request body as json: %v\nbody=%s", err, strings.TrimSpace(string(body)))
	}
	return out
}

func isKimiCodingContractRequest(body map[string]any) bool {
	stream, _ := body["stream"].(bool)
	return stream && asInt(body["max_tokens"]) >= 16000
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	default:
		return 0
	}
}

func writeAnthropicStreamOK(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/event-stream")
	f, _ := w.(http.Flusher)
	write := func(event string, data string) {
		_, _ = io.WriteString(w, "event: "+event+"\n")
		_, _ = io.WriteString(w, "data: "+data+"\n\n")
		if f != nil {
			f.Flush()
		}
	}
	write("content_block_start", `{"content_block":{"type":"text"}}`)
	write("content_block_delta", fmt.Sprintf(`{"delta":{"type":"text_delta","text":%q}}`, text))
	write("content_block_stop", `{}`)
	write("message_delta", `{"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
	write("message_stop", `{}`)
}
