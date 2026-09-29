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

// 电子书学科聚合（学科 → 书 树的第一层）
export function listEbookSubjects() {
    return request({
        url: '/datum/ebook/subjects',
        method: 'get'
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

// 在线预览地址（inline 流；iframe/epub.js 直连场景用 query token）
export function ebookPreviewUrl(ebookId) {
    const base = import.meta.env.VITE_APP_BASE_API
    return `${base}/datum/ebook/content?ebookId=${ebookId}&token=${encodeURIComponent(getToken() || '')}`
}

// 下载地址（attachment 流；主进程 downloadFileDirectly 用 Bearer 头携带 token）
export function ebookDownloadUrl(ebookId) {
    const base = import.meta.env.VITE_APP_BASE_API
    return `${base}/datum/download/ebook?ebookId=${ebookId}`
}

// 查询当前用户收藏的电子书 id 集合
export function listEbookFavorite() {
    return request({
        url: '/datum/ebook/favorite/list',
        method: 'get'
    })
}

// 查询单本电子书收藏状态
export function getEbookFavoriteStatus(ebookId) {
    return request({
        url: '/datum/ebook/favorite',
        method: 'get',
        params: { ebookId }
    })
}

// 收藏电子书
export function addEbookFavorite(data) {
    return request({
        url: '/datum/ebook/favorite',
        method: 'post',
        data: data
    })
}

// 取消收藏电子书
export function delEbookFavorite(ebookId) {
    return request({
        url: '/datum/ebook/favorite/' + ebookId,
        method: 'delete'
    })
}

// 举报电子书
export function addEbookReport(data) {
    return request({
        url: '/datum/ebook/report',
        method: 'post',
        data: data
    })
}
