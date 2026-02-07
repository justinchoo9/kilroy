package runtime

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFinalOutcome_Save_WritesJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "nested", "final.json")
	fo := &FinalOutcome{
		Timestamp:         time.Unix(123, 0).UTC(),
		Status:            FinalSuccess,
		RunID:             "r1",
		FinalGitCommitSHA: "abc",
		CXDBContextID:     "c1",
		CXDBHeadTurnID:    "t1",
	}
	if err := fo.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}

