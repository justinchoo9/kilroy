package engine

import (
	"strings"
	"testing"

	"github.com/strongdm/kilroy/internal/attractor/model"
)

type setGraphAttrTransform struct {
	key   string
	value string
}

func (t setGraphAttrTransform) ID() string { return "set_attr" }
func (t setGraphAttrTransform) Apply(g *model.Graph) error {
	g.Attrs[t.key] = t.value
	return nil
}

type appendGraphAttrTransform struct {
	key    string
	suffix string
}

func (t appendGraphAttrTransform) ID() string { return "append_attr" }
func (t appendGraphAttrTransform) Apply(g *model.Graph) error {
	g.Attrs[t.key] = g.Attrs[t.key] + t.suffix
	return nil
}

type fixBadConditionTransform struct{}

func (t fixBadConditionTransform) ID() string { return "fix_condition" }
func (t fixBadConditionTransform) Apply(g *model.Graph) error {
	for _, e := range g.Edges {
		if e == nil {
			continue
		}
		if strings.TrimSpace(e.Attrs["condition"]) == "outcome=" {
			e.Attrs["condition"] = "outcome=success"
		}
	}
	return nil
}

func TestPrepare_Transforms_RunBetweenParseAndValidate_InRegistrationOrder(t *testing.T) {
	dot := []byte(`
digraph G {
  start [shape=Mdiamond]
  cond [shape=diamond]
  exit [shape=Msquare]
  start -> cond
  cond -> exit [condition="outcome="]
}
`)

	// No transforms: validation fails.
	if _, _, err := Prepare(dot); err == nil {
		t.Fatalf("expected validation error, got nil")
	}

	reg := NewTransformRegistry()
	reg.Register(setGraphAttrTransform{key: "x", value: "1"})
	reg.Register(appendGraphAttrTransform{key: "x", suffix: "2"})
	reg.Register(fixBadConditionTransform{})

	g, _, err := PrepareWithRegistry(dot, reg)
	if err != nil {
		t.Fatalf("PrepareWithRegistry: %v", err)
	}
	if got := g.Attrs["x"]; got != "12" {
		t.Fatalf("transform order: got %q want %q", got, "12")
	}
}

