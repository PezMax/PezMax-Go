# PezMax Go 迁移：剩余待办清单

> 生成日期：2026-09-23。
> 本文件由 `pezmax-desktop-go-migration.md`（迁移总文档）与 `ruoyi-residues.md`（RuoYi 残留清单）整合而来：**已完成项已删除**，仅保留未完成工作；两份旧文档保留作历史底账，后续以本文件为准滚动更新。
> **2026-09-29 起第二阶段开发以 `pezmax-next-tasks.md` 为准**（本文件未竟项已并入其任务九，编号沿用；此处不再滚动更新）。
> 所有条目均已对照当前代码逐项核实（非照抄旧文档结论），证据以 `文件:行` 标注。

## 进度基线（一句话）

datum 七大子模块（数据层 / 认证 / 个人中心 / 文件 / 下载收藏 / 举报 / 通知）与基础组件（LibreOffice 转档、fileurl）**已全部落地并通过 E2E**；codegen 管理面 9 个 generated 视图已就位；后端工程收尾（权限审计 / Swagger）已完成（2026-09-24）。剩余工作集中在：**数据 ETL 与切换、admin-web 完善（含 #15 审核专用页面）**——桌面端已零旧后端依赖。2026-09-24 另完成对照 Java 原版的全端点缺口审计：新增 #18~#25，岗位 / 数据权限 / 在线用户 / 管理端注册等决策不迁移（见「不迁移决策」节）。

---

## P0 · 后端缺失端点（桌面端在用功能，阻断切换）

### 1. ~~书签管理端点~~ ✅ **已完成（2026-09-23）**

已实现 `k_admin/internal/kadmin/datum_bookmark.go`：匿名 `GET /datum/bookmark/list`（TableDataInfo 形状；支持 title/keyword 模糊、url 精确、resourceType/collection/subject 过滤；匿名仅出已审核，带 userId 出该用户全部状态）、匿名 `GET /datum/bookmark/:id`（待审仅属主可见）、属主 `POST/PUT/DELETE /datum/bookmark`（PUT 为部分字段合并语义，兼容桌面端字符串/数字双形态 id）、`POST /datum/bookmark/uploadCover`（multipart ≤5MB，JPG/PNG/GIF/WEBP 嗅探，先验属主再落存储，响应 `{bookmarkId,url}`）。gin v1.3 GET 树冲突：`/datum/bookmark/favorite/list` 并入通配分发器（活动模块只保留 POST）。配套修复桌面端 `SidePanel.vue` 三处 `code===200` 硬判定（列表/保存/封面上传，兼容 `code===0`），`electron-vite build` 通过；Go 侧 6 个新测试 + 既有 datum 套件全绿。

### 2. ~~平台用户管理端点~~ ✅ **已完成（2026-09-23）**

已实现 `k_admin/internal/kadmin/datum_user_admin.go`：匿名 `GET /datum/user/list`（TableDataInfo 形状；userName 模糊 + status 过滤 + 分页）与匿名 `GET /datum/user/:userId`（公开投影，载荷不含 password 哈希；排行/抽屉/编辑器三处桌面消费方解锁）；属主管理写 `POST/PUT/DELETE /datum/user`（PUT 为局部合并语义，支持桌面端数字/字符串双形态 id 与 status；封禁即 status '0'，重名 409，自删保护，删除为硬删并连带清理 ptmj_security）、`POST /datum/user/resetSecurityAnswers`（按用户名三组成套替换（答案重新哈希）或全空清除，部分提供 400）。封禁/解封/删除/计数调整后失效排行哈希缓存（同上传链路）。gin v1.3 GET/DELETE 树冲突：`/datum/user` 全部读接口与 rank、rank/cache 清理并入通配分发器（`datumUserGet`/`datumUserDeleteDispatch`，datum_file.go 原直挂 rank 路由已移除）。配套修复桌面端 `MainEditor.vue`/`FileInfoDrawer.vue` 两处 `code===200` 硬判定（兼容 `code===0`），`electron-vite build` 通过；Go 侧 6 个新测试 + 既有 datum 套件全绿。注：管理写接口当前仅 `requireDatumAuth` 门禁，`datum:user:*` 细粒度权限按 P1 #8 统一接线。

### 3. ~~`/unlockscreen`（锁屏解锁）~~ ✅ **已移除（2026-09-24 决策：生产环境功能异常无法触发，非重要功能）**

