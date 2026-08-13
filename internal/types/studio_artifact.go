package types

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	StudioArtifactTypePPT   = "ppt"
	StudioArtifactTypeHTML  = "html"
	StudioArtifactTypeTable = "table"
	StudioArtifactTypeDoc   = "doc"
)

const (
	StudioArtifactStatusReady = "ready"
)

type StudioArtifact struct {
	ID        string `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID  uint64 `json:"tenant_id" gorm:"not null;index"`
	UserID    string `json:"user_id" gorm:"type:varchar(64);not null;index"`
	SessionID string `json:"session_id,omitempty" gorm:"type:varchar(36);not null;default:'';index"`
	ParentID  string `json:"parent_id,omitempty" gorm:"type:varchar(36);not null;default:'';index"`

	Type     string `json:"type" gorm:"type:varchar(32);not null;index"`
	Title    string `json:"title" gorm:"type:varchar(255);not null"`
	Filename string `json:"filename" gorm:"type:varchar(255);not null"`
	MimeType string `json:"mime_type" gorm:"type:varchar(128);not null"`
	Content  string `json:"-" gorm:"type:text;not null"`
	Size     int64  `json:"size" gorm:"not null;default:0"`

	Prompt   string  `json:"prompt,omitempty" gorm:"type:text;not null;default:''"`
	Source   string  `json:"source,omitempty" gorm:"type:varchar(32);not null;default:'studio'"`
	Version  int     `json:"version" gorm:"not null;default:1"`
	Status   string  `json:"status" gorm:"type:varchar(16);not null;default:'ready'"`
	Metadata JSONMap `json:"metadata,omitempty" gorm:"type:jsonb;not null;default:'{}'"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (StudioArtifact) TableName() string {
	return "studio_artifacts"
}

func (a *StudioArtifact) BeforeCreate(_ *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if a.Status == "" {
		a.Status = StudioArtifactStatusReady
	}
	if a.Source == "" {
		a.Source = "studio"
	}
	if a.Version <= 0 {
		a.Version = 1
	}
	if a.Metadata == nil {
		a.Metadata = JSONMap{}
	}
	return nil
}

func IsValidStudioArtifactType(t string) bool {
	switch t {
	case StudioArtifactTypePPT, StudioArtifactTypeHTML, StudioArtifactTypeTable, StudioArtifactTypeDoc:
		return true
	default:
		return false
	}
}

type StudioArtifactListRequest struct {
	Type     string
	Limit    int
	Offset   int
	Query    string
	TenantID uint64
	UserID   string
}

type StudioArtifactListResponse struct {
	Items  []*StudioArtifact `json:"items"`
	Total  int64             `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

type StudioArtifactCreateRequest struct {
	Type      string `json:"type" binding:"required"`
	Title     string `json:"title,omitempty"`
	Prompt    string `json:"prompt,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Source    string `json:"source,omitempty"`
}
