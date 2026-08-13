<template>
  <aside
    class="studio-sidebar"
    :class="{ 'is-collapsed': collapsed }"
    aria-label="Studio office workspace"
  >
    <button
      type="button"
      class="studio-toggle"
      :aria-label="collapsed ? '展开 Studio' : '收起 Studio'"
      @click="toggleCollapsed"
    >
      <span class="studio-toggle__spark">✦</span>
      <span v-if="!collapsed" class="studio-toggle__text">EggKid Studio</span>
      <t-icon :name="collapsed ? 'chevron-left' : 'chevron-right'" size="16px" />
    </button>

    <transition name="studio-content-fade">
      <div v-if="!collapsed" class="studio-panel">
        <header class="studio-header">
          <div>
            <p class="studio-eyebrow">Office Copilot</p>
            <h2>办公产出工作台</h2>
          </div>
          <span class="studio-badge">Beta</span>
        </header>

        <section class="studio-section">
          <div class="studio-section-head">
            <div>
              <div class="studio-section-title">常用生成</div>
              <p class="studio-section-desc">生成后会自动保存到下方记录，可预览和下载。</p>
            </div>
          </div>
          <div class="studio-card-grid studio-card-grid--primary">
            <article
              v-for="item in primaryItems"
              :key="item.key"
              class="studio-card studio-card--primary"
            >
              <button
                type="button"
                class="studio-card__main"
                :disabled="disabled"
                @click="useTemplate(item)"
              >
                <span class="studio-card__icon">{{ item.icon }}</span>
                <span class="studio-card__body">
                  <span class="studio-card__title">{{ item.title }}</span>
                  <span class="studio-card__desc">{{ item.description }}</span>
                </span>
              </button>
              <button
                type="button"
                class="studio-card__generate"
                :disabled="disabled || generatingType === item.key"
                @click="generateArtifact(item)"
              >
                <t-icon v-if="generatingType === item.key" name="loading" class="studio-spin" />
                <span>{{ generatingType === item.key ? '生成中' : '生成文件' }}</span>
              </button>
            </article>
          </div>
        </section>

        <section class="studio-section">
          <div class="studio-section-title">辅助分析</div>
          <div class="studio-card-grid studio-card-grid--secondary">
            <button
              v-for="item in secondaryItems"
              :key="item.key"
              type="button"
              class="studio-card studio-card--flat"
              :disabled="disabled"
              @click="useTemplate(item)"
            >
              <span class="studio-card__icon">{{ item.icon }}</span>
              <span class="studio-card__body">
                <span class="studio-card__title">{{ item.title }}</span>
                <span class="studio-card__desc">{{ item.description }}</span>
              </span>
            </button>
          </div>
        </section>

        <section class="studio-section studio-records">
          <div class="studio-section-head">
            <div>
              <div class="studio-section-title">生成记录</div>
              <p class="studio-section-desc">{{ recordSummary }}</p>
            </div>
            <button type="button" class="studio-icon-button" :disabled="recordsLoading" @click="loadArtifacts">
              <t-icon :name="recordsLoading ? 'loading' : 'refresh'" :class="{ 'studio-spin': recordsLoading }" />
            </button>
          </div>

          <div class="studio-record-toolbar">
            <label class="studio-select-all">
              <input
                type="checkbox"
                :checked="allVisibleSelected"
                :disabled="artifacts.length === 0"
                @change="toggleSelectAll"
              />
              <span>全选</span>
            </label>
            <button
              type="button"
              class="studio-danger-button"
              :disabled="selectedIds.size === 0 || deleting"
              @click="deleteSelected"
            >
              批量删除
            </button>
          </div>

          <div v-if="recordsLoading && artifacts.length === 0" class="studio-empty">正在加载记录…</div>
          <div v-else-if="artifacts.length === 0" class="studio-empty">
            还没有生成记录。先点上面的“生成文件”，这里就会长出小仓库。
          </div>
          <div v-else class="studio-record-list">
            <article v-for="artifact in artifacts" :key="artifact.id" class="studio-record">
              <label class="studio-record__check">
                <input
                  type="checkbox"
                  :checked="selectedIds.has(artifact.id)"
                  @change="toggleSelected(artifact.id)"
                />
              </label>
              <div class="studio-record__main">
                <div class="studio-record__top">
                  <span class="studio-record__type">{{ typeLabel(artifact.type) }}</span>
                  <span class="studio-record__version">v{{ artifact.version }}</span>
                </div>
                <div class="studio-record__title" :title="artifact.title">{{ artifact.title }}</div>
                <div class="studio-record__meta">
                  {{ formatTime(artifact.created_at) }} · {{ formatBytes(artifact.size) }}
                </div>
                <div class="studio-record__actions">
                  <button type="button" @click="previewArtifactRecord(artifact)">预览</button>
                  <button type="button" @click="downloadArtifactRecord(artifact)">下载</button>
                  <button type="button" :disabled="regeneratingId === artifact.id" @click="regenerateArtifactRecord(artifact)">
                    {{ regeneratingId === artifact.id ? '生成中' : '再生成' }}
                  </button>
                  <button type="button" class="danger" :disabled="deleting" @click="deleteOne(artifact)">删除</button>
                </div>
              </div>
            </article>
          </div>
        </section>

        <div class="studio-footer">
          <span class="studio-footer__dot"></span>
          点卡片主体会把任务模板发送到对话；点“生成文件”会保存为 Studio 记录。
        </div>
      </div>
    </transition>

    <t-dialog
      v-model:visible="previewVisible"
      width="760px"
      :header="previewArtifact?.title || '预览'"
      placement="center"
      :footer="false"
    >
      <div class="studio-preview">
        <div class="studio-preview__meta">
          <span>{{ previewArtifact?.filename }}</span>
          <button v-if="previewArtifact" type="button" @click="downloadArtifactRecord(previewArtifact)">下载</button>
        </div>
        <iframe
          v-if="previewArtifact?.type === 'html'"
          class="studio-preview__frame"
          :srcdoc="previewContent"
          sandbox=""
        />
        <pre v-else class="studio-preview__text">{{ previewContent }}</pre>
      </div>
    </t-dialog>
  </aside>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  batchDeleteStudioArtifacts,
  createStudioArtifact,
  deleteStudioArtifact,
  downloadStudioArtifact,
  listStudioArtifacts,
  previewStudioArtifact,
  regenerateStudioArtifact,
  type StudioArtifact,
  type StudioArtifactType,
} from '@/api/studio'

