<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>权限管理</h1>
        <p>选择部门后，在子菜单中管理部门职位与职位菜单权限</p>
      </div>
      <a-space wrap>
        <a-button @click="loadOverview">
          <ReloadOutlined />
          刷新
        </a-button>
      </a-space>
    </section>

    <a-row class="rbac-layout" :gutter="[16, 16]">
      <a-col :xs="24" :lg="8">
        <section class="panel">
          <div class="panel-title">
            <h2>部门</h2>
            <a-button
              v-if="canCreateDepartment"
              size="small"
              type="primary"
              @click="openDepartmentDrawer()"
            >
              <PlusOutlined />
              新建部门
            </a-button>
          </div>
          <a-input
            v-model:value="departmentKeyword"
            allow-clear
            class="toolbar-input"
            placeholder="搜索部门"
          >
            <template #prefix>
              <SearchOutlined />
            </template>
          </a-input>
          <a-list
            class="role-list"
            :data-source="filteredDepartments"
            :loading="loading"
            item-layout="horizontal"
          >
            <template #renderItem="{ item }">
              <a-list-item
                :class="[
                  'role-list-item',
                  { active: item.id === selectedDepartmentId },
                ]"
                @click="selectDepartment(item.id)"
              >
                <a-list-item-meta>
                  <template #avatar>
                    <a-avatar
                      :style="{
                        backgroundColor:
                          item.status === 1
                            ? 'hsl(var(--primary))'
                            : 'hsl(var(--muted-foreground))',
                      }"
                    >
                      {{ item.name.slice(0, 1) }}
                    </a-avatar>
                  </template>
                  <template #title>
                    <a-space :size="6">
                      <span>{{ item.name }}</span>
                      <a-tag v-if="item.status !== 1">停用</a-tag>
                    </a-space>
                  </template>
                  <template #description>
                    {{ item.code || '未设置编码' }} ·
                    {{ item.roleIds.length }} 个职位
                  </template>
                </a-list-item-meta>
              </a-list-item>
            </template>
          </a-list>
        </section>
      </a-col>

      <a-col :xs="24" :lg="16">
        <section class="panel">
          <a-empty v-if="!selectedDepartment" description="请选择部门" />
          <template v-else>
            <div class="panel-title">
              <div>
                <h2>{{ selectedDepartment.name }}</h2>
                <p class="muted-text">
                  {{ selectedDepartment.code || '未设置编码' }}
                  <template v-if="selectedDepartment.description">
                    · {{ selectedDepartment.description }}
                  </template>
                </p>
              </div>
              <a-space wrap>
                <a-button
                  v-if="canUpdateDepartment"
                  @click="openDepartmentDrawer(selectedDepartment)"
                >
                  <EditOutlined />
                  编辑部门
                </a-button>
                <a-popconfirm
                  v-if="canDeleteDepartment"
                  title="确认删除该部门？"
                  @confirm="removeDepartment"
                >
                  <a-button danger>
                    <DeleteOutlined />
                    删除
                  </a-button>
                </a-popconfirm>
              </a-space>
            </div>

            <a-tabs v-model:active-key="activeTab" class="rbac-tabs">
              <a-tab-pane key="departmentRoles" tab="部门职位">
                <div class="table-toolbar department-role-toolbar">
                  <div>
                    <strong>部门下的职位</strong>
                    <p class="muted-text">
                      选择已有职位加入当前部门，或直接新建职位
                    </p>
                  </div>
                  <a-space wrap>
                    <a-select
                      v-model:value="pendingDepartmentRoleId"
                      allow-clear
                      show-search
                      option-filter-prop="label"
                      class="control-lg"
                      :options="availableRoleOptions"
                      placeholder="选择已有职位"
                    />
                    <a-button
                      v-if="canAssignDepartmentRoles"
                      :disabled="!pendingDepartmentRoleId"
                      :loading="savingDepartmentRoles"
                      @click="addRoleToDepartment"
                    >
                      添加职位
                    </a-button>
                    <a-button
                      v-if="canCreateRole"
                      type="primary"
                      @click="openRoleDrawer()"
                    >
                      <PlusOutlined />
                      新建职位
                    </a-button>
                  </a-space>
                  <p class="position-scroll-hint muted-text">
                    表格内容较多时可左右滑动查看全部操作
                  </p>
                </div>

                <a-empty
                  v-if="departmentRoles.length === 0"
                  description="当前部门暂无职位"
                />
                <a-table
                  v-else
                  row-key="id"
                  size="small"
                  :columns="roleColumns"
                  :data-source="departmentRoles"
                  :pagination="false"
                  :scroll="{ x: 840 }"
                >
                  <template #bodyCell="{ column, record }">
                    <template v-if="column.key === 'role'">
                      <a-space>
                        <a-avatar
                          :style="{
                            backgroundColor: record.isAdmin
                              ? 'hsl(var(--warning))'
                              : 'hsl(var(--primary))',
                          }"
                        >
                          {{ record.name.slice(0, 1) }}
                        </a-avatar>
                        <div class="name-cell">
                          <strong>{{ record.name }}</strong>
                          <span>{{ record.slug }}</span>
                        </div>
                      </a-space>
                    </template>
                    <template v-else-if="column.key === 'summary'">
                      <a-space :size="4" wrap>
                        <a-tag>{{ record.menuIds.length }} 菜单</a-tag>
                        <a-tag>{{ record.permissionIds.length }} 标识</a-tag>
                        <a-tag>{{ record.userIds.length }} 人</a-tag>
                        <a-tag v-if="record.isAdmin" color="gold"
                          >最高权限</a-tag
                        >
                      </a-space>
                    </template>
                    <template v-else-if="column.key === 'action'">
                      <a-space wrap>
                        <a-button
                          v-if="canAssignRoleMenus"
                          size="small"
                          type="primary"
                          ghost
                          @click="selectRoleForMenus(record)"
                        >
                          配置菜单
                        </a-button>
                        <a-button
                          v-if="canAssignRolePermissions"
                          size="small"
                          type="primary"
                          ghost
                          @click="selectRoleForPermissions(record)"
                        >
                          配置 API 权限
                        </a-button>
                        <a-button
                          v-if="canUpdateRole"
                          size="small"
                          :disabled="record.isAdmin"
                          @click="openRoleDrawer(record)"
                        >
                          改名
                        </a-button>
                        <a-popconfirm
                          v-if="canAssignDepartmentRoles"
                          title="确认将该职位移出当前部门？职位本身不会被删除。"
                          @confirm="removeRoleFromDepartment(record)"
                        >
                          <a-button
                            size="small"
                            :disabled="savingDepartmentRoles"
                          >
                            移出部门
                          </a-button>
                        </a-popconfirm>
                        <a-popconfirm
                          v-if="canDeleteRole"
                          title="确认全局删除该职位？所有部门、人员和菜单权限关联将同步删除。"
                          @confirm="removeRole(record)"
                        >
                          <a-button
                            size="small"
                            danger
                            :disabled="record.isAdmin"
                            >删除</a-button
                          >
                        </a-popconfirm>
                      </a-space>
                    </template>
                  </template>
                </a-table>
              </a-tab-pane>

              <a-tab-pane key="roleMenus" tab="职位菜单权限">
                <div class="table-toolbar">
                  <a-select
                    v-model:value="selectedRoleId"
                    class="control-lg"
                    :options="departmentRoleOptions"
                    placeholder="选择部门下的职位"
                  />
                  <a-button
                    type="primary"
                    :disabled="
                      !selectedRole ||
                      selectedRole.isAdmin ||
                      !canAssignRoleMenus
                    "
                    :loading="savingMenus"
                    @click="saveMenus"
                  >
                    保存菜单权限
                  </a-button>
                </div>
                <a-empty
                  v-if="!selectedRole"
                  description="请先在部门职位中选择或添加职位"
                />
                <template v-else>
                  <a-alert
                    v-if="selectedRole.isAdmin"
                    class="form-alert"
                    type="info"
                    show-icon
                    message="admin 最高权限职位不受菜单限制，仅展示当前系统权限全集。"
                  />
                  <a-tree
                    v-model:checkedKeys="checkedMenuKeys"
                    checkable
                    default-expand-all
                    :disabled="selectedRole.isAdmin"
                    :tree-data="menuTreeData"
                  />
                </template>
              </a-tab-pane>

              <a-tab-pane key="rolePermissions" tab="职位 API 权限">
                <div class="table-toolbar">
                  <a-select
                    v-model:value="selectedRoleId"
                    class="control-lg"
                    :options="departmentRoleOptions"
                    placeholder="选择部门下的职位"
                  />
                  <a-button
                    type="primary"
                    :disabled="
                      !selectedRole ||
                      selectedRole.isAdmin ||
                      !canAssignRolePermissions
                    "
                    :loading="savingPermissions"
                    @click="savePermissions"
                  >
                    保存 API 权限
                  </a-button>
                </div>
                <a-empty
                  v-if="!selectedRole"
                  description="请先在部门职位中选择或添加职位"
                />
                <template v-else>
                  <a-alert
                    class="form-alert"
                    type="info"
                    show-icon
                    :message="
                      selectedRole.isAdmin
                        ? 'admin 最高权限职位拥有全部 API 权限。'
                        : '菜单权限只授予页面浏览和读取能力；按钮与对应 API 必须在此逐项显式授权。'
                    "
                  />
                  <a-collapse class="permission-groups" :bordered="false">
                    <a-collapse-panel
                      v-for="group in permissionGroups"
                      :key="group.key"
                    >
                      <template #header>
                        <div class="permission-group-title">
                          <strong>{{ group.title }}</strong>
                          <span class="muted-text">
                            {{ group.pageUri || '未关联页面' }} ·
                            {{ group.items.length }} 个标识
                          </span>
                        </div>
                      </template>
                      <div class="permission-list">
                        <div
                          v-for="permission in group.items"
                          :key="permission.id"
                          class="permission-item"
                        >
                          <a-checkbox
                            :checked="isPermissionChecked(permission.id)"
                            :disabled="
                              selectedRole.isAdmin ||
                              permission.scope === 'page' ||
                              !canAssignRolePermissions
                            "
                            @change="
                              togglePermission(
                                permission.id,
                                Boolean($event.target?.checked),
                              )
                            "
                          >
                            {{ permission.name }}
                          </a-checkbox>
                          <a-space :size="4" wrap>
                            <a-tag
                              :color="
                                permission.scope === 'page' ? 'blue' : 'purple'
                              "
                            >
                              {{ permissionScopeLabel(permission.scope) }}
                            </a-tag>
                            <a-tag
                              v-if="isPermissionInherited(permission.id)"
                              color="cyan"
                            >
                              随菜单继承
                            </a-tag>
                            <code class="permission-code">
                              {{ permission.slug }}
                            </code>
                          </a-space>
                          <div class="permission-meta">
                            <span v-if="permission.button">
                              按钮：{{ permission.button }}
                            </span>
                            <span>
                              API：{{ permission.httpMethod || '—' }}
                              {{ permission.httpPath || '—' }}
                            </span>
                          </div>
                        </div>
                      </div>
                    </a-collapse-panel>
                  </a-collapse>
                </template>
              </a-tab-pane>
            </a-tabs>
          </template>
        </section>
      </a-col>
    </a-row>

    <a-drawer
      v-model:open="departmentDrawerOpen"
      :title="editingDepartment ? '编辑部门' : '新建部门'"
      width="420"
      :destroy-on-close="true"
    >
      <a-form
        ref="departmentFormRef"
        :model="departmentForm"
        :rules="departmentRules"
        layout="vertical"
      >
        <a-form-item label="部门名称" name="name">
          <a-input
            v-model:value="departmentForm.name"
            placeholder="例如：运营部"
          />
        </a-form-item>
        <a-form-item label="部门编码" name="code">
          <a-input
            v-model:value="departmentForm.code"
            placeholder="例如：operation"
          />
        </a-form-item>
        <a-form-item label="排序" name="sort">
          <a-input-number
            v-model:value="departmentForm.sort"
            class="full-width"
            :min="0"
          />
        </a-form-item>
        <a-form-item label="状态" name="status">
          <a-select
            v-model:value="departmentForm.status"
            :options="[
              { label: '启用', value: 1 },
              { label: '停用', value: 2 },
            ]"
          />
        </a-form-item>
        <a-form-item label="描述" name="description">
          <a-textarea
            v-model:value="departmentForm.description"
            :rows="3"
            placeholder="可选，描述部门职责"
          />
        </a-form-item>
      </a-form>
      <template #extra>
        <a-space>
          <a-button @click="departmentDrawerOpen = false">取消</a-button>
          <a-button
            type="primary"
            :loading="savingDepartment"
            @click="submitDepartment"
          >
            保存
          </a-button>
        </a-space>
      </template>
    </a-drawer>

    <a-drawer
      v-model:open="roleDrawerOpen"
      :title="editingRole ? '编辑职位' : '新建职位'"
      width="420"
      :destroy-on-close="true"
    >
      <a-form
        ref="roleFormRef"
        :model="roleForm"
        :rules="roleRules"
        layout="vertical"
      >
        <a-form-item label="职位名称" name="name">
          <a-input v-model:value="roleForm.name" placeholder="例如：运营人员" />
        </a-form-item>
        <a-form-item label="职位标识" name="slug">
          <a-input v-model:value="roleForm.slug" placeholder="例如：operator" />
        </a-form-item>
      </a-form>
      <template #extra>
        <a-space>
          <a-button @click="roleDrawerOpen = false">取消</a-button>
          <a-button type="primary" :loading="savingRole" @click="submitRole"
            >保存</a-button
          >
        </a-space>
      </template>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons-vue';