连根移除桌面端锁屏功能（排查发现 `/lock` 路由本就未在 router/index.js 注册，功能早已断裂、`lock.vue` 从未挂载）：删除 `api/login.js` 的 `unlockScreen()`（POST 根路径 `/unlockscreen`，Go 从未实现）、`views/lock.vue`、`store/modules/lock.js`，并剥离 `Navbar.vue` 锁屏入口、`permission.js` 锁屏路由守卫分支、`user.js` 登录后的 `unlockScreen()` 调用。`electron-vite build` 通过。

### 4. ~~批量过审 `PUT /datum/file/approvePendingByUser/{userId}`~~ ⏸ **延后（2026-09-24 决策）**

原 Java 契约存在，Go 未实现（`datum_file.go:140-148` 无此路由）。**决策：后台批量过审功能先延后实现**——预计在 admin-web 重建一个审核专用页面，统一进行试卷/书签审核（见 P2 #15），过审动作与批量端点随该页面一并设计接线（实现时注意过审后失效树/排行哈希缓存，同上传链路）。过渡期管理面继续走 SQL/generated 视图。

---

## P0 · 桌面端旧契约切换（迁移必办）

### 5. ~~通知铃铛 HeaderNotice 切换~~ ✅ **已完成（2026-09-23，采方案 2）**

桌面端铃铛改接 datum popup 端点、本地记已读，旧契约目录已删：

- `HeaderNotice/index.vue`：改调 `getUserPopupNotifications`（`/system/notification/user/popup`，与 NotificationCenter/home 弹窗同源）；用户 ID 兜底链与 home/index.vue 一致（userStore.id → getInfo）。popup 载荷已含正文，预览不再二次请求详情；类型标签改为 5 类型映射（1-版本更新 2-系统故障 3-系统维护 4-资料下架 5-公告）。
- 已读记账：`ptmj_notification` 无 per-user 已读模型，按桌面端本地库惯例落主进程 SQLite——`main-utils/database.js` 新增 `notice_reads` 表（UNIQUE(user_id, notify_id)，INSERT OR IGNORE 幂等）+ `listNoticeReadIds`/`markNoticesRead`；`main/index.js` 新增 `notice-read:list`/`notice-read:mark` IPC；`preload/index.js` 暴露 `electronAPI.noticeReads`。已读写入即刷盘；非 Electron 环境降级为空集（均视为未读）。
- `src/renderer/api/system/` 整目录已删除（全仓 grep 无 `/system/notice` 残留）。`electron-vite build` 通过；sql.js 直测 notice_reads 建表/幂等/按用户隔离通过。

### 6. ~~前端兼容分支清理~~ ✅ **已完成（2026-09-24）**

- `utils/request.js`：响应拦截器删除 RuoYi 信封分支（HTTP 200 + `code===500` 兜底、`code===601` 警告、`errorCode` 映射、in-body 401 双轨）——HTTP 200 即业务成功（kadmin `body.code` 恒为 0），错误以真实 HTTP 状态码走 error 分支；`download()` 非文件响应兜底改读 `msg/message`。
- `utils/errorCode.js`（B3）删除：唯一剩余消费方为孤儿插件 `plugins/download.js`（指向 Go 侧从未实现的 `/common/download*`，`$download` 全仓零调用），一并删除并注销 `plugins/index.js` 注册。
- `store/modules/user.js` login action：删除 `res.token` 双格式分支，仅读 `res.data?.token`（kadmin 信封）。
- 孤儿清理：删除 `views/datum/security/index.vue` + `api/datum/security.js`（`/datum/security` CRUD 无路由挂载、端点未实现，不迁移）。
- `electron-vite build` 通过；全仓 grep 无 `errorCode`/`datum/security`/`$download` 残留。

---

## P1 · 工程收尾（Go 侧）

### 7. ~~Excel 导出端点接线~~ ✖ **已取消（2026-09-24 决策：管理端 Excel 导出非必要）**

相关代码已全部移除：`internal/kadmin/platform/excelx` 组件（excelx.go + 测试，datum 侧本就零引用）；桌面端 datum/file、datum/report、datum/notification 三个管理视图的导出按钮与 `handleExport`（调用的是从未实现的 `/datum/*/export`）及其孤儿依赖——`utils/request.js` 的 `download()`、`main.js` 全局注册——一并删除。`go build ./internal/...`、`go vet ./internal/kadmin/...`、`electron-vite build` 均通过。`/datum/file/export`、通知管理导出两个端点不再实现。

### 8. ~~权限与审计接入~~ ✅ **已完成（2026-09-24）**

