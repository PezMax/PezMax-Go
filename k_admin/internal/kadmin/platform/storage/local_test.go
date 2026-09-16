package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingReader struct {
	delivered bool
}

func (r *failingReader) Read(buffer []byte) (int, error) {
	if !r.delivered {
		r.delivered = true
		return copy(buffer, []byte("partial")), nil
	}
	return 0, errors.New("read failed")
}

func TestLocalPutOpenDelete(t *testing.T) {
	root := t.TempDir()
	store := NewLocal(root)
	objectKey := "avatars/20260725/avatar.png"
	payload := []byte("image payload")

	if err := store.Put(context.Background(), objectKey, bytes.NewReader(payload), int64(len(payload)), "image/png"); err != nil {
		t.Fatalf("put local object: %v", err)
	}

	body, info, err := store.Open(context.Background(), objectKey)
	if err != nil {
		t.Fatalf("open local object: %v", err)
	}
	actual, err := io.ReadAll(body)
	if err != nil {
		body.Close()
		t.Fatalf("read local object: %v", err)
	}
	if err := body.Close(); err != nil {
		t.Fatalf("close local object: %v", err)
	}
	if !bytes.Equal(actual, payload) {
		t.Fatalf("unexpected local payload: %q", actual)
	}
	if info.Size != int64(len(payload)) || info.ContentType != "image/png" {
		t.Fatalf("unexpected local object info: %#v", info)
	}

	if err := store.Delete(context.Background(), objectKey); err != nil {
		t.Fatalf("delete local object: %v", err)
	}
	if _, _, err := store.Open(context.Background(), objectKey); err == nil {
		t.Fatal("expected deleted local object to be missing")
	}
}

func TestLocalPathNeverEscapesRoot(t *testing.T) {
	root := t.TempDir()
	store := NewLocal(root)
	for _, objectKey := range []string{"../outside.txt", "/../../outside.txt", "avatars/../safe.txt"} {
		target, err := store.Path(objectKey)
		if err != nil {
			t.Fatalf("resolve %q: %v", objectKey, err)
		}
		relative, err := filepath.Rel(root, target)
		if err != nil {
			t.Fatalf("relative path for %q: %v", objectKey, err)
		}
		if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			t.Fatalf("object key %q escaped root to %q", objectKey, target)
		}
	}
}

func TestLocalPutDoesNotExposePartialObject(t *testing.T) {
	root := t.TempDir()
	store := NewLocal(root)
	objectKey := "attachments/report.pdf"

	if err := store.Put(context.Background(), objectKey, &failingReader{}, 99, "application/pdf"); err == nil {
		t.Fatal("expected local write failure")
	}
	target, err := store.Path(objectKey)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("partial object must not be visible: %v", err)
	}
}

func TestCleanObjectKeyRejectsInvalidValues(t *testing.T) {
	for _, objectKey := range []string{"", "/", "\x00"} {
		if _, err := CleanObjectKey(objectKey); !errors.Is(err, ErrInvalidObjectKey) {
			t.Fatalf("expected %q to be rejected, got %v", objectKey, err)
		}
	}
}
