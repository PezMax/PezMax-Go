package kadmin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func datumTreeNodeAt(t *testing.T, nodes []interface{}, labels ...string) map[string]interface{} {
	t.Helper()
	for index, label := range labels {
		var matched map[string]interface{}
		for _, raw := range nodes {
			node, ok := raw.(map[string]interface{})
			if ok && node["label"] == label {
				matched = node
				break
			}
		}
		if matched == nil {
			t.Fatalf("missing tree path %v at %q: %v", labels, label, nodes)
		}
		if index == len(labels)-1 {
			return matched
		}
		if matched["type"] != "folder" {
			t.Fatalf("tree path %v has non-folder at %q: %v", labels, label, matched)
		}
		var ok bool
		nodes, ok = matched["children"].([]interface{})
		if !ok {
			t.Fatalf("tree path %v has invalid children at %q: %v", labels, label, matched)
		}
	}
	t.Fatal("tree path must contain at least one label")
	return nil
}

func datumTreeIDs(t *testing.T, nodes []interface{}) (map[string]string, map[int64]bool) {
	t.Helper()
	folders, files, used := map[string]string{}, map[int64]bool{}, map[string]bool{}
	var walk func([]interface{}, string)
	walk = func(children []interface{}, parent string) {
		for _, raw := range children {
			node := raw.(map[string]interface{})
			path := parent + "/" + node["label"].(string)
			id, ok := node["id"].(string)
			if !ok || id == "" || used[id] {
				t.Fatalf("missing or duplicate tree ID at %s: %v", path, node)
			}
			used[id] = true
			if node["type"] == "file" {
				fileID := int64(node["fileId"].(float64))
				if id != fmt.Sprintf("file-%d", fileID) {
					t.Fatalf("leaf ID = %s, fileId = %d", id, fileID)
				}
				files[fileID] = true
				continue
			}
			if node["type"] != "folder" || !strings.HasPrefix(id, "folder") {
				t.Fatalf("invalid folder at %s: %v", path, node)
			}
			folders[path] = id
			walk(node["children"].([]interface{}), path)
		}
	}
	walk(nodes, "")
	return folders, files
}

