# PezMax-Go 后续开发任务清单（第二阶段）

> 生成日期：2026-09-29。
> 定位：承接 `pezmax-remaining-tasks.md`（第一阶段迁移收尾清单，未竟旧项见本文任务九），按本轮新目标**按提出顺序**整理为任务一~七，任务八起为本轮代码核查新发现的遗漏项，任务九并入旧清单未竟项。
> 所有条目均对照当前代码与 Java 原版（`pezmax/1.2/PezMax-Backend`）逐项核实，证据以 `文件:行` 标注。

## 0. 影响排期的三个关键结论（本轮核查得出）

1. **CDN、QoS、电子书在 Java 原版中不存在**（全仓大小写不敏感检索 `cdn`/`qos`/`ebook`/`电子书`/`限速` 均为零命中；下载是无节流的 8KB 缓冲代理循环，`PtmjFileDownloadServiceImpl.java:88-122`）。这三项是**新增能力而非移植**，没有旧契约可对照，按新产品特性设计。
2. **下载链路现状（Go 侧已统一）**：全部走服务端代理流式端点 `GET /datum/download/file?fileId=`（datum 会话鉴权 + `ptmj_file_download` 记录 + Range 断点续传 + ETag，`datum_activity.go:473-536`；MinIO 侧 Seek 触发新的带 Range GET，`storage/minio.go:469-528`）；桌面端 Electron 主进程 `.part` 文件续传（`src/main/index.js:508-653`）。边缘 nginx（compose profile `edge`）已有**弹性分档限速**，但**无任何缓存能力**（`deploy/nginx.conf.template` 仅三个 location，零 `proxy_cache`）。无 presigned URL、无 X-Accel-Redirect。
3. **Redis 现状**：共 5 类用途——admin JWT 刷新令牌/JTI 黑名单（`auth_jwt.go:204-283`）、验证码+登录锁定+幂等中间件（`security.go`）、datum 桌面端会话（`datum_user_auth.go:38-135`）、树/排行 `{hash,payload}` 哈希缓存（`datum_file.go:57-131`）。**排行榜真数据是 SQL top50（`stats.go:28-50`），Redis 只是缓存层**；但 `stateStore.load` 对 Redis 故障零降级——Redis 不可用时排行直接 503（`datum_file.go:493-495`），这是「排行榜显示异常」的头号嫌疑（见任务五）。

---

## 任务一 · CDN 剪切机制定案与落地（或确定替代方案）

**用户目标原文**：完成此特殊开发环境下的 cdn 剪切机制（或找到替代）。

### 现状与约束

- 本仓库部署形态：docker-compose 单机（kadmin + postgres + redis + minio + 可选 edge nginx）。`KADMIN_MINIO_PUBLIC_BASE` 是 URL 出口的唯一切换点（`fileurl.go:55-60`，compose 默认注入 `http://127.0.0.1:29000`，`docker-compose.yml:127`）。
- 「特殊开发环境」下（无公网域名/无法接真 CDN 的内网部署），下载流量当前 100% 回源 kadmin 进程，MinIO 公共读桶直链只用于封面/头像等静态资源。
- 边缘 nginx 模板头部（`nginx.conf.template:1-35`）已自述设计缺口：仅 per-connection 整形，无跨连接全局整形，建议 `tc` 或应用侧令牌桶——这两条路与 CDN 剪切需一并定案。

### 方案选项（需先出决策记录，再实施）

| 方案 | 内容 | 取舍 |
| --- | --- | --- |
| A. edge nginx 升级为「迷你 CDN」（推荐替代） | `/datum/download/` 增加 `proxy_cache`（热点对象边缘缓存 + `stale-while-revalidate` + 磁盘配额），保留鉴权回源；命中后不再消耗 kadmin/MinIO 带宽 | 零新增组件；鉴权与下载记录语义不变（首字节仍回源）；缺点是缓存仍在本机，只是进程卸载 |
| B. MinIO presigned 短链直连分发 | 下载端点改签发 302/presigned URL，客户端直连 MinIO | 真正「剪切」后端流量，但**绕过 datum 会话鉴权与 `ptmj_file_download` 记录**，需短时效签名 + 记录改预写或回调，改动面大；且 `ptmj` 桶当前公共读，签名意义弱化 |
| C. 真 CDN 接入 | 域名 + 回源 + `KADMIN_MINIO_PUBLIC_BASE` 切换 | 环境允许时才可行，本期仅保证该配置项为单一切换点，不做实施 |