import { useAccess } from '@vben/access';
import { message } from 'ant-design-vue';
import type { FormInstance } from 'ant-design-vue';
import { computed, onMounted, reactive, ref, watch } from 'vue';

import {
  createDepartment,
  createRole,
  deleteDepartment,
  deleteRole,
  getRbacOverview,
  updateDepartment,
  updateDepartmentRoles,
  updateRole,
  updateRoleMenus,
  updateRolePermissions,
} from '#/api/kadmin/rbac';
import type {
  RbacDepartment,
  RbacMenu,
  RbacPermission,
  RbacRole,
} from '#/api/kadmin/rbac';
import { KADMIN_PERMISSION } from '#/api/kadmin/permissions';

type TreeNode = {
  title: string;
  key: number;
  children?: TreeNode[];
};
const { hasAccessByCodes } = useAccess();
const canCreateDepartment = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.RBAC_DEPARTMENT_CREATE, '*']),
);
const canUpdateDepartment = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.RBAC_DEPARTMENT_UPDATE, '*']),
);
const canDeleteDepartment = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.RBAC_DEPARTMENT_DELETE, '*']),
);
const canAssignDepartmentRoles = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.RBAC_DEPARTMENT_ROLES, '*']),
);
const canCreateRole = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.RBAC_ROLE_CREATE, '*']),
);
const canUpdateRole = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.RBAC_ROLE_UPDATE, '*']),
);
const canDeleteRole = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.RBAC_ROLE_DELETE, '*']),
);
const canAssignRoleMenus = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.RBAC_ROLE_MENUS, '*']),
);
const canAssignRolePermissions = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.RBAC_ROLE_PERMISSIONS, '*']),
);

