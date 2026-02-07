package engine

import "testing"

func TestPrepare_ExpandsGoalInPrompts(t *testing.T) {
	g, _, err := Prepare([]byte(`
digraph G {
  graph [goal="Do the thing"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="Goal is: $goal"]
  start -> a -> exit
}
`))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if got := g.Nodes["a"].Attr("prompt", ""); got != "Goal is: Do the thing" {
		t.Fatalf("prompt: %q", got)
	}
}

