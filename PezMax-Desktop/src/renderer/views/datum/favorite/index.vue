<template>
  <div class="favorite-page" :class="{ embedded }">
    <div v-if="!props.embedded" class="page-hero">
      <div>
        <h3>我的收藏</h3>
        <p>当前账号收藏的文件和书签。</p>
      </div>
    </div>

    <div class="top-bar">
      <div class="mode-tabs" role="tablist">
        <button :class="{ active: activeType === 'file' }" @click="activeType = 'file'">
          <span>文件</span>
        </button>
        <button :class="{ active: activeType === 'ebook' }" @click="activeType = 'ebook'">
          <span>电子书</span>
        </button>
        <button :class="{ active: activeType === 'bookmark' }" @click="activeType = 'bookmark'">
          <span>书签</span>
        </button>
      </div>
      <el-input v-model.trim="query.keyword" :placeholder="favoriteSearchPlaceholder" clearable @keyup.enter="loadList">
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <el-button class="reset-btn" @click="resetQuery">重置</el-button>
    </div>

    <el-table v-if="activeType === 'file'" v-loading="loading" :data="pagedList" class="panel-table" empty-text="暂无收藏文件">
      <el-table-column label="科目" prop="subject" min-width="120" />
      <el-table-column label="文件名称" prop="fileName" min-width="220" />
      <el-table-column label="文件类型" prop="fileFormat" min-width="120" />
      <el-table-column label="文件大小" prop="fileSize" min-width="140" />
      <el-table-column label="学校" prop="school" min-width="120" />
      <el-table-column label="操作" width="260" align="center" class-name="action-column" header-class-name="action-column">
        <template #default="{ row }">
          <el-button class="row-action download-action" :loading="downloadingIds.has(row.fileId)" @click="downloadFile(row)">
            <el-icon><Download /></el-icon>
            <span>下载</span>
          </el-button>
          <el-button class="row-action jump-action" @click="jumpToFile(row)">
            <span>跳转</span>
          </el-button>
          <el-button class="row-action favorite-action" :loading="removingIds.has(row.fileId)" @click="removeItem(row)">
            <svg class="heart-icon" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M12 21.35 10.55 20.03C5.4 15.36 2 12.27 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09A6 6 0 0 1 16.5 3C19.58 3 22 5.42 22 8.5c0 3.77-3.4 6.86-8.55 11.54L12 21.35z" />
            </svg>
            <span>取消</span>
          </el-button>
        </template>
      </el-table-column>
      <template #empty>
        <div class="empty-state">
          <el-icon class="empty-icon"><Star /></el-icon>
          <div class="empty-title">暂无收藏</div>
          <div class="empty-desc">收藏的试卷和资料会显示在这里</div>
        </div>
      </template>
    </el-table>

    <el-table v-else-if="activeType === 'ebook'" v-loading="loading" :data="pagedList" class="panel-table" empty-text="暂无收藏电子书">
      <el-table-column label="书名" prop="fileName" min-width="220" show-overflow-tooltip />
      <el-table-column label="学科分类" prop="subject" min-width="110" />
      <el-table-column label="格式" prop="fileFormat" width="90" />
      <el-table-column label="大小" prop="fileSize" min-width="110" />
      <el-table-column label="操作" width="260" align="center" class-name="action-column" header-class-name="action-column">
        <template #default="{ row }">
          <el-button class="row-action download-action" :loading="downloadingIds.has(`ebook-${row.fileId}`)" @click="downloadEbook(row)">
            <el-icon><Download /></el-icon>
            <span>下载</span>
          </el-button>
          <el-button class="row-action jump-action" @click="jumpToEbook(row)">
            <span>跳转</span>
          </el-button>
          <el-button class="row-action favorite-action" :loading="removingIds.has(`ebook-${row.fileId}`)" @click="removeItem(row)">
            <svg class="heart-icon" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M12 21.35 10.55 20.03C5.4 15.36 2 12.27 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09A6 6 0 0 1 16.5 3C19.58 3 22 5.42 22 8.5c0 3.77-3.4 6.86-8.55 11.54L12 21.35z" />
            </svg>
            <span>取消</span>
          </el-button>
        </template>
      </el-table-column>
      <template #empty>
        <div class="empty-state">
          <el-icon class="empty-icon"><Star /></el-icon>
          <div class="empty-title">暂无收藏电子书</div>
          <div class="empty-desc">收藏的电子书会显示在这里</div>
        </div>
      </template>
    </el-table>

    <el-table v-else v-loading="loading" :data="pagedList" class="panel-table" empty-text="暂无收藏书签">
      <el-table-column label="标题" prop="title" min-width="180" show-overflow-tooltip />
      <el-table-column label="链接" prop="url" min-width="220" show-overflow-tooltip />
      <el-table-column label="专栏" prop="collection" min-width="130" show-overflow-tooltip />
      <el-table-column label="类型" prop="resourceTypeLabel" width="120" />
      <el-table-column label="操作" width="132" align="center" class-name="action-column" header-class-name="action-column">
        <template #default="{ row }">
          <el-button class="row-action favorite-action" :loading="removingIds.has(`bookmark-${row.id}`)" @click="removeItem(row)">
            <svg class="heart-icon" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M12 21.35 10.55 20.03C5.4 15.36 2 12.27 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09A6 6 0 0 1 16.5 3C19.58 3 22 5.42 22 8.5c0 3.77-3.4 6.86-8.55 11.54L12 21.35z" />
            </svg>
            <span>取消</span>
          </el-button>
        </template>
      </el-table-column>
      <template #empty>
        <div class="empty-state">
          <el-icon class="empty-icon"><Star /></el-icon>
          <div class="empty-title">暂无收藏书签</div>
          <div class="empty-desc">收藏的公共书签会显示在这里</div>
        </div>
      </template>
    </el-table>

    <div v-if="total > 0" class="pager-wrap">
      <el-pagination
        v-model:current-page="query.pageNum"
        v-model:page-size="query.pageSize"
        layout="total, prev, pager, next"
        :total="total"
        @current-change="syncPagedList"
      />
    </div>
  </div>
