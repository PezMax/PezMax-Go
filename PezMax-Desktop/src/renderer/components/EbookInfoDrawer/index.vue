<template>
  <el-drawer
    v-model="drawerVisible"
    :with-header="false"
    direction="rtl"
    size="380px"
    class="ide-file-info-drawer"
    modal-class="ide-file-info-overlay"
    destroy-on-close
  >
    <div class="drawer-container" v-if="fileInfo">
      <!-- 顶部：图标与标题 -->
      <div class="drawer-header animate-item item-1">
        <div class="file-icon-wrapper">
          <svg-icon icon-class="education" class="huge-icon" />
        </div>
        <div class="file-title-wrapper">
          <div class="title-with-status">
            <h2 class="file-name" :title="fileInfo.ebookName || '未知电子书'">
              {{ fileInfo.ebookName || '未知电子书' }}
            </h2>
            <span class="status-badge" :class="statusInfo.class" v-if="statusInfo.text">
              {{ statusInfo.text }}
            </span>
          </div>
          <div class="file-tags">
            <span class="file-ext-tag">{{ formatLabel }}</span>
            <span class="file-size-tag">{{ formatSize(fileInfo.ebookSize) }}</span>
          </div>
        </div>
      </div>

      <!-- 中部：元数据卡片网格（与试卷文件详情同构） -->
      <div class="drawer-body">
        <div class="info-section animate-item item-2">
          <h3 class="section-title">基本信息</h3>
          <div class="info-grid">
            <div class="info-card copyable" @click="copyText(fileInfo.ebookSubject || '未分类', '学科分类')">
              <span class="info-label">学科分类</span>
              <span class="info-value">{{ fileInfo.ebookSubject || '未分类' }}</span>
              <el-icon class="copy-icon"><CopyDocument /></el-icon>
            </div>

            <div class="info-card copyable" @click="copyText(formatEbookType(fileInfo.ebookType), '电子书类型')">
              <span class="info-label">电子书类型</span>
              <span class="info-value">{{ formatEbookType(fileInfo.ebookType) }}</span>
              <el-icon class="copy-icon"><CopyDocument /></el-icon>
            </div>

            <div class="info-card copyable" @click="copyText(fileInfo.author || '-', '作者')">
              <span class="info-label">作者</span>
              <span class="info-value">{{ fileInfo.author || '-' }}</span>
              <el-icon class="copy-icon"><CopyDocument /></el-icon>
            </div>

            <div class="info-card copyable" @click="copyText(fileInfo.publisher || '-', '出版社')">
              <span class="info-label">出版社</span>
              <span class="info-value">{{ fileInfo.publisher || '-' }}</span>
              <el-icon class="copy-icon"><CopyDocument /></el-icon>
            </div>
          </div>
        </div>

        <div class="info-section animate-item item-3">
          <h3 class="section-title">来源信息</h3>
          <div class="info-grid single-col">
            <div class="info-card copyable" @click="copyText(contributorName, '贡献者')">
              <el-avatar :size="32" :src="contributorAvatar" class="contributor-avatar">
                {{ uploaderInitial }}
              </el-avatar>
              <div class="user-meta">
                <span class="info-label">贡献者</span>
                <span class="info-value author">@{{ contributorName }}</span>
              </div>
              <el-icon class="copy-icon"><CopyDocument /></el-icon>
            </div>

            <div class="info-card">
              <span class="info-label">上架时间</span>
              <span class="info-value time-val">{{ fileInfo.createTime || '-' }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 底部：主要操作区（与试卷文件详情一致） -->
      <div class="drawer-footer animate-item item-4">
        <button class="action-btn primary-btn" @click="handleDownload">
          <el-icon><Download /></el-icon>
          <span>下载电子书</span>
        </button>

        <button
          class="action-btn favorite-btn"
          :class="{ active: isFavorite }"
          :disabled="favoriteLoading"
          @click="handleToggleFavorite"
        >
          <el-icon>
            <StarFilled v-if="isFavorite" />
            <Star v-else />
          </el-icon>
          <span>{{ favoriteLoading ? '处理中...' : (isFavorite ? '已收藏' : '收藏') }}</span>
        </button>

        <button class="action-btn secondary-btn" @click="copyText(fileInfo.ebookName || '未知电子书', '书名')">
          <el-icon><DocumentCopy /></el-icon>
          <span>复制书名</span>
        </button>

        <button class="action-btn danger-btn-full" @click="handleReport" title="举报该电子书">
          <el-icon><Warning /></el-icon>
          <span>举报该电子书</span>
        </button>
      </div>
    </div>
  </el-drawer>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { CopyDocument, Download, DocumentCopy, Warning, Star, StarFilled } from '@element-plus/icons-vue'
import { normalizeAvatar } from '@/utils/avatar'
import { getUser } from '@/api/datum/user'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  fileInfo: {
    type: Object,
    default: () => ({})
  },
  isFavorite: {
    type: Boolean,
    default: false
  },
  favoriteLoading: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['update:modelValue', 'download-file', 'report-file', 'toggle-favorite'])

