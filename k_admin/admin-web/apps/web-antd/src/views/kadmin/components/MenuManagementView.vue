<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>菜单管理</h1>
        <p>维护后台导航菜单的层级、路径与图标</p>
      </div>
      <a-space wrap>
        <a-button
          v-if="canLayout"
          :disabled="loading"
          @click="layoutEditorOpen = true"
        >
          <ApartmentOutlined />
          菜单布局
        </a-button>
        <a-button :loading="loading" @click="refreshMenuData">
          <ReloadOutlined />
          刷新
        </a-button>
        <a-button v-if="canCreate" type="primary" @click="openDrawer()">
          <PlusOutlined />
          新增菜单
        </a-button>
      </a-space>
    </section>

    <section class="panel">
      <a-form :model="filters" layout="inline" class="search-form">
        <a-form-item label="关键词">
          <a-input
            v-model:value="filters.keyword"
            allow-clear
            class="control-lg"
            placeholder="菜单标题 / 路由 / 图标"
          />
        </a-form-item>
        <a-form-item label="类型">
          <a-select
            v-model:value="filters.type"
            allow-clear
            class="control-md"
            :options="typeOptions"
            placeholder="全部"
          />
        </a-form-item>
        <a-form-item>
          <a-space>
            <a-button type="primary" @click="applySearch">
              <SearchOutlined />
              查询
            </a-button>
            <a-button @click="resetSearch">
              <ClearOutlined />
              重置
            </a-button>
          </a-space>
        </a-form-item>
      </a-form>
    </section>

    <section class="panel">
      <div class="table-toolbar">
        <span class="muted-text"> 当前菜单树按导航展示顺序排列。 </span>
        <a-segmented v-model:value="density" :options="['默认', '紧凑']" />
      </div>

      <a-table
        row-key="id"
        :columns="columns"
        :data-source="filteredMenus"
        :expanded-row-keys="expandedRowKeys"
        :loading="loading"
        :pagination="false"
        :row-class-name="menuRowClassName"
        :scroll="{ x: 1040 }"
        :size="density === '紧凑' ? 'small' : 'middle'"
        @expanded-rows-change="onExpandedRowsChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'title'">
            <div class="menu-title-cell">
              <span class="menu-icon-box" :title="record.icon || '未设置图标'">
                <IconifyIcon v-if="record.icon" :icon="record.icon" />
                <span v-else class="menu-icon-fallback">—</span>
              </span>
              <div class="name-cell">
                <strong>{{ record.title }}</strong>
                <span>#{{ record.id }}</span>
              </div>
            </div>
          </template>

          <template v-else-if="column.key === 'uri'">
            <span v-if="record.uri" class="code-cell">{{ record.uri }}</span>
            <span v-else class="muted-text">—</span>
          </template>

          <template v-else-if="column.key === 'type'">
            <a-tag :color="typeColor(record.type)">
              {{ typeText(record.type) }}
            </a-tag>
          </template>

          <template v-else-if="column.key === 'action'">
            <a-space>
              <a-button
                v-if="record.type === ADMIN_MENU_TYPE.DIRECTORY && canCreate"
                type="link"
                size="small"
                @click="openDrawer(undefined, record)"
              >
                新增子级
              </a-button>
              <a-button
                v-if="canUpdate"
                type="link"
                size="small"
                @click="openDrawer(record)"
              >
                编辑
              </a-button>
              <a-popconfirm
                v-if="canDelete"
                title="确认删除该菜单？存在子菜单时将无法删除，请先删除子菜单。"
                @confirm="removeMenu(record)"
              >
                <a-button type="link" size="small" danger>删除</a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </section>

    <a-drawer
      v-model:open="drawerOpen"
      :destroy-on-close="true"
      :title="editingMenu ? '编辑菜单' : '新增菜单'"
      width="560"
    >
      <a-form ref="formRef" :model="formState" :rules="rules" layout="vertical">
        <a-form-item label="菜单类型" name="type">
          <a-select
            v-model:value="formState.type"
            :options="formTypeOptions"
            placeholder="请先选择菜单类型"
            @change="handleMenuTypeChange"
          />
        </a-form-item>

        <a-alert
          v-if="!hasSelectedMenuType"
          class="form-alert"
          show-icon
          type="info"
          message="请先选择菜单类型"
          description="目录/分组用于分类；菜单访问系统页面；外链会在用户确认后按系统参数设置跳转。"
        />

        <template v-if="hasSelectedMenuType">
          <a-form-item label="上级菜单" name="parentId">
            <a-tree-select
              v-model:value="formState.parentId"
              allow-clear
              tree-default-expand-all
              :field-names="{
                children: 'children',
                label: 'title',
                value: 'id',
              }"
              :tree-data="parentOptions"
              placeholder="根菜单"
            />
          </a-form-item>
          <a-form-item label="菜单标题" name="title">
            <a-input
              v-model:value="formState.title"
              :placeholder="
                isDirectoryType ? '例如：系统管理' : '例如：菜单管理'
              "
            />
          </a-form-item>
          <a-form-item
            v-if="isMenuType || isExternalType"
            :label="isExternalType ? '外链地址' : '访问路径'"
            name="uri"
          >
            <a-input
              v-model:value="formState.uri"
              :placeholder="
                isExternalType
                  ? '例如：https://example.com'
                  : '例如：/kadmin/menus'
              "
            />
          </a-form-item>
          <a-form-item label="图标" name="icon">
            <IconPicker
              v-model="formState.icon"
              allow-clear
              class="menu-icon-picker"
              :auto-fetch-api="false"
              :icons="menuIconOptions"
              :input-component="iconPickerInput"
              icon-slot="addonAfter"
              model-value-prop="value"
              placeholder="请选择或搜索菜单图标"
              prefix=""
              @change="handleIconChange"
            />
          </a-form-item>
          <template v-if="isMenuType">
            <a-form-item label="分组标题" name="header">
              <a-input
                v-model:value="formState.header"
                placeholder="可选，对应 header 字段"
              />
            </a-form-item>
            <a-row :gutter="12">
              <a-col :span="12">
                <a-form-item label="插件名" name="pluginName">
                  <a-input
                    v-model:value="formState.pluginName"
                    placeholder="可选"
                  />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="UUID" name="uuid">
                  <a-input v-model:value="formState.uuid" placeholder="可选" />
                </a-form-item>
              </a-col>
            </a-row>
          </template>
        </template>
      </a-form>
      <template #extra>
        <a-space>
          <a-button @click="drawerOpen = false">取消</a-button>
          <a-button
            type="primary"
            :disabled="
              !hasSelectedMenuType || (editingMenu ? !canUpdate : !canCreate)
            "
            :loading="saving"
            @click="submitForm"
          >
            保存
          </a-button>
        </a-space>
      </template>
    </a-drawer>

    <MenuSorter
      v-model:open="layoutEditorOpen"
      :menus="menus"
      :saving="layoutSaving"
      @save="saveMenuLayout"
    />
  </div>