</template>

<script setup>
import { reactive, ref, watch, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Star, Download } from '@element-plus/icons-vue'
import useUserStore from '@/store/modules/user'
import { getInfo } from '@/api/login'
import { listFavorite, delFavorite } from '@/api/datum/favorite'
import { listFavoriteBookmark, delBookmarkFavorite } from '@/api/datum/bookmarkFavorite'
import { listDesktopEbookFavorite, delDesktopEbookFavorite, ebookDownloadUrl, ebookPreviewUrl } from '@/api/datum/ebook'
import { getToken } from '@/utils/auth'
import { fetchAllPages } from '@/utils/pagination'

defineOptions({ name: 'FavoritePage' })
const baseURL = import.meta.env.VITE_APP_BASE_API
const props = defineProps({
  embedded: { type: Boolean, default: false }
})
const userStore = useUserStore()
const router = useRouter()

const loading = ref(false)
const list = ref([])
const pagedList = ref([])
const total = ref(0)
const currentUserId = ref('')
const removingIds = reactive(new Set())
const downloadingIds = reactive(new Set())
const activeType = ref('file')

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: ''
})

const resourceTypeOptions = [
  { label: '网课/视频', value: 'course' },
  { label: '博客/文章', value: 'blog' },
  { label: '学术/论文', value: 'paper' },
  { label: '工具/开源', value: 'tool' },
  { label: '娱乐/资源', value: 'entertainment' },
  { label: '其他', value: 'other' }
]

