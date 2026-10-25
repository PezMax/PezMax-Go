package main

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPublicPDFUploadsSupportCredentialFreeTextReads(t *testing.T) {
	router := gin.New()
	router.Any("/uploads/:file", immutableUploadsCache, func(c *gin.Context) { c.String(http.StatusOK, "synthetic public resource") })
	for _, test := range []struct {
		method, file, origin string
		allowed              bool
	}{
		{http.MethodGet, "fixture.pdf", "http://localhost", true},
		{http.MethodHead, "fixture.PDF", "null", true},
		{http.MethodGet, "fixture.pdf", "", false},
		{http.MethodGet, "fixture.png", "http://localhost", false},
		{http.MethodPost, "fixture.pdf", "http://localhost", false},
	} {
		request := httptest.NewRequest(test.method, "/uploads/"+test.file, nil)
		request.Header.Set("Origin", test.origin)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if (response.Header().Get("Access-Control-Allow-Origin") == "*") != test.allowed {
			t.Fatalf("unexpected public PDF CORS header for %+v", test)
		}
		if response.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Fatal("public file reads must not enable credentials")
		}
		if !strings.Contains(response.Header().Get("Cache-Control"), "immutable") {
			t.Fatal("static caching was lost")
		}
	}
}

func TestListenHTTPReportsAddressInUse(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve test address: %v", err)
	}
	defer occupied.Close()

	listener, err := listenHTTP(occupied.Addr().String())
	if listener != nil {
		_ = listener.Close()
	}
	if err == nil {
		t.Fatal("expected an address-in-use error")
	}
	if !strings.Contains(err.Error(), "HTTP 服务监听") {
		t.Fatalf("unexpected startup error: %v", err)
	}
}

func TestSetupBackendConvertsInitializationPanicToError(t *testing.T) {
	_, _, err := setupBackend(nil, nil, nil)
	if err == nil {
		t.Fatal("expected initialization error")
	}
	if !strings.Contains(err.Error(), "初始化 GoAdmin 公共组件失败") {
		t.Fatalf("unexpected initialization error: %v", err)
	}
}

func TestGinDebugErrorWriterOnlyFiltersNonErrorDebugMessages(t *testing.T) {
	var output bytes.Buffer
	writer := ginDebugErrorWriter{destination: &output}

	messages := []string{
		"[GIN-debug] [WARNING] debug mode\n",
		"[GIN-debug] GET /api\n",
		"[GIN-debug] [ERROR] listen failed\n",
		"KAdmin 后端服务启动成功\n",
	}
	for _, message := range messages {
		written, err := writer.Write([]byte(message))
		if err != nil {
			t.Fatalf("write message: %v", err)
		}
		if written != len(message) {
			t.Fatalf("written bytes = %d, want %d", written, len(message))
		}
	}

	want := "[GIN-debug] [ERROR] listen failed\nKAdmin 后端服务启动成功\n"
	if output.String() != want {
		t.Fatalf("filtered output = %q, want %q", output.String(), want)
	}
}

func TestLoadEnvironmentReadsFileWithoutOverridingProcess(t *testing.T) {
	const loadedKey = "KADMIN_TEST_ENV_LOADED"
	previousValue, previouslySet := os.LookupEnv(loadedKey)
	if err := os.Unsetenv(loadedKey); err != nil {
		t.Fatalf("unset test environment: %v", err)
	}
	t.Cleanup(func() {
		if previouslySet {
			_ = os.Setenv(loadedKey, previousValue)
			return
		}
		_ = os.Unsetenv(loadedKey)
	})
	t.Setenv("KADMIN_TEST_ENV_EXISTING", "process")

	path := filepath.Join(t.TempDir(), ".env")
	content := loadedKey + "=file\nKADMIN_TEST_ENV_EXISTING=file\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test environment: %v", err)
	}
	if err := loadEnvironment(path); err != nil {
		t.Fatalf("load environment: %v", err)
	}
	if got := os.Getenv(loadedKey); got != "file" {
		t.Fatalf("loaded value = %q, want file", got)
	}
	if got := os.Getenv("KADMIN_TEST_ENV_EXISTING"); got != "process" {
		t.Fatalf("existing value = %q, want process", got)
	}
}

func TestLoadEnvironmentAllowsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := loadEnvironment(path); err != nil {
		t.Fatalf("missing environment file should be optional: %v", err)
	}
}

func TestValidateCredentials(t *testing.T) {
	for _, test := range []struct {
		name     string
		key      string
		value    string
		wantFail bool
	}{
		{name: "valid"},
		{name: "missing database password", key: "KADMIN_DB_PASSWORD", wantFail: true},
		{name: "missing redis password", key: "KADMIN_REDIS_PASSWORD", wantFail: true},
		{name: "short jwt secret", key: "KADMIN_JWT_SECRET", value: "short", wantFail: true},
		{name: "missing minio secret", key: "KADMIN_MINIO_SECRET_KEY", wantFail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("KADMIN_DB_PASSWORD", "database-password")
			t.Setenv("KADMIN_REDIS_PASSWORD", "redis-password")
			t.Setenv("KADMIN_JWT_SECRET", strings.Repeat("x", 32))
			t.Setenv("KADMIN_MINIO_ENABLED", "true")
			t.Setenv("KADMIN_MINIO_ACCESS_KEY", "minio-user")
			t.Setenv("KADMIN_MINIO_SECRET_KEY", "minio-password")
			if test.key != "" {
				t.Setenv(test.key, test.value)
			}
			if err := validateCredentials(); (err != nil) != test.wantFail {
				t.Fatalf("validateCredentials() error = %v, want failure %v", err, test.wantFail)
			}
		})
	}
}
