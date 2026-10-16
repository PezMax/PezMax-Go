<!-- Composite-key favorites are maintained manually; generic codegen is single-key. -->
<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>电子书收藏</h1>
        <p>按电子书和用户管理收藏记录</p>
      </div>
      <a-space wrap>
        <a-button :loading="loading" @click="fetchList">
          <ReloadOutlined />
          刷新
        </a-button>
        <a-button v-if="canCreate" type="primary" @click="openCreate">
          <PlusOutlined />
          新增
        </a-button>
      </a-space>
    </section>

    <a-alert
      v-if="errorText"
      class="form-alert"
      type="error"
      show-icon
      closable
      :message="errorText"
      @close="errorText = ''"
    />

    <section class="panel">
      <div class="table-toolbar">
        <a-tag color="blue">共 {{ total }} 条</a-tag>
      </div>
      <a-table
        :row-key="ebookFavoriteKey"
        class="compact-user-table"
        size="small"
        :columns="columns"
        :data-source="items"
        :loading="loading"
        :pagination="pagination"
        :scroll="{ x: 1100 }"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'ebookId'">
            <a-typography-text
              :content="record.ebookId ?? '-'"
              :ellipsis="{ tooltip: record.ebookId ?? '-' }"
            />
          </template>
          <template v-else-if="column.key === 'userId'">
            <a-typography-text
              :content="record.userId ?? '-'"
              :ellipsis="{ tooltip: record.userId ?? '-' }"
            />
          </template>
          <template v-else-if="column.key === 'action'">
            <a-space :size="2">
              <a-tooltip v-if="canUpdate" title="编辑">
                <a-button type="text" shape="circle" @click="openEdit(record)">
                  <EditOutlined />
                </a-button>
              </a-tooltip>
              <a-popconfirm
                v-if="canDelete"
                title="确认删除该记录？"
                @confirm="remove(record)"
              >
                <a-tooltip title="删除">
                  <a-button type="text" shape="circle" danger>
                    <DeleteOutlined />
                  </a-button>
                </a-tooltip>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </section>

    <a-modal
      :open="modalOpen"
      :title="editingId ? '编辑电子书收藏' : '新增电子书收藏'"
      :confirm-loading="submitting"
      :mask-closable="false"
      :destroy-on-close="true"
      width="560px"
      @ok="submit"
      @cancel="requestClose"
    >
      <a-form ref="formRef" :model="formState" :rules="rules" layout="vertical">
        <a-form-item name="ebookId" label="电子书 ID">
          <a-input-number
            v-model:value="formState.ebookId"
            :disabled="editingId !== null"
            :min="1"
            :precision="0"
            style="width: 100%"
            placeholder="请输入电子书 ID"
            @change="markDirty"
          />
        </a-form-item>
        <a-form-item name="userId" label="用户 ID">
          <a-input-number
            v-model:value="formState.userId"
            :min="1"
            :precision="0"
            style="width: 100%"
            placeholder="请输入User Id"
            @change="markDirty"
          />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  ReloadOutlined,
} from '@ant-design/icons-vue';
import { useAccess } from '@vben/access';
import { message, Modal, type FormInstance } from 'ant-design-vue';
import { computed, onMounted, reactive, ref } from 'vue';

import {
  ebookFavoriteKey,
  createEbookFavorite,
  deleteEbookFavorite,
  getEbookFavoriteList,
  updateEbookFavorite,
  type EbookFavorite,
  type EbookFavoritePayload,
} from '#/api/kadmin/generated/ebook_favorite';

type TablePagination = { current?: number; pageSize?: number };

const { hasAccessByCodes } = useAccess();
const canCreate = computed(() => hasAccessByCodes(['system:ebook_favorite:create', '*']));
const canUpdate = computed(() => hasAccessByCodes(['system:ebook_favorite:update', '*']));
const canDelete = computed(() => hasAccessByCodes(['system:ebook_favorite:delete', '*']));

const loading = ref(false);
const errorText = ref('');
const items = ref<EbookFavorite[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);

const columns = [
  { title: 'Ebook Id', dataIndex: 'ebookId', key: 'ebookId', width: 160, ellipsis: true },
  { title: 'User Id', dataIndex: 'userId', key: 'userId', width: 160, ellipsis: true },
  { title: '操作', key: 'action', width: 120, fixed: 'right' },
];

const pagination = computed(() => ({
  current: page.value,
  pageSize: pageSize.value,
  pageSizeOptions: ['10', '20', '50', '100'],
  showSizeChanger: true,
  showTotal: (value: number) => `共 ${value} 条`,
  total: total.value,
}));

async function fetchList() {
  loading.value = true;
  errorText.value = '';
  try {
    const result = await getEbookFavoriteList({
      page: page.value,
      pageSize: pageSize.value,
    });
    items.value = result.items;
    total.value = result.total;
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : '加载失败';
  } finally {
    loading.value = false;
  }
}

function handleTableChange(paginationValue: TablePagination) {
  page.value = paginationValue.current ?? 1;
  pageSize.value = paginationValue.pageSize ?? 20;
  void fetchList();
}

const modalOpen = ref(false);
const submitting = ref(false);
const editingId = ref<null | EbookFavorite>(null);
const formRef = ref<FormInstance>();
const dirty = ref(false);

const formState = reactive<Record<string, any>>({
  ebookId: undefined,
  userId: undefined,
});

const rules: Record<string, unknown> = {
  ebookId: [{ required: true, type: 'number', min: 1, message: '请输入有效资源 ID' }],
  userId: [{ required: true, type: 'number', min: 1, message: '请输入有效用户 ID' }],
};

function openCreate() {
  editingId.value = null;
  dirty.value = false;
  formState.ebookId = undefined;
  formState.userId = undefined;
  modalOpen.value = true;
}

async function openEdit(record: EbookFavorite) {
  editingId.value = { ebookId: record.ebookId, userId: record.userId };
  formState.ebookId = record.ebookId;
  dirty.value = false;
  formState.userId = record.userId;
  modalOpen.value = true;
}

function requestClose() {
  if (!dirty.value) {
    modalOpen.value = false;
    return;
  }
  Modal.confirm({
    title: '放弃修改？',
    content: '当前表单有未保存的修改。',
    okText: '放弃',
    okType: 'danger',
    cancelText: '继续编辑',
    onOk: () => {
      dirty.value = false;
      modalOpen.value = false;
    },
  });
}

function markDirty() {
  if (modalOpen.value) {
    dirty.value = true;
  }
}

async function submit() {
  try {
    await formRef.value?.validate();
  } catch {
    return;
  }
  submitting.value = true;
  try {
    const payload = {
      ebookId: formState.ebookId,
      userId: formState.userId,
    } as EbookFavoritePayload;
    if (editingId.value === null) {
      await createEbookFavorite(payload);
      message.success('新增成功');
    } else {
      await updateEbookFavorite(editingId.value, payload);
      message.success('修改成功');
    }
    modalOpen.value = false;
    dirty.value = false;
    await fetchList();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '提交失败');
  } finally {
    submitting.value = false;
  }
}

async function remove(record: EbookFavorite) {
  try {
    await deleteEbookFavorite(record);
    message.success('删除成功');
    await fetchList();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除失败');
  }
}

onMounted(() => {
  void fetchList();
});
</script>