function formatFileSizeBytes(val) {
  if (val === null || val === undefined || val === '') return '-'
  const n = Number(val)
  if (!Number.isFinite(n)) return `${val} 字节`
  if (n < 1024) return `${Math.trunc(n)} B`
  const kb = n / 1024
  if (kb < 1024) return `${kb.toFixed(1)} KB`
  const mb = kb / 1024
  if (mb < 1024) return `${mb.toFixed(1)} MB`
  const gb = mb / 1024
  return `${gb.toFixed(2)} GB`
}

function truncateFileName(name) {
  if (!name) return '-'
  if (name.length <= 10) return name
  return name.slice(0, 10) + '....'
}

const resolveCurrentUser = async () => {
  if (userStore.id) {
    currentUserId.value = `${userStore.id}`
    return
  }
  const info = await getInfo()
  currentUserId.value = `${info?.user?.userId || ''}`
}

const buildFileMeta = async (row) => {
  return {
    ...row,
    fileName: truncateFileName(row.fileName) || `文件-${row.fileId}`,
    fileFormat: row.fileFormat || '-',
    fileSize: formatFileSizeBytes(row.fileSize),
    school: row.fileSchool || row.school || '-',
    subject: row.fileSubject || row.subject || '-'
  }
}

const buildEbookMeta = (row) => {
  return {
    ...row,
    // 统一映射到文件表格的字段名，下载/跳转/取消逻辑复用同一套 id 约定
    fileId: row.ebookId ?? row.fileId,
    fileName: truncateFileName(row.ebookName) || `电子书-${row.ebookId}`,
    fileFormat: row.ebookFormat || '-',
    fileSize: formatFileSizeBytes(row.ebookSize),
    subject: row.ebookSubject || row.subject || '-',
    fileUrl: row.ebookUrl || '',
    ebookStatus: row.ebookStatus,
    userId: row.userId
  }
}

const buildBookmarkMeta = (row) => {
  return {
    ...row,
    title: row.title || `书签-${row.id}`,
    collection: row.collection || '-',
    resourceTypeLabel: resourceTypeOptions.find((item) => item.value === row.resourceType)?.label || row.resourceType || '-'
  }
}

const syncPagedList = () => {
  const start = (query.pageNum - 1) * query.pageSize
  pagedList.value = list.value.slice(start, start + query.pageSize)
}

const loadList = async () => {
  loading.value = true
  try {
    if (!currentUserId.value) {
      await resolveCurrentUser()
    }
    // 单页上限 100：按 total 逐页拉全量后本地过滤/分页（与原 pageSize=1000 行为一致）
    const lister = activeType.value === 'file'
      ? listFavorite
      : activeType.value === 'ebook'
        ? listDesktopEbookFavorite
        : listFavoriteBookmark
    const res = await fetchAllPages(lister, { userId: currentUserId.value })
    const source = res.rows || []
    const rows = activeType.value === 'file'
      ? await Promise.all(source.map(buildFileMeta))
      : activeType.value === 'ebook'
        ? source.map(buildEbookMeta)
        : source.map(buildBookmarkMeta)
    const keyword = query.keyword.toLowerCase()
    list.value = keyword
      ? rows.filter((item) => {
        const fields = activeType.value === 'file'
          ? [item.subject]
          : activeType.value === 'ebook'
            ? [item.subject, item.author, item.publisher]
            : [item.title, item.url, item.collection, item.subject, item.description]
        return fields.some((value) => `${value || ''}`.toLowerCase().includes(keyword))
      })
      : rows
    total.value = list.value.length
    query.pageNum = 1
    syncPagedList()
  } finally {
    loading.value = false
  }
}

const resetQuery = () => {
  query.keyword = ''
  query.pageNum = 1
  loadList()
}

