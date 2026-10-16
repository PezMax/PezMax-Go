package kadmin

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Add expiration and fault injection to the existing thread-safe Redis fixture.
// The production cache uses TTL to decide whether a snapshot needs renewal.
type datumWarmupRedis struct {
	base     *fakeSecurityRedis
	mu       sync.Mutex
	expires  map[string]time.Time
	calls    map[string]int
	failures map[string]error
}

func newDatumWarmupRedis(base *fakeSecurityRedis) *datumWarmupRedis {
	return &datumWarmupRedis{
		base: base, expires: make(map[string]time.Time),
		calls: make(map[string]int), failures: make(map[string]error),
	}
}

func (f *datumWarmupRedis) do(args ...string) (interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(args) < 2 {
		return f.base.do(args...)
	}
	command, key := args[0], args[1]
	f.calls[command+":"+key]++
	if err := f.failures[command+":"+key]; err != nil {
		return nil, err
	}
	if deadline, ok := f.expires[key]; ok && !deadline.After(time.Now()) {
		_, _ = f.base.do("DEL", key)
		delete(f.expires, key)
	}
	if command == "TTL" {
		if _, err := f.base.do("GET", key); errors.Is(err, errRedisNil) {
			return int64(-2), nil
		} else if err != nil {
			return nil, err
		}
		if deadline, ok := f.expires[key]; ok {
			return int64(time.Until(deadline) / time.Second), nil
		}
		return int64(-1), nil
	}
	result, err := f.base.do(args...)
	if err != nil {
		return result, err
	}
	switch command {
	case "SET":
		delete(f.expires, key)
		for index := 3; index+1 < len(args); index++ {
			if args[index] == "EX" {
				seconds, _ := strconv.ParseInt(args[index+1], 10, 64)
				f.expires[key] = time.Now().Add(time.Duration(seconds) * time.Second)
			}
		}
	case "DEL":
		for _, deleted := range args[1:] {
			delete(f.expires, deleted)
		}
	}
	return result, nil
}

func (f *datumWarmupRedis) expireIn(key string, ttl time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.expires[key] = time.Now().Add(ttl)
}

func (f *datumWarmupRedis) fail(command, key string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures[command+":"+key] = err
}

func (f *datumWarmupRedis) callCount(command, key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[command+":"+key]
}

func newDatumWarmupFixture(t *testing.T) (*Store, *fakeDatumDB, *datumWarmupRedis) {
	t.Helper()
	t.Setenv(datumFileStateTTLKey, "1h")
	store, database := newDatumAuthStore(t, nil)
	redis := newDatumWarmupRedis(store.datum.redis.(*fakeSecurityRedis))
	store.datum.redis = redis
	return store, database, redis
}

func requireDatumWarmupEntry(t *testing.T, redis redisDoer, key string) datumStateEntry {
	t.Helper()
	raw, err := redis.do("GET", key)
	if err != nil {
		t.Fatalf("read warmed cache %s: %v", key, err)
	}
	text, ok := raw.(string)
	if !ok {
		t.Fatalf("cache %s value = %#v", key, raw)
	}
	var entry datumStateEntry
	if err := json.Unmarshal([]byte(text), &entry); err != nil {
		t.Fatalf("decode cache %s: %v", key, err)
	}
	if entry.Hash == "" || !json.Valid(entry.Payload) {
		t.Fatalf("cache %s entry = %#v", key, entry)
	}
	return entry
}

func awaitDatumWarmup(t *testing.T, ready func() bool) {
	t.Helper()
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for !ready() {
		select {
		case <-timeout.C:
			t.Fatal("background cache maintenance did not finish")
		case <-tick.C:
		}
	}
}

