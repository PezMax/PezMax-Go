<template>
  <div>
    <el-popover ref="noticePopover" placement="bottom-end" :width="320" trigger="manual" v-model:visible="noticeVisible" popper-class="notice-popover">
      <!-- 弹出内容 -->
      <div class="notice-header">
        <span class="notice-title">通知公告</span>
        <span class="notice-mark-all" @click="markAllRead">全部已读</span>
      </div>
      <div v-if="noticeLoading" class="notice-loading">
        <el-icon class="is-loading"><Loading /></el-icon> 加载中...
      </div>
      <div v-else-if="noticeList.length === 0" class="notice-empty">
        <el-icon style="font-size:24px;display:block;margin-bottom:6px;"><Postcard /></el-icon>
        暂无公告
      </div>
      <div v-else>
        <div v-for="item in noticeList" :key="item.noticeId" class="notice-item" :class="{ 'is-read': item.isRead }" @click="previewNotice(item)">
          <el-tag size="small" :type="noticeTagType(item.noticeType)" class="notice-tag">
            {{ noticeTypeLabel(item.noticeType) }}
          </el-tag>
          <span class="notice-item-title">{{ item.noticeTitle }}</span>
          <span class="notice-item-date">{{ item.createTime }}</span>
        </div>
      </div>

      <!-- 触发器 -->
      <template #reference>
        <div class="right-menu-item hover-effect notice-trigger" @mouseenter="onNoticeEnter" @mouseleave="onNoticeLeave">
          <svg-icon icon-class="bell" />
          <span v-if="unreadCount > 0" class="notice-badge">{{ unreadCount }}</span>
        </div>
      </template>
    </el-popover>

    <!-- 预览弹窗 -->
    <el-dialog v-model="previewVisible" :title="previewTitle" width="680px" append-to-body class="notice-preview-dialog">
      <div class="notice-preview-meta">
        <el-tag size="small" :type="noticeTagType(previewNoticeType)">
          {{ noticeTypeLabel(previewNoticeType) }}
        </el-tag>
        <span v-if="previewCreateBy" class="notice-preview-info">
          <el-icon><User /></el-icon> {{ previewCreateBy }}
        </span>
        <span v-if="previewCreateTime" class="notice-preview-info">
          <el-icon><Timer /></el-icon> {{ previewCreateTime }}
        </span>
      </div>
      <div class="notice-preview-divider"></div>
      <div class="notice-preview-content" v-html="previewContent"></div>
    </el-dialog>
  </div>
</template>

<script setup>
// 通知铃铛：已切换至 kadmin datum 通知（/system/notification/user/popup，与
// NotificationCenter/home 弹窗同源）。ptmj_notification 是广播表、无 per-user
// 已读模型，已读状态按桌面端本地库惯例记在主进程 SQLite（notice_reads 表）。
import { getUserPopupNotifications } from '@/api/datum/notification'
import useUserStore from '@/store/modules/user'

// 1-版本更新 2-系统故障 3-系统维护 4-资料下架 5-日常滚动
const NOTICE_TYPE_LABELS = { 1: '版本更新', 2: '系统故障', 3: '系统维护', 4: '资料下架', 5: '公告' }

const userStore = useUserStore()
const noticePopover = ref(null)
const noticeList = ref([])
const unreadCount = ref(0)
const noticeLoading = ref(false)
const noticeVisible = ref(false)
const noticeLeaveTimer = ref(null)
const previewVisible = ref(false)
const previewTitle = ref('')
const previewContent = ref('')
const previewNoticeType = ref('')
const previewCreateBy = ref('')
const previewCreateTime = ref('')

function noticeTypeLabel(type) {
  return NOTICE_TYPE_LABELS[`${type}`] || '公告'
}

function noticeTagType(type) {
  switch (`${type}`) {
    case '2':
    case '3':
      return 'warning'
    case '4':
      return 'danger'
    default:
      return 'success'
  }
}

// 解析当前登录用户 ID（与 home/index.vue 弹窗通知同一兜底链）
async function resolveCurrentUserId() {
  if (userStore.id) return `${userStore.id}`
  try {
    await userStore.getInfo()
  } catch (error) {
    console.warn('铃铛获取用户信息失败：', error)
  }
  return `${userStore.id || ''}`
}

// 本地已读状态经主进程 SQLite 存取；非 Electron 环境降级为空集（均视为未读）
async function loadLocalReadIds(userId) {
  try {
    const result = await window.electronAPI?.noticeReads?.list(userId)
    return result?.success ? result.ids || [] : []
  } catch (error) {
    console.warn('铃铛本地已读读取失败：', error)
    return []
  }
}

function markLocalRead(userId, notifyIds) {
  try {
    const result = window.electronAPI?.noticeReads?.mark(userId, notifyIds)
    if (result && typeof result.catch === 'function') result.catch(() => {})
  } catch (error) {
    console.warn('铃铛本地已读写入失败：', error)
  }
}

