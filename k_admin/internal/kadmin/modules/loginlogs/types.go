package loginlogs

import "time"

const (
	ListPermission      = "system:login-log:list"
	DeletePermission    = "system:login-log:delete"
	RetentionPermission = "system:login-log:retention"

	StatusSuccess = "success"
	StatusFailed  = "failed"

	ResultSuccess         = "success"
	ResultAccountNotFound = "account_not_found"
	ResultInvalidPassword = "invalid_password"
	ResultAccountDisabled = "account_disabled"
	ResultAccountLocked   = "account_locked"
	ResultAccountUnlocked = "account_unlocked"
	ResultCaptchaInvalid  = "captcha_invalid"
	ResultSystemError     = "system_error"

	defaultRetentionDays = 90
	maxPageSize          = 100
	maxRetentionDays     = 3650
)

type Attempt struct {
	Account       string
	UserID        *int64
	IP            string
	UserAgent     string
	Result        string
	FailureReason string
	DurationMs    int64
	OccurredAt    time.Time
}

type Entry struct {
	ID            int64  `json:"id"`
	Account       string `json:"account"`
	UserID        *int64 `json:"userId"`
	IP            string `json:"ip"`
	UserAgent     string `json:"userAgent"`
	Browser       string `json:"browser"`
	OS            string `json:"os"`
	Status        string `json:"status"`
	Result        string `json:"result"`
	FailureReason string `json:"failureReason"`
	DurationMs    int64  `json:"durationMs"`
	OccurredAt    string `json:"occurredAt"`
	CreatedAt     string `json:"createdAt"`
}

type Filter struct {
	Page      int
	PageSize  int
	Account   string
	IP        string
	Status    string
	Result    string
	StartedAt *time.Time
	EndedAt   *time.Time
}

type Page struct {
	Items    []Entry `json:"items"`
	Total    int64   `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
}

type Retention struct {
	Days      int    `json:"days"`
	UpdatedBy int64  `json:"updatedBy"`
	UpdatedAt string `json:"updatedAt"`
}

type CleanupResult struct {
	DeletedCount int64 `json:"deletedCount"`
}

func validResult(value string) bool {
	switch value {
	case ResultSuccess, ResultAccountNotFound, ResultInvalidPassword,
		ResultAccountDisabled, ResultAccountLocked, ResultAccountUnlocked, ResultCaptchaInvalid, ResultSystemError:
		return true
	default:
		return false
	}
}
