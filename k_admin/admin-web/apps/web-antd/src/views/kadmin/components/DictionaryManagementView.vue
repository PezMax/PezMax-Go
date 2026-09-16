<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>字典管理</h1>
        <p>维护系统选项、状态和值域</p>
      </div>
    </section>

    <section class="panel">
      <a-form :model="filters" layout="inline" class="search-form">
        <a-form-item label="关键词">
          <a-input
            v-model:value="filters.keyword"
            allow-clear
            class="control-lg"
            placeholder="类型名称 / 编码 / 字典项"
            @press-enter="searchAll"
          >
            <template #prefix>
              <SearchOutlined />
            </template>
          </a-input>
        </a-form-item>
        <a-form-item label="状态">
          <a-select
            v-model:value="filters.status"
            allow-clear
            class="control-md"
            :options="statusOptions"
            placeholder="全部"
          />
        </a-form-item>
        <a-form-item>
          <a-space wrap>
            <a-button type="primary" @click="searchAll">
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

    <div class="dictionary-layout">
      <section class="panel dictionary-type-panel">
        <div class="table-toolbar">
          <span class="muted-text">字典类型</span>
          <a-space>
            <a-button
              v-if="canCreateType"
              type="primary"
              @click="openTypeDrawer()"
            >
              <PlusOutlined />
              新增
            </a-button>
            <a-button @click="loadTypes">
              <ReloadOutlined />
              刷新
            </a-button>
          </a-space>
        </div>

        <a-list
          class="dictionary-type-list"
          item-layout="vertical"
          :data-source="types"
          :loading="typeLoading"
        >
          <template #renderItem="{ item }">
            <a-list-item
              :class="[
                'dictionary-type-item',
                { active: item.code === selectedTypeCode },
              ]"
              @click="selectType(item)"
            >
              <div class="dictionary-type-title">
                <strong>{{ item.name }}</strong>
                <a-tag :color="statusColor(item.status)">{{
                  statusText(item.status)
                }}</a-tag>
              </div>
              <div class="dictionary-code">{{ item.code }}</div>
              <p>{{ item.description || '无描述' }}</p>
              <div class="dictionary-type-actions" @click.stop>
                <a-button
                  v-if="canUpdateType"
                  type="link"
                  size="small"
                  @click="openTypeDrawer(item)"
                  >编辑</a-button
                >
                <a-popconfirm
                  v-if="canDeleteType"
                  title="删除类型会同步删除字典项，确认继续？"
                  @confirm="removeType(item)"
                >
                  <a-button type="link" size="small" danger>删除</a-button>
                </a-popconfirm>
              </div>
            </a-list-item>
          </template>
        </a-list>
      </section>

      <section class="panel dictionary-data-panel">
        <div class="table-toolbar">
          <div>
            <strong>{{ selectedType?.name || '字典项' }}</strong>
            <p class="muted-text">
              {{ selectedType?.code || '请选择左侧字典类型' }}
            </p>
          </div>
          <a-space wrap>
            <a-button
              v-if="canCreateData"
              type="primary"
              :disabled="!selectedType"
              @click="openDataDrawer()"
            >
              <PlusOutlined />
              新增字典项
            </a-button>
            <a-button @click="loadData">
              <ReloadOutlined />
              刷新
            </a-button>
          </a-space>
        </div>

        <a-table
          row-key="id"
          size="small"
          class="compact-user-table"
          :columns="dataColumns"
          :data-source="dataItems"
          :loading="dataLoading"
          :pagination="dataPagination"
          :scroll="{ x: 960 }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'label'">
              <a-space>
                <a-tag :color="record.color || undefined">{{
                  record.label
                }}</a-tag>
                <a-tag v-if="record.isDefault" color="blue">默认</a-tag>
              </a-space>
            </template>

            <template v-else-if="column.key === 'status'">
              <a-tag :color="statusColor(record.status)">{{
                statusText(record.status)
              }}</a-tag>
            </template>

            <template v-else-if="column.key === 'action'">
              <a-space>
                <a-button
                  v-if="canUpdateData"
                  type="link"
                  size="small"
                  @click="openDataDrawer(record)"
                  >编辑</a-button
                >
                <a-popconfirm
                  v-if="canDeleteData"
                  title="确认删除该字典项？"
                  @confirm="removeData(record)"
                >
                  <a-button type="link" size="small" danger>删除</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </section>
    </div>

    <a-drawer
      v-model:open="typeDrawerOpen"
      :title="editingType ? '编辑字典类型' : '新增字典类型'"
      width="520"
      :destroy-on-close="true"
    >
      <a-form
        ref="typeFormRef"
        :model="typeForm"
        :rules="typeRules"
        layout="vertical"
      >
        <a-form-item label="类型名称" name="name">
          <a-input v-model:value="typeForm.name" placeholder="例如：性别" />
        </a-form-item>
        <a-form-item label="类型编码" name="code">
          <a-input
            v-model:value="typeForm.code"
            placeholder="例如：sys_gender"
          />
        </a-form-item>
        <a-form-item label="排序" name="sort">
          <a-input-number
            v-model:value="typeForm.sort"
            :min="0"
            class="full-width"
          />
        </a-form-item>
        <a-form-item label="状态" name="status">
          <a-radio-group v-model:value="typeForm.status">
            <a-radio-button :value="1">启用</a-radio-button>
            <a-radio-button :value="2">停用</a-radio-button>
          </a-radio-group>
        </a-form-item>
        <a-form-item label="描述" name="description">
          <a-textarea
            v-model:value="typeForm.description"
            :rows="4"
            placeholder="可选"
          />
        </a-form-item>
      </a-form>
      <template #extra>
        <a-space>
          <a-button @click="typeDrawerOpen = false">取消</a-button>
          <a-button
            type="primary"
            :disabled="editingType ? !canUpdateType : !canCreateType"
            :loading="savingType"
            @click="submitType"
            >保存</a-button
          >
        </a-space>
      </template>
    </a-drawer>

    <a-drawer
      v-model:open="dataDrawerOpen"
      :title="editingData ? '编辑字典项' : '新增字典项'"
      width="560"
      :destroy-on-close="true"
    >
      <a-form
        ref="dataFormRef"
        :model="dataForm"
        :rules="dataRules"
        layout="vertical"
      >
        <a-form-item label="所属类型" name="dictType">
          <a-select
            v-model:value="dataForm.dictType"
            :options="typeOptions"
            placeholder="请选择字典类型"
          />
        </a-form-item>
        <a-form-item label="标签" name="label">
          <a-input v-model:value="dataForm.label" placeholder="例如：Male" />
        </a-form-item>
        <a-form-item label="值" name="value">
          <a-input v-model:value="dataForm.value" placeholder="例如：male" />
        </a-form-item>
        <a-form-item label="颜色" name="color">
          <a-select
            v-model:value="dataForm.color"
            allow-clear
            :options="colorOptions"
            placeholder="选择标签颜色"
          />
        </a-form-item>
        <a-form-item label="CSS 类" name="cssClass">
          <a-input
            v-model:value="dataForm.cssClass"
            placeholder="可选，自定义前端样式类"
          />
        </a-form-item>
        <a-form-item label="排序" name="sort">
          <a-input-number
            v-model:value="dataForm.sort"
            :min="0"
            class="full-width"
          />
        </a-form-item>
        <a-form-item label="状态" name="status">
          <a-radio-group v-model:value="dataForm.status">
            <a-radio-button :value="1">启用</a-radio-button>
            <a-radio-button :value="2">停用</a-radio-button>
          </a-radio-group>
        </a-form-item>
        <a-form-item label="默认项" name="isDefault">
          <a-switch v-model:checked="dataForm.isDefault" />
        </a-form-item>
        <a-form-item label="备注" name="remark">
          <a-textarea
            v-model:value="dataForm.remark"
            :rows="4"
            placeholder="可选"
          />
        </a-form-item>
      </a-form>
      <template #extra>
        <a-space>
          <a-button @click="dataDrawerOpen = false">取消</a-button>
          <a-button
            type="primary"
            :disabled="editingData ? !canUpdateData : !canCreateData"
            :loading="savingData"
            @click="submitData"
            >保存</a-button
          >
        </a-space>
      </template>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import {
  ClearOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons-vue';
import { useAccess } from '@vben/access';
import { message } from 'ant-design-vue';
import type { FormInstance } from 'ant-design-vue';
import { computed, onMounted, reactive, ref } from 'vue';

import {
  createDictionaryData,
  createDictionaryType,
  deleteDictionaryData,
  deleteDictionaryType,
  getDictionaryData,
  getDictionaryTypes,
  updateDictionaryData,
  updateDictionaryType,
} from '#/api/kadmin/dictionaries';
import type { DictionaryData, DictionaryType } from '#/api/kadmin/dictionaries';
import { KADMIN_PERMISSION } from '#/api/kadmin/permissions';

const { hasAccessByCodes } = useAccess();
const canCreateType = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.DICTIONARY_TYPE_CREATE, '*']),
);
const canUpdateType = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.DICTIONARY_TYPE_UPDATE, '*']),
);
const canDeleteType = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.DICTIONARY_TYPE_DELETE, '*']),
);
const canCreateData = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.DICTIONARY_DATA_CREATE, '*']),
);
const canUpdateData = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.DICTIONARY_DATA_UPDATE, '*']),
);
const canDeleteData = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.DICTIONARY_DATA_DELETE, '*']),
);