func TestDatumTreeRestoresSubjectSchoolHierarchyAndFileMetadata(t *testing.T) {
	store, database := newDatumAuthStore(t, nil)
	for _, file := range []struct {
		id                     int64
		name, subject, school  string
		fileType, year, status int64
		remark                 string
	}{
		{2011, "期末A.pdf", "高等数学", "QLU", 1, 2024, 1, "章节/上"},
		{2012, "旧试卷.pdf", "高等数学", "QLU", 1, 2023, 1, ""},
		{2013, "期中.pdf", "高等数学", "QLU", 2, 2025, 1, ""},
		{2014, "别校试卷.pdf", "高等数学", "另一所大学", 1, 2024, 1, "章节/上"},
		{2015, "物理补考.pdf", "大学物理", "QLU", 3, 2022, 1, ""},
		{2016, "期末B.pdf", "高等数学", "QLU", 1, 2024, 1, " 章节 // 下 / "},
		{2020, "待审核.pdf", "高等数学", "QLU", 1, 2024, 0, ""},
		{2021, "未通过.pdf", "高等数学", "QLU", 1, 2024, 2, ""},
		{2022, "被举报.pdf", "高等数学", "QLU", 1, 2024, 3, ""},
		{2023, "已删除.pdf", "高等数学", "QLU", 1, 2024, 1, ""},
	} {
		seedApprovedFile(database, file.id, 7, file.name, file.subject, file.fileType, file.year, file.status)
		row := database.files[len(database.files)-1]
		row["file_school"], row["remark"] = file.school, file.remark
		if file.id == 2011 {
			row["reviewer"] = "reviewer-1"
			row["create_by"], row["create_time"] = "uploader-7", "2024-05-01 10:30:00"
			row["update_by"], row["update_time"] = "editor-8", "2024-05-02 12:00:00"
		}
		if file.id == 2023 {
			row["del_flag"] = int64(1)
		}
	}
	engine := datumEngine(t, store)
	response := datumJSON(t, engine, http.MethodGet, "/datum/file/tree", "", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("tree status = %d body %s", response.Code, response.Body.String())
	}
	tree := datumBody(t, response)["data"].([]interface{})
	if len(tree) != 2 || tree[0].(map[string]interface{})["label"] != "大学物理" || tree[1].(map[string]interface{})["label"] != "高等数学" {
		t.Fatalf("tree roots must be subjects ordered by name: %v", tree)
	}
	leaf := datumTreeNodeAt(t, tree, "高等数学", "QLU", "期末", "2024", "章节", "上", "期末A.pdf")
	for key, want := range map[string]interface{}{
		"id": "file-2011", "type": "file", "label": "期末A.pdf", "fileName": "期末A.pdf",
		"fileId": float64(2011), "userId": float64(7), "fileSize": float64(1024), "fileYear": float64(2024),
		"fileType": float64(1), "fileStatus": float64(1), "delFlag": float64(0),
		"fileUrl": "http://files.example.com/ptmj/a.pdf", "fileFormat": "pdf", "fileSubject": "高等数学",
		"fileSchool": "QLU", "remark": "章节/上", "reviewer": "reviewer-1",
		"createBy": "uploader-7", "createTime": "2024-05-01 10:30:00",
		"updateBy": "editor-8", "updateTime": "2024-05-02 12:00:00",
	} {
		if got := leaf[key]; got != want {
			t.Errorf("leaf %s = %#v, want %#v", key, got, want)
		}
	}
	for _, labels := range [][]string{
		{"高等数学", "QLU", "期末", "2023", "旧试卷.pdf"},
		{"高等数学", "QLU", "期中", "2025", "期中.pdf"},
		{"高等数学", "另一所大学", "期末", "2024", "章节", "上", "别校试卷.pdf"},
		{"大学物理", "QLU", "补考", "2022", "物理补考.pdf"},
		{"高等数学", "QLU", "期末", "2024", "章节", "下", "期末B.pdf"},
	} {
		datumTreeNodeAt(t, tree, labels...)
	}
	years := datumTreeNodeAt(t, tree, "高等数学", "QLU", "期末")["children"].([]interface{})
	if len(years) != 2 || years[0].(map[string]interface{})["label"] != "2024" || years[1].(map[string]interface{})["label"] != "2023" {
		t.Fatalf("year folders must be newest first: %v", years)
	}
	before, files := datumTreeIDs(t, tree)
	if len(files) != 6 {
		t.Fatalf("tree must only contain 6 approved, non-deleted files: %v", files)
	}
	for _, id := range []int64{2020, 2021, 2022, 2023} {
		if files[id] {
			t.Errorf("unpublished file %d appeared in the tree", id)
		}
	}
	// Adding an earlier subject changes traversal order without changing any
	// existing folder or leaf identity, so desktop expansion state survives.
	seedApprovedFile(database, 2000, 7, "new.pdf", "AAA", 1, 2025, 1)
	store.invalidateDatumTree()
	response = datumJSON(t, engine, http.MethodGet, "/datum/file/tree", "", nil)
	afterTree := datumBody(t, response)["data"].([]interface{})
	after, _ := datumTreeIDs(t, afterTree)
	for path, id := range before {
		if after[path] != id {
			t.Errorf("folder ID at %s changed from %s to %s", path, id, after[path])
		}
	}
}

