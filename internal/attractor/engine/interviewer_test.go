package engine

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestAutoApproveInterviewer_SelectsFirstOption(t *testing.T) {
	i := &AutoApproveInterviewer{}
	ans := i.Ask(Question{
		Type:  QuestionSingleSelect,
		Text:  "choose",
		Stage: "s",
		Options: []Option{
			{Key: "A", Label: "Approve", To: "a"},
			{Key: "F", Label: "Fix", To: "f"},
		},
	})
	if ans.Value != "A" {
		t.Fatalf("value: got %q want %q", ans.Value, "A")
	}
}

func TestAutoApproveInterviewer_NoOptions_DefaultsToYES(t *testing.T) {
	i := &AutoApproveInterviewer{}
	ans := i.Ask(Question{Type: QuestionConfirm, Text: "ok?", Stage: "s"})
	if ans.Value != "YES" {
		t.Fatalf("value: got %q want %q", ans.Value, "YES")
	}
}

func TestCallbackInterviewer_DelegatesToFn(t *testing.T) {
	i := &CallbackInterviewer{Fn: func(q Question) Answer {
		_ = q
		return Answer{Value: "X"}
	}}
	ans := i.Ask(Question{Type: QuestionSingleSelect, Text: "t", Stage: "s"})
	if ans.Value != "X" {
		t.Fatalf("value: got %q want %q", ans.Value, "X")
	}
}

func TestQueueInterviewer_PopsAnswersInOrder(t *testing.T) {
	i := &QueueInterviewer{Answers: []Answer{{Value: "A"}, {Value: "B"}}}
	if got := i.Ask(Question{}).Value; got != "A" {
		t.Fatalf("first: %q", got)
	}
	if got := i.Ask(Question{}).Value; got != "B" {
		t.Fatalf("second: %q", got)
	}
}

func TestConsoleInterviewer_SingleSelect_ReadsInputAndWritesPrompt(t *testing.T) {
	rIn, wIn, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rIn.Close() }()
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rOut.Close() }()

	go func() {
		_, _ = wIn.Write([]byte("F\n"))
		_ = wIn.Close()
	}()

	i := &ConsoleInterviewer{In: rIn, Out: wOut}
	ans := i.Ask(Question{
		Type:  QuestionSingleSelect,
		Text:  "choose",
		Stage: "gate",
		Options: []Option{
			{Key: "A", Label: "Approve"},
			{Key: "F", Label: "Fix"},
		},
	})
	_ = wOut.Close()

	outBytes, _ := io.ReadAll(rOut)
	outText := string(outBytes)
	if !strings.Contains(outText, "[gate] choose") {
		t.Fatalf("expected prompt to include stage/text; got:\n%s", outText)
	}
	if ans.Value != "F" {
		t.Fatalf("answer: got %q want %q", ans.Value, "F")
	}
}

func TestConsoleInterviewer_FreeText_ReadsText(t *testing.T) {
	rIn, wIn, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rIn.Close() }()
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rOut.Close() }()

	go func() {
		_, _ = wIn.Write([]byte("hello world\n"))
		_ = wIn.Close()
	}()

	i := &ConsoleInterviewer{In: rIn, Out: wOut}
	ans := i.Ask(Question{
		Type:  QuestionFreeText,
		Text:  "type something",
		Stage: "s",
	})
	_ = wOut.Close()

	if got := ans.Text; got != "hello world" {
		t.Fatalf("answer text: got %q want %q", got, "hello world")
	}
}

func TestConsoleInterviewer_Confirm_ParsesYesNo(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "y\n", want: "YES"},
		{input: "yes\n", want: "YES"},
		{input: "n\n", want: "NO"},
		{input: "\n", want: "NO"},
	} {
		rIn, wIn, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		rOut, wOut, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}

		go func() {
			_, _ = wIn.Write([]byte(tc.input))
			_ = wIn.Close()
		}()

		i := &ConsoleInterviewer{In: rIn, Out: wOut}
		ans := i.Ask(Question{
			Type:  QuestionConfirm,
			Text:  "ok?",
			Stage: "s",
		})
		_ = wOut.Close()
		_, _ = io.ReadAll(rOut)
		_ = rIn.Close()
		_ = rOut.Close()

		if got := ans.Value; got != tc.want {
			t.Fatalf("confirm(%q): got %q want %q", strings.TrimSpace(tc.input), got, tc.want)
		}
	}
}

func TestConsoleInterviewer_MultiSelect_ParsesCommaSeparatedValues(t *testing.T) {
	rIn, wIn, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rIn.Close() }()
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rOut.Close() }()

	go func() {
		_, _ = wIn.Write([]byte("A, C ,D\n"))
		_ = wIn.Close()
	}()

	i := &ConsoleInterviewer{In: rIn, Out: wOut}
	ans := i.Ask(Question{
		Type:  QuestionMultiSelect,
		Text:  "choose many",
		Stage: "gate",
		Options: []Option{
			{Key: "A", Label: "Apple"},
			{Key: "C", Label: "Carrot"},
			{Key: "D", Label: "Donut"},
		},
	})
	_ = wOut.Close()

	if got := strings.Join(ans.Values, ","); got != "A,C,D" {
		t.Fatalf("values: got %q want %q", got, "A,C,D")
	}
}