const typeLoading = ref(false);
const dataLoading = ref(false);
const savingType = ref(false);
const savingData = ref(false);
const types = ref<DictionaryType[]>([]);
const dataItems = ref<DictionaryData[]>([]);
const selectedTypeCode = ref('');
const typeDrawerOpen = ref(false);
const dataDrawerOpen = ref(false);
const editingType = ref<DictionaryType | null>(null);
const editingData = ref<DictionaryData | null>(null);
const typeFormRef = ref<FormInstance>();
const dataFormRef = ref<FormInstance>();

const filters = reactive<{
  keyword: string;
  status?: number;
}>({
  keyword: '',
  status: undefined,
});

const typeForm = reactive({
  name: '',
  code: '',
  description: '',
  sort: 0,
  status: 1,
});

const dataForm = reactive({
  dictType: '',
  label: '',
  value: '',
  color: '',
  cssClass: '',
  isDefault: false,
  sort: 0,
  status: 1,
  remark: '',
});

const dataPagination = {
  pageSize: 10,
  showSizeChanger: true,
  showTotal: (total: number) => `共 ${total} 项`,
};

const dataColumns = [
  { title: '标签', key: 'label', width: 180, fixed: 'left' },
  { title: '值', dataIndex: 'value', key: 'value', width: 160 },
  { title: '排序', dataIndex: 'sort', key: 'sort', width: 90 },
  { title: '状态', key: 'status', width: 100 },
  { title: '备注', dataIndex: 'remark', key: 'remark', width: 220 },
  { title: '更新时间', dataIndex: 'updatedAt', key: 'updatedAt', width: 160 },
  { title: '操作', key: 'action', width: 140, fixed: 'right' },
];

