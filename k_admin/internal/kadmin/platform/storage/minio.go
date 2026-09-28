package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

type MinioConfig struct {
	Endpoints  []string
	AccessKey  string
	SecretKey  string
	Bucket     string
	UseSSL     bool
	Region     string
	PublicBase string
	Timeout    time.Duration
}

type Minio struct {
	config MinioConfig
}

func NewMinio(config MinioConfig) *Minio {
	return &Minio{config: config}
}

func (m *Minio) Put(ctx context.Context, objectKey string, body io.Reader, size int64, contentType string) error {
	if err := m.validateConfig(); err != nil {
		return err
	}
	source, cleanup, payloadSize, payloadHash, err := replayablePayload(body, size)
	if err != nil {
		return err
	}
	defer cleanup()
	var lastErr error
	for _, endpoint := range m.config.Endpoints {
		client := newMinioHTTPClient(m.config, endpoint)
		if client == nil {
			continue
		}
		if err := client.health(ctx); err != nil {
			lastErr = err
			continue
		}
		if err := client.ensureBucket(ctx); err != nil {
			lastErr = err
			continue
		}
		if _, err := source.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if err := client.putObject(ctx, objectKey, source, payloadSize, payloadHash, contentType); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return minioEndpointError(lastErr)
}

func replayablePayload(body io.Reader, expectedSize int64) (io.ReadSeeker, func(), int64, string, error) {
	if body == nil {
		return nil, func() {}, 0, "", fmt.Errorf("upload body is required")
	}
	hash := sha256.New()
	var (
		source  io.ReadSeeker
		written int64
		err     error
	)
	cleanup := func() {}
	if seeker, ok := body.(io.ReadSeeker); ok {
		source = seeker
		if _, err = source.Seek(0, io.SeekStart); err == nil {
			written, err = io.Copy(hash, source)
		}
	} else {
		temporary, err := os.CreateTemp("", "kadmin-minio-upload-*")
		if err != nil {
			return nil, cleanup, 0, "", err
		}
		source = temporary
		cleanup = func() {
			temporary.Close()
			_ = os.Remove(temporary.Name())
		}
		written, err = io.Copy(io.MultiWriter(hash, temporary), body)
	}
	if err != nil {
		cleanup()
		return nil, func() {}, 0, "", err
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, func() {}, 0, "", err
	}
	if expectedSize >= 0 && written != expectedSize {
		cleanup()
		return nil, func() {}, 0, "", fmt.Errorf("upload size changed: got %d, want %d", written, expectedSize)
	}
	return source, cleanup, written, hex.EncodeToString(hash.Sum(nil)), nil
}

func (m *Minio) Open(ctx context.Context, objectKey string) (io.ReadCloser, ObjectInfo, error) {
	if err := m.validateConfig(); err != nil {
		return nil, ObjectInfo{}, err
	}
	var lastErr error
	for _, endpoint := range m.config.Endpoints {
		client := newMinioHTTPClient(m.config, endpoint)
		if client == nil {
			continue
		}
		body, info, err := client.getObject(ctx, objectKey)
		if err != nil {
			lastErr = err
			continue
		}
		return body, info, nil
	}
	return nil, ObjectInfo{}, minioEndpointError(lastErr)
}

func (m *Minio) Delete(ctx context.Context, objectKey string) error {
	if err := m.validateConfig(); err != nil {
		return err
	}
	var lastErr error
	for _, endpoint := range m.config.Endpoints {
		client := newMinioHTTPClient(m.config, endpoint)
		if client == nil {
			continue
		}
		if err := client.deleteObject(ctx, objectKey); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return minioEndpointError(lastErr)
}

func (m *Minio) validateConfig() error {
	if m.config.AccessKey == "" || m.config.SecretKey == "" || m.config.Bucket == "" {
		return fmt.Errorf("minio config is incomplete")
	}
	return nil
}

func minioEndpointError(lastErr error) error {
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("minio endpoint is not configured")
}

type minioHTTPClient struct {
	config       MinioConfig
	baseURL      string
	client       *http.Client
	streamClient *http.Client
}

func newMinioHTTPClient(config MinioConfig, endpoint string) *minioHTTPClient {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		scheme := "http"
		if config.UseSSL {
			scheme = "https"
		}
		endpoint = scheme + "://" + endpoint
	}
	endpoint = strings.TrimRight(endpoint, "/")
	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return nil
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &minioHTTPClient{
		config:  config,
		baseURL: endpoint,
		client:  &http.Client{Timeout: timeout},
		// 流式读取对象体时不能带整体超时：大文件转发动辄数分钟，
		// http.Client.Timeout 会把读 body 一起掐断；取消由请求 ctx 负责。
		streamClient: &http.Client{},
	}
}

func (m *minioHTTPClient) health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.baseURL+"/minio/health/live", nil)
	if err != nil {
		return err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("minio health check failed: %s", resp.Status)
	}
	return nil
}