interface StudioItem {
  key: StudioArtifactType | 'source-trace' | 'parse-trace'
  icon: string
  title: string
  description: string
  prompt: string
  artifactType?: StudioArtifactType
}

const emit = defineEmits<{
  (event: 'use-template', prompt: string): void
  (event: 'collapse-change', collapsed: boolean): void
}>()

const props = defineProps({
  disabled: { type: Boolean, default: false },
  sessionId: { type: String, default: '' },
})

const STORAGE_KEY = 'eggkid-studio-sidebar-collapsed'
const collapsed = ref(false)
const artifacts = ref<StudioArtifact[]>([])
const selectedIds = ref<Set<string>>(new Set())
const recordsLoading = ref(false)
const deleting = ref(false)
const generatingType = ref<string>('')
const regeneratingId = ref('')
const previewVisible = ref(false)
const previewArtifact = ref<StudioArtifact | null>(null)
const previewContent = ref('')

const primaryItems: StudioItem[] = [
  {
    key: 'ppt',
    artifactType: 'ppt',
    icon: '📊',
    title: 'PPT 制作',
    description: '沉淀汇报大纲、项目复盘、方案评审结构',
    prompt: `请作为企业办公汇报助手，基于当前对话和知识库内容，生成一份 PPT 方案。
要求：
1. 给出标题、受众、汇报目标；
2. 输出 8-12 页页面结构；
3. 每页包含页面标题、核心观点、三条以内要点、建议图表/配图；
4. 最后给出讲稿提示和可能被问到的问题。`,
  },
  {
    key: 'html',
    artifactType: 'html',
    icon: '🧩',
    title: 'HTML 页面',
    description: '生成可预览的单页看板、流程图或说明页',
    prompt: `请作为前端 Artifact 助手，生成一个完整、可运行的 HTML 单文件。
要求：
1. 只使用 HTML/CSS/JavaScript；
2. 页面适合企业内部汇报或知识展示；
3. 视觉简洁，包含响应式布局；
4. 如适合，请加入流程图、表格或指标卡片；
5. 说明如何保存为 .html 并打开预览。`,
  },
  {
    key: 'table',
    artifactType: 'table',
    icon: '📋',
    title: '表格生成',
    description: '生成任务跟进表、风险清单、需求拆解 CSV',
    prompt: `请作为企业表格助手，把当前任务整理成结构化表格。
建议字段：序号、模块、任务、负责人、优先级、状态、截止时间、风险、备注。
要求：
1. 字段清晰、适合复制到 Excel；
2. 对不确定信息用“待补充”标记；
3. 最后补充你建议我继续追问或补齐的关键信息。`,
  },
  {
    key: 'doc',
    artifactType: 'doc',
    icon: '📝',
    title: '办公文档',
    description: '周报、会议纪要、邮件、SOP、复盘草稿',
    prompt: `请作为企业办公文档助手，先判断当前任务适合输出日报、周报、会议纪要、邮件、SOP 还是项目复盘。
然后按所选文档类型输出：
1. 标题；
2. 背景/目标；
3. 关键事项；
4. 风险与阻塞；
5. 下一步计划；
6. 可直接复制使用的正式版本。`,
  },
]

