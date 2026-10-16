package kadmin

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newGzipTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GzipMiddleware())
	router.GET("/json", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": strings.Repeat("pezmax", 100)})
	})
	router.GET("/binary", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/octet-stream", []byte(strings.Repeat("\x00", 2048)))
	})
	router.GET("/text", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(strings.Repeat("a", 2048)))
	})
	return router
}

func doGzipRequest(t *testing.T, router *gin.Engine, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestGzipMiddlewareCompressesJSON(t *testing.T) {
	router := newGzipTestRouter(t)
	recorder := doGzipRequest(t, router, "/json", map[string]string{"Accept-Encoding": "gzip"})

	if encoding := recorder.Header().Get("Content-Encoding"); encoding != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", encoding)
	}
	if length := recorder.Header().Get("Content-Length"); length != "" {
		t.Fatalf("Content-Length should be dropped when compressing, got %q", length)
	}
	if vary := recorder.Header().Get("Vary"); !strings.Contains(vary, "Accept-Encoding") {
		t.Fatalf("Vary header missing Accept-Encoding, got %q", vary)
	}
	reader, err := gzip.NewReader(recorder.Body)
	if err != nil {
		t.Fatalf("open gzip body: %v", err)
	}
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if !strings.Contains(string(decompressed), "pezmax") {
		t.Fatalf("decompressed payload corrupted: %q", string(decompressed[:64]))
	}
	if recorder.Body.Len() >= len(decompressed) {
		t.Fatalf("response was not actually compressed: %d >= %d", recorder.Body.Len(), len(decompressed))
	}
}

func TestGzipMiddlewarePassthroughWithoutAcceptEncoding(t *testing.T) {
	router := newGzipTestRouter(t)
	recorder := doGzipRequest(t, router, "/json", nil)

	if encoding := recorder.Header().Get("Content-Encoding"); encoding != "" {
		t.Fatalf("Content-Encoding should be empty, got %q", encoding)
	}
	if !strings.Contains(recorder.Body.String(), "pezmax") {
		t.Fatal("identity body should be intact")
	}
}

func TestGzipMiddlewareSkipsBinaryContentType(t *testing.T) {
	router := newGzipTestRouter(t)
	recorder := doGzipRequest(t, router, "/binary", map[string]string{"Accept-Encoding": "gzip"})

	if encoding := recorder.Header().Get("Content-Encoding"); encoding != "" {
		t.Fatalf("binary responses must not be compressed, got %q", encoding)
	}
	if !strings.Contains(recorder.Body.String(), "\x00") {
		t.Fatal("binary body should be passed through untouched")
	}
}

func TestGzipMiddlewareSkipsRangeRequests(t *testing.T) {
	router := newGzipTestRouter(t)
	recorder := doGzipRequest(t, router, "/text", map[string]string{
		"Accept-Encoding": "gzip",
		"Range":           "bytes=0-99",
	})

	if encoding := recorder.Header().Get("Content-Encoding"); encoding != "" {
		t.Fatalf("range responses must not be compressed, got %q", encoding)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (gin ignores range itself)", recorder.Code)
	}
}

func TestGzipMiddlewareSkipsSmallKnownLength(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GzipMiddleware())
	router.GET("/tiny", func(c *gin.Context) {
		c.Header("Content-Length", "2")
		c.Data(http.StatusOK, "text/plain", []byte("ok"))
	})
	recorder := doGzipRequest(t, router, "/tiny", map[string]string{"Accept-Encoding": "gzip"})

	if encoding := recorder.Header().Get("Content-Encoding"); encoding != "" {
		t.Fatalf("tiny responses must not be compressed, got %q", encoding)
	}
	if recorder.Body.String() != "ok" {
		t.Fatalf("body = %q, want %q", recorder.Body.String(), "ok")
	}
}

func TestGzipMiddlewareNegotiatesEncodingQuality(t *testing.T) {
	router := newGzipTestRouter(t)
	for _, tc := range []struct {
		name, encoding string
		compressed     bool
	}{
		{"explicitly disabled", "gzip;q=0", false},
		{"disabled despite wildcard", "gzip;q=0, *;q=1", false},
		{"positive quality", "br, gzip;q=0.5", true},
		{"wildcard", "br, *;q=0.5", true},
		{"disabled wildcard", "*;q=0", false},
		{"invalid quality", "gzip;q=invalid", false},
		{"quality outside range", "gzip;q=2", false},
		{"mixed casing", "GZIP; Q=0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := doGzipRequest(t, router, "/text", map[string]string{"Accept-Encoding": tc.encoding})
			if got := recorder.Header().Get("Content-Encoding") == "gzip"; got != tc.compressed {
				t.Fatalf("Accept-Encoding %q: compressed = %v, want %v", tc.encoding, got, tc.compressed)
			}
			if !tc.compressed && recorder.Body.String() != strings.Repeat("a", 2048) {
				t.Fatal("identity response should remain readable")
			}
		})
	}
}

func TestGzipMiddlewareVariesIdentityResponses(t *testing.T) {
	router := newGzipTestRouter(t)
	for _, encoding := range []string{"", "identity", "gzip;q=0"} {
		recorder := doGzipRequest(t, router, "/text", map[string]string{"Accept-Encoding": encoding})
		if !strings.Contains(recorder.Header().Get("Vary"), "Accept-Encoding") {
			t.Fatalf("Accept-Encoding %q: identity response must vary by Accept-Encoding", encoding)
		}
	}
}
