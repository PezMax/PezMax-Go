<template>
    <div class="ebook-page">
        <div class="ebook-hero">
            <svg-icon icon-class="education" class="ebook-hero-icon" />
            <div class="ebook-hero-text">
                <h3>电子书</h3>
                <p>电子书在线阅读与下载，支持 PDF / EPUB 在线预览。</p>
            </div>
            <div class="ebook-search">
                <el-input
                    v-model="keyword"
                    placeholder="搜索书名 / 作者 / 出版社"
                    clearable
                    :prefix-icon="Search"
                    @keyup.enter="handleSearch"
                    @clear="handleSearch"
                />
                <el-button type="primary" :icon="Search" @click="handleSearch">搜索</el-button>
                <el-button :icon="Refresh" circle title="刷新" @click="loadBooks(1)" />
            </div>
        </div>

        <div class="ebook-body">
            <!-- 左侧书架：不做年份/学校细分，直接按书名展示 -->
            <div class="ebook-shelf">
                <div v-loading="loading" class="ebook-list">
                    <div
                        v-for="book in books"
                        :key="book.ebookId"
                        :class="[
                            'ebook-item',
                            { active: current && current.ebookId === book.ebookId }
                        ]"
                        @click="selectBook(book)"
                    >
                        <div class="ebook-cover">
                            <img
                                v-if="book.coverUrl"
                                :src="normalizeFileUrl(book.coverUrl)"
                                alt=""
                                draggable="false"
                            />
                            <svg-icon v-else icon-class="education" class="cover-fallback" />
                        </div>
                        <div class="ebook-meta">
                            <div class="ebook-title" :title="book.ebookName">
                                {{ book.ebookName }}
                            </div>
                            <div class="ebook-sub" :title="authorLine(book)">
                                {{ authorLine(book) }}
                            </div>
                            <div class="ebook-tags">
                                <span class="ebook-format" :class="`fmt-${formatOf(book)}`">{{
                                    formatOf(book)
                                }}</span>
                                <span v-if="book.ebookSize" class="ebook-size">{{
                                    formatSize(book.ebookSize)
                                }}</span>
                            </div>
                        </div>
                        <el-icon
                            class="ebook-download-icon"
                            title="下载"
                            @click.stop="downloadBook(book)"
                        >
                            <Download />
                        </el-icon>
                    </div>
                    <div v-if="!loading && books.length === 0" class="ebook-empty">
                        <img
                            class="empty-img"
                            src="@/assets/images/ebook.png"
                            alt=""
                            draggable="false"
                        />
                        <div class="empty-title">暂无电子书</div>
                        <div class="empty-desc">
                            书架上还没有已上架的电子书{{ keyword ? '，试试换个关键词' : '' }}
                        </div>
                    </div>
                </div>
                <el-pagination
                    v-if="total > pageSize"
                    class="ebook-pager"
                    small
                    background
                    layout="prev, pager, next"
                    :total="total"
                    :page-size="pageSize"
                    :current-page="pageNum"
                    @current-change="(page) => loadBooks(page)"
                />
            </div>

            <!-- 右侧阅读器 -->
            <div class="ebook-reader">
                <template v-if="current">
                    <div class="reader-header">
                        <div class="reader-info">
                            <span class="reader-title" :title="current.ebookName">{{
                                current.ebookName
                            }}</span>
                            <span class="reader-sub">{{ authorLine(current) }}</span>
                        </div>
                        <div class="reader-actions">
                            <template v-if="formatOf(current) === 'epub' && rendition">
                                <el-button
                                    size="small"
                                    circle
                                    :icon="Minus"
                                    title="减小字号"
                                    @click="changeFontSize(-10)"
                                />
                                <span class="font-percent">{{ fontSize }}%</span>
                                <el-button
                                    size="small"
                                    circle
                                    :icon="Plus"
                                    title="增大字号"
                                    @click="changeFontSize(10)"
                                />
                            </template>
                            <el-button
                                type="primary"
                                :icon="Download"
                                :loading="downloading"
                                @click="downloadBook(current)"
                                >{{ downloading ? '下载中…' : '下载电子书' }}</el-button
                            >
                        </div>
                    </div>

                    <div
                        v-loading="previewLoading"
                        class="reader-stage"
                        element-loading-text="正在加载内容..."
                    >
                        <!-- PDF：Chromium 内建阅读器（缩放/翻页由浏览器提供，与试卷预览一致） -->
                        <iframe
                            v-if="formatOf(current) === 'pdf' && !previewLoading"
                            :src="previewUrl"
                            class="reader-iframe"
                            frameborder="0"
                        ></iframe>

                        <!-- EPUB：epub.js 渲染 -->
                        <template v-else-if="formatOf(current) === 'epub' && !previewLoading">
                            <div ref="epubContainer" class="epub-container"></div>
                            <div class="epub-nav">
                                <el-button
                                    :icon="ArrowLeft"
                                    circle
                                    title="上一页"
                                    @click="prevPage"
                                />
                                <el-button
                                    :icon="ArrowRight"
                                    circle
                                    title="下一页"
                                    @click="nextPage"
                                />
                            </div>
                        </template>

                        <!-- 其他格式：不在线预览 -->
                        <div v-else-if="!previewLoading" class="reader-placeholder">
                            <svg-icon icon-class="education" class="placeholder-icon" />
                            <div class="placeholder-title">
                                {{ formatOf(current) }} 格式暂不支持在线预览
                            </div>
                            <div class="placeholder-desc">下载后使用本地阅读器打开即可</div>
                        </div>
                    </div>
                </template>

                <div v-else class="reader-placeholder reader-empty">
                    <img
                        class="empty-img"
                        src="@/assets/images/ebook.png"
                        alt=""
                        draggable="false"
                    />
                    <div class="empty-title">选择一本书开始阅读</div>
                    <div class="empty-desc">左侧书架点击书名即可在线预览或下载</div>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, nextTick, onBeforeUnmount, watch } from 'vue'
