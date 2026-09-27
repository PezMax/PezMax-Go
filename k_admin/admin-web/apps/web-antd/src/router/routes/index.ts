import type { RouteRecordRaw } from 'vue-router';

import { traverseTreeValues } from '@vben/utils';

import { $t } from '#/locales';

import { coreRoutes, fallbackNotFoundRoute } from './core';

/** 需要登录但不属于业务菜单的本地工具页。 */
const authenticatedUtilityRoutes: RouteRecordRaw[] = [
  {
    name: 'Forbidden',
    path: '/403',
    component: () => import('#/views/_core/fallback/forbidden.vue'),
    meta: {
      hideInBreadcrumb: true,
      hideInMenu: true,
      hideInTab: true,
      title: '403',
    },
  },
  {
    name: 'Profile',
    path: '/profile',
    component: () => import('#/views/_core/profile/index.vue'),
    meta: {
      hideInMenu: true,
      icon: 'lucide:user',
      title: $t('page.auth.profile'),
    },
  },
  {
    name: 'CodegenConfig',
    path: '/kadmin/codegen-config',
    component: () => import('#/views/kadmin/components/CodegenConfigView.vue'),
    meta: {
      hideInMenu: true,
      title: '生成配置',
    },
  },
  {
    name: 'NotificationCenter',
    path: '/kadmin/notifications',
    component: () => import('#/views/kadmin/components/NotificationCenterView.vue'),
    meta: {
      hideInMenu: true,
      title: '站内通知',
    },
  },
];

// 工具页随根布局静态注册，但不加入 coreRouteNames，仍会经过登录校验。
const initialRoutes = coreRoutes.map((route) =>
  route.path === '/'
    ? {
        ...route,
        children: [...(route.children ?? []), ...authenticatedUtilityRoutes],
      }
    : route,
);

/** 初始路由包含基础页、需登录的本地工具页和 404 兜底页。 */
const routes: RouteRecordRaw[] = [...initialRoutes, fallbackNotFoundRoute];

/** 基本路由列表，这些路由不需要进入权限拦截 */
const coreRouteNames = traverseTreeValues(coreRoutes, (route) => route.name);
export { coreRouteNames, routes };
