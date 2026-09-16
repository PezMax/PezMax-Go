package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/gogf/gf/v2/os/gcron"
)

type Manager struct {
	cron       *gcron.Cron
	entries    map[int64]string
	handlers   map[string]TaskHandler
	repository *repository
	mu         sync.Mutex
	running    map[int64]bool
	closed     bool
	runs       sync.WaitGroup
}

type ManagerOptions struct {
	Connection   db.Connection
	RefreshCache func() error
}

func NewManager(options ManagerOptions) (*Manager, error) {
	manager := &Manager{
		cron:       gcron.New(),
		entries:    make(map[int64]string),
		handlers:   make(map[string]TaskHandler),
		repository: &repository{conn: options.Connection},
		running:    make(map[int64]bool),
	}
	manager.handlers[HandlerLogCleanup] = manager.logCleanupHandler
	manager.handlers[HandlerCacheRefresh] = cacheRefreshHandler(options.RefreshCache)

	if err := manager.repository.markInterruptedExecutions(); err != nil {
		return nil, fmt.Errorf("mark interrupted task executions: %w", err)
	}
	jobs, err := manager.repository.listEnabledJobs()
	if err != nil {
		return nil, fmt.Errorf("load scheduled tasks: %w", err)
	}
	for _, job := range jobs {
		if err := manager.schedule(job); err != nil {
			manager.Close()
			return nil, fmt.Errorf("schedule task %q: %w", job.Name, err)
		}
	}
	return manager, nil
}

func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	m.mu.Unlock()
	m.cron.Close()
	m.runs.Wait()
}

func (m *Manager) ListJobs(filter JobFilter) (Page[Job], error) {
	return m.repository.listJobs(filter)
}

func (m *Manager) GetJob(id int64) (Job, bool, error) {
	return m.repository.getJob(id)
}

func (m *Manager) CreateJob(payload JobPayload, createdBy int64) (Job, error) {
	if err := m.validatePayload(&payload); err != nil {
		return Job{}, err
	}
	job, err := m.repository.createJob(payload, createdBy)
	if err != nil {
		return Job{}, err
	}
	if job.Status == statusEnabled {
		if err := m.schedule(job); err != nil {
			_, _ = m.repository.setStatus(job.ID, statusPaused)
			return Job{}, err
		}
	}
	job, _, err = m.repository.getJob(job.ID)
	return job, err
}

func (m *Manager) UpdateJob(id int64, payload JobPayload) (Job, error) {
	if err := m.validatePayload(&payload); err != nil {
		return Job{}, err
	}
	job, err := m.repository.updateJob(id, payload)
	if err != nil {
		return Job{}, err
	}
	m.unschedule(id)
	if job.Status == statusEnabled {
		if err := m.schedule(job); err != nil {
			_, _ = m.repository.setStatus(job.ID, statusPaused)
			return Job{}, err
		}
	}
	job, _, err = m.repository.getJob(id)
	return job, err
}

func (m *Manager) SetStatus(id int64, status string) (Job, error) {
	if status != statusEnabled && status != statusPaused {
		return Job{}, errors.New("invalid task status")
	}
	job, err := m.repository.setStatus(id, status)
	if err != nil {
		return Job{}, err
	}
	m.unschedule(id)
	if status == statusEnabled {
		if err := m.schedule(job); err != nil {
			_, _ = m.repository.setStatus(job.ID, statusPaused)
			return Job{}, err
		}
	}
	job, _, err = m.repository.getJob(id)
	return job, err
}

func (m *Manager) DeleteJob(id int64) error {
	if err := m.repository.deleteJob(id); err != nil {
		return err
	}
	m.unschedule(id)
	return nil
}

func (m *Manager) RunNow(id int64, triggeredBy int64) (JobExecution, error) {
	job, found, err := m.repository.getJob(id)
	if err != nil {
		return JobExecution{}, err
	}
	if !found {
		return JobExecution{}, ErrJobNotFound
	}
	return m.execute(job, triggerManual, &triggeredBy)
}

func (m *Manager) ListExecutions(filter ExecutionFilter) (Page[JobExecution], error) {
	return m.repository.listExecutions(filter)
}

func (m *Manager) GetExecution(id int64) (JobExecution, bool, error) {
	return m.repository.getExecution(id)
}