const removeItem = async (row) => {
  const isBookmark = activeType.value === 'bookmark'
  const isEbook = activeType.value === 'ebook'
  const id = isBookmark ? `bookmark-${row.id}` : isEbook ? `ebook-${row.fileId}` : row.fileId
  const title = isBookmark ? row.title : row.fileName
  await ElMessageBox.confirm(`确认取消收藏「${title}」吗？`, '提示', { type: 'warning' })
  removingIds.add(id)
  try {
    if (isBookmark) {
      await delBookmarkFavorite(currentUserId.value, row.id)
    } else if (isEbook) {
      await delDesktopEbookFavorite(currentUserId.value, row.fileId)
    } else {
      await delFavorite(currentUserId.value, row.fileId)
    }
    ElMessage.success('已取消收藏')
    const detail = isBookmark
      ? { bookmarkId: row.id, favorited: false, type: 'bookmark' }
      : isEbook
        ? { ebookId: row.fileId, favorited: false, type: 'ebook' }
        : { fileId: row.fileId, favorited: false, type: 'file' }
    window.dispatchEvent(new CustomEvent('favorite-updated', { detail }))
    await loadList()
  } finally {
    removingIds.delete(id)
  }
}

const downloadFile = async (row) => {
  const fileId = row.fileId
  const fileName = row.fileName || `文件-${fileId}`

  downloadingIds.add(fileId)
  try {
    // 统一下载方法：主进程流式写盘 + /datum/download/file（服务端记录/鉴权/限速）
    const result = await window.electronAPI.downloadFileDirectly({
      url: `${baseURL}/datum/download/file?fileId=${fileId}`,
      fileName,
      token: getToken()
    })

    if (result && result.success) {
      ElMessage.success(`下载完成: ${fileName}`)
      try {
        const record = {
          fileId: Number(fileId) || 0,
          fileName,
          fileUrl: row.fileUrl || '',
          fileSize: Number(row.fileSize) || 0,
          fileFormat: row.fileFormat || '',
          fileSchool: row.fileSchool || '',
          fileSubject: row.fileSubject || '',
          fileYear: row.fileYear != null ? Number(row.fileYear) : null,
          fileType: row.fileType != null ? Number(row.fileType) : null,
          localPath: result.filePath || '',
          userId: userStore.id ? Number(userStore.id) : null
        }
        await window.electronAPI.downloadRecords.add(record)
        await window.electronAPI.downloadRecords.flush()
      } catch (e) {
        console.warn('记录下载失败:', e)
      }
    } else if (!result || result.reason !== 'canceled') {
      ElMessage.error(`保存失败: ${(result && result.message) || '未知错误'}`)
    }
  } catch (e) {
    console.error('下载出错:', e)
    ElMessage.error(`下载失败：${e?.message || '请检查网络'}`)
  } finally {
    downloadingIds.delete(fileId)
  }
}

const jumpToFile = (row) => {
  const fileData = {
    id: row.fileId,
    label: row.fileName,
    url: row.fileUrl,
    type: 'file',
    fileInfo: { ...row }
  }
  sessionStorage.setItem('pendingOpenFile', JSON.stringify(fileData))
  router.push('/index')
}

// 电子书下载：/datum/download/ebook 专有端点（落 ptmj_ebook_download 记录）+
// 本地 SQLite 下载记录（ebook 前缀类型，与试卷记录同一张表）
const downloadEbook = async (row) => {
  const ebookId = row.fileId
  const format = String(row.fileFormat || '').toLowerCase()
  const fileName = (row.ebookName || row.fileName || `电子书-${ebookId}`) + (format && format !== '-' ? `.${format}` : '')

  downloadingIds.add(`ebook-${ebookId}`)
  try {
    const result = await window.electronAPI.downloadFileDirectly({
      url: ebookDownloadUrl(ebookId),
      fileName,
      token: getToken()
    })
    if (result && result.success) {
      ElMessage.success(`下载完成: ${fileName}`)
      try {
        await window.electronAPI.downloadRecords.add({
          fileId: -Number(ebookId) || 0, // 负数 id 标记电子书记录，避免与试卷 fileId 混淆
          fileName,
          fileUrl: row.fileUrl || '',
          fileSize: Number(row.ebookSize) || 0,
          fileFormat: row.fileFormat || '',
          fileSchool: '',
          fileSubject: row.ebookSubject || row.subject || '',
          fileYear: null,
          fileType: row.ebookType != null ? Number(row.ebookType) : null,
          localPath: result.filePath || '',
          userId: userStore.id ? Number(userStore.id) : null
        })
        await window.electronAPI.downloadRecords.flush()
      } catch (e) {
        console.warn('记录电子书下载失败:', e)
      }
    } else if (!result || result.reason !== 'canceled') {
      ElMessage.error(`保存失败: ${(result && result.message) || '未知错误'}`)
    }
  } catch (e) {
    console.error('电子书下载出错:', e)
    ElMessage.error(`下载失败：${e?.message || '请检查网络'}`)
  } finally {
    downloadingIds.delete(`ebook-${ebookId}`)
  }
}

