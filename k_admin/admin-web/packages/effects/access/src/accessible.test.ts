import type {
  GenerateMenuAndRoutesOptions,
  RouteRecordRaw,
  RouteRecordStringComponent,
} from '@vben/types';

import { describe, expect, it } from 'vitest';

import {
  generateAccessible,
  installAccessibleRoutes,
  mergeRootChildren,
  mergeRoutesByName,
} from './accessible';

type AccessRouter = GenerateMenuAndRoutesOptions['router'];

describe('mergeRoutesByName', () => {
  it('keeps a backend route under its updated parent', () => {
    const component = () => Promise.resolve({});
    const backendRoutes = [
      {
        name: 'Dashboard',
        path: '/dashboard',
        children: [
          {
            name: 'UserManagement',
            path: '/kadmin/users',
            meta: { title: '用户管理' },
          },
        ],
      },
      {
        name: 'KAdmin',
        path: '/kadmin',
        children: [],
      },
    ] as unknown as RouteRecordRaw[];
    const frontendRoutes = [
      {
        name: 'Dashboard',
        path: '/dashboard',
        children: [],
      },
      {
        name: 'KAdmin',
        path: '/kadmin',
        children: [
          {
            name: 'UserManagement',
            path: 'users',
            component,
            meta: { title: '静态标题' },
          },
        ],
      },
    ] as RouteRecordRaw[];

    const result = mergeRoutesByName(backendRoutes, frontendRoutes);
    const dashboardRoute = result.find((route) => route.name === 'Dashboard');
    const kadminRoute = result.find((route) => route.name === 'KAdmin');
    const movedRoute = dashboardRoute?.children?.find(
      (route) => route.name === 'UserManagement',
    );

    expect(movedRoute).toMatchObject({
      component,
      path: '/kadmin/users',
      meta: { title: '用户管理' },
    });
    expect(kadminRoute?.children).toEqual([]);
  });

  it('keeps backend order and hides frontend-only routes', () => {
    const component = () => Promise.resolve({});
    const backendRoutes = [
      {
        name: 'Second',
        path: '/second',
        meta: { order: 0, title: '后端标题' },
      },
      {
        name: 'First',
        path: '/first',
        meta: { order: 1 },
        children: [{ name: 'Moved', path: '/moved' }],
      },
    ] as RouteRecordRaw[];
    const frontendRoutes = [
      {
        name: 'First',
        path: '/first',
        children: [],
      },
      {
        name: 'Second',
        path: '/second-static',
        component,
        meta: { title: '静态标题' },
        children: [{ name: 'Moved', path: 'moved', component }],
      },
      { name: 'FrontendOnly', path: '/frontend-only', component },
    ] as RouteRecordRaw[];

    const result = mergeRoutesByName(backendRoutes, frontendRoutes);

    expect(result.map((route) => route.name)).toEqual([
      'Second',
      'First',
      'FrontendOnly',
    ]);
    expect(result[0]).toMatchObject({
      component,
      path: '/second',
      meta: { order: 0, title: '后端标题' },
    });
    expect(result[0]?.children).toEqual([]);
    expect(result[1]?.children?.[0]).toMatchObject({
      component,
      name: 'Moved',
      path: '/moved',
    });
    expect(result[2]).toMatchObject({
      name: 'FrontendOnly',
      meta: { hideInMenu: true },
    });
  });

  it('turns a static page into a componentless directory after nesting a menu under it', () => {
    const parentComponent = () => Promise.resolve({});
    const childComponent = () => Promise.resolve({});
    const backendRoutes = [
      {
        name: 'KAdmin',
        path: '/kadmin',
        children: [
          {
            name: 'Users',
            path: '/kadmin/users',
            children: [{ name: 'Rbac', path: '/kadmin/rbac' }],
          },
        ],
      },
    ] as RouteRecordRaw[];
    const frontendRoutes = [
      {
        name: 'KAdmin',
        path: '/kadmin',
        children: [
          {
            name: 'Users',
            path: 'users',
            component: parentComponent,
          },
          {
            name: 'Rbac',
            path: 'rbac',
            component: childComponent,
          },
        ],
      },
    ] as RouteRecordRaw[];

    const result = mergeRoutesByName(backendRoutes, frontendRoutes);
    const users = result[0]?.children?.[0];

    expect(users?.component).toBeUndefined();
    expect(users?.children?.[0]).toMatchObject({
      component: childComponent,
      name: 'Rbac',
      path: '/kadmin/rbac',
    });
  });
});

