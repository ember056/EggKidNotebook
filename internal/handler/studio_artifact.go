package handler

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type StudioArtifactHandler struct {
	service interfaces.StudioArtifactService
}

func NewStudioArtifactHandler(svc interfaces.StudioArtifactService) *StudioArtifactHandler {
	return &StudioArtifactHandler{service: svc}
}

func studioContext(c *gin.Context) (string, uint64, bool) {
	uidVal, ok := c.Get(types.UserIDContextKey.String())
	if !ok {
		c.Error(apperrors.NewUnauthorizedError("user ID not found"))
		return "", 0, false
	}
	userID, _ := uidVal.(string)
	if userID == "" {
		c.Error(apperrors.NewUnauthorizedError("user ID not found"))
		return "", 0, false
	}
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		c.Error(apperrors.NewUnauthorizedError("workspace ID not found"))
		return "", 0, false
	}
	return userID, tenantID, true
}

func (h *StudioArtifactHandler) ListArtifacts(c *gin.Context) {
	ctx := c.Request.Context()
	userID, tenantID, ok := studioContext(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	resp, err := h.service.List(ctx, types.StudioArtifactListRequest{
		Type:     c.Query("type"),
		Query:    c.Query("q"),
		Limit:    limit,
		Offset:   offset,
		TenantID: tenantID,
		UserID:   userID,
	})
	if err != nil {
		h.writeStudioError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

func (h *StudioArtifactHandler) CreateArtifact(c *gin.Context) {
	ctx := c.Request.Context()
	userID, tenantID, ok := studioContext(c)
	if !ok {
		return
	}
	var req types.StudioArtifactCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid request body").WithDetails(err.Error()))
		return
	}
	artifact, err := h.service.Create(ctx, tenantID, userID, req)
	if err != nil {
		h.writeStudioError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": artifact})
}

func (h *StudioArtifactHandler) RegenerateArtifact(c *gin.Context) {
	ctx := c.Request.Context()
	userID, tenantID, ok := studioContext(c)
	if !ok {
		return
	}
	artifact, err := h.service.Regenerate(ctx, tenantID, userID, c.Param("id"))
	if err != nil {
		h.writeStudioError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": artifact})
}

func (h *StudioArtifactHandler) PreviewArtifact(c *gin.Context) {
	h.serveArtifact(c, false)
}

func (h *StudioArtifactHandler) DownloadArtifact(c *gin.Context) {
	h.serveArtifact(c, true)
}

func (h *StudioArtifactHandler) DeleteArtifact(c *gin.Context) {
	ctx := c.Request.Context()
	userID, tenantID, ok := studioContext(c)
	if !ok {
		return
	}
	if err := h.service.Delete(ctx, tenantID, userID, c.Param("id")); err != nil {
		h.writeStudioError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type batchDeleteStudioArtifactsRequest struct {
	IDs []string `json:"ids"`
}

func (h *StudioArtifactHandler) BatchDeleteArtifacts(c *gin.Context) {
	ctx := c.Request.Context()
	userID, tenantID, ok := studioContext(c)
	if !ok {
		return
	}
	var req batchDeleteStudioArtifactsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid request body").WithDetails(err.Error()))
		return
	}
	deleted, err := h.service.BatchDelete(ctx, tenantID, userID, req.IDs)
	if err != nil {
		h.writeStudioError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"deleted": deleted}})
}

func (h *StudioArtifactHandler) serveArtifact(c *gin.Context, attachment bool) {
	ctx := c.Request.Context()
	userID, tenantID, ok := studioContext(c)
	if !ok {
		return
	}
	artifact, err := h.service.Get(ctx, tenantID, userID, c.Param("id"))
	if err != nil {
		h.writeStudioError(c, err)
		return
	}
	disposition := "inline"
	if attachment {
		disposition = "attachment"
	}
	filename := url.QueryEscape(artifact.Filename)
	c.Header("Content-Type", artifact.MimeType)
	c.Header("Content-Disposition", fmt.Sprintf("%s; filename*=UTF-8''%s", disposition, filename))
	c.String(http.StatusOK, artifact.Content)
}

func (h *StudioArtifactHandler) writeStudioError(c *gin.Context, err error) {
	switch {
	case stderrors.Is(err, service.ErrStudioArtifactInvalidType),
		stderrors.Is(err, service.ErrStudioArtifactEmptyID),
		stderrors.Is(err, service.ErrStudioArtifactEmptyIDs):
		c.Error(apperrors.NewBadRequestError(err.Error()))
	case stderrors.Is(err, service.ErrStudioArtifactNotFound):
		c.Error(apperrors.NewNotFoundError(err.Error()))
	default:
		logger.ErrorWithFields(c.Request.Context(), err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
	}
}