### 待办

1. **决策记录**：A/B/C 选型与理由写入 `docs/`（本文档此处回填），明确「特殊开发环境」定义（内网/无域名/单机）。
2. 若采方案 A：`nginx.conf.template` 补 `proxy_cache_path` + `/datum/download/` 缓存指令（注意 Range 请求缓存策略与 `Authorization` 头不得进 cache key）；`stub_status` 观测命中率；compose `edge` 服务挂缓存卷。
3. 若采方案 B：presigned 端点 + 下载记录补偿机制设计（与任务三 QoS 联动：直连后 nginx 对 MinIO 端口同样需要限速 location）。
4. 验收：缓存命中时 kadmin 无回源日志、断电重启 edge 后热点文件仍可下载；带宽受任务三档位约束。

**关联**：与任务三同一链路，建议协同设计后一起实施。

---

## 任务二 · 电子书模块桌面端开发

**用户目标原文**：电子书模块的桌面端开发。

### 现状

- **后端契约已就位（2026-09-29）**：电子书 = `ptmj_file` 的 `fileType=7`（`datum_ebook.go:23`）；`GET /datum/ebook/list` 分页输出已过审电子书（行结构同 `/datum/file/list`）；下载不设专有端点，统一走 `GET /datum/download/file`（鉴权 + 记录 + nginx 限速）；上传白名单已含 `pdf/epub/mobi/azw3`（`datum_file.go:509-513`）；类型 7 已入类型字典与树聚合。
- **桌面端仅占位**：`views/datum/ebook/index.vue` 是静态"功能建设中"页；home IDE 内嵌 `EbookView`（`ActivityBar.vue:19` → `home/index.vue:16-18`）；`api/datum/` 下无 ebook 模块，全 api 目录零 "ebook" 字符串。
- ⚠️ `modules/datum/schema.go:199-264` 的 `ptmj_ebook`/`ptmj_ebook_download`/`ptmj_ebook_favorite` 三张占位表**无任何运行时消费方**（仅 `ptmj_ebook_report` 有 generated 管理端）。桌面端开发**维持 fileType=7 单源**，不要启用这四张表。

### 待办（桌面端）

1. 新建 `api/datum/ebook.js`：`GET /datum/ebook/list`（复用 `normalizeKAdminPage` 分页桥接，参照 `api/datum/file.js`）。
2. `EbookView`（home 内嵌视图）实现：列表/分页/关键词搜索/封面或类型徽标（pdf/epub/mobi/azw3）、点击看详情（复用 `FileInfoDrawer`）、下载接入 `downloadFileDirectly`（注意先修任务八.1 的 `savedPath` 字段 bug，否则本地记录 localPath 恒空）。
3. 收藏/举报复用文件链路（fileId 同源，`favorite`/`report` API 直接可用）。
4. 上传入口：`UploadPanel` 类型下拉补「电子书(7)」选项（后端白名单已就位）；确认转档链路对 pdf/epub 不触发 LibreOffice（非 office 格式本就跳过）。
5. 阅读体验（可选增强）：pdf 用现有 `MainEditor` 预览 iframe；epub/mobi/azw3 无浏览器原生预览，先走「下载后本地打开」。
6. **运营项：电子书内容上架**（首批内容导入 + 过审），无内容则上线即空栏。

### 验收

列表 → 搜索 → 详情 → 下载（本地 SQLite 记录 + 服务端 `ptmj_file_download`）→ 我的下载页可见，全链路手工回归；`/datum/ebook` 路由页与 home 内嵌视图行为一致。

---

## 任务三 · QoS 限制策略补全

**用户目标原文**：qos 限制策略。

### 现状（已实现的部分）

- 边缘 nginx 弹性分档（`deploy/nginx.conf.template:44-54`）：按 `$connections_writing` 自适应——0-1 并发→FAST（默认 `5m`）、2-3→MID（`2560k`）、4+→LOW（`640k`）；档位默认值已在 8ca5e83 翻五倍。
- `/datum/download/` location（88-95 行）：per-IP `limit_conn 1` + 全局 `limit_conn ${KADMIN_EDGE_DL_MAXCONN}`（默认 16）+ `limit_rate`。
- env 单源：`KADMIN_EDGE_DL_RATE_FAST/MID/LOW`、`KADMIN_EDGE_DL_MAXCONN`（`docker-compose.yml:175-179`、`.env.example:30-33`）。
- 设计立场已注明：QoS 只在网关层，与试卷下载共用链路（`modules/datum/schema.go:227`）。

