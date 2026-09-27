<!--
  文件分页预览器（PDF 走 pdfjs 按需渲染）。

  - PDF：pdfjs-dist 逐页渲染——打开文档后只解析目录结构，翻到哪页才渲染/拉取哪页；
    真实 MinIO 地址配合 HTTP Range（disableAutoFetch）可做到网络层面按需加载，
    mock 的 blob 地址会整体读入内存（本地即时，无网络开销）；
  - 图片：单页展示，支持点击放大；
  - 其他格式：降级为“暂不支持在线预览”的提示（后端上传时已将 doc/ppt 转为 PDF，该分支主要兜底 txt/md）。
-->
<script setup lang="ts">
import type { PDFDocumentProxy, RenderTask } from 'pdfjs-dist';

import {
  ArrowLeftOutlined,
  ArrowRightOutlined,
  DownloadOutlined,
  FileTextOutlined,
} from '@ant-design/icons-vue';
import * as pdfjsLib from 'pdfjs-dist';
import workerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';

import { previewKindOf } from '#/api/kadmin/fileAudit';

pdfjsLib.GlobalWorkerOptions.workerSrc = workerUrl;

const props = withDefaults(
  defineProps<{
    fileFormat: string;
    fileName: string;
    fileUrl?: string;
    /** 后端可选提供的总页数；PDF 场景以 pdfjs 解析结果为准 */
    pageCount?: number;
    /** 预览区高度 */
    height?: number;
  }>(),
  {
    fileUrl: '',
    pageCount: undefined,
    height: 460,
  },
);

const page = defineModel<number>('page', { default: 1 });

const kind = computed(() => previewKindOf(props.fileFormat));

const stageRef = ref<HTMLDivElement>();
const canvasRef = ref<HTMLCanvasElement>();
const docLoading = ref(false);
const rendering = ref(false);
const loadError = ref('');
/** pdfjs 打开文档后解析出的实际页数 */
const pdfPages = ref<number>();

let pdfDoc: undefined | PDFDocumentProxy;
let loadingTask: undefined | ReturnType<typeof pdfjsLib.getDocument>;
let renderTask: undefined | RenderTask;
let resizeTimer: undefined | ReturnType<typeof setTimeout>;

const effectiveTotal = computed(() =>
  kind.value === 'pdf' ? pdfPages.value : props.pageCount,
);

const canPrev = computed(() => page.value > 1);
const canNext = computed(
  () => effectiveTotal.value === undefined || page.value < effectiveTotal.value,
);

watch(
  () => props.fileUrl,
  () => {
    page.value = 1;
  },
);

watch(
  [() => props.fileUrl, kind] as const,
  ([url, kindValue]) => {
    if (kindValue === 'pdf') {
      void loadDocument(url);
    } else {
      destroyDocument();
    }
  },
  { immediate: true },
);

watch(page, () => {
  if (kind.value === 'pdf') {
    if (stageRef.value) stageRef.value.scrollTop = 0;
    void renderCurrentPage();
  }
});

async function loadDocument(url: string) {
  destroyDocument();
  if (!url) return;
  docLoading.value = true;
  loadError.value = '';
  try {
    loadingTask = pdfjsLib.getDocument({
      url,
      // 按需拉取：服务端支持 Range 时只取当前页所需的字节区间
      disableAutoFetch: true,
      rangeChunkSize: 128 * 1024,
    });
    pdfDoc = await loadingTask.promise;
    pdfPages.value = pdfDoc.numPages;
    await renderCurrentPage();
  } catch (error) {
    // 切换文件/组件卸载触发的中断不算错误
    if (!loadingTask) return;
    loadError.value = error instanceof Error ? error.message : '加载 PDF 失败';
  } finally {
    docLoading.value = false;
  }
}

async function renderCurrentPage() {
  const doc = pdfDoc;
  const canvas = canvasRef.value;
  const stage = stageRef.value;
  if (!doc || !canvas || !stage) return;
  const total = pdfPages.value ?? 1;
  const pageNo = Math.min(Math.max(1, Math.trunc(page.value)), total);

  renderTask?.cancel();
  rendering.value = true;
  try {
    const pdfPage = await doc.getPage(pageNo);
    if (pdfDoc !== doc) return;
    const availableWidth = Math.max(stage.clientWidth - 24, 240);
    const baseViewport = pdfPage.getViewport({ scale: 1 });
    const scale = availableWidth / baseViewport.width;
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    const viewport = pdfPage.getViewport({ scale: scale * dpr });
    canvas.width = Math.floor(viewport.width);
    canvas.height = Math.floor(viewport.height);
    canvas.style.width = `${Math.floor(viewport.width / dpr)}px`;
    canvas.style.height = `${Math.floor(viewport.height / dpr)}px`;
    const task = pdfPage.render({ canvas, viewport });
    renderTask = task;
    await task.promise;
  } catch (error) {
    const name = (error as { name?: string }).name;
    if (name === 'RenderingCancelledException' || name === 'AbortException') {
      return;
    }
    console.error('PDF page render failed:', error);
  } finally {
    if (pdfDoc === doc) {
      rendering.value = false;
    }
  }
}

