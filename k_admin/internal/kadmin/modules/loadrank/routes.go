package loadrank

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/transport/httpx"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	Connection        db.Connection
	RequireAuth       gin.HandlerFunc
	RequirePermission func(...string) gin.HandlerFunc
}

func Register(api *gin.RouterGroup, dependencies Dependencies) (*Sampler, error) {
	if err := EnsureSchema(dependencies.Connection); err != nil {
		return nil, err
	}
	sampler, err := NewSampler(SamplerOptions{
		Settings: &settingsRepository{conn: dependencies.Connection},
		Buckets:  &bucketRepository{conn: dependencies.Connection},
	})
	if err != nil {
		return nil, err
	}
	RegisterRoutes(api, sampler, dependencies)
	return sampler, nil
}

func RegisterRoutes(api *gin.RouterGroup, sampler *Sampler, dependencies Dependencies) {
	group := api.Group("/load-ranking", dependencies.RequireAuth)
	group.GET("/status", dependencies.RequirePermission(ViewPermission), func(c *gin.Context) {
		httpx.Success(c, sampler.Status())
	})
	group.PATCH("/status", dependencies.RequirePermission(UpdatePermission), func(c *gin.Context) {
		var payload struct {
			Enabled *bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil || payload.Enabled == nil {
			httpx.Fail(c, http.StatusBadRequest, "invalid load ranking sampling status")
			return
		}
		status, err := sampler.SetEnabled(*payload.Enabled, loadRankCurrentUserID(c))
		if err != nil {
			httpx.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.Success(c, status)
	})
	group.GET("/rankings", dependencies.RequirePermission(ViewPermission), func(c *gin.Context) {
		query, err := parseRankingQuery(c)
		if err != nil {
			httpx.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		rows, err := sampler.buckets.QueryBuckets(bucketQueryFilter{
			StartedAt:  query.StartedAt,
			EndedAt:    query.EndedAt,
			Route:      query.Route,
			Method:     query.Method,
			StatusCode: query.StatusCode,
		})
		if err != nil {
			httpx.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		items, windowSeconds := aggregateRows(rows, query.GroupBy)
		items = sortRankingItems(items, query.Dimension, query.Order)
		response := sliceRankingPage(items, query.Page, query.PageSize)
		response.WindowSeconds = windowSeconds
		response.StartedAt = query.StartedAt.Format(time.RFC3339)
		response.EndedAt = query.EndedAt.Format(time.RFC3339)
		httpx.Success(c, response)
	})
}

func loadRankCurrentUserID(c *gin.Context) int64 {
	value, exists := c.Get("vben_user")
	if !exists {
		return 0
	}
	user, ok := value.(models.UserModel)
	if !ok {
		return 0
	}
	return user.Id
}

// rankingQuery is the parsed ranking request. The time range is always
// populated so every summary-table scan is bounded.
type rankingQuery struct {
	Page       int
	PageSize   int
	Route      string
	Method     string
	StatusCode *int64
	GroupBy    string
	Dimension  string
	Order      string
	StartedAt  time.Time
	EndedAt    time.Time
}

var rankingGroupBys = map[string]bool{"route": true, "method": true, "status": true}
var rankingDimensions = map[string]bool{
	"requestCount": true, "qps": true, "errorRate": true, "avgDurationMs": true,
}

func parseRankingQuery(c *gin.Context) (rankingQuery, error) {
	query := rankingQuery{
		Page:      positiveRankingInt(c, "page", 1),
		PageSize:  positiveRankingInt(c, "pageSize", defaultPageSize),
		Route:     truncateRoute(c.Query("route")),
		Method:    strings.ToUpper(strings.TrimSpace(c.Query("method"))),
		GroupBy:   strings.TrimSpace(c.Query("groupBy")),
		Dimension: strings.TrimSpace(c.Query("dimension")),
		Order:     strings.ToLower(strings.TrimSpace(c.Query("order"))),
	}
	if query.PageSize > maxPageSize {
		query.PageSize = maxPageSize
	}
	if query.Method != "" && len(query.Method) > 16 {
		return query, errorsForRankingQuery("method")
	}
	if query.GroupBy == "" {
		query.GroupBy = "route"
	}
	if !rankingGroupBys[query.GroupBy] {
		return query, errorsForRankingQuery("groupBy")
	}
	if query.Dimension == "" {
		query.Dimension = "requestCount"
	}
	if !rankingDimensions[query.Dimension] {
		return query, errorsForRankingQuery("dimension")
	}
	if query.Order == "" {
		query.Order = "desc"
	}
	if query.Order != "asc" && query.Order != "desc" {
		return query, errorsForRankingQuery("order")
	}
	if value := strings.TrimSpace(c.Query("statusCode")); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 100 || parsed > 599 {
			return query, errorsForRankingQuery("statusCode")
		}
		query.StatusCode = &parsed
	}

	now := time.Now()
	var err error
	if query.StartedAt, err = parseRankingTime(c.Query("startedAt"), now.Add(-time.Hour)); err != nil {
		return query, errorsForRankingQuery("startedAt")
	}
	if query.EndedAt, err = parseRankingTime(c.Query("endedAt"), now); err != nil {
		return query, errorsForRankingQuery("endedAt")
	}
	if !query.EndedAt.After(query.StartedAt) {
		return query, fmt.Errorf("endedAt must be after startedAt")
	}
	if query.EndedAt.Sub(query.StartedAt) > maxQueryWindow {
		return query, fmt.Errorf("time range must not exceed %d days", int(maxQueryWindow/(24*time.Hour)))
	}
	return query, nil
}

func errorsForRankingQuery(field string) error {
	return fmt.Errorf("invalid %s", field)
}

func parseRankingTime(value string, fallback time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func positiveRankingInt(c *gin.Context, key string, fallback int) int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

// aggregateRows groups summary-table rows by the requested dimension and
// computes the statistics. Definitions (统计口径):
//
//   - requestCount = sum of request_count in the window
//   - errorCount   = sum of error_count; an error is a status >= 400
//   - errorRate    = errorCount / requestCount, 0 when requestCount is 0
//   - avgDurationMs = sum(total_duration_ms) / requestCount, 0 when 0
//   - qps          = requestCount / windowSeconds, where windowSeconds is the
//     span between the first and the last bucket boundary of the filtered
//     result (at least one bucket, i.e. bucketSize seconds); 0 when no rows
func aggregateRows(rows []bucketRow, groupBy string) ([]RankingItem, float64) {
	windowSeconds := 0.0
	if len(rows) > 0 {
		first := rows[0].bucketStart
		last := rows[0].bucketStart
		for _, row := range rows {
			if row.bucketStart.Before(first) {
				first = row.bucketStart
			}
			if row.bucketStart.After(last) {
				last = row.bucketStart
			}
		}
		windowSeconds = last.Add(bucketSize).Sub(first).Seconds()
	}

	groups := make(map[string]*RankingItem)
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		key, item := rankingGroupKey(row, groupBy)
		grouped := groups[key]
		if grouped == nil {
			groups[key] = &item
			order = append(order, key)
			continue
		}
		grouped.RequestCount += row.requestCount
		grouped.ErrorCount += row.errorCount
		grouped.AvgDurationMs += float64(row.totalDurationMs)
		if row.maxDurationMs > grouped.MaxDurationMs {
			grouped.MaxDurationMs = row.maxDurationMs
		}
	}

	items := make([]RankingItem, 0, len(groups))
	for _, key := range order {
		item := groups[key]
		if item.RequestCount > 0 {
			item.ErrorRate = float64(item.ErrorCount) / float64(item.RequestCount)
			item.AvgDurationMs = item.AvgDurationMs / float64(item.RequestCount)
		}
		if windowSeconds > 0 {
			item.QPS = float64(item.RequestCount) / windowSeconds
		}
		items = append(items, *item)
	}
	return items, windowSeconds
}

func rankingGroupKey(row bucketRow, groupBy string) (string, RankingItem) {
	base := RankingItem{
		RequestCount:  row.requestCount,
		ErrorCount:    row.errorCount,
		AvgDurationMs: float64(row.totalDurationMs),
		MaxDurationMs: row.maxDurationMs,
	}
	switch groupBy {
	case "method":
		base.Method = row.method
		return row.method, base
	case "status":
		status := row.statusCode
		base.StatusCode = &status
		return strconv.FormatInt(status, 10), base
	default:
		base.Route = row.route
		return row.route, base
	}
}

func sortRankingItems(items []RankingItem, dimension string, order string) []RankingItem {
	sorted := append([]RankingItem(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, right := rankingValue(sorted[i], dimension), rankingValue(sorted[j], dimension)
		if left != right {
			if order == "asc" {
				return left < right
			}
			return left > right
		}
		return rankingTieBreak(sorted[i]) < rankingTieBreak(sorted[j])
	})
	return sorted
}

func rankingValue(item RankingItem, dimension string) float64 {
	switch dimension {
	case "qps":
		return item.QPS
	case "errorRate":
		return item.ErrorRate
	case "avgDurationMs":
		return item.AvgDurationMs
	default:
		return float64(item.RequestCount)
	}
}

func rankingTieBreak(item RankingItem) string {
	switch {
	case item.Route != "":
		return item.Route
	case item.Method != "":
		return item.Method
	default:
		return ""
	}
}

func sliceRankingPage(items []RankingItem, page int, pageSize int) RankingResponse {
	start := (page - 1) * pageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return RankingResponse{
		Items:    items[start:end],
		Total:    int64(len(items)),
		Page:     page,
		PageSize: pageSize,
	}
}