const loading = ref(false);
const savingDepartment = ref(false);
const savingDepartmentRoles = ref(false);
const savingRole = ref(false);
const savingMenus = ref(false);
const savingPermissions = ref(false);
const departments = ref<RbacDepartment[]>([]);
const roles = ref<RbacRole[]>([]);
const menus = ref<RbacMenu[]>([]);
const permissions = ref<RbacPermission[]>([]);
const selectedDepartmentId = ref<number>();
const selectedRoleId = ref<number>();
const departmentKeyword = ref('');
const activeTab = ref('departmentRoles');
const checkedMenuKeys = ref<number[]>([]);
const checkedPermissionKeys = ref<number[]>([]);
const pendingDepartmentRoleId = ref<number>();
const departmentDrawerOpen = ref(false);
const roleDrawerOpen = ref(false);
const editingDepartment = ref<RbacDepartment | null>(null);
const editingRole = ref<RbacRole | null>(null);
const departmentFormRef = ref<FormInstance>();
const roleFormRef = ref<FormInstance>();

const departmentForm = reactive({
  name: '',
  code: '',
  description: '',
  sort: 0,
  status: 1,
});

const roleForm = reactive({
  name: '',
  slug: '',
});

const departmentRules = {
  name: [{ required: true, message: '请输入部门名称' }],
};

