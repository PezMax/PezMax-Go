package kadmin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Apply year ranges in SQL before paging; scanning only the newest catalog
// pages can incorrectly report no older papers even when they exist.
func datumFileSearchYears(c *gin.Context) (int64, int64, bool) {
	var years [2]int64
	for i, key := range []string{"yearFrom", "yearTo"} {
		raw := strings.TrimSpace(c.Query(key))
		if raw == "" {
			continue
		}
		year, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || year < 1900 || year > 2100 {
			fail(c, http.StatusBadRequest, "年份需为 1900 到 2100 之间的整数")
			return 0, 0, false
		}
		years[i] = year
	}
	if years[0] > 0 && years[1] > 0 && years[0] > years[1] {
		fail(c, http.StatusBadRequest, "起始年份不能晚于结束年份")
		return 0, 0, false
	}
	return years[0], years[1], true
}
