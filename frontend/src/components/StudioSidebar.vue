<template>
  <aside
    class="studio-sidebar"
    :class="{ 'is-collapsed': collapsed }"
    aria-label="Studio workspace"
    data-studio-drop-surface="true"
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
        <template v-if="activeTool">
          <header class="studio-workspace-header">
            <div>
              <p class="studio-eyebrow">Workspace</p>
              <h2>{{ activeTool.title }}</h2>
              <p class="studio-section-desc">{{ activeTool.description }}</p>
            </div>
            <div class="studio-workspace-actions">
              <t-button variant="outline" size="small" @click="backToHome">返回</t-button>
              <t-button v-if="activeTool.prompt" variant="outline" size="small" @click="sendPromptToChat">发到对话</t-button>
            </div>
          </header>

          <section class="studio-section">
            <div class="studio-section-title">任务说明</div>
            <div class="studio-helper-box">
              {{ activeTool.prompt }}
            </div>
          </section>

          <section v-if="activeTool.artifactType" class="studio-section">
            <div class="studio-section-title">生成配置</div>
            <div class="studio-form-grid">
              <div class="studio-field">
                <label>标题</label>
                <t-input v-model="form.title" :disabled="disabled" placeholder="例如：长鑫存储 AIX 周报" />
              </div>

              <div class="studio-field">
                <label>受众</label>
                <t-select v-model="form.audience" :disabled="disabled" :options="audienceOptions" placeholder="选择受众" />
              </div>

              <div class="studio-field">
                <label>风格</label>
                <t-select v-model="form.style" :disabled="disabled" :options="styleOptions" placeholder="选择风格" />
              </div>

              <div v-if="activeTool.artifactType === 'ppt'" class="studio-field">
                <label>页数</label>
                <t-input-number v-model="form.pageCount" :min="4" :max="20" :disabled="disabled" theme="column" />
              </div>

              <div v-if="activeTool.artifactType === 'doc'" class="studio-field">
                <label>文档类型</label>
                <t-select v-model="form.docType" :disabled="disabled" :options="docTypeOptions" />
              </div>

              <div v-if="activeTool.artifactType === 'html'" class="studio-field">
                <label>页面主题</label>
                <t-select v-model="form.htmlTheme" :disabled="disabled" :options="htmlThemeOptions" />
              </div>

              <div v-if="activeTool.artifactType === 'table'" class="studio-field studio-field--full">
                <label>表头字段</label>
                <t-input v-model="form.tableColumns" :disabled="disabled" placeholder="序号,模块,任务,负责人,状态,备注" />
              </div>

              <div v-if="activeTool.artifactType === 'table'" class="studio-field">
                <label>预估行数</label>
                <t-input-number v-model="form.tableRows" :min="3" :max="50" :disabled="disabled" theme="column" />
              </div>

              <div class="studio-field studio-field--full">
                <label>补充信息</label>
                <t-textarea
                  v-model="form.notes"
                  :disabled="disabled"
                  :autosize="{ minRows: 4, maxRows: 8 }"
                  placeholder="补充风格、重点、限制、必须包含的章节等"
                />
              </div>
            </div>
          </section>

          <section v-if="activeTool.artifactType" class="studio-section">
            <div class="studio-section-title">参考模板</div>
            <div
              class="studio-dropzone"
              :class="{ 'is-dragover': isDragOver }"
              @dragenter.stop.prevent="handleDragEnter"
              @dragover.stop.prevent="handleDragOver"
              @dragleave.stop.prevent="handleDragLeave"
              @drop.stop.prevent="handleDrop"
              @click="openFilePicker"
            >
              <input ref="referenceInputRef" type="file" class="studio-file-input" multiple @change="handleFileChange" />
              <div class="studio-dropzone__icon">⇪</div>
              <div class="studio-dropzone__text">
                <strong>点击上传或拖拽到此处上传参考模板</strong>
                <span>支持 .pptx / .docx / .xlsx / .md / .txt / .html 等，先上传名字和摘要，后续可扩展真模板解析。</span>
              </div>
            </div>

            <div v-if="referenceTemplates.length > 0" class="studio-reference-list">
              <article v-for="item in referenceTemplates" :key="item.key" class="studio-reference-item">
                <div class="studio-reference-item__main">
                  <div class="studio-reference-item__title">{{ item.name }}</div>
                  <div class="studio-reference-item__meta">
                    {{ formatBytes(item.size) }} · {{ item.kind || 'file' }}
                  </div>
                  <div v-if="item.summary" class="studio-reference-item__summary">{{ item.summary }}</div>
                </div>
                <button type="button" class="studio-reference-item__remove" @click="removeReference(item.key)">移除</button>
              </article>
            </div>

            <div class="studio-upload-actions">
              <t-button size="small" variant="outline" :disabled="referenceTemplates.length === 0" @click="clearReferences">清空模板</t-button>
            </div>
          </section>

          <section class="studio-section">
            <div class="studio-section-title">生成说明</div>
            <div class="studio-helper-box">
              {{ generationPreview }}
            </div>
            <div class="studio-upload-actions">
              <t-button theme="primary" :loading="generating" :disabled="disabled" @click="generateArtifact">
                {{ generating ? '生成中' : '开始生成' }}
              </t-button>
              <t-button variant="outline" :disabled="disabled" @click="sendPromptToChat">把配置发到对话</t-button>
            </div>
          </section>
        </template>

        <template v-else>
          <header class="studio-header">
            <div>
              <p class="studio-eyebrow">Office Copilot</p>
              <h2>办公产出工作台</h2>
            </div>
            <span class="studio-badge">Beta</span>
          </header>

          <section class="studio-section">
            <div class="studio-section-title">常用生成</div>
            <div class="studio-card-grid studio-card-grid--primary">
              <button
                v-for="item in primaryItems"
                :key="item.key"
                type="button"
                class="studio-card"
                :disabled="disabled"
                @click="openTool(item)"
              >
                <span class="studio-card__icon">{{ item.icon }}</span>
                <span class="studio-card__body">
                  <span class="studio-card__title">{{ item.title }}</span>
                  <span class="studio-card__desc">{{ item.description }}</span>
                </span>
              </button>
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
                @click="openTool(item)"
              >
                <span class="studio-card__icon">{{ item.icon }}</span>
                <span class="studio-card__body">
                  <span class="studio-card__title">{{ item.title }}</span>
                  <span class="studio-card__desc">{{ item.description }}</span>
                </span>
              </button>
            </div>
          </section>
        </template>

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
            还没有生成记录。先打开一个工具，填点信息再点“开始生成”。
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
          现在是“工具内部界面”模式了：先配置，再生成，再记录。
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
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
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