const roleRules = {
  name: [{ required: true, message: '请输入职位名称' }],
  slug: [{ required: true, message: '请输入职位标识' }],
};

const filteredDepartments = computed(() => {
  const keyword = departmentKeyword.value.trim().toLowerCase();
  if (!keyword) return departments.value;
  return departments.value.filter(
    (department) =>
      department.name.toLowerCase().includes(keyword) ||
      department.code.toLowerCase().includes(keyword),
  );
});

const departmentRoles = computed(() => {
  if (!selectedDepartment.value) return [];
  return selectedDepartment.value.roleIds
    .map((roleId) => roles.value.find((role) => role.id === roleId))
    .filter(Boolean) as RbacRole[];
});

const availableRoleOptions = computed(() => {
  const assignedRoleIds = new Set(selectedDepartment.value?.roleIds || []);
  return roles.value
    .filter((role) => !assignedRoleIds.has(role.id))
    .map((role) => ({
      label: `${role.name}（${role.slug}）`,
      value: role.id,
    }));
});

const selectedDepartment = computed(() =>
  departments.value.find(
    (department) => department.id === selectedDepartmentId.value,
  ),
);

const roleColumns = [
  { title: '职位', key: 'role', width: 240, fixed: 'left' },
  { title: '权限概览', key: 'summary', width: 220 },
  { title: '操作', key: 'action', width: 380, fixed: 'right' },
];