// 加载站内通知（popup 端点）并合并本地已读状态
async function loadNoticeTop() {
  noticeLoading.value = true
  try {
    const userId = await resolveCurrentUserId()
    const res = await getUserPopupNotifications(userId || undefined)
    const rows = ((res.code === 200 || res.code === 0 ? res.data : res) || []).map((item) => ({
      noticeId: item.notifyId,
      noticeTitle: item.title,
      noticeType: item.notifyType,
      createTime: item.createTime,
      content: item.content
    }))
    const readIds = await loadLocalReadIds(userId)
    const readSet = new Set(readIds.map(Number))
    noticeList.value = rows.map((item) => ({ ...item, isRead: readSet.has(Number(item.noticeId)) }))
    unreadCount.value = noticeList.value.filter((item) => !item.isRead).length
  } catch (error) {
    console.warn('铃铛通知加载失败：', error)
    noticeList.value = []
    unreadCount.value = 0
  } finally {
    noticeLoading.value = false
  }
}

onMounted(() => loadNoticeTop())

// 鼠标移入铃铛区域
function onNoticeEnter() {
  clearTimeout(noticeLeaveTimer.value)
  noticeVisible.value = true
  nextTick(() => {
    const popper = noticePopover.value?.popperRef?.contentRef
    if (popper && !popper._noticeBound) {
      popper._noticeBound = true
      popper.addEventListener('mouseenter', () => clearTimeout(noticeLeaveTimer.value))
      popper.addEventListener('mouseleave', () => {
        noticeLeaveTimer.value = setTimeout(() => { noticeVisible.value = false }, 100)
      })
    }
  })
}

// 鼠标离开铃铛区域
function onNoticeLeave() {
  noticeLeaveTimer.value = setTimeout(() => { noticeVisible.value = false }, 150)
}

// 预览公告详情（popup 载荷已含正文，直接取列表项，省一次详情请求）
function previewNotice(item) {
  const userId = `${userStore.id || ''}`
  if (!item.isRead) {
    markLocalRead(userId, [item.noticeId])
    const idx = noticeList.value.indexOf(item)
    if (idx !== -1) noticeList.value[idx] = { ...item, isRead: true }
    unreadCount.value = Math.max(0, unreadCount.value - 1)
  }
  previewTitle.value = item.noticeTitle
  previewContent.value = item.content
  previewNoticeType.value = item.noticeType
  previewCreateBy.value = ''
  previewCreateTime.value = item.createTime
  previewVisible.value = true
}

// 全部已读
function markAllRead() {
  const ids = noticeList.value.filter((n) => !n.isRead).map((n) => n.noticeId)
  if (!ids.length) return
  markLocalRead(`${userStore.id || ''}`, ids)
  noticeList.value = noticeList.value.map((n) => ({ ...n, isRead: true }))
  unreadCount.value = 0
}
</script>

<style lang="scss" scoped>
.notice-trigger {
  position: relative;
  transform: translateX(-6px);
  .svg-icon { width: 1.2em; height: 1.2em; vertical-align: -0.2em; }
  .notice-badge {
    position: absolute;
    top: 7px;
    right: -3px;
    background: #f56c6c;
    color: #fff;
    border-radius: 10px;
    font-size: 10px;
    height: 16px;
    line-height: 16px;
    padding: 0 4px;
    min-width: 16px;
    text-align: center;
    white-space: nowrap;
    pointer-events: none;
  }
}
.notice-popover { padding: 0 !important; }
.notice-popover .notice-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  background: #f7f9fb;
  border-bottom: 1px solid #eee;
  font-size: 13px;
  font-weight: 600;
  color: #333;
}
.notice-popover .notice-mark-all {
  font-size: 12px;
  color: var(--el-color-primary);
  font-weight: normal;
  cursor: pointer;
}
.notice-popover .notice-mark-all:hover { color: #2b7cc1; }
.notice-popover .notice-loading,
.notice-popover .notice-empty {
  padding: 24px;
  text-align: center;
  color: #bbb;
  font-size: 12px;
  line-height: 1.8;
}
.notice-popover .notice-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-bottom: 1px solid #f5f5f5;
  cursor: pointer;
  transition: background 0.15s;
}
.notice-popover .notice-item:last-child { border-bottom: none; }
.notice-popover .notice-item:hover { background: #f7f9fb; }
.notice-popover .notice-item.is-read .notice-tag,
.notice-popover .notice-item.is-read .notice-item-title,
.notice-popover .notice-item.is-read .notice-item-date { opacity: 0.45; filter: grayscale(1); color: #999; }
.notice-popover .notice-tag { flex-shrink: 0; }
.notice-popover .notice-item-title {
  flex: 1;
  font-size: 12px;
  color: #333;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.notice-popover .notice-item-date {
  flex-shrink: 0;
  font-size: 11px;
  color: #bbb;
}
</style>

<style>
.notice-preview-dialog .el-dialog__body { padding: 0 10px 10px; }
.notice-preview-dialog .notice-preview-meta {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 0;
  font-size: 12px;
  color: #888;
}
.notice-preview-dialog .notice-preview-info { display: flex; align-items: center; gap: 4px; }
.notice-preview-dialog .notice-preview-divider {
  height: 1px;
  background: linear-gradient(to right, transparent, #e2e8f0, transparent);
  margin-bottom: 16px;
}
.notice-preview-dialog .notice-preview-content {
  font-size: 14px;
  line-height: 1.85;
  color: #2d3748;
  word-break: break-word;
}
.notice-preview-dialog .notice-preview-content img { max-width: 100%; border-radius: 4px; }
.notice-preview-dialog .notice-preview-content p { margin: 0 0 1em; }
.notice-preview-dialog .notice-preview-content a { color: var(--el-color-primary); text-decoration: underline; }
</style>