### 缺口（按优先级）

1. **档位配置系统化**：档位目前是 env 注入模板，改档要重启 edge 容器；且 `system_config` 里无任何 rate/limit/qos 配置面（grep 零命中）。→ 方案：档位入 kadmin 系统配置（热读）+ edge 侧定期拉取（`/api/` 端点或定时 envsubst 重建），或明确接受"改档=重启边缘"并写入部署文档。
2. **全局令牌桶缺失**：nginx 原生 `limit_rate` 只按连接整形，多连接可叠加突破总带宽（模板头部注释已自认）。→ 应用侧 egress 限速中间件（Go，`http.ResponseWriter` 包装按字节配额），或宿主机 `tc`。选型与任务一方案联动。
3. **每用户维度缺失**：现仅 per-IP；「不禁止一号多用」决策下，同一账号多 IP 可绕过 per-IP 限制。→ 下载端点按 datum userID 做并发/频次核算（Redis 计数，依赖任务五的前置项）。
4. **下载取消能力**：`src/main/index.js` 全程无 `request.abort()` 调用，用户无法取消下载释放连接配额。→ IPC 取消 + 浮动进度条取消按钮（与任务六可中断加载同批实施）。
5. 电子书与试卷共享链路（`/datum/download/`），若电子书文件更大更热，评估是否需要独立档位或独立 location。

### 验收

并发 N 路下载实测：总带宽 ≤ 目标值（验证令牌桶）、档位随负载切换（验证 stub/日志）、单用户多连接/多 IP 无法突破单用户配额；取消下载即时释放连接与带宽。

---

## 任务四 · IP/URL 映射转换：后台新页面控制（先开源公版，再独立修脏库）

**用户目标原文**：ip 映射转换，由后台新页面控制（现有的需要重新上线的数据库中的 url 地址错误，minio 的 url 地址被错误的改为了本地，先完成开源可公用版本，再独立解决该问题）。

### 现状

- 写入侧已保证正确：`fileurl.Builder` 用 `KADMIN_MINIO_PUBLIC_BASE` 组装公网 URL 入库，未配置则启动报错（`platform/fileurl/fileurl.go:28-87`）。
- 读取侧对存量脏数据**基本放行**：`datumReadableFileURL`（`datum_file.go:334-376`）只重建 `minio://` 形态，http 内网 host 原样透传，依赖桌面端 `normalizeFileUrl` 客户端兜底改写（`utils/url.js:15-49`，命中 `localhost|127.0.0.1|minio|server|172.*` 即换 host）。
- **后端没有任何管理端点可批量修正存量 `file_url`**（旧清单 #12 一直标"可延后"）；Java 原版仅有过一次性的 SQL `REGEXP_REPLACE` 迁移脚本（`sql/update_file_url_add_school.sql:38-48`），无运行时机制。
- 待重新上线的库：minio URL 被误改为本地地址（用户报告），即任务背景中"先公版、后私修"的私修对象。

### 待办 · 第一阶段：开源公版（通用能力，不写死任何 host）

1. **映射规则模型**：新表（或 system_config 结构化项）存「旧 host/前缀 → 新 host」规则列表，支持启用/停用、备注、排序；管理端 CRUD。
2. **admin-web 新页面「存储地址映射」**（Vben，挂 system 目录）：
   - 规则维护列表；
   - **dry-run 预览**：选定目标表（`ptmj_file.file_url`、书签封面 `cover_url`、用户 `avatar`）+ 规则，返回受影响行数与前后样例 diff（抽样 N 行）；
   - **执行批量 UPDATE**：事务内分批（如 500 行/批）、进度回报、执行前自动导出受影响行备份（JSON/CSV 落 MinIO 或本地）；
   - 执行历史与**一键回滚**（按备份重放）。
3. **后端端点**：`/api/url-mappings` CRUD + `/api/url-mappings/preview` + `/api/url-mappings/apply` + `/api/url-mappings/rollback`；管理端 JWT + 新权限码（如 `system:url_mapping:*`）+ 幂等 + 审计中间件（对齐 datum 管理写惯例，见旧清单 #8 的做法）。
4. **可选开关「读取时改写」**：响应组装前按启用规则改写 URL（作为桌面端 `normalizeFileUrl` 的服务端替代），默认关——存量修完后无需开启。
5. 约束：纯 host/前缀映射，不做路径段重排（历史那次"插入 school 段"的形态变更不纳入公版）。