const selectedRole = computed(() =>
  roles.value.find((role) => role.id === selectedRoleId.value),
);

const permissionGroups = computed(() => {
  const groups = new Map<
    string,
    { key: string; title: string; pageUri: string; items: RbacPermission[] }
  >();
  for (const permission of permissions.value) {
    const pageUri = permission.pageUri || '';
    const key = pageUri || 'other-api';
    const group = groups.get(key) || {
      key,
      title: permission.pageTitle || '其他 API',
      pageUri,
      items: [],
    };
    group.items.push(permission);
    groups.set(key, group);
  }
  return [...groups.values()].map((group) => ({
    ...group,
    items: [...group.items].toSorted((left, right) => {
      if (left.scope === right.scope) return left.id - right.id;
      return left.scope === 'page' ? -1 : 1;
    }),
  }));
});

const inheritedPermissionIds = computed(() => {
  const menuUris = new Set(
    (selectedRole.value?.menus || []).map((menu) => menu.uri || ''),
  );
  return new Set(
    permissions.value
      .filter(
        (permission) =>
          permission.scope === 'page' &&
          !!permission.pageUri &&
          menuUris.has(permission.pageUri),
      )
      .map((permission) => permission.id),
  );
});

function isPermissionChecked(permissionId: number) {
  return (
    selectedRole.value?.isAdmin ||
    inheritedPermissionIds.value.has(permissionId) ||
    checkedPermissionKeys.value.includes(permissionId)
  );
}

function isPermissionInherited(permissionId: number) {
  return inheritedPermissionIds.value.has(permissionId);
}

function permissionScopeLabel(scope: string) {
  if (scope === 'page') return '页面';
  if (scope === 'button') return '按钮';
  return 'API';
}

const menuTreeData = computed(() => toTreeData(menus.value));

const departmentRoleOptions = computed(() =>
  departmentRoles.value.map((role) => ({
    label: `${role.name}（${role.slug}）`,
    value: role.id,
  })),
);

watch(selectedDepartment, (department) => {
  pendingDepartmentRoleId.value = undefined;
  if (!department || !department.roleIds.includes(selectedRoleId.value || 0)) {
    selectedRoleId.value = department?.roleIds[0];
  }
});

