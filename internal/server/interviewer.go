package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/strongdm/kilroy/internal/attractor/engine"
)

// WebInterviewer satisfies engine.Interviewer by parking questions until an
// HTTP client answers them. The engine goroutine blocks on Ask() until an
// answer is posted via Answer() or the timeout expires.
//
// There is at most one pending question at a time since the engine is
// single-threaded per pipeline (runLoop calls Interviewer.Ask synchronously).
type WebInterviewer struct {
	mu       sync.Mutex
	pending  *pendingQuestion
	timeout  time.Duration
	qidSeq   uint64
	cancelCh chan struct{}
}

type pendingQuestion struct {
	ID       string
	Question engine.Question
	AskedAt  time.Time
	answerCh chan engine.Answer
}

// NewWebInterviewer creates a new WebInterviewer with the given timeout.
// If timeout <= 0, defaults to 30 minutes.
func NewWebInterviewer(timeout time.Duration) *WebInterviewer {
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	return &WebInterviewer{timeout: timeout, cancelCh: make(chan struct{})}
}

// Ask implements engine.Interviewer. It blocks until an answer is posted or timeout.
func (wi *WebInterviewer) Ask(q engine.Question) engine.Answer {
	wi.mu.Lock()
	wi.qidSeq++
	qid := fmt.Sprintf("q-%d", wi.qidSeq)
	ch := make(chan engine.Answer, 1)
	pq := &pendingQuestion{
		ID:       qid,
		Question: q,
		AskedAt:  time.Now().UTC(),
		answerCh: ch,
	}
	wi.pending = pq
	wi.mu.Unlock()

	defer func() {
		wi.mu.Lock()
		if wi.pending == pq {
			wi.pending = nil
		}
		wi.mu.Unlock()
	}()

	timer := time.NewTimer(wi.timeout)
	defer timer.Stop()

	select {
	case ans := <-ch:
		return ans
	case <-timer.C:
		return engine.Answer{TimedOut: true}
	case <-wi.cancelCh:
		return engine.Answer{TimedOut: true}
	}
}

// Pending returns the current pending question, or nil if none.
func (wi *WebInterviewer) Pending() *PendingQuestion {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	if wi.pending == nil {
		return nil
	}
	pq := wi.pending
	opts := make([]QuestionOption, len(pq.Question.Options))
	for i, o := range pq.Question.Options {
		opts[i] = QuestionOption{Key: o.Key, Label: o.Label, To: o.To}
	}
	return &PendingQuestion{
		QuestionID: pq.ID,
		Type:       string(pq.Question.Type),
		Text:       pq.Question.Text,
		Stage:      pq.Question.Stage,
		Options:    opts,
		AskedAt:    pq.AskedAt,
	}
}

// Cancel unblocks any in-flight Ask() call, causing it to return a TimedOut answer.
// Safe to call multiple times.
func (wi *WebInterviewer) Cancel() {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	select {
	case <-wi.cancelCh:
		// already closed
	default:
		close(wi.cancelCh)
	}
}

// Answer delivers an answer to the pending question. Returns false if qid
// doesn't match or no question is pending.
func (wi *WebInterviewer) Answer(qid string, ans engine.Answer) bool {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	if wi.pending == nil || wi.pending.ID != qid {
		return false
	}
	select {
	case wi.pending.answerCh <- ans:
		wi.pending = nil // prevent duplicate answers via race with Ask()'s defer
		return true
	default:
		return false // already answered
	}
}