### 待办 · 第二阶段：私有脏库修复（独立执行，不进开源主干）

1. 用公版工具对目标库 dry-run（规则：误改的本地 host → 真实 minio 公网 host）→ 核对抽样 diff → 执行 → 回验。
2. 修复后桌面端 `normalizeFileUrl` 保留一个版本周期作兜底，随后在任务八清理批次中标记弃用（服务端已保证正确 + 库已修正后，客户端改写反而可能改错新地址）。

### 验收

dry-run 计数与实际 UPDATE 行数一致；抽样旧文件下载/预览/头像/封面全部正常；误执行可从备份回滚；开源仓库内无任何写死的目标 host。

---

## 任务五 · Redis 应用扩展 + 排行榜显示异常修复

**用户目标原文**：开发 redis 其他应用，现在只用来存储 token 令牌，排行榜本依赖 redis 但是现在显示异常。

### 澄清与诊断（排行显示异常）

实际用途不止存 token（见「0.3」共 5 类），但排行链路有一个真实缺陷：**Redis 故障 → 排行 503 零降级**（`stateStore.load` 对非 miss 的 Redis 错误直接上抛，`datum_file.go:88-92、493-495`；文件树同链路）。桌面端 rank 视图的信封/unchanged/字段兼容逻辑本身核对无误（`rank/index.vue:246-258` 双兼容 `code 200/0`、unchanged 回退本地缓存、`count/uploads` 双字段已适配）。

**排查顺序**（按嫌疑排序）：

1. 部署环境 Redis 是否可用（b88e814 刚加 redis 中间件，若实例未起/地址错，排行与树全部 503）；
2. 库内数据：`TopUploaders` 要求 `status='1' AND count>0`（`stats.go:28-50`）——新库/重上线库若 count 未迁或为 0，榜为空；
3. 头像 URL 脏数据（内网 host）导致渲染破图（视觉上"显示异常"）；
4. 上述都不是再抓包看响应体。

**修复**：`stateStore.load` 增加降级——Redis 错误时记告警日志、直接 `compute()` 返回且不写缓存；排行/树在 Redis 故障时退化为直查 SQL 而非 503。

### 前置小任务：Redis 客户端能力补齐

现用自研 RESP 客户端仅覆盖所需命令（`auth_jwt.go:42-90`），无 ZSet/Scan/TTL 等。扩展下列应用前先补齐命令面（或评估换成熟客户端），避免每个用途手搓协议。

### 扩展应用（按价值排序，可选做）

| # | 应用 | 说明 |
| --- | --- | --- |
| 1 | 热门下载榜 / 最新上传 | 真排序集场景（ZSet），或复用现有 `{hash,payload}` 协议低成本实现 |
| 2 | QoS 支撑（对接任务三） | 每用户下载并发/频次计数（Lua INCR 已有先例 `security.go:415-470`）、应用侧全局令牌桶状态 |
| 3 | 缓存运维面（旧清单 #21 并入） | 管理端查看 Redis 信息/键列表/按前缀清理（对齐 Java `/monitor/cache` 7 接口的 subset），并入 system-monitor |
| 4 | 封号会话失效（旧清单 #19 半条并入） | 封禁/删除用户时 `DEL` 其全部 datum 会话键：需会话键→用户反向索引（如 `SET <prefix>:datum:user:<uid> → token 列表`）或 SCAN |
| 5 | 树/排行缓存预热与续期 | 启动时预热 + TTL 滑动，避免过期后首请求慢 |

### 验收

`docker stop redis` 后排行/文件树仍可用（仅告警日志、无缓存）；新增用途各有 Go 测试；封禁用户存量会话立即失效（补 #19 验证）。

---

## 任务六 · 桌面端加载策略：可中断 + 可跳转

**用户目标原文**：桌面端加载策略更改，加载时可中断加载并跳转至其他页面。

### 现状（阻塞链完整画像）

