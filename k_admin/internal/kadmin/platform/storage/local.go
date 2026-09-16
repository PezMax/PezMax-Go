package storage

import (
	"context"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

type Local struct {
	root string
}

func NewLocal(root string) *Local {
	return &Local{root: root}
}

func (l *Local) Put(_ context.Context, objectKey string, body io.Reader, _ int64, _ string) error {
	target, err := l.Path(objectKey)
	if err != nil {
		return err
	}
	directory := filepath.Dir(target)
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".kadmin-upload-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	copyErr := error(nil)
	if err := temporary.Chmod(0644); err != nil {
		copyErr = err
	} else {
		_, copyErr = io.Copy(temporary, body)
	}
	closeErr := temporary.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(temporaryName, target)
}

func (l *Local) Open(_ context.Context, objectKey string) (io.ReadCloser, ObjectInfo, error) {
	target, err := l.Path(objectKey)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	file, err := os.Open(target)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, ObjectInfo{}, err
	}
	return file, ObjectInfo{
		ContentType: mime.TypeByExtension(filepath.Ext(target)),
		Size:        info.Size(),
	}, nil
}

func (l *Local) Delete(_ context.Context, objectKey string) error {
	target, err := l.Path(objectKey)
	if err != nil {
		return err
	}
	return os.Remove(target)
}

func (l *Local) Path(objectKey string) (string, error) {
	cleaned, err := CleanObjectKey(objectKey)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(l.root)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(cleaned)))
	if err != nil {
		return "", err
	}
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return "", ErrInvalidObjectKey
	}
	return target, nil
}