</template>

<script setup lang="ts">
import type { FormInstance } from 'ant-design-vue';
import type { VNode } from 'vue';

import type {
  AdminMenu,
  AdminMenuPayload,
  AdminMenuPosition,
  AdminMenuType,
} from '#/api/kadmin/menus';

import { computed, onMounted, reactive, ref } from 'vue';

import { IconPicker } from '@vben/common-ui';
import { IconifyIcon } from '@vben/icons';
import { useAccess } from '@vben/access';
import { useUserStore } from '@vben/stores';

import {
  ApartmentOutlined,
  ClearOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons-vue';
import { Input, message } from 'ant-design-vue';

import {
  ADMIN_MENU_TYPE,
  createAdminMenu,
  deleteAdminMenu,
  getAdminMenuTree,
  updateAdminMenu,
  updateAdminMenuLayout,
} from '#/api/kadmin/menus';
import { KADMIN_PERMISSION } from '#/api/kadmin/permissions';
import { router } from '#/router';
import { refreshNavigation } from '#/router/access';

import { canSetMenuAsItem, filterMenuParentOptions } from './menu-sort';
import MenuSorter from './MenuSorter.vue';

const { hasAccessByCodes } = useAccess();
const canCreate = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.MENU_CREATE, '*']),
);
const canUpdate = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.MENU_UPDATE, '*']),
);
const canLayout = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.MENU_LAYOUT, '*']),
);
const canDelete = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.MENU_DELETE, '*']),
);

const loading = ref(false);
const saving = ref(false);
const drawerOpen = ref(false);
const layoutEditorOpen = ref(false);
const layoutSaving = ref(false);
const density = ref('默认');
const menus = ref<AdminMenu[]>([]);
const editingMenu = ref<AdminMenu | null>(null);
const formRef = ref<FormInstance>();
const expandedRowKeys = ref<number[]>([]);
const userStore = useUserStore();
let navigationRefreshQueue: Promise<void> = Promise.resolve();

const filters = reactive<{
  keyword: string;
  type?: AdminMenuType;
}>({
  keyword: '',
  type: undefined,
});

