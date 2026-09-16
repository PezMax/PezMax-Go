# data/migrations 目录说明

本目录保留历史迁移记录（仅 PostgreSQL 方言，其余方言文件已删除，原生仅支持 pgsql）。

- `admin_*.postgres.sql`：从 GoAdmin 到当前版本 KAdmin 的历史增量迁移，仅作追溯用途。
- `rollback/`：对应历史迁移的回滚脚本（仅 PostgreSQL）。

**权威种子脚本**：`tests/data/admin_pg.sql`（Docker Compose 挂载为
`/docker-entrypoint-initdb.d/10-admin_pg.sql`，首次创建 volume 时自动导入）。
它已合并全部历史迁移的最终结构与初始数据（菜单、权限、角色、字典、站点配置等，
包括代码生成与站内通知模块），新用户执行 `docker compose up -d postgres`
即可获得完整可用功能；存量库由后端启动时的模块 `EnsureSchema` 幂等补齐，
无需手工执行本目录中的任何脚本。

后续数据库结构变更请同步更新：模块 `schema.go`（启动迁移）与
`tests/data/admin_pg.sql`（全新安装快照）。
