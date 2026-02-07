package engine

import "testing"

func TestPrepare_ReturnsErrorOnValidationErrors(t *testing.T) {
	_, _, err := Prepare([]byte(`digraph G { exit [shape=Msquare] }`))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
