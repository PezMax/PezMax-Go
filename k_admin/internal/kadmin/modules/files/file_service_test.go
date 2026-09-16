package files

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/storage"
)

type fakeObjectStore struct {
	deleteCalls int
	deleteErr   error
	openErr     error
	putBody     []byte
	putCalls    int
	putErr      error
}

func (f *fakeObjectStore) Put(_ context.Context, _ string, body io.Reader, _ int64, _ string) error {
	f.putCalls++
	f.putBody, _ = io.ReadAll(body)
	return f.putErr
}

func (f *fakeObjectStore) Open(context.Context, string) (io.ReadCloser, storage.ObjectInfo, error) {
	if f.openErr != nil {
		return nil, storage.ObjectInfo{}, f.openErr
	}
	return nil, storage.ObjectInfo{}, os.ErrNotExist
}

func (f *fakeObjectStore) Delete(context.Context, string) error {
	f.deleteCalls++
	return f.deleteErr
}

type fakeFileRepository struct {
	createErr    error
	record       fileRecord
	statusErrors map[string]error
	statuses     []string
}

func (f *fakeFileRepository) Create(record *fileRecord) error {
	if f.createErr != nil {
		return f.createErr
	}
	record.ID = 42
	f.record = *record
	return nil
}

func (f *fakeFileRepository) FindReady(id int64) (fileRecord, bool, error) {
	if id != f.record.ID || f.record.Status != fileStatusReady {
		return fileRecord{}, false, nil
	}
	return f.record, true, nil
}

func (f *fakeFileRepository) UpdateStatus(id int64, status string, storageName string, bucket string, deletedAt *time.Time) error {
	f.statuses = append(f.statuses, status)
	if err := f.statusErrors[status]; err != nil {
		return err
	}
	f.record.ID = id
	f.record.Status = status
	if storageName != "" {
		f.record.Storage = storageName
	}
	if bucket != "" {
		f.record.Bucket = bucket
	}
	f.record.DeletedAt = deletedAt
	return nil
}

func TestFileServicePrefersRemoteStore(t *testing.T) {
	remote := &fakeObjectStore{}
	service := &fileService{
		local:  storage.NewLocal(t.TempDir()),
		remote: remote,
	}
	payload := []byte("avatar")

	storageName, err := service.put(context.Background(), "avatars/avatar.png", payload, "image/png")
	if err != nil {
		t.Fatalf("put remote object: %v", err)
	}
	if storageName != minioStorageName || remote.putCalls != 1 || !bytes.Equal(remote.putBody, payload) {
		t.Fatalf("unexpected remote upload result: storage=%q calls=%d body=%q", storageName, remote.putCalls, remote.putBody)
	}
	if _, ok := service.localPath("avatars/avatar.png"); ok {
		t.Fatal("did not expect a local fallback after remote success")
	}
}

func TestFileServiceFallsBackToLocalStore(t *testing.T) {
	remote := &fakeObjectStore{putErr: errors.New("remote unavailable")}
	service := &fileService{
		local:  storage.NewLocal(t.TempDir()),
		remote: remote,
	}
	payload := []byte("avatar")

	storageName, err := service.put(context.Background(), "avatars/avatar.png", payload, "image/png")
	if err != nil {
		t.Fatalf("put fallback object: %v", err)
	}
	if storageName != localStorageName || remote.putCalls != 1 {
		t.Fatalf("unexpected fallback result: storage=%q calls=%d", storageName, remote.putCalls)
	}
	localPath, ok := service.localPath("avatars/avatar.png")
	if !ok {
		t.Fatal("expected local fallback file")
	}
	actual, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("read fallback file: %v", err)
	}
	if !bytes.Equal(actual, payload) {
		t.Fatalf("unexpected fallback payload: %q", actual)
	}
}

func TestCreateManagedFileMarksFailedWhenStorageUnavailable(t *testing.T) {
	rootFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(rootFile, []byte("blocked"), 0644); err != nil {
		t.Fatalf("create blocked local root: %v", err)
	}
	remote := &fakeObjectStore{putErr: errors.New("remote unavailable")}
	repository := &fakeFileRepository{}
	service := &fileService{
		local:      storage.NewLocal(rootFile),
		remote:     remote,
		repository: repository,
	}

	_, err := service.createManagedFile(context.Background(), testValidatedManagedFile(t), 7)
	if !errors.Is(err, errFileStorageUnavailable) {
		t.Fatalf("expected storage error, got %v", err)
	}
	if len(repository.statuses) != 1 || repository.statuses[0] != fileStatusFailed {
		t.Fatalf("unexpected status updates: %#v", repository.statuses)
	}
}

func TestCreateManagedFileDeletesObjectWhenFinalizeFails(t *testing.T) {
	remote := &fakeObjectStore{}
	repository := &fakeFileRepository{statusErrors: map[string]error{
		fileStatusReady: errors.New("database unavailable"),
	}}
	service := &fileService{
		local:        storage.NewLocal(t.TempDir()),
		remote:       remote,
		remoteBucket: "kadmin",
		repository:   repository,
	}

	_, err := service.createManagedFile(context.Background(), testValidatedManagedFile(t), 7)
	if !errors.Is(err, errFileMetadata) {
		t.Fatalf("expected metadata error, got %v", err)
	}
	if remote.deleteCalls != 1 {
		t.Fatalf("expected uploaded object cleanup, got %d deletes", remote.deleteCalls)
	}
	if len(repository.statuses) != 2 || repository.statuses[1] != fileStatusFailed {
		t.Fatalf("unexpected status updates: %#v", repository.statuses)
	}
}

func TestCreateManagedFileDoesNotFallbackWhenRemoteFails(t *testing.T) {
	remote := &fakeObjectStore{putErr: errors.New("remote unavailable")}
	repository := &fakeFileRepository{}
	localRoot := t.TempDir()
	service := &fileService{
		local:      storage.NewLocal(localRoot),
		remote:     remote,
		repository: repository,
	}
	upload := testValidatedManagedFile(t)

	_, err := service.createManagedFile(context.Background(), upload, 7)
	if !errors.Is(err, errFileStorageUnavailable) {
		t.Fatalf("expected storage error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(localRoot, filepath.FromSlash(upload.ObjectKey))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generic upload must not silently fall back to local storage: %v", err)
	}
}

func TestOpenManagedFileReportsStorageUnavailable(t *testing.T) {
	repository := &fakeFileRepository{record: fileRecord{
		ID:        42,
		ObjectKey: "attachments/report.pdf",
		Storage:   minioStorageName,
		Status:    fileStatusReady,
	}}
	service := &fileService{
		local:      storage.NewLocal(t.TempDir()),
		remote:     &fakeObjectStore{openErr: errors.New("remote unavailable")},
		repository: repository,
	}

	_, _, _, err := service.openManagedFile(context.Background(), 42)
	if !errors.Is(err, errFileStorageUnavailable) {
		t.Fatalf("expected storage unavailable error, got %v", err)
	}
}

func testValidatedManagedFile(t *testing.T) validatedManagedFile {
	t.Helper()
	header := testMultipartFileHeader(t, "report.pdf", []byte("%PDF-1.7"))
	return validatedManagedFile{
		File:         header,
		Size:         header.Size,
		ContentType:  "application/pdf",
		Extension:    ".pdf",
		OriginalName: "report.pdf",
		ObjectKey:    "attachments/2026/07/25/report.pdf",
		Purpose:      filePurposeAttachment,
		Visibility:   fileVisibilityPrivate,
		SHA256:       strings.Repeat("a", 64),
	}
}