- `GlobalLoader` 全屏遮罩 `z-index:9999` 吞掉全部点击，无跳过按钮（`GlobalLoader.vue:47-64`）。
- home 启动：`onMounted` 串行 `await fetchTreeData() → refreshFavoriteIds() → loadPopupNotifications()`，最后**固定 400ms 定时器**才收起（`home/index.vue:698-709`）；`refreshFavoriteIds` 走 `fetchAllPages` 顺序翻页、上限 100 页（`utils/pagination.js:8-19`）；axios 单请求 10s 超时（`request.js:71`）——最坏情况首屏被阻塞数十秒且无法逃离。
- 视图切换还有 300ms 假 loading（`home/index.vue:733-758`）。
- **全仓零取消机制**：`AbortController`/`CancelToken`/axios `signal` 零命中；`beforeRouteLeave` 零使用。唯一取消路径是上传取消 IPC。
- 附加 bug：第二个下载进度监听无条件覆盖 `downloadPercent`，批量下载进度显示错乱（`home/index.vue:1033-1039`）。

### 待办

1. **请求可取消基建**：`request.js` 支持传入 `signal`；建立「页面/视图级 AbortController 池」，切换视图/路由时统一 abort 未决请求（`forceChangeView`/`handleViewChange` 与路由守卫挂钩）。
2. **GlobalLoader 去阻断**：改为非全屏吞点（`pointer-events` 穿透）或加「跳过/进入」按钮；加载中允许侧栏 ActivityBar、顶栏、菜单点击切换视图。
3. **启动链改造**：三个启动请求并行化、各自独立超时与失败降级（树失败出空态+重试按钮，不再卡壳）；固定 400ms/300ms 假延时全部移除；收藏全量翻页改为后端提供「收藏 id 全量端点」或一次大 pageSize（≤100 页循环必须消失）。
4. 视图切换假 loading 改骨架屏或直接切换。
5. 下载进度多文件并行显示（修覆盖 bug），与任务三.4 取消按钮同批。
6. `MainEditor` 预览的裸 `fetch(url)`（`MainEditor.vue:573-591`）纳入取消管理。

### 验收

断网/弱网冷启动：3s 内可进入界面并自由切换页面；切换后无迟到响应写入旧视图状态（幽灵更新）；正常网络无视觉回退。

---

## 任务七 · 桌面端用户详情（查看其他用户部分数据）

**用户目标原文**：完成桌面端的用户详情，可查看其他用户的部分数据。

### 现状

- 榜单页已有维形：点击排行行 → 右侧面板展示详情，先渲染列表数据再异步 `getUser(userId)` 刷新（`rank/index.vue:282-307`）→ `GET /datum/user/{userId}`（匿名公开投影）。
- 后端公开投影现状：`datumUserAdminPayload`（`datum_user_admin.go:28-40`）返回 `userId/userName/nickName/avatar/count/status/createTime/updateTime/remark`——比 Java 原版（仅 `{userName, avatar}`，`PtmjDesktopUserMapper.java:16-17`）已宽。**注意 `remark`（备注）与 `status` 直出给匿名访客，隐私边界需定版**。
- 其余两处「他人信息」仅展示不可下钻：`FileInfoDrawer` 贡献者卡片（`FileInfoDrawer.vue:181-185`）、`MainEditor` 上传者（`MainEditor.vue:520-523`），同样调 `getUser`。

### 待办

1. **公开投影字段定版（后端）**：匿名 `GET /datum/user/{userId}` 收敛为「部分数据」——建议 `userId/userName/nickName/avatar/count(上传数)/createTime`；`remark/status/updateTime` 从匿名投影剔除（`remark` 若为运营备注绝不应公开）；被封用户返回可识别状态供前端显示「该用户不可见」。
2. **新端点：TA 的公开文件列表**（后端）：`GET /datum/desktop/user/{userId}/files`（匿名、分页、仅 `file_status=1` 已过审、复用 `datumFilePayload` 行结构）——用户详情看「TA 上传过什么」。
3. **桌面端统一用户详情 Drawer**：新组件复用三处入口（排行行、FileInfoDrawer 贡献者、MainEditor 上传者），内容 = 头像/昵称/上传数/加入时间 + 公开文件列表（可点击进入预览/下载）；替换 rank 页现有内嵌面板或由 Drawer 包装。
4. 上传计数与公开文件列表的一致性：count 为事务维护计数器，列表为实查——展示时以列表 total 为准或注明。

### 验收

三个入口打开同一 Drawer；被封/已删用户显示占位态；匿名访问不泄漏 remark/status/密保等任何敏感字段（抓包验证）。

---

## 任务八 · 本轮核查新发现的缺陷批次（部分为线上静默失败，建议优先修）