// 跳转到首页并打开电子书预览（与试卷跳转同一 pendingOpenFile 通道）
const jumpToEbook = (row) => {
  const format = String(row.fileFormat || '').toLowerCase()
  const ebookData = {
    id: `ebook-${row.fileId}`,
    rawId: row.fileId,
    label: row.ebookName || row.fileName,
    type: 'ebook',
    fileExt: format,
    cover: row.coverUrl || '',
    ebookSize: row.ebookSize,
    ebookSubject: row.ebookSubject || row.subject,
    ebookType: row.ebookType,
    userId: row.userId,
    originalData: {
      type: 'ebook',
      rawId: row.fileId,
      ebookId: row.fileId,
      ebookName: row.ebookName || row.fileName,
      ebookFormat: format,
      ebookUrl: row.fileUrl,
      ebookSize: row.ebookSize,
      ebookSubject: row.ebookSubject || row.subject,
      ebookType: row.ebookType,
      userId: row.userId
    }
  }
  sessionStorage.setItem('pendingOpenFile', JSON.stringify(ebookData))
  router.push('/index')
}

const favoriteSearchPlaceholder = computed(() => {
  if (activeType.value === 'file') return '按科目筛选'
  if (activeType.value === 'ebook') return '按学科、作者或出版社筛选'
  return '按书签标题、链接或专栏筛选'
})

watch(activeType, () => {
  query.pageNum = 1
  loadList()
})
onMounted(loadList)

defineExpose({ refresh: loadList, total })
</script>

