package kadmin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/datum"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/storage"
)

type knowledgeManifestDB struct {
	*fakeDatumDB
	t *testing.T
}

func (db *knowledgeManifestDB) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	if !strings.Contains(query, "COUNT(*) OVER() AS total") {
		return db.fakeDatumDB.Query(query, args...)
	}
	for _, guard := range []string{"file_status = 1 AND del_flag = 0", "ebook_status = 1 AND del_flag = 0",
		"file_format IN ('pdf','epub','txt','md')", "ebook_format IN ('pdf','epub')", "LIMIT 200", "BETWEEN 1 AND 16777216"} {
		if !strings.Contains(query, guard) {
			db.t.Fatalf("missing manifest guard: %s", guard)
		}
	}
	return []map[string]interface{}{{"kind": "file", "id": int64(9007199254740993), "title": "Synthetic notes", "format": "pdf",
		"subject": "Software", "school": "Campus", "year": int64(2026), "url": "minio://private-bucket/notes.pdf",
		"size": int64(1024), "updated": "2026-10-27", "total": int64(201)}}, nil
}

func TestDatumKnowledgeManifestAuthAndProjection(t *testing.T) {
	store, db, engine, token := seedActivityFixture(t)
	store.conn = &knowledgeManifestDB{db, t}
	if got := datumJSON(t, engine, http.MethodGet, "/datum/knowledge/manifest", "", nil).Code; got != http.StatusUnauthorized {
		t.Fatalf("anonymous manifest = %d", got)
	}
	response := datumJSON(t, engine, http.MethodGet, "/datum/knowledge/manifest", token, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("manifest = %s", response.Body.String())
	}
	data := datumBody(t, response)["data"].(map[string]interface{})
	row := data["sources"].([]interface{})[0].(map[string]interface{})
	if row["id"] != "9007199254740993" || data["partial"] != true || data["total"] != float64(201) {
		t.Fatalf("manifest projection = %v", data)
	}
	if strings.Contains(response.Body.String(), "private-bucket") || len(row["revision"].(string)) != 64 {
		t.Fatal("source leaked address or omitted revision")
	}
}

func TestDatumKnowledgeSourceContentRevisionAndWithdrawal(t *testing.T) {
	_, db, engine, token := seedActivityFixture(t)
	seedEbook(db, 2001, 7, "Synthetic book", "Author", "epub", "Software", 1)
	previous := datumKnowledgeOpen
	defer func() { datumKnowledgeOpen = previous }()
	opens := 0
	datumKnowledgeOpen = func(_ context.Context, bucket, key string) (io.ReadCloser, storage.ObjectInfo, error) {
		opens++
		return io.NopCloser(strings.NewReader("synthetic document")), storage.ObjectInfo{Size: 18}, nil
	}
	for _, kind := range []string{"file", "ebook"} {
		id := "1001"
		if kind == "ebook" {
			id = "2001"
		}
		base := "/datum/knowledge/"
		params := "?kind=" + kind + "&id=" + id
		response := datumJSON(t, engine, http.MethodGet, base+"source"+params, token, nil)
		if response.Code != http.StatusOK {
			t.Fatalf("source = %s", response.Body.String())
		}
		data := datumBody(t, response)["data"].(map[string]interface{})
		revision := data["revision"].(string)
		response = datumJSON(t, engine, http.MethodGet, base+"content"+params+"&revision="+revision, "", nil)
		if response.Code != http.StatusOK || response.Body.String() != "synthetic document" || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("content = %s", response.Body.String())
		}
		before := opens
		response = datumJSON(t, engine, http.MethodGet, base+"content"+params+"&revision=stale", "", nil)
		if response.Code != http.StatusConflict || opens != before {
			t.Fatal("stale citation opened object")
		}
		if kind == "file" {
			db.files[0]["file_status"] = int64(0)
		} else {
			db.ebooks[0]["ebook_status"] = int64(0)
		}
		response = datumJSON(t, engine, http.MethodGet, base+"content"+params, token, nil)
		if response.Code != http.StatusNotFound || opens != before {
			t.Fatal("withdrawn source readable")
		}
	}
	if len(db.downloads) != 0 || db.ebookDownloads != 0 {
		t.Fatal("indexing polluted download history")
	}
}

func TestDatumKnowledgeRevisionIncludesMetadataAndContentAddress(t *testing.T) {
	source := datum.KnowledgeSource{Kind: "file", ID: "1", Title: "Notes", Format: "pdf", URL: "minio://a/1.pdf", Size: 12}
	initial := source.Versioned().Revision
	for _, mutate := range []func(*datum.KnowledgeSource){
		func(s *datum.KnowledgeSource) { s.Title = "New title" },
		func(s *datum.KnowledgeSource) { s.URL = "minio://a/2.pdf" },
		func(s *datum.KnowledgeSource) { s.Updated = "2026-10-27" },
		func(s *datum.KnowledgeSource) { s.Subject = "Software" },
	} {
		changed := source
		mutate(&changed)
		if changed.Versioned().Revision == initial {
			t.Fatal("revision unchanged")
		}
	}
	encoded, _ := json.Marshal(source.Versioned())
	if strings.Contains(string(encoded), "minio:") {
		t.Fatal("source projection exposed object address")
	}
}

func TestDatumKnowledgeContentEnforcesActualObjectLimit(t *testing.T) {
	_, _, engine, _ := seedActivityFixture(t)
	previous := datumKnowledgeOpen
	defer func() { datumKnowledgeOpen = previous }()
	datumKnowledgeOpen = func(_ context.Context, _, _ string) (io.ReadCloser, storage.ObjectInfo, error) {
		return io.NopCloser(strings.NewReader("")), storage.ObjectInfo{Size: datumKnowledgeMaxBytes + 1}, nil
	}
	response := datumJSON(t, engine, http.MethodGet, "/datum/knowledge/content?kind=file&id=1001", "", nil)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize = %d", response.Code)
	}
}