const secondaryItems: StudioItem[] = [
  {
    key: 'source-trace',
    icon: '🔎',
    title: '知识溯源',
    description: '整理引用片段、命中文档和可信度',
    prompt: `请作为 RAG 知识溯源助手，基于当前回答或检索结果，整理一份“知识溯源说明”。
请输出：
1. 回答中最关键的 5 个结论；
2. 每个结论对应的来源文档/片段；
3. 哪些结论证据充分，哪些仍需要人工确认；
4. 如有冲突信息，请列出冲突点和建议处理方式。`,
  },
  {
    key: 'parse-trace',
    icon: '⏱️',
    title: '解析 Trace',
    description: '复盘解析、后处理、索引各节点耗时',
    prompt: `请作为知识库解析链路 Trace 分析助手，帮我设计或复盘一次文档入库流程。
请按节点输出：
1. 文件接收；
2. 文档解析（如 MinerU/PDF/Office）；
3. 清洗与后处理；
4. Chunk 切分；
5. Embedding；
6. Rerank/索引建立；
7. Wiki 摘要/实体概念抽取；
8. 完成或失败原因。
每个节点包含：状态、预估耗时、可能失败原因、排查建议。`,
  },
]

const recordSummary = computed(() => {
  if (artifacts.value.length === 0) return '暂无记录'
  return `共 ${artifacts.value.length} 条，已选择 ${selectedIds.value.size} 条`
})

const allVisibleSelected = computed(() => {
  return artifacts.value.length > 0 && artifacts.value.every((item) => selectedIds.value.has(item.id))
})

const toggleCollapsed = () => {
  collapsed.value = !collapsed.value
  localStorage.setItem(STORAGE_KEY, collapsed.value ? '1' : '0')
  emit('collapse-change', collapsed.value)
}

const useTemplate = (item: StudioItem) => {
  emit('use-template', item.prompt)
}