1. **下载记录字段名不一致（P0 级，直接影响我的下载功能）**：主进程返回 `{ success: true, savedPath }`（`src/main/index.js:621`），渲染层却读 `result.filePath`（`home/index.vue:340,965`、`download/index.vue:225-243`）→ `localPath` 恒记空串，重新下载的成功分支永不执行。统一字段名并回归。
2. **遗漏的 `code===200` 硬判定**（kadmin 信封 `code=0` 下静默失败，此前批次修了 5 处但漏了这些）：
   - `FileInfoDrawer.vue:171`（文件详情）
   - `ReportTimelinePanel.vue:143,167`（举报列表/时间线）
   - `UploadPanel.vue:239,256,286,331,348`（学校联想、重名校验、科目联想）→ 上传面板联想功能在 Go 后端下应该是坏的
   统一改为 `code===200 || code===0` 或改读 HTTP 成功语义。
3. **近似重复文件**：`views/home/components/ReporTimeLinePanel.vue`（拼写错误版，且 import 了 `addReport`）与在用的 `ReportTimelinePanel.vue` 并存，删除前者。
4. **死代码**：`electronAPI.saveFile`（`preload/index.js:29`、`src/main/index.js:378`）全仓零调用方，删除。
5. **`normalizeFileUrl` 弃用计划**：任务四两阶段完成后，客户端 host 改写逻辑标记弃用并观察一个版本（服务端已正确时客户端二次改写有改错风险）。
6. **`.env.production` 的 `127.0.0.1:9033`**（旧清单 #14）：上线前换真实域名，与任务四第二阶段一并处理。

---

## 任务九 · 旧清单未竟项并入（防失管，编号沿用 `pezmax-remaining-tasks.md`）

| 旧# | 任务 | 与本文关系 | 建议批次 |
| --- | --- | --- | --- |
| 10-14 | 数据 ETL（MySQL→PG、MinIO mirror、存量 URL 批量 UPDATE、冒烟切换、生产域名） | #12 存量 URL 更新**由任务四公版工具承接**（不再手写 SQL）；#14 并入任务八.6 | 上线前必办 |
| 15 | admin-web 审核专用页面（试卷/书签审核 + 批量过审 #4 + 审核联动封禁 #19 半条） | 独立实施；封禁「存量会话失效」半条并入任务五.4 | P2 |
| 16 | 内嵌 admin 入口下线（`VITE_AUTH_ENTRY_MODE`） | 独立小任务 | P2 |
| 18 | `ptmj_file_favorite` generated 管理端 CRUD | 独立（codegen 流程） | P2 |
| 19 | 封号机制补全 | 会话失效半条并入任务五.4；审核联动封禁随 #15 | P2 |
| 20 | 批量删除对齐（file/user 单删 vs Java 批量契约） | 随 #15 评估 | P2 |
| 21 | Redis 缓存管理面 | **并入任务五.3** | — |
| 22 | 管理端个人中心（改密/改资料） | 独立 | P2 |
| 23 | XSS 请求体过滤评估 | 独立评估 | P2 |
| 24 | 桌面端 `/common/upload` 遗留依赖清理（FileUpload/ImageUpload/Editor 组件 action 指向未实现端点） | 可并入任务八批次 | P2 |
| 25 | `POST /datum/file/upload` 独立直传 | 仅 #15/编辑器需要时实施 | P2 |
| 17 | 前端长期项 B1/B4/B7/D3 | 长期滚动 | P3 |

---

## 建议实施顺序（编号保持用户目标顺序，执行顺序按依赖与风险重排）

1. **任务五.1 排行异常**（先诊断后修，降级改造半天级）+ **任务八.1/八.2 静默失败 bug 批**——小、独立、直接止血。
2. **任务二 电子书桌面端**（后端零改动，纯前端 + 内容上架运营）。
3. **任务六 加载策略**（含请求取消基建，是任务三.4 与若干体验项的地基）。
4. **任务七 用户详情**（后端小端点 + 前端 Drawer）。
5. **任务四.一 地址映射公版**（管理页 + 批量修正工具）。
6. **任务一 CDN 决策 + 任务三 QoS**（同链路协同设计：边缘缓存/令牌桶/per-user 核算一次定案）。
7. **任务四.二 私有脏库修复 + 任务九 #10-14 ETL/冒烟/域名**（全部就绪后集中做上线切换）。
8. 任务九其余 P2 项按需排期。

## 完成勾销记录

- 2026-09-29 建档：按第二轮目标整理任务一~七；核查新增任务八（6 项缺陷）；旧清单未竟项并入任务九。