const formState = reactive<AdminMenuPayload>({
  parentId: 0,
  type: undefined,
  order: 0,
  title: '',
  icon: '',
  uri: '',
  header: '',
  pluginName: '',
  uuid: '',
});

const rules = {
  parentId: [{ validator: validateParentMenu }],
  title: [{ required: true, message: '请输入菜单标题' }],
  type: [{ validator: validateMenuTypeChange }],
  uri: [{ validator: validateMenuURI }],
};

const typeOptions = [
  { label: '菜单', value: ADMIN_MENU_TYPE.MENU },
  { label: '外链', value: ADMIN_MENU_TYPE.EXTERNAL },
  { label: '目录/分组', value: ADMIN_MENU_TYPE.DIRECTORY },
];

const menuIconOptions = [
  'lucide:layout-dashboard',
  'lucide:home',
  'lucide:menu',
  'lucide:folder',
  'lucide:folder-open',
  'lucide:folder-kanban',
  'lucide:book-open',
  'lucide:file-text',
  'lucide:scroll-text',
  'lucide:database',
  'lucide:server',
  'lucide:settings',
  'lucide:settings-2',
  'lucide:sliders-horizontal',
  'lucide:shield-check',
  'lucide:lock-keyhole',
  'lucide:users',
  'lucide:user-cog',
  'lucide:mail',
  'lucide:bell',
  'lucide:message-square',
  'lucide:calendar-clock',
  'lucide:chart-no-axes-column',
  'lucide:area-chart',
  'lucide:external-link',
  'lucide:link',
  'lucide:workflow',
  'lucide:key-round',
  'lucide:tags',
  'lucide:list-tree',
  'lucide:package',
  'lucide:boxes',
  'lucide:upload',
  'lucide:download',
  'carbon:workspace',
  'carbon:api',
];
const iconPickerInput = Input as unknown as VNode;

const formTypeOptions = computed(() =>
  typeOptions.map((option) => ({
    ...option,
    disabled:
      option.value !== ADMIN_MENU_TYPE.DIRECTORY &&
      !canSetMenuAsItem(editingMenu.value),
  })),
);

const columns = [
  { title: '菜单', key: 'title', width: 260, fixed: 'left' },
  { title: '路径', key: 'uri', dataIndex: 'uri', width: 220 },
  { title: '类型', key: 'type', dataIndex: 'type', width: 96 },
  { title: '更新时间', key: 'updatedAt', dataIndex: 'updatedAt', width: 168 },
  { title: '操作', key: 'action', width: 196, fixed: 'right' },
];

const filteredMenus = computed(() => {
  const keyword = filters.keyword.trim().toLowerCase();
  return filterMenuTree(menus.value, (menu) => {
    const matchKeyword =
      !keyword ||
      menu.title.toLowerCase().includes(keyword) ||
      menu.uri.toLowerCase().includes(keyword) ||
      menu.icon.toLowerCase().includes(keyword);
    const matchType = filters.type === undefined || menu.type === filters.type;
    return matchKeyword && matchType;
  });
});

const parentOptions = computed(() =>
  filterMenuParentOptions(menus.value, editingMenu.value?.id),
);
const hasSelectedMenuType = computed(() => formState.type !== undefined);
const isDirectoryType = computed(
  () => formState.type === ADMIN_MENU_TYPE.DIRECTORY,
);
const isMenuType = computed(() => formState.type === ADMIN_MENU_TYPE.MENU);
const isExternalType = computed(
  () => formState.type === ADMIN_MENU_TYPE.EXTERNAL,
);

onMounted(() => {
  void loadMenus();
});

async function loadMenus(): Promise<boolean> {
  loading.value = true;
  try {
    menus.value = await getAdminMenuTree();
    // a-table 的 defaultExpandAllRows 仅在挂载时生效，此时数据尚未返回；
    // 改为受控展开：数据到达后自动展开所有含子菜单的节点，保证子菜单可见。
    expandedRowKeys.value = collectExpandableKeys(menus.value);
    return true;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载菜单失败');
    return false;
  } finally {
    loading.value = false;
  }
}

async function refreshMenuData() {
  if (await loadMenus()) {
    await queueNavigationRefresh();
  }
}

function onExpandedRowsChange(keys: (number | string)[]) {
  expandedRowKeys.value = keys as number[];
}

function applySearch() {
  // 菜单树为前端实时过滤，输入即生效，此处仅保留入口与其他列表页保持一致。
}

function resetSearch() {
  filters.keyword = '';
  filters.type = undefined;
}

