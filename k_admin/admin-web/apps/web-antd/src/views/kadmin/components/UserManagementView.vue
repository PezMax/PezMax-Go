<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>用户管理</h1>
        <p>维护账号与职位分配</p>
      </div>
    </section>

    <section class="panel">
      <a-form :model="filters" layout="inline" class="search-form">
        <a-form-item label="关键词">
          <a-input
            v-model:value="filters.keyword"
            allow-clear
            class="control-md"
            placeholder="账号 / 昵称"
            @press-enter="searchUsers"
          >
            <template #prefix>
              <SearchOutlined />
            </template>
          </a-input>
        </a-form-item>
        <a-form-item label="部门">
          <a-input
            v-model:value="filters.department"
            allow-clear
            class="control-md"
            placeholder="部门名称"
            @press-enter="searchUsers"
          >
            <template #prefix>
              <SearchOutlined />
            </template>
          </a-input>
        </a-form-item>
        <a-form-item label="职位">
          <a-input
            v-model:value="filters.role"
            allow-clear
            class="control-md"
            placeholder="职位名称"
            @press-enter="searchUsers"
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
            <a-button type="primary" @click="searchUsers">
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
        <span class="muted-text">按条件筛选用户，并在行内维护单个用户</span>
        <a-space wrap>
          <a-button v-if="canCreate" type="primary" @click="openUserDrawer()">
            <PlusOutlined />
            新增用户
          </a-button>
          <a-button @click="loadUsers">
            <ReloadOutlined />
            刷新
          </a-button>
          <a-button v-if="canImport" @click="openImportModal">
            <UploadOutlined />
            导入
          </a-button>
          <a-dropdown v-if="canExport">
            <a-button>
              <DownloadOutlined />
              导出
            </a-button>
            <template #overlay>
              <a-menu @click="handleExport">
                <a-menu-item key="xlsx">导出 Excel</a-menu-item>
                <a-menu-item key="csv">导出 CSV</a-menu-item>
                <a-menu-item key="sql">导出 SQL</a-menu-item>
              </a-menu>
            </template>
          </a-dropdown>
        </a-space>
      </div>

      <a-table
        row-key="id"
        class="compact-user-table"
        size="small"
        :columns="columns"
        :data-source="users"
        :loading="loading"
        :pagination="pagination"
        :scroll="{ x: 1080 }"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'user'">
            <a-space>
              <a-avatar :src="avatarSource(record.avatar)">
                {{ (record.name || record.username).slice(0, 1).toUpperCase() }}
              </a-avatar>
              <div class="name-cell">
                <strong>{{ record.name || record.username }}</strong>
                <span>{{ record.username }}</span>
              </div>
            </a-space>
          </template>

          <template v-else-if="column.key === 'departments'">
            <a-space :size="4" wrap>
              <a-tag v-for="department in record.departments" :key="department">
                {{ department }}
              </a-tag>
              <a-tag v-if="record.departments.length === 0">未分配</a-tag>
            </a-space>
          </template>

          <template v-else-if="column.key === 'roles'">
            <a-space :size="4" wrap>
              <a-tag v-for="role in record.roles" :key="role">{{ role }}</a-tag>
              <a-tag v-if="record.id === 1" color="gold">最高权限</a-tag>
              <a-tag v-if="record.roles.length === 0 && record.id !== 1"
                >未分配</a-tag
              >
            </a-space>
          </template>

          <template v-else-if="column.key === 'status'">
            <div class="status-switch-cell">
              <a-switch
                :checked="record.status === 'enable'"
                :checked-children="statusLabel('enable')"
                :disabled="record.id === 1 || !canUpdateStatus"
                :loading="isStatusUpdating(record.id)"
                :un-checked-children="statusLabel('disable')"
                @change="toggleUserStatus(record, $event)"
              />
              <a-tag :color="statusColor(record.status)">
                {{ statusLabel(record.status) }}
              </a-tag>
            </div>
          </template>

          <template v-else-if="column.key === 'action'">
            <a-space>
              <a-button
                v-if="canUpdate"
                type="link"
                size="small"
                @click="openUserDrawer(record)"
              >
                编辑
              </a-button>
              <a-button
                v-if="canResetPassword"
                type="link"
                size="small"
                @click="openPasswordModal(record)"
              >
                重置密码
              </a-button>
              <a-popconfirm
                v-if="canUnlock"
                title="确认清除该账号的临时登录锁定？"
                @confirm="unlockUserLogin(record)"
              >
                <a-button type="link" size="small">解锁登录</a-button>
              </a-popconfirm>
              <a-popconfirm
                v-if="canDelete"
                title="确认删除该用户？"
                @confirm="removeUser(record)"
              >
                <a-button
                  type="link"
                  size="small"
                  danger
                  :disabled="record.id === 1"
                >
                  删除
                </a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </section>

    <a-drawer
      v-model:open="userDrawerOpen"
      :title="editingUser ? '编辑用户' : '新增用户'"
      width="min(520px, 100vw)"
      :destroy-on-close="true"
      @after-open-change="handleUserDrawerOpenChange"
    >
      <a-form
        ref="userFormRef"
        :model="userForm"
        :rules="userRules"
        layout="vertical"
      >
        <a-form-item label="账号" name="username">
          <a-input
            v-model:value="userForm.username"
            placeholder="请输入登录账号"
          />
        </a-form-item>
        <a-form-item v-if="!editingUser" label="初始密码" name="password">
          <a-input-password
            v-model:value="userForm.password"
            placeholder="至少 8 位，包含字母和数字"
          />
        </a-form-item>
        <a-form-item label="昵称" name="name">
          <a-input
            v-model:value="userForm.name"
            placeholder="可留空，默认使用账号"
          />
        </a-form-item>
        <a-form-item label="头像" name="avatar">
          <div class="avatar-uploader">
            <a-avatar :size="64" :src="avatarFormSource">
              {{ avatarInitial }}
            </a-avatar>
            <a-space wrap>
              <a-upload
                v-if="canUploadAvatar"
                accept="image/jpeg,image/png,image/webp,image/gif,image/bmp"
                :before-upload="beforeAvatarUpload"
                :disabled="avatarUploading"
                :show-upload-list="false"
              >
                <a-button :loading="avatarUploading">
                  <UploadOutlined />
                  上传头像
                </a-button>
              </a-upload>
              <a-button
                v-if="userForm.avatar && (editingUser ? canUpdate : canCreate)"
                :disabled="avatarUploading"
                @click="clearAvatar"
              >
                <DeleteOutlined />
                清除
              </a-button>
            </a-space>
          </div>
        </a-form-item>
        <a-form-item label="部门">
          <a-select
            v-model:value="userForm.departmentIds"
            mode="multiple"
            allow-clear
            :disabled="editingUser?.id === 1"
            :options="departmentOptions"
            placeholder="先选择部门，再选择该部门下的职位"
          />
        </a-form-item>
        <a-form-item label="职位">
          <a-select
            v-model:value="userForm.roleIds"
            mode="multiple"
            :disabled="editingUser?.id === 1"
            :options="roleOptions"
            :placeholder="
              userForm.departmentIds.length
                ? '请选择所选部门下的职位（可跨部门多选）'
                : '请选择职位'
            "
          />
          <div v-if="effectiveDepartments.length" class="muted-text form-hint">
            将隶属部门：{{
              effectiveDepartments.join('、')
            }}；担任多个职位时权限取并集
          </div>
        </a-form-item>
        <a-form-item label="状态" name="status">
          <a-select
            v-model:value="userForm.status"
            :options="statusOptions"
            :disabled="editingUser?.id === 1"
            placeholder="请选择账号状态"
          />
        </a-form-item>
      </a-form>
      <template #extra>
        <a-space>
          <a-button @click="userDrawerOpen = false">取消</a-button>
          <a-button
            type="primary"
            :disabled="editingUser ? !canUpdate : !canCreate"
            :loading="savingUser"
            @click="submitUser"
          >
            保存
          </a-button>
        </a-space>
      </template>
    </a-drawer>

    <a-modal
      v-model:open="passwordModalOpen"
      title="重置密码"
      :confirm-loading="savingPassword"
      @ok="submitPassword"
    >
      <a-form
        ref="passwordFormRef"
        :model="passwordForm"
        :rules="passwordRules"
        layout="vertical"
      >
        <a-form-item label="新密码" name="password">
          <a-input-password
            v-model:value="passwordForm.password"
            placeholder="至少 8 位，包含字母和数字"
          />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal
      v-model:open="importModalOpen"
      title="导入用户"
      :confirm-loading="importing"
      @ok="submitImport"
    >
      <a-form :model="importForm" layout="vertical">
        <a-form-item label="格式">
          <a-radio-group v-model:value="importForm.format">
            <a-radio-button value="xlsx">Excel</a-radio-button>
            <a-radio-button value="csv">CSV</a-radio-button>
            <a-radio-button value="sql">SQL</a-radio-button>
          </a-radio-group>
        </a-form-item>
        <a-form-item v-if="importForm.format === 'xlsx'" label="Excel 文件">
          <a-upload
            accept=".xlsx"
            :before-upload="beforeImportUpload"
            :show-upload-list="false"
          >
            <a-button>
              <UploadOutlined />
              选择文件
            </a-button>
          </a-upload>
          <p class="muted-text">
            {{ importFileName || '支持导入导出的 users.xlsx 格式' }}
          </p>
        </a-form-item>
        <a-form-item v-else label="导入内容">
          <a-textarea
            v-model:value="importForm.content"
            :rows="8"
            placeholder="CSV 表头支持 username,password,name,avatar,status,role_ids；SQL 使用导出的 INSERT 格式"
          />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import {
  ClearOutlined,
  DeleteOutlined,
  DownloadOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  UploadOutlined,
} from '@ant-design/icons-vue';
import { useAccess } from '@vben/access';
import { message } from 'ant-design-vue';
import type { FormInstance, UploadProps } from 'ant-design-vue';
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue';