func (m *minioHTTPClient) ensureBucket(ctx context.Context) error {
	resp, err := m.signedRequest(ctx, http.MethodHead, "/"+EscapePath(m.config.Bucket), nil, "")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("minio bucket check failed: %s", resp.Status)
	}

	resp, err = m.signedRequest(ctx, http.MethodPut, "/"+EscapePath(m.config.Bucket), nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("minio bucket create failed: %s", resp.Status)
	}
	return nil
}

func (m *minioHTTPClient) putObject(ctx context.Context, objectKey string, body io.Reader, size int64, payloadHash string, contentType string) error {
	resp, err := m.signedPayloadRequest(
		ctx,
		http.MethodPut,
		"/"+EscapePath(m.config.Bucket)+"/"+EscapePath(objectKey),
		body,
		size,
		payloadHash,
		contentType,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("minio upload failed: %s: %s", resp.Status, minioErrorSnippet(resp.Body))
	}
	return nil
}

// minioErrorSnippet reads a bounded prefix of an error response body so
// callers log MinIO's actual rejection reason (SignatureDoesNotMatch,
// RequestTimeTooSkewed, AccessDenied, …) instead of a bare status code.
func minioErrorSnippet(body io.Reader) string {
	if body == nil {
		return ""
	}
	snippet, _ := io.ReadAll(io.LimitReader(body, 512))
	text := strings.TrimSpace(string(snippet))
	if text == "" {
		return "(empty body)"
	}
	return text
}

func (m *minioHTTPClient) getObject(ctx context.Context, objectKey string) (*seekableObject, ObjectInfo, error) {
	resp, err := m.getObjectResponse(ctx, objectKey, 0)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	return &seekableObject{
		client:    m,
		ctx:       ctx,
		objectKey: objectKey,
		body:      resp.Body,
		size:      resp.ContentLength,
	}, ObjectInfo{
		ContentType: resp.Header.Get("Content-Type"),
		Size:        resp.ContentLength,
	}, nil
}

// getObjectResponse 发起签名 GET；rangeStart > 0 时附带 Range 头转发客户端的
// 断点续传请求（S3 签名不强制覆盖 Range，MinIO 原生支持 ranged GET）。
func (m *minioHTTPClient) getObjectResponse(ctx context.Context, objectKey string, rangeStart int64) (*http.Response, error) {
	endpoint := m.baseURL + "/" + EscapePath(m.config.Bucket) + "/" + EscapePath(objectKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if rangeStart > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", rangeStart))
	}
	m.sign(req, sha256Hex(nil), "range")
	resp, err := m.streamClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("minio get failed: %s", resp.Status)
	}
	return resp, nil
}

func (m *minioHTTPClient) deleteObject(ctx context.Context, objectKey string) error {
	resp, err := m.signedRequest(
		ctx,
		http.MethodDelete,
		"/"+EscapePath(m.config.Bucket)+"/"+EscapePath(objectKey),
		nil,
		"",
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusNotFound {
			return os.ErrNotExist
		}
		return fmt.Errorf("minio delete failed: %s", resp.Status)
	}
	return nil
}