<style scoped lang="scss">
.favorite-page {
  background: var(--ide-editor-bg, #fff);
  border: 1px solid var(--ide-border, #ebeef5);
  border-radius: 16px;
  padding: 20px;
  min-width: 680px;
  box-shadow: 0 12px 26px rgba(15, 23, 42, 0.06);
  transition: box-shadow 0.24s ease, border-color 0.24s ease;
}
.favorite-page:hover {
  border-color: rgba(var(--ide-accent-rgb, 64, 158, 255), 0.35);
  box-shadow: 0 16px 34px rgba(15, 23, 42, 0.08);
}
.favorite-page.embedded {
  border: none;
  box-shadow: none;
  padding: 0;
}
.page-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  border-radius: 14px;
  background: linear-gradient(
      135deg,
      rgba(var(--ide-accent-rgb, 64, 158, 255), 0.16),
      rgba(var(--ide-accent-rgb, 64, 158, 255), 0.04)
  );
  margin-bottom: 16px;
  border: 1px solid rgba(var(--ide-accent-rgb, 64, 158, 255), 0.18);
}
.page-hero h3 {
  margin: 0;
  font-size: 22px;
  letter-spacing: 0.3px;
  color: var(--ide-text-active, #303133);
}
.page-hero p {
  margin: 4px 0 0;
  color: var(--ide-text-light, #909399);
  font-size: 13px;
}
.top-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  align-items: center;
  padding: 14px;
  min-width: 640px;
  border: 1px solid var(--ide-border, #ebeef5);
  border-radius: 16px;
  background: color-mix(in srgb, var(--ide-panel-bg, #fff) 94%, transparent);
}
.top-bar .el-input {
  min-width: 200px;
  max-width: 560px;
}
.mode-tabs {
  display: inline-flex;
  flex: 0 0 auto;
  padding: 4px;
  border: 1px solid var(--ide-border, #dcdfe6);
  border-radius: 12px;
  background: var(--ide-editor-bg, #fff);
}
.mode-tabs button {
  height: 34px;
  min-width: 72px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: var(--ide-text, #606266);
  cursor: pointer;
  font-weight: 700;
}
.mode-tabs button.active {
  color: var(--ide-accent, #409eff);
  background: rgba(var(--ide-accent-rgb, 64, 158, 255), 0.12);
}
.top-bar :deep(.el-input__wrapper) {
  min-height: 42px;
  border-radius: 12px;
  box-shadow: 0 0 0 1px var(--ide-border, #dcdfe6) inset;
}
.reset-btn {
  min-height: 42px;
  flex-shrink: 0;
  border-radius: 12px;
  padding: 0 22px;
  font-weight: 700;
}
.panel-table {
  border-radius: 16px;
  overflow: hidden;
  min-width: 640px;
  border: 1px solid var(--ide-border, #ebeef5);
  background: color-mix(in srgb, var(--ide-editor-bg, #fff) 92%, transparent);
}
.favorite-page :deep(.el-table__body-wrapper) {
  overflow-x: auto;
}
:deep(.panel-table .el-table__header th.el-table__cell) {
  height: 56px;
  padding: 0;
  background: rgba(var(--ide-accent-rgb, 64, 158, 255), 0.06);
  color: var(--ide-text-active, #303133);
  font-weight: 700;
}
:deep(.panel-table .el-table__header .cell) {
  line-height: 56px;
  padding-left: 20px;
  padding-right: 20px;
}
:deep(.panel-table .el-table__cell) {
  padding: 18px 0;
}
:deep(.panel-table .el-table__body .cell) {
  padding-left: 20px;
  padding-right: 20px;
}
:deep(.panel-table .action-column .cell) {
  display: flex;
  justify-content: center;
  overflow: visible;
  padding-left: 12px;
  padding-right: 12px;
  text-overflow: clip;
  white-space: nowrap;
}
:deep(.panel-table .el-table__row td.el-table__cell) {
  border-bottom-color: rgba(148, 163, 184, 0.18);
}
:deep(.panel-table .el-table__row) {
  transition: background-color 0.22s ease;
}
:deep(.panel-table .el-table__row:hover td.el-table__cell) {
  background: rgba(var(--ide-accent-rgb, 64, 158, 255), 0.08) !important;
}
:deep(.panel-table .el-table__empty-block) {
  min-height: 176px;
}
:deep(.panel-table .el-table__empty-text) {
  width: 100%;
  line-height: normal;
}
.pager-wrap {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
.empty-state {
  min-height: 176px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--ide-text-light, #909399);
}
.empty-icon {
  margin-bottom: 12px;
  color: var(--ide-accent, #409eff);
  font-size: 32px;
  opacity: 0.55;
}
.empty-title {
  color: var(--ide-text, #606266);
  font-size: 15px;
  font-weight: 650;
  line-height: 1.4;
}
.empty-desc {
  margin-top: 4px;
  color: var(--ide-text-light, #909399);
  font-size: 13px;
  line-height: 1.5;
}
.row-action {
  min-width: auto;
  height: 32px;
  padding: 0 4px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--ide-accent, #409eff);
  font-weight: 650;
  transition: color 0.18s ease, opacity 0.18s ease;
}
.row-action:hover {
  background: transparent;
  color: var(--ide-accent, #409eff);
  opacity: 0.72;
}
.heart-icon {
  width: 16px;
  height: 16px;
  fill: currentColor;
  margin-right: 5px;
}
@media (max-width: 720px) {
  .top-bar {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
