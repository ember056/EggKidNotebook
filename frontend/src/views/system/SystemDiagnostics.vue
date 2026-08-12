<template>
  <div class="system-diagnostics">
    <header class="section-header sd-header">
      <div class="sd-title-block">
        <h2>系统状态</h2>
        <p class="section-description">
          一眼检查应用、数据库、Redis、解析引擎、存储和后台队列是否处于可用状态。
        </p>
      </div>
      <div class="sd-actions">
        <label class="sd-auto-refresh">
          <span class="sd-live-dot" :class="{ 'sd-live-dot--active': autoRefresh }" />
          <span>自动刷新</span>
          <t-switch
            v-model="autoRefresh"
            size="small"
            aria-label="自动刷新系统状态"
          />
        </label>
        <button
          type="button"
          class="sd-refresh"
          :disabled="loading"
          title="刷新"
          aria-label="刷新系统状态"
          @click="reload"
        >
          <t-icon
            :name="loading ? 'loading' : 'refresh'"
            :class="{ 'sd-refresh-spin': loading }"
          />
        </button>
      </div>
    </header>

    <div v-if="loading && !loadedOnce" class="sd-loading" aria-live="polite">
      <t-skeleton
        animation="gradient"
        :row-col="[
          { width: '36%', height: '32px' },
          { width: '72%', height: '16px' },
          { width: '100%', height: '110px' },
          { width: '100%', height: '110px' },
        ]"
      />
    </div>

    <div v-else-if="error" class="sd-state sd-state--error" role="alert">
      <div class="sd-state-icon"><t-icon name="error-circle" size="24px" /></div>
      <div class="sd-state-copy">
        <strong>系统状态读取失败</strong>
        <span>{{ error }}</span>
      </div>
      <t-button size="small" variant="outline" @click="reload">重试</t-button>
    </div>

    <template v-else-if="diagnostics">
      <section class="sd-overview" :class="`sd-overview--${diagnostics.status}`">
        <div class="sd-overview-main">
          <div class="sd-status-orb">
            <t-icon :name="statusIcon(diagnostics.status)" />
          </div>
          <div>
            <div class="sd-kicker">当前总体状态</div>
            <h3>{{ statusLabel(diagnostics.status) }}</h3>
            <p>{{ overviewMessage }}</p>
          </div>
        </div>
        <div class="sd-overview-meta">
          <div>
            <span>检查时间</span>
            <strong>{{ formatDateTime(diagnostics.generated_at) }}</strong>
          </div>
          <div>
            <span>运行时长</span>
            <strong>{{ formatDuration(diagnostics.uptime_seconds || 0) }}</strong>
          </div>
          <div>
            <span>异常项</span>
            <strong>{{ issueCount }}</strong>
          </div>
        </div>
      </section>

      <section class="sd-guide">
        <div>
          <h3>运维提示</h3>
          <p>
            这里适合作为每天跟进项目时的第一站：先确认基础服务健康，再进入任务队列查看解析积压，
            最后定位具体失败文件或模型连接。
          </p>
        </div>
        <t-button variant="outline" size="small" @click="openRuntimeQueues">
          <template #icon><t-icon name="queue" /></template>
          查看任务队列
        </t-button>
      </section>

      <section class="sd-checks">
        <article
          v-for="check in diagnostics.checks"
          :key="check.key"
          class="sd-check-card"
          :class="`sd-check-card--${check.status}`"
        >
          <div class="sd-check-topline">
            <div>
              <h3>{{ check.name }}</h3>
              <p>{{ check.message || '暂无说明' }}</p>
            </div>
            <t-tag :theme="statusTheme(check.status)" variant="light">
              {{ statusLabel(check.status) }}
            </t-tag>
          </div>

          <div class="sd-check-meta">
            <span v-if="check.latency_ms != null">
              <t-icon name="time" />
              {{ check.latency_ms }}ms
            </span>
            <span>
              <t-icon name="calendar" />
              {{ formatDateTime(check.checked_at) }}
            </span>
          </div>

          <div v-if="metadataEntries(check).length > 0" class="sd-metadata">
            <div
              v-for="item in metadataEntries(check)"
              :key="item.key"
              class="sd-metadata-item"
            >
              <span>{{ item.key }}</span>
              <strong>{{ item.value }}</strong>
            </div>
          </div>

          <div v-if="check.remediation" class="sd-remediation">
            <t-icon name="tips" />
            <span>{{ check.remediation }}</span>
          </div>
        </article>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  getSystemDiagnostics,
  type DiagnosticCheck,
  type DiagnosticStatus,
  type SystemDiagnosticsResponse,
} from '@/api/system'

