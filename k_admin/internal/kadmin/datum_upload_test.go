package kadmin

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDatumMultipartFileLimitsEntireRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name    string
		padding int
		status  int
	}{
		{name: "small request", status: http.StatusOK},
		{name: "large metadata", padding: 2 << 20, status: http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			var payload bytes.Buffer
			writer := multipart.NewWriter(&payload)
			part, err := writer.CreateFormFile("file", "small.txt")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := part.Write([]byte("ok")); err != nil {
				t.Fatal(err)
			}
			if test.padding > 0 {
				if err := writer.WriteField("metadata", strings.Repeat("x", test.padding)); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/upload", bytes.NewReader(payload.Bytes()))
			request.Header.Set("Content-Type", writer.FormDataContentType())
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = request
			file, ok := datumMultipartFile(context, 1<<20, "文件不能为空")
			cleanupDatumMultipart(context)
			if recorder.Code != test.status || ok != (test.status == http.StatusOK) {
				t.Fatalf("status = %d, file valid = %v", recorder.Code, ok)
			}
			if ok && file.Size != 2 {
				t.Fatalf("file size = %d", file.Size)
			}
		})
	}
}

func TestDatumFileConverterIsSharedPerStore(t *testing.T) {
	store := &Store{}
	if store.fileConverter() != store.fileConverter() {
		t.Fatal("converter must share one concurrency limit across uploads")
	}
}

func TestValidDatumUTF8AcrossReadBoundary(t *testing.T) {
	valid := strings.Repeat("a", (64<<10)-1) + "世" + "界"
	if ok, err := validDatumUTF8(strings.NewReader(valid)); err != nil || !ok {
		t.Fatalf("valid UTF-8 rejected: valid=%v err=%v", ok, err)
	}
	invalid := strings.Repeat("a", (64<<10)-1) + string([]byte{0xe4, 0xff})
	if ok, err := validDatumUTF8(strings.NewReader(invalid)); err != nil || ok {
		t.Fatalf("invalid UTF-8 accepted: valid=%v err=%v", ok, err)
	}
}

type failingUploadMetadataDB struct {
	*fakeDatumDB
	insertTable string
	failAvatar  bool
}

func (d *failingUploadMetadataDB) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	if d.insertTable != "" && strings.Contains(query, "INSERT INTO "+d.insertTable) {
		return nil, errors.New("metadata unavailable")
	}
	return d.fakeDatumDB.Query(query, args...)
}

func (d *failingUploadMetadataDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	if d.failAvatar && strings.Contains(query, "UPDATE ptmj_user SET avatar") {
		return nil, errors.New("metadata unavailable")
	}
	return d.fakeDatumDB.Exec(query, args...)
}

func TestDatumUploadCleansObjectAfterMetadataFailure(t *testing.T) {
	for _, test := range []struct {
		name, route, table, filename string
		fields                       map[string]string
	}{
		{"file", "/datum/file", "ptmj_file", "notes.txt", map[string]string{
			"fileName": "notes.txt", "fileSubject": "math", "fileYear": "2024", "fileType": "1",
		}},
		{"ebook", "/datum/ebook/upload", "ptmj_ebook", "book.epub", map[string]string{
			"ebookSubject": "math", "ebookType": "1",
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, database, engine, token := seedActivityFixture(t)
			store.conn = &failingUploadMetadataDB{fakeDatumDB: database, insertTable: test.table}
			originalPut, originalDelete := datumFilePut, datumFileDeleteObject
			var putKey, deletedKey string
			datumFilePut = func(_ context.Context, key string, body io.Reader, size int64, _ string) (string, error) {
				putKey = key
				data, err := io.ReadAll(body)
				if err != nil || int64(len(data)) != size {
					t.Fatalf("uploaded stream size = %d, expected %d, error %v", len(data), size, err)
				}
				return "http://files.example.com/ptmj/" + key, nil
			}
			datumFileDeleteObject = func(_ context.Context, key string) error {
				deletedKey = key
				return nil
			}
			t.Cleanup(func() { datumFilePut, datumFileDeleteObject = originalPut, originalDelete })

			var payload bytes.Buffer
			writer := multipart.NewWriter(&payload)
			part, err := writer.CreateFormFile("file", test.filename)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = part.Write([]byte("valid upload"))
			for key, value := range test.fields {
				_ = writer.WriteField(key, value)
			}
			_ = writer.Close()
			request := httptest.NewRequest(http.MethodPost, test.route, bytes.NewReader(payload.Bytes()))
			request.Header.Set("Content-Type", writer.FormDataContentType())
			request.Header.Set("Authorization", "Bearer "+token)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusInternalServerError || putKey == "" || deletedKey != putKey {
				t.Fatalf("status=%d put=%q deleted=%q", recorder.Code, putKey, deletedKey)
			}
		})
	}
}

func TestDatumAvatarUploadRemovesLocalFileAfterMetadataFailure(t *testing.T) {
	store, database, engine, token := seedActivityFixture(t)
	store.conn = &failingUploadMetadataDB{fakeDatumDB: database, failAvatar: true}
	root := t.TempDir()
	t.Setenv("KADMIN_MINIO_ENABLED", "false")
	t.Setenv("KADMIN_DATUM_AVATAR_LOCAL_ROOT", root)
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	part, err := writer.CreateFormFile("file", "avatar.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/datum/desktop/user/profile/avatar/upload", bytes.NewReader(payload.Bytes()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("avatar status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return errors.New("orphan avatar remains")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
