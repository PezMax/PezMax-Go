package monitor

import (
	"context"
	"errors"
	"sync"
	"time"
)

const defaultSamplingInterval = 3 * time.Second

type ManagerOptions struct {
	Settings  settingsStore
	Collector Collector
	Interval  time.Duration
	StartedAt time.Time
}

type Manager struct {
	settings  settingsStore
	collector Collector
	interval  time.Duration
	mu        sync.RWMutex
	collectMu sync.Mutex
	enabled   bool
	metrics   *Metrics
	lastError string
	ctx       context.Context
	cancel    context.CancelFunc
	wake      chan struct{}
	done      chan struct{}
}

func NewManager(options ManagerOptions) (*Manager, error) {
	if options.Settings == nil {
		return nil, errors.New("monitor settings store is required")
	}
	if options.Interval <= 0 {
		options.Interval = defaultSamplingInterval
	}
	if options.StartedAt.IsZero() {
		options.StartedAt = time.Now()
	}
	if options.Collector == nil {
		options.Collector = newSystemCollector(options.StartedAt)
	}
	enabled, err := options.Settings.LoadEnabled()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	manager := &Manager{
		settings: options.Settings, collector: options.Collector, interval: options.Interval,
		enabled: enabled, ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1), done: make(chan struct{}),
	}
	go manager.loop()
	return manager, nil
}

func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.cancel()
	<-m.done
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var metrics *Metrics
	if m.metrics != nil {
		copy := *m.metrics
		copy.Host.IPAddresses = append([]string(nil), m.metrics.Host.IPAddresses...)
		metrics = &copy
	}
	return Status{
		Enabled: m.enabled, SamplingIntervalSeconds: int(m.interval / time.Second),
		Metrics: metrics, LastError: m.lastError,
	}
}

func (m *Manager) SetEnabled(enabled bool, updatedBy int64) (Status, error) {
	if err := m.settings.SaveEnabled(enabled, updatedBy); err != nil {
		return Status{}, err
	}
	m.mu.Lock()
	m.enabled = enabled
	if !enabled {
		m.metrics = nil
		m.lastError = ""
	}
	m.mu.Unlock()
	m.signal()
	return m.Status(), nil
}

func (m *Manager) loop() {
	defer close(m.done)
	for {
		if !m.isEnabled() {
			select {
			case <-m.ctx.Done():
				return
			case <-m.wake:
				continue
			}
		}

		m.collect()
		timer := time.NewTimer(m.interval)
		select {
		case <-m.ctx.Done():
			timer.Stop()
			return
		case <-m.wake:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
	}
}

func (m *Manager) collect() {
	m.collectMu.Lock()
	defer m.collectMu.Unlock()
	if !m.isEnabled() {
		return
	}
	ctx, cancel := context.WithTimeout(m.ctx, m.interval)
	defer cancel()
	metrics, err := m.collector.Collect(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.enabled {
		return
	}
	if err != nil {
		m.lastError = err.Error()
		return
	}
	m.metrics = &metrics
	m.lastError = ""
}

func (m *Manager) isEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

func (m *Manager) signal() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}