const POLL_INTERVAL_MS = 8000

const router = useRouter()
const diagnostics = ref<SystemDiagnosticsResponse | null>(null)
const loading = ref(false)
const loadedOnce = ref(false)
const error = ref('')
const autoRefresh = ref(true)
let pollTimer: ReturnType<typeof setInterval> | null = null

const issueCount = computed(() => diagnostics.value?.checks.filter((check) => (
  check.status === 'warning' || check.status === 'error'
)).length || 0)

const overviewMessage = computed(() => {
  if (!diagnostics.value) return ''
  if (diagnostics.value.status === 'ok') return '核心服务检查通过，可以继续进行知识库解析、对话和模型测试。'
  if (diagnostics.value.status === 'warning') return '系统可用，但存在需要关注的风险项，建议优先处理黄色检查项。'
  if (diagnostics.value.status === 'error') return '存在阻断性异常，请先处理红色检查项再继续部署或大批量解析。'
  return '部分能力未启用，通常出现在 Lite 模式或未配置外部服务时。'
})

async function reload() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    diagnostics.value = await getSystemDiagnostics()
    loadedOnce.value = true
  } catch (err: any) {
    error.value = err?.message || '未知错误'
    if (loadedOnce.value) {
      MessagePlugin.error(error.value)
    }
  } finally {
    loading.value = false
  }
}

function statusLabel(status: DiagnosticStatus): string {
  const labels: Record<DiagnosticStatus, string> = {
    ok: '正常',
    warning: '需关注',
    error: '异常',
    disabled: '未启用',
  }
  return labels[status] || status
}

function statusTheme(status: DiagnosticStatus): string {
  if (status === 'ok') return 'success'
  if (status === 'warning') return 'warning'
  if (status === 'error') return 'danger'
  return 'default'
}

function statusIcon(status: DiagnosticStatus): string {
  if (status === 'ok') return 'check-circle'
  if (status === 'warning') return 'error-circle'
  if (status === 'error') return 'close-circle'
  return 'info-circle'
}

function formatDateTime(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
}

