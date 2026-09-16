import type { AdminMenu, AdminMenuType } from '#/api/kadmin/menus';

import { describe, expect, it } from 'vitest';

import { ADMIN_MENU_TYPE } from '#/api/kadmin/menus';

import {
  canIndentMenu,
  canMoveMenu,
  canOutdentMenu,
  canSetMenuAsItem,
  filterMenuParentOptions,
  flattenMenuPositions,
  indentMenuTree,
  moveMenuTree,
  outdentMenuTree,
  relocateMenuTree,
} from './menu-sort';

function menu(
  id: number,
  children: AdminMenu[] = [],
  type: AdminMenuType = ADMIN_MENU_TYPE.MENU,
): AdminMenu {
  return {
    id,
    parentId: 0,
    type,
    order: 0,
    title: `Menu ${id}`,
    icon: '',
    uri: `/menu-${id}`,
    children,
  };
}

function directory(id: number, children: AdminMenu[] = []): AdminMenu {
  return menu(id, children, ADMIN_MENU_TYPE.DIRECTORY);
}

describe('menu layout', () => {
  it('reorders sibling menus', () => {
    const result = moveMenuTree(
      [directory(1, [menu(2), menu(3)]), menu(4)],
      3,
      'up',
    );

    expect(result[0]?.children?.map((item) => item.id)).toEqual([3, 2]);
    expect(flattenMenuPositions(result)).toContainEqual({
      id: 3,
      parentId: 1,
      order: 0,
    });
  });

  it('does not move a menu beyond its sibling boundary', () => {
    const tree = [directory(1, [menu(2), menu(3)]), menu(4)];
    const result = moveMenuTree(tree, 2, 'up');

    expect(canMoveMenu(tree, 2, 'up')).toBe(false);
    expect(canMoveMenu(tree, 2, 'down')).toBe(true);
    expect(result[0]?.children?.map((item) => item.id)).toEqual([2, 3]);
  });

  it('moves a menu before a node under another parent', () => {
    const original = [
      directory(1, [menu(2), menu(3)]),
      directory(4, [menu(5)]),
    ];
    const result = relocateMenuTree(original, 3, 5, 'before');

    expect(result[0]?.children?.map((item) => item.id)).toEqual([2]);
    expect(result[1]?.children?.map((item) => item.id)).toEqual([3, 5]);
    expect(flattenMenuPositions(result)).toContainEqual({
      id: 3,
      parentId: 4,
      order: 0,
    });
    expect(original[0]?.children?.map((item) => item.id)).toEqual([2, 3]);
  });

  it('moves a menu inside another node', () => {
    const result = relocateMenuTree(
      [directory(1, [directory(2)]), menu(3)],
      3,
      2,
      'inside',
    );

    expect(result).toMatchObject([
      {
        id: 1,
        children: [{ id: 2, children: [{ id: 3, parentId: 2, order: 0 }] }],
      },
    ]);
  });

  it('rejects moving a menu into its descendant', () => {
    const tree = [directory(1, [directory(2, [menu(3)])]), menu(4)];
    const result = relocateMenuTree(tree, 1, 3, 'inside');

    expect(flattenMenuPositions(result)).toEqual(flattenMenuPositions(tree));
  });

  it('indents and outdents a menu', () => {
    const tree = [directory(1), menu(2), menu(3)];
    const indented = indentMenuTree(tree, 2);

    expect(canIndentMenu(tree, 1)).toBe(false);
    expect(canIndentMenu(tree, 2)).toBe(true);
    expect(indented[0]?.children?.map((item) => item.id)).toEqual([2]);
    expect(canOutdentMenu(indented, 2)).toBe(true);

    const outdented = outdentMenuTree(indented, 2);
    expect(outdented.map((item) => item.id)).toEqual([1, 2, 3]);
    expect(canOutdentMenu(outdented, 2)).toBe(false);
  });

  it('does not indent a menu under a menu item', () => {
    const tree = [menu(1), menu(2)];

    expect(canIndentMenu(tree, 2)).toBe(false);
    expect(flattenMenuPositions(indentMenuTree(tree, 2))).toEqual(
      flattenMenuPositions(tree),
    );
    expect(flattenMenuPositions(relocateMenuTree(tree, 2, 1, 'inside'))).toEqual(
      flattenMenuPositions(tree),
    );
  });

  it('only exposes directories as parent options', () => {
    const tree = [
      directory(1, [menu(2), directory(3)]),
      menu(4),
      directory(5),
    ];

    expect(filterMenuParentOptions(tree).map((item) => item.id)).toEqual([1, 5]);
    expect(filterMenuParentOptions(tree)[0]?.children?.map((item) => item.id)).toEqual([
      3,
    ]);
    expect(filterMenuParentOptions(tree, 3)[0]?.children).toEqual([]);
  });

  it('only allows childless nodes to become menu items', () => {
    expect(canSetMenuAsItem(directory(1))).toBe(true);
    expect(canSetMenuAsItem(directory(1, [menu(2)]))).toBe(false);
  });
});
