package loadrank

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
)

const defaultFlushInterval = 10 * time.Second

type metricKey struct {
	bucket int64
	route  string
	method string
	status int64
}

type metricAggregate struct {
	requestCount    int64
	errorCount      int64
	totalDurationMs int64
	maxDurationMs   int64
}

// Sampler aggregates request log events into per-minute summary buckets and
// periodically flushes them to the summary table. While sampling is disabled
// Observe returns immediately and no metric rows are written.
type Sampler struct {
	settings      settingsStore
	buckets       bucketStore
	routes        *RouteIndex
	flushInterval time.Duration
	now           func() time.Time

	mu         sync.RWMutex
	enabled    bool
	lastError  string
	lastFlush  time.Time
	aggregates map[metricKey]*metricAggregate

	ctx    context.Context
	cancel context.CancelFunc
	wake   chan struct{}
	done   chan struct{}
}

type SamplerOptions struct {
	Settings      settingsStore
	Buckets       bucketStore
	FlushInterval time.Duration
	Now           func() time.Time
}

func NewSampler(options SamplerOptions) (*Sampler, error) {
	if options.Settings == nil || options.Buckets == nil {
		return nil, errMissingDependency()
	}
	enabled, err := options.Settings.LoadEnabled()
	if err != nil {
		return nil, err
	}
	interval := options.FlushInterval
	if interval <= 0 {
		interval = defaultFlushInterval
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	ctx, cancel := context.WithCancel(context.Background())
	sampler := &Sampler{
		settings:      options.Settings,
		buckets:       options.Buckets,
		flushInterval: interval,
		now:           now,
		enabled:       enabled,
		aggregates:    make(map[metricKey]*metricAggregate),
		ctx:           ctx,
		cancel:        cancel,
		wake:          make(chan struct{}, 1),
		done:          make(chan struct{}),
	}
	go sampler.loop()
	return sampler, nil
}

type missingDependencyError struct{}

func (e *missingDependencyError) Error() string {
	return "load ranking sampler dependencies are required"
}

func errMissingDependency() error {
	return &missingDependencyError{}
}

// Close stops the flush loop and flushes any pending aggregates collected
// while sampling was enabled.
func (s *Sampler) Close() {
	if s == nil {
		return
	}
	s.cancel()
	<-s.done
}

// SetRouteIndex installs the registered route template index. Requests that
// do not match any registered route fall back to heuristic normalization.
func (s *Sampler) SetRouteIndex(index *RouteIndex) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes = index
}

// Status returns the current sampling status.
func (s *Sampler) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := Status{
		Enabled:              s.enabled,
		LastError:            s.lastError,
		BucketSeconds:        int(bucketSize / time.Second),
		FlushIntervalSeconds: int(s.flushInterval / time.Second),
		RetentionDays:        defaultRetentionDays,
	}
	if !s.lastFlush.IsZero() {
		status.LastFlushAt = s.lastFlush.Format(time.RFC3339)
	}
	return status
}

// SetEnabled persists and applies the sampling switch. Disabling drains the
// aggregates sampled while enabled, then stops all metric writes.
func (s *Sampler) SetEnabled(enabled bool, updatedBy int64) (Status, error) {
	if err := s.settings.SaveEnabled(enabled, updatedBy); err != nil {
		return s.Status(), err
	}
	if enabled {
		s.mu.Lock()
		s.enabled = true
		s.mu.Unlock()
		s.signal()
	} else {
		s.drain()
		s.mu.Lock()
		s.enabled = false
		s.mu.Unlock()
	}
	return s.Status(), nil
}

// Observe folds one request event into the in-memory aggregates. It is a
// no-op while sampling is disabled, so disabling stops every extra metric
// write immediately.
func (s *Sampler) Observe(event models.OperationLogEvent) {
	if !s.isEnabled() {
		return
	}
	if event.StatusCode == nil || event.DurationMs == nil {
		return
	}
	status := *event.StatusCode
	if status < 100 || status > 599 {
		return
	}
	duration := *event.DurationMs
	if duration < 0 {
		duration = 0
	}
	route := truncateRoute(event.Path)
	if template := s.resolveTemplate(event.Method, route); template != "" {
		route = template
	} else {
		route = normalizeRouteTemplate(route)
	}
	method := strings.ToUpper(strings.TrimSpace(event.Method))
	if method == "" {
		method = "GET"
	}

	s.mu.Lock()
	key := metricKey{bucket: s.now().Truncate(bucketSize).Unix(), route: route, method: method, status: status}
	aggregate := s.aggregates[key]
	if aggregate == nil {
		aggregate = &metricAggregate{}
		s.aggregates[key] = aggregate
	}
	aggregate.requestCount++
	if status >= 400 {
		aggregate.errorCount++
	}
	aggregate.totalDurationMs += duration
	if duration > aggregate.maxDurationMs {
		aggregate.maxDurationMs = duration
	}
	s.mu.Unlock()
}

func (s *Sampler) isEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

func (s *Sampler) resolveTemplate(method string, path string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.routes == nil {
		return ""
	}
	return s.routes.Resolve(method, path)
}

func (s *Sampler) loop() {
	defer close(s.done)
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			s.flush()
			return
		case <-s.wake:
			s.flush()
		case <-ticker.C:
			s.flush()
		}
	}
}

func (s *Sampler) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// flush drains pending aggregates into the summary table and prunes expired
// buckets. It only writes while sampling is enabled.
func (s *Sampler) flush() {
	if !s.isEnabled() {
		return
	}
	s.drain()
	if err := s.buckets.PruneBefore(s.now().Add(-time.Duration(defaultRetentionDays) * 24 * time.Hour)); err != nil {
		s.recordError(err)
	}
	s.mu.Lock()
	s.lastFlush = s.now()
	s.mu.Unlock()
}

// drain swaps the pending aggregates out and writes them; it is used by both
// the flush loop and SetEnabled(false) so no sampled data is lost on shutdown
// or on disabling.
func (s *Sampler) drain() {
	s.mu.Lock()
	pending := s.aggregates
	s.aggregates = make(map[metricKey]*metricAggregate)
	s.mu.Unlock()
	if len(pending) == 0 {
		return
	}
	for key, aggregate := range pending {
		row := bucketAggregate{
			route:           key.route,
			method:          key.method,
			statusCode:      key.status,
			requestCount:    aggregate.requestCount,
			errorCount:      aggregate.errorCount,
			totalDurationMs: aggregate.totalDurationMs,
			maxDurationMs:   aggregate.maxDurationMs,
		}
		if err := s.buckets.UpsertBucket(time.Unix(key.bucket, 0).UTC(), row); err != nil {
			s.recordError(err)
			return
		}
	}
}

func (s *Sampler) recordError(err error) {
	log.Printf("写入接口负载指标失败：%v", err)
	s.mu.Lock()
	s.lastError = err.Error()
	s.mu.Unlock()
}
