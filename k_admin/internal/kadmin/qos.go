package kadmin

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"golang.org/x/time/rate"

	"github.com/gin-gonic/gin"
)

// 出站 QoS：把 bulk 流量（试卷/电子书统一下载、头像/封面等文件字节）压在
// 总上行的固定份额内（默认 80%），其余留给文件树、hash 校验、通知轮询等
// 小响应。交互侧刻意不做整形——hash 命中的响应接近 0 字节、列表响应仅几
// KB，只要 bulk 不越界，交互流量天然有带宽；这避免了任何调度器。
//
// 分层：本中间件是“总量权威上限”（无 nginx 的部署里是唯一整形层）；nginx
// 边缘的弹性档位负责“多个下载之间怎么分”，两层自然叠加（有效速率 =
// min(档位, 桶余量)），空闲时令牌积累支持短突发，与 nginx 独占档的“空闲
// 提速”哲学一致。
//
// 计量的是压缩后的线上字节：本中间件必须注册在 gzip 之前（更外层），否则
// 会把未压缩字节当线上字节，JSON 会被多限 5–10 倍。

const (
	qosUplinkKey         = "KADMIN_QOS_UPLINK_MBPS"
	qosBulkShareKey      = "KADMIN_QOS_BULK_SHARE"
	qosBurstKey          = "KADMIN_QOS_BURST_MB"
	qosDefaultUplinkMbps = 10.0 // 生产未声明上行时的假设值
	qosDefaultBulkShare  = 0.8
	// 单次 WaitN 的分片上限：rate.Limiter.WaitN 对 n > burst 直接报错，而
	// JSON/文件块的单次 Write 可能超过桶容量，按 io.Copy 的 32KB 粒度分片。
	qosChunkSize = 32 << 10
	// 桶容量下限：保证任意单个分片都能一次取得
	qosMinBurst = 2 * qosChunkSize
)

// 划入 bulk 共享桶的字节出口：统一下载（试卷/电子书）+ 托管文件内容 +
// 上传文件读取（头像/封面）+ 控制台上传物。其余路径（树/hash/通知/列表/
// 控制台/Swagger）直通。
var qosBulkPrefixes = []string{
	"/datum/download/",
	"/api/files/",
	"/api/uploads/",
	"/uploads",
}

type egressQoS struct {
	bulk *rate.Limiter
}

var activeEgressQoS atomic.Pointer[egressQoS]

func qosBulkPath(path string) bool {
	for _, prefix := range qosBulkPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// SetupEgressQoS 解析环境变量并装配 QoS 单例，返回（是否启用, 状态描述）
// 供启动日志。语义：显式声明上行（KADMIN_QOS_UPLINK_MBPS>0）即启用——任何
// 模式；未声明时生产（非 debug）默认按 10Mbps 启用、开发默认关闭；显式置
// 0 强制关闭。
func SetupEgressQoS(debug bool) (bool, string) {
	disarm := func(reason string) (bool, string) {
		activeEgressQoS.Store(nil)
		return false, reason
	}

	raw := strings.TrimSpace(os.Getenv(qosUplinkKey))
	var uplinkMbps float64
	if raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value < 0 {
			return disarm(qosUplinkKey + " 无效，QoS 关闭")
		}
		if value == 0 {
			return disarm(qosUplinkKey + "=0，QoS 显式关闭")
		}
		uplinkMbps = value
	} else if debug {
		return disarm("开发模式默认关闭（声明 " + qosUplinkKey + " 可开启）")
	} else {
		uplinkMbps = qosDefaultUplinkMbps
	}

	share := qosDefaultBulkShare
	if raw := strings.TrimSpace(os.Getenv(qosBulkShareKey)); raw != "" {
		if value, err := strconv.ParseFloat(raw, 64); err == nil && value > 0 && value <= 1 {
			share = value
		}
	}

	bulkBytesPerSec := uplinkMbps * 125000 * share // 1Mbps = 125,000 B/s
	burstBytes := 2 * bulkBytesPerSec
	if raw := strings.TrimSpace(os.Getenv(qosBurstKey)); raw != "" {
		if value, err := strconv.ParseFloat(raw, 64); err == nil && value > 0 {
			burstBytes = value * 1024 * 1024
		}
	}
	if burstBytes < qosMinBurst {
		burstBytes = qosMinBurst
	}

	activeEgressQoS.Store(&egressQoS{
		bulk: rate.NewLimiter(rate.Limit(bulkBytesPerSec), int(burstBytes)),
	})
	return true, fmt.Sprintf("总上行 %.0fMbps × bulk %.0f%% = %.2fMB/s（burst %.1fMB）；头像/封面/试卷/电子书共享 bulk 桶",
		uplinkMbps, share*100, bulkBytesPerSec/(1<<20), burstBytes/(1<<20))
}

// EgressQoSMiddleware 对 bulk 路径包上节流 ResponseWriter；其余路径零开销直通。
func EgressQoSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		q := activeEgressQoS.Load()
		if q == nil || !qosBulkPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		writer := &throttledResponseWriter{
			ResponseWriter: c.Writer,
			limiter:        (*q).bulk,
			ctx:            c.Request.Context(),
		}
		c.Writer = writer
		defer func() {
			if recovered := recover(); recovered != nil {
				// gin v1.3 的 Render 对写错误 panic；客户端断开后的中止是预期
				// 路径，就地收场（连接已死，源读取已在 Write 报错时停止）。
				if !writer.aborted {
					panic(recovered)
				}
			}
		}()
		c.Next()
	}
}

// throttledResponseWriter 只在 Write 时取 token（请求头/状态码不占额度）；
// ctx 取消（客户端断开）即刻中止等待。
type throttledResponseWriter struct {
	gin.ResponseWriter
	limiter *rate.Limiter
	ctx     context.Context
	aborted bool // Write 与 recover 同属请求 goroutine，无并发访问
}

func (w *throttledResponseWriter) Write(data []byte) (int, error) {
	written := 0
	for written < len(data) {
		n := len(data) - written
		if n > qosChunkSize {
			n = qosChunkSize
		}
		if err := w.limiter.WaitN(w.ctx, n); err != nil {
			w.aborted = true
			return written, err
		}
		m, err := w.ResponseWriter.Write(data[written : written+n])
		written += m
		if err != nil {
			w.aborted = true
			return written, err
		}
	}
	return written, nil
}
