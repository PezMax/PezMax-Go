import { beforeEach, describe, expect, it, vi } from 'vitest';

import { request } from './client';
import {
  ADMIN_MENU_TYPE,
  type AdminMenu,
  updateAdminMenuLayout,
} from './menus';

vi.mock('./client', () => ({
  request: vi.fn(),
}));

const requestMock = vi.mocked(request);
const positions = [{ id: 1, order: 0, parentId: 0 }];
const tree: AdminMenu[] = [
  {
    children: [],
    icon: '',
    id: 1,
    order: 0,
    parentId: 0,
    title: 'Root',
    type: ADMIN_MENU_TYPE.DIRECTORY,
    uri: '/root',
  },
];

describe('updateAdminMenuLayout', () => {
  beforeEach(() => {
    requestMock.mockReset();
  });

  it('returns the menu tree from the current API', async () => {
    requestMock.mockResolvedValueOnce(tree);

    await expect(updateAdminMenuLayout(positions)).resolves.toBe(tree);
    expect(requestMock).toHaveBeenCalledTimes(1);
  });

  it('reloads the tree when a legacy API returns true', async () => {
    requestMock.mockResolvedValueOnce(true).mockResolvedValueOnce(tree);

    await expect(updateAdminMenuLayout(positions)).resolves.toBe(tree);
    expect(requestMock).toHaveBeenNthCalledWith(1, '/api/admin-menus', {
      body: JSON.stringify({ items: positions }),
      method: 'PUT',
    });
    expect(requestMock).toHaveBeenNthCalledWith(2, '/api/admin-menus/tree');
  });
});

describe('admin menu types', () => {
  it('keeps external links distinct from internal menus', () => {
    expect(ADMIN_MENU_TYPE.EXTERNAL).toBe(2);
  });
});
