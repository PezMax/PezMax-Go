package main

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