func TestDatumStateWarmerPrimesBothResourcesBeforeReturning(t *testing.T) {
	store, database, redis := newDatumWarmupFixture(t)
	seedApprovedFile(database, 1001, 7, "warmup.pdf", "math", 1, 2024, 1)
	database.users["alice"] = &datumUser{UserID: 7, UserName: "alice", Status: "1", Count: 3}
	var refreshBefore time.Duration
	warmer := startDatumStateWarmer(func(threshold time.Duration) error {
		refreshBefore = threshold
		return store.warmDatumStates(threshold)
	}, time.Minute)
	t.Cleanup(warmer.Close)
	if refreshBefore != 2*time.Minute {
		t.Fatalf("startup refresh threshold = %v, want 2m", refreshBefore)
	}
	// No HTTP request has been sent; both complete payloads must already exist.
	tree := requireDatumWarmupEntry(t, redis, store.treeStateKey())
	rank := requireDatumWarmupEntry(t, redis, store.rankStateKey())
	if !bytes.Contains(tree.Payload, []byte("warmup.pdf")) {
		t.Fatalf("warmed tree payload = %s", tree.Payload)
	}
	if !bytes.Contains(rank.Payload, []byte(`"userName":"alice"`)) {
		t.Fatalf("warmed rank payload = %s", rank.Payload)
	}
}

func TestDatumStateWarmKeepsValidSnapshotsAndHashes(t *testing.T) {
	store, _, redis := newDatumWarmupFixture(t)
	if err := store.warmDatumStates(time.Minute); err != nil {
		t.Fatal(err)
	}
	keys := []string{store.treeStateKey(), store.rankStateKey()}
	before := make(map[string]datumStateEntry)
	for _, key := range keys {
		before[key] = requireDatumWarmupEntry(t, redis, key)
	}
	if err := store.warmDatumStates(time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		after := requireDatumWarmupEntry(t, redis, key)
		if after.Hash != before[key].Hash || !bytes.Equal(after.Payload, before[key].Payload) {
			t.Fatalf("valid cache %s was replaced", key)
		}
		if sets := redis.callCount("SET", key); sets != 1 {
			t.Fatalf("cache %s was recomputed %d times, want 1", key, sets)
		}
	}
}

func TestDatumStateWarmAttemptsResourcesIndependently(t *testing.T) {
	for _, failedResource := range []string{"tree", "rank"} {
		t.Run(failedResource, func(t *testing.T) {
			store, _, redis := newDatumWarmupFixture(t)
			failedKey, otherKey := store.treeStateKey(), store.rankStateKey()
			if failedResource == "rank" {
				failedKey, otherKey = otherKey, failedKey
			}
			unavailable := errors.New("redis temporarily unavailable")
			redis.fail("GET", failedKey, unavailable)
			if err := store.warmDatumStates(time.Minute); !errors.Is(err, unavailable) {
				t.Fatalf("warm error = %v, want Redis failure", err)
			}
			requireDatumWarmupEntry(t, redis, otherKey)
			if redis.callCount("GET", failedKey) == 0 {
				t.Fatal("failed resource was never attempted")
			}
		})
	}
}

func TestDatumStateWarmerRestoresMissingKeysWithoutRequests(t *testing.T) {
	store, _, redis := newDatumWarmupFixture(t)
	warmer := startDatumStateWarmer(store.warmDatumStates, 5*time.Millisecond)
	t.Cleanup(warmer.Close)
	tree := requireDatumWarmupEntry(t, redis, store.treeStateKey())
	rank := requireDatumWarmupEntry(t, redis, store.rankStateKey())
	if _, err := redis.do("DEL", store.treeStateKey(), store.rankStateKey()); err != nil {
		t.Fatal(err)
	}
	awaitDatumWarmup(t, func() bool {
		_, treeErr := redis.do("GET", store.treeStateKey())
		_, rankErr := redis.do("GET", store.rankStateKey())
		return treeErr == nil && rankErr == nil
	})
	if requireDatumWarmupEntry(t, redis, store.treeStateKey()).Hash == tree.Hash {
		t.Fatal("recreated tree must receive a fresh hash")
	}
	if requireDatumWarmupEntry(t, redis, store.rankStateKey()).Hash == rank.Hash {
		t.Fatal("recreated rank must receive a fresh hash")
	}
}

