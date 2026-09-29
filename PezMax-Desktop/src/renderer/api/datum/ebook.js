import request from '@/utils/request'
import { getToken } from '@/utils/auth'

// 查询电子书列表（匿名可读，仅已上架；keyword 匹配书名/作者/出版社）
export function listEbook(query) {
    return request({
        url: '/datum/ebook/list',
        method: 'get',
        params: query
    })
}

// 拉取电子书内容（ArrayBuffer，供 epub.js 渲染；datum 会话鉴权）
export function fetchEbookContent(ebookId) {
    return request({
        url: '/datum/ebook/content',
        method: 'get',
        params: { ebookId },
        responseType: 'arraybuffer'
    })
}

// 在线预览地址（inline 流；iframe 直连场景用 query token）
export function ebookPreviewUrl(ebookId) {
    const base = import.meta.env.VITE_APP_BASE_API
    return `${base}/datum/ebook/content?ebookId=${ebookId}&token=${encodeURIComponent(getToken() || '')}`
}

// 下载地址（attachment 流；主进程 downloadFileDirectly 用 Bearer 头携带 token）
export function ebookDownloadUrl(ebookId) {
    const base = import.meta.env.VITE_APP_BASE_API
    return `${base}/datum/download/ebook?ebookId=${ebookId}`
}
