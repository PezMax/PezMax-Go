import { createApp, defineComponent, h, nextTick } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import BookmarkFavoriteListView from '../../../views/kadmin/generated/bookmark_favorite/BookmarkFavoriteListView.vue';
import EbookFavoriteListView from '../../../views/kadmin/generated/ebook_favorite/EbookFavoriteListView.vue';
import { request } from '../client';
import * as bookmarkApi from './bookmark_favorite';
import * as ebookApi from './ebook_favorite';

vi.mock('../client', () => ({ request: vi.fn() }));
vi.mock('@vben/access', () => ({ useAccess: () => ({ hasAccessByCodes: () => true }) }));
vi.mock('ant-design-vue', () => ({ message: { success: vi.fn(), error: vi.fn() }, Modal: { confirm: vi.fn() } }));

const requestMock = vi.mocked(request);
const dispose: Array<() => void> = [];

function mountPage(component: typeof BookmarkFavoriteListView) {
  const app = createApp(component);
  app.config.warnHandler = () => {};
  const stub = defineComponent({
    props: { model: { type: Object, default: () => ({}) }, rules: { type: Object, default: () => ({}) } },
    setup(props, { slots, expose }) {
      // 执行页面声明的 required / 数值 min 规则，让必填负例有真实校验语义。
      expose({
        validate: async () => {
          const problems: string[] = [];
          for (const [field, fieldRules] of Object.entries(props.rules)) {
            const value = (props.model as Record<string, unknown>)[field];
            for (const rule of fieldRules as Array<{ required?: boolean; min?: number; message: string }>) {
              if (rule.required && (value === undefined || value === null)) {
                problems.push(rule.message);
                break;
              }
              if (rule.min !== undefined && typeof value === 'number' && value < rule.min) {
                problems.push(rule.message);
                break;
              }
            }
          }
          if (problems.length > 0) throw new Error(problems.join('; '));
        },
      });
      return () => h('div', {}, slots.default?.());
    },
  });
  for (const name of ['AAlert', 'AButton', 'AForm', 'AFormItem', 'AInputNumber', 'AModal', 'APopconfirm', 'ASpace', 'ATable', 'ATag', 'ATooltip', 'ATypographyText']) app.component(name, stub);
  const root = document.createElement('div');
  document.body.append(root);
  const vm = app.mount(root);
  dispose.push(() => { app.unmount(); root.remove(); });
  return (vm.$ as unknown as { setupState: Record<string, any> }).setupState;
}

describe('composite favorite API and pages', () => {
  beforeEach(() => {
    requestMock.mockReset();
    requestMock.mockResolvedValue({ items: [], total: 0 });
  });
  afterEach(() => {
    for (const cleanup of dispose.splice(0)) cleanup();
  });

  for (const spec of [
    { name: 'bookmark', resource: 'bookmarkId', path: '/api/bookmark-favorites', component: BookmarkFavoriteListView, key: bookmarkApi.bookmarkFavoriteKey, get: bookmarkApi.getBookmarkFavorite, create: bookmarkApi.createBookmarkFavorite, update: bookmarkApi.updateBookmarkFavorite, remove: bookmarkApi.deleteBookmarkFavorite },
    { name: 'ebook', resource: 'ebookId', path: '/api/ebook-favorites', component: EbookFavoriteListView, key: ebookApi.ebookFavoriteKey, get: ebookApi.getEbookFavorite, create: ebookApi.createEbookFavorite, update: ebookApi.updateEbookFavorite, remove: ebookApi.deleteEbookFavorite },
  ]) {
    const key = { [spec.resource]: 42, userId: 7 } as any;
    const moved = { [spec.resource]: 42, userId: 8 } as any;

    it(`${spec.name}: get, edit and delete address both keys`, async () => {
      await spec.get(key);
      await spec.update(key, moved);
      await spec.remove(key);
      expect(requestMock).toHaveBeenNthCalledWith(1, `${spec.path}/42,7`);
      expect(requestMock).toHaveBeenNthCalledWith(2, `${spec.path}/42,7`, { method: 'PUT', body: JSON.stringify(moved) });
      expect(requestMock).toHaveBeenNthCalledWith(3, `${spec.path}/42,7`, { method: 'DELETE' });
      expect(spec.key(key)).not.toEqual(spec.key(moved));
      expect(() => spec.key({ [spec.resource]: 42 } as any)).toThrow('正整数');
    });

    it(`${spec.name}: creation includes resource and user IDs`, async () => {
      await spec.create(key);
      expect(requestMock).toHaveBeenCalledWith(spec.path, { method: 'POST', body: JSON.stringify(key) });
    });

    it(`${spec.name}: form creation keeps both required identifiers`, async () => {
      const page = mountPage(spec.component);
      await nextTick();
      page.openCreate();
      page.formState[spec.resource] = 42;
      page.formState.userId = 7;
      await page.submit();
      expect(requestMock).toHaveBeenCalledWith(spec.path, { method: 'POST', body: JSON.stringify(key) });
    });

    it(`${spec.name}: form validation blocks missing or invalid identifiers`, async () => {
      for (const state of [
        { [spec.resource]: undefined, userId: 7 },
        { [spec.resource]: 42, userId: undefined },
        { [spec.resource]: 0, userId: 7 },
        { [spec.resource]: 42, userId: 0 },
        { [spec.resource]: -42, userId: 7 },
        { [spec.resource]: 42, userId: -3 },
      ] as Array<Record<string, number | undefined>>) {
        const page = mountPage(spec.component);
        await nextTick();
        page.openCreate();
        Object.assign(page.formState, state);
        await page.submit();
        expect(requestMock).not.toHaveBeenCalledWith(spec.path, { method: 'POST', body: expect.any(String) });
        dispose.pop()?.();
      }
    });

    it(`${spec.name}: editing preserves the original key when user changes`, async () => {
      const page = mountPage(spec.component);
      await nextTick();
      await page.openEdit(key);
      page.formState.userId = 8;
      await page.submit();
      expect(requestMock).toHaveBeenCalledWith(`${spec.path}/42,7`, { method: 'PUT', body: JSON.stringify(moved) });
      await page.remove(key);
      expect(requestMock).toHaveBeenCalledWith(`${spec.path}/42,7`, { method: 'DELETE' });
    });
  }
});