func TestDatumStateWarmerRetriesAfterStartupFailure(t *testing.T) {
	store, _, redis := newDatumWarmupFixture(t)
	unavailable := errors.New("startup Redis outage")
	redis.fail("GET", store.treeStateKey(), unavailable)
	redis.fail("GET", store.rankStateKey(), unavailable)
	warmer := startDatumStateWarmer(store.warmDatumStates, 5*time.Millisecond)
	t.Cleanup(warmer.Close)
	redis.fail("GET", store.treeStateKey(), nil)
	redis.fail("GET", store.rankStateKey(), nil)
	awaitDatumWarmup(t, func() bool {
		_, treeErr := redis.do("GET", store.treeStateKey())
		_, rankErr := redis.do("GET", store.rankStateKey())
		return treeErr == nil && rankErr == nil
	})
	requireDatumWarmupEntry(t, redis, store.treeStateKey())
	requireDatumWarmupEntry(t, redis, store.rankStateKey())
}

func TestDatumStateWarmRefreshesNearExpiryInPlace(t *testing.T) {
	for _, changed := range []bool{false, true} {
		name := "same-payload"
		if changed {
			name = "changed-payload"
		}
		t.Run(name, func(t *testing.T) {
			redis := newDatumWarmupRedis(newFakeSecurityRedis())
			state := newDatumStateStore(redis)
			state.ttl = time.Hour
			const key = "test:state"
			before, _, err := state.load(key, func() (interface{}, error) { return []string{"old"}, nil })
			if err != nil {
				t.Fatal(err)
			}
			redis.expireIn(key, 30*time.Second)
			warmed, err := state.warm(key, func() (interface{}, error) {
				// Readers can still access the old snapshot during computation.
				current := requireDatumWarmupEntry(t, redis, key)
				if current.Hash != before.Hash {
					t.Fatal("old snapshot disappeared before its replacement was ready")
				}
				if changed {
					return []string{"new"}, nil
				}
				return []string{"old"}, nil
			}, time.Minute)
			if err != nil || !warmed {
				t.Fatalf("warm = %v, %v", warmed, err)
			}
			after := requireDatumWarmupEntry(t, redis, key)
			if (after.Hash != before.Hash) != changed {
				t.Fatalf("hash changed = %v, content changed = %v", after.Hash != before.Hash, changed)
			}
			rawTTL, err := redis.do("TTL", key)
			if err != nil || rawTTL.(int64) < 3590 {
				t.Fatalf("renewed TTL = %v, %v", rawTTL, err)
			}
		})
	}
}

func TestDatumStateWarmFailurePreservesOldSnapshot(t *testing.T) {
	for _, failure := range []string{"compute", "write"} {
		t.Run(failure, func(t *testing.T) {
			redis := newDatumWarmupRedis(newFakeSecurityRedis())
			state := newDatumStateStore(redis)
			const key = "test:state"
			before, _, err := state.load(key, func() (interface{}, error) { return []string{"old"}, nil })
			if err != nil {
				t.Fatal(err)
			}
			redis.expireIn(key, 30*time.Second)
			unavailable := errors.New("refresh temporarily unavailable")
			if failure == "write" {
				redis.fail("SET", key, unavailable)
			}
			warmed, err := state.warm(key, func() (interface{}, error) {
				if failure == "compute" {
					return nil, unavailable
				}
				return []string{"new"}, nil
			}, time.Minute)
			if warmed || !errors.Is(err, unavailable) {
				t.Fatalf("failed refresh = %v, %v", warmed, err)
			}
			after := requireDatumWarmupEntry(t, redis, key)
			if after.Hash != before.Hash || !bytes.Equal(after.Payload, before.Payload) {
				t.Fatal("failed refresh destroyed the previous snapshot")
			}
		})
	}
}

func TestDatumStateHotLoadContinuesDuringRefresh(t *testing.T) {
	store, _, redis := newDatumWarmupFixture(t)
	key := store.treeStateKey()
	before, _, err := store.stateStore().load(key, func() (interface{}, error) { return []string{"old"}, nil })
	if err != nil {
		t.Fatal(err)
	}
	redis.expireIn(key, 30*time.Second)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	refreshed := make(chan error, 1)
	go func() {
		_, err := store.stateStore().warm(key, func() (interface{}, error) {
			close(entered)
			<-release
			return []string{"new"}, nil
		}, time.Minute)
		refreshed <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("refresh did not start")
	}
	read := make(chan error, 1)
	go func() {
		entry, rebuilt, err := store.stateStore().load(key, func() (interface{}, error) {
			return nil, errors.New("hot read unexpectedly queried the database")
		})
		if err == nil && (rebuilt || entry.Hash != before.Hash || !bytes.Equal(entry.Payload, before.Payload)) {
			err = errors.New("hot read did not reuse the previous snapshot")
		}
		read <- err
	}()
	select {
	case err := <-read:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("hot read waited for background recomputation")
	}
	unblock()
	select {
	case err := <-refreshed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("refresh did not finish")
	}
}

