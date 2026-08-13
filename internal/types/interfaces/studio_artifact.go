package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

type StudioArtifactRepository interface {
	List(ctx context.Context, req types.StudioArtifactListRequest) (*types.StudioArtifactListResponse, error)
	Get(ctx context.Context, tenantID uint64, userID, id string) (*types.StudioArtifact, error)
	Create(ctx context.Context, artifact *types.StudioArtifact) error
	Delete(ctx context.Context, tenantID uint64, userID, id string) (bool, error)
	BatchDelete(ctx context.Context, tenantID uint64, userID string, ids []string) (int64, error)
	NextVersion(ctx context.Context, tenantID uint64, userID, rootID string) (int, error)
}

type StudioArtifactService interface {
	List(ctx context.Context, req types.StudioArtifactListRequest) (*types.StudioArtifactListResponse, error)
	Get(ctx context.Context, tenantID uint64, userID, id string) (*types.StudioArtifact, error)
	Create(ctx context.Context, tenantID uint64, userID string, req types.StudioArtifactCreateRequest) (*types.StudioArtifact, error)
	Regenerate(ctx context.Context, tenantID uint64, userID, id string) (*types.StudioArtifact, error)
	Delete(ctx context.Context, tenantID uint64, userID, id string) error
	BatchDelete(ctx context.Context, tenantID uint64, userID string, ids []string) (int64, error)
}