watch(
  selectedRole,
  (role) => {
    checkedMenuKeys.value = role?.menuIds ? [...role.menuIds] : [];
    checkedPermissionKeys.value = role?.permissionIds
      ? [...role.permissionIds]
      : [];
  },
  { immediate: true },
);

onMounted(() => {
  void loadOverview();
});

async function loadOverview() {
  loading.value = true;
  try {
    const data = await getRbacOverview();
    departments.value = data.departments || [];
    roles.value = (data.roles || []).map((role) => ({
      ...role,
      menuIds: role.menuIds || [],
      permissionIds: role.permissionIds || [],
      permissions: role.permissions || [],
      userIds: role.userIds || [],
    }));
    menus.value = data.menus || [];
    permissions.value = data.permissions || [];
    if (
      !selectedDepartmentId.value ||
      !departments.value.some(
        (department) => department.id === selectedDepartmentId.value,
      )
    ) {
      selectedDepartmentId.value = departments.value[0]?.id;
    }
    if (
      selectedDepartment.value &&
      !selectedDepartment.value.roleIds.includes(selectedRoleId.value || 0)
    ) {
      selectedRoleId.value = selectedDepartment.value.roleIds[0];
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载权限数据失败');
  } finally {
    loading.value = false;
  }
}

function selectDepartment(departmentId: number) {
  selectedDepartmentId.value = departmentId;
}

function openDepartmentDrawer(department?: RbacDepartment) {
  editingDepartment.value = department || null;
  departmentForm.name = department?.name || '';
  departmentForm.code = department?.code || '';
  departmentForm.description = department?.description || '';
  departmentForm.sort = department?.sort || 0;
  departmentForm.status = department?.status || 1;
  departmentDrawerOpen.value = true;
}

async function submitDepartment() {
  await departmentFormRef.value?.validate();
  savingDepartment.value = true;
  try {
    const payload = {
      name: departmentForm.name.trim(),
      code: departmentForm.code.trim(),
      description: departmentForm.description.trim(),
      sort: Number(departmentForm.sort) || 0,
      status: Number(departmentForm.status) || 1,
    };
    const department = editingDepartment.value
      ? await updateDepartment(editingDepartment.value.id, payload)
      : await createDepartment(payload);
    await loadOverview();
    selectedDepartmentId.value = department.id;
    departmentDrawerOpen.value = false;
    message.success('部门已保存');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存部门失败');
  } finally {
    savingDepartment.value = false;
  }
}

async function removeDepartment() {
  if (!selectedDepartment.value) return;
  try {
    await deleteDepartment(selectedDepartment.value.id);
    selectedDepartmentId.value = undefined;
    selectedRoleId.value = undefined;
    await loadOverview();
    message.success('部门已删除');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除部门失败');
  }
}

async function addRoleToDepartment() {
  const department = selectedDepartment.value;
  const roleId = pendingDepartmentRoleId.value;
  if (!department || !roleId) return;
  savingDepartmentRoles.value = true;
  try {
    await updateDepartmentRoles(department.id, [
      ...new Set([...department.roleIds, roleId]),
    ]);
    await loadOverview();
    selectedDepartmentId.value = department.id;
    selectedRoleId.value = roleId;
    pendingDepartmentRoleId.value = undefined;
    message.success('职位已加入当前部门');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '添加部门职位失败');
  } finally {
    savingDepartmentRoles.value = false;
  }
}

async function removeRoleFromDepartment(role: RbacRole) {
  const department = selectedDepartment.value;
  if (!department) return;
  savingDepartmentRoles.value = true;
  try {
    const roleIds = department.roleIds.filter((roleId) => roleId !== role.id);
    await updateDepartmentRoles(department.id, roleIds);
    await loadOverview();
    selectedDepartmentId.value = department.id;
    if (selectedRoleId.value === role.id) {
      selectedRoleId.value = roleIds[0];
    }
    message.success('职位已移出当前部门');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '移出部门职位失败');
  } finally {
    savingDepartmentRoles.value = false;
  }
}

function selectRoleForMenus(role: RbacRole) {
  selectedRoleId.value = role.id;
  activeTab.value = 'roleMenus';
}

function selectRoleForPermissions(role: RbacRole) {
  selectedRoleId.value = role.id;
  activeTab.value = 'rolePermissions';
}

