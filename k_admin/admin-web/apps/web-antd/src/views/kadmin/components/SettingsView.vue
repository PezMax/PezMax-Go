<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>参数配置</h1>
        <p>Key-Value 系统配置项</p>
      </div>
      <a-space wrap>
        <a-button :loading="initialLoading" @click="loadSettings">
          <ReloadOutlined />
          重新加载
        </a-button>
        <a-button
          v-if="canUpdate"
          type="primary"
          :loading="submitLoading"
          @click="submit"
        >
          <SaveOutlined />
          保存
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
            placeholder="配置键 / 名称"
          >
            <template #prefix>
              <SearchOutlined />
            </template>
          </a-input>
        </a-form-item>
        <a-form-item label="类型">
          <a-select
            v-model:value="filters.scope"
            allow-clear
            class="control-md"
            :options="scopeOptions"
            placeholder="全部"
          />
        </a-form-item>
        <a-form-item>
          <a-space wrap>
            <a-button type="primary" @click="search">
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
        <div class="settings-file">
          <span class="muted-text">本地文件</span>
          <strong>{{ filePath || '-' }}</strong>
        </div>
        <a-space wrap>
          <a-button v-if="canUpdate" @click="openCustomModal">
            <PlusOutlined />
            新增
          </a-button>
          <a-button v-if="canUpdate" @click="restoreDefaults">
            <UndoOutlined />
            恢复默认
          </a-button>
        </a-space>
      </div>

      <a-alert
        v-if="apiError"
        class="form-alert"
        type="error"
        show-icon
        closable
        :message="apiError"
        @close="apiError = ''"
      />

      <a-table
        row-key="key"
        class="compact-user-table"
        size="small"
        :columns="columns"
        :data-source="filteredItems"
        :loading="initialLoading"
        :pagination="pagination"
        :scroll="{ x: 960 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'key'">
            <div class="config-key-cell">
              <strong>{{ record.label || record.key }}</strong>
              <span>{{ record.key }}</span>
            </div>
          </template>

          <template v-else-if="column.key === 'value'">
            <a-switch
              v-if="record.type === 'boolean'"
              :checked="record.value === 'true'"
              :disabled="!canUpdate"
              checked-children="开"
              un-checked-children="关"
              @change="updateBooleanValue(record.key, $event)"
            />
            <a-select
              v-else-if="record.type === 'select'"
              class="config-value-control"
              :value="record.value"
              :disabled="!canUpdate"
              :options="optionItems(record)"
              @change="updateSelectValue(record.key, $event)"
            />
            <a-input-password
              v-else-if="record.type === 'password'"
              class="config-value-control"
              :value="record.value"
              :disabled="!canUpdate"
              autocomplete="new-password"
              @change="updateInputValue(record.key, $event)"
            />
            <a-input
              v-else
              class="config-value-control"
              :disabled="!canUpdate"
              :value="record.value"
              @change="updateInputValue(record.key, $event)"
            />
          </template>

          <template v-else-if="column.key === 'status'">
            <a-space wrap>
              <a-tag :color="record.builtin ? 'blue' : 'default'">
                {{ record.builtin ? '内置' : '自定义' }}
              </a-tag>
            </a-space>
          </template>

          <template v-else-if="column.key === 'description'">
            <span class="muted-text">{{ record.description || '-' }}</span>
          </template>

          <template v-else-if="column.key === 'action'">
            <a-space>
              <a-button
                v-if="canUpdate"
                type="link"
                size="small"
                @click="resetItem(record)"
              >
                默认值
              </a-button>
              <a-popconfirm
                v-if="canUpdate && !record.builtin"
                title="确认删除该配置项？"
                @confirm="removeCustomItem(record)"
              >
                <a-button type="link" size="small" danger>删除</a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </section>

    <a-modal
      v-model:open="customModalOpen"
      title="新增配置项"
      :confirm-loading="customSaving"
      @ok="addCustomItem"
    >
      <a-form
        ref="customFormRef"
        :model="customForm"
        :rules="customRules"
        layout="vertical"
      >
        <a-form-item label="配置键" name="key">
          <a-input
            v-model:value="customForm.key"
            placeholder="例如：site.notice"
          />
        </a-form-item>
        <a-form-item label="配置值" name="value">
          <a-input v-model:value="customForm.value" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import {
  ClearOutlined,
  PlusOutlined,
  ReloadOutlined,
  SaveOutlined,
  SearchOutlined,
  UndoOutlined,
} from '@ant-design/icons-vue';
import { useAccess } from '@vben/access';
import type { ThemeModeType } from '@vben/types';
import { updatePreferences } from '@vben/preferences';
import { useUserStore } from '@vben/stores';
import { message } from 'ant-design-vue';
import type { FormInstance } from 'ant-design-vue';
import { computed, onMounted, reactive, ref } from 'vue';

import { getSystemConfig, updateSystemConfig } from '#/api/kadmin/systemConfig';
import type { SystemConfigItem } from '#/api/kadmin/systemConfig';
import { KADMIN_PERMISSION } from '#/api/kadmin/permissions';
import { router } from '#/router';
import { refreshNavigation } from '#/router/access';

const { hasAccessByCodes } = useAccess();
const canUpdate = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.SYSTEM_CONFIG_UPDATE, '*']),
);

const defaultValues: Record<string, string> = {
  'auth.default_username': 'admin',
  'auth.default_password': 'admin',
  'security.captcha_enabled': 'false',
  'security.captcha_ttl_seconds': '120',
  'security.login_lock_enabled': 'true',
  'security.login_failure_threshold': '5',
  'security.login_ip_failure_threshold': '20',
  'security.login_failure_window_minutes': '15',
  'security.login_lock_minutes': '15',
  'security.login_ip_whitelist': '127.0.0.1,::1',
  'security.idempotency_ttl_seconds': '300',
  'ui.theme_mode': 'auto',
  'navigation.external_link_target': 'new_tab',
};

