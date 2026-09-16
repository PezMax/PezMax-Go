package kadmin

import "github.com/GoAdminGroup/go-admin/internal/kadmin/modules/jobs"

// SwaggerResponse documents the stable KAdmin success envelope.
type SwaggerResponse struct {
	Code    int         `json:"code" example:"0"`
	Message string      `json:"message" example:"ok"`
	Msg     string      `json:"msg" example:"ok"`
	Data    interface{} `json:"data"`
}

// SwaggerErrorResponse documents the stable KAdmin error envelope.
type SwaggerErrorResponse struct {
	Code    int         `json:"code" example:"400"`
	Message string      `json:"message" example:"invalid request"`
	Msg     string      `json:"msg" example:"invalid request"`
	Data    interface{} `json:"data"`
}

// SwaggerJobPayload exposes the scheduled task request model to the generator.
type SwaggerJobPayload = jobs.JobPayload
