package runtime

import (
	"fmt"
	"sync"
)

// Context is the shared key-value store for a pipeline run.
// Values must be JSON-serializable for checkpointing.
type Context struct {
	mu     sync.RWMutex
	values map[string]any
	logs   []string
}

func NewContext() *Context {
	return &Context{
		values: map[string]any{},
		logs:   []string{},
	}
}

func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil {
		c.values = map[string]any{}
	}
	c.values[key] = value
}

func (c *Context) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.values[key]
	return v, ok
}

func (c *Context) GetString(key string, def string) string {
	v, ok := c.Get(key)
	if !ok || v == nil {
		return def
	}
	return fmt.Sprint(v)
}

func (c *Context) AppendLog(entry string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logs = append(c.logs, entry)
}

func (c *Context) SnapshotValues() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]any, len(c.values))
	for k, v := range c.values {
		out[k] = v
	}
	return out
}

func (c *Context) SnapshotLogs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, len(c.logs))
	copy(out, c.logs)
	return out
}

func (c *Context) Clone() *Context {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := NewContext()
	for k, v := range c.values {
		out.values[k] = v
	}
	out.logs = append(out.logs, c.logs...)
	return out
}

func (c *Context) ApplyUpdates(updates map[string]any) {
	if len(updates) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range updates {
		if c.values == nil {
			c.values = map[string]any{}
		}
		c.values[k] = v
	}
}

func (c *Context) ReplaceSnapshot(values map[string]any, logs []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values = map[string]any{}
	for k, v := range values {
		c.values[k] = v
	}
	c.logs = append([]string{}, logs...)
}
