package files

import "time"

type uploadedFileResponse struct {
	URL     string `json:"url"`
	Storage string `json:"storage"`
	Name    string `json:"name"`
}

type fileRecord struct {
	ID           int64
	ObjectKey    string
	OriginalName string
	Extension    string
	ContentType  string
	Size         int64
	SHA256       string
	Storage      string
	Bucket       string
	Purpose      string
	Visibility   string
	Status       string
	CreatedBy    int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
	ExpiresAt    *time.Time
}

type managedFileResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	Extension   string `json:"extension"`
	Storage     string `json:"storage"`
	Purpose     string `json:"purpose"`
	Visibility  string `json:"visibility"`
	Status      string `json:"status"`
	CreatedBy   int64  `json:"createdBy"`
	CreatedAt   string `json:"createdAt"`
	ExpiresAt   string `json:"expiresAt,omitempty"`
}

func fileResponse(record fileRecord) managedFileResponse {
	response := managedFileResponse{
		ID:          record.ID,
		Name:        record.OriginalName,
		URL:         managedFileContentURL(record.ID),
		Size:        record.Size,
		ContentType: record.ContentType,
		Extension:   record.Extension,
		Storage:     record.Storage,
		Purpose:     record.Purpose,
		Visibility:  record.Visibility,
		Status:      record.Status,
		CreatedBy:   record.CreatedBy,
		CreatedAt:   record.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if record.ExpiresAt != nil {
		response.ExpiresAt = record.ExpiresAt.Format("2006-01-02 15:04:05")
	}
	return response
}
