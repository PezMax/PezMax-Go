package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