const themeModes = new Set<ThemeModeType>(['auto', 'light', 'dark']);

const initialLoading = ref(false);
const submitLoading = ref(false);
const apiError = ref('');
const filePath = ref('');
const items = ref<SystemConfigItem[]>([]);
const customModalOpen = ref(false);
const customSaving = ref(false);
const customFormRef = ref<FormInstance>();
const userStore = useUserStore();

const filters = reactive<{
  keyword: string;
  scope?: 'builtin' | 'custom';
}>({
  keyword: '',
  scope: undefined,
});

const customForm = reactive({
  key: '',
  value: '',
});

const scopeOptions = [
  { label: '内置', value: 'builtin' },
  { label: '自定义', value: 'custom' },
];

const columns = [
  { title: '配置项', key: 'key', width: 260, fixed: 'left' },
  { title: '配置值', key: 'value', width: 280 },
  { title: '状态', key: 'status', width: 170 },
  { title: '备注', key: 'description', width: 220 },
  { title: '操作', key: 'action', width: 150, fixed: 'right' },
];

const pagination = {
  pageSize: 10,
  showSizeChanger: true,
  showTotal: (total: number) => `共 ${total} 项`,
};

const customRules = {
  key: [
    { required: true, message: '请输入配置键' },
    {
      pattern: /^[A-Za-z0-9._:-]+$/,
      message: '仅支持字母、数字、点、下划线、短横线和冒号',
    },
  ],
};

const filteredItems = computed(() => {
  const keyword = filters.keyword.trim().toLowerCase();
  return items.value.filter((item) => {
    if (filters.scope === 'builtin' && !item.builtin) return false;
    if (filters.scope === 'custom' && item.builtin) return false;
    if (!keyword) return true;
    return `${item.key} ${item.label || ''} ${item.description || ''}`
      .toLowerCase()
      .includes(keyword);
  });
});

onMounted(() => {
  void loadSettings();
});

async function loadSettings() {
  initialLoading.value = true;
  apiError.value = '';
  try {
    const data = await getSystemConfig();
    filePath.value = data.filePath;
    items.value = data.items || [];
  } catch (error) {
    apiError.value =
      error instanceof Error ? error.message : '加载参数配置失败';
  } finally {
    initialLoading.value = false;
  }
}

async function submit() {
  apiError.value = '';
  submitLoading.value = true;
  try {
    const data = await updateSystemConfig(items.value);
    filePath.value = data.filePath;
    items.value = data.items || [];
    syncThemeModePreference(items.value);
    message.success('参数配置已保存');
    await refreshNavigationMenusSafely();
  } catch (error) {
    apiError.value =
      error instanceof Error ? error.message : '保存参数配置失败';
  } finally {
    submitLoading.value = false;
  }
}

async function refreshNavigationMenusSafely() {
  try {
    await refreshNavigation({
      roles: userStore.userInfo?.roles ?? [],
      router,
    });
  } catch {
    message.warning('配置已保存，导航刷新失败，请刷新页面后生效');
  }
}

function search() {
  filters.keyword = filters.keyword.trim();
}

function resetSearch() {
  filters.keyword = '';
  filters.scope = undefined;
}

function updateItemValue(key: string, value: string) {
  const item = items.value.find((entry) => entry.key === key);
  if (item) {
    item.value = value;
  }
}

function updateBooleanValue(key: string, checked: boolean | string | number) {
  updateItemValue(key, checked ? 'true' : 'false');
}

function updateSelectValue(key: string, value: string | number) {
  updateItemValue(key, String(value));
}

function updateInputValue(key: string, event: Event) {
  const target = event.target as HTMLInputElement | null;
  updateItemValue(key, target?.value || '');
}

function optionItems(item: SystemConfigItem) {
  const labels: Record<string, string> = {
    auto: '跟随电脑主题',
    current_page: '当前页面跳转',
    dark: '黑夜',
    light: '白天',
    new_tab: '新标签页跳转',
  };
  return (item.options || []).map((value) => ({
    label: labels[value] || value,
    value,
  }));
}

function syncThemeModePreference(configItems: SystemConfigItem[]) {
  const mode = configItems.find((item) => item.key === 'ui.theme_mode')?.value;
  if (!mode || !themeModes.has(mode as ThemeModeType)) {
    return;
  }
  updatePreferences({ theme: { mode: mode as ThemeModeType } });
}

function resetItem(record: SystemConfigItem) {
  updateItemValue(record.key, defaultValues[record.key] ?? '');
}

function restoreDefaults() {
  items.value = items.value.map((item) => ({
    ...item,
    value: defaultValues[item.key] ?? item.value,
  }));
}

function openCustomModal() {
  customForm.key = '';
  customForm.value = '';
  customModalOpen.value = true;
}

async function addCustomItem() {
  await customFormRef.value?.validate();
  const key = customForm.key.trim();
  if (items.value.some((item) => item.key === key)) {
    message.error('配置键已存在');
    return;
  }

  customSaving.value = true;
  items.value = [
    ...items.value,
    {
      key,
      value: customForm.value,
      label: key,
      type: 'text',
      builtin: false,
    },
  ];
  customSaving.value = false;
  customModalOpen.value = false;
}

function removeCustomItem(record: SystemConfigItem) {
  items.value = items.value.filter((item) => item.key !== record.key);
}
</script>
