package datum

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

// KnowledgeSource is a projection of a live public catalog row, never a URL
// supplied by a model. Revision changes when its content address or metadata do.
type KnowledgeSource struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Format   string `json:"format"`
	Subject  string `json:"subject"`
	School   string `json:"school"`
	Year     int64  `json:"year"`
	Revision string `json:"revision"`
	URL      string `json:"-"`
	Size     int64  `json:"-"`
	Updated  string `json:"-"`
}

func (s KnowledgeSource) Versioned() KnowledgeSource {
	encoded, _ := json.Marshal([]interface{}{s.Kind, s.ID, s.Title, s.Format, s.Subject, s.School, s.Year, s.URL, s.Size, s.Updated})
	digest := sha256.Sum256(encoded)
	s.Revision = hex.EncodeToString(digest[:])
	return s
}

func KnowledgeManifest(conn db.Connection) ([]KnowledgeSource, int64, error) {
	// A bounded window over the business catalog. Public status is enforced in
	// SQL before counting/paging; neither pending nor deleted rows are indexed.
	rows, err := conn.Query(`SELECT *, COUNT(*) OVER() AS total FROM (
		SELECT 'file' AS kind, file_id AS id, file_name AS title, file_format AS format,
		file_subject AS subject, file_school AS school, file_year AS year,
		file_url AS url, file_size AS size, update_time AS updated
		FROM ptmj_file WHERE file_status = 1 AND del_flag = 0
		AND file_format IN ('pdf','epub','txt','md') AND file_size BETWEEN 1 AND 16777216
		UNION ALL
		SELECT 'ebook' AS kind, ebook_id AS id, ebook_name AS title, ebook_format AS format,
		ebook_subject AS subject, '' AS school, 0 AS year,
		ebook_url AS url, ebook_size AS size, update_time AS updated
		FROM ptmj_ebook WHERE ebook_status = 1 AND del_flag = 0
		AND ebook_format IN ('pdf','epub') AND ebook_size BETWEEN 1 AND 16777216
	) AS sources ORDER BY updated DESC NULLS LAST, kind, id DESC LIMIT 200`)
	if err != nil {
		return nil, 0, err
	}
	sources := make([]KnowledgeSource, 0, len(rows))
	var total int64
	for _, row := range rows {
		total = ScanInt64(row["total"])
		sources = append(sources, (KnowledgeSource{Kind: ScanString(row["kind"]), ID: ScanString(row["id"]),
			Title: ScanString(row["title"]), Format: ScanString(row["format"]), Subject: ScanString(row["subject"]),
			School: ScanString(row["school"]), Year: ScanInt64(row["year"]), URL: ScanString(row["url"]),
			Size: ScanInt64(row["size"]), Updated: ScanString(row["updated"])}).Versioned())
	}
	return sources, total, nil
}