const loadArtifacts = async () => {
  recordsLoading.value = true
  try {
    const res = await listStudioArtifacts({ limit: 30 })
    artifacts.value = res.data?.items || []
    const visibleIDs = new Set(artifacts.value.map((item) => item.id))
    selectedIds.value = new Set([...selectedIds.value].filter((id) => visibleIDs.has(id)))
  } catch (error: any) {
    MessagePlugin.error(error?.message || '加载 Studio 记录失败')
  } finally {
    recordsLoading.value = false
  }
}

const generateArtifact = async (item: StudioItem) => {
  if (!item.artifactType) return
  generatingType.value = item.key
  try {
    await createStudioArtifact({
      type: item.artifactType,
      title: item.title,
      prompt: item.prompt,
      session_id: props.sessionId,
      source: 'studio',
    })
    MessagePlugin.success(`${item.title} 已生成`)
    await loadArtifacts()
  } catch (error: any) {
    MessagePlugin.error(error?.message || '生成失败')
  } finally {
    generatingType.value = ''
  }
}

const toggleSelected = (id: string) => {
  const next = new Set(selectedIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selectedIds.value = next
}

const toggleSelectAll = () => {
  if (allVisibleSelected.value) {
    selectedIds.value = new Set()
    return
  }
  selectedIds.value = new Set(artifacts.value.map((item) => item.id))
}

const deleteOne = async (artifact: StudioArtifact) => {
  deleting.value = true
  try {
    await deleteStudioArtifact(artifact.id)
    MessagePlugin.success('已删除')
    selectedIds.value.delete(artifact.id)
    selectedIds.value = new Set(selectedIds.value)
    await loadArtifacts()
  } catch (error: any) {
    MessagePlugin.error(error?.message || '删除失败')
  } finally {
    deleting.value = false
  }
}

const deleteSelected = async () => {
  const ids = [...selectedIds.value]
  if (ids.length === 0) return
  deleting.value = true
  try {
    const res = await batchDeleteStudioArtifacts(ids)
    MessagePlugin.success(`已删除 ${res.data?.deleted ?? ids.length} 条记录`)
    selectedIds.value = new Set()
    await loadArtifacts()
  } catch (error: any) {
    MessagePlugin.error(error?.message || '批量删除失败')
  } finally {
    deleting.value = false
  }
}

const regenerateArtifactRecord = async (artifact: StudioArtifact) => {
  regeneratingId.value = artifact.id
  try {
    await regenerateStudioArtifact(artifact.id)
    MessagePlugin.success('已重新生成一个新版本')
    await loadArtifacts()
  } catch (error: any) {
    MessagePlugin.error(error?.message || '重新生成失败')
  } finally {
    regeneratingId.value = ''
  }
}

const previewArtifactRecord = async (artifact: StudioArtifact) => {
  try {
    const blob = await previewStudioArtifact(artifact.id)
    previewContent.value = await blob.text()
    previewArtifact.value = artifact
    previewVisible.value = true
  } catch (error: any) {
    MessagePlugin.error(error?.message || '预览失败')
  }
}

const downloadArtifactRecord = async (artifact: StudioArtifact) => {
  try {
    const blob = await downloadStudioArtifact(artifact.id)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = artifact.filename
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  } catch (error: any) {
    MessagePlugin.error(error?.message || '下载失败')
  }
}

const typeLabel = (type: StudioArtifactType) => {
  switch (type) {
    case 'html':
      return 'HTML'
    case 'table':
      return '表格'
    case 'doc':
      return '文档'
    default:
      return 'PPT'
  }
}

const formatBytes = (size: number) => {
  if (!Number.isFinite(size) || size <= 0) return '0 B'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

const formatTime = (value: string) => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

onMounted(() => {
  collapsed.value = localStorage.getItem(STORAGE_KEY) === '1'
  emit('collapse-change', collapsed.value)
  if (!collapsed.value) {
    loadArtifacts()
  }
})
</script>

<style lang="less" scoped>
.studio-sidebar {
  position: fixed;
  top: 76px;
  right: 16px;
  bottom: 132px;
  z-index: 30;
  width: 340px;
  pointer-events: none;
}

.studio-toggle,
.studio-panel {
  pointer-events: auto;
}

.studio-toggle {
  width: 100%;
  height: 42px;
  padding: 0 12px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 16px;
  background: color-mix(in srgb, var(--td-bg-color-container) 92%, transparent);
  color: var(--td-text-color-primary);
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.08);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  backdrop-filter: blur(14px);
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;

  &:hover {
    border-color: var(--td-brand-color);
    box-shadow: 0 14px 36px rgba(15, 23, 42, 0.12);
    transform: translateY(-1px);
  }
}

.studio-toggle__spark {
  width: 22px;
  height: 22px;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  background: linear-gradient(135deg, #5b8cff, #8b5cf6);
  font-size: 13px;
}

.studio-toggle__text {
  flex: 1;
  text-align: left;
  font-size: 14px;
  font-weight: 700;
}

.studio-panel {
  margin-top: 10px;
  height: calc(100% - 52px);
  padding: 16px;
  box-sizing: border-box;
  border: 1px solid var(--td-component-stroke);
  border-radius: 20px;
  background:
    radial-gradient(circle at 20% 0%, rgba(91, 140, 255, 0.12), transparent 28%),
    var(--td-bg-color-container);
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.12);
  overflow-y: auto;
}

.studio-header,
.studio-section-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.studio-header {
  margin-bottom: 18px;

  h2 {
    margin: 4px 0 0;
    font-size: 18px;
    line-height: 1.35;
    color: var(--td-text-color-primary);
  }
}

.studio-eyebrow,
.studio-section-desc {
  margin: 0;
}

.studio-eyebrow {
  font-size: 12px;
  line-height: 1;
  color: var(--td-brand-color);
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.studio-badge {
  flex-shrink: 0;
  padding: 4px 8px;
  border-radius: 999px;
  font-size: 12px;
  color: var(--td-brand-color);
  background: var(--td-brand-color-light);
}

.studio-section + .studio-section {
  margin-top: 18px;
}

.studio-section-title {
  margin-bottom: 4px;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
  font-weight: 700;
}

.studio-section-desc {
  font-size: 12px;
  line-height: 1.45;
  color: var(--td-text-color-secondary);
}

.studio-card-grid {
  display: grid;
  gap: 10px;
}

.studio-card-grid--primary {
  grid-template-columns: 1fr;
  margin-top: 10px;
}

.studio-card {
  width: 100%;
  min-height: 76px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 16px;
  background: color-mix(in srgb, var(--td-bg-color-container) 86%, var(--td-brand-color-light) 14%);
  color: var(--td-text-color-primary);
  transition: transform 0.18s ease, border-color 0.18s ease, background 0.18s ease;

  &:hover {
    border-color: var(--td-brand-color);
    background: var(--td-bg-color-container-hover);
  }
}

.studio-card--primary {
  overflow: hidden;
}

.studio-card__main,
.studio-card--flat {
  width: 100%;
  padding: 12px;
  border: 0;
  background: transparent;
  text-align: left;
  cursor: pointer;
  display: flex;
  gap: 12px;
  align-items: flex-start;
  color: inherit;

  &:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }
}

.studio-card--flat {
  border: 1px solid var(--td-component-stroke);
}

.studio-card__generate {
  width: calc(100% - 24px);
  margin: 0 12px 12px;
  height: 30px;
  border: 0;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  background: var(--td-brand-color-light);
  color: var(--td-brand-color);
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;

  &:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
}

.studio-card__icon {
  width: 34px;
  height: 34px;
  border-radius: 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--td-bg-color-page);
  font-size: 18px;
  flex-shrink: 0;
}