import { ElMessage } from 'element-plus'
import {
    Search,
    Refresh,
    Download,
    ArrowLeft,
    ArrowRight,
    Plus,
    Minus
} from '@element-plus/icons-vue'
import { listEbook, fetchEbookContent, ebookPreviewUrl, ebookDownloadUrl } from '@/api/datum/ebook'
import { normalizeFileUrl } from '@/utils/url'
import { getToken } from '@/utils/auth'

defineOptions({ name: 'Ebook' })

const books = ref([])
const total = ref(0)
const pageNum = ref(1)
const pageSize = 20
const keyword = ref('')
const loading = ref(false)
const current = ref(null)

const previewLoading = ref(false)
const previewUrl = ref('')
const epubContainer = ref(null)

const downloading = ref(false)

// epub.js 运行时句柄（动态 import，避免打进首屏包）
let epubBook = null
let rendition = null
let loadSeq = 0 // 快速切书防串台
const fontSize = ref(100)

const formatOf = (book) =>
    String(book?.ebookFormat || '')
        .toLowerCase()
        .trim() || 'unknown'
const authorLine = (book) => [book?.author, book?.publisher].filter(Boolean).join(' · ') || '佚名'

function formatSize(bytes) {
    const value = Number(bytes) || 0
    if (value >= 1024 * 1024) return (value / 1024 / 1024).toFixed(1) + ' MB'
    if (value >= 1024) return Math.round(value / 1024) + ' KB'
    return value + ' B'
}

/** 拉取书架列表 */
async function loadBooks(page = 1) {
    loading.value = true
    try {
        const response = await listEbook({
            pageNum: page,
            pageSize,
            keyword: keyword.value.trim()
        })
        books.value = response.rows || []
        total.value = Number(response.total) || 0
        pageNum.value = page
        // 翻页/搜索后当前书不在本页时清空阅读器
        if (current.value && !books.value.some((b) => b.ebookId === current.value.ebookId)) {
            closeBook()
        }
    } catch (error) {
        console.error('[ebook] 列表加载失败:', error)
    } finally {
        loading.value = false
    }
}

function handleSearch() {
    loadBooks(1)
}

/** 选中一本书：按格式分流预览 */
async function selectBook(book) {
    if (current.value && current.value.ebookId === book.ebookId) return
    await teardownEpub()
    current.value = book
    const format = formatOf(book)
    if (format === 'pdf') {
        previewUrl.value = ebookPreviewUrl(book.ebookId)
    } else if (format === 'epub') {
        await nextTick()
        openEpub(book)
    }
}

async function closeBook() {
    await teardownEpub()
    current.value = null
    previewUrl.value = ''
}

/** EPUB 渲染：内容经鉴权接口取 ArrayBuffer 后交给 epub.js */
async function openEpub(book) {
    const seq = ++loadSeq
    previewLoading.value = true
    try {
        const buffer = await fetchEbookContent(book.ebookId)
        if (seq !== loadSeq || !epubContainer.value) return
        const ePub = (await import('epubjs')).default
        epubBook = ePub(buffer)
        rendition = epubBook.renderTo(epubContainer.value, {
            width: '100%',
            height: '100%',
            spread: 'none'
        })
        rendition.themes.fontSize(`${fontSize.value}%`)
        await rendition.display()
    } catch (error) {
        console.error('[ebook] EPUB 加载失败:', error)
        ElMessage.error('EPUB 加载失败，可下载后本地阅读')
    } finally {
        if (seq === loadSeq) previewLoading.value = false
    }
}