datum 管理侧写端点全部从 datum 会话切到**管理端 JWT + `requirePermission` + `datum:*` 权限标识**（属主语义的桌面端读写——资料/书签 CRUD、收藏下载、个人中心、举报提交——仍走 datum 会话，不受影响；这些管理写的桌面消费方此前已确认全部是孤儿视图）：

- 权限拆分：`datum:user:manage`（平台用户 CRUD / resetSecurityAnswers / rank 缓存清理）、`datum:file:manage`（file-tree 缓存清理）、`datum:notification:manage`（通知 CRUD）、`datum:report:audit`、`datum:bookmarkReport:audit`，种子入 `bootstrap/permissions.go`（按钮级，PageURI 指向既有 generated 管理页）。
- 幂等：datum 组挂载 `idempotencyMiddleware`，`POST /datum/user`、`POST /datum/notification` 要求 Idempotency-Key（对齐 /api 管理创建）。
- 审计：datum 组挂载 `businessAuditMiddleware` + 专用描述器（`datum_business_audit.go`），覆盖用户/通知 CRUD 与两类举报审核共 8 类管理变更；操作人归属 `datumActorName` 优先记录管理员名。
- 菜单：generated 模块 `ensureMenu` 已自注册 /business 目录下 9 个管理页入口，bootstrap 种子无需重复。

### 9. ~~Swagger 文档~~ ✅ **已完成（2026-09-24）**

新增 `swagger_datum.go`：77 个 datum 桌面端端点的 swag 注释锚点（按 认证/用户/文件/书签/活动/通知/举报 分 Tag，标明匿名 / datum 会话 / 管理端 JWT+权限 三档门禁与 RuoYi TableDataInfo 形状），补 `SwaggerTableDataResponse` 模型；`go:generate` 重新生成 docs。顺带修复了历史遗留的 `TestSwaggerDocumentsAllKAdminRoutes`（旧 spec 未含 generated 模块注解）与 `TestRegisterApplicationRoutesIncludesEveryModule`（product 示例表已在 fad4279 移除注册但测试期望未同步）。

### 18. ptmj_file_favorite 管理端 CRUD（generated 模块补齐）

2026-09-24 缺口审计发现：10 张 `ptmj_*` 表中 **`ptmj_file_favorite` 是唯一没有管理入口的**——generated 注册 9 个模块（bookmark / bookmark_favorite / bookmark_report / file / file_download / notification / platform_user / report / user_security），唯独缺 file_favorite；Java 原版管理侧 `GET /datum/favorite/list`（分页）、`PUT /datum/favorite`、`DELETE /datum/favorite/{fileIds}` 三个端点 Go 均未实现。桌面端收藏读写不受影响（`/datum/favorite` POST + `GET /:fileId`、`/datum/desktop/favorite/*` 已可用），缺的是管理后台对该表的可见性。**方案**：按 codegen 流程补生成 `generated/file_favorite` 模块（5 条 CRUD + 权限种子 + 菜单自注册），与既有 9 模块同构。

### 19. 封号机制补全：审核联动封禁 + 存量会话失效

决策背景：**不禁止一号多用**（不做在线用户强退），但封号必须即时生效。当前两处缺口（2026-09-24 审计确认）：

- **审核联动**：Java `POST /datum/report/handle` 支持审核属实时可选 `banUser` 封禁被举报文件的上传者；Go 的合并实现 `POST /datum/report/audit/:reportId`（datum_report.go）只有"属实→文件下架 status 3 + type-4 通知"，无封禁分支。书签举报审核同构。随 #15 审核专用页面一并设计接线。
- **会话失效**：`requireDatumAuth`（datum_user_auth.go）仅解析 Redis datum 会话得到 userID，**不复查 `ptmj_user.status`**——管理端封禁（`PUT /datum/user` status '0'）后，被封用户存量会话仍可用至 5 天 TTL 过期。需在 datum 会话校验链路补状态复查（或封禁时主动清除该用户全部 datum 会话键），并同步失效排行哈希缓存（现有链路已做）。注意复查 status 需容忍读库瞬断（降级放行或短暂 503 需明确取舍）。

---

## P1 · 数据迁移与切换（C 项，生产切换前必办）

实施细节（类型映射 7.3、对账红线 7.4、ETL 红线）见旧文档 `pezmax-desktop-go-migration.md` 第七章，此处只列待办：

