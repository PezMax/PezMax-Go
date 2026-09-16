package loginlogs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/medama-io/go-useragent"
)

const cleanupInterval = 24 * time.Hour

type Manager struct {
	repository *repository
	parser     *useragent.Parser
	cancel     context.CancelFunc
	done       chan struct{}
}

func NewManager(repository *repository) (*Manager, error) {
	if repository == nil || repository.conn == nil {
		return nil, errors.New("login audit repository is required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	manager := &Manager{repository: repository, parser: useragent.NewParser(), cancel: cancel, done: make(chan struct{})}
	go manager.cleanupLoop(ctx)
	return manager, nil
}

func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.cancel()
	<-m.done
}

func (m *Manager) Record(attempt Attempt) error {
	if m == nil {
		return errors.New("login audit manager is unavailable")
	}
	attempt.Account = truncate(strings.TrimSpace(attempt.Account), 100)
	attempt.IP = truncate(strings.TrimSpace(attempt.IP), 45)
	attempt.UserAgent = truncate(strings.TrimSpace(attempt.UserAgent), 1024)
	attempt.FailureReason = truncate(strings.TrimSpace(attempt.FailureReason), 255)
	if !validResult(attempt.Result) {
		return fmt.Errorf("invalid login audit result %q", attempt.Result)
	}
	if attempt.DurationMs < 0 {
		attempt.DurationMs = 0
	}
	if attempt.OccurredAt.IsZero() {
		attempt.OccurredAt = time.Now()
	}
	agent := m.parser.Parse(attempt.UserAgent)
	browser := browserLabel(attempt.UserAgent, fmt.Sprint(agent.Browser()), agent.BrowserVersion())
	operatingSystem := truncate(fmt.Sprint(agent.OS()), 100)
	return m.repository.insert(attempt, browser, operatingSystem)
}

func (m *Manager) List(filter Filter) (Page, error) {
	return m.repository.list(filter)
}

func (m *Manager) Delete(ids []int64) (CleanupResult, error) {
	count, err := m.repository.delete(ids)
	return CleanupResult{DeletedCount: count}, err
}

func (m *Manager) Retention() (Retention, error) {
	return m.repository.retention()
}

func (m *Manager) SetRetention(days int, updatedBy int64) (Retention, error) {
	if days < 1 || days > maxRetentionDays {
		return Retention{}, fmt.Errorf("retention days must be between 1 and %d", maxRetentionDays)
	}
	setting, err := m.repository.setRetention(days, updatedBy)
	if err != nil {
		return Retention{}, err
	}
	_, err = m.repository.cleanup(setting.Days)
	return setting, err
}

func (m *Manager) CleanupExpired() (CleanupResult, error) {
	setting, err := m.repository.retention()
	if err != nil {
		return CleanupResult{}, err
	}
	count, err := m.repository.cleanup(setting.Days)
	return CleanupResult{DeletedCount: count}, err
}

func (m *Manager) cleanupLoop(ctx context.Context) {
	defer close(m.done)
	_, _ = m.CleanupExpired()
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = m.CleanupExpired()
		}
	}
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func browserLabel(userAgent, parsedName, parsedVersion string) string {
	for _, signature := range []struct {
		name  string
		token string
	}{
		{name: "Edge", token: "Edg/"},
		{name: "Opera", token: "OPR/"},
		{name: "Chrome", token: "Chrome/"},
		{name: "Firefox", token: "Firefox/"},
	} {
		if version := tokenVersion(userAgent, signature.token); version != "" {
			return truncate(signature.name+" "+version, 100)
		}
	}
	if strings.Contains(userAgent, "Safari/") {
		if version := tokenVersion(userAgent, "Version/"); version != "" {
			return truncate("Safari "+version, 100)
		}
	}
	browser := strings.TrimSpace(parsedName)
	if version := strings.TrimSpace(parsedVersion); version != "" {
		browser = strings.TrimSpace(browser + " " + version)
	}
	return truncate(browser, 100)
}

func tokenVersion(userAgent, token string) string {
	index := strings.Index(userAgent, token)
	if index < 0 {
		return ""
	}
	value := userAgent[index+len(token):]
	if end := strings.IndexAny(value, " ;)"); end >= 0 {
		value = value[:end]
	}
	return strings.TrimSpace(value)
}
