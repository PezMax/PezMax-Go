package files

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	avatarMaxSize   = 2 << 20
	avatarUploadDir = "avatars"
	maxFileSize     = 50 << 20
	maxRequestSize  = maxFileSize + (1 << 20)
)

const (
	filePurposeAvatar      = "avatar"
	filePurposeEditorImage = "editor-image"
	filePurposeAttachment  = "attachment"
	filePurposeImportTemp  = "import-temp"

	fileVisibilityPublic  = "public"
	fileVisibilityPrivate = "private"

	fileStatusPending  = "pending"
	fileStatusReady    = "ready"
	fileStatusFailed   = "failed"
	fileStatusDeleting = "deleting"
	fileStatusDeleted  = "deleted"
)

type validatedAvatar struct {
	Body        []byte
	ContentType string
	Name        string
	ObjectKey   string
}

type managedFilePolicy struct {
	Purpose      string
	Directory    string
	Visibility   string
	MaxSize      int64
	AllowedTypes map[string]map[string]bool
	ExpiresAfter time.Duration
}

type validatedManagedFile struct {
	File         *multipart.FileHeader
	Size         int64
	ContentType  string
	Extension    string
	OriginalName string
	ObjectKey    string
	Purpose      string
	Visibility   string
	SHA256       string
	ExpiresAt    *time.Time
}

type fileRequestError struct {
	Status  int
	Message string
}

func (e *fileRequestError) Error() string {
	return e.Message
}

var managedFilePolicies = map[string]managedFilePolicy{
	filePurposeAvatar: {
		Purpose:      filePurposeAvatar,
		Directory:    "avatars",
		Visibility:   fileVisibilityPrivate,
		MaxSize:      avatarMaxSize,
		AllowedTypes: imageFileTypes(true),
	},
	filePurposeEditorImage: {
		Purpose:      filePurposeEditorImage,
		Directory:    "editor-images",
		Visibility:   fileVisibilityPrivate,
		MaxSize:      10 << 20,
		AllowedTypes: imageFileTypes(false),
	},
	filePurposeAttachment: {
		Purpose:    filePurposeAttachment,
		Directory:  "attachments",
		Visibility: fileVisibilityPrivate,
		MaxSize:    maxFileSize,
		AllowedTypes: mergeFileTypes(
			imageFileTypes(true),
			map[string]map[string]bool{
				"application/json": {".json": true},
				"application/pdf":  {".pdf": true},
				"application/zip":  {".docx": true, ".xlsx": true, ".zip": true},
				"text/csv":         {".csv": true},
				"text/plain":       {".csv": true, ".json": true, ".txt": true},
			},
		),
	},
	filePurposeImportTemp: {
		Purpose:      filePurposeImportTemp,
		Directory:    "imports",
		Visibility:   fileVisibilityPrivate,
		MaxSize:      20 << 20,
		ExpiresAfter: 24 * time.Hour,
		AllowedTypes: map[string]map[string]bool{
			"application/zip": {".xlsx": true},
			"text/csv":        {".csv": true},
			"text/plain":      {".csv": true},
		},
	},
}