const contributorInfo = ref(null)

const drawerVisible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

// 抽屉打开时拉取贡献者信息（头像/昵称），与文件详情抽屉同款
watch(() => props.modelValue, async (val) => {
  if (val) {
    contributorInfo.value = null
    const userId = props.fileInfo?.userId
    if (userId) {
      try {
        // 静默查询：上传账号可能已注销，失败时展示兜底昵称即可
        const res = await getUser(userId, { silent: true })
        if ((res.code === 200 || res.code === 0) && res.data) {
          contributorInfo.value = res.data.user || res.data
        }
      } catch (err) {
        console.warn('获取贡献者详情失败:', err)
      }
    }
  }
})

const formatLabel = computed(() => {
  const format = String(props.fileInfo?.ebookFormat || '').trim()
  return format ? `.${format.toUpperCase()}` : '未知'
})

const contributorName = computed(() => {
  const user = contributorInfo.value || {}
  return user.nickName || user.nickname || user.userName || user.username ||
         props.fileInfo?.createBy || props.fileInfo?.create_by || '匿名英雄'
})

const contributorAvatar = computed(() => {
  const user = contributorInfo.value || {}
  return normalizeAvatar(user.avatar || user.userAvatar || '')
})

const uploaderInitial = computed(() => {
  const name = contributorName.value
  return name && name !== '匿名英雄' ? name.charAt(0).toUpperCase() : '?'
})

const statusInfo = computed(() => {
  const status = props.fileInfo?.ebookStatus ?? props.fileInfo?.status
  if (status === undefined || status === null) return { text: '', class: '' }
  const numStatus = Number(status)
  if (numStatus === 0) return { text: '未审核', class: 'status-pending' }
  if (numStatus === 1) return { text: '审核通过', class: 'status-approved' }
  if (numStatus === 3) return { text: '被举报', class: 'status-reported' }
  return { text: `未知状态(${status})`, class: 'status-pending' }
})

const formatSize = (size) => {
  if (!size) return '未知大小'
  const numSize = Number(size)
  if (isNaN(numSize)) return size
  if (numSize < 1024) return numSize + ' B'
  if (numSize < 1024 * 1024) return (numSize / 1024).toFixed(2) + ' KB'
  return (numSize / (1024 * 1024)).toFixed(2) + ' MB'
}

const formatEbookType = (type) => {
  const typeMap = {
    1: '教材',
    2: '教辅/参考书',
    3: '课外读物',
    4: '其他'
  }
  return typeMap[Number(type)] || type || '未知分类'
}

const copyText = async (text, label) => {
  if (!text || text === '-') return
  const successMsg = `${label} 已复制`
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
      ElMessage.success(successMsg)
      return
    }
    const input = document.createElement('input')
    input.setAttribute('value', text)
    input.style.position = 'fixed'
    input.style.opacity = '0'
    document.body.appendChild(input)
    input.select()
    const successful = document.execCommand('copy')
    document.body.removeChild(input)
    if (successful) {
      ElMessage.success(successMsg)
    } else {
      throw new Error('复制失败')
    }
  } catch (err) {
    console.error('复制失败:', err)
    ElMessage.warning({
      message: '复制失败，您的浏览器不支持自动复制',
      plain: true,
      customClass: 'file-info-msg'
    })
  }
}