async function teardownEpub() {
    loadSeq++
    if (rendition) {
        try {
            await rendition.destroy()
        } catch (_) {
            /* 已销毁则忽略 */
        }
        rendition = null
    }
    if (epubBook) {
        try {
            epubBook.destroy()
        } catch (_) {
            /* 已销毁则忽略 */
        }
        epubBook = null
    }
}

async function prevPage() {
    if (rendition) await rendition.prev()
}
async function nextPage() {
    if (rendition) await rendition.next()
}

function changeFontSize(delta) {
    const next = Math.min(200, Math.max(60, fontSize.value + delta))
    fontSize.value = next
    if (rendition) rendition.themes.fontSize(`${next}%`)
}

/** 下载：主进程流式写盘（断点续传/保存框与试卷下载同链路；完成由
 *  downloadFileDirectly 的 Promise resolve 回报，不订阅 IPC 进度事件） */
async function downloadBook(book) {
    if (downloading.value || !window.electronAPI?.downloadFileDirectly) {
        if (!window.electronAPI?.downloadFileDirectly) ElMessage.warning('当前环境不支持下载')
        return
    }
    const format = formatOf(book)
    const fileName = book.ebookName + (format !== 'unknown' ? `.${format}` : '')
    downloading.value = true
    try {
        const result = await window.electronAPI.downloadFileDirectly({
            url: ebookDownloadUrl(book.ebookId),
            fileName,
            token: getToken()
        })
        if (result && result.success) {
            ElMessage.success(`下载完成: ${fileName}`)
        } else if (!result || result.reason !== 'canceled') {
            ElMessage.error(`下载失败: ${(result && result.message) || '未知错误'}`)
        }
    } catch (error) {
        console.error('[ebook] 下载出错:', error)
        ElMessage.error(`下载失败：${error?.message || '请检查网络或联系管理员'}`)
    } finally {
        downloading.value = false
    }
}

// epub 分页键盘快捷键（左右方向键）
const onKeydown = (e) => {
    if (!rendition) return
    if (e.key === 'ArrowLeft') prevPage()
    if (e.key === 'ArrowRight') nextPage()
}
window.addEventListener('keydown', onKeydown)

// 视图切换/组件卸载时释放 epub 资源与全局监听
onBeforeUnmount(async () => {
    window.removeEventListener('keydown', onKeydown)
    await teardownEpub()
})

// 首次进入自动选中第一本书
watch(
    () => books.value,
    (rows) => {
        if (!current.value && rows && rows.length > 0) selectBook(rows[0])
    }
)

loadBooks(1)
</script>