func validateManagedFile(file *multipart.FileHeader, purpose string) (validatedManagedFile, error) {
	policy, ok := managedFilePolicies[strings.TrimSpace(purpose)]
	if !ok {
		return validatedManagedFile{}, &fileRequestError{Status: http.StatusBadRequest, Message: "invalid file purpose"}
	}
	if file == nil || file.Size <= 0 {
		return validatedManagedFile{}, &fileRequestError{Status: http.StatusBadRequest, Message: "file is required"}
	}
	if file.Size > policy.MaxSize {
		return validatedManagedFile{}, &fileRequestError{Status: http.StatusRequestEntityTooLarge, Message: "file is too large"}
	}

	opened, err := file.Open()
	if err != nil {
		return validatedManagedFile{}, &fileRequestError{Status: http.StatusBadRequest, Message: "open file failed"}
	}
	defer opened.Close()
	head := make([]byte, 512)
	headSize, readErr := io.ReadFull(opened, head)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return validatedManagedFile{}, &fileRequestError{Status: http.StatusBadRequest, Message: "read file failed"}
	}
	if headSize == 0 {
		return validatedManagedFile{}, &fileRequestError{Status: http.StatusBadRequest, Message: "file is required"}
	}

	hash := sha256.New()
	if _, err := hash.Write(head[:headSize]); err != nil {
		return validatedManagedFile{}, &fileRequestError{Status: http.StatusBadRequest, Message: "read file failed"}
	}
	remaining, err := io.Copy(hash, io.LimitReader(opened, policy.MaxSize+1-int64(headSize)))
	if err != nil {
		return validatedManagedFile{}, &fileRequestError{Status: http.StatusBadRequest, Message: "read file failed"}
	}
	actualSize := int64(headSize) + remaining
	if actualSize > policy.MaxSize {
		return validatedManagedFile{}, &fileRequestError{Status: http.StatusRequestEntityTooLarge, Message: "file is too large"}
	}

	contentType := detectedMediaType(head[:headSize])
	extension := strings.ToLower(filepath.Ext(file.Filename))
	allowedExtensions, ok := policy.AllowedTypes[contentType]
	if !ok || !allowedExtensions[extension] {
		return validatedManagedFile{}, &fileRequestError{Status: http.StatusUnsupportedMediaType, Message: "file type is not allowed"}
	}

	now := time.Now()
	var expiresAt *time.Time
	if policy.ExpiresAfter > 0 {
		expires := now.Add(policy.ExpiresAfter)
		expiresAt = &expires
	}
	return validatedManagedFile{
		File:         file,
		Size:         actualSize,
		ContentType:  contentType,
		Extension:    extension,
		OriginalName: safeOriginalFilename(file.Filename),
		ObjectKey:    path.Join(policy.Directory, now.Format("2006/01/02"), uuid.NewString()+extension),
		Purpose:      policy.Purpose,
		Visibility:   policy.Visibility,
		SHA256:       hex.EncodeToString(hash.Sum(nil)),
		ExpiresAt:    expiresAt,
	}, nil
}

func detectedMediaType(body []byte) string {
	contentType := http.DetectContentType(body)
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		return strings.ToLower(mediaType)
	}
	return strings.ToLower(strings.TrimSpace(contentType))
}

func safeOriginalFilename(filename string) string {
	filename = path.Base(strings.ReplaceAll(filename, "\\", "/"))
	filename = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, filename))
	if filename == "" || filename == "." {
		return "file"
	}
	runes := []rune(filename)
	if len(runes) > 255 {
		filename = string(runes[:255])
	}
	return filename
}

func imageFileTypes(includeBMP bool) map[string]map[string]bool {
	result := map[string]map[string]bool{
		"image/gif":  {".gif": true},
		"image/jpeg": {".jpeg": true, ".jpg": true},
		"image/png":  {".png": true},
		"image/webp": {".webp": true},
	}
	if includeBMP {
		result["image/bmp"] = map[string]bool{".bmp": true}
	}
	return result
}

func mergeFileTypes(groups ...map[string]map[string]bool) map[string]map[string]bool {
	result := make(map[string]map[string]bool)
	for _, group := range groups {
		for contentType, extensions := range group {
			if result[contentType] == nil {
				result[contentType] = make(map[string]bool)
			}
			for extension := range extensions {
				result[contentType][extension] = true
			}
		}
	}
	return result
}

func validateAvatar(file *multipart.FileHeader) (validatedAvatar, error) {
	if file == nil {
		return validatedAvatar{}, fmt.Errorf("avatar file is required")
	}
	if file.Size <= 0 || file.Size > avatarMaxSize {
		return validatedAvatar{}, fmt.Errorf("avatar image must be smaller than 2MB")
	}

	opened, err := file.Open()
	if err != nil {
		return validatedAvatar{}, fmt.Errorf("open avatar file failed")
	}
	defer opened.Close()

	body, err := io.ReadAll(io.LimitReader(opened, avatarMaxSize+1))
	if err != nil || len(body) == 0 || int64(len(body)) > avatarMaxSize {
		return validatedAvatar{}, fmt.Errorf("read avatar file failed")
	}

	contentType := http.DetectContentType(body)
	if !strings.HasPrefix(contentType, "image/") {
		return validatedAvatar{}, fmt.Errorf("avatar must be an image")
	}
	ext := safeImageExt(file.Filename, contentType)
	return validatedAvatar{
		Body:        body,
		ContentType: contentType,
		Name:        file.Filename,
		ObjectKey:   path.Join(avatarUploadDir, time.Now().Format("20060102"), uuid.NewString()+ext),
	}, nil
}

func safeImageExt(filename string, contentType string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg":
		return ext
	}
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	default:
		return ".img"
	}
}
