package kadmin

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

const datumStateWarmInterval = 30 * time.Second

type datumStateWarmer struct {
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

// The first pass is synchronous so a cold cache is ready before HTTP starts.
// Failures are non-fatal: the maintenance loop retries without requiring a
// desktop request, and each resource is attempted independently.
func startDatumStateWarmer(warm func(time.Duration) error, interval time.Duration) *datumStateWarmer {
	refreshBefore := 2 * interval
	if err := warm(refreshBefore); err != nil {
		log.Printf("文件树/排行榜缓存预热失败，将在后台重试：%v", err)
	} else {
		log.Print("文件树/排行榜缓存预热完成")
	}
	w := &datumStateWarmer{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(w.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-w.stop:
				return
			case <-ticker.C:
				select {
				case <-w.stop:
					return
				default:
				}
				if err := warm(refreshBefore); err != nil {
					log.Printf("文件树/排行榜缓存维护失败，下轮重试：%v", err)
				}
			}
		}
	}()
	return w
}

func (w *datumStateWarmer) Close() {
	if w == nil {
		return
	}
	w.once.Do(func() { close(w.stop) })
	<-w.done
}

func (s *Store) warmDatumStates(refreshBefore time.Duration) error {
	state := s.stateStore()
	var failures []error
	for _, resource := range []struct {
		name    string
		key     string
		compute func() (interface{}, error)
	}{
		{"文件树", s.treeStateKey(), s.computeDatumTree},
		{"排行榜", s.rankStateKey(), s.computeDatumRank},
	} {
		if _, err := state.warm(resource.key, resource.compute, refreshBefore); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", resource.name, err))
		}
	}
	return errors.Join(failures...)
}
