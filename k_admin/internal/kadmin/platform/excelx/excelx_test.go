package excelx

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/360EntSecGroup-Skylar/excelize"
)

// ptmj_fileHeaders 对齐原 RuoYi 导出列结构：实体字段顺序（ptmj_file 表 DDL 顺序）。
var ptmjFileHeaders = []string{
	"file_id", "user_id", "file_name", "file_url", "file_size", "file_format",
	"file_year", "file_type", "file_school", "file_subject", "reviewer",
	"file_status", "del_flag", "create_by", "create_time", "update_by",
	"update_time", "remark",
}

func TestBuildSingleSheetRoundTrip(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 30, 0, 0, time.Local)
	payload, err := Build(Sheet{
		Headers: ptmjFileHeaders,
		Rows: [][]interface{}{
			{int64(1007), int64(1), "试卷.docx", "http://minio:9000/ptmj/x.docx", int64(204800), "docx", int64(2024), int64(1), "QLU", "高数", "", int64(1), int64(0), "uploader", "2024-06-01 10:00:00", "", nil, "期末/final"},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(payload) == 0 || !bytes.HasPrefix(payload, []byte("PK")) {
		t.Fatalf("payload is not an xlsx file (%d bytes)", len(payload))
	}
	file, err := excelize.OpenReader(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	rows := file.GetRows("Sheet1")
	if len(rows) != 2 {
		t.Fatalf("row count = %d, want 2", len(rows))
	}
	if strings.Join(rows[0], ",") != strings.Join(ptmjFileHeaders, ",") {
		t.Fatalf("header row = %v", rows[0])
	}
	if rows[1][2] != "试卷.docx" || rows[1][9] != "高数" {
		t.Fatalf("data row mismatch: %v", rows[1])
	}
	if ExportFilename("试卷导出", now) != "试卷导出_20260920123000.xlsx" {
		t.Fatalf("unexpected export filename %s", ExportFilename("试卷导出", now))
	}
	if MIMEType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("unexpected mime %s", MIMEType)
	}
}

func TestBuildMultipleNamedSheets(t *testing.T) {
	payload, err := Build(
		Sheet{Name: "文件", Headers: []string{"file_id"}, Rows: [][]interface{}{{int64(1)}}},
		Sheet{Name: "下载", Headers: []string{"download_id"}, Rows: [][]interface{}{{int64(9)}}},
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	file, err := excelize.OpenReader(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if rows := file.GetRows("下载"); len(rows) != 2 || rows[1][0] != "9" {
		t.Fatalf("second sheet rows = %v", rows)
	}
}

func TestBuildRejectsBadSheets(t *testing.T) {
	if _, err := Build(); err == nil {
		t.Fatal("empty sheets should fail")
	}
	if _, err := Build(Sheet{Name: "s"}); err == nil {
		t.Fatal("missing headers should fail")
	}
	if _, err := Build(Sheet{Headers: []string{"a"}, Rows: [][]interface{}{{1, 2, 3}}}); err == nil {
		t.Fatal("row wider than headers should fail")
	}
	if _, err := Build(
		Sheet{Headers: []string{"a"}},
		Sheet{Headers: []string{"a"}},
	); err == nil {
		t.Fatal("unnamed second sheet should fail")
	}
}
