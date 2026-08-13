package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterStudioRoutes(r *gin.RouterGroup, h *handler.StudioArtifactHandler, g *rbacGuards) {
	if h == nil {
		return
	}
	studio := r.Group("/studio", g.Viewer())
	{
		studio.GET("/artifacts", h.ListArtifacts)
		studio.POST("/artifacts", h.CreateArtifact)
		studio.DELETE("/artifacts", h.BatchDeleteArtifacts)
		studio.POST("/artifacts/:id/regenerate", h.RegenerateArtifact)
		studio.GET("/artifacts/:id/preview", h.PreviewArtifact)
		studio.GET("/artifacts/:id/download", h.DownloadArtifact)
		studio.DELETE("/artifacts/:id", h.DeleteArtifact)
	}
}
