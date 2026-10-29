package kadmin

import (
	"github.com/GoAdminGroup/go-admin/internal/kadmin/rag/llm"
	"github.com/gin-gonic/gin"
)

// registerRAGRoutes exposes the model-layer readiness summary to operators.
// Model inference itself stays internal; only the redacted configuration is
// returned, never API keys.
func registerRAGRoutes(api *gin.RouterGroup, s *Store) {
	group := api.Group("/rag", s.requireAuth(), s.requireAdmin())
	group.GET("/status", s.ragStatus)
}

func (s *Store) ragStatus(c *gin.Context) {
	if s.ragModels == nil {
		success(c, llm.Status{Framework: "eino"})
		return
	}
	success(c, s.ragModels.Status())
}