const statusOptions = [
  { label: '启用', value: 1 },
  { label: '停用', value: 2 },
];

const colorOptions = [
  'blue',
  'green',
  'red',
  'orange',
  'purple',
  'magenta',
  'cyan',
  'gold',
].map((value) => ({
  label: value,
  value,
}));

const typeRules = {
  name: [{ required: true, message: '请输入类型名称' }],
  code: [{ required: true, message: '请输入类型编码' }],
};

const dataRules = {
  dictType: [{ required: true, message: '请选择字典类型' }],
  label: [{ required: true, message: '请输入标签' }],
  value: [{ required: true, message: '请输入值' }],
};

const selectedType = computed(() =>
  types.value.find((item) => item.code === selectedTypeCode.value),
);

const typeOptions = computed(() =>
  types.value.map((item) => ({
    label: `${item.name}（${item.code}）`,
    value: item.code,
  })),
);

onMounted(() => {
  void loadTypes();
});

async function loadTypes() {
  typeLoading.value = true;
  try {
    const data = await getDictionaryTypes({
      keyword: filters.keyword.trim(),
      status: filters.status,
    });
    types.value = data.items || [];
    if (!types.value.some((item) => item.code === selectedTypeCode.value)) {
      selectedTypeCode.value = types.value[0]?.code || '';
    }
    await loadData();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载字典类型失败');
  } finally {
    typeLoading.value = false;
  }
}

