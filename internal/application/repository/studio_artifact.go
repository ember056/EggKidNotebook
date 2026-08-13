package repository

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

type studioArtifactRepository struct {
	db *gorm.DB
}

func NewStudioArtifactRepository(db *gorm.DB) interfaces.StudioArtifactRepository {
	return &studioArtifactRepository{db: db}
}

func (r *studioArtifactRepository) List(
	ctx context.Context, req types.StudioArtifactListRequest,
) (*types.StudioArtifactListResponse, error) {
	query := r.db.WithContext(ctx).
		Model(&types.StudioArtifact{}).
		Where("tenant_id = ? AND user_id = ?", req.TenantID, req.UserID)
	if req.Type != "" {
		query = query.Where("type = ?", req.Type)
	}
	if req.Query != "" {
		like := "%" + req.Query + "%"
		query = query.Where("(title LIKE ? OR filename LIKE ? OR prompt LIKE ?)", like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var items []*types.StudioArtifact
	if err := query.
		Order("created_at DESC").
		Limit(req.Limit).
		Offset(req.Offset).
		Find(&items).Error; err != nil {
		return nil, err
	}

	return &types.StudioArtifactListResponse{
		Items:  items,
		Total:  total,
		Limit:  req.Limit,
		Offset: req.Offset,
	}, nil
}

func (r *studioArtifactRepository) Get(
	ctx context.Context, tenantID uint64, userID, id string,
) (*types.StudioArtifact, error) {
	var artifact types.StudioArtifact
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND user_id = ?", id, tenantID, userID).
		First(&artifact).Error
	if err != nil {
		return nil, err
	}
	return &artifact, nil
}

func (r *studioArtifactRepository) Create(ctx context.Context, artifact *types.StudioArtifact) error {
	return r.db.WithContext(ctx).Create(artifact).Error
}

func (r *studioArtifactRepository) Delete(
	ctx context.Context, tenantID uint64, userID, id string,
) (bool, error) {
	res := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND user_id = ?", id, tenantID, userID).
		Delete(&types.StudioArtifact{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *studioArtifactRepository) BatchDelete(
	ctx context.Context, tenantID uint64, userID string, ids []string,
) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND id IN ?", tenantID, userID, ids).
		Delete(&types.StudioArtifact{})
	return res.RowsAffected, res.Error
}

func (r *studioArtifactRepository) NextVersion(
	ctx context.Context, tenantID uint64, userID, rootID string,
) (int, error) {
	if rootID == "" {
		return 1, nil
	}
	var artifact types.StudioArtifact
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND (id = ? OR parent_id = ?)", tenantID, userID, rootID, rootID).
		Order("version DESC").
		First(&artifact).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 1, nil
		}
		return 0, err
	}
	return artifact.Version + 1, nil
}