<style scoped lang="scss">
/* 与 RankView 一致：作为首页 IDE 视图嵌入时填满内容区 */
.ebook-page {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
    overflow: hidden;
    background: var(--ide-panel-bg, #fff);
    border: 1px solid var(--ide-border, #ebeef5);
    border-radius: 12px;
    padding: 16px;
}
.ebook-hero {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 16px;
    border-radius: 12px;
    border: 1px solid rgba(var(--ide-accent-rgb, 64, 158, 255), 0.18);
    background: linear-gradient(
        135deg,
        rgba(var(--ide-accent-rgb, 64, 158, 255), 0.14),
        rgba(var(--ide-accent-rgb, 64, 158, 255), 0.04)
    );
}
.ebook-hero-icon {
    font-size: 22px;
    color: var(--ide-accent, #409eff);
}
.ebook-hero-text h3 {
    margin: 0;
    font-size: 18px;
    letter-spacing: 0.3px;
    color: var(--ide-text-active, #303133);
}
.ebook-hero-text p {
    margin: 2px 0 0;
    color: var(--ide-text-light, #909399);
    font-size: 13px;
}
.ebook-search {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 8px;
    :deep(.el-input) {
        width: 260px;
    }
}

.ebook-body {
    flex: 1;
    min-height: 0;
    display: flex;
    gap: 12px;
}

/* 左侧书架 */
.ebook-shelf {
    width: 320px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    min-height: 0;
    border: 1px solid var(--ide-border, #ebeef5);
    border-radius: 12px;
    background: color-mix(in srgb, var(--ide-editor-bg, #fff) 94%, transparent);
    overflow: hidden;
}
.ebook-list {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 8px;
}
.ebook-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 10px;
    border-radius: 10px;
    cursor: pointer;
    transition: background 0.15s ease;
    &:hover {
        background: rgba(var(--ide-accent-rgb, 64, 158, 255), 0.08);
        .ebook-download-icon {
            opacity: 1;
        }
    }
    &.active {
        background: rgba(var(--ide-accent-rgb, 64, 158, 255), 0.14);
        .ebook-title {
            color: var(--ide-accent, #409eff);
        }
    }
}
.ebook-cover {
    width: 38px;
    height: 50px;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 6px;
    overflow: hidden;
    background: rgba(var(--ide-accent-rgb, 64, 158, 255), 0.1);
    img {
        width: 100%;
        height: 100%;
        object-fit: cover;
    }
    .cover-fallback {
        font-size: 18px;
        color: var(--ide-accent, #409eff);
        opacity: 0.8;
    }
}
.ebook-meta {
    flex: 1;
    min-width: 0;
}
.ebook-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--ide-text-active, #303133);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
.ebook-sub {
    margin-top: 2px;
    font-size: 12px;
    color: var(--ide-text-light, #909399);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
.ebook-tags {
    margin-top: 4px;
    display: flex;
    align-items: center;
    gap: 6px;
}
.ebook-format {
    padding: 0 6px;
    border-radius: 4px;
    font-size: 11px;
    line-height: 18px;
    font-weight: 600;
    text-transform: uppercase;
    color: #fff;
    background: var(--ide-accent, #409eff);
    &.fmt-epub {
        background: #16a34a;
    }
    &.fmt-mobi,
    &.fmt-azw3 {
        background: #9333ea;
    }
}
.ebook-size {
    font-size: 11px;
    color: var(--ide-text-light, #909399);
}
.ebook-download-icon {
    flex-shrink: 0;
    font-size: 16px;
    color: var(--ide-text, #606266);
    opacity: 0;
    transition:
        opacity 0.15s ease,
        color 0.15s ease;
    &:hover {
        color: var(--ide-accent, #409eff);
    }
}
.ebook-pager {
    flex-shrink: 0;
    padding: 8px;
    justify-content: center;
}

/* 右侧阅读器 */
.ebook-reader {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    border: 1px solid var(--ide-border, #ebeef5);
    border-radius: 12px;
    background: color-mix(in srgb, var(--ide-editor-bg, #fff) 96%, transparent);
    overflow: hidden;
}
.reader-header {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 16px;
    border-bottom: 1px solid var(--ide-border, #ebeef5);
}
.reader-info {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: baseline;
    gap: 10px;
}
.reader-title {
    font-size: 15px;
    font-weight: 600;
    color: var(--ide-text-active, #303133);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
.reader-sub {
    font-size: 12px;
    color: var(--ide-text-light, #909399);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
.reader-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    .font-percent {
        min-width: 42px;
        text-align: center;
        font-size: 12px;
        color: var(--ide-text, #606266);
    }
}
.reader-stage {
    flex: 1;
    min-height: 0;
    position: relative;
    display: flex;
}
.reader-iframe {
    flex: 1;
    width: 100%;
    height: 100%;
    border: 0;
}
.epub-container {
    flex: 1;
    min-width: 0;
    min-height: 0;
}
.epub-nav {
    position: absolute;
    bottom: 16px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    gap: 10px;
    padding: 6px 10px;
    border-radius: 999px;
    border: 1px solid var(--ide-border, #ebeef5);
    background: rgba(255, 255, 255, 0.75);
    backdrop-filter: blur(6px);
}

/* 占位与空态 */
.reader-placeholder {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    color: var(--ide-text-light, #909399);
}
.reader-empty {
    padding-bottom: 40px;
}
.placeholder-icon {
    font-size: 44px;
    opacity: 0.5;
    margin-bottom: 8px;
}
.placeholder-title {
    font-size: 15px;
    font-weight: 600;
    color: var(--ide-text-active, #303133);
}
.placeholder-desc {
    font-size: 13px;
}
.empty-state,
.ebook-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 40px 0;
    color: var(--ide-text-light, #909399);
}
.empty-img {
    width: 56px;
    height: 56px;
    object-fit: contain;
    margin-bottom: 8px;
    opacity: 0.75;
    user-select: none;
}
.empty-title {
    font-size: 16px;
    font-weight: 700;
    color: var(--ide-text-active, #303133);
}
.empty-desc {
    font-size: 13px;
}
</style>
