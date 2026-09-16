import type { Component, DefineComponent } from 'vue';

import type {
  AccessModeType,
  GenerateMenuAndRoutesOptions,
  RouteRecordRaw,
} from '@vben/types';

import { defineComponent, h } from 'vue';

import {
  cloneDeep,
  generateMenus,
  generateRoutesByBackend,
  generateRoutesByFrontend,
  isFunction,
  isString,
  mapTree,
} from '@vben/utils';

type AccessRouter = GenerateMenuAndRoutesOptions['router'];

const rootBaseChildren = new WeakMap<AccessRouter, RouteRecordRaw[]>();
const standaloneRouteRemovers = new WeakMap<AccessRouter, Array<() => void>>();

async function generateAccessible(
  mode: AccessModeType,
  options: GenerateMenuAndRoutesOptions,
) {
  const { router } = options;

  options.routes = cloneDeep(options.routes);

  // 生成路由
  const accessibleRoutes = await generateRoutes(mode, options);

  installAccessibleRoutes(router, accessibleRoutes);

  // 生成菜单
  const accessibleMenus = generateMenus(accessibleRoutes, options.router);

  return { accessibleMenus, accessibleRoutes };
}

function installAccessibleRoutes(
  router: AccessRouter,
  accessibleRoutes: RouteRecordRaw[],
) {
  const root = router.getRoutes().find((item) => item.path === '/');
  const rootRoutes: RouteRecordRaw[] = [];
  const standaloneRoutes: RouteRecordRaw[] = [];

  for (const route of accessibleRoutes) {
    const clone = { ...route } as RouteRecordRaw;
    if (root && !route.meta?.noBasicLayout) {
      // Root 已提供布局容器，目录节点不能再次挂载自己的布局组件。
      if (clone.children?.length) {
        delete clone.component;
      }
      rootRoutes.push(clone);
    } else {
      standaloneRoutes.push(clone);
    }
  }

  for (const removeRoute of standaloneRouteRemovers.get(router) ?? []) {
    removeRoute();
  }

  if (root) {
    let baseChildren = rootBaseChildren.get(router);
    if (!baseChildren) {
      baseChildren = (root.children ?? []).map(
        (route) => ({ ...route }) as RouteRecordRaw,
      );
      rootBaseChildren.set(router, baseChildren);
    }
    root.children = mergeRootChildren(baseChildren, rootRoutes);
    if (root.name) {
      router.removeRoute(root.name);
    }
    router.addRoute(root);
  }

  standaloneRouteRemovers.set(
    router,
    standaloneRoutes.map((route) => router.addRoute(route)),
  );
}

function mergeRootChildren(
  baseChildren: RouteRecordRaw[],
  accessChildren: RouteRecordRaw[],
): RouteRecordRaw[] {
  const result: RouteRecordRaw[] = [];
  const indexes = new Map<string, number>();

  for (const route of [...baseChildren, ...accessChildren]) {
    const key = route.name
      ? `name:${String(route.name)}`
      : `path:${route.path}`;
    const index = indexes.get(key);
    if (index === undefined) {
      indexes.set(key, result.length);
      result.push(route);
    } else {
      result[index] = route;
    }
  }
  return result;
}

/**
 * Generate routes
 * @param mode
 * @param options
 */
async function generateRoutes(
  mode: AccessModeType,
  options: GenerateMenuAndRoutesOptions,
) {
  const { forbiddenComponent, roles, routes } = options;

  let resultRoutes: RouteRecordRaw[] = routes;
  switch (mode) {
    case 'backend': {
      resultRoutes = await generateRoutesByBackend(options);
      break;
    }
    case 'frontend': {
      resultRoutes = await generateRoutesByFrontend(
        routes,
        roles || [],
        forbiddenComponent,
      );
      break;
    }
    case 'mixed': {
      const [frontend_resultRoutes, backend_resultRoutes] = await Promise.all([
        generateRoutesByFrontend(routes, roles || [], forbiddenComponent),
        generateRoutesByBackend(options),
      ]);
      resultRoutes = mergeRoutesByName(
        backend_resultRoutes,
        frontend_resultRoutes,
      );
      break;
    }
  }

  /**
   * 调整路由树，做以下处理：
   * 1. 对未添加redirect的路由添加redirect
   * 2. 将懒加载的组件名称修改为当前路由的名称（如果启用了keep-alive的话）
   */
  resultRoutes = mapTree(resultRoutes, (route, parent) => {
    // 重新包装component，使用与路由名称相同的name以支持keep-alive的条件缓存。
    if (
      route.meta?.keepAlive &&
      isFunction(route.component) &&
      route.name &&
      isString(route.name)
    ) {
      const originalComponent = route.component as () => Promise<{
        default: Component | DefineComponent;
      }>;
      route.component = async () => {
        const component = await originalComponent();
        if (!component.default) return component;
        return defineComponent({
          name: route.name as string,
          setup(props, { attrs, slots }) {
            return () => h(component.default, { ...props, ...attrs }, slots);
          },
        });
      };
    }

    // 如果有redirect或者没有子路由，则直接返回
    if (route.redirect || !route.children || route.children.length === 0) {
      return route;
    }
    const firstChild = route.children[0];

    if (!firstChild?.path || firstChild.path.startsWith('/')) {
      return route;
    }

    if (parent && parent.redirect) {
      const parentSplit = (parent.redirect as string).split('/');
      parentSplit.splice(-1, 2, route.path, firstChild.path);
      const redirectPath = parentSplit.join('/');
      route.redirect = redirectPath;
    } else {
      route.redirect = `${route.path}/${firstChild.path}`;
    }

    return route;
  });

  return resultRoutes;
}