import { getDictionaryData } from '#/api/kadmin/dictionaries';
import type { DictionaryData } from '#/api/kadmin/dictionaries';
import {
  deleteFile,
  getFileContent,
  isManagedFileUrl,
  managedFileId,
  uploadFile,
} from '#/api/kadmin/files';
import { KADMIN_PERMISSION } from '#/api/kadmin/permissions';
import {
  createUser,
  deleteUser,
  exportUsers,
  getUserOptions,
  getUsers,
  importUsers,
  resetUserPassword,
  updateUser,
  updateUserStatus,
  unlockUser,
} from '#/api/kadmin/users';
import type {
  ManagedUser,
  ManagedUserDepartmentOption,
  ManagedUserRoleOption,
  UserImportExportFormat,
} from '#/api/kadmin/users';

type TablePagination = {
  current?: number;
  pageSize?: number;
};
const { hasAccessByCodes } = useAccess();
const canCreate = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.USER_CREATE, '*']),
);
const canUpdate = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.USER_UPDATE, '*']),
);
const canUpdateStatus = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.USER_STATUS, '*']),
);
const canDelete = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.USER_DELETE, '*']),
);
const canResetPassword = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.USER_PASSWORD, '*']),
);
const canUnlock = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.USER_UNLOCK, '*']),
);
const canImport = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.USER_IMPORT, '*']),
);
const canExport = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.USER_EXPORT, '*']),
);
const canUploadAvatar = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.USER_AVATAR, '*']),
);

