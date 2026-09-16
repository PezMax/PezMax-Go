package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	ListPermission      = "system:job:list"
	CreatePermission    = "system:job:create"
	UpdatePermission    = "system:job:update"
	DeletePermission    = "system:job:delete"
	RunPermission       = "system:job:run"
	LogListPermission   = "system:job-log:list"
	HandlerLogCleanup   = "log_cleanup"
	HandlerCacheRefresh = "cache_refresh"

	statusEnabled = "enabled"
	statusPaused  = "paused"

	triggerScheduled = "scheduled"
	triggerManual    = "manual"

	executionRunning = "running"
	executionSuccess = "success"
	executionFailed  = "failed"
)

var (
	ErrAlreadyRunning = errors.New("task is already running")
	ErrBuiltInTask    = errors.New("built-in task cannot be deleted")
	ErrJobNotFound    = errors.New("task not found")
	ErrNameExists     = errors.New("task name already exists")
)

type Job struct {
	ID             int64           `json:"id"`
	Name           string          `json:"name"`
	Handler        string          `json:"handler"`
	CronExpression string          `json:"cronExpression"`
	Parameters     json.RawMessage `json:"parameters" swaggertype:"object"`
	Description    string          `json:"description"`
	Status         string          `json:"status"`
	BuiltIn        bool            `json:"builtIn"`
	LastRunAt      string          `json:"lastRunAt"`
	NextRunAt      string          `json:"nextRunAt"`
	CreatedBy      int64           `json:"createdBy"`
	CreatedAt      string          `json:"createdAt"`
	UpdatedAt      string          `json:"updatedAt"`
}

type JobPayload struct {
	Name           string          `json:"name"`
	Handler        string          `json:"handler"`
	CronExpression string          `json:"cronExpression"`
	Parameters     json.RawMessage `json:"parameters" swaggertype:"object"`
	Description    string          `json:"description"`
	Status         string          `json:"status"`
}

type JobExecution struct {
	ID          int64  `json:"id"`
	JobID       *int64 `json:"jobId"`
	JobName     string `json:"jobName"`
	Handler     string `json:"handler"`
	Trigger     string `json:"trigger"`
	Status      string `json:"status"`
	Output      string `json:"output"`
	Error       string `json:"error"`
	DurationMs  int64  `json:"durationMs"`
	TriggeredBy *int64 `json:"triggeredBy"`
	StartedAt   string `json:"startedAt"`
	FinishedAt  string `json:"finishedAt"`
	CreatedAt   string `json:"createdAt"`
}

type JobFilter struct {
	Page     int
	PageSize int
	Keyword  string
	Handler  string
	Status   string
}

type ExecutionFilter struct {
	Page     int
	PageSize int
	Keyword  string
	JobID    int64
	Status   string
	Trigger  string
}

type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

type TaskHandler func(job Job) (string, error)

func normalizePayload(payload *JobPayload) error {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Handler = strings.TrimSpace(payload.Handler)
	payload.CronExpression = strings.TrimSpace(payload.CronExpression)
	payload.Description = strings.TrimSpace(payload.Description)
	payload.Status = strings.TrimSpace(payload.Status)
	if payload.Status == "" {
		payload.Status = statusEnabled
	}
	if payload.Parameters == nil || len(payload.Parameters) == 0 {
		payload.Parameters = json.RawMessage("{}")
	}

	switch {
	case payload.Name == "":
		return errors.New("task name is required")
	case len([]rune(payload.Name)) > 100:
		return errors.New("task name is too long")
	case payload.Handler == "":
		return errors.New("task handler is required")
	case payload.CronExpression == "":
		return errors.New("cron expression is required")
	case payload.Status != statusEnabled && payload.Status != statusPaused:
		return errors.New("invalid task status")
	case len([]rune(payload.Description)) > 500:
		return errors.New("task description is too long")
	case len(payload.Parameters) > 16*1024:
		return errors.New("task parameters are too large")
	}
	if err := validateCronExpression(payload.CronExpression); err != nil {
		return errors.New("invalid cron expression")
	}
	var object map[string]interface{}
	if err := json.Unmarshal(payload.Parameters, &object); err != nil || object == nil {
		return errors.New("task parameters must be a JSON object")
	}
	normalized, _ := json.Marshal(object)
	payload.Parameters = normalized
	return nil
}

func nextRun(expression string, from time.Time) (time.Time, error) {
	fields, err := parseCronFields(expression)
	if err != nil {
		return time.Time{}, err
	}
	candidate := from.Truncate(time.Minute).Add(time.Minute)
	limit := candidate.AddDate(5, 0, 0)
	for candidate.Before(limit) {
		if fields[0][candidate.Minute()] && fields[1][candidate.Hour()] && fields[2][candidate.Day()] &&
			fields[3][int(candidate.Month())] && fields[4][int(candidate.Weekday())] {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}
	return time.Time{}, errors.New("cron expression has no run time in the next five years")
}

func validateCronExpression(expression string) error {
	_, err := parseCronFields(expression)
	return err
}

func gcronPattern(expression string) string {
	return "# " + strings.Join(strings.Fields(expression), " ")
}

func parseCronFields(expression string) ([5]map[int]bool, error) {
	var parsed [5]map[int]bool
	parts := strings.Fields(expression)
	if len(parts) != 5 {
		return parsed, errors.New("cron expression must contain five fields")
	}
	ranges := [5][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 6}}
	for index, part := range parts {
		values, err := parseCronField(part, ranges[index][0], ranges[index][1], index == 2 || index == 4)
		if err != nil {
			return parsed, err
		}
		parsed[index] = values
	}
	return parsed, nil
}

func parseCronField(field string, minimum, maximum int, allowQuestion bool) (map[int]bool, error) {
	values := make(map[int]bool, maximum-minimum+1)
	if field == "*" || (allowQuestion && field == "?") {
		for value := minimum; value <= maximum; value++ {
			values[value] = true
		}
		return values, nil
	}
	for _, item := range strings.Split(field, ",") {
		step := 1
		base := item
		hasStep := false
		if slash := strings.Split(item, "/"); len(slash) == 2 {
			base = slash[0]
			hasStep = true
			parsedStep, err := strconv.Atoi(slash[1])
			if err != nil || parsedStep <= 0 {
				return nil, fmt.Errorf("invalid cron step %q", item)
			}
			step = parsedStep
		} else if len(slash) > 2 {
			return nil, fmt.Errorf("invalid cron item %q", item)
		}

		start, end := minimum, maximum
		if base != "*" {
			rangeParts := strings.Split(base, "-")
			parsedStart, err := strconv.Atoi(rangeParts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid cron value %q", item)
			}
			start, end = parsedStart, parsedStart
			if len(rangeParts) == 2 {
				parsedEnd, parseErr := strconv.Atoi(rangeParts[1])
				if parseErr != nil {
					return nil, fmt.Errorf("invalid cron range %q", item)
				}
				end = parsedEnd
			} else if hasStep {
				end = maximum
			} else if len(rangeParts) > 2 {
				return nil, fmt.Errorf("invalid cron range %q", item)
			}
		}
		if start < minimum || end > maximum || start > end {
			return nil, fmt.Errorf("cron value %q is out of range", item)
		}
		for value := start; value <= end; value += step {
			values[value] = true
		}
	}
	if len(values) == 0 {
		return nil, errors.New("cron field has no values")
	}
	return values, nil
}
