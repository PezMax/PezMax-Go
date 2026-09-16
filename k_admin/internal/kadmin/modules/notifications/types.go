package notifications

import (
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/gin-gonic/gin"
)

const (
	ListPermission      = "system:notification:list"
	CreatePermission    = "system:notification:create"
	ReadPermission      = "system:notification:read"
	DeletePermission    = "system:notification:delete"
	ReadAllPermission   = "system:notification:read-all"
	ClearReadPermission = "system:notification:clear-read"
)

// Notification types render with different accents in the frontend.
const (
	TypeInfo    = "info"
	TypeSuccess = "success"
	TypeWarning = "warning"
)

// Dependencies carries the KAdmin services the notifications module needs.
type Dependencies struct {
	Connection        db.Connection
	RequireAuth       gin.HandlerFunc
	RequirePermission func(...string) gin.HandlerFunc
}

// Notification is a persisted in-site message.
type Notification struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Link      string `json:"link"`
	Type      string `json:"type"`
	IsRead    bool   `json:"isRead"`
	CreatedAt string `json:"createdAt"`
}

// Payload creates a notification, used by system event producers.
type Payload struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Link    string `json:"link"`
	Type    string `json:"type"`
}

// Filter carries list query parameters.
type Filter struct {
	Page       int
	PageSize   int
	UnreadOnly bool
}

// Page is a paginated list result with the global unread count.
type Page struct {
	Items    []Notification `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Unread   int64          `json:"unread"`
}