const handleReport = () => {
  emit('report-file', props.fileInfo)
}

const handleToggleFavorite = () => {
  if (props.favoriteLoading) return
  emit('toggle-favorite', props.fileInfo)
}

const handleDownload = () => {
  emit('download-file', props.fileInfo)
}
</script>

<style lang="scss" scoped>
/* 与 FileInfoDrawer 同构：整体布局、卡片、底部操作区样式保持一致 */
:deep(.el-drawer) {
  border-radius: 24px 0 0 24px;
  overflow: hidden;
  box-shadow: -8px 0 32px rgba(0, 0, 0, 0.1);
  will-change: transform;
}

.drawer-container {
  height: 100%;
  display: flex;
  flex-direction: column;
  padding: 24px 32px 48px 32px;
  color: var(--ide-text);
  font-family: 'Inter', -apple-system, sans-serif;
  overflow: hidden;
}

.animate-item {
  opacity: 0;
  transform: translateY(10px);
  animation: slideUpFade 0.3s cubic-bezier(0.2, 0.8, 0.2, 1) forwards;
}
.item-1 { animation-delay: 0.05s; }
.item-2 { animation-delay: 0.1s; }
.item-3 { animation-delay: 0.15s; }
.item-4 { animation-delay: 0.2s; }

@keyframes slideUpFade {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.drawer-header {
  display: flex;
  align-items: flex-start;
  gap: 20px;
  padding-bottom: 32px;
  border-bottom: 1px solid var(--ide-border);
}

.file-icon-wrapper {
  flex-shrink: 0;
  width: 64px;
  height: 64px;
  background: rgba(245, 245, 247, 0.8);
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--ide-border);
  box-shadow:
    0 8px 16px rgba(0, 0, 0, 0.04),
    inset 0 2px 4px rgba(255, 255, 255, 0.1);

  html.dark & {
    background: rgba(20, 20, 20, 0.8);
  }

  .huge-icon {
    font-size: 32px;
    color: var(--ide-accent);
  }
}

.file-title-wrapper {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.title-with-status {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  justify-content: space-between;
}

.file-name {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  line-height: 1.4;
  color: var(--ide-text-active);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  text-overflow: ellipsis;
  word-break: break-all;
  flex: 1;
}

.status-badge {
  flex-shrink: 0;
  font-size: 11px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: 8px;
  border: 1px solid transparent;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 6px rgba(0,0,0,0.05);

  &.status-pending {
    color: #e6a23c;
    background: rgba(230, 162, 60, 0.1);
    border-color: rgba(230, 162, 60, 0.2);
  }

  &.status-approved {
    color: #67c23a;
    background: rgba(103, 194, 58, 0.1);
    border-color: rgba(103, 194, 58, 0.2);
  }

  &.status-reported {
    color: #f56c6c;
    background: rgba(245, 108, 108, 0.1);
    border-color: rgba(245, 108, 108, 0.2);
  }
}

.file-tags {
  display: flex;
  gap: 8px;

  span {
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.5px;
    padding: 4px 10px;
    border-radius: 6px;
  }

  .file-ext-tag {
    background: rgba(var(--ide-accent-rgb, 64, 158, 255), 0.1);
    color: var(--ide-accent);
    border: 1px solid rgba(var(--ide-accent-rgb, 64, 158, 255), 0.2);
  }

  .file-size-tag {
    background: rgba(245, 245, 247, 0.8);
    color: var(--ide-text-light);
    border: 1px solid var(--ide-border);

    html.dark & {
      background: rgba(20, 20, 20, 0.8);
    }
  }
}

.drawer-body {
  flex: 1;
  padding: 16px 0 20px 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 24px;

  &::-webkit-scrollbar { display: none; }
}

.info-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.section-title {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--ide-text-light);
  text-transform: uppercase;
  letter-spacing: 1px;
}