function openDrawer(record?: AdminMenu, parent?: AdminMenu) {
  editingMenu.value = record || null;
  Object.assign(formState, {
    parentId: record?.parentId ?? parent?.id ?? 0,
    type: record?.type,
    order: record?.order ?? nextChildOrder(parent),
    title: record?.title ?? '',
    icon: record?.icon ?? '',
    uri: record?.uri ?? '',
    header: record?.header ?? '',
    pluginName: record?.pluginName ?? '',
    uuid: record?.uuid ?? '',
  });
  drawerOpen.value = true;
}

function handleMenuTypeChange(value: AdminMenuType) {
  if (value === ADMIN_MENU_TYPE.DIRECTORY) {
    clearPageFields();
  } else if (value === ADMIN_MENU_TYPE.EXTERNAL) {
    clearPageMetadata();
  }
}

function handleIconChange(icon: string) {
  formState.icon = icon;
}

function clearPageFields() {
  formState.uri = '';
  clearPageMetadata();
}

function clearPageMetadata() {
  formState.header = '';
  formState.pluginName = '';
  formState.uuid = '';
}

async function submitForm() {
  await formRef.value?.validate();
  saving.value = true;
  try {
    const payload = normalizePayload();
    await (editingMenu.value
      ? updateAdminMenu(editingMenu.value.id, payload)
      : createAdminMenu(payload));
    drawerOpen.value = false;
    message.success('菜单已保存');
    await loadMenus();
    await queueNavigationRefresh();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存菜单失败');
  } finally {
    saving.value = false;
  }
}

async function removeMenu(record: AdminMenu) {
  try {
    await deleteAdminMenu(record.id);
    message.success('菜单已删除');
    await loadMenus();
    await queueNavigationRefresh();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除菜单失败');
  }
}

async function saveMenuLayout(items: AdminMenuPosition[]) {
  layoutSaving.value = true;
  let saved = false;
  try {
    menus.value = await updateAdminMenuLayout(items);
    expandedRowKeys.value = collectExpandableKeys(menus.value);
    layoutEditorOpen.value = false;
    message.success('菜单布局已保存');
    saved = true;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存菜单布局失败');
  } finally {
    layoutSaving.value = false;
  }

  if (saved) {
    void queueNavigationRefresh();
  }
}

function queueNavigationRefresh(): Promise<void> {
  navigationRefreshQueue = navigationRefreshQueue.then(
    refreshNavigationMenusSafely,
    refreshNavigationMenusSafely,
  );
  return navigationRefreshQueue;
}

async function refreshNavigationMenusSafely() {
  try {
    await refreshNavigationMenus();
  } catch {
    message.warning('菜单数据已更新，导航刷新失败，请刷新页面后查看最新布局');
  }
}

async function refreshNavigationMenus() {
  await refreshNavigation({
    roles: userStore.userInfo?.roles ?? [],
    router,
  });
  const currentRoute = router.currentRoute.value;
  await router.replace({
    force: true,
    hash: currentRoute.hash,
    path: currentRoute.path,
    query: currentRoute.query,
  });
}

function normalizePayload(): AdminMenuPayload {
  const parentId = Number(formState.parentId) || 0;
  const parentChanged =
    editingMenu.value !== null && editingMenu.value.parentId !== parentId;
  const menuType = formState.type ?? ADMIN_MENU_TYPE.MENU;
  const includeURI = menuType !== ADMIN_MENU_TYPE.DIRECTORY;
  const includePageFields = menuType === ADMIN_MENU_TYPE.MENU;
  return {
    parentId,
    type: menuType,
    order: parentChanged
      ? nextChildOrder(findMenuById(menus.value, parentId))
      : Number(formState.order) || 0,
    title: formState.title.trim(),
    icon: formState.icon?.trim(),
    uri: includeURI ? formState.uri?.trim() : '',
    header: includePageFields ? formState.header?.trim() : '',
    pluginName: includePageFields ? formState.pluginName?.trim() : '',
    uuid: includePageFields ? formState.uuid?.trim() : '',
  };
}

function validateParentMenu(_rule: unknown, value: unknown): Promise<void> {
  const parentId = Number(value) || 0;
  if (parentId === 0) {
    return Promise.resolve();
  }
  const parent = findMenuById(menus.value, parentId);
  if (!parent || parent.type !== ADMIN_MENU_TYPE.DIRECTORY) {
    return Promise.reject(new Error('上级菜单必须是目录/分组'));
  }
  return Promise.resolve();
}

