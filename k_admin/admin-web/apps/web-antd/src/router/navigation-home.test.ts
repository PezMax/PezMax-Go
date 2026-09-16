import type { Router } from 'vue-router';

import type { MenuRecordRaw } from '@vben/types';

import { describe, expect, it } from 'vitest';

import { FORBIDDEN_PATH, resolveAccessibleHomePath } from './navigation-home';

function createRouter(availablePaths: string[]): Router {
  return {
    resolve: (path: string) => ({
      matched: availablePaths.includes(path)
        ? [{ components: { default: {} }, name: 'AllowedRoute' }]
        : [{ name: 'FallbackNotFound' }],
    }),
  } as unknown as Router;
}

const menus: MenuRecordRaw[] = [
  {
    children: [
      { name: '用户管理', order: 0, path: '/kadmin/users' },
      { name: '菜单管理', order: 1, path: '/kadmin/menus' },
    ],
    name: 'KAdmin',
    order: 0,
    path: '/kadmin',
  },
];

describe('resolveAccessibleHomePath', () => {
  it('keeps an accessible configured home', () => {
    const router = createRouter(['/dashboard/workspace']);

    expect(
      resolveAccessibleHomePath(router, '/dashboard/workspace', menus),
    ).toBe('/dashboard/workspace');
  });

  it('falls back to the first database-ordered menu leaf', () => {
    const router = createRouter(['/kadmin/menus', '/kadmin/users']);

    expect(
      resolveAccessibleHomePath(router, '/dashboard/workspace', menus),
    ).toBe('/kadmin/users');
  });

  it('falls back to forbidden when the user has no menus', () => {
    const router = createRouter([]);

    expect(resolveAccessibleHomePath(router, '/dashboard/workspace', [])).toBe(
      FORBIDDEN_PATH,
    );
  });
});
