package loadrank

const (
	ViewPermission   = "system:load-rank:view"
	UpdatePermission = "system:load-rank:update"
)

// Status describes the HTTP metric sampling state, mirroring the monitor module.
type Status struct {
	Enabled              bool   `json:"enabled"`
	LastError            string `json:"lastError"`
	BucketSeconds        int    `json:"bucketSeconds"`
	FlushIntervalSeconds int    `json:"flushIntervalSeconds"`
	RetentionDays        int    `json:"retentionDays"`
	LastFlushAt          string `json:"lastFlushAt,omitempty"`
}

// RankingItem is one aggregated group (route, method or status code) within the
// query window.
type RankingItem struct {
	Route         string  `json:"route,omitempty"`
	Method        string  `json:"method,omitempty"`
	StatusCode    *int64  `json:"statusCode,omitempty"`
	RequestCount  int64   `json:"requestCount"`
	ErrorCount    int64   `json:"errorCount"`
	ErrorRate     float64 `json:"errorRate"`
	QPS           float64 `json:"qps"`
	AvgDurationMs float64 `json:"avgDurationMs"`
	MaxDurationMs int64   `json:"maxDurationMs"`
}

// RankingResponse is the paged ranking result.
type RankingResponse struct {
	Items         []RankingItem `json:"items"`
	Total         int64         `json:"total"`
	Page          int           `json:"page"`
	PageSize      int           `json:"pageSize"`
	WindowSeconds float64       `json:"windowSeconds"`
	StartedAt     string        `json:"startedAt"`
	EndedAt       string        `json:"endedAt"`
}