function formatDuration(seconds: number): string {
  if (!seconds || seconds < 0) return '-'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days}天 ${hours}小时`
  if (hours > 0) return `${hours}小时 ${minutes}分钟`
  return `${Math.max(1, minutes)}分钟`
}

function metadataEntries(check: DiagnosticCheck): Array<{ key: string; value: string }> {
  if (!check.metadata) return []
  return Object.entries(check.metadata)
    .filter(([, value]) => value !== undefined && value !== null && value !== '')
    .slice(0, 8)
    .map(([key, value]) => ({ key, value: formatMetadataValue(value) }))
}

function formatMetadataValue(value: unknown): string {
  if (Array.isArray(value)) {
    return value.length > 0 ? value.join(', ') : '-'
  }
  if (typeof value === 'object' && value !== null) {
    return JSON.stringify(value)
  }
  return String(value)
}

function startPolling() {
  stopPolling()
  if (!autoRefresh.value) return
  pollTimer = setInterval(reload, POLL_INTERVAL_MS)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function openRuntimeQueues() {
  router.replace({
    path: '/platform/settings',
    query: { section: 'runtime-queues' },
  })
}

watch(autoRefresh, startPolling)

onMounted(() => {
  reload()
  startPolling()
})

onUnmounted(stopPolling)
</script>

<style scoped>
.system-diagnostics {
  display: flex;
  flex-direction: column;
  gap: 20px;
  color: #1f2937;
}

.sd-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.sd-title-block h2 {
  margin: 0 0 8px;
}

.sd-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.sd-auto-refresh {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  border: 1px solid #e5e7eb;
  border-radius: 999px;
  color: #4b5563;
  font-size: 13px;
  background: #fff;
}

.sd-live-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #d1d5db;
}

.sd-live-dot--active {
  background: #10b981;
  box-shadow: 0 0 0 5px rgba(16, 185, 129, 0.12);
}

.sd-refresh {
  width: 36px;
  height: 36px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: #fff;
  color: #374151;
  cursor: pointer;
  transition: all 0.18s ease;
}

.sd-refresh:hover:not(:disabled) {
  border-color: #0052d9;
  color: #0052d9;
  box-shadow: 0 8px 18px rgba(0, 82, 217, 0.12);
}

.sd-refresh:disabled {
  cursor: not-allowed;
  opacity: 0.7;
}

.sd-refresh-spin {
  animation: sd-spin 1s linear infinite;
}

.sd-loading,
.sd-state,
.sd-overview,
.sd-guide,
.sd-check-card {
  border: 1px solid #eef2f7;
  border-radius: 18px;
  background: #fff;
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.04);
}

.sd-loading {
  padding: 22px;
}

.sd-state {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 18px;
}

.sd-state--error {
  border-color: #fed7d7;
  background: #fff8f8;
}

.sd-state-icon {
  color: #e34d59;
}

.sd-state-copy {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 4px;
}

.sd-overview {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(280px, 0.7fr);
  gap: 18px;
  padding: 22px;
  background: linear-gradient(135deg, #f8fbff 0%, #ffffff 58%, #f8fafc 100%);
}

.sd-overview--warning {
  background: linear-gradient(135deg, #fffbeb 0%, #ffffff 62%, #f8fafc 100%);
}

.sd-overview--error {
  background: linear-gradient(135deg, #fff1f2 0%, #ffffff 62%, #f8fafc 100%);
}

.sd-overview-main {
  display: flex;
  gap: 16px;
  align-items: center;
}

.sd-status-orb {
  width: 54px;
  height: 54px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 18px;
  color: #0052d9;
  background: rgba(0, 82, 217, 0.1);
  font-size: 26px;
}

.sd-overview--warning .sd-status-orb {
  color: #b7791f;
  background: rgba(245, 158, 11, 0.14);
}

.sd-overview--error .sd-status-orb {
  color: #c53030;
  background: rgba(227, 77, 89, 0.12);
}

.sd-kicker {
  color: #64748b;
  font-size: 13px;
  font-weight: 600;
}

.sd-overview h3 {
  margin: 4px 0 8px;
  font-size: 28px;
  line-height: 1.1;
}

.sd-overview p,
.sd-guide p,
.sd-check-topline p {
  margin: 0;
  color: #64748b;
  line-height: 1.65;
}

.sd-overview-meta {
  display: grid;
  gap: 10px;
}

.sd-overview-meta > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.72);
  border: 1px solid rgba(226, 232, 240, 0.8);
}

.sd-overview-meta span {
  color: #64748b;
  font-size: 13px;
}

.sd-overview-meta strong {
  color: #0f172a;
  font-size: 14px;
}

.sd-guide {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 18px;
  padding: 18px;
}

.sd-guide h3 {
  margin: 0 0 6px;
  font-size: 16px;
}

.sd-checks {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.sd-check-card {
  padding: 18px;
  border-left: 4px solid #22c55e;
}

.sd-check-card--warning {
  border-left-color: #f59e0b;
}

.sd-check-card--error {
  border-left-color: #e34d59;
}

.sd-check-card--disabled {
  border-left-color: #94a3b8;
}

.sd-check-topline {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.sd-check-topline h3 {
  margin: 0 0 6px;
  font-size: 16px;
}

.sd-check-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 12px;
  color: #64748b;
  font-size: 12px;
}

.sd-check-meta span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.sd-metadata {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 14px;
}

.sd-metadata-item {
  min-width: 0;
  padding: 8px 10px;
  border-radius: 10px;
  background: #f8fafc;
  border: 1px solid #eef2f7;
}

.sd-metadata-item span,
.sd-metadata-item strong {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sd-metadata-item span {
  color: #94a3b8;
  font-size: 12px;
  margin-bottom: 3px;
}

.sd-metadata-item strong {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.sd-remediation {
  display: flex;
  gap: 8px;
  margin-top: 14px;
  padding: 10px 12px;
  border-radius: 12px;
  background: #fff7ed;
  color: #9a3412;
  font-size: 13px;
  line-height: 1.6;
}

@keyframes sd-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@media (max-width: 980px) {
  .sd-overview,
  .sd-checks {
    grid-template-columns: 1fr;
  }

  .sd-header,
  .sd-guide {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