func TestDatumTreeEmptyPayloadIsArray(t *testing.T) {
	store, _ := newDatumAuthStore(t, nil)
	response := datumJSON(t, datumEngine(t, store), http.MethodGet, "/datum/file/tree", "", nil)
	body := datumBody(t, response)
	if response.Code != http.StatusOK {
		t.Fatalf("empty tree status = %d body %s", response.Code, response.Body.String())
	}
	if tree, ok := body["data"].([]interface{}); !ok || len(tree) != 0 {
		t.Fatalf("empty tree data = %#v, want []", body["data"])
	}
	entry := requireDatumWarmupEntry(t, store.datum.redis, store.treeStateKey())
	if !bytes.Equal(entry.Payload, []byte("[]")) {
		t.Fatalf("empty cached payload = %s, want []", entry.Payload)
	}
}

func TestDatumTreeLegacyCacheMigratesWithoutInvalidatingRank(t *testing.T) {
	for _, trigger := range []string{"startup-warmup", "tree-request"} {
		t.Run(trigger, func(t *testing.T) {
			store, database, redis := newDatumWarmupFixture(t)
			seedApprovedFile(database, 1001, 7, "试卷.pdf", "高等数学", 1, 2024, 1)
			legacyTree := datumStateEntry{Hash: "legacy-tree", Payload: json.RawMessage(`[{"label":"期末","type":"folder","children":[]}]`)}
			legacyRank := datumStateEntry{Hash: "legacy-rank", Payload: json.RawMessage(`[{"userId":7,"userName":"alice","count":3}]`)}
			for key, entry := range map[string]datumStateEntry{store.treeStateKey(): legacyTree, store.rankStateKey(): legacyRank} {
				encoded, err := json.Marshal(entry)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := redis.do("SET", key, string(encoded), "EX", "86400"); err != nil {
					t.Fatal(err)
				}
			}
			engine := datumEngine(t, store)
			if trigger == "startup-warmup" {
				warmer := startDatumStateWarmer(store.warmDatumStates, time.Hour)
				t.Cleanup(warmer.Close)
			}
			response := datumJSON(t, engine, http.MethodGet, "/datum/file/tree?hash="+legacyTree.Hash, "", nil)
			body := datumBody(t, response)
			if response.Code != http.StatusOK || body["unchanged"] != false || treeHashOf(t, body) == legacyTree.Hash {
				t.Fatalf("legacy client must receive a new hash and full tree: %v", body)
			}
			datumTreeNodeAt(t, body["data"].([]interface{}), "高等数学", "QLU", "期末", "2024", "试卷.pdf")
			migrated := requireDatumWarmupEntry(t, redis, store.treeStateKey())
			if migrated.Version != datumTreeStateVersion || redis.callCount("SET", store.treeStateKey()) != 2 {
				t.Fatalf("tree migration version = %d, writes = %d", migrated.Version, redis.callCount("SET", store.treeStateKey()))
			}
			// Once migrated, neither GET nor the maintenance pass should rebuild
			// a healthy current-schema snapshot or change the client's hash.
			response = datumJSON(t, engine, http.MethodGet, "/datum/file/tree?hash="+migrated.Hash, "", nil)
			body = datumBody(t, response)
			if body["unchanged"] != true || treeHashOf(t, body) != migrated.Hash {
				t.Fatalf("current tree cache was not reused: %v", body)
			}
			if err := store.warmDatumStates(time.Minute); err != nil {
				t.Fatal(err)
			}
			if redis.callCount("SET", store.treeStateKey()) != 2 {
				t.Fatal("current-schema tree was rebuilt during maintenance")
			}
			rankResponse := datumJSON(t, engine, http.MethodGet, "/datum/user/rank?hash="+legacyRank.Hash, "", nil)
			if datumBody(t, rankResponse)["unchanged"] != true {
				t.Fatalf("unversioned ranking cache was invalidated: %s", rankResponse.Body.String())
			}
			rank := requireDatumWarmupEntry(t, redis, store.rankStateKey())
			if rank.Version != 0 || rank.Hash != legacyRank.Hash || !bytes.Equal(rank.Payload, legacyRank.Payload) || redis.callCount("SET", store.rankStateKey()) != 1 {
				t.Fatalf("ranking cache changed during tree migration: %#v", rank)
			}
		})
	}
}
