import request from '@/utils/request'

// 通知铃铛（layout/components/HeaderNotice）专用公告接口。
// 注意：以下端点仍指向旧 RuoYi 后端 /system/notice/*，属于迁移待办——
// k_admin 已有站内通知模块（/api/notifications），datum 通知落地后应整体切换并删除本文件。

// 首页顶部公告列表（带已读状态）
export function listNoticeTop() {
  return request({
    url: '/system/notice/listTop',
    method: 'get'
  })
}

// 查询公告详细
export function getNotice(noticeId) {
  return request({
    url: '/system/notice/' + noticeId,
    method: 'get'
  })
}

// 标记公告已读
export function markNoticeRead(noticeId) {
  return request({
    url: '/system/notice/markRead',
    method: 'post',
    params: { noticeId }
  })
}

// 批量标记已读
export function markNoticeReadAll(ids) {
  return request({
    url: '/system/notice/markReadAll',
    method: 'post',
    params: { ids }
  })
}
