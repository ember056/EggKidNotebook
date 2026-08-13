import { del, get, getDown, post } from '@/utils/request'

export type StudioArtifactType = 'ppt' | 'html' | 'table' | 'doc'

export interface StudioArtifact {
  id: string
  tenant_id: number
  user_id: string
  session_id?: string
  parent_id?: string
  type: StudioArtifactType
  title: string
  filename: string
  mime_type: string
  size: number
  prompt?: string
  source?: string
  version: number
  status: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface StudioArtifactListResponse {
  items: StudioArtifact[]
  total: number
  limit: number
  offset: number
}

export interface CreateStudioArtifactRequest {
  type: StudioArtifactType
  title?: string
  prompt?: string
  session_id?: string
  source?: string
}

export function listStudioArtifacts(params: { type?: string; q?: string; limit?: number; offset?: number } = {}) {
  const search = new URLSearchParams()
  if (params.type) search.set('type', params.type)
  if (params.q) search.set('q', params.q)
  if (params.limit) search.set('limit', String(params.limit))
  if (typeof params.offset === 'number') search.set('offset', String(params.offset))
  const suffix = search.toString() ? `?${search.toString()}` : ''
  return get<{ success: boolean; data: StudioArtifactListResponse }>(`/api/v1/studio/artifacts${suffix}`)
}

export function createStudioArtifact(payload: CreateStudioArtifactRequest) {
  return post<{ success: boolean; data: StudioArtifact }>('/api/v1/studio/artifacts', payload)
}

export function regenerateStudioArtifact(id: string) {
  return post<{ success: boolean; data: StudioArtifact }>(
    `/api/v1/studio/artifacts/${encodeURIComponent(id)}/regenerate`,
    {},
  )
}

export function deleteStudioArtifact(id: string) {
  return del(`/api/v1/studio/artifacts/${encodeURIComponent(id)}`)
}

export function batchDeleteStudioArtifacts(ids: string[]) {
  return del<{ success: boolean; data: { deleted: number } }>('/api/v1/studio/artifacts', { ids })
}

export function downloadStudioArtifact(id: string) {
  return getDown(`/api/v1/studio/artifacts/${encodeURIComponent(id)}/download`)
}

export function previewStudioArtifact(id: string) {
  return getDown(`/api/v1/studio/artifacts/${encodeURIComponent(id)}/preview`)
}