const loading = ref(false);
const savingUser = ref(false);
const savingPassword = ref(false);
const avatarUploading = ref(false);
const importing = ref(false);
const statusUpdatingIds = ref<number[]>([]);
const users = ref<ManagedUser[]>([]);
const avatarSources = ref<Record<string, string>>({});
const avatarPreviewObjectUrl = ref('');
const pendingAvatarFileId = ref<number>();
const roles = ref<ManagedUserRoleOption[]>([]);
const departments = ref<ManagedUserDepartmentOption[]>([]);
const statusItems = ref<DictionaryData[]>([]);
const userDrawerOpen = ref(false);
const passwordModalOpen = ref(false);
const importModalOpen = ref(false);
const importFileName = ref('');
const editingUser = ref<ManagedUser | null>(null);
const passwordUser = ref<ManagedUser | null>(null);
const userFormRef = ref<FormInstance>();
const passwordFormRef = ref<FormInstance>();
const managedAvatarObjectUrls = new Map<string, string>();
let avatarLoadGeneration = 0;
let avatarUploadGeneration = 0;

const filters = reactive({
  keyword: '',
  department: '',
  role: '',
  status: undefined as string | undefined,
});

const userForm = reactive({
  username: '',
  password: '',
  name: '',
  avatar: '',
  status: 'enable',
  roleIds: [] as number[],
  departmentIds: [] as number[],
});

const passwordForm = reactive({
  password: '',
});

const importForm = reactive<{
  format: UserImportExportFormat;
  content: string;
}>({
  format: 'xlsx',
  content: '',
});

const pagination = reactive({
  current: 1,
  pageSize: 10,
  showSizeChanger: true,
  showTotal: (total: number) => `共 ${total} 人`,
});

