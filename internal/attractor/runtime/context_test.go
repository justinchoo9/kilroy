package runtime

import "testing"

func TestContext_SetGetAndSnapshots(t *testing.T) {
	c := NewContext()
	c.Set("a", "1")
	if v, ok := c.Get("a"); !ok || v != "1" {
		t.Fatalf("Get(a)=%v ok=%v", v, ok)
	}
	if got := c.GetString("a", ""); got != "1" {
		t.Fatalf("GetString(a)=%q", got)
	}
	if got := c.GetString("missing", "d"); got != "d" {
		t.Fatalf("GetString(missing)=%q", got)
	}

	c.AppendLog("warn")
	vals := c.SnapshotValues()
	logs := c.SnapshotLogs()
	if vals["a"] != "1" || len(logs) != 1 || logs[0] != "warn" {
		t.Fatalf("snapshots: vals=%v logs=%v", vals, logs)
	}
}

func TestContext_CloneAndReplaceSnapshot(t *testing.T) {
	c := NewContext()
	c.Set("x", 1)
	c.AppendLog("l1")

	cl := c.Clone()
	cl.Set("x", 2)
	cl.AppendLog("l2")

	if got := c.GetString("x", ""); got != "1" {
		t.Fatalf("original mutated: %q", got)
	}
	if got := cl.GetString("x", ""); got != "2" {
		t.Fatalf("clone not updated: %q", got)
	}

	c.ReplaceSnapshot(map[string]any{"k": "v"}, []string{"l3"})
	if got := c.GetString("k", ""); got != "v" {
		t.Fatalf("ReplaceSnapshot values: %q", got)
	}
	if _, ok := c.Get("x"); ok {
		t.Fatalf("ReplaceSnapshot should replace values entirely")
	}
	if logs := c.SnapshotLogs(); len(logs) != 1 || logs[0] != "l3" {
		t.Fatalf("ReplaceSnapshot logs: %v", logs)
	}
}

