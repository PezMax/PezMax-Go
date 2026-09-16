import type {
  ComponentRecordType,
  GenerateMenuAndRoutesOptions,
} from '@vben/types';

import { generateAccessible } from '@vben/access';
import { useAccessStore } from '@vben/stores';

import { message } from 'ant-design-vue';

import { getAllMenusApi } from '#/api';
import { BasicLayout, IFrameView } from '#/layouts';
import { $t } from '#/locales';
import { KADMIN_ACCESS_MODE } from '#/preferences';

const forbiddenComponent = () => import('#/views/_core/fallback/forbidden.vue');
type GenerateAccessOptions = Pick<
  GenerateMenuAndRoutesOptions,
  'roles' | 'router'
>;

async function generateAccess(options: GenerateAccessOptions) {
  const pageMap: ComponentRecordType = import.meta.glob('../views/**/*.vue');

  const layoutMap: ComponentRecordType = {
    BasicLayout,
    IFrameView,
  };

  return await generateAccessible(KADMIN_ACCESS_MODE, {
    ...options,
    fetchMenuListAsync: async () => {
      message.loading({
        content: `${$t('common.loadingMenu')}...`,
        duration: 1.5,
      });
      return await getAllMenusApi();
    },
    // 可以指定没有权限跳转403页面
    forbiddenComponent,
    // 如果 route.meta.menuVisibleWithForbidden = true
    layoutMap,
    pageMap,
    // 服务端菜单树是业务路由的唯一来源，本地页面由 pageMap 解析组件。
    routes: [],
  });
}

async function refreshNavigation(options: GenerateAccessOptions) {
  const result = await generateAccess(options);
  const accessStore = useAccessStore();
  accessStore.setAccessMenus(result.accessibleMenus);
  accessStore.setAccessRoutes(result.accessibleRoutes);
  accessStore.setIsAccessChecked(true);
  return result;
}

export { generateAccess, refreshNavigation };