const columns = [
  { title: '用户', key: 'user', width: 220, fixed: 'left' },
  { title: '部门', key: 'departments', width: 220 },
  { title: '职位', key: 'roles', width: 260 },
  { title: '状态', key: 'status', width: 170 },
  { title: '创建时间', dataIndex: 'createdAt', key: 'createdAt', width: 160 },
  { title: '操作', key: 'action', width: 280, fixed: 'right' },
];

const userRules = computed(() => ({
  username: [{ required: true, message: '请输入账号' }],
  status: [{ required: true, message: '请选择账号状态' }],
  password: editingUser.value
    ? []
    : [
        { required: true, message: '请输入初始密码' },
        { min: 8, message: '密码至少 8 位' },
      ],
}));

const passwordRules = {
  password: [
    { required: true, message: '请输入新密码' },
    { min: 8, message: '密码至少 8 位' },
  ],
};

const departmentOptions = computed(() =>
  departments.value.map((department) => ({
    label: department.name,
    value: department.id,
  })),
);

const roleOptions = computed(() => {
  const selectedDepartmentIds = new Set(userForm.departmentIds);
  let allowedRoleIds: Set<number> | null = null;
  if (selectedDepartmentIds.size > 0) {
    allowedRoleIds = new Set<number>();
    for (const department of departments.value) {
      if (selectedDepartmentIds.has(department.id)) {
        for (const roleId of department.roleIds || []) {
          allowedRoleIds.add(roleId);
        }
      }
    }
  }
  return roles.value
    .filter((role) => !allowedRoleIds || allowedRoleIds.has(role.id))
    .map((role) => ({
      label: `${role.name}（${role.slug}）`,
      value: role.id,
    }));
});

// 用户隶属部门由所选职位的所属部门推导，跨部门选职即可同时隶属多个部门。
const effectiveDepartments = computed(() => {
  const selectedRoleIds = new Set(userForm.roleIds);
  if (selectedRoleIds.size === 0) {
    return [];
  }
  const names: string[] = [];
  const seen = new Set<number>();
  for (const department of departments.value) {
    const hasRole = (department.roleIds || []).some((roleId) =>
      selectedRoleIds.has(roleId),
    );
    if (hasRole && !seen.has(department.id)) {
      seen.add(department.id);
      names.push(department.name);
    }
  }
  return names;
});

const statusOptions = computed(() =>
  statusItems.value.map((item) => ({
    label: item.label,
    value: item.value,
  })),
);

const avatarInitial = computed(() =>
  (userForm.name || userForm.username || 'U').slice(0, 1).toUpperCase(),
);
const avatarFormSource = computed(
  () => avatarPreviewObjectUrl.value || avatarSource(userForm.avatar),
);

onMounted(() => {
  void Promise.all([loadUsers(), loadRbacData(), loadStatusOptions()]);
});

onUnmounted(() => {
  avatarLoadGeneration += 1;
  avatarUploadGeneration += 1;
  clearAvatarPreview();
  for (const objectUrl of managedAvatarObjectUrls.values()) {
    URL.revokeObjectURL(objectUrl);
  }
  managedAvatarObjectUrls.clear();
  void discardPendingAvatar();
});

async function loadUsers() {
  loading.value = true;
  try {
    const data = await getUsers({
      keyword: filters.keyword.trim(),
      department: filters.department.trim(),
      role: filters.role.trim(),
      status: filters.status,
    });
    users.value = (data.items || []).map((user) => ({
      ...user,
      roleIds: user.roleIds || [],
      roles: user.roles || [],
      departmentIds: user.departmentIds || [],
      departments: user.departments || [],
      status: user.status || 'enable',
    }));
    void loadManagedAvatarSources(users.value);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载用户失败');
  } finally {
    loading.value = false;
  }
}

function avatarSource(avatar?: string) {
  if (!avatar) {
    return undefined;
  }
  return isManagedFileUrl(avatar) ? avatarSources.value[avatar] : avatar;
}

async function loadManagedAvatarSources(records: ManagedUser[]) {
  const generation = ++avatarLoadGeneration;
  const requiredUrls = new Set(
    records
      .map((record) => record.avatar || '')
      .filter((avatar) => isManagedFileUrl(avatar)),
  );

  for (const [stableUrl, objectUrl] of managedAvatarObjectUrls) {
    if (!requiredUrls.has(stableUrl)) {
      URL.revokeObjectURL(objectUrl);
      managedAvatarObjectUrls.delete(stableUrl);
    }
  }
  avatarSources.value = Object.fromEntries(managedAvatarObjectUrls);

  const loaded = await Promise.all(
    [...requiredUrls]
      .filter((stableUrl) => !managedAvatarObjectUrls.has(stableUrl))
      .map(async (stableUrl) => {
        const id = managedFileId(stableUrl);
        if (!id) {
          return undefined;
        }
        try {
          return { blob: await getFileContent(id), stableUrl };
        } catch {
          return undefined;
        }
      }),
  );
  if (generation !== avatarLoadGeneration) {
    return;
  }

  for (const item of loaded) {
    if (item) {
      managedAvatarObjectUrls.set(
        item.stableUrl,
        URL.createObjectURL(item.blob),
      );
    }
  }
  avatarSources.value = Object.fromEntries(managedAvatarObjectUrls);
}