function destroyDocument() {
  renderTask?.cancel();
  renderTask = undefined;
  const task = loadingTask;
  loadingTask = undefined;
  pdfDoc = undefined;
  pdfPages.value = undefined;
  loadError.value = '';
  // destroy() 会让挂起的 getDocument promise 以 AbortException 拒绝
  void task?.destroy();
}

function jump(delta: number) {
  if (delta < 0 && !canPrev.value) return;
  if (delta > 0 && !canNext.value) return;
  page.value += delta;
}

function handlePageInput(value: number | string | null) {
  const target = Number(value);
  if (!Number.isFinite(target) || target < 1) return;
  const max = effectiveTotal.value ?? Number.MAX_SAFE_INTEGER;
  page.value = Math.min(Math.trunc(target), max);
}

onMounted(() => {
  const stage = stageRef.value;
  if (!stage || typeof ResizeObserver === 'undefined') return;
  const observer = new ResizeObserver(() => {
    if (resizeTimer) clearTimeout(resizeTimer);
    resizeTimer = setTimeout(() => {
      if (pdfDoc) void renderCurrentPage();
    }, 150);
  });
  observer.observe(stage);
  onBeforeUnmount(() => observer.disconnect());
});

onBeforeUnmount(() => {
  if (resizeTimer) clearTimeout(resizeTimer);
  destroyDocument();
});
</script>

<template>
  <div class="file-preview">
    <div
      v-if="kind === 'pdf'"
      ref="stageRef"
      class="file-preview-stage file-preview-stage-pdf"
      :style="{ height: `${height}px` }"
    >
      <a-spin :spinning="docLoading || rendering" wrapper-class-name="pdf-spin-wrap">
        <canvas v-show="!loadError" ref="canvasRef" class="file-preview-canvas" />
      </a-spin>
      <div v-if="loadError" class="file-preview-empty">
        <FileTextOutlined class="file-preview-empty-icon" />
        <p>{{ loadError }}</p>
      </div>
    </div>

    <div
      v-else-if="kind === 'image' && fileUrl"
      class="file-preview-stage"
      :style="{ height: `${height}px` }"
    >
      <div class="file-preview-image">
        <a-image
          :src="fileUrl"
          :root-style="{ maxHeight: '100%' }"
          :width="`min(100%, 620px)`"
        />
      </div>
    </div>

    <div v-else class="file-preview-stage" :style="{ height: `${height}px` }">
      <div class="file-preview-empty">
        <FileTextOutlined class="file-preview-empty-icon" />
        <p>该格式暂不支持在线预览</p>
        <a-space>
          <a-button v-if="fileUrl" :href="fileUrl" download type="primary" target="_blank">
            <DownloadOutlined />
            下载查看
          </a-button>
        </a-space>
      </div>
    </div>

    <div v-if="kind !== 'unsupported'" class="file-preview-bar">
      <a-space :size="8">
        <a-button size="small" :disabled="!canPrev" @click="jump(-1)">
          <ArrowLeftOutlined />
        </a-button>
        <span class="file-preview-page">
          第
          <a-input-number
            class="file-preview-page-input"
            size="small"
            :min="1"
            :max="effectiveTotal ?? undefined"
            :value="page"
            :controls="false"
            @change="handlePageInput"
          />
          页
        </span>
        <span class="file-preview-total">
          {{ effectiveTotal ? `/ 共 ${effectiveTotal} 页` : '' }}
        </span>
        <a-button size="small" :disabled="!canNext" @click="jump(1)">
          <ArrowRightOutlined />
        </a-button>
      </a-space>
      <span class="file-preview-name" :title="fileName">{{ fileName }}</span>
    </div>
  </div>
</template>

<style scoped>
.file-preview {
  display: grid;
  gap: 10px;
  min-width: 0;
}

.file-preview-stage {
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid var(--kadmin-border, hsl(var(--border)));
  border-radius: 8px;
  background: hsl(var(--muted) / 40%);
}

.file-preview-stage-pdf {
  overflow: auto;
  padding: 12px;
}

.file-preview-stage-pdf :deep(.pdf-spin-wrap) {
  width: 100%;
  display: flex;
  justify-content: center;
}

.file-preview-canvas {
  display: block;
  max-width: 100%;
}

.file-preview-image {
  display: flex;
  max-height: 100%;
  padding: 12px;
}

.file-preview-image :deep(.ant-image) {
  max-height: 100%;
}

.file-preview-image :deep(img) {
  max-height: calc(100% - 24px);
  object-fit: contain;
}

.file-preview-empty {
  display: grid;
  gap: 10px;
  justify-items: center;
  align-content: center;
  color: var(--kadmin-muted, hsl(var(--muted-foreground)));
}

.file-preview-empty-icon {
  font-size: 42px;
  opacity: 0.5;
}

.file-preview-empty p {
  margin: 0;
}

.file-preview-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.file-preview-page {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--kadmin-text, hsl(var(--foreground)));
}

.file-preview-page-input {
  width: 52px;
  padding: 0;
  text-align: center;
}

.file-preview-total {
  color: var(--kadmin-muted, hsl(var(--muted-foreground)));
}

.file-preview-name {
  min-width: 0;
  overflow: hidden;
  color: var(--kadmin-muted, hsl(var(--muted-foreground)));
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
