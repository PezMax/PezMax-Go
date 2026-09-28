package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestMinioPutStreamsSignedPayload(t *testing.T) {
	payload := []byte("streamed upload")
	var (
		receivedBody []byte
		receivedHash string
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/minio/health/live":
			writer.WriteHeader(http.StatusOK)
		case request.Method == http.MethodHead && request.URL.Path == "/kadmin":
			writer.WriteHeader(http.StatusOK)
		case request.Method == http.MethodPut && request.URL.Path == "/kadmin/attachments/report.pdf":
			receivedHash = request.Header.Get("X-Amz-Content-Sha256")
			receivedBody, _ = io.ReadAll(request.Body)
			writer.WriteHeader(http.StatusOK)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	store := NewMinio(MinioConfig{
		Endpoints: []string{server.URL},
		AccessKey: "access",
		SecretKey: "secret",
		Bucket:    "kadmin",
		Region:    "us-east-1",
	})

	if err := store.Put(context.Background(), "attachments/report.pdf", bytes.NewReader(payload), int64(len(payload)), "application/pdf"); err != nil {
		t.Fatalf("put MinIO object: %v", err)
	}
	wantHash := sha256.Sum256(payload)
	if !bytes.Equal(receivedBody, payload) || receivedHash != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("unexpected streamed payload: hash=%q body=%q", receivedHash, receivedBody)
	}
}

func TestMinioMapsMissingObjectToNotExist(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	store := NewMinio(MinioConfig{
		Endpoints: []string{server.URL},
		AccessKey: "access",
		SecretKey: "secret",
		Bucket:    "kadmin",
		Region:    "us-east-1",
	})

	if _, _, err := store.Open(context.Background(), "missing.pdf"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("open missing object: %v", err)
	}
	if err := store.Delete(context.Background(), "missing.pdf"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("delete missing object: %v", err)
	}
}

func TestMinioOpenReturnsSeekableObject(t *testing.T) {
	payload := []byte("0123456789abcdef")
	var seenRanges []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/minio/health/live":
			writer.WriteHeader(http.StatusOK)
		case request.Method == http.MethodGet && request.URL.Path == "/kadmin/attachments/blob.bin":
			if rangeHeader := request.Header.Get("Range"); rangeHeader != "" {
				seenRanges = append(seenRanges, rangeHeader)
				var start int
				if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &start); err != nil {
					t.Errorf("bad range header %q: %v", rangeHeader, err)
					return
				}
				writer.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
				writer.Header().Set("Content-Type", "application/octet-stream")
				writer.WriteHeader(http.StatusPartialContent)
				_, _ = writer.Write(payload[start:])
				return
			}
			writer.Header().Set("Content-Type", "application/octet-stream")
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(payload)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	store := NewMinio(MinioConfig{
		Endpoints: []string{server.URL},
		AccessKey: "access",
		SecretKey: "secret",
		Bucket:    "kadmin",
		Region:    "us-east-1",
	})

	body, info, err := store.Open(context.Background(), "attachments/blob.bin")
	if err != nil {
		t.Fatalf("open object: %v", err)
	}
	defer body.Close()
	seeker, ok := body.(io.ReadSeeker)
	if !ok {
		t.Fatalf("expected seekable object, got %T", body)
	}
	if info.Size != int64(len(payload)) {
		t.Fatalf("size = %d, want %d", info.Size, len(payload))
	}

	// ServeContent 的典型探测路径：Seek 到末尾取尺寸，再回到起点读全量。
	if size, err := seeker.Seek(0, io.SeekEnd); err != nil || size != int64(len(payload)) {
		t.Fatalf("seek end: size=%d err=%v", size, err)
	}
	if _, err := seeker.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek start: %v", err)
	}
	full, err := io.ReadAll(seeker)
	if err != nil || !bytes.Equal(full, payload) {
		t.Fatalf("read full body: err=%v body=%q", err, full)
	}

	// 断点续传路径：Seek 到中间位置后应触发 ranged GET 并读到正确切片。
	if _, err := seeker.Seek(10, io.SeekStart); err != nil {
		t.Fatalf("seek middle: %v", err)
	}
	tail, err := io.ReadAll(seeker)
	if err != nil || string(tail) != string(payload[10:]) {
		t.Fatalf("read after seek: err=%v body=%q", err, tail)
	}
	if len(seenRanges) != 1 || seenRanges[0] != "bytes=10-" {
		t.Fatalf("expected exactly one ranged GET bytes=10-, got %v", seenRanges)
	}
}

func TestReplayablePayloadUsesSeekableSource(t *testing.T) {
	payload := []byte("streamed upload")
	source, cleanup, size, hash, err := replayablePayload(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("prepare payload: %v", err)
	}
	defer cleanup()

	actual, err := io.ReadAll(source)
	if err != nil {
		t.Fatalf("read replayable payload: %v", err)
	}
	wantHash := sha256.Sum256(payload)
	if !bytes.Equal(actual, payload) || size != int64(len(payload)) || hash != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("unexpected payload result: size=%d hash=%q body=%q", size, hash, actual)
	}
}

func TestReplayablePayloadSpoolsNonSeekableSource(t *testing.T) {
	payload := []byte("streamed upload")
	source, cleanup, _, _, err := replayablePayload(bytes.NewBuffer(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("prepare payload: %v", err)
	}
	temporary, ok := source.(*os.File)
	if !ok {
		cleanup()
		t.Fatalf("expected temporary file, got %T", source)
	}
	name := temporary.Name()
	cleanup()
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Fatalf("temporary payload was not removed: %v", err)
	}
}

func TestReplayablePayloadRejectsChangedSize(t *testing.T) {
	_, cleanup, _, _, err := replayablePayload(bytes.NewReader([]byte("payload")), 99)
	defer cleanup()
	if err == nil {
		t.Fatal("expected changed payload size to fail")
	}
}