interface ReferenceTemplate {
  key: string
  name: string
  size: number
  kind: string
  summary: string
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
const activeToolKey = ref<StudioItem['key'] | null>(null)
const artifacts = ref<StudioArtifact[]>([])
const selectedIds = ref<Set<string>>(new Set())
const recordsLoading = ref(false)
const deleting = ref(false)
const generating = ref(false)
const regeneratingId = ref('')
const previewVisible = ref(false)
const previewArtifact = ref<StudioArtifact | null>(null)
const previewContent = ref('')
const isDragOver = ref(false)
const referenceInputRef = ref<HTMLInputElement | null>(null)
const referenceTemplates = ref<ReferenceTemplate[]>([])

const form = reactive({
  title: '',
  audience: '部门同事',
  style: '正式',
  pageCount: 10,
  docType: '周报',
  htmlTheme: '简洁科技',
  tableColumns: '序号,模块,任务,负责人,状态,备注',
  tableRows: 8,
  notes: '',
})

const audienceOptions = [
  { label: '部门同事', value: '部门同事' },
  { label: '直属领导', value: '直属领导' },
  { label: '管理层', value: '管理层' },
  { label: '项目评审', value: '项目评审' },
  { label: '面试展示', value: '面试展示' },
]

const styleOptions = [
  { label: '正式', value: '正式' },
  { label: '简洁', value: '简洁' },
  { label: '科技感', value: '科技感' },
  { label: '咨询风', value: '咨询风' },
  { label: '复盘风', value: '复盘风' },
]

const docTypeOptions = [
  { label: '周报', value: '周报' },
  { label: '会议纪要', value: '会议纪要' },
  { label: 'SOP', value: 'SOP' },
  { label: '项目复盘', value: '项目复盘' },
  { label: '邮件', value: '邮件' },
]

const htmlThemeOptions = [
  { label: '简洁科技', value: '简洁科技' },
  { label: '仪表盘', value: '仪表盘' },
  { label: '流程看板', value: '流程看板' },
  { label: '汇报风', value: '汇报风' },
]

const primaryItems: StudioItem[] = [
  {
    key: 'ppt',
    artifactType: 'ppt',
    icon: '📊',
    title: 'PPT 制作',
    description: '沉淀汇报大纲、项目复盘、方案评审结构',
    prompt: '请生成一份企业汇报 PPT 方案，包含封面、背景、目标、方案、风险和下一步。',
  },
  {
    key: 'html',
    artifactType: 'html',
    icon: '🧩',
    title: 'HTML 页面',
    description: '生成可预览的单页看板、流程图或说明页',
    prompt: '请生成一个可直接打开的 HTML 单文件，适合企业内部展示。',
  },
  {
    key: 'table',
    artifactType: 'table',
    icon: '📋',
    title: '表格生成',
    description: '生成任务跟进表、风险清单、需求拆解 CSV',
    prompt: '请把当前任务整理成适合 Excel 的结构化表格。',
  },
  {
    key: 'doc',
    artifactType: 'doc',
    icon: '📝',
    title: '办公文档',
    description: '周报、会议纪要、邮件、SOP、复盘草稿',
    prompt: '请生成一份适合企业办公场景的正式文档草稿。',
  },
]

const secondaryItems: StudioItem[] = [
  {
    key: 'source-trace',
    icon: '🔎',
    title: '知识溯源',
    description: '整理引用片段、命中文档和可信度',
    prompt: '请整理回答中的关键结论、来源片段和冲突点。',
  },
  {
    key: 'parse-trace',
    icon: '⏱️',
    title: '解析 Trace',
    description: '复盘解析、后处理、索引各节点耗时',
    prompt: '请设计一个文档入库 Trace，包含解析、清洗、切块、Embedding、Rerank 等节点。',
  },
]

const activeTool = computed(() => [...primaryItems, ...secondaryItems].find((item) => item.key === activeToolKey.value) || null)
const activeArtifactType = computed(() => activeTool.value?.artifactType || '')
const recordFilterType = computed(() => activeArtifactType.value || '')
const recordSummary = computed(() => {
  if (artifacts.value.length === 0) return '暂无记录'
  return `共 ${artifacts.value.length} 条，已选择 ${selectedIds.value.size} 条`
})
const allVisibleSelected = computed(() => artifacts.value.length > 0 && artifacts.value.every((item) => selectedIds.value.has(item.id)))

const generationPreview = computed(() => {
  if (!activeTool.value) return ''
  if (!activeArtifactType.value) return '当前是辅助分析工具，主要用于把思路整理成可发送到对话的提示词。'
  const refText = referenceTemplates.value.length
    ? referenceTemplates.value.map((item) => `- ${item.name} (${item.kind})`).join('\n')
    : '- 无参考模板'
  return [
    `标题：${form.title || activeTool.value.title}`,
    `受众：${form.audience}`,
    `风格：${form.style}`,
    activeArtifactType.value === 'ppt' ? `页数：${form.pageCount}` : '',
    activeArtifactType.value === 'doc' ? `文档类型：${form.docType}` : '',
    activeArtifactType.value === 'html' ? `主题：${form.htmlTheme}` : '',
    activeArtifactType.value === 'table' ? `表头：${form.tableColumns}` : '',
    activeArtifactType.value === 'table' ? `预估行数：${form.tableRows}` : '',
    form.notes ? `补充：${form.notes}` : '补充：无',
    `参考模板：\n${refText}`,
  ].filter(Boolean).join('\n')
})

const toggleCollapsed = () => {
  collapsed.value = !collapsed.value
  localStorage.setItem(STORAGE_KEY, collapsed.value ? '1' : '0')
  emit('collapse-change', collapsed.value)
}

const resetFormForTool = (tool: StudioItem) => {
  form.title = tool.title
  form.audience = '部门同事'
  form.style = '正式'
  form.pageCount = 10
  form.docType = '周报'
  form.htmlTheme = '简洁科技'
  form.tableColumns = '序号,模块,任务,负责人,状态,备注'
  form.tableRows = 8
  form.notes = ''
  referenceTemplates.value = []
}

const openTool = (tool: StudioItem) => {
  activeToolKey.value = tool.key
  resetFormForTool(tool)
  void loadArtifacts()
}

const backToHome = () => {
  activeToolKey.value = null
  selectedIds.value = new Set()
  referenceTemplates.value = []
  previewVisible.value = false
}

const sendPromptToChat = () => {
  if (!activeTool.value) return
  emit('use-template', buildPrompt())
}

const buildPrompt = () => {
  if (!activeTool.value) return ''
  const refs = referenceTemplates.value.length
    ? referenceTemplates.value.map((item) => `- ${item.name}: ${item.summary || 'no summary'}`).join('\n')
    : '- none'
  const common = [
    `任务类型：${activeTool.value.title}`,
    `目标受众：${form.audience}`,
    `内容风格：${form.style}`,
    form.notes ? `补充要求：${form.notes}` : '',
    `参考模板：\n${refs}`,
  ].filter(Boolean)

  if (!activeArtifactType.value) {
    return [activeTool.value.prompt, ...common].join('\n')
  }

  if (activeArtifactType.value === 'ppt') {
    return [
      activeTool.value.prompt,
      ...common,
      `请输出 ${form.pageCount} 页结构，包含封面、背景、目标、方案、风险、总结和 Q&A。`,
    ].join('\n')
  }
  if (activeArtifactType.value === 'html') {
    return [
      activeTool.value.prompt,
      ...common,
      `页面主题：${form.htmlTheme}`,
      '请输出完整可运行的单文件 HTML，并说明如何预览。',
    ].join('\n')
  }
  if (activeArtifactType.value === 'table') {
    return [
      activeTool.value.prompt,
      ...common,
      `表头字段：${form.tableColumns}`,
      `预估行数：${form.tableRows}`,
      '请输出 CSV 格式内容。',
    ].join('\n')
  }
  return [
    activeTool.value.prompt,
    ...common,
    `文档类型：${form.docType}`,
  ].join('\n')
}

const loadArtifacts = async () => {
  recordsLoading.value = true
  try {
    const params = recordFilterType.value ? { type: recordFilterType.value, limit: 30 } : { limit: 30 }
    const res = await listStudioArtifacts(params)
    artifacts.value = res.data?.items || []
    const visibleIDs = new Set(artifacts.value.map((item) => item.id))
    selectedIds.value = new Set([...selectedIds.value].filter((id) => visibleIDs.has(id)))
  } catch (error: any) {
    MessagePlugin.error(error?.message || '加载 Studio 记录失败')
  } finally {
    recordsLoading.value = false
  }
}

const generateArtifact = async () => {
  if (!activeTool.value || !activeArtifactType.value) return
  generating.value = true
  try {
    const prompt = buildPrompt()
    await createStudioArtifact({
      type: activeArtifactType.value,
      title: form.title || activeTool.value.title,
      prompt,
      session_id: props.sessionId,
      source: 'studio',
    })
    MessagePlugin.success(`${activeTool.value.title} 已生成`)
    await loadArtifacts()
  } catch (error: any) {
    MessagePlugin.error(error?.message || '生成失败')
  } finally {
    generating.value = false
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

const openFilePicker = () => {
  referenceInputRef.value?.click()
}

const summarizeFile = async (file: File) => {
  const name = file.name
  const kind = file.type || name.split('.').pop() || 'file'
  const isTextLike = /^(text\/|application\/json|application\/xml|application\/csv|application\/javascript)/i.test(file.type)
    || /\.(md|txt|csv|json|xml|html?|css|js|ts|tsx|vue)$/i.test(name)
  if (!isTextLike) {
    return {
      key: `${name}-${file.lastModified}`,
      name,
      size: file.size,
      kind,
      summary: '',
    }
  }
  try {
    const text = await file.text()
    const summary = text.replace(/\s+/g, ' ').trim().slice(0, 220)
    return {
      key: `${name}-${file.lastModified}`,
      name,
      size: file.size,
      kind,
      summary,
    }
  } catch {
    return {
      key: `${name}-${file.lastModified}`,
      name,
      size: file.size,
      kind,
      summary: '',
    }
  }
}

const addFiles = async (files: FileList | File[]) => {
  const list = Array.from(files)
  if (list.length === 0) return
  const items = await Promise.all(list.map((file) => summarizeFile(file)))
  const seen = new Set(referenceTemplates.value.map((item) => item.key))
  for (const item of items) {
    if (!seen.has(item.key)) {
      referenceTemplates.value.push(item)
    }
  }
}

const handleFileChange = async (event: Event) => {
  const input = event.target as HTMLInputElement
  if (input.files) {
    await addFiles(input.files)
    input.value = ''
  }
  isDragOver.value = false
}

const handleDragEnter = (event: DragEvent) => {
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'copy'
  }
  isDragOver.value = true
}

const handleDragOver = (event: DragEvent) => {
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'copy'
  }
  isDragOver.value = true
}

const handleDragLeave = (event: DragEvent) => {
  const current = event.currentTarget as HTMLElement | null
  const related = event.relatedTarget as Node | null
  if (current && related && current.contains(related)) return
  isDragOver.value = false
}

const handleDrop = async (event: DragEvent) => {
  isDragOver.value = false
  if (event.dataTransfer?.files) {
    await addFiles(event.dataTransfer.files)
  }
}

const removeReference = (key: string) => {
  referenceTemplates.value = referenceTemplates.value.filter((item) => item.key !== key)
}

const clearReferences = () => {
  referenceTemplates.value = []
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

watch(activeArtifactType, () => {
  if (activeTool.value?.artifactType) {
    void loadArtifacts()
  }
})

onMounted(() => {
  collapsed.value = localStorage.getItem(STORAGE_KEY) === '1'
  emit('collapse-change', collapsed.value)
  void loadArtifacts()
})

onBeforeUnmount(() => {
  selectedIds.value = new Set()
})
</script>

<style lang="less" scoped>
.studio-sidebar {
  position: fixed;
  top: 76px;
  right: 16px;
  bottom: 132px;
  z-index: 30;
  width: 360px;
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
.studio-workspace-header,
.studio-section-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.studio-header h2,
.studio-workspace-header h2 {
  margin: 4px 0 0;
  font-size: 18px;
  line-height: 1.35;
  color: var(--td-text-color-primary);
}

.studio-workspace-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
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

.studio-helper-box {
  padding: 12px;
  border-radius: 14px;
  background: var(--td-bg-color-page);
  color: var(--td-text-color-secondary);
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
}

.studio-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.studio-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.studio-field--full {
  grid-column: 1 / -1;
}

.studio-field label {
  font-size: 12px;
  color: var(--td-text-color-secondary);
  font-weight: 600;
}

.studio-dropzone {
  display: grid;
  grid-template-columns: 42px 1fr;
  gap: 12px;
  align-items: center;
  padding: 14px;
  border: 1px dashed var(--td-component-stroke);
  border-radius: 16px;
  background: var(--td-bg-color-page);
  cursor: pointer;
  transition: border-color 0.2s ease, background 0.2s ease;

  &.is-dragover {
    border-color: var(--td-brand-color);
    background: var(--td-brand-color-light);
  }
}

.studio-dropzone__icon {
  width: 42px;
  height: 42px;
  border-radius: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--td-bg-color-container);
  color: var(--td-brand-color);
  font-size: 20px;
  font-weight: 700;
}

.studio-dropzone__text {
  display: flex;
  flex-direction: column;
  gap: 4px;

  strong {
    font-size: 13px;
  }

  span {
    font-size: 12px;
    color: var(--td-text-color-secondary);
    line-height: 1.5;
  }
}

.studio-file-input {
  display: none;
}

.studio-reference-list {
  display: grid;
  gap: 10px;
  margin-top: 12px;
}

.studio-reference-item {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  border-radius: 14px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
}

.studio-reference-item__main {
  min-width: 0;
}

.studio-reference-item__title {
  font-size: 13px;
  font-weight: 700;
}

.studio-reference-item__meta,
.studio-reference-item__summary {
  margin-top: 4px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
  line-height: 1.5;
}

.studio-reference-item__summary {
  white-space: pre-wrap;
}

.studio-reference-item__remove {
  border: 0;
  background: transparent;
  color: var(--td-error-color);
  cursor: pointer;
}

.studio-upload-actions {
  margin-top: 12px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
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
  cursor: pointer;
  text-align: left;
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 12px;
}

.studio-card--flat {
  background: var(--td-bg-color-container);
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
    width: min(360px, calc(100vw - 24px));
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

  .studio-form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