.studio-card__body {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.studio-card__title {
  font-size: 14px;
  font-weight: 700;
  line-height: 1.3;
}

.studio-card__desc {
  font-size: 12px;
  line-height: 1.45;
  color: var(--td-text-color-secondary);
}

.studio-icon-button {
  width: 30px;
  height: 30px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 10px;
  background: var(--td-bg-color-container);
  color: var(--td-text-color-secondary);
  cursor: pointer;
}

.studio-record-toolbar {
  margin: 10px 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.studio-select-all {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
}

.studio-danger-button {
  border: 0;
  border-radius: 10px;
  padding: 6px 10px;
  background: color-mix(in srgb, var(--td-error-color) 12%, transparent);
  color: var(--td-error-color);
  cursor: pointer;
  font-size: 12px;

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
}

.studio-empty {
  padding: 18px 12px;
  border-radius: 14px;
  background: var(--td-bg-color-page);
  color: var(--td-text-color-secondary);
  font-size: 12px;
  line-height: 1.6;
}

.studio-record-list {
  display: grid;
  gap: 10px;
}

.studio-record {
  display: grid;
  grid-template-columns: 20px 1fr;
  gap: 8px;
  padding: 10px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 14px;
  background: var(--td-bg-color-container);
}

.studio-record__check {
  padding-top: 2px;
}

.studio-record__top,
.studio-record__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.studio-record__type,
.studio-record__version {
  padding: 2px 6px;
  border-radius: 999px;
  font-size: 11px;
  line-height: 1.3;
  background: var(--td-brand-color-light);
  color: var(--td-brand-color);
}

.studio-record__version {
  background: var(--td-bg-color-page);
  color: var(--td-text-color-placeholder);
}

.studio-record__title {
  margin-top: 6px;
  font-size: 13px;
  font-weight: 700;
  color: var(--td-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.studio-record__meta {
  margin-top: 4px;
  font-size: 11px;
  color: var(--td-text-color-placeholder);
}

.studio-record__actions {
  margin-top: 8px;

  button {
    border: 0;
    padding: 0;
    background: transparent;
    color: var(--td-brand-color);
    font-size: 12px;
    cursor: pointer;

    &:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    &.danger {
      color: var(--td-error-color);
    }
  }
}

.studio-footer {
  margin-top: 16px;
  padding: 10px 12px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--td-text-color-secondary);
  background: var(--td-bg-color-page);
}

.studio-footer__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--td-success-color);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--td-success-color) 16%, transparent);
  flex-shrink: 0;
}