| # | 任务 | 备注 |
| --- | --- | --- |
| 10 | MySQL 3306 → PG ETL：10 张 `ptmj_*` 表，**显式插旧主键**，迁完逐表重置 identity 序列到 `max(id)+1` | 必须脚本化可重跑；数据量约 2500 行 |
| 11 | MinIO `mc mirror` 旧桶 → 新桶 | bucket 名沿用 `ptmj`；核对对象数与总大小；新侧初始化公共读 policy |
| 12 | 存量 `file_url` 批量 UPDATE（localhost/内网 → 公网前缀） | **可延后**：未执行前桌面端 `utils/url.js` 本地兜底继续生效；新上传由 `fileurl.Builder` 保证正确 |
| 13 | 桌面端冒烟切换：老账号登录 → 文件树 → **下载一个旧 ID 文件** → 上传新文件 | 旧 MySQL + 旧后端保留作回滚，稳定后再切生产 |
| 14 | `.env.production` 的 `127.0.0.1:9033` 换真实生产域名 | 当前已指向 kadmin 端口，仅剩域名替换 |

---

## P2 · 可选增强

| # | 任务 | 说明 |
| --- | --- | --- |
| 15 | admin-web 管理增强 | **重建审核专用页面**（2026-09-24 决策，取代原"generated 视图之上补审核入口"）：统一承载试卷（ptmj_file）/书签（ptmj_bookmark）审核与批量过审，端点随页面接线（见 #4）；**审核属实的可选封禁上传者分支随本页一并设计（见 #19）**；其余不变：举报处理时间线视图、通知 5 类型表单、platform_user 视图加重置密保入口 |
| 16 | 内嵌 admin 入口下线 | `VITE_AUTH_ENTRY_MODE=admin`（`electron/main/index.js` 仍在读）——改为跳转 admin-web 链接或整体移除 |
| 17 | 前端长期项（B 类） | B1 `ruoyi.js` 更名拆分、B4 记住密码改系统凭据库、B7 权限指令对齐 kadmin 权限码、D3 `/dev-api` 更名（需同步 vite proxy 与 electron 主进程） |
| 20 | 批量删除对齐 | `DELETE /datum/file/:fileId`、`DELETE /datum/user/{userId}` 仅支持单个 ID，Java 契约为 `{fileIds}`/`{userIds}` 逗号分隔批量（举报/下载记录的 `:ids` 已是批量）。随 #15 审核页面评估是否接线；generated 视图单删亦可用作过渡 |
| 21 | Redis 缓存管理面 | Java `/monitor/cache` 7 个接口（信息/名列表/键列表/取值/按名清/按键清/全清）无对应；kadmin 目前无任何缓存运维入口。可并入 system-monitor 扩展 |
| 22 | 管理端个人中心 | 当前管理员改密码/改资料（Java `SysProfileController` 的 updateProfile/updatePwd）；kadmin 仅有管理员重置他人密码（`PUT /api/users/:id/password`）。结合"密保构建完整账户"体系设计改密验证 |
| 23 | XSS 请求体过滤评估 | Java `XssFilter` 默认开启（清洗 /system/* /monitor/* /tool/* 请求体）；kadmin 仅审计字段脱敏，无请求体清洗。JSON API + 参数化查询下风险有限，评估后决定是否实现 |
| 24 | 桌面端 `/common/upload` 遗留依赖清理 | FileUpload / ImageUpload / Editor 三个 RuoYi 通用组件默认 action 与 `SidePanel.vue` 的 `uploadUrl` 变量均指向 Go 侧从未实现的 `/common/upload`（当前无实际触发点，属潜在坑）；改接 `/api/files` 或删除未用组件 |
| 25 | `POST /datum/file/upload` 独立直传 | Java 桶根直传（仅返回 fileUrl，不落库）；桌面端主流程不使用。仅当 #15 审核页/编辑器需要附件直传时实现 |

---

## 不迁移决策（2026-09-24 移植缺口审计）

对照 Java 原版（`pezmax/1.2/PezMax-Backend`）全端点审计后，以下能力**决策不迁移**，后续审计不再重复标记：

| 能力 | Java 原版 | 不迁移理由 |
| --- | --- | --- |
| 岗位管理 SysPost | `/system/post` 全部 8 接口 | 产品端使用独立身份验证机制，kadmin 岗位机制设计上不生效；产品模块下的 user 是 pezmax 用户，无法登录后台，仅有应用端操作权限 |
| 部门/角色数据权限 @DataScope | role `PUT /dataScope` + AOP 切面 | 同上——部门/角色维度数据权限不适用 |
| 在线用户管理/强退 | `/monitor/online/list` + 强退 | 不禁止一号多用；账号治理走封号机制（见 #19） |
| 管理端注册 | `POST /register`（开关控制） | 管理账户由管理员创建并以密保完善，后端注册功能多余 |
| codegen 补齐 / jobLog 清理 | `createTable`/`synchDb`/`batchGenCode`、jobLog 批删/clean | 已列入 kadmin 更新计划，不计入本清单 |
| Excel 导出 | 全部 export 端点 | 已取消（见 #7） |

## 建议实施顺序

1. ~~**#1 书签端点**~~（✅ 已完成）→ ~~**#3 unlockscreen**~~（✅ 2026-09-24 随锁屏功能移除）→ ~~**#5 铃铛切换**~~（✅ 已完成）→ ~~**#6 前端兼容分支清理**~~（✅ 已完成）：**桌面端零旧后端依赖已达成（2026-09-24）**。
2. ~~**#2 用户管理**~~（✅ 已完成）+ ~~**#8 权限审计**~~（✅ 已完成）+ ~~**#9 Swagger**~~（✅ 已完成）：后端收尾批次全部完成（**#4 批量过审已延后**随审核专用页面、**#7 Excel 已取消**）。
3. **#10~#14 数据迁移与冒烟切换**：上两步完成后再动数据。
4. P2 项按需排期。



## 其他
~~完成电子书下载模块，电子书模块与文件模块类似，共享qos~~ → 后端契约已就位（2026-09-29）：电子书 = `ptmj_file` 的 fileType=7（`datumEbookFileType`，已入 datumFileTypeNames/hash 树/上传格式白名单 pdf/epub/mobi/azw3）；`GET /datum/ebook/list` 分页输出已过审电子书（行结构同 /datum/file/list）；**下载不设专有端点**，统一走 `GET /datum/download/file?fileId=`（datum 会话鉴权 + ptmj_file_download 记录 + nginx /datum/download/ 弹性限速）。剩余为桌面端电子书页面 UI 接线与电子书内容上架。


## 完成勾销记录

- 2026-09-23 建档：全量核对代码后整合两份旧文档；无勾销项。
- 2026-09-23 勾销 #1 书签管理端点：datum_bookmark.go 落地 + SidePanel code===0 兼容补丁，详见 P0 第 1 节。
- 2026-09-23 勾销 #2 平台用户管理端点：datum_user_admin.go 落地 + /datum/user 通配分发器重构 + MainEditor/FileInfoDrawer code===0 兼容补丁，详见 P0 第 2 节。
- 2026-09-23 勾销 #5 通知铃铛切换（方案 2）：HeaderNotice 改接 popup 端点 + 主进程 SQLite notice_reads 表记已读 + 删除 api/system/ 目录，详见 P0 第 5 节。
- 2026-09-24 勾销 #6 前端兼容分支清理：request.js 删除 RuoYi 信封分支（code 500/601、errorCode 映射、in-body 401 双轨）+ user.js 删除 res.token 双格式 + 删除孤儿 security 视图/API 与 errorCode.js、plugins/download.js，详见 P0 第 6 节。
- 2026-09-24 #4 批量过审延后（决策）：后端收尾批次不再实现 `approvePendingByUser`，改为 admin-web 重建审核专用页面统一进行试卷/书签审核，过审端点随页面一并接线；#15 同步更新，详见 P0 第 4 节。
- 2026-09-24 勾销 #7 Excel 导出（决策取消）：excelx 组件删除 + 桌面端三视图导出按钮与 download() 工具移除，详见 P1 第 7 节。
- 2026-09-24 勾销 #3 unlockscreen（决策移除）：锁屏功能连根删除（API/store/视图/入口/守卫分支），详见 P0 第 3 节；桌面端零旧后端依赖自此达成。
- 2026-09-24 勾销 #8 权限与审计：datum 管理写全部切管理端 JWT + datum:* 权限 + 幂等/审计中间件，详见 P1 第 8 节。
- 2026-09-24 勾销 #9 Swagger：swagger_datum.go 77 端点注释 + spec 重新生成，详见 P1 第 9 节。
- 2026-09-24 移植缺口全量审计（对照 Java 原版全端点 + 双前端调用交叉验证）：新增 #18 file_favorite 管理端 CRUD、#19 封号机制补全（审计时确认封禁后存量 datum 会话不失效）；#20~#25 入 P2。岗位 / 数据权限 / 在线用户 / 管理端注册决策不迁移，codegen/jobLog 留在 kadmin 更新计划，详见「不迁移决策」节。
