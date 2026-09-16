package kadmin

import (
	"net/http"

	_ "github.com/GoAdminGroup/go-admin/internal/kadmin/docs"
	"github.com/gin-gonic/gin"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

const swaggerIndexPath = "/swagger/index.html"

// RegisterSwagger registers the generated KAdmin API specification and UI.
func RegisterSwagger(r *gin.Engine, enabled bool) {
	if !enabled {
		return
	}

	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, swaggerIndexPath)
	})
	r.GET("/swagger/*any", gin.WrapH(httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
		httpSwagger.DocExpansion("list"),
	)))
}
