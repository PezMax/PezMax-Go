package kadmin

import (
	"compress/gzip"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// gzipMinLength 之下的已定长响应不压缩：压缩头的固定开销会反噬带宽收益。
const gzipMinLength = 256

var gzipWriterPool = sync.Pool{
	New: func() interface{} { return gzip.NewWriter(nil) },
}

// GzipMiddleware 对文本类响应（JSON/HTML/CSS/JS/XML/SVG）做透明 gzip 压缩。
// 上行受限的主机上这是收益最大的一步：JSON 列表与后台控制台资源通常能压掉
// 70%–90%。必须最先 Use，保证位于所有其他写响应的中间件之外。
//
// 刻意不压缩的场景：非 2xx、HEAD、携带 Range（206 与 gzip 语义冲突）、
// 已有 Content-Encoding、非文本类 Content-Type（图片/PDF/二进制本身已压缩，
// 再压只耗 CPU 不省带宽）、以及已知小于 gzipMinLength 的响应。
func GzipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodHead || !acceptsGzip(c.Request.Header) {
			c.Next()
			return
		}
		writer := &gzipResponseWriter{ResponseWriter: c.Writer, request: c.Request}
		c.Writer = writer
		c.Next()
		writer.finish()
	}
}

func acceptsGzip(header http.Header) bool {
	for _, value := range header.Values("Accept-Encoding") {
		for _, part := range strings.Split(value, ",") {
			encoding := strings.TrimSpace(strings.ToLower(part))
			if encoding == "gzip" || strings.HasPrefix(encoding, "gzip;") {
				return true
			}
		}
	}
	return false
}

func compressibleContentType(contentType string) bool {
	if index := strings.Index(contentType, ";"); index >= 0 {
		contentType = contentType[:index]
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case strings.HasPrefix(contentType, "text/"):
		return true
	case strings.HasPrefix(contentType, "application/json"), strings.HasSuffix(contentType, "+json"):
		return true
	case strings.HasPrefix(contentType, "application/javascript"), strings.HasPrefix(contentType, "application/x-javascript"):
		return true
	case strings.HasPrefix(contentType, "application/xml"), strings.HasSuffix(contentType, "+xml"):
		return true
	default:
		return false
	}
}

// gzipResponseWriter 把压缩决策推迟到 WriteHeaderNow——gin 真正向底层写出
// 响应头之前的那一刻，此时状态码、Content-Type、Content-Length、Range 均已
// 确定。gin 的 WriteHeader 只记录状态不落盘，因此这里是注入压缩头的唯一
// 可靠时机。
type gzipResponseWriter struct {
	gin.ResponseWriter
	request     *http.Request
	gzipWriter  *gzip.Writer
	decided     bool
	compressing bool
	written     int64
}

func (w *gzipResponseWriter) WriteHeaderNow() {
	if w.decided {
		return
	}
	w.decided = true
	header := w.Header()
	header.Add("Vary", "Accept-Encoding")
	if w.shouldCompress(header) {
		header.Del("Content-Length")
		header.Set("Content-Encoding", "gzip")
		w.gzipWriter = gzipWriterPool.Get().(*gzip.Writer)
		w.gzipWriter.Reset(w.ResponseWriter)
		w.compressing = true
	}
	w.ResponseWriter.WriteHeaderNow()
}

func (w *gzipResponseWriter) shouldCompress(header http.Header) bool {
	if w.ResponseWriter.Written() || w.request == nil {
		return false
	}
	if status := w.ResponseWriter.Status(); status < 200 || status > 299 {
		return false
	}
	if w.request.Header.Get("Range") != "" {
		return false
	}
	if header.Get("Content-Encoding") != "" {
		return false
	}
	if !compressibleContentType(header.Get("Content-Type")) {
		return false
	}
	if contentLength := header.Get("Content-Length"); contentLength != "" {
		if size, err := strconv.ParseInt(contentLength, 10, 64); err == nil && size < gzipMinLength {
			return false
		}
	}
	return true
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	w.WriteHeaderNow()
	if w.compressing {
		n, err := w.gzipWriter.Write(data)
		w.written += int64(n)
		return n, err
	}
	return w.ResponseWriter.Write(data)
}

func (w *gzipResponseWriter) WriteString(value string) (int, error) {
	w.WriteHeaderNow()
	if w.compressing {
		n, err := w.gzipWriter.Write([]byte(value))
		w.written += int64(n)
		return n, err
	}
	return w.ResponseWriter.WriteString(value)
}

func (w *gzipResponseWriter) Flush() {
	w.WriteHeaderNow()
	if w.compressing {
		_ = w.gzipWriter.Flush()
	}
	w.ResponseWriter.Flush()
}

func (w *gzipResponseWriter) Size() int {
	if w.compressing {
		return int(w.written)
	}
	return w.ResponseWriter.Size()
}

// finish 在请求收尾时收尾压缩流（写出 gzip trailer），并把 writer 归还对象池。
func (w *gzipResponseWriter) finish() {
	if !w.compressing {
		return
	}
	w.compressing = false
	_ = w.gzipWriter.Close()
	w.gzipWriter.Reset(nil)
	gzipWriterPool.Put(w.gzipWriter)
	w.gzipWriter = nil
}
