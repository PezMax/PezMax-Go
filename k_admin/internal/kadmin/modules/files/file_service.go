package files

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/storage"
)

const (
	localStorageName = "local"
	minioStorageName = "minio"
)

type fileService struct {
	local        *storage.Local
	remote       storage.ObjectStore
	remoteBucket string
	repository   fileRepository
}

var (
	errFileNotFound           = errors.New("file not found")
	errFileContentNotFound    = errors.New("file content not found")
	errFileMetadata           = errors.New("file metadata unavailable")
	errFileStorageUnavailable = errors.New("file storage unavailable")
)

func newFileService(settings fileStorageSettings) *fileService {
	service := &fileService{
		local:        storage.NewLocal(settings.LocalRoot),
		remoteBucket: settings.Minio.Bucket,
	}
	if settings.MinioEnabled {
		service.remote = storage.NewMinio(settings.Minio)
	}
	return service
}

func newManagedFileService(settings fileStorageSettings, repository fileRepository) *fileService {
	service := newFileService(settings)
	service.local = storage.NewLocal(settings.ManagedLocalRoot)
	service.repository = repository
	return service
}

func newFileServiceFromEnv() *fileService {
	return newFileService(loadFileStorageSettings())
}

func (s *fileService) put(ctx context.Context, objectKey string, body []byte, contentType string) (string, error) {
	if s.remote != nil {
		if err := s.remote.Put(ctx, objectKey, bytes.NewReader(body), int64(len(body)), contentType); err == nil {
			return minioStorageName, nil
		}
	}
	if err := s.local.Put(ctx, objectKey, bytes.NewReader(body), int64(len(body)), contentType); err != nil {
		return "", err
	}
	return localStorageName, nil
}

func (s *fileService) putManaged(ctx context.Context, upload validatedManagedFile) (string, error) {
	if upload.File == nil {
		return "", fmt.Errorf("upload file is not configured")
	}
	body, err := upload.File.Open()
	if err != nil {
		return "", err
	}
	defer body.Close()

	if s.remote != nil {
		if err := s.remote.Put(ctx, upload.ObjectKey, body, upload.Size, upload.ContentType); err != nil {
			return "", err
		}
		return minioStorageName, nil
	}
	if err := s.local.Put(ctx, upload.ObjectKey, body, upload.Size, upload.ContentType); err != nil {
		return "", err
	}
	return localStorageName, nil
}

func (s *fileService) localPath(objectKey string) (string, bool) {
	localPath, err := s.local.Path(objectKey)
	if err != nil {
		return "", false
	}
	if _, err := os.Stat(localPath); err != nil {
		return "", false
	}
	return localPath, true
}

func (s *fileService) openRemote(ctx context.Context, objectKey string) (io.ReadCloser, storage.ObjectInfo, error) {
	if s.remote == nil {
		return nil, storage.ObjectInfo{}, os.ErrNotExist
	}
	return s.remote.Open(ctx, objectKey)
}

func (s *fileService) createManagedFile(ctx context.Context, upload validatedManagedFile, userID int64) (fileRecord, error) {
	if s.repository == nil {
		return fileRecord{}, fmt.Errorf("%w: repository is not configured", errFileMetadata)
	}
	now := time.Now()
	record := fileRecord{
		ObjectKey:    upload.ObjectKey,
		OriginalName: upload.OriginalName,
		Extension:    upload.Extension,
		ContentType:  upload.ContentType,
		Size:         upload.Size,
		SHA256:       upload.SHA256,
		Storage:      fileStatusPending,
		Purpose:      upload.Purpose,
		Visibility:   upload.Visibility,
		Status:       fileStatusPending,
		CreatedBy:    userID,
		CreatedAt:    now,
		UpdatedAt:    now,
		ExpiresAt:    upload.ExpiresAt,
	}
	if err := s.repository.Create(&record); err != nil {
		return fileRecord{}, fmt.Errorf("%w: create pending record: %v", errFileMetadata, err)
	}

	storageName, err := s.putManaged(ctx, upload)
	if err != nil {
		_ = s.repository.UpdateStatus(record.ID, fileStatusFailed, "", "", nil)
		return fileRecord{}, fmt.Errorf("%w: %v", errFileStorageUnavailable, err)
	}
	bucket := ""
	if storageName == minioStorageName {
		bucket = s.remoteBucket
	}
	if err := s.repository.UpdateStatus(record.ID, fileStatusReady, storageName, bucket, nil); err != nil {
		cleanupErr := s.deleteObject(ctx, storageName, upload.ObjectKey)
		_ = s.repository.UpdateStatus(record.ID, fileStatusFailed, storageName, bucket, nil)
		if cleanupErr != nil {
			return fileRecord{}, fmt.Errorf("%w: finalize record: %v; cleanup object: %v", errFileMetadata, err, cleanupErr)
		}
		return fileRecord{}, fmt.Errorf("%w: finalize record: %v", errFileMetadata, err)
	}
	record.Storage = storageName
	record.Bucket = bucket
	record.Status = fileStatusReady
	record.UpdatedAt = time.Now()
	return record, nil
}