function setAvatarPreview(file: File) {
  clearAvatarPreview();
  avatarPreviewObjectUrl.value = URL.createObjectURL(file);
}

function clearAvatarPreview() {
  if (avatarPreviewObjectUrl.value) {
    URL.revokeObjectURL(avatarPreviewObjectUrl.value);
    avatarPreviewObjectUrl.value = '';
  }
}

async function discardPendingAvatar() {
  const fileId = pendingAvatarFileId.value;
  pendingAvatarFileId.value = undefined;
  if (fileId) {
    await deleteFile(fileId).catch(() => undefined);
  }
}

function handleUserDrawerOpenChange(open: boolean) {
  if (open) {
    return;
  }
  avatarUploadGeneration += 1;
  avatarUploading.value = false;
  clearAvatarPreview();
  void discardPendingAvatar();
}

async function loadRbacData() {
  try {
    const data = await getUserOptions();
    roles.value = data.roles || [];
    departments.value = data.departments || [];
  } catch {
    roles.value = [];
    departments.value = [];
  }
}

async function loadStatusOptions() {
  try {
    const data = await getDictionaryData({ dictType: 'sys_status', status: 1 });
    statusItems.value = data.items || [];
  } catch {
    statusItems.value = [
      {
        id: 1,
        dictType: 'sys_status',
        label: 'Enable',
        value: 'enable',
        isDefault: true,
        sort: 1,
        status: 1,
        color: 'green',
      },
      {
        id: 2,
        dictType: 'sys_status',
        label: 'Disable',
        value: 'disable',
        isDefault: false,
        sort: 2,
        status: 1,
        color: 'red',
      },
    ];
  }
}

function resetSearch() {
  filters.keyword = '';
  filters.department = '';
  filters.role = '';
  filters.status = undefined;
  pagination.current = 1;
  void loadUsers();
}

function searchUsers() {
  pagination.current = 1;
  void loadUsers();
}

function handleTableChange(pager: TablePagination) {
  pagination.current = pager.current || 1;
  pagination.pageSize = pager.pageSize || 10;
}

function openUserDrawer(record?: ManagedUser) {
  editingUser.value = record || null;
  userForm.username = record?.username || '';
  userForm.password = '';
  userForm.name = record?.name || '';
  userForm.avatar = record?.avatar || '';
  userForm.status = record?.status || 'enable';
  userForm.roleIds = record?.roleIds ? [...record.roleIds] : [];
  userForm.departmentIds = record?.departmentIds
    ? [...record.departmentIds]
    : [];
  clearAvatarPreview();
  userDrawerOpen.value = true;
}

const beforeAvatarUpload: UploadProps['beforeUpload'] = async (file) => {
  if (!file.type.startsWith('image/')) {
    message.warning('请选择图片文件');
    return false;
  }
  if (file.size > 2 * 1024 * 1024) {
    message.warning('头像图片不能超过 2MB');
    return false;
  }

  const generation = ++avatarUploadGeneration;
  avatarUploading.value = true;
  try {
    const data = await uploadFile(file, 'avatar');
    if (generation !== avatarUploadGeneration || !userDrawerOpen.value) {
      void deleteFile(data.id).catch(() => undefined);
      return false;
    }
    await discardPendingAvatar();
    pendingAvatarFileId.value = data.id;
    userForm.avatar = data.url;
    setAvatarPreview(file);
    message.success('头像上传成功');
  } catch (error) {
    if (generation === avatarUploadGeneration) {
      message.error(error instanceof Error ? error.message : '上传头像失败');
    }
  } finally {
    if (generation === avatarUploadGeneration) {
      avatarUploading.value = false;
    }
  }
  return false;
};

function clearAvatar() {
  userForm.avatar = '';
  clearAvatarPreview();
  void discardPendingAvatar();
}

