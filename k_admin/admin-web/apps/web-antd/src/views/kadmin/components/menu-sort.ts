import type { AdminMenu, AdminMenuPosition } from '#/api/kadmin/menus';

import { ADMIN_MENU_TYPE } from '#/api/kadmin/menus';

export type MenuMoveDirection = 'down' | 'up';
export type MenuPlacement = 'after' | 'before' | 'inside';

export function normalizeMenuTree(
  items: AdminMenu[],
  parentId = 0,
): AdminMenu[] {
  return items.map((item, index) => ({
    ...item,
    parentId,
    order: index,
    children: normalizeMenuTree(item.children ?? [], item.id),
  }));
}

export function moveMenuTree(
  items: AdminMenu[],
  menuId: number,
  direction: MenuMoveDirection,
): AdminMenu[] {
  const tree = normalizeMenuTree(items);
  moveWithinSiblings(tree, menuId, direction);
  return normalizeMenuTree(tree);
}

export function relocateMenuTree(
  items: AdminMenu[],
  menuId: number,
  targetId: number,
  placement: MenuPlacement,
): AdminMenu[] {
  const tree = normalizeMenuTree(items);
  if (menuId === targetId) {
    return tree;
  }

  const source = findMenu(tree, menuId);
  const target = findMenu(tree, targetId);
  if (
    !source ||
    !target ||
    findMenu(source.children ?? [], targetId) ||
    (placement === 'inside' && target.type !== ADMIN_MENU_TYPE.DIRECTORY)
  ) {
    return tree;
  }

  const detached = detachMenu(tree, menuId);
  if (!detached || !insertMenu(tree, targetId, detached, placement)) {
    return normalizeMenuTree(items);
  }
  return normalizeMenuTree(tree);
}

export function indentMenuTree(
  items: AdminMenu[],
  menuId: number,
): AdminMenu[] {
  const tree = normalizeMenuTree(items);
  const location = findMenuLocation(tree, menuId);
  const previousSibling = location?.siblings[location.index - 1];
  if (!previousSibling || previousSibling.type !== ADMIN_MENU_TYPE.DIRECTORY) {
    return tree;
  }
  return relocateMenuTree(tree, menuId, previousSibling.id, 'inside');
}

export function outdentMenuTree(
  items: AdminMenu[],
  menuId: number,
): AdminMenu[] {
  const tree = normalizeMenuTree(items);
  const location = findMenuLocation(tree, menuId);
  if (!location || location.parentId === 0) {
    return tree;
  }
  return relocateMenuTree(tree, menuId, location.parentId, 'after');
}

export function canMoveMenu(
  items: AdminMenu[],
  menuId: number,
  direction: MenuMoveDirection,
): boolean {
  const index = items.findIndex((item) => item.id === menuId);
  if (index !== -1) {
    return direction === 'up' ? index > 0 : index < items.length - 1;
  }
  return items.some((item) =>
    canMoveMenu(item.children ?? [], menuId, direction),
  );
}

export function canIndentMenu(items: AdminMenu[], menuId: number): boolean {
  const location = findMenuLocation(items, menuId);
  const previousSibling = location?.siblings[location.index - 1];
  return previousSibling?.type === ADMIN_MENU_TYPE.DIRECTORY;
}

export function canOutdentMenu(items: AdminMenu[], menuId: number): boolean {
  return (findMenuLocation(items, menuId)?.parentId ?? 0) !== 0;
}

export function canSetMenuAsItem(menu?: AdminMenu | null): boolean {
  return !menu?.children?.length;
}

export function filterMenuParentOptions(
  items: AdminMenu[],
  excludeId?: number,
): AdminMenu[] {
  const result: AdminMenu[] = [];
  for (const item of items) {
    if (item.id === excludeId || item.type !== ADMIN_MENU_TYPE.DIRECTORY) {
      continue;
    }
    result.push({
      ...item,
      children: filterMenuParentOptions(item.children ?? [], excludeId),
    });
  }
  return result;
}

export function flattenMenuPositions(
  items: AdminMenu[],
  parentId = 0,
): AdminMenuPosition[] {
  return items.flatMap((item, index) => [
    { id: item.id, parentId, order: index },
    ...flattenMenuPositions(item.children ?? [], item.id),
  ]);
}

function moveWithinSiblings(
  items: AdminMenu[],
  menuId: number,
  direction: MenuMoveDirection,
): boolean {
  const index = items.findIndex((item) => item.id === menuId);
  if (index !== -1) {
    const targetIndex = direction === 'up' ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= items.length) {
      return false;
    }
    const current = items[index];
    const target = items[targetIndex];
    if (!current || !target) {
      return false;
    }
    items[index] = target;
    items[targetIndex] = current;
    return true;
  }
  for (const item of items) {
    if (moveWithinSiblings(item.children ?? [], menuId, direction)) {
      return true;
    }
  }
  return false;
}

function detachMenu(items: AdminMenu[], menuId: number): AdminMenu | undefined {
  const index = items.findIndex((item) => item.id === menuId);
  if (index !== -1) {
    return items.splice(index, 1)[0];
  }
  for (const item of items) {
    const detached = detachMenu(item.children ?? [], menuId);
    if (detached) {
      return detached;
    }
  }
}

function insertMenu(
  items: AdminMenu[],
  targetId: number,
  menu: AdminMenu,
  placement: MenuPlacement,
): boolean {
  const index = items.findIndex((item) => item.id === targetId);
  if (index !== -1) {
    if (placement === 'inside') {
      const target = items[index];
      if (!target) {
        return false;
      }
      target.children = [...(target.children ?? []), menu];
    } else {
      items.splice(placement === 'before' ? index : index + 1, 0, menu);
    }
    return true;
  }
  for (const item of items) {
    if (insertMenu(item.children ?? [], targetId, menu, placement)) {
      return true;
    }
  }
  return false;
}

function findMenu(items: AdminMenu[], menuId: number): AdminMenu | undefined {
  for (const item of items) {
    if (item.id === menuId) {
      return item;
    }
    const child = findMenu(item.children ?? [], menuId);
    if (child) {
      return child;
    }
  }
}

interface MenuLocation {
  index: number;
  parentId: number;
  siblings: AdminMenu[];
}

function findMenuLocation(
  items: AdminMenu[],
  menuId: number,
  parentId = 0,
): MenuLocation | undefined {
  const index = items.findIndex((item) => item.id === menuId);
  if (index !== -1) {
    return { index, parentId, siblings: items };
  }
  for (const item of items) {
    const location = findMenuLocation(item.children ?? [], menuId, item.id);
    if (location) {
      return location;
    }
  }
}
