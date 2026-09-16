package storage

import (
	"context"
	"errors"
	"io"
	"net/url"
	"path"
	"strings"
)

var ErrInvalidObjectKey = errors.New("invalid object key")

type ObjectInfo struct {
	ContentType string
	Size        int64
}

type ObjectStore interface {
	Put(ctx context.Context, objectKey string, body io.Reader, size int64, contentType string) error
	Open(ctx context.Context, objectKey string) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, objectKey string) error
}

func CleanObjectKey(value string) (string, error) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "/"))
	cleaned := path.Clean("/" + value)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." || cleaned == "" || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "\x00") {
		return "", ErrInvalidObjectKey
	}
	return cleaned, nil
}

func EscapePath(value string) string {
	segments := strings.Split(strings.Trim(value, "/"), "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}
