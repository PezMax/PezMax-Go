import request from '@/utils/request'

// 查询通知列表
export function listNotification(query) {
    return request({
        url: '/datum/notification/list',
        method: 'get',
        params: query
    })
}

// 查询通知详细
export function getNotification(notifyId) {
    return request({
        url: '/datum/notification/' + notifyId,
        method: 'get'
    })
}

// 新增通知
export function addNotification(data) {
    return request({
        url: '/datum/notification',
        method: 'post',
        data: data
    })
}

// 修改通知
export function updateNotification(data) {
    return request({
        url: '/datum/notification',
        method: 'put',
        data: data
    })
}

// 删除通知
export function delNotification(notifyId) {
    return request({
        url: '/datum/notification/' + notifyId,
        method: 'delete'
    })
}
/**
 * lxq
 *获取用户端需要以弹窗形式展示的通知列表
 * extraParams 可携带 { hash } 参与 hash 轮询协议：内容未变时后端返回
 * unchanged=true 且不带 data（与文件树同款协议，见后端 respondNotificationFeed）
 */
export function getUserPopupNotifications(userId, extraParams) {
  return request({
    url: '/system/notification/user/popup',
    method: 'get',
    params: { userId, ...(extraParams || {}) }
  })
}
/**
 * lxq
 *获取用户端需要以滚动形式展示的通知列表
 * params 可携带 { hash } 参与 hash 轮询协议（见 getUserPopupNotifications）
 */
export function getUserScrollNotifications(params){
  return request({
    url:'/system/notification/user/scroll',
    method:'get',
    params: params || {}
  })
}