async function loadData() {
  dataLoading.value = true;
  try {
    const data = await getDictionaryData({
      dictType: selectedTypeCode.value,
      keyword: filters.keyword.trim(),
      status: filters.status,
    });
    dataItems.value = data.items || [];
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载字典项失败');
  } finally {
    dataLoading.value = false;
  }
}

function selectType(item: DictionaryType) {
  selectedTypeCode.value = item.code;
  void loadData();
}

function searchAll() {
  void loadTypes();
}

function resetSearch() {
  filters.keyword = '';
  filters.status = undefined;
  void loadTypes();
}

function openTypeDrawer(record?: DictionaryType) {
  editingType.value = record || null;
  typeForm.name = record?.name || '';
  typeForm.code = record?.code || '';
  typeForm.description = record?.description || '';
  typeForm.sort = record?.sort || 0;
  typeForm.status = record?.status || 1;
  typeDrawerOpen.value = true;
}

async function submitType() {
  await typeFormRef.value?.validate();
  savingType.value = true;
  try {
    const payload = {
      name: typeForm.name.trim(),
      code: typeForm.code.trim(),
      description: typeForm.description.trim(),
      sort: typeForm.sort,
      status: typeForm.status,
    };
    const oldCode = editingType.value?.code;
    await (editingType.value
      ? updateDictionaryType(editingType.value.id, payload)
      : createDictionaryType(payload));
    typeDrawerOpen.value = false;
    selectedTypeCode.value = payload.code || oldCode || selectedTypeCode.value;
    await loadTypes();
    message.success('字典类型已保存');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存字典类型失败');
  } finally {
    savingType.value = false;
  }
}

async function removeType(record: DictionaryType) {
  try {
    await deleteDictionaryType(record.id);
    if (selectedTypeCode.value === record.code) {
      selectedTypeCode.value = '';
    }
    await loadTypes();
    message.success('字典类型已删除');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除字典类型失败');
  }
}

function openDataDrawer(record?: DictionaryData) {
  editingData.value = record || null;
  dataForm.dictType = record?.dictType || selectedTypeCode.value;
  dataForm.label = record?.label || '';
  dataForm.value = record?.value || '';
  dataForm.color = record?.color || '';
  dataForm.cssClass = record?.cssClass || '';
  dataForm.isDefault = Boolean(record?.isDefault);
  dataForm.sort = record?.sort || 0;
  dataForm.status = record?.status || 1;
  dataForm.remark = record?.remark || '';
  dataDrawerOpen.value = true;
}

async function submitData() {
  await dataFormRef.value?.validate();
  savingData.value = true;
  try {
    const payload = {
      dictType: dataForm.dictType.trim(),
      label: dataForm.label.trim(),
      value: dataForm.value.trim(),
      color: dataForm.color.trim(),
      cssClass: dataForm.cssClass.trim(),
      isDefault: dataForm.isDefault,
      sort: dataForm.sort,
      status: dataForm.status,
      remark: dataForm.remark.trim(),
    };
    await (editingData.value
      ? updateDictionaryData(editingData.value.id, payload)
      : createDictionaryData(payload));
    dataDrawerOpen.value = false;
    selectedTypeCode.value = payload.dictType;
    await loadData();
    message.success('字典项已保存');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存字典项失败');
  } finally {
    savingData.value = false;
  }
}

async function removeData(record: DictionaryData) {
  try {
    await deleteDictionaryData(record.id);
    await loadData();
    message.success('字典项已删除');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除字典项失败');
  }
}

function statusColor(status: number) {
  return status === 1 ? 'green' : 'default';
}

function statusText(status: number) {
  return status === 1 ? '启用' : '停用';
}
</script>
