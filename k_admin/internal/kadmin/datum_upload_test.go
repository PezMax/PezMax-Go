package kadmin

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