func TestDatumStateColdLoadsShareStoreLock(t *testing.T) {
	store, _, _ := newDatumWarmupFixture(t)
	var computes atomic.Int32
	const readers = 16
	start := make(chan struct{})
	type result struct {
		entry datumStateEntry
		err   error
	}
	results := make(chan result, readers)
	var ready sync.WaitGroup
	ready.Add(readers)
	for index := 0; index < readers; index++ {
		go func() {
			ready.Done()
			<-start
			entry, _, err := store.stateStore().load(store.treeStateKey(), func() (interface{}, error) {
				computes.Add(1)
				time.Sleep(5 * time.Millisecond)
				return []string{"shared"}, nil
			})
			results <- result{entry, err}
		}()
	}
	ready.Wait()
	close(start)
	var hash string
	for index := 0; index < readers; index++ {
		select {
		case result := <-results:
			if result.err != nil {
				t.Fatal(result.err)
			}
			if hash == "" {
				hash = result.entry.Hash
			} else if result.entry.Hash != hash {
				t.Fatal("simultaneous readers received different snapshots")
			}
		case <-time.After(time.Second):
			t.Fatal("cold cache readers did not finish")
		}
	}
	if count := computes.Load(); count != 1 {
		t.Fatalf("cold cache computations = %d, want 1", count)
	}
}

func TestDatumStateInvalidationCannotBeUndoneByInFlightCompute(t *testing.T) {
	store, _, _ := newDatumWarmupFixture(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	loaded := make(chan error, 1)
	go func() {
		_, _, err := store.stateStore().load(store.treeStateKey(), func() (interface{}, error) {
			close(entered)
			<-release
			return []string{"old"}, nil
		})
		loaded <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("initial computation did not start")
	}
	invalidated := make(chan struct{})
	go func() {
		store.invalidateDatumTree()
		close(invalidated)
	}()
	// An unlocked DEL can finish here, before the stale compute writes its SET.
	// A correct invalidation waits for that SET and removes it afterward.
	select {
	case <-invalidated:
	case <-time.After(20 * time.Millisecond):
	}
	unblock()
	select {
	case err := <-loaded:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("initial computation did not finish")
	}
	select {
	case <-invalidated:
	case <-time.After(time.Second):
		t.Fatal("invalidation did not finish")
	}
	entry, rebuilt, err := store.stateStore().load(store.treeStateKey(), func() (interface{}, error) {
		return []string{"new"}, nil
	})
	if err != nil || !rebuilt || string(entry.Payload) != `["new"]` {
		t.Fatalf("post-invalidation load = %s, rebuilt=%v, err=%v", entry.Payload, rebuilt, err)
	}
}

func TestDatumStateRuntimeCloseWaitsAndIsIdempotent(t *testing.T) {
	var calls atomic.Int32
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	const interval = 5 * time.Millisecond
	warmer := startDatumStateWarmer(func(time.Duration) error {
		if calls.Add(1) > 1 {
			entered <- struct{}{}
			<-release
		}
		return nil
	}, interval)
	t.Cleanup(func() { unblock(); warmer.Close() })
	runtime := &Runtime{datumStates: warmer}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("periodic computation did not start")
	}
	closed := make(chan struct{}, 2)
	for index := 0; index < 2; index++ {
		go func() { runtime.Close(); closed <- struct{}{} }()
	}
	select {
	case <-closed:
		t.Fatal("Close returned while maintenance was still running")
	case <-time.After(10 * time.Millisecond):
	}
	unblock()
	for index := 0; index < 2; index++ {
		select {
		case <-closed:
		case <-time.After(time.Second):
			t.Fatal("idempotent Close did not return")
		}
	}
	before := calls.Load()
	time.Sleep(3 * interval)
	if calls.Load() != before {
		t.Fatal("cache maintenance continued after Runtime.Close")
	}
	runtime.Close()
}
