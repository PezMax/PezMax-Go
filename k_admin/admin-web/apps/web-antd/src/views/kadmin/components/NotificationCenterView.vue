<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>站内通知</h1>
        <p>系统消息中心 · 未读 {{ unread }} 条</p>
      </div>
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
      <a-form :model="filters" layout="inline" class="search-form">
        <a-form-item label="状态">
          <a-select
            v-model:value="filters.status"
            class="control-md"
            :options="statusOptions"
          />
        </a-form-item>
        <a-form-item>
          <a-space wrap>
            <a-button type="primary" @click="searchNotifications">
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
        <span class="muted-text">按状态筛选消息，并在行内维护单条消息</span>
        <a-space wrap>
          <a-button :loading="loading" @click="fetchList">
            <ReloadOutlined />
            刷新
          </a-button>
          <a-button
            v-if="canReadAll"
            :disabled="unread <= 0"
            @click="markAllRead"
          >
            <CheckOutlined />
            全部已读
          </a-button>
          <a-popconfirm
            v-if="canClearRead"
            title="确认清空全部已读通知？"
            ok-type="danger"
            @confirm="clearRead"
          >
            <a-button danger>
              <DeleteOutlined />
              清空已读
            </a-button>
          </a-popconfirm>
        </a-space>
      </div>

      <a-table
        row-key="id"
        class="compact-user-table"
        size="small"
        :columns="columns"
        :data-source="items"
        :loading="loading"
        :pagination="pagination"
        :scroll="{ x: 960 }"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'title'">
            <a-space :size="6">
              <a-badge v-if="!record.isRead" status="processing" />
              <strong>{{ record.title }}</strong>
            </a-space>
            <div class="notification-content">{{ record.content }}</div>
          </template>
          <template v-else-if="column.key === 'type'">
            <a-tag :color="typeMeta(record.type).color">
              {{ typeMeta(record.type).label }}
            </a-tag>
          </template>
          <template v-else-if="column.key === 'createdAt'">
            {{ formatDateTime(record.createdAt) }}
          </template>
          <template v-else-if="column.key === 'link'">
            <a v-if="record.link" :href="record.link" target="_blank">
              {{ record.link }}
            </a>
            <span v-else>-</span>
          </template>
          <template v-else-if="column.key === 'isRead'">
            <a-tag :color="record.isRead ? 'default' : 'blue'">
              {{ record.isRead ? '已读' : '未读' }}
            </a-tag>
          </template>
          <template v-else-if="column.key === 'action'">
            <a-space :size="2">
              <a-button
                v-if="!record.isRead && canRead"
                type="link"
                size="small"
                @click="markRead(record)"
              >
                标记已读
              </a-button>
              PUT 116.=118:
              <a-popconfirm
                v-if="canDelete"
                title="确认删除该通知？"
                @confirm="remove(record)"
              >
                <a-button type="link" size="small" danger>删除</a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import {
  CheckOutlined,
  ClearOutlined,
  DeleteOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons-vue';
import { useAccess } from '@vben/access';
import { message } from 'ant-design-vue';
import { computed, onMounted, reactive, ref } from 'vue';

import {
  clearReadNotifications,
  deleteNotification,
  getNotifications,
  markAllNotificationsRead,
  markNotificationRead,
} from '#/api/kadmin/notifications';
import type {
  KadminNotification,
  NotificationType,
} from '#/api/kadmin/notifications';
import { KADMIN_PERMISSION } from '#/api/kadmin/permissions';

type TablePagination = { current?: number; pageSize?: number };
type NotificationStatusFilter = 'all' | 'unread';
const { hasAccessByCodes } = useAccess();
const canRead = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.NOTIFICATION_READ, '*']),
);
const canDelete = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.NOTIFICATION_DELETE, '*']),
);
const canReadAll = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.NOTIFICATION_READ_ALL, '*']),
);
const canClearRead = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.NOTIFICATION_CLEAR_READ, '*']),
);

const loading = ref(false);
const errorText = ref('');
const items = ref<KadminNotification[]>([]);
const unread = ref(0);
const page = ref(1);
const pageSize = ref(20);
const filters = reactive<{ status: NotificationStatusFilter }>({
  status: 'all',
});

const columns = [
  { title: '通知', key: 'title', width: 320, fixed: 'left' },
  { title: '类型', key: 'type', width: 90 },
  { title: '跳转', key: 'link', width: 180 },
  { title: '时间', key: 'createdAt', width: 170 },
  { title: '状态', key: 'isRead', width: 80 },
  { title: '操作', key: 'action', width: 130, fixed: 'right' },
];

const pagination = reactive({
  current: 1,
  pageSize: 20,
  pageSizeOptions: ['10', '20', '50', '100'],
  showSizeChanger: true,
  showTotal: (total: number) => `共 ${total} 条`,
  total: 0,
});
const statusOptions = [
  { label: '全部消息', value: 'all' },
  { label: '仅看未读', value: 'unread' },
];

function typeMeta(type: NotificationType) {
  switch (type) {
    case 'success': {
      return { color: 'green', label: '成功' };
    }
    case 'warning': {
      return { color: 'orange', label: '警告' };
    }
    default: {
      return { color: 'blue', label: '信息' };
    }
  }
}

async function fetchList() {
  loading.value = true;
  errorText.value = '';
  try {
    const result = await getNotifications({
      page: page.value,
      pageSize: pageSize.value,
      unreadOnly: filters.status === 'unread' ? true : undefined,
    });
    items.value = result.items;
    unread.value = result.unread;
    pagination.total = result.total;
    pagination.current = result.page;
    pagination.pageSize = result.pageSize;
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : '加载失败';
  } finally {
    loading.value = false;
  }
}

function searchNotifications() {
  page.value = 1;
  pagination.current = 1;
  void fetchList();
}

function resetSearch() {
  filters.status = 'all';
  searchNotifications();
}

function handleTableChange(paginationValue: TablePagination) {
  page.value = paginationValue.current ?? 1;
  pageSize.value = paginationValue.pageSize ?? 20;
  void fetchList();
}

async function markRead(record: KadminNotification) {
  try {
    await markNotificationRead(record.id);
    await fetchList();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '标记已读失败');
  }
}

async function markAllRead() {
  try {
    await markAllNotificationsRead();
    message.success('已全部标记为已读');
    await fetchList();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '标记已读失败');
  }
}

async function clearRead() {
  try {
    await clearReadNotifications();
    message.success('已清空已读通知');
    await fetchList();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '清空失败');
  }
}

async function remove(record: KadminNotification) {
  try {
    await deleteNotification(record.id);
    await fetchList();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除失败');
  }
}

function formatDateTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value || '-' : date.toLocaleString();
}

onMounted(() => {
  void fetchList();
});
</script>

<style scoped>
.notification-content {
  margin-top: 4px;
  color: var(--kadmin-muted);
  font-size: 12px;
}
</style>