function validateMenuTypeChange(_rule: unknown, value: unknown): Promise<void> {
  if (value === undefined || value === null) {
    return Promise.reject(new Error('请选择菜单类型'));
  }
  if (
    Number(value) !== ADMIN_MENU_TYPE.DIRECTORY &&
    !canSetMenuAsItem(editingMenu.value)
  ) {
    return Promise.reject(new Error('存在子菜单的节点必须保持为目录/分组'));
  }
  return Promise.resolve();
}

function validateMenuURI(): Promise<void> {
  const uri = formState.uri?.trim() || '';
  if (formState.type === ADMIN_MENU_TYPE.EXTERNAL) {
    if (!/^https?:\/\//i.test(uri)) {
      return Promise.reject(
        new Error('外链地址必须以 http:// 或 https:// 开头'),
      );
    }
  } else if (formState.type === ADMIN_MENU_TYPE.MENU && !uri) {
    return Promise.reject(new Error('请输入访问路径'));
  }
  return Promise.resolve();
}

function findMenuById(items: AdminMenu[], id: number): AdminMenu | undefined {
  if (id === 0) {
    return undefined;
  }
  for (const item of items) {
    if (item.id === id) {
      return item;
    }
    const child = findMenuById(item.children ?? [], id);
    if (child) {
      return child;
    }
  }
}

function filterMenuTree(
  items: AdminMenu[],
  predicate: (menu: AdminMenu) => boolean,
): AdminMenu[] {
  const result: AdminMenu[] = [];
  for (const item of items) {
    const children = item.children
      ? filterMenuTree(item.children, predicate)
      : [];
    if (predicate(item) || children.length > 0) {
      result.push({
        ...item,
        children,
      });
    }
  }
  return result;
}

function collectExpandableKeys(items: AdminMenu[]): number[] {
  const keys: number[] = [];
  for (const item of items) {
    if (item.children && item.children.length > 0) {
      keys.push(item.id, ...collectExpandableKeys(item.children));
    }
  }
  return keys;
}

function nextChildOrder(parent?: AdminMenu) {
  const children = parent?.children || menus.value;
  let maxOrder = 0;
  for (const item of children) {
    maxOrder = Math.max(maxOrder, item.order);
  }
  return maxOrder + 1;
}

function menuRowClassName(record: AdminMenu) {
  if (!record.parentId) {
    return 'menu-row menu-row-root';
  }

  const depth = getMenuDepth(record.id, filteredMenus.value) ?? 1;
  return `menu-row menu-row-child menu-row-level-${Math.min(depth, 3)}`;
}

function typeText(type: AdminMenuType) {
  if (type === ADMIN_MENU_TYPE.MENU) return '菜单';
  if (type === ADMIN_MENU_TYPE.EXTERNAL) return '外链';
  return '目录/分组';
}

function typeColor(type: AdminMenuType) {
  if (type === ADMIN_MENU_TYPE.MENU) return 'blue';
  if (type === ADMIN_MENU_TYPE.EXTERNAL) return 'green';
  return 'default';
}

function getMenuDepth(
  id: number,
  items: AdminMenu[],
  depth = 0,
): number | undefined {
  for (const item of items) {
    if (item.id === id) {
      return depth;
    }

    if (item.children?.length) {
      const childDepth = getMenuDepth(id, item.children, depth + 1);
      if (childDepth !== undefined) {
        return childDepth;
      }
    }
  }
}
</script>

<style scoped>
:deep(.menu-row > td) {
  transition:
    background-color 0.24s ease,
    box-shadow 0.24s ease;
}

:deep(.menu-row-child) {
  animation: menu-child-slide-in 0.24s ease both;
}

:deep(.menu-row-child > td) {
  background-color: color-mix(
    in srgb,
    var(--kadmin-soft-bg) 60%,
    var(--kadmin-bg)
  );
}

:deep(.menu-row-level-2 > td) {
  background-color: color-mix(
    in srgb,
    var(--kadmin-soft-bg) 78%,
    var(--kadmin-bg)
  );
}

:deep(.menu-row-level-3 > td) {
  background-color: color-mix(
    in srgb,
    var(--kadmin-soft-bg) 94%,
    var(--kadmin-bg)
  );
}

:deep(.menu-row-child > td:first-child) {
  box-shadow: inset 3px 0 0
    color-mix(in srgb, var(--kadmin-active-text) 55%, var(--kadmin-border));
}

:deep(.menu-row-child:hover > td) {
  background-color: var(--kadmin-hover-bg) !important;
}

@keyframes menu-child-slide-in {
  from {
    opacity: 0;
    transform: translateY(-8px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
