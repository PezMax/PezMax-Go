import router from './router'
import { ElMessage } from 'element-plus'
import NProgress from 'nprogress'
import 'nprogress/nprogress.css'

import { isPathMatch } from '@/utils/validate'
import { isRelogin } from '@/utils/request'
import useUserStore from '@/store/modules/user'
import useLockStore from '@/store/modules/lock'
import useSettingsStore from '@/store/modules/settings'
import { PTMJ_AUTH_ROUTES, PTMJ_AUTH_ROUTE_LIST,isPtmjAuthRoute} from '@/constants/ptmjAuth'
import { ElMessageBox } from 'element-plus'
import { getToken, removeToken } from '@/utils/auth'


NProgress.configure({ showSpinner: false })

const whiteList = PTMJ_AUTH_ROUTE_LIST

const isWhiteList = (path) => {
  return whiteList.some(pattern => isPathMatch(pattern, path))
}
//LYZ四次修改：增加对认证页路由的判断函数
let lastWindowMode = null
const syncWindowModeByRoute = (path) => {
  const mode = isPtmjAuthRoute(path) ? 'auth' : 'main'
  // 仅在窗口模式真正发生变化时才通知主进程，避免每次路由切换都触发窗口居中
  if (mode !== lastWindowMode) {
    lastWindowMode = mode
    window.electronAPI?.setWindowMode?.(mode)
  }
}

router.beforeEach((to, from, next) => {
  syncWindowModeByRoute(to.path)//LYZ四次修改：同步主窗口和认证窗口的显示模式
  NProgress.start()
  if (getToken()) {
    to.meta.title && useSettingsStore().setTitle(to.meta.title)
    const isLock = useLockStore().isLock
    /* has token*/
    if (to.path === PTMJ_AUTH_ROUTES.login) {
      next({ path: '/' })
      NProgress.done()
    } else if (isWhiteList(to.path)) {
      next()
    } else if (isLock && to.path !== '/lock') {
      next({ path: '/lock' })
      NProgress.done()
    } else if (!isLock && to.path === '/lock') {
      next({ path: '/' })
      NProgress.done()
    } else {
      if (useUserStore().roles.length === 0) {
        isRelogin.show = true
        // 判断当前用户是否已拉取完user_info信息
        useUserStore().getInfo().then((res) => {
          //LYZ四次修改：检查是否被封号
          const payload = res?.data || res
          const status = payload?.user?.status

          if (String(status) === '0') {
            removeToken()
            useUserStore().logOut()
            ElMessageBox.alert('账号已被封禁，无法登录', '提示', { type: 'warning' })
            next({ path: PTMJ_AUTH_ROUTES.login, replace: true })
            NProgress.done()
            return
          }
          isRelogin.show = false
          // 桌面端路由已全部静态声明（router/index.js constantRoutes），
          // 拉取完用户信息后直接放行，不再向后端请求 /getRouters 动态路由
          next({ ...to, replace: true })
        }).catch(err => {
          useUserStore().logOut().then(() => {
            removeToken()//LYZ四次修改：被封号直接删除token
            ElMessage.error(err)
            next({ path: '/' })
          })
        })
      } else {
        next()
      }
    }
  } else {
    // 没有token
    if (isWhiteList(to.path)) {
      // 在免登录白名单，直接进入
      next()
    } else {
      next(`${PTMJ_AUTH_ROUTES.login}?redirect=${to.fullPath}`) // 否则全部重定向到登录页
      NProgress.done()
    }
  }
})

router.afterEach(() => {
  NProgress.done()
})