async function submitUser() {
  await userFormRef.value?.validate();
  savingUser.value = true;
  try {
    const payload = {
      username: userForm.username.trim(),
      name: userForm.name.trim(),
      avatar: userForm.avatar.trim(),
      status: userForm.status,
      roleIds: userForm.roleIds,
    };
    await (editingUser.value
      ? updateUser(editingUser.value.id, payload)
      : createUser({
          ...payload,
          password: userForm.password.trim(),
        }));
    pendingAvatarFileId.value = undefined;
    clearAvatarPreview();
    userDrawerOpen.value = false;
    await loadUsers();
    message.success('用户已保存');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存用户失败');
  } finally {
    savingUser.value = false;
  }
}

function openPasswordModal(record?: ManagedUser) {
  passwordUser.value = record || null;
  passwordForm.password = '';
  if (!passwordUser.value) {
    return;
  }
  passwordModalOpen.value = true;
}

async function submitPassword() {
  await passwordFormRef.value?.validate();
  if (!passwordUser.value) return;
  savingPassword.value = true;
  try {
    await resetUserPassword(
      passwordUser.value.id,
      passwordForm.password.trim(),
    );
    passwordModalOpen.value = false;
    message.success('密码已重置');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '重置密码失败');
  } finally {
    savingPassword.value = false;
  }
}

async function unlockUserLogin(record: ManagedUser) {
  try {
    await unlockUser(record.id);
    message.success('临时登录锁定已清除');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '解锁登录失败');
  }
}

async function removeUser(record: ManagedUser) {
  if (record.id === 1) return;
  try {
    await deleteUser(record.id);
    await loadUsers();
    message.success('用户已删除');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除用户失败');
  }
}

function isStatusUpdating(id: number) {
  return statusUpdatingIds.value.includes(id);
}

async function toggleUserStatus(record: ManagedUser, checked: boolean) {
  if (record.id === 1) return;
  const nextStatus = checked ? 'enable' : 'disable';
  if (record.status === nextStatus) return;

  const previousStatus = record.status;
  statusUpdatingIds.value = [...statusUpdatingIds.value, record.id];
  record.status = nextStatus;
  try {
    const user = await updateUserStatus(record.id, nextStatus);
    record.status = user.status || nextStatus;
    message.success(nextStatus === 'enable' ? '账号已启用' : '账号已停用');
  } catch (error) {
    record.status = previousStatus;
    message.error(error instanceof Error ? error.message : '更新账号状态失败');
  } finally {
    statusUpdatingIds.value = statusUpdatingIds.value.filter(
      (id) => id !== record.id,
    );
  }
}

function openImportModal() {
  importForm.format = 'xlsx';
  importForm.content = '';
  importFileName.value = '';
  importModalOpen.value = true;
}

const beforeImportUpload: UploadProps['beforeUpload'] = async (file) => {
  try {
    importForm.content = await readFileAsBase64(file);
    importFileName.value = file.name;
    message.success(`${file.name} 已选择`);
  } catch {
    message.error('读取 Excel 文件失败');
  }
  return false;
};

async function submitImport() {
  if (!importForm.content.trim()) {
    message.warning(
      importForm.format === 'xlsx' ? '请选择 Excel 文件' : '请粘贴导入内容',
    );
    return;
  }
  importing.value = true;
  try {
    const data = await importUsers(importForm.format, importForm.content);
    importModalOpen.value = false;
    await loadUsers();
    message.success(`已导入 ${data.imported} 个用户`);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '导入失败');
  } finally {
    importing.value = false;
  }
}

async function handleExport(event: { key: UserImportExportFormat }) {
  try {
    const { blob, filename } = await exportUsers(event.key);
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    link.click();
    URL.revokeObjectURL(url);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '导出失败');
  }
}

function statusLabel(status: string) {
  return (
    statusItems.value.find((item) => item.value === status)?.label ||
    status ||
    'Enable'
  );
}

function statusColor(status: string) {
  return (
    statusItems.value.find((item) => item.value === status)?.color ||
    (status === 'disable' ? 'red' : 'green')
  );
}

async function readFileAsBase64(file: File) {
  return arrayBufferToBase64(await file.arrayBuffer());
}

function arrayBufferToBase64(buffer: ArrayBuffer) {
  const bytes = new Uint8Array(buffer);
  const chunkSize = 0x80_00;
  let binary = '';
  for (let i = 0; i < bytes.length; i += chunkSize) {
    const chunk = bytes.subarray(i, i + chunkSize);
    binary += String.fromCodePoint(...chunk);
  }
  return window.btoa(binary);
}
</script>
