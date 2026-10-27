package datum

import (
	"reflect"
	"strings"
	"testing"
)

func TestFileSearchRangesApplyBeforePaginationAndKeepVisibility(t *testing.T) {
	conn := newScriptedConn()
	queries := 0
	conn.onSQL(func(query string, args []interface{}) ([]map[string]interface{}, error) {
		queries++
		for _, clause := range []string{"del_flag = 0", "file_status = 1", "file_year >= ?", "file_year <= ?"} {
			if !strings.Contains(query, clause) {
				t.Fatalf("missing visibility or range condition %s: %s", clause, query)
			}
		}
		want := []interface{}{int64(2022), int64(2024), "%数学%", "%数学%", "%数学%"}
		if strings.Contains(query, "count(*)") {
			if !reflect.DeepEqual(args, want) {
				t.Fatalf("count parameters = %#v, want %#v", args, want)
			}
			return []map[string]interface{}{{"count": int64(1)}}, nil
		}
		want = append(want, 5, 0)
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("page parameters = %#v, want %#v", args, want)
		}
		return []map[string]interface{}{{"file_id": int64(7), "file_year": int64(2023), "file_status": int64(1)}}, nil
	}, "FROM ptmj_file")
	page, err := NewFileRepo(conn).List(FileFilter{OnlyApproved: true, Keyword: "数学", YearFrom: 2022, YearTo: 2024, PageSize: 5})
	if err != nil || page.Total != 1 || queries != 2 || len(page.Items.([]File)) != 1 {
		t.Fatalf("range search = %#v, err %v, queries %d", page, err, queries)
	}
}
