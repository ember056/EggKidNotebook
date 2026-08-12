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
      <span v-if="!collapsed" class="studio-toggle__text">Studio</span>
      <t-icon :name="collapsed ? 'chevron-left' : 'chevron-right'" size="16px" />
    </button>

    <transition name="studio-content-fade">
      <div v-if="!collapsed" class="studio-panel">
        <header class="studio-header">
          <div>
            <p class="studio-eyebrow">EggKid Studio</p>
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
              class="studio-card studio-card--primary"
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

        <section class="studio-section">
          <div class="studio-section-title">辅助分析</div>
          <div class="studio-card-grid studio-card-grid--secondary">
            <button
              v-for="item in secondaryItems"
              :key="item.key"
              type="button"
              class="studio-card"
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

        <div class="studio-footer">
          <span class="studio-footer__dot"></span>
          点击卡片会把任务模板发送到当前对话
        </div>
      </div>
    </transition>
  </aside>
</template>

<script setup>
import { onMounted, ref } from 'vue';

const emit = defineEmits(['use-template', 'collapse-change']);

defineProps({
  disabled: { type: Boolean, default: false },
});

const STORAGE_KEY = 'eggkid-studio-sidebar-collapsed';
const collapsed = ref(false);

const primaryItems = [
  {
    key: 'ppt',
    icon: '📊',
    title: 'PPT 制作',
    description: '把对话/知识库内容整理成汇报大纲',
    prompt: `请作为企业办公汇报助手，基于当前对话和已选知识库内容，帮我生成一份 PPT 方案。

要求：
1. 先给出标题、受众、汇报目标；
2. 输出 8-12 页页面结构；
3. 每页包含：页标题、核心观点、三条以内要点、建议图表/配图；
4. 最后给出一页“讲稿提示”和一页“可能被问到的问题”。`,
  },
  {
    key: 'html',
    icon: '🧩',
    title: 'HTML 预览',
    description: '生成可直接预览的网页/图表/流程图',
    prompt: `请作为前端 Artifact 助手，基于我的需求生成一个完整、可运行的 HTML 单文件。

要求：
1. 只使用 HTML/CSS/JavaScript，不依赖构建工具；
2. 页面适合企业内部汇报或知识展示；
3. 视觉简洁，包含响应式布局；
4. 如果适合，请加入流程图、表格或指标卡片；
5. 最后说明如何把代码保存为 .html 并打开预览。`,
  },
  {
    key: 'office-doc',
    icon: '📝',
    title: '办公文档',
    description: '周报、会议纪要、邮件、SOP 一键成稿',
    prompt: `请作为企业办公文档助手，先判断我当前任务更适合输出日报、周报、会议纪要、邮件、SOP 还是项目复盘。

然后按所选文档类型输出：
1. 标题；
2. 背景/目标；
3. 关键事项；
4. 风险与阻塞；
5. 下一步计划；
6. 可直接复制使用的正式版本。

如果信息不足，请先列出你需要我补充的 3 个关键问题。`,
  },
];

const secondaryItems = [
  {
    key: 'source-trace',
    icon: '🔎',
    title: '知识溯源',
    description: '整理引用片段、命中文档和可信度',
    prompt: `请作为 RAG 知识溯源助手，基于当前回答或检索结果，帮我整理一份“知识溯源说明”。

请输出：
1. 回答中最关键的 5 个结论；
2. 每个结论对应的来源文档/片段；
3. 哪些结论证据充分，哪些仍需要人工确认；
4. 如果有冲突信息，请列出冲突点和建议处理方式。`,
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

每个节点请包含：状态、预估耗时、可能失败原因、排查建议。`,
  },
];

const toggleCollapsed = () => {
  collapsed.value = !collapsed.value;
  localStorage.setItem(STORAGE_KEY, collapsed.value ? '1' : '0');
  emit('collapse-change', collapsed.value);
};

const useTemplate = (item) => {
  emit('use-template', item.prompt);
};

onMounted(() => {
  collapsed.value = localStorage.getItem(STORAGE_KEY) === '1';
  emit('collapse-change', collapsed.value);
});
</script>

<style lang="less" scoped>
.studio-sidebar {
  position: fixed;
  top: 76px;
  right: 16px;
  bottom: 132px;
  z-index: 30;
  width: 320px;
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

.studio-header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
  margin-bottom: 18px;

  h2 {
    margin: 4px 0 0;
    font-size: 18px;
    line-height: 1.35;
    color: var(--td-text-color-primary);
  }
}

.studio-eyebrow {
  margin: 0;
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
  margin-bottom: 10px;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
  font-weight: 700;
}

.studio-card-grid {
  display: grid;
  gap: 10px;
}

.studio-card-grid--primary {
  grid-template-columns: 1fr;
}

.studio-card {
  width: 100%;
  min-height: 76px;
  padding: 12px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 16px;
  background: color-mix(in srgb, var(--td-bg-color-container) 86%, var(--td-brand-color-light) 14%);
  text-align: left;
  cursor: pointer;
  display: flex;
  gap: 12px;
  align-items: flex-start;
  color: var(--td-text-color-primary);
  transition: transform 0.18s ease, border-color 0.18s ease, background 0.18s ease;

  &:hover:not(:disabled) {
    transform: translateY(-2px);
    border-color: var(--td-brand-color);
    background: var(--td-bg-color-container-hover);
  }

  &:disabled {
    cursor: not-allowed;
    opacity: 0.55;
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
    width: min(320px, calc(100vw - 24px));
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
    width: min(300px, calc(100vw - 20px));
  }
}
</style>