function openRoleDrawer(role?: RbacRole) {
  editingRole.value = role || null;
  roleForm.name = role?.name || '';
  roleForm.slug = role?.slug || '';
  roleDrawerOpen.value = true;
}

async function submitRole() {
  await roleFormRef.value?.validate();
  savingRole.value = true;
  try {
    const currentDepartmentId = editingRole.value
      ? undefined
      : selectedDepartment.value?.id;
    const currentDepartmentRoleIds = editingRole.value
      ? []
      : [...(selectedDepartment.value?.roleIds || [])];
    const payload = {
      name: roleForm.name.trim(),
      slug: roleForm.slug.trim(),
    };
    const role = editingRole.value
      ? await updateRole(editingRole.value.id, payload)
      : await createRole(payload);
    if (currentDepartmentId) {
      await updateDepartmentRoles(currentDepartmentId, [
        ...new Set([...currentDepartmentRoleIds, role.id]),
      ]);
    }
    await loadOverview();
    if (currentDepartmentId) {
      selectedDepartmentId.value = currentDepartmentId;
    }
    selectedRoleId.value = role.id;
    roleDrawerOpen.value = false;
    message.success('职位已保存');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存职位失败');
  } finally {
    savingRole.value = false;
  }
}

async function removeRole(role: RbacRole) {
  if (role.isAdmin) return;
  try {
    await deleteRole(role.id);
    if (selectedRoleId.value === role.id) {
      selectedRoleId.value = undefined;
    }
    await loadOverview();
    message.success('职位已删除');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除职位失败');
  }
}

async function saveMenus() {
  if (!selectedRole.value || selectedRole.value.isAdmin) return;
  savingMenus.value = true;
  try {
    await updateRoleMenus(
      selectedRole.value.id,
      checkedMenuKeys.value.map(Number),
    );
    await loadOverview();
    message.success('菜单权限已保存');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存菜单权限失败');
  } finally {
    savingMenus.value = false;
  }
}

function togglePermission(permissionId: number, checked: boolean) {
  if (inheritedPermissionIds.value.has(permissionId)) return;
  const next = new Set(checkedPermissionKeys.value);
  if (checked) {
    next.add(permissionId);
  } else {
    next.delete(permissionId);
  }
  checkedPermissionKeys.value = [...next];
}

async function savePermissions() {
  if (!selectedRole.value || selectedRole.value.isAdmin) return;
  const explicitPermissionIds = checkedPermissionKeys.value.filter(
    (permissionId) =>
      permissions.value.find((permission) => permission.id === permissionId)
        ?.scope !== 'page',
  );
  savingPermissions.value = true;
  try {
    await updateRolePermissions(selectedRole.value.id, explicitPermissionIds);
    await loadOverview();
    message.success('权限标识已保存');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存权限标识失败');
  } finally {
    savingPermissions.value = false;
  }
}

function toTreeData(items: RbacMenu[]): TreeNode[] {
  return items.map((item) => ({
    title: item.uri ? `${item.title} (${item.uri})` : item.title,
    key: item.id,
    children: item.children ? toTreeData(item.children) : undefined,
  }));
}
</script>

<style scoped>
.position-scroll-hint {
  display: none;
  margin: 0 0 8px;
  font-size: 12px;
}

.permission-groups {
  margin-top: 12px;
  background: transparent;
}

.permission-group-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.permission-list {
  display: grid;
  gap: 8px;
}

.permission-item {
  display: grid;
  min-width: 0;
  gap: 6px;
  padding: 10px 12px;
  background: var(--kadmin-soft-bg);
  border: 1px solid var(--kadmin-border);
  border-radius: 8px;
}

.permission-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  color: var(--kadmin-muted);
  font-size: 12px;
}

.permission-code {
  overflow-wrap: anywhere;
  color: var(--kadmin-text);
}

@media (max-width: 768px) {
  .rbac-layout {
    margin-inline: 0 !important;
  }

  .rbac-layout > :deep(.ant-col) {
    padding-inline: 0 !important;
  }

  .position-scroll-hint {
    display: block;
  }
}
</style>