describe('mergeRootChildren', () => {
  it('replaces each access snapshot without retaining an old root parent', () => {
    const constants = [
      { name: 'Constant', path: '/constant' },
    ] as RouteRecordRaw[];
    const rootSnapshot = [
      { name: 'Moved', path: '/moved' },
    ] as RouteRecordRaw[];
    const nestedSnapshot = [
      {
        name: 'Parent',
        path: '/parent',
        children: [{ name: 'Moved', path: '/moved' }],
      },
    ] as RouteRecordRaw[];

    const firstResult = mergeRootChildren(constants, rootSnapshot);
    expect(firstResult.map((route) => route.name)).toEqual([
      'Constant',
      'Moved',
    ]);

    const result = mergeRootChildren(constants, nestedSnapshot);
    expect(result.map((route) => route.name)).toEqual(['Constant', 'Parent']);
    expect(result[1]?.children?.map((route) => route.name)).toEqual(['Moved']);
  });

  it('lets the latest access route replace a same-name base child', () => {
    const result = mergeRootChildren(
      [{ name: 'Shared', path: '/old' }] as RouteRecordRaw[],
      [{ name: 'Shared', path: '/new' }] as RouteRecordRaw[],
    );

    expect(result).toEqual([{ name: 'Shared', path: '/new' }]);
  });
});

describe('installAccessibleRoutes', () => {
  it('removes stale root routes across repeated layout changes', () => {
    const { getRoot, router } = createFakeRouter([
      { name: 'Constant', path: '/constant' },
    ]);
    const movedRoute = { name: 'Moved', path: '/moved' } as RouteRecordRaw;

    installAccessibleRoutes(router, [movedRoute]);
    expect(getRoot().children?.map((route) => route.name)).toEqual([
      'Constant',
      'Moved',
    ]);

    installAccessibleRoutes(router, [
      {
        name: 'Parent',
        path: '/parent',
        children: [movedRoute],
      },
    ] as RouteRecordRaw[]);
    expect(getRoot().children?.map((route) => route.name)).toEqual([
      'Constant',
      'Parent',
    ]);
    expect(
      getRoot().children?.[1]?.children?.map((route) => route.name),
    ).toEqual(['Moved']);

    installAccessibleRoutes(router, [movedRoute]);
    expect(getRoot().children?.map((route) => route.name)).toEqual([
      'Constant',
      'Moved',
    ]);
    expect(countRoutesByName(getRoot(), 'Moved')).toBe(1);
  });

  it('removes standalone routes missing from the next snapshot', () => {
    const { routes, router } = createFakeRouter();

    installAccessibleRoutes(router, [
      {
        meta: { noBasicLayout: true },
        name: 'Standalone',
        path: '/standalone',
      },
    ] as RouteRecordRaw[]);
    expect(routes.some((route) => route.name === 'Standalone')).toBe(true);

    installAccessibleRoutes(router, []);
    expect(routes.some((route) => route.name === 'Standalone')).toBe(false);
  });
});