/**
 * 根据 name 合并前后端路由
 * @param baseRoutes 后端路由
 * @param extraRoutes 前端路由
 */
function mergeRoutesByName(
  baseRoutes: RouteRecordRaw[],
  extraRoutes: RouteRecordRaw[],
): RouteRecordRaw[] {
  const backendRouteNames = collectRouteNames(baseRoutes);
  const frontendRouteMap = collectRoutesByName(extraRoutes);

  return mergeRouteLevel(
    baseRoutes,
    extraRoutes,
    backendRouteNames,
    frontendRouteMap,
  );
}

function mergeRouteLevel(
  backendRoutes: RouteRecordRaw[],
  frontendRoutes: RouteRecordRaw[],
  backendRouteNames: Set<string>,
  frontendRouteMap: Map<string, RouteRecordRaw>,
): RouteRecordRaw[] {
  const result = backendRoutes.map((backendRoute) => {
    const routeName = getRouteName(backendRoute);
    const frontendRoute = routeName
      ? frontendRouteMap.get(routeName)
      : undefined;
    const backendChildren = backendRoute.children ?? [];
    const frontendChildren = frontendRoute?.children ?? [];
    const merged = {
      ...frontendRoute,
      ...backendRoute,
      meta: {
        ...frontendRoute?.meta,
        ...backendRoute.meta,
      },
    } as RouteRecordRaw;

    // 后端菜单一旦拥有子级就表示目录。静态页面组件通常不包含 RouterView，
    // 若继续保留组件，移动到它下面的子菜单会匹配但无法渲染。
    if (backendChildren.length > 0) {
      delete merged.component;
    }

    if (backendChildren.length > 0 || frontendChildren.length > 0) {
      merged.children = mergeRouteLevel(
        backendChildren,
        frontendChildren,
        backendRouteNames,
        frontendRouteMap,
      );
    }

    return merged;
  });

  for (const frontendRoute of frontendRoutes) {
    const routeName = getRouteName(frontendRoute);
    // 后端路由可能已被移动到其他父级，不能按前端静态树补回旧位置。
    if (routeName && backendRouteNames.has(routeName)) {
      continue;
    }

    const clone = { ...frontendRoute } as RouteRecordRaw;
    clone.meta = {
      ...frontendRoute.meta,
      hideInMenu: true,
      title: frontendRoute.meta?.title ?? routeName ?? '',
    };
    if (frontendRoute.children?.length) {
      clone.children = mergeRouteLevel(
        [],
        frontendRoute.children,
        backendRouteNames,
        frontendRouteMap,
      );
    }
    result.push(clone);
  }

  return result;
}

function collectRouteNames(
  routes: RouteRecordRaw[],
  names = new Set<string>(),
): Set<string> {
  for (const route of routes) {
    const name = getRouteName(route);
    if (name) {
      names.add(name);
    }
    if (route.children?.length) {
      collectRouteNames(route.children, names);
    }
  }
  return names;
}

function collectRoutesByName(
  routes: RouteRecordRaw[],
  routeMap = new Map<string, RouteRecordRaw>(),
): Map<string, RouteRecordRaw> {
  for (const route of routes) {
    const name = getRouteName(route);
    if (name) {
      routeMap.set(name, route);
    }
    if (route.children?.length) {
      collectRoutesByName(route.children, routeMap);
    }
  }
  return routeMap;
}

function getRouteName(route: RouteRecordRaw): string | undefined {
  return isString(route.name) ? route.name : undefined;
}

export {
  generateAccessible,
  installAccessibleRoutes,
  mergeRootChildren,
  mergeRoutesByName,
};
