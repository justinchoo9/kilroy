package engine

import "github.com/strongdm/kilroy/internal/attractor/model"

// Transform can mutate the parsed graph between parse and validate (attractor-spec DoD).
type Transform interface {
	ID() string
	Apply(g *model.Graph) error
}

// TransformRegistry stores transforms to apply in registration order.
type TransformRegistry struct {
	transforms []Transform
}

func NewTransformRegistry() *TransformRegistry { return &TransformRegistry{} }

func (r *TransformRegistry) Register(t Transform) {
	if r == nil || t == nil {
		return
	}
	r.transforms = append(r.transforms, t)
}

func (r *TransformRegistry) List() []Transform {
	if r == nil || len(r.transforms) == 0 {
		return nil
	}
	return append([]Transform{}, r.transforms...)
}

// Built-in transforms.

type goalExpansionTransform struct{}

func (t goalExpansionTransform) ID() string { return "expand_goal" }
func (t goalExpansionTransform) Apply(g *model.Graph) error {
	expandGoal(g)
	return nil
}

