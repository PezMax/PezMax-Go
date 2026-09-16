import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  confirmExternalNavigation,
  externalNavigationOpensNewWindow,
  setExternalNavigationGuard,
} from './use-navigation';

afterEach(() => setExternalNavigationGuard(undefined));

describe('external navigation confirmation', () => {
  it('uses the registered confirmation guard', async () => {
    const guard = vi.fn().mockResolvedValue(false);
    setExternalNavigationGuard(guard);
    const route = {
      meta: { confirmExternal: true, title: 'Documentation' },
    } as never;

    await expect(
      confirmExternalNavigation(route, 'https://example.com/docs'),
    ).resolves.toBe(false);
    expect(guard).toHaveBeenCalledWith({
      openInNewWindow: true,
      title: 'Documentation',
      url: 'https://example.com/docs',
    });
  });

  it('does not prompt ordinary external routes', async () => {
    const guard = vi.fn();
    setExternalNavigationGuard(guard);

    await expect(
      confirmExternalNavigation({ meta: {} } as never, 'https://example.com'),
    ).resolves.toBe(true);
    expect(guard).not.toHaveBeenCalled();
  });

  it('uses confirmation metadata preserved on a generated menu', async () => {
    const guard = vi.fn().mockResolvedValue(true);
    setExternalNavigationGuard(guard);

    await expect(
      confirmExternalNavigation(
        {
          confirmExternal: true,
          name: 'External documentation',
          path: 'https://example.com',
        },
        'https://example.com',
      ),
    ).resolves.toBe(true);
    expect(guard).toHaveBeenCalledWith({
      openInNewWindow: true,
      title: 'External documentation',
      url: 'https://example.com',
    });
  });
});

describe('external navigation target', () => {
  it('defaults external links to a new window', () => {
    expect(
      externalNavigationOpensNewWindow({
        meta: { link: 'https://example.com' },
      } as never),
    ).toBe(true);
  });

  it('supports current-page route and menu metadata', () => {
    expect(
      externalNavigationOpensNewWindow({
        meta: { link: 'https://example.com', openInNewWindow: false },
      } as never),
    ).toBe(false);
    expect(
      externalNavigationOpensNewWindow({
        name: 'Example',
        openInNewWindow: false,
        path: 'https://example.com',
      }),
    ).toBe(false);
  });
});
