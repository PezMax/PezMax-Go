import type { RouteRecordNormalized } from 'vue-router';

import type { MenuRecordRaw } from '@vben/types';

import { useRouter } from 'vue-router';

import { useAccessStore } from '@vben/stores';
import { isHttpUrl, openRouteInNewWindow, openWindow } from '@vben/utils';

interface ExternalNavigationDetails {
  openInNewWindow: boolean;
  title?: string;
  url: string;
}

type ExternalNavigationGuard = (
  details: ExternalNavigationDetails,
) => boolean | Promise<boolean>;

let externalNavigationGuard: ExternalNavigationGuard | undefined;

function setExternalNavigationGuard(guard?: ExternalNavigationGuard) {
  externalNavigationGuard = guard;
  return () => {
    if (externalNavigationGuard === guard) externalNavigationGuard = undefined;
  };
}

async function confirmExternalNavigation(
  route: MenuRecordRaw | RouteRecordNormalized | undefined,
  url: string,
) {
  const isRoute = route && 'meta' in route;
  const confirmExternal = isRoute
    ? route.meta.confirmExternal
    : route?.confirmExternal;
  if (confirmExternal !== true) return true;
  const routeTitle = isRoute ? route.meta.title : route?.name;
  const title = typeof routeTitle === 'string' ? routeTitle : undefined;
  if (externalNavigationGuard) {
    return Boolean(
      await externalNavigationGuard({
        openInNewWindow: externalNavigationOpensNewWindow(route),
        title,
        url,
      }),
    );
  }
  return window.confirm(`即将打开外部链接：\n${url}\n\n是否继续？`);
}

function externalNavigationOpensNewWindow(
  route: MenuRecordRaw | RouteRecordNormalized | undefined,
) {
  if (!route) return true;
  return 'meta' in route
    ? route.meta.openInNewWindow !== false
    : route.openInNewWindow !== false;
}

function useNavigation() {
  const router = useRouter();
  const accessStore = useAccessStore();
  const routeMetaMap = new Map<string, RouteRecordNormalized>();

  // 初始化路由映射
  const initRouteMetaMap = () => {
    const routes = router.getRoutes();
    routes.forEach((route) => {
      routeMetaMap.set(route.path, route);
    });
  };

  initRouteMetaMap();

  // 监听路由变化
  router.afterEach(() => {
    initRouteMetaMap();
  });

  // 检查是否应该在新窗口打开
  const shouldOpenInNewWindow = (path: string): boolean => {
    if (isHttpUrl(path)) {
      return externalNavigationOpensNewWindow(accessStore.getMenuByPath(path));
    }
    const route = routeMetaMap.get(path);
    if (route?.meta?.link) {
      return externalNavigationOpensNewWindow(route);
    }
    return route?.meta?.openInNewWindow === true;
  };

  const resolveHref = (path: string): string => {
    return router.resolve(path).href;
  };

  const navigation = async (path: string) => {
    try {
      const route = routeMetaMap.get(path);
      const menu = accessStore.getMenuByPath(path);
      const { openInNewWindow = false, query = {}, link } = route?.meta ?? {};

      // 检查是否有外链
      if (link && typeof link === 'string') {
        if (!(await confirmExternalNavigation(route ?? menu, link))) return;
        openWindow(link, {
          target: externalNavigationOpensNewWindow(route ?? menu)
            ? '_blank'
            : '_self',
        });
        return;
      }

      if (isHttpUrl(path)) {
        if (!(await confirmExternalNavigation(menu, path))) return;
        openWindow(path, {
          target: externalNavigationOpensNewWindow(menu) ? '_blank' : '_self',
        });
      } else if (openInNewWindow) {
        openRouteInNewWindow(resolveHref(path));
      } else {
        await router.push({
          path,
          query,
        });
      }
    } catch (error) {
      console.error('Navigation failed:', error);
      throw error;
    }
  };

  const willOpenedByWindow = (path: string) => {
    return shouldOpenInNewWindow(path);
  };

  return { navigation, willOpenedByWindow };
}

export {
  confirmExternalNavigation,
  externalNavigationOpensNewWindow,
  setExternalNavigationGuard,
  useNavigation,
};
export type { ExternalNavigationDetails, ExternalNavigationGuard };
