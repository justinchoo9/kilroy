package engine

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/strongdm/kilroy/internal/llm"
)

type preflightProbeTestAdapter struct {
	name       string
	completeFn func(ctx context.Context, req llm.Request) (llm.Response, error)
	streamFn   func(ctx context.Context, req llm.Request) (llm.Stream, error)
}

func (a preflightProbeTestAdapter) Name() string { return a.name }

func (a preflightProbeTestAdapter) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	if a.completeFn == nil {
		return llm.Response{}, fmt.Errorf("completeFn is nil")
	}
	return a.completeFn(ctx, req)
}

func (a preflightProbeTestAdapter) Stream(ctx context.Context, req llm.Request) (llm.Stream, error) {
	if a.streamFn == nil {
		return nil, fmt.Errorf("streamFn is nil")
	}
	return a.streamFn(ctx, req)
}

func TestRunProviderAPIPromptProbe_RetriesRequestTimeoutAndSucceeds(t *testing.T) {
	t.Setenv("KILROY_PREFLIGHT_API_PROMPT_PROBE_TIMEOUT_MS", "100")
	t.Setenv("KILROY_PREFLIGHT_API_PROMPT_PROBE_RETRIES", "2")
	t.Setenv("KILROY_PREFLIGHT_API_PROMPT_PROBE_BASE_DELAY_MS", "1")
	t.Setenv("KILROY_PREFLIGHT_API_PROMPT_PROBE_MAX_DELAY_MS", "5")

	var calls atomic.Int32
	client := llm.NewClient()
	client.Register(preflightProbeTestAdapter{
		name: "zai",
		completeFn: func(ctx context.Context, req llm.Request) (llm.Response, error) {
			if calls.Add(1) == 1 {
				return llm.Response{}, llm.NewRequestTimeoutError("zai", "context deadline exceeded")
			}
			return llm.Response{
				Provider: "zai",
				Model:    req.Model,
				Message:  llm.Assistant("OK"),
			}, nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	text, err := runProviderAPIPromptProbe(ctx, client, "zai", "glm-4.7")
	if err != nil {
		t.Fatalf("runProviderAPIPromptProbe: %v", err)
	}
	if text != "OK" {
		t.Fatalf("probe text=%q want %q", text, "OK")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("retry calls=%d want 2", got)
	}
}

func TestRunProviderAPIPromptProbe_DoesNotRetryInvalidRequest(t *testing.T) {
	t.Setenv("KILROY_PREFLIGHT_API_PROMPT_PROBE_TIMEOUT_MS", "100")
	t.Setenv("KILROY_PREFLIGHT_API_PROMPT_PROBE_RETRIES", "3")
	t.Setenv("KILROY_PREFLIGHT_API_PROMPT_PROBE_BASE_DELAY_MS", "1")
	t.Setenv("KILROY_PREFLIGHT_API_PROMPT_PROBE_MAX_DELAY_MS", "5")

	var calls atomic.Int32
	client := llm.NewClient()
	client.Register(preflightProbeTestAdapter{
		name: "zai",
		completeFn: func(ctx context.Context, req llm.Request) (llm.Response, error) {
			calls.Add(1)
			return llm.Response{}, llm.ErrorFromHTTPStatus("zai", 400, "invalid_request_error", nil, nil)
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := runProviderAPIPromptProbe(ctx, client, "zai", "glm-4.7")
	if err == nil {
		t.Fatalf("expected probe error, got nil")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("invalid-request calls=%d want 1", got)
	}
}

func TestRunProviderAPIPromptProbeTarget_StreamTransport_UsesStreamPath(t *testing.T) {
	t.Setenv("KILROY_PREFLIGHT_API_PROMPT_PROBE_TIMEOUT_MS", "100")
	t.Setenv("KILROY_PREFLIGHT_API_PROMPT_PROBE_RETRIES", "0")

	var streamCalls atomic.Int32
	client := llm.NewClient()
	client.Register(preflightProbeTestAdapter{
		name: "zai",
		completeFn: func(ctx context.Context, req llm.Request) (llm.Response, error) {
			return llm.Response{}, fmt.Errorf("complete path should not be used for stream probe")
		},
		streamFn: func(ctx context.Context, req llm.Request) (llm.Stream, error) {
			streamCalls.Add(1)
			s := llm.NewChanStream(func() {})
			go func() {
				defer s.CloseSend()
				s.Send(llm.StreamEvent{Type: llm.StreamEventStreamStart})
				s.Send(llm.StreamEvent{Type: llm.StreamEventTextStart, TextID: "text_1"})
				s.Send(llm.StreamEvent{Type: llm.StreamEventTextDelta, TextID: "text_1", Delta: "OK"})
				resp := llm.Response{
					Provider: "zai",
					Model:    req.Model,
					Message:  llm.Assistant("OK"),
					Finish:   llm.FinishReason{Reason: "stop"},
				}
				s.Send(llm.StreamEvent{Type: llm.StreamEventTextEnd, TextID: "text_1"})
				s.Send(llm.StreamEvent{
					Type:         llm.StreamEventFinish,
					FinishReason: &resp.Finish,
					Response:     &resp,
				})
			}()
			return s, nil
		},
	})

	maxTokens := 16
	target := preflightAPIPromptProbeTarget{
		Provider:  "zai",
		Model:     "glm-4.7",
		Mode:      "one_shot",
		Transport: preflightAPIPromptProbeTransportStream,
		Request: llm.Request{
			Provider:  "zai",
			Model:     "glm-4.7",
			Messages:  []llm.Message{llm.User(preflightPromptProbeText)},
			MaxTokens: &maxTokens,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	text, err := runProviderAPIPromptProbeTarget(ctx, client, target)
	if err != nil {
		t.Fatalf("runProviderAPIPromptProbeTarget(stream): %v", err)
	}
	if text != "OK" {
		t.Fatalf("probe text=%q want %q", text, "OK")
	}
	if got := streamCalls.Load(); got != 1 {
		t.Fatalf("stream calls=%d want 1", got)
	}
}