.studio-preview__meta {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  margin-bottom: 12px;
  font-size: 12px;
  color: var(--td-text-color-secondary);

  button {
    border: 0;
    background: transparent;
    color: var(--td-brand-color);
    cursor: pointer;
  }
}

.studio-preview__frame {
  width: 100%;
  height: 560px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 12px;
  background: #fff;
}

.studio-preview__text {
  max-height: 560px;
  margin: 0;
  padding: 14px;
  border-radius: 12px;
  background: var(--td-bg-color-page);
  color: var(--td-text-color-primary);
  white-space: pre-wrap;
  word-break: break-word;
  overflow: auto;
}

.studio-spin {
  animation: studio-spin 1s linear infinite;
}

@keyframes studio-spin {
  to {
    transform: rotate(360deg);
  }
}

.studio-sidebar.is-collapsed {
  width: 52px;

  .studio-toggle {
    width: 52px;
    padding: 0;
    justify-content: center;

    :deep(.t-icon) {
      display: none;
    }
  }
}

.studio-content-fade-enter-active,
.studio-content-fade-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.studio-content-fade-enter-from,
.studio-content-fade-leave-to {
  opacity: 0;
  transform: translateX(12px);
}

@media (max-width: 1279px) {
  .studio-sidebar {
    top: 88px;
    right: 12px;
    bottom: 118px;
    width: min(340px, calc(100vw - 24px));
  }

  .studio-sidebar:not(.is-collapsed) {
    z-index: 80;
  }
}

@media (max-width: 720px) {
  .studio-sidebar {
    top: 72px;
    right: 10px;
    bottom: 104px;
    width: min(310px, calc(100vw - 20px));
  }
}
</style>