func (m *minioHTTPClient) signedRequest(ctx context.Context, method string, requestPath string, body io.Reader, contentType string) (*http.Response, error) {
	var payload []byte
	var err error
	if body != nil {
		payload, err = io.ReadAll(body)
		if err != nil {
			return nil, err
		}
	}

	endpoint := m.baseURL + requestPath
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	m.sign(req, sha256Hex(payload))
	return m.client.Do(req)
}

func (m *minioHTTPClient) signedPayloadRequest(ctx context.Context, method string, requestPath string, body io.Reader, size int64, payloadHash string, contentType string) (*http.Response, error) {
	endpoint := m.baseURL + requestPath
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.ContentLength = size
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	m.sign(req, payloadHash)
	return m.client.Do(req)
}

func (m *minioHTTPClient) sign(req *http.Request, payloadHash string, extraSignedHeaders ...string) {
	now := time.Now().UTC()
	date := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	for _, name := range extraSignedHeaders {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" && req.Header.Get(name) != "" {
			signedHeaders = append(signedHeaders, name)
		}
	}
	sort.Strings(signedHeaders)
	canonicalHeaders := strings.Builder{}
	for _, name := range signedHeaders {
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteByte(':')
		if name == "host" {
			canonicalHeaders.WriteString(req.URL.Host)
		} else {
			canonicalHeaders.WriteString(req.Header.Get(name))
		}
		canonicalHeaders.WriteByte('\n')
	}

	canonicalQuery := canonicalQueryString(req.URL.Query())
	scope := strings.Join([]string{date, m.config.Region, "s3", "aws4_request"}, "/")
	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		canonicalQuery,
		canonicalHeaders.String(),
		strings.Join(signedHeaders, ";"),
		payloadHash,
	}, "\n")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	signingKey := awsSigningKey(m.config.SecretKey, date, m.config.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		m.config.AccessKey,
		scope,
		strings.Join(signedHeaders, ";"),
		signature,
	))
}

func canonicalQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0)
	for key, vals := range values {
		sort.Strings(vals)
		for _, value := range vals {
			parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(value))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "&")
}

func awsSigningKey(secret string, date string, region string, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(value))
	return mac.Sum(nil)
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

// seekableObject 把 MinIO 对象流包装成可 Seek 的读取器：偏移变化时按需发起
// 新的签名 ranged GET（MinIO 原生支持），从而让 http.ServeContent 直接提供
// Range 断点续传，而无需把整个对象读进内存。偏移回退不做预取——下一次
// Read 才按新偏移重新取流，ServeContent 探测尺寸（Seek 到末尾再回 0）不会
// 产生多余的往返。
type seekableObject struct {
	client    *minioHTTPClient
	ctx       context.Context
	objectKey string
	body      io.ReadCloser
	offset    int64
	size      int64
}

func (o *seekableObject) Read(p []byte) (int, error) {
	if o.body == nil {
		if o.size >= 0 && o.offset >= o.size {
			return 0, io.EOF
		}
		resp, err := o.client.getObjectResponse(o.ctx, o.objectKey, o.offset)
		if err != nil {
			return 0, err
		}
		o.body = resp.Body
	}
	n, err := o.body.Read(p)
	o.offset += int64(n)
	return n, err
}

func (o *seekableObject) Seek(offset int64, whence int) (int64, error) {
	var target int64
	switch whence {
	case io.SeekStart:
		target = offset
	case io.SeekCurrent:
		target = o.offset + offset
	case io.SeekEnd:
		if o.size < 0 {
			return 0, fmt.Errorf("storage: seek from end requires known object size")
		}
		target = o.size + offset
	default:
		return 0, fmt.Errorf("storage: invalid seek whence %d", whence)
	}
	if target < 0 {
		return 0, fmt.Errorf("storage: negative seek target %d", target)
	}
	if target == o.offset && o.body != nil {
		return target, nil
	}
	if o.body != nil {
		_ = o.body.Close()
		o.body = nil
	}
	o.offset = target
	return target, nil
}

func (o *seekableObject) Close() error {
	if o.body == nil {
		return nil
	}
	err := o.body.Close()
	o.body = nil
	return err
}