func (m *Manager) validatePayload(payload *JobPayload) error {
	if err := normalizePayload(payload); err != nil {
		return err
	}
	if _, ok := m.handlers[payload.Handler]; !ok {
		return errors.New("unknown task handler")
	}
	return validateHandlerParameters(payload.Handler, payload.Parameters)
}

func (m *Manager) schedule(job Job) error {
	if err := validateCronExpression(job.CronExpression); err != nil {
		return err
	}
	entryName := fmt.Sprintf("kadmin-job-%d", job.ID)
	entry, err := m.cron.AddSingleton(context.Background(), gcronPattern(job.CronExpression), func(context.Context) {
		latest, found, loadErr := m.repository.getJob(job.ID)
		if loadErr != nil {
			log.Printf("加载定时任务 %d 失败：%v", job.ID, loadErr)
			return
		}
		if !found || latest.Status != statusEnabled {
			return
		}
		if _, runErr := m.execute(latest, triggerScheduled, nil); runErr != nil && !errors.Is(runErr, ErrAlreadyRunning) {
			log.Printf("执行定时任务 %s 失败：%v", latest.Name, runErr)
		}
	}, entryName)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		entry.Close()
		return errors.New("task scheduler is stopped")
	}
	if oldEntry, ok := m.entries[job.ID]; ok {
		m.cron.Remove(oldEntry)
	}
	m.entries[job.ID] = entryName
	m.mu.Unlock()

	next, err := nextRun(job.CronExpression, time.Now())
	if err != nil {
		m.unschedule(job.ID)
		return err
	}
	if err := m.repository.updateNextRun(job.ID, &next); err != nil {
		m.unschedule(job.ID)
		return err
	}
	return nil
}

func (m *Manager) unschedule(id int64) {
	m.mu.Lock()
	entryName, ok := m.entries[id]
	if ok {
		delete(m.entries, id)
	}
	m.mu.Unlock()
	if ok {
		m.cron.Remove(entryName)
	}
}

func (m *Manager) execute(job Job, trigger string, triggeredBy *int64) (execution JobExecution, err error) {
	if !m.beginRun(job.ID) {
		return JobExecution{}, ErrAlreadyRunning
	}
	defer m.endRun(job.ID)

	releaseLock, locked, err := m.repository.tryAdvisoryLock(job.ID)
	if err != nil {
		return JobExecution{}, err
	}
	if !locked {
		return JobExecution{}, ErrAlreadyRunning
	}
	defer releaseLock()

	handler, ok := m.handlers[job.Handler]
	if !ok {
		return JobExecution{}, errors.New("unknown task handler")
	}
	started := time.Now()
	logID, err := m.repository.createExecution(job, trigger, triggeredBy, started)
	if err != nil {
		return JobExecution{}, err
	}

	status := executionSuccess
	output := ""
	errorMessage := ""
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("task panicked: %v", recovered)
				log.Printf("定时任务 %s panic：%v\n%s", job.Name, recovered, debug.Stack())
			}
		}()
		output, err = handler(job)
	}()
	if err != nil {
		status = executionFailed
		errorMessage = err.Error()
	}
	finished := time.Now()
	duration := finished.Sub(started).Milliseconds()
	if finishErr := m.repository.finishExecution(logID, status, output, errorMessage, finished, duration); finishErr != nil {
		return JobExecution{}, finishErr
	}

	next, nextErr := nextRun(job.CronExpression, finished)
	if nextErr == nil {
		nextPointer := &next
		if updateErr := m.repository.updateRunTimes(job.ID, started, nextPointer); updateErr != nil {
			log.Printf("更新定时任务 %s 的运行时间失败：%v", job.Name, updateErr)
		}
	}
	execution, _, loadErr := m.repository.getExecution(logID)
	if loadErr != nil {
		return JobExecution{}, loadErr
	}
	return execution, err
}

func (m *Manager) beginRun(id int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.running[id] {
		return false
	}
	m.running[id] = true
	m.runs.Add(1)
	return true
}

func (m *Manager) endRun(id int64) {
	m.mu.Lock()
	delete(m.running, id)
	m.mu.Unlock()
	m.runs.Done()
}
