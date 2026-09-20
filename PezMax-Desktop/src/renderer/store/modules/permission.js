import { constantRoutes } from '@/router'

// 桌面端页面全部由 router/index.js 的 constantRoutes 静态声明，
// 原 RuoYi 动态路由链（/getRouters + filterAsyncRouter）已随旧后端契约移除。
// 保留 Sidebar/TagsView/Breadcrumb/TopNav/Settings 消费的状态字段，
// 取值即 constantRoutes，setSidebarRouters 仅供布局设置回退默认菜单。
const usePermissionStore = defineStore(
  'permission',
  {
    state: () => ({
      routes: constantRoutes,
      addRoutes: [],
      defaultRoutes: constantRoutes,
      topbarRouters: constantRoutes,
      sidebarRouters: constantRoutes
    }),
    actions: {
      setSidebarRouters(routes) {
        this.sidebarRouters = routes
      }
    }
  })

export default usePermissionStore
