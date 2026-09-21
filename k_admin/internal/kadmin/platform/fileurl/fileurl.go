// Package fileurl manages the canonical public form of ptmj_* object URLs.
//
// Write side (the contract): uploads MUST persist the URL produced by
// Builder.ObjectURL — publicBase/bucket/objectKey — so end-user machines can
// fetch files directly from MinIO. Internal MinIO addresses (minio:9000,
// 127.0.0.1, …) must never be written to PostgreSQL.
//
// Read side: rows imported from the legacy RuoYi database may still carry
// internal hosts or the minio:// scheme. Parse extracts the bucket/object
// pair from any stored form, host-agnostic, so server-side downloads and
// deletions keep working before the deferred one-off ETL fixes those rows.
// Bulk-fixing legacy URLs is that ETL's job, not a runtime rewrite concern.
package fileurl

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// ErrUnsupportedURL reports a stored value that is neither an absolute
// http(s) URL nor a minio:// URL (relative legacy paths have no bucket to
// identify and cannot be parsed).
var ErrUnsupportedURL = errors.New("fileurl: unsupported object URL")

// Builder composes canonical public object URLs for the configured bucket.
type Builder struct {
	publicBase *url.URL
	bucket     string
}

// NewBuilder validates the public endpoint and returns a builder. The public
// base must be reachable from end-user machines (e.g. http://files.example.com:29000);
// any path portion is treated as a prefix and preserved.
func NewBuilder(bucket, publicBase string) (*Builder, error) {
	publicBase = strings.TrimSpace(publicBase)
	if publicBase == "" {
		return nil, fmt.Errorf("fileurl: public endpoint is required")
	}
	parsed, err := url.Parse(publicBase)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("fileurl: invalid public endpoint %q", publicBase)
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return &Builder{
		publicBase: parsed,
		bucket:     strings.Trim(strings.TrimSpace(bucket), "/"),
	}, nil
}

// FromEnv builds a builder from KADMIN_MINIO_BUCKET and
// KADMIN_MINIO_PUBLIC_BASE. It fails when KADMIN_MINIO_PUBLIC_BASE is unset,
// which is intentional: without it the write-side contract cannot be honored.
func FromEnv(getenv func(key, fallback string) string) (*Builder, error) {
	return NewBuilder(getenv("KADMIN_MINIO_BUCKET", "kadmin"), getenv("KADMIN_MINIO_PUBLIC_BASE", ""))
}

// Bucket returns the bucket URLs are composed against.
func (b *Builder) Bucket() string {
	return b.bucket
}

// ObjectURL renders the canonical public URL of an object key. The key is
// cleaned against traversal and percent-escaped per segment.
func (b *Builder) ObjectURL(objectKey string) (string, error) {
	key, err := cleanObjectKey(objectKey)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(b.publicBase.String(), "/") + "/" + b.bucket + "/" + escapePath(key), nil
}

// MustObjectURL is ObjectURL without the error for log/sample use; invalid
// keys render as an empty string.
func (b *Builder) MustObjectURL(objectKey string) string {
	value, _ := b.ObjectURL(objectKey)
	return value
}

// Describe renders a compact summary for startup logs.
func (b *Builder) Describe() string {
	return fmt.Sprintf("bucket=%s public=%s", b.bucket, b.publicBase.String())
}

// Parse splits a stored object URL into its bucket and object key without
// caring which host the URL points at. Accepted forms:
//
//	minio://bucket/object/key
//	https://any-host[:port]/bucket/object/key
//
// The object key is returned percent-decoded and cleaned. Values that carry
// no bucket/key structure (relative paths, data:, empty) fail with
// ErrUnsupportedURL.
func Parse(rawURL string) (bucket string, objectKey string, err error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", "", fmt.Errorf("%w: empty value", ErrUnsupportedURL)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrUnsupportedURL, err)
	}

	var segments []string
	switch {
	case parsed.Scheme == "minio":
		if parsed.Host != "" {
			segments = append(segments, parsed.Host)
		}
		segments = append(segments, splitPath(parsed.Path)...)
	case parsed.Scheme == "http" || parsed.Scheme == "https":
		segments = splitPath(parsed.Path)
	default:
		return "", "", fmt.Errorf("%w: scheme %q", ErrUnsupportedURL, parsed.Scheme)
	}
	if len(segments) < 2 {
		return "", "", fmt.Errorf("%w: no object key in %q", ErrUnsupportedURL, rawURL)
	}
	key, err := cleanObjectKey(strings.Join(segments[1:], "/"))
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrUnsupportedURL, err)
	}
	return segments[0], key, nil
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

// The two helpers below mirror platform/storage key handling without
// importing it, keeping this package dependency-free.

func cleanObjectKey(value string) (string, error) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "/"))
	cleaned := path.Clean("/" + value)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." || cleaned == "" || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "\x00") {
		return "", errors.New("invalid object key")
	}
	return cleaned, nil
}

func escapePath(value string) string {
	segments := strings.Split(strings.Trim(value, "/"), "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}