.info-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;

  &.single-col {
    grid-template-columns: 1fr;
  }
}

.info-card {
  position: relative;
  background: rgba(255, 255, 255, 0.6);
  border-radius: 12px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  border: 1px solid var(--ide-border);
  transition: all 0.3s ease;
  overflow: hidden;

  html.dark & {
    background: rgba(45, 45, 45, 0.6);
  }

  .info-label {
    font-size: 12px;
    color: var(--ide-text-light);
    font-weight: 500;
  }

  .info-value {
    font-size: 14px;
    color: var(--ide-text-active);
    font-weight: 500;
    word-break: break-all;

    &.time-val {
      font-family: 'JetBrains Mono', 'Consolas', monospace;
      font-size: 13px;
      font-weight: 400;
    }
  }

  .copy-icon {
    position: absolute;
    top: 16px;
    right: 16px;
    font-size: 14px;
    color: var(--ide-accent);
    opacity: 0;
    transform: scale(0.8);
    transition: all 0.2s ease;
  }

  &.copyable {
    cursor: pointer;

    &:hover {
      background: rgba(255, 255, 255, 0.8);
      border-color: rgba(var(--ide-accent-rgb, 64, 158, 255), 0.3);
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.04);
      transform: translateY(-2px);

      html.dark & {
        background: rgba(60, 60, 60, 0.8);
      }

      .copy-icon {
        opacity: 1;
        transform: scale(1);
      }
    }

    &:active {
      transform: translateY(0);
    }
  }
}

.info-card .contributor-avatar {
  position: absolute;
  right: 16px;
  bottom: 16px;
  border: 2px solid var(--ide-bg);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
}

.drawer-footer {
  padding-top: 12px;
  margin-bottom: 8px;
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 100%;
  height: 44px;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  border: none;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);
  outline: none;

  .el-icon {
    font-size: 18px;
  }

  &:active {
    transform: scale(0.98);
  }
}

.primary-btn {
  background: var(--ide-accent);
  color: white;
  box-shadow: 0 4px 12px rgba(var(--ide-accent-rgb, 64, 158, 255), 0.3);
  flex: 1 1 40%;

  &:hover {
    background: var(--ide-accent-hover, #66b1ff);
    box-shadow: 0 6px 16px rgba(var(--ide-accent-rgb, 64, 158, 255), 0.4);
    transform: translateY(-2px);
  }
}

.favorite-btn {
  background: var(--ide-accent);
  color: white;
  box-shadow: 0 4px 12px rgba(var(--ide-accent-rgb, 64, 158, 255), 0.3);
  flex: 1 1 40%;

  &:hover:not(:disabled) {
    background: var(--ide-accent-hover, #66b1ff);
    box-shadow: 0 6px 16px rgba(var(--ide-accent-rgb, 64, 158, 255), 0.4);
    transform: translateY(-2px);
  }

  &:disabled {
    cursor: not-allowed;
    opacity: 0.66;
  }
}

.secondary-btn {
  background-color: rgba(245, 245, 247, 0.8);
  color: var(--ide-text);
  border: 1px solid var(--ide-border);
  flex: 1 1 40%;

  html.dark & {
    background-color: rgba(20, 20, 20, 0.8);
  }

  &:hover {
    border-color: var(--ide-accent);
    color: var(--ide-accent);
    transform: translateY(-2px);
  }
}

.danger-btn-full {
  background: rgba(245, 245, 247, 0.8);
  color: #f56c6c;
  border: 1px solid var(--ide-border);
  flex: 1 1 100%;

  html.dark & {
    background: rgba(20, 20, 20, 0.8);
  }

  &:hover {
    background: #f56c6c;
    color: white;
    border-color: #f56c6c;
    box-shadow: 0 4px 12px rgba(245, 108, 108, 0.3);
    transform: translateY(-2px);
  }
}
</style>
