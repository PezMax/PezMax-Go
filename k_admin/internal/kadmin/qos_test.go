package kadmin

import (
	"bytes"
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/gin-gonic/gin"
)

func TestQoSBulkPathClassification(t *testing.T) {
	bulk := []string{
		"/datum/download/file",     // 试卷/电子书统一下载
		"/api/files/5/content",     // 托管文件内容
		"/api/uploads/2026/a.png",  // 头像/封面
		"/uploads/2026/01/x.jpg",   // 控制台上传物
	}
	interactive := []string{
		"/datum/file/tree",
		"/datum/file/list",
		"/datum/ebook/list",
		"/datum/user/rank",
		"/system/notification/user/scroll",
		"/admin/",
		"/swagger/index.html",
		"/api/users/export",
	}
	for _, path := range bulk {
		if !qosBulkPath(path) {
			t.Fatalf("%q should be bulk", path)
		}
	}
	for _, path := range interactive {
		if qosBulkPath(path) {
			t.Fatalf("%q should be interactive", path)
		}
	}
}

func TestQoSSetupSemantics(t *testing.T) {
	defer activeEgressQoS.Store(nil)

	// 未声明 + 开发 → 默认关闭
	t.Setenv("KADMIN_QOS_UPLINK_MBPS", "")
	if enabled, _ := SetupEgressQoS(true); enabled {
		t.Fatal("dev mode should default off")
	}
	if activeEgressQoS.Load() != nil {
		t.Fatal("disabled QoS must be disarmed")
	}

	// 未声明 + 生产 → 默认按 10Mbps 启用
	if enabled, _ := SetupEgressQoS(false); !enabled {
		t.Fatal("production should default on")
	}

	// 显式 0 → 任何模式都强制关闭
	t.Setenv("KADMIN_QOS_UPLINK_MBPS", "0")
	if enabled, _ := SetupEgressQoS(false); enabled {
		t.Fatal("explicit 0 must disable")
	}

	// 显式声明在开发模式同样生效：5Mbps × 80% = 500KB/s
	t.Setenv("KADMIN_QOS_UPLINK_MBPS", "5")
	t.Setenv("KADMIN_QOS_BULK_SHARE", "0.8")
	if enabled, _ := SetupEgressQoS(true); !enabled {
		t.Fatal("explicit uplink must enable QoS even in dev")
	}
	q := activeEgressQoS.Load()
	if q == nil {
		t.Fatal("QoS not armed")
	}
	if math.Abs(float64(q.bulk.Limit())-500000) > 1 {
		t.Fatalf("bulk limit = %v, want ~500000 B/s", q.bulk.Limit())
	}
	if q.bulk.Burst() < qosMinBurst {
		t.Fatalf("burst = %d, want >= %d", q.bulk.Burst(), qosMinBurst)
	}
}

// bulk 路径被限速（200KB @ 100KB/s - 64KB burst ≈ 1.3s），交互路径同体量直通。
func TestQoSMiddlewareThrottlesBulkAndPassesInteractive(t *testing.T) {
	defer activeEgressQoS.Store(nil)
	activeEgressQoS.Store(&egressQoS{bulk: rate.NewLimiter(100000, qosMinBurst)})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(EgressQoSMiddleware())
	payload := bytes.Repeat([]byte{0}, 200000)
	router.GET("/datum/download/file", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/octet-stream", payload)
	})
	router.GET("/datum/file/tree", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", payload)
	})

	start := time.Now()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/datum/download/file", nil))
	if elapsed := time.Since(start); elapsed < 600*time.Millisecond {
		t.Fatalf("bulk response not throttled, elapsed = %v", elapsed)
	}
	if recorder.Body.Len() != len(payload) {
		t.Fatalf("bulk body truncated: %d", recorder.Body.Len())
	}

	start = time.Now()
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/datum/file/tree", nil))
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("interactive response throttled, elapsed = %v", elapsed)
	}
	if recorder.Body.Len() != len(payload) {
		t.Fatalf("interactive body truncated: %d", recorder.Body.Len())
	}
}

// 客户端断开（ctx 取消）时等待即刻中止，不悬挂。
func TestQoSMiddlewareStopsOnClientDisconnect(t *testing.T) {
	defer activeEgressQoS.Store(nil)
	// 极小速率 + 大响应：等待必然跨越 ctx 取消点
	activeEgressQoS.Store(&egressQoS{bulk: rate.NewLimiter(1, qosMinBurst)})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(EgressQoSMiddleware())
	router.GET("/datum/download/file", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/octet-stream", bytes.Repeat([]byte{0}, 1<<20))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodGet, "/datum/download/file", nil)
	request = request.WithContext(ctx)
	done := make(chan struct{})
	go func() {
		router.ServeHTTP(httptest.NewRecorder(), request)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Write did not abort after client disconnect")
	}
}