func (s *fileService) getManagedFile(id int64) (fileRecord, error) {
	if s.repository == nil {
		return fileRecord{}, fmt.Errorf("%w: repository is not configured", errFileMetadata)
	}
	record, ok, err := s.repository.FindReady(id)
	if err != nil {
		return fileRecord{}, fmt.Errorf("%w: %v", errFileMetadata, err)
	}
	if !ok {
		return fileRecord{}, errFileNotFound
	}
	return record, nil
}

func (s *fileService) openManagedFile(ctx context.Context, id int64) (io.ReadCloser, storage.ObjectInfo, fileRecord, error) {
	record, err := s.getManagedFile(id)
	if err != nil {
		return nil, storage.ObjectInfo{}, fileRecord{}, err
	}
	body, info, err := s.openObject(ctx, record.Storage, record.ObjectKey)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, storage.ObjectInfo{}, fileRecord{}, fmt.Errorf("%w: %v", errFileContentNotFound, err)
		}
		return nil, storage.ObjectInfo{}, fileRecord{}, fmt.Errorf("%w: %v", errFileStorageUnavailable, err)
	}
	return body, info, record, nil
}

func (s *fileService) deleteManagedFile(ctx context.Context, id int64) error {
	record, err := s.getManagedFile(id)
	if err != nil {
		return err
	}
	if err := s.repository.UpdateStatus(id, fileStatusDeleting, "", "", nil); err != nil {
		return fmt.Errorf("%w: mark deleting: %v", errFileMetadata, err)
	}
	if err := s.deleteObject(ctx, record.Storage, record.ObjectKey); err != nil {
		_ = s.repository.UpdateStatus(id, fileStatusReady, "", "", nil)
		return fmt.Errorf("%w: %v", errFileStorageUnavailable, err)
	}
	deletedAt := time.Now()
	if err := s.repository.UpdateStatus(id, fileStatusDeleted, "", "", &deletedAt); err != nil {
		return fmt.Errorf("%w: mark deleted: %v", errFileMetadata, err)
	}
	return nil
}

func (s *fileService) openObject(ctx context.Context, storageName string, objectKey string) (io.ReadCloser, storage.ObjectInfo, error) {
	switch storageName {
	case localStorageName:
		return s.local.Open(ctx, objectKey)
	case minioStorageName:
		if s.remote == nil {
			return nil, storage.ObjectInfo{}, os.ErrNotExist
		}
		return s.remote.Open(ctx, objectKey)
	default:
		return nil, storage.ObjectInfo{}, fmt.Errorf("unsupported storage %q", storageName)
	}
}

func (s *fileService) deleteObject(ctx context.Context, storageName string, objectKey string) error {
	var err error
	switch storageName {
	case localStorageName:
		err = s.local.Delete(ctx, objectKey)
	case minioStorageName:
		if s.remote == nil {
			return fmt.Errorf("minio is not configured")
		}
		err = s.remote.Delete(ctx, objectKey)
	default:
		return fmt.Errorf("unsupported storage %q", storageName)
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
