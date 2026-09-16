-- 将 goadmin_menu 直接适配为当前 Vben 菜单目录结构（与 vbenapi/menu_seed.go 一致）。
-- 清理 GoAdmin 原生遗留菜单（Admin/Users/Roles/Permission/Menu/Operation log），
-- 重建 Dashboard / KAdmin 两棵树，并重置 role_menu 授权。
-- 幂等：可重复执行；使用单事务，失败整体回滚。

BEGIN;

-- 1. 清理旧菜单授权，避免重新编号后出现悬空引用
DELETE FROM goadmin_role_menu;

-- 2. 清空旧菜单
DELETE FROM goadmin_menu;

-- 3. 序列回到起点，使用干净连续的 id
SELECT setval('goadmin_menu_myid_seq', 1, false);

-- 4. 写入当前菜单目录（type=0 目录/分组，type=1 菜单；icon/uri 与 menu.go 绑定一致）
INSERT INTO goadmin_menu
    (id, parent_id, type, "order", title, header, icon, uri, uuid, plugin_name, created_at, updated_at)
VALUES
    (1,  0, 0,  1, 'Dashboard',  NULL, 'lucide:layout-dashboard',  '/dashboard',            NULL, '', now(), now()),
    (2,  1, 1,  1, '分析页',      NULL, 'lucide:area-chart',        '/dashboard/analytics',  NULL, '', now(), now()),
    (3,  1, 1,  2, '工作台',      NULL, 'carbon:workspace',         '/dashboard/workspace',  NULL, '', now(), now()),
    (4,  0, 0, 10, 'KAdmin 管理', NULL, 'lucide:settings-2',        '/kadmin',               NULL, '', now(), now()),
    (5,  4, 1,  1, '用户管理',    NULL, 'lucide:users',             '/kadmin/users',         NULL, '', now(), now()),
    (6,  4, 1,  2, '权限管理',    NULL, 'lucide:shield-check',      '/kadmin/rbac',          NULL, '', now(), now()),
    (7,  4, 1,  3, '菜单管理',    NULL, 'lucide:menu',              '/kadmin/menus',         NULL, '', now(), now()),
    (8,  4, 1,  4, '字典管理',    NULL, 'lucide:book-open',         '/kadmin/dictionary',    NULL, '', now(), now()),
    (9,  4, 1,  5, '参数配置',    NULL, 'lucide:sliders-horizontal','/kadmin/settings',      NULL, '', now(), now()),
    (10, 4, 1,  6, '资源工作台',  NULL, 'lucide:folder-kanban',     '/kadmin/resources',     NULL, '', now(), now());

-- 5. 序列指向当前最大 id，避免后续插入冲突
SELECT setval('goadmin_menu_myid_seq', (SELECT MAX(id) FROM goadmin_menu), true);

-- 6. 重新授权：管理员(role 1)拥有全部菜单；Operator(role 2)保留 Dashboard 子树，避免被完全锁住
INSERT INTO goadmin_role_menu (role_id, menu_id, created_at, updated_at)
SELECT 1, id, now(), now() FROM goadmin_menu
UNION ALL
SELECT 2, id, now(), now() FROM goadmin_menu WHERE id IN (1, 2, 3);

COMMIT;
