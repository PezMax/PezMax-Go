package files

import (
	"os"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/storage"
)

const (
	uploadRoot      = "upload"
	managedFileRoot = "data/files"
	minioRegion     = "us-east-1"
)

type fileStorageSettings struct {
	LocalRoot        string
	ManagedLocalRoot string
	MinioEnabled     bool
	Minio            storage.MinioConfig
}

func loadFileStorageSettings() fileStorageSettings {
	return fileStorageSettings{
		LocalRoot:        uploadRoot,
		ManagedLocalRoot: envString("KADMIN_FILE_LOCAL_ROOT", managedFileRoot),
		MinioEnabled:     envBool("KADMIN_MINIO_ENABLED", false),
		Minio: storage.MinioConfig{
			Endpoints:  uniqueStrings([]string{envString("KADMIN_MINIO_ENDPOINT", "127.0.0.1:19000"), envString("KADMIN_MINIO_INTERNAL_ENDPOINT", "minio:9000")}),
			AccessKey:  envString("KADMIN_MINIO_ACCESS_KEY", "kadmin_minio"),
			SecretKey:  envString("KADMIN_MINIO_SECRET_KEY", "kadmin_minio_pwd"),
			Bucket:     envString("KADMIN_MINIO_BUCKET", "kadmin"),
			UseSSL:     envBool("KADMIN_MINIO_USE_SSL", false),
			Region:     envString("KADMIN_MINIO_REGION", minioRegion),
			PublicBase: envString("KADMIN_MINIO_PUBLIC_BASE", ""),
			Timeout:    3 * time.Second,
		},
	}
}

func envString(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	case "0", "false", "f", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