describe('backend navigation snapshots', () => {
  it('replaces database order and parent relationships without static routes', async () => {
    const snapshots: RouteRecordStringComponent[][] = [
      [
        {
          meta: { order: 0, title: 'Dashboard' },
          name: 'Dashboard',
          path: '/dashboard',
          children: [
            {
              component: '/dashboard/analytics/index',
              meta: { order: 0, title: '分析页' },
              name: 'Analytics',
              path: '/dashboard/analytics',
            },
          ],
        },
        {
          meta: { order: 1, title: 'KAdmin' },
          name: 'KAdmin',
          path: '/kadmin',
          children: [
            {
              component: '/kadmin/components/MenuManagementView',
              meta: { order: 0, title: '菜单管理' },
              name: 'KAdminMenus',
              path: '/kadmin/menus',
            },
            {
              component: '/kadmin/components/UserManagementView',
              meta: { order: 1, title: '用户管理' },
              name: 'KAdminUsers',
              path: '/kadmin/users',
            },
          ],
        },
      ],
      [
        {
          meta: { order: 0, title: 'Dashboard' },
          name: 'Dashboard',
          path: '/dashboard',
          children: [
            {
              component: '/dashboard/analytics/index',
              meta: { order: 0, title: '分析页' },
              name: 'Analytics',
              path: '/dashboard/analytics',
            },
          ],
        },
        {
          meta: { order: 1, title: 'KAdmin' },
          name: 'KAdmin',
          path: '/kadmin',
          children: [
            {
              component: '/kadmin/components/UserManagementView',
              meta: { order: 0, title: '用户管理' },
              name: 'KAdminUsers',
              path: '/kadmin/users',
            },
            {
              component: '/kadmin/components/MenuManagementView',
              meta: { order: 1, title: '菜单管理' },
              name: 'KAdminMenus',
              path: '/kadmin/menus',
            },
          ],
        },
      ],
      [
        {
          meta: { order: 0, title: 'Dashboard' },
          name: 'Dashboard',
          path: '/dashboard',
          children: [
            {
              component: '/dashboard/analytics/index',
              meta: { order: 0, title: '分析页' },
              name: 'Analytics',
              path: '/dashboard/analytics',
            },
            {
              component: '/kadmin/components/MenuManagementView',
              meta: { order: 1, title: '菜单管理' },
              name: 'KAdminMenus',
              path: '/kadmin/menus',
            },
          ],
        },
        {
          meta: { order: 1, title: 'KAdmin' },
          name: 'KAdmin',
          path: '/kadmin',
          children: [
            {
              component: '/kadmin/components/UserManagementView',
              meta: { order: 0, title: '用户管理' },
              name: 'KAdminUsers',
              path: '/kadmin/users',
            },
          ],
        },
      ],
    ];
    let snapshotIndex = 0;
    const { getRoot, router } = createFakeRouter();
    const analyticsComponent = () => Promise.resolve({});
    const menuComponent = () => Promise.resolve({});
    const userComponent = () => Promise.resolve({});
    const options: GenerateMenuAndRoutesOptions = {
      fetchMenuListAsync: async () => snapshots[snapshotIndex++] ?? [],
      pageMap: {
        '../views/dashboard/analytics/index.vue': analyticsComponent,
        '../views/kadmin/components/MenuManagementView.vue': menuComponent,
        '../views/kadmin/components/UserManagementView.vue': userComponent,
      },
      router,
      routes: [
        {
          meta: { order: -1, title: '静态菜单' },
          name: 'StaticMenu',
          path: '/static-menu',
        },
      ],
    };

    const first = await generateAccessible('backend', options);
    expect(
      first.accessibleMenus[1]?.children?.map((menu) => menu.name),
    ).toEqual(['菜单管理', '用户管理']);

    const second = await generateAccessible('backend', options);
    expect(
      second.accessibleMenus[1]?.children?.map((menu) => menu.name),
    ).toEqual(['用户管理', '菜单管理']);
    expect(
      second.accessibleRoutes[1]?.children?.map((route) => route.name),
    ).toEqual(['KAdminUsers', 'KAdminMenus']);
    expect(second.accessibleRoutes[1]?.children?.[1]?.component).toBe(
      menuComponent,
    );
    expect(
      getRoot().children?.[1]?.children?.map((route) => route.name),
    ).toEqual(['KAdminUsers', 'KAdminMenus']);

    const third = await generateAccessible('backend', options);
    expect(
      third.accessibleMenus[0]?.children?.map((menu) => menu.name),
    ).toEqual(['分析页', '菜单管理']);
    expect(
      third.accessibleMenus[1]?.children?.map((menu) => menu.name),
    ).toEqual(['用户管理']);
    expect(third.accessibleRoutes.map((route) => route.name)).toEqual([
      'Dashboard',
      'KAdmin',
    ]);
    expect(getRoot().children?.map((route) => route.name)).toEqual([
      'Dashboard',
      'KAdmin',
    ]);
    expect(
      getRoot().children?.[0]?.children?.map((route) => route.name),
    ).toEqual(['Analytics', 'KAdminMenus']);
    expect(
      getRoot().children?.[1]?.children?.map((route) => route.name),
    ).toEqual(['KAdminUsers']);
  });
});

function createFakeRouter(baseChildren: RouteRecordRaw[] = []) {
  const root = {
    children: baseChildren,
    name: 'Root',
    path: '/',
  } as RouteRecordRaw;
  const routes: RouteRecordRaw[] = [root];

  const router = {
    addRoute(route: RouteRecordRaw) {
      removeRoute(route);
      routes.push(route);
      return () => removeRoute(route);
    },
    getRoutes() {
      return flattenRoutes(routes);
    },
    removeRoute(name: RouteRecordRaw['name']) {
      if (name) {
        removeRoute({ name } as RouteRecordRaw);
      }
    },
  } as unknown as AccessRouter;

  function removeRoute(route: RouteRecordRaw) {
    removeMatchingRoute(routes, route);
  }

  return {
    getRoot: () =>
      routes.find((route) => route.name === 'Root') as RouteRecordRaw,
    router,
    routes,
  };
}

function removeMatchingRoute(
  routes: RouteRecordRaw[],
  target: RouteRecordRaw,
): boolean {
  const index = routes.findIndex((route) =>
    target.name ? route.name === target.name : route.path === target.path,
  );
  if (index !== -1) {
    routes.splice(index, 1);
    return true;
  }
  return routes.some((route) =>
    removeMatchingRoute(route.children ?? [], target),
  );
}

function flattenRoutes(routes: RouteRecordRaw[]): RouteRecordRaw[] {
  return routes.flatMap((route) => [
    route,
    ...flattenRoutes(route.children ?? []),
  ]);
}

function countRoutesByName(route: RouteRecordRaw, name: string): number {
  return (
    (route.name === name ? 1 : 0) +
    (route.children ?? []).reduce(
      (count, child) => count + countRoutesByName(child, name),
      0,
    )
  );
}
