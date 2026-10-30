package llm

import (
	"context"
	"sync"
	"sync/atomic"
)

// Manager atomically publishes validated model runtimes. Existing requests may
// finish on the prior runtime while new requests immediately see the replacement.
type Manager struct {
	current atomic.Pointer[Runtime]
	mu      sync.Mutex
}

func NewManager(initial *Runtime) *Manager {
	manager := &Manager{}
	manager.current.Store(initial)
	return manager
}

func (m *Manager) Current() *Runtime {
	if m == nil {
		return nil
	}
	return m.current.Load()
}

func (m *Manager) Status() Status {
	if m == nil || m.Current() == nil {
		return statusFromConfig(Config{})
	}
	return m.Current().Status()
}

func (m *Manager) Enabled() bool {
	return m != nil && m.Current() != nil && m.Current().Enabled()
}

func (m *Manager) Config() Config {
	if m == nil || m.Current() == nil {
		return Config{}
	}
	return m.Current().Config()
}

func (m *Manager) Publish(candidate *Runtime) {
	if m == nil || candidate == nil {
		return
	}
	m.mu.Lock()
	previous := m.current.Swap(candidate)
	m.mu.Unlock()
	previous.Close()
}

func (m *Manager) Replace(ctx context.Context, config Config) error {
	candidate, err := New(ctx, config)
	if err != nil {
		return err
	}
	m.Publish(candidate)
	return nil
}

func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	previous := m.current.Swap(nil)
	m.mu.Unlock()
	previous.Close()
}
