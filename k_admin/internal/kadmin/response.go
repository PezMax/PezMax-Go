package kadmin

import (
	"github.com/GoAdminGroup/go-admin/internal/kadmin/transport/httpx"
	"github.com/gin-gonic/gin"
)

func success(c *gin.Context, data interface{}) {
	httpx.Success(c, data)
}

func fail(c *gin.Context, status int, message string) {
	httpx.Fail(c, status, message)
}

func cors() gin.HandlerFunc {
	return httpx.CORS()
}
