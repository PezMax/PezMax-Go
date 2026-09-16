<script lang="ts" setup>
import type { NotificationItem } from '@vben/layouts';

import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';

import { AuthenticationLoginExpiredModal } from '@vben/common-ui';
import { VBEN_DOC_URL, VBEN_GITHUB_URL } from '@vben/constants';
import { useWatermark } from '@vben/hooks';
import { BookOpenText, CircleHelp, SvgGithubIcon } from '@vben/icons';
import {
  BasicLayout,
  LockScreen,
  Notification,
  setExternalNavigationGuard,
  UserDropdown,
} from '@vben/layouts';
import { preferences, usePreferences } from '@vben/preferences';
import { useAccessStore, useUserStore } from '@vben/stores';
import { openWindow } from '@vben/utils';
import { message, Modal } from 'ant-design-vue';

import {
  clearReadNotifications,
  deleteNotification,
  getNotifications,
  markAllNotificationsRead,
  markNotificationRead,
} from '#/api/kadmin/notifications';
import type { KadminNotification } from '#/api/kadmin/notifications';
import { $t } from '#/locales';
import { useAuthStore } from '#/store';
import LoginForm from '#/views/_core/authentication/login.vue';

const notifications = ref<NotificationItem[]>([]);
const unreadCount = ref(0);

function toNotificationItem(item: KadminNotification): NotificationItem {
  return {
    id: item.id,
    avatar: preferences.app.defaultAvatar,
    date: relativeDate(item.createdAt),
    isRead: item.isRead,
    message: item.content,
    title: item.title,
    link: item.link || undefined,
  };
}

function relativeDate(value: string): string {
  if (!value) {
    return '';
  }
  const parsed = new Date(value.replace(' ', 'T'));
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  const diffMs = Date.now() - parsed.getTime();
  const minutes = Math.floor(diffMs / 60_000);
  if (minutes < 1) {
    return '刚刚';
  }
  if (minutes < 60) {
    return `${minutes}分钟前`;
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return `${hours}小时前`;
  }
  const days = Math.floor(hours / 24);
  if (days < 7) {
    return `${days}天前`;
  }
  return value.slice(0, 16);
}

async function fetchNotifications() {
  try {
    const page = await getNotifications({ page: 1, pageSize: 20 });
    notifications.value = page.items.map(toNotificationItem);
    unreadCount.value = page.unread;
  } catch {
    // 铃铛拉取失败保持静默，避免轮询反复打扰用户。
  }
}

let pollTimer: null | ReturnType<typeof setInterval> = null;

const router = useRouter();
const userStore = useUserStore();
const authStore = useAuthStore();
const accessStore = useAccessStore();
const { destroyWatermark, updateWatermark } = useWatermark();
const { isDark } = usePreferences();
const canReadNotification = computed(
  () =>
    accessStore.accessCodes.includes('system:notification:read') ||
    accessStore.accessCodes.includes('*'),
);
const canDeleteNotification = computed(
  () =>
    accessStore.accessCodes.includes('system:notification:delete') ||
    accessStore.accessCodes.includes('*'),
);
const canClearNotifications = computed(
  () =>
    accessStore.accessCodes.includes('system:notification:clear-read') ||
    accessStore.accessCodes.includes('*'),
);
const canReadAllNotifications = computed(
  () =>
    accessStore.accessCodes.includes('system:notification:read-all') ||
    accessStore.accessCodes.includes('*'),
);
const showDot = computed(() => unreadCount.value > 0);

const removeExternalNavigationGuard = setExternalNavigationGuard(
  ({ openInNewWindow, title, url }) =>
    new Promise<boolean>((resolve) => {
      let settled = false;
      const settle = (value: boolean) => {
        if (settled) return;
        settled = true;
        resolve(value);
      };
      Modal.confirm({
        cancelText: '取消',
        content: openInNewWindow
          ? `即将在新标签页打开：${url}`
          : `当前页面将离开系统并访问：${url}`,
        okText: '继续访问',
        onCancel: () => settle(false),
        onOk: () => settle(true),
        afterClose: () => settle(false),
        title: title ? `确认访问“${title}”？` : '确认访问外部链接？',
      });
    }),
);

onBeforeUnmount(() => {
  removeExternalNavigationGuard();
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
});

onMounted(() => {
  void fetchNotifications();
  pollTimer = setInterval(() => {
    void fetchNotifications();
  }, 60_000);
});

