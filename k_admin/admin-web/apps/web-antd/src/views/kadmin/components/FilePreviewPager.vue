<!--
  文件分页预览器。

  - PDF：iframe 内嵌浏览器原生查看器，通过 #page= 片段实现翻页（外层提供统一的翻页控件）；
  - 图片：单页展示，支持点击放大；
  - 其他格式：降级为“暂不支持在线预览”的提示（后端上传时已将 doc/ppt 转为 PDF，该分支主要兜底 txt/md）。
-->
<script setup lang="ts">
import {
  ArrowLeftOutlined,
  ArrowRightOutlined,
  DownloadOutlined,
  FileTextOutlined,
} from '@ant-design/icons-vue';
import { computed, watch } from 'vue';

import { previewKindOf } from '#/api/kadmin/fileAudit';

const props = withDefaults(
  defineProps<{
    fileFormat: string;
    fileName: string;
    fileUrl?: string;
    /** 后端解析出的总页数；未提供时仅支持前后翻页 */
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

watch(
  () => props.fileUrl,
  () => {
    page.value = 1;
  },
);

const iframeSrc = computed(() =>
  kind.value === 'pdf' && props.fileUrl
    ? `${props.fileUrl}#page=${page.value}&toolbar=0&navpanes=0`
    : '',
);

const canPrev = computed(() => page.value > 1);
const canNext = computed(
  () => props.pageCount === undefined || page.value < props.pageCount,
);

function jump(delta: number) {
  if (delta < 0 && !canPrev.value) return;
  if (delta > 0 && !canNext.value) return;
  page.value += delta;
}

function handlePageInput(value: number | string | null) {
  const target = Number(value);
  if (!Number.isFinite(target) || target < 1) return;
  const max = props.pageCount ?? Number.MAX_SAFE_INTEGER;
  page.value = Math.min(Math.trunc(target), max);
}
</script>

<template>
  <div class="file-preview">
    <div class="file-preview-stage" :style="{ height: `${height}px` }">
      <iframe
        v-if="kind === 'pdf' && fileUrl"
        :key="`${fileUrl}#${page}`"
        :src="iframeSrc"
        :style="{ height: `${height}px` }"
        class="file-preview-frame"
        title="文件预览"
      />
      <div v-else-if="kind === 'image' && fileUrl" class="file-preview-image">
        <a-image
          :src="fileUrl"
          :root-style="{ maxHeight: '100%' }"
          :width="`min(100%, 620px)`"
        />
      </div>
      <div v-else class="file-preview-empty">
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
            :max="pageCount ?? undefined"
            :value="page"
            :controls="false"
            @change="handlePageInput"
          />
          页
        </span>
        <span class="file-preview-total">
          {{ pageCount ? `/ 共 ${pageCount} 页` : '' }}
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

.file-preview-frame {
  width: 100%;
  border: 0;
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
