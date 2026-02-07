package engine

import (
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestNewRunID_ReturnsValidULIDAndIsFilesystemSafe(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id, err := NewRunID()
		if err != nil {
			t.Fatalf("NewRunID: %v", err)
		}
		if strings.TrimSpace(id) == "" {
			t.Fatalf("empty id")
		}
		if _, err := ulid.ParseStrict(id); err != nil {
			t.Fatalf("ParseStrict(%q): %v", id, err)
		}
		if strings.Contains(id, "/") || strings.Contains(id, "\\") {
			t.Fatalf("id contains path separator: %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id: %q", id)
		}
		seen[id] = true
	}
}
