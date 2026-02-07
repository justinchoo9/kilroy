package engine

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

// ConsoleInterviewer prompts on stdin/stdout. Intended for interactive runs.
// In non-interactive environments, prefer AutoApproveInterviewer or QueueInterviewer.
type ConsoleInterviewer struct {
	In  *os.File
	Out *os.File
}

func (i *ConsoleInterviewer) Ask(q Question) Answer {
	in := i.In
	out := i.Out
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}

	_, _ = fmt.Fprintf(out, "\n[%s] %s\n", q.Stage, strings.TrimSpace(q.Text))
	switch q.Type {
	case QuestionFreeText:
		_, _ = fmt.Fprint(out, "> ")
		s, _ := bufio.NewReader(in).ReadString('\n')
		return Answer{Text: strings.TrimSpace(s)}
	case QuestionConfirm:
		_, _ = fmt.Fprint(out, "(y/n)> ")
		s, _ := bufio.NewReader(in).ReadString('\n')
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "y" || s == "yes" {
			return Answer{Value: "YES"}
		}
		return Answer{Value: "NO"}
	case QuestionMultiSelect:
		for _, o := range q.Options {
			_, _ = fmt.Fprintf(out, "  [%s] %s\n", o.Key, o.Label)
		}
		_, _ = fmt.Fprint(out, "comma-separated> ")
		s, _ := bufio.NewReader(in).ReadString('\n')
		raw := strings.TrimSpace(s)
		if raw == "" {
			return Answer{}
		}
		parts := strings.Split(raw, ",")
		var vals []string
		for _, p := range parts {
			if v := strings.TrimSpace(p); v != "" {
				vals = append(vals, v)
			}
		}
		return Answer{Values: vals}
	default:
		// SINGLE_SELECT (default)
		for _, o := range q.Options {
			_, _ = fmt.Fprintf(out, "  [%s] %s\n", o.Key, o.Label)
		}
		_, _ = fmt.Fprint(out, "> ")
		s, _ := bufio.NewReader(in).ReadString('\n')
		return Answer{Value: strings.TrimSpace(s)}
	}
}

type CallbackInterviewer struct {
	Fn func(Question) Answer
}

func (i *CallbackInterviewer) Ask(q Question) Answer {
	if i == nil || i.Fn == nil {
		return Answer{}
	}
	return i.Fn(q)
}

// QueueInterviewer returns pre-seeded answers in order. Useful for tests.
type QueueInterviewer struct {
	mu      sync.Mutex
	Answers []Answer
}

func (i *QueueInterviewer) Ask(q Question) Answer {
	_ = q
	i.mu.Lock()
	defer i.mu.Unlock()
	if len(i.Answers) == 0 {
		return Answer{}
	}
	a := i.Answers[0]
	i.Answers = i.Answers[1:]
	return a
}

func acceleratorKey(label string) string {
	s := strings.TrimSpace(label)
	if s == "" {
		return ""
	}
	// Patterns:
	// [K] Label
	// K) Label
	// K - Label
	// Fallback: first character.
	if len(s) >= 4 && s[0] == '[' && s[2] == ']' && s[3] == ' ' {
		return strings.ToUpper(string(s[1]))
	}
	if len(s) >= 3 && s[1] == ')' && s[2] == ' ' {
		return strings.ToUpper(string(s[0]))
	}
	if len(s) >= 4 && s[1] == ' ' && s[2] == '-' && s[3] == ' ' {
		return strings.ToUpper(string(s[0]))
	}
	return strings.ToUpper(string(s[0]))
}
