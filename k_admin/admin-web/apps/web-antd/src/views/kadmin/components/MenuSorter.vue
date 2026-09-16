<script setup lang="ts">
import type { MenuMoveDirection } from './menu-sort';

import type { AdminMenu, AdminMenuPosition } from '#/api/kadmin/menus';

import { computed, ref, watch } from 'vue';

import { IconifyIcon } from '@vben/icons';

import {
  ArrowDownOutlined,
  ArrowLeftOutlined,
  ArrowRightOutlined,
  ArrowUpOutlined,
  CompressOutlined,
  ExpandOutlined,
} from '@ant-design/icons-vue';

import {
  canIndentMenu,
  canMoveMenu,
  canOutdentMenu,
  flattenMenuPositions,
  indentMenuTree,
  moveMenuTree,
  normalizeMenuTree,
  outdentMenuTree,
} from './menu-sort';

const props = defineProps<{
  menus: AdminMenu[];
  open: boolean;
  saving?: boolean;
}>();

const emit = defineEmits<{
  save: [items: AdminMenuPosition[]];
  'update:open': [open: boolean];
}>();

const tree = ref<AdminMenu[]>([]);
const expandedKeys = ref<number[]>([]);
const dirty = ref(false);

const modalOpen = computed({
  get: () => props.open,
  set: (open: boolean) => emit('update:open', open),
});
const menuCount = computed(() => flattenMenuPositions(tree.value).length);
const rootCount = computed(() => tree.value.length);

watch(
  () => props.open,
  (open) => {
    if (!open) {
      return;
    }
    tree.value = normalizeMenuTree(props.menus);
    expandedKeys.value = collectMenuIds(tree.value);
    dirty.value = false;
  },
);

watch(
  () => props.menus,
  (menus) => {
    if (!props.open || dirty.value) {
      return;
    }
    tree.value = normalizeMenuTree(menus);
    expandedKeys.value = collectMenuIds(tree.value);
  },
);

function move(menuId: number, direction: MenuMoveDirection) {
  applyTree(moveMenuTree(tree.value, menuId, direction));
}

function changeLevel(menuId: number, direction: 'in' | 'out') {
  applyTree(
    direction === 'in'
      ? indentMenuTree(tree.value, menuId)
      : outdentMenuTree(tree.value, menuId),
  );
}

function save() {
  emit('save', flattenMenuPositions(tree.value));
}

function expandAll() {
  expandedKeys.value = collectMenuIds(tree.value);
}

function collapseAll() {
  expandedKeys.value = [];
}

function applyTree(nextTree: AdminMenu[]): boolean {
  const current = flattenMenuPositions(tree.value);
  const next = flattenMenuPositions(nextTree);
  if (
    current.length === next.length &&
    current.every(
      (item, index) =>
        item.id === next[index]?.id &&
        item.parentId === next[index]?.parentId &&
        item.order === next[index]?.order,
    )
  ) {
    return false;
  }
  tree.value = nextTree;
  dirty.value = true;
  return true;
}

function collectMenuIds(items: AdminMenu[]): number[] {
  return items.flatMap((item) => [
    item.id,
    ...collectMenuIds(item.children ?? []),
  ]);
}
</script>

<template>
  <a-modal
    v-model:open="modalOpen"
    :confirm-loading="saving"
    :ok-button-props="{ disabled: !dirty }"
    ok-text="保存布局"
    title="菜单布局"
    :width="760"
    @ok="save"
  >
    <div class="sort-toolbar">
      <a-space :size="8" wrap>
        <a-tag>{{ menuCount }} 个菜单</a-tag>
        <a-tag>{{ rootCount }} 个根菜单</a-tag>
      </a-space>
      <a-space>
        <a-tooltip title="展开全部">
          <a-button type="text" shape="circle" @click="expandAll">
            <ExpandOutlined />
          </a-button>
        </a-tooltip>
        <a-tooltip title="收起全部">
          <a-button type="text" shape="circle" @click="collapseAll">
            <CompressOutlined />
          </a-button>
        </a-tooltip>
      </a-space>
    </div>

    <div class="sort-tree-wrap">
      <a-empty v-if="tree.length === 0" description="暂无菜单" />
      <a-tree
        v-else
        v-model:expanded-keys="expandedKeys"
        block-node
        :selectable="false"
        :field-names="{ children: 'children', key: 'id', title: 'title' }"
        :tree-data="tree"
      >
        <template #title="item">
          <div class="sort-node">
            <IconifyIcon v-if="item.icon" :icon="item.icon" class="menu-icon" />
            <span class="menu-name" :title="item.title">{{ item.title }}</span>
            <span v-if="item.uri" class="menu-path">{{ item.uri }}</span>
            <a-space :size="0" class="sort-actions">
              <a-tooltip title="提升一级">
                <a-button
                  type="text"
                  shape="circle"
                  size="small"
                  :disabled="saving || !canOutdentMenu(tree, item.id)"
                  @click.stop="changeLevel(item.id, 'out')"
                >
                  <ArrowLeftOutlined />
                </a-button>
              </a-tooltip>
              <a-tooltip title="设为上一目录/分组的子级">
                <a-button
                  type="text"
                  shape="circle"
                  size="small"
                  :disabled="saving || !canIndentMenu(tree, item.id)"
                  @click.stop="changeLevel(item.id, 'in')"
                >
                  <ArrowRightOutlined />
                </a-button>
              </a-tooltip>
              <a-tooltip title="上移">
                <a-button
                  type="text"
                  shape="circle"
                  size="small"
                  :disabled="saving || !canMoveMenu(tree, item.id, 'up')"
                  @click.stop="move(item.id, 'up')"
                >
                  <ArrowUpOutlined />
                </a-button>
              </a-tooltip>
              <a-tooltip title="下移">
                <a-button
                  type="text"
                  shape="circle"
                  size="small"
                  :disabled="saving || !canMoveMenu(tree, item.id, 'down')"
                  @click.stop="move(item.id, 'down')"
                >
                  <ArrowDownOutlined />
                </a-button>
              </a-tooltip>
            </a-space>
          </div>
        </template>
      </a-tree>
    </div>
  </a-modal>
</template>

<style scoped>
.sort-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 40px;
  margin-bottom: 8px;
}

.sort-tree-wrap {
  min-height: 360px;
  max-height: 60vh;
  padding: 8px;
  overflow: auto;
  border: 1px solid hsl(var(--border));
  border-radius: 6px;
}

.sort-node {
  display: flex;
  gap: 8px;
  align-items: center;
  width: 100%;
  min-width: 0;
  padding-right: 8px;
  overflow: hidden;
}

.menu-icon {
  flex: none;
  width: 16px;
  height: 16px;
}

.menu-name {
  flex: 0 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  font-weight: 500;
  white-space: nowrap;
}

.menu-path {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  color: hsl(var(--muted-foreground));
  white-space: nowrap;
}

.sort-actions {
  flex: none;
  margin-left: auto;
}

:deep(.ant-tree-node-content-wrapper) {
  min-width: 0;
}

:deep(.ant-tree-title) {
  display: block;
  min-width: 0;
}

@media (max-width: 640px) {
  .sort-tree-wrap {
    min-height: 300px;
  }

  .menu-path {
    display: none;
  }

  .sort-node {
    gap: 6px;
  }

  .sort-actions :deep(.ant-btn) {
    width: 26px;
    min-width: 26px;
    height: 26px;
  }
}
</style>
