import type { Router } from 'vue-router';

import type { MenuRecordRaw } from '@vben/types';

const FORBIDDEN_PATH = '/403';
const NOT_FOUND_ROUTE_NAME = 'FallbackNotFound';

function resolveAccessibleHomePath(
  router: Router,
  preferredPath: string,
  menus: MenuRecordRaw[],
): string {
  const resolved = router.resolve(preferredPath);
  if (isNavigableMatch(resolved.matched)) {
    return preferredPath;
  }
  return findFirstNavigablePath(router, menus) ?? FORBIDDEN_PATH;
}

function findFirstNavigablePath(
  router: Router,
  menus: MenuRecordRaw[],
): string | undefined {
  for (const menu of menus) {
    const childPath = findFirstNavigablePath(router, menu.children ?? []);
    if (childPath) {
      return childPath;
    }
    if (menu.path && isNavigableMatch(router.resolve(menu.path).matched)) {
      return menu.path;
    }
  }
}

function isNavigableMatch(matched: ReturnType<Router['resolve']>['matched']) {
  const route = matched.at(-1);
  return (
    route?.name !== NOT_FOUND_ROUTE_NAME &&
    (!!route?.redirect || Object.keys(route?.components ?? {}).length > 0)
  );
}

export { FORBIDDEN_PATH, resolveAccessibleHomePath };