const menus = computed(() => [
  {
    handler: () => {
      router.push({ name: 'Profile' });
    },
    icon: 'lucide:user',
    text: $t('page.auth.profile'),
  },
  {
    handler: () => {
      openWindow(VBEN_DOC_URL, {
        target: '_blank',
      });
    },
    icon: BookOpenText,
    text: $t('ui.widgets.document'),
  },
  {
    handler: () => {
      openWindow(VBEN_GITHUB_URL, {
        target: '_blank',
      });
    },
    icon: SvgGithubIcon,
    text: 'GitHub',
  },
  {
    handler: () => {
      openWindow(`${VBEN_GITHUB_URL}/issues`, {
        target: '_blank',
      });
    },
    icon: CircleHelp,
    text: $t('ui.widgets.qa'),
  },
]);

const avatar = computed(() => {
  return userStore.userInfo?.avatar ?? preferences.app.defaultAvatar;
});

async function handleLogout() {
  await authStore.logout(false);
}

async function handleNoticeClear() {
  try {
    await clearReadNotifications();
    await fetchNotifications();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '清空失败');
  }
}

async function markRead(id: number | string) {
  try {
    await markNotificationRead(Number(id));
    await fetchNotifications();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '标记已读失败');
  }
}

async function remove(id: number | string) {
  try {
    await deleteNotification(Number(id));
    await fetchNotifications();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除失败');
  }
}

async function handleMakeAll() {
  try {
    await markAllNotificationsRead();
    await fetchNotifications();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '标记已读失败');
  }
}

const viewAll = () => {
  void router.push('/kadmin/notifications');
};

const handleClick = (item: NotificationItem) => {
  // 如果通知项有链接，点击时跳转
  if (item.link) {
    navigateTo(item.link, item.query, item.state);
  }
};

function navigateTo(
  link: string,
  query?: Record<string, any>,
  state?: Record<string, any>,
) {
  if (link.startsWith('http://') || link.startsWith('https://')) {
    // 外部链接，在新标签页打开
    window.open(link, '_blank');
  } else {
    // 内部路由链接，支持 query 参数和 state
    router.push({
      path: link,
      query: query || {},
      state,
    });
  }
}

watch(
  () => ({
    enable: preferences.app.watermark,
    content: preferences.app.watermarkContent,
    isDark: isDark.value,
  }),
  async ({ enable, content, isDark: isDarkValue }) => {
    if (enable) {
      const watermarkColor = isDarkValue
        ? 'rgba(255, 255, 255, 0.12)'
        : 'rgba(0, 0, 0, 0.12)';

      await updateWatermark({
        advancedStyle: {
          colorStops: [
            {
              color: watermarkColor,
              offset: 0,
            },
            {
              color: watermarkColor,
              offset: 1,
            },
          ],
          type: 'linear',
        },
        content:
          content ||
          `${userStore.userInfo?.username} - ${userStore.userInfo?.realName}`,
      });
    } else {
      destroyWatermark();
    }
  },
  {
    immediate: true,
  },
);
</script>

<template>
  <BasicLayout @clear-preferences-and-logout="handleLogout">
    <template #user-dropdown>
      <UserDropdown
        :avatar
        :menus
        :text="userStore.userInfo?.realName"
        description="ann.vben@gmail.com"
        tag-text="Pro"
        @logout="handleLogout"
        @clear-preferences-and-logout="handleLogout"
      />
    </template>
    <template #notification>
      <Notification
        :allow-clear="canClearNotifications"
        :allow-make-all="canReadAllNotifications"
        :allow-read="canReadNotification"
        :allow-remove="canDeleteNotification"
        :dot="showDot"
        :notifications="notifications"
        @clear="handleNoticeClear"
        @make-all="handleMakeAll"
        @on-click="handleClick"
        @read="(item) => item.id && markRead(item.id)"
        @remove="(item) => item.id && remove(item.id)"
        @view-all="viewAll"
      />
    </template>
    <template #extra>
      <AuthenticationLoginExpiredModal
        v-model:open="accessStore.loginExpired"
        :avatar
      >
        <LoginForm />
      </AuthenticationLoginExpiredModal>
    </template>
    <template #lock-screen>
      <LockScreen :avatar @to-login="handleLogout" />
    </template>
  </BasicLayout>
</template>
