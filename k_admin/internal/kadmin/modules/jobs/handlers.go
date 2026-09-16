package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
)

type cleanupParameters struct {
	RetentionDays        int `json:"retentionDays"`
	TaskLogRetentionDays int `json:"taskLogRetentionDays"`
}

func validateHandlerParameters(handler string, raw json.RawMessage) error {
	switch handler {
	case HandlerLogCleanup:
		parameters, err := parseCleanupParameters(raw)
		if err != nil {
			return err
		}
		if parameters.RetentionDays < 1 || parameters.RetentionDays > 3650 ||
			parameters.TaskLogRetentionDays < 1 || parameters.TaskLogRetentionDays > 3650 {
			return errors.New("log retention days must be between 1 and 3650")
		}
	case HandlerCacheRefresh:
		return nil
	default:
		return errors.New("unknown task handler")
	}
	return nil
}

func parseCleanupParameters(raw json.RawMessage) (cleanupParameters, error) {
	parameters := cleanupParameters{RetentionDays: 30, TaskLogRetentionDays: 90}
	if len(raw) == 0 {
		return parameters, nil
	}
	if err := json.Unmarshal(raw, &parameters); err != nil {
		return cleanupParameters{}, errors.New("invalid log cleanup parameters")
	}
	return parameters, nil
}

func (m *Manager) logCleanupHandler(job Job) (string, error) {
	parameters, err := parseCleanupParameters(job.Parameters)
	if err != nil {
		return "", err
	}
	requestResult, err := m.repository.conn.Exec(`DELETE FROM public.goadmin_operation_log
		WHERE occurred_at < CURRENT_TIMESTAMP - (? * INTERVAL '1 day')`, parameters.RetentionDays)
	if err != nil {
		return "", fmt.Errorf("clean request logs: %w", err)
	}
	requestCount, _ := requestResult.RowsAffected()
	taskResult, err := m.repository.conn.Exec(`DELETE FROM public.kadmin_job_logs
		WHERE status <> 'running' AND started_at < CURRENT_TIMESTAMP - (? * INTERVAL '1 day')`, parameters.TaskLogRetentionDays)
	if err != nil {
		return "", fmt.Errorf("clean task logs: %w", err)
	}
	taskCount, _ := taskResult.RowsAffected()
	return fmt.Sprintf("清理请求日志 %d 条，任务日志 %d 条", requestCount, taskCount), nil
}

func cacheRefreshHandler(refresh func() error) TaskHandler {
	return func(Job) (string, error) {
		if refresh == nil {
			return "", errors.New("cache refresh handler is unavailable")
		}
		if err := refresh(); err != nil {
			return "", err
		}
		return "权限与菜单基础数据已刷新", nil
	}
}
