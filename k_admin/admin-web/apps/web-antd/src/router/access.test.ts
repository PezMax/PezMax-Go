import type { GenerateMenuAndRoutesOptions } from '@vben/types';

import { beforeEach, describe, expect, it, vi } from 'vitest';

import { generateAccess, refreshNavigation } from './access';

const mocks = vi.hoisted(() => ({
  generateAccessible: vi.fn(),
  getAllMenusApi: vi.fn(),
  loading: vi.fn(),
  setAccessMenus: vi.fn(),
  setAccessRoutes: vi.fn(),
  setIsAccessChecked: vi.fn(),
}));

vi.mock('@vben/access', () => ({
  generateAccessible: mocks.generateAccessible,
}));

vi.mock('@vben/stores', () => ({
  useAccessStore: () => ({
    setAccessMenus: mocks.setAccessMenus,
    setAccessRoutes: mocks.setAccessRoutes,
    setIsAccessChecked: mocks.setIsAccessChecked,
  }),
}));

vi.mock('ant-design-vue', () => ({
  message: { loading: mocks.loading },
}));

vi.mock('#/api', () => ({
  getAllMenusApi: mocks.getAllMenusApi,
}));

vi.mock('#/layouts', () => ({
  BasicLayout: {},
  IFrameView: {},
}));

vi.mock('#/locales', () => ({
  $t: (key: string) => key,
}));

vi.mock('#/preferences', () => ({
  KADMIN_ACCESS_MODE: 'backend',
}));

const router = {} as GenerateMenuAndRoutesOptions['router'];
const accessResult = {
  accessibleMenus: [{ name: '菜单管理', path: '/kadmin/menus' }],
  accessibleRoutes: [{ name: 'KAdminMenus', path: '/kadmin/menus' }],
};

describe('server-driven navigation', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.generateAccessible.mockResolvedValue(accessResult);
  });

  it('always generates navigation in backend mode without static routes', async () => {
    mocks.getAllMenusApi.mockResolvedValue([]);

    await generateAccess({ roles: ['admin'], router });

    expect(mocks.generateAccessible).toHaveBeenCalledWith(
      'backend',
      expect.objectContaining({
        roles: ['admin'],
        router,
        routes: [],
      }),
    );

    const options = mocks.generateAccessible.mock.calls[0]?.[1];
    await expect(options.fetchMenuListAsync()).resolves.toEqual([]);
    expect(mocks.getAllMenusApi).toHaveBeenCalledOnce();
  });

  it('replaces the sidebar store with the latest server snapshot', async () => {
    await expect(refreshNavigation({ router })).resolves.toBe(accessResult);

    expect(mocks.setAccessMenus).toHaveBeenCalledWith(
      accessResult.accessibleMenus,
    );
    expect(mocks.setAccessRoutes).toHaveBeenCalledWith(
      accessResult.accessibleRoutes,
    );
    expect(mocks.setIsAccessChecked).toHaveBeenCalledWith(true);
  });
});
