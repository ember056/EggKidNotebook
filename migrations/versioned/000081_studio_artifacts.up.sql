CREATE TABLE IF NOT EXISTS studio_artifacts (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    session_id VARCHAR(36) NOT NULL DEFAULT '',
    parent_id VARCHAR(36) NOT NULL DEFAULT '',
    type VARCHAR(32) NOT NULL,
    title VARCHAR(255) NOT NULL,
    filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(128) NOT NULL,
    content TEXT NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    prompt TEXT NOT NULL DEFAULT '',
    source VARCHAR(32) NOT NULL DEFAULT 'studio',
    version INTEGER NOT NULL DEFAULT 1,
    status VARCHAR(16) NOT NULL DEFAULT 'ready',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

CREATE INDEX IF NOT EXISTS idx_studio_artifacts_tenant_user_created
    ON studio_artifacts(tenant_id, user_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_studio_artifacts_type
    ON studio_artifacts(tenant_id, user_id, type)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_studio_artifacts_parent
    ON studio_artifacts(parent_id)
    WHERE deleted_at IS NULL;
