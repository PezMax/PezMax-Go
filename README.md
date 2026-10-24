# PezMax-Go · 拼图满绩

面向学习资料分享与整理的桌面应用，集中管理试卷、课件、资源书签和电子书。

本仓库承接 PezMax 原 Java 版本的 Go 迁移，后续开发在这里进行。项目包含 **Electron 桌面客户端、Go 后端、Vue Web 管理后台及 Docker Compose 部署配置**。当前业务后端使用 PostgreSQL、Redis 和 MinIO；日常运行不需要 Java 或 MySQL，MySQL 仅涉及旧数据迁移。

[项目仓库](https://github.com/PezMax/PezMax-Go) · [问题反馈](https://github.com/PezMax/PezMax-Go/issues) · [许可证](./LICENSE)

## 目录

- [功能概览](#功能概览)
- [架构与技术栈](#架构与技术栈)
- [项目结构](#项目结构)
- [环境要求](#环境要求)
- [本地开发](#本地开发)
- [配置说明](#配置说明)
- [Docker 部署](#docker-部署)
- [文件存储与预览](#文件存储与预览)
- [构建与发布](#构建与发布)
- [接口与二次开发](#接口与二次开发)
- [检查与测试](#检查与测试)
- [数据初始化与旧版迁移](#数据初始化与旧版迁移)
- [常见问题](#常见问题)
- [贡献与反馈](#贡献与反馈)
- [开源来源与许可](#开源来源与许可)

## 功能概览

### 桌面客户端

- **学习资料管理**：按学科、学校、类型、年份组织文件，提供树形浏览、搜索、详情和上传入口，支持多文件与文件夹上传。
- **内容预览**：预览 PDF、图片、Markdown 和文本；Word、PowerPoint 文档上传时通过 LibreOffice 转为 PDF。
- **资源书签**：分类整理外部链接，支持封面上传、收藏和举报。
- **电子书**：独立书目、学科分类与搜索，支持 PDF、EPUB 在线阅读，以及下载、收藏、举报；MOBI、AZW3 下载后使用本地阅读器打开。
- **找资料助手**：独立 Python + LangGraph 模块，用户自配模型 API Key，以自然语言检索真实资料、电子书和书签，支持连续追问并直接打开预览；界面使用 Ant Design Vue。详见[配置与架构说明](./PezMax-Desktop/docs/assistant.md)。
- **个人中心**：注册、登录、密保找回密码、头像及资料设置，查看个人上传、下载、收藏和举报记录。
- **贡献排行与通知**：展示资料贡献排行，接收公告、弹窗和滚动通知；通知已读状态保存在客户端本地。
- **桌面集成**：下载进度、断点续传、本地下载记录、快捷键、启动设置、主题及背景自定义，并集成更新检查。

### Web 管理后台

- 用户、部门、职位、角色、菜单及页面/按钮权限管理，按服务端菜单生成前端路由。
- 管理平台用户、资料、书签、电子书、举报和通知等业务数据。
- 资料与电子书审核工作台，支持待审处理、批量通过、按上传者通过和举报复审。
- 字典与系统配置、登录安全策略、操作日志、业务审计和登录审计。
- 定时任务及执行日志、系统监控、接口负载采样与排行。
- PostgreSQL 表驱动的代码生成器，支持生成内容预览、ZIP 下载及带冲突保护的项目写入。

以上以当前源码实现为准。历史迁移任务文档可能描述旧阶段状态，不能代替当前功能说明。

## 架构与技术栈

```text
Electron + Vue 桌面端 ── /datum/*、兼容通知路径 ──┐
                                                ├─ Go / Gin 后端
Vue Web 管理后台 ────── /api/* ──────────────────┘     ├─ PostgreSQL：业务、权限与审计数据
                                                      ├─ Redis：会话、验证码、令牌状态与缓存
可选 Nginx edge ─────── 反向代理、压缩、下载限流          └─ MinIO：资料、电子书、封面等对象
```

| 层级 | 技术与用途 |
| --- | --- |
| 桌面应用 | Electron 39、Vue 3、Element Plus、Ant Design Vue（助手）、Pinia、Vue Router、electron-vite |
| 本地智能体 | Python 3.11+、LangGraph StateGraph、HTTPX；通过 stdio 与 Electron 通信，安装版自带运行时 |
| 本地持久化 | sql.js / SQLite 保存下载记录和通知已读状态，JSON 保存客户端设置 |
| Web 后台 | Vben Admin、Vue 3、TypeScript、Ant Design Vue、Vite、pnpm workspace |
| 服务端 | Go、Gin、GoAdmin 公共组件 |
| 数据库 | PostgreSQL 16 |
| 会话与缓存 | Redis 7 |
| 对象存储 | MinIO |
| 文档转换 | LibreOffice Writer / Impress |
| 部署 | Docker Compose，可选 Nginx edge |

桌面用户和管理员使用独立的身份体系：

| 使用者 | 用户数据 | 登录方式 | 会话 |
| --- | --- | --- | --- |
| 桌面平台用户 | `ptmj_user` | `/datum/user/login` | Redis 保存的 opaque Bearer token，默认有效期 120 小时 |
| Web 管理员 | `goadmin_users` | `/api/auth/login` | JWT access / refresh token，Redis 管理令牌状态 |

两种账号及令牌不能互换。后端还保留 `/admin` 的 GoAdmin 兼容后台；新增管理功能主要在 Web 后台开发。

## 项目结构

```text
PezMax-Go/
├── README.md
├── LICENSE                         # 主项目 MIT 许可证
├── .env.example                    # 仓库级 Compose 配置模板
├── docker-compose.yml              # 推荐入口：依赖服务 + app / edge profiles
├── deploy/
│   ├── nginx.conf.template          # 边缘代理、压缩、下载限并发与分档限速
│   └── kadmin-proxy-headers.conf
├── PezMax-Agent/                    # 独立 Python + LangGraph 模块
│   ├── src/pezmax_agent/            # 图编排、模型适配、只读检索与 stdio 协议
│   ├── tests/                      # 图、模型、目录与进程协议测试
│   └── scripts/build.py            # 构建独立运行时，普通用户无需安装 Python
├── PezMax-Desktop/
│   ├── src/main/                    # Electron 主进程、IPC、下载、本地库与更新
│   ├── src/preload/                 # 主进程与渲染进程桥接
│   ├── src/renderer/                # Vue 桌面界面、接口、状态及路由
│   ├── docs/                       # VitePress 用户文档
│   ├── resources/、build/           # 图标和打包资源
│   ├── .env.development
│   ├── .env.production
│   └── electron-builder.yml
└── k_admin/
    ├── main.go                      # 应用入口，从当前工作目录加载 .env
    ├── go.mod                       # Go 模块位于此处，仓库根目录无 go.mod
    ├── .env.example                 # Go 后端配置模板（默认值与仓库级模板不同）
    ├── Dockerfile.app               # 应用镜像，内置 LibreOffice 与中文字体
    ├── admin-web/                   # Web 后台 workspace，主应用 apps/web-antd
    ├── internal/kadmin/
    │   ├── datum_*.go               # 桌面业务 HTTP 适配层
    │   ├── modules/                 # datum、文件、任务、监控、通知、代码生成等
    │   ├── platform/                # 存储、转换与文件 URL 基础能力
    │   ├── bootstrap/               # 权限和菜单种子
    │   ├── generated/               # 代码生成器维护的管理接口
    │   └── docs/                    # Swagger 生成文件
    ├── sql/                         # ptmj 表结构、用户及书签 ETL 脚本
    ├── tests/data/admin_pg.sql       # 管理系统 PostgreSQL 初始化种子
    ├── data/migrations/              # 历史迁移记录
    └── adapter/、engine/、modules/…  # 保留的 GoAdmin 公共代码
```

`k_admin/docker-compose.yml` 是 KAdmin 子项目的独立开发配置，使用另一组端口、库名和凭据。开发整个 PezMax-Go 时，统一使用**仓库根目录**的 Compose 文件及下文配置，避免混用。

## 环境要求

| 工具 | 要求 |
| --- | --- |
| Go | 1.22.10 或更高版本，与 `k_admin/go.mod` 对齐 |
| Node.js | 推荐 24.x；Web 后台声明支持 `^22.18.0 \|\| ^24.0.0` |
| npm | 用于桌面端，随 Node.js 安装 |
| Python | 开发/打包智能体时需要 3.11+；在桌面目录执行 `npm run agent:setup` 初始化，安装版无需另装 |
| pnpm | Web 后台要求 11+，项目指定 `pnpm@11.7.0`，推荐通过 Corepack 使用 |
| Docker | Docker Engine / Docker Desktop 与 Compose v2，命令为 `docker compose` |
| LibreOffice | 本机 Go 后端处理 DOC / DOCX / PPT / PPTX 时需要；应用镜像已内置 |

下文本地命令按 **Windows PowerShell** 编写。桌面端部分 npm 脚本使用 Windows 的 `set` 和 PowerShell 清理命令，macOS / Linux 请使用后文给出的直接构建方式。

## 本地开发

推荐在 Docker 中运行 PostgreSQL、Redis、MinIO，在宿主机分别运行 Go 后端、Web 后台和桌面端。

### 1. 获取项目并配置服务

```powershell
git clone https://github.com/PezMax/PezMax-Go.git
Set-Location PezMax-Go

# 已有配置时保留原文件，按模板补充缺失项即可
Copy-Item .env.example .env
Copy-Item k_admin/.env.example k_admin/.env
```

仓库根目录 `.env` 供 Compose 使用；`go run .` 从 `k_admin` 启动时读取 `k_admin/.env`。两份环境文件均不应提交。复制模板后，先填写数据库、Redis、MinIO 密码和 JWT 密钥；Compose 缺少这些值会拒绝启动。已使用过仓库中旧示例凭据的部署还需轮换密码和 JWT 密钥，因为旧值仍存在于 Git 历史中。

**仅复制模板还不够**：两者默认库名和端口不同。使用仓库级 Compose 默认值时，将 `k_admin/.env` 的对应项调整为（尖括号内容替换成根目录 `.env` 的相同值）：

```dotenv
KADMIN_DB_HOST=127.0.0.1
KADMIN_DB_PORT=25432
KADMIN_DB_USER=postgres
KADMIN_DB_PASSWORD=<与 PEZMAX_DB_PASSWORD 相同>
KADMIN_DB_NAME=pezmax

KADMIN_APP_PORT=9033
KADMIN_APP_HOST=127.0.0.1
KADMIN_APP_DEBUG=true
KADMIN_SWAGGER_ENABLED=false

KADMIN_REDIS_HOST=127.0.0.1
KADMIN_REDIS_PORT=26379
KADMIN_REDIS_PASSWORD=<与 PEZMAX_REDIS_PASSWORD 相同>
KADMIN_REDIS_DB=0

KADMIN_JWT_SECRET=<与根目录 KADMIN_JWT_SECRET 相同>
KADMIN_DATUM_SESSION_TTL=120h

KADMIN_MINIO_ENABLED=true
KADMIN_MINIO_ENDPOINT=http://127.0.0.1:29000
KADMIN_MINIO_INTERNAL_ENDPOINT=http://127.0.0.1:29000
KADMIN_MINIO_PUBLIC_BASE=http://127.0.0.1:29000
KADMIN_MINIO_ACCESS_KEY=<与 PEZMAX_MINIO_ACCESS_KEY 相同>
KADMIN_MINIO_SECRET_KEY=<与 PEZMAX_MINIO_SECRET_KEY 相同>
KADMIN_MINIO_BUCKET=ptmj
KADMIN_MINIO_USE_SSL=false
```

PostgreSQL、Redis、MinIO 和 Compose 内后端的宿主机端口默认只绑定 `127.0.0.1`；容器间仍通过内部网络互通。需要其他机器直连时，为对应服务设置 `PEZMAX_DB_BIND_IP`、`PEZMAX_REDIS_BIND_IP`、`PEZMAX_MINIO_BIND_IP` 或 `PEZMAX_APP_BIND_IP` 为可信私网 IP，并配合防火墙。`edge` 仍按 `PEZMAX_EDGE_PORT` 对外提供入口。若远程桌面端必须直连 MinIO 预览，还需显式配置可访问的 `PEZMAX_MINIO_PUBLIC_BASE` 与绑定地址。
宿主机直跑后端并用容器内的 edge 代理时，需把 `KADMIN_APP_HOST` 调整为可由容器访问的可信地址，而不能保持默认回环绑定。

在仓库根目录启动依赖：

```powershell
docker compose up -d postgres redis minio minio-init
docker compose ps -a
```

等待 PostgreSQL、Redis 健康检查通过，`minio-init` 正常退出。MinIO 控制台为 `http://127.0.0.1:29001`，登录凭据来自根目录 `.env` 的 `PEZMAX_MINIO_ACCESS_KEY` 和 `PEZMAX_MINIO_SECRET_KEY`。直链预览还需按[文件存储与预览](#文件存储与预览)设置读取策略。

### 2. 启动 Go 后端

在新的 PowerShell 窗口中，从仓库根目录执行：

```powershell
Set-Location k_admin
go mod download
go run .
```

服务默认监听 `http://127.0.0.1:9033`。访问 `/` 会跳转到 `/admin`，不会显示独立 Vue 后台。

如果本机安装了 LibreOffice，但 `soffice` 不在 PATH 中，在 `k_admin/.env` 指定完整路径，例如：

```dotenv
KADMIN_SOFFICE_BIN=C:/Program Files/LibreOffice/program/soffice.exe
```

缺少 LibreOffice 不会阻止服务启动，但需要转为 PDF 的 Office 文件上传会失败。

### 3. 启动 Web 管理后台

在新的 PowerShell 窗口中，从仓库根目录执行：

```powershell
Set-Location k_admin/admin-web
corepack enable
corepack pnpm install
```

当前仓库没有随源码提供 Web 后台应用的 `.env` 文件。在 `k_admin/admin-web/apps/web-antd/.env` 创建以下基础配置，供开发和构建共用：

```dotenv
VITE_APP_TITLE=PezMax 管理后台
VITE_APP_NAMESPACE=pezmax-admin
VITE_GLOB_API_URL=/api
VITE_BASE=/
VITE_PORT=5666
```

```powershell
# 工作目录仍为 k_admin/admin-web
corepack pnpm dev
```

按上述配置访问 `http://127.0.0.1:5666`。未设置 `VITE_PORT` 时，配置代码默认使用 **5173**；实际端口以终端输出为准。开发服务器将 `/api` 代理到 `http://127.0.0.1:9033`，后端端口发生变化时需同步修改 `apps/web-antd/vite.config.ts`。

首次初始化的管理员账号为 **`admin` / `admin`**，仅用于本地开发，使用后应修改密码。桌面用户需通过客户端注册或迁移旧用户数据。

### 4. 启动桌面客户端

在新的 PowerShell 窗口中，从仓库根目录执行：

```powershell
Set-Location PezMax-Desktop
npm ci
npm run dev
```

开发环境默认通过 `.env.development` 配置：

```dotenv
VITE_APP_BASE_API=/dev-api
VITE_APP_TARGET_URL=http://127.0.0.1:9033
```

`/dev-api` 是桌面渲染进程的开发代理前缀，代理转发时会去掉它；真实后端路由仍为 `/datum/*`。桌面渲染服务器配置端口为 **80**，若被其他服务占用或无法绑定，可修改 `electron.vite.config.mjs` 的 `renderer.server.port`。

注册一个桌面用户后，即可上传、收藏、下载及查看个人记录。新安装没有旧版学习资料，公开列表可能为空；资料和电子书上传后还需在管理端审核上架。

### 默认访问地址

| 服务 | 地址 / 端口 |
| --- | --- |
| Go 后端 / 兼容后台 | `http://127.0.0.1:9033/admin` |
| Web 管理后台 | `http://127.0.0.1:5666`，按上文显式配置 |
| 桌面开发渲染服务 | 80，Electron 开发窗口自动加载 |
| PostgreSQL | `127.0.0.1:25432` |
| Redis | `127.0.0.1:26379` |
| MinIO S3 API | `http://127.0.0.1:29000` |
| MinIO 控制台 | `http://127.0.0.1:29001` |
| 可选 Nginx edge | `http://127.0.0.1:8080` |
| 可选 Swagger | `http://127.0.0.1:9033/swagger/index.html`，默认关闭 |

## 配置说明

### 配置文件与作用域

| 文件 / 来源 | 读取者 | 说明 |
| --- | --- | --- |
| 根目录 `.env` | 仓库级 Docker Compose | 主要为 `PEZMAX_*` 与 edge 参数；app 服务显式映射成后端 `KADMIN_*` |
| `k_admin/.env` | 从 `k_admin` 启动的 Go 进程 | `godotenv.Load` 不覆盖已存在的进程环境变量 |
| `PezMax-Desktop/.env.development` | 桌面开发构建 | 代理前缀和后端目标地址 |
| `PezMax-Desktop/.env.production` | 桌面生产构建 | 渲染端真实 API 地址；修改后需重新构建 |
| `k_admin/admin-web/apps/web-antd/.env` 等 | Web 后台 Vite | API 根路径、标题、开发端口等，需自行创建 |
| 进程环境变量 | Go / Electron 等进程 | 用于覆盖后端配置，或提供 Electron 主进程更新源等设置 |

Compose 的 app 服务**不读取宿主机的 `k_admin/.env`**，而是使用根目录 Compose 的 `environment` 映射。高级参数若未列在该映射中，需要在 Compose override 中显式传入。

### 常用后端配置

| 变量 | 用途 / 默认值 |
| --- | --- |
| `KADMIN_DB_HOST/PORT/USER/PASSWORD/NAME` | PostgreSQL 连接，按实际部署填写 |
| `KADMIN_REDIS_HOST/PORT/PASSWORD/DB` | Redis 连接，默认 DB 为 0 |
| `KADMIN_APP_PORT` | HTTP 监听端口，默认 `9033` |
| `KADMIN_APP_DEBUG` | Gin 调试模式，本机默认 `true`，生产建议 `false` |
| `KADMIN_SWAGGER_ENABLED` | Swagger 开关，默认 `false` |
| `KADMIN_JWT_SECRET` | 管理端 JWT 签名密钥，部署时替换 |
| `KADMIN_JWT_ACCESS_TTL` | 管理端 access token 有效期，默认 `30m` |
| `KADMIN_JWT_REFRESH_TTL` | 管理端 refresh token 有效期，默认 `168h` |
| `KADMIN_DATUM_SESSION_TTL` | 桌面会话有效期，默认 `120h` |
| `KADMIN_DATUM_STATE_TTL` | 桌面文件树 / 排行缓存有效期，默认 `24h` |
| `KADMIN_DATUM_UPLOAD_MAX_MB` | 资料 / 电子书单文件上传上限，默认 `200` MiB；Office 转 PDF 仍限 `200` MiB |
| `KADMIN_MINIO_ENABLED` | 管理文件模块的 MinIO 开关；桌面 datum 上传直接依赖 MinIO |
| `KADMIN_MINIO_ENDPOINT/INTERNAL_ENDPOINT` | 后端访问 MinIO 的连接地址 |
| `KADMIN_MINIO_ACCESS_KEY/SECRET_KEY/BUCKET` | MinIO 凭据与桶名 |
| `KADMIN_MINIO_PUBLIC_BASE` | 客户端可访问的对象存储地址，写入数据库 URL 时使用 |
| `KADMIN_MINIO_USE_SSL` | 未带协议的 MinIO 地址是否使用 HTTPS |
| `KADMIN_FILE_LOCAL_ROOT` | 管理文件模块的本地存储根目录，默认 `data/files` |
| `KADMIN_SOFFICE_BIN` | LibreOffice 可执行文件路径，默认 `soffice` |
| `KADMIN_CONVERT_TIMEOUT` | 单次转档超时，默认 `2m` |
| `KADMIN_CONVERT_CONCURRENCY` | 转档最大并发，默认 `2` |

后端更多配置见 [k_admin/.env.example](./k_admin/.env.example)。端口、MinIO 凭据和 JWT 密钥应在所选运行方式中保持一致。

## Docker 部署

### 依赖服务与应用服务

以下命令均在**仓库根目录**执行。先从 `.env.example` 创建并调整 `.env`。

```powershell
# 默认仅启动 PostgreSQL、Redis、MinIO 和建桶任务
docker compose up -d

# 构建并启动 Go 应用与 Web 管理端；自动等待依赖服务就绪
docker compose --profile app up -d --build kadmin admin-web

# 查看后端状态与日志
docker compose --profile app ps -a
docker compose logs --tail 100 kadmin
```

应用镜像使用 `k_admin/Dockerfile.app`，包含 LibreOffice、中文字体和 Go 可执行文件。容器内使用 `postgres:5432`、`redis:6379`、`minio:9000`；宿主机连接则使用映射后的 25432、26379、29000，二者不要混用。

默认对外应用端口为 9033，可通过 `PEZMAX_APP_PORT` 修改。`PEZMAX_MINIO_PUBLIC_BASE` 必须设置为**客户端机器也能访问**的地址；跨机器部署时不能保留 `127.0.0.1`。

Compose 同时托管 Web 管理端，默认使用与 Go 后端相同的绑定地址和 `18088` 端口，`/api` 反代到 Go 服务。桌面安装包仍需单独分发。生产环境关闭 Go 调试模式时，在 Compose override 的 `services.kadmin.environment` 中加入 `KADMIN_APP_DEBUG: "false"`。`Luminous` 的自动验证和部署见 [CI/CD 说明](./deploy/cd/README.md)。

### 可选 Nginx edge

后端也在 Compose 中运行时：

```powershell
docker compose --profile app --profile edge up -d --build kadmin admin-web edge
```

默认代理 `kadmin:9033`，客户端可改为访问 `http://服务器地址:8080`。如果后端通过宿主机 `go run .` 运行，在根目录 `.env` 设置：

```dotenv
KADMIN_EDGE_UPSTREAM=host.docker.internal:9033
```

```powershell
docker compose --profile edge up -d edge
```

`edge` 不会自动启动后端；启动前保证所选上游已经可用。

| 根目录配置 | 默认值 | 含义 |
| --- | --- | --- |
| `PEZMAX_EDGE_PORT` | `8080` | edge 对外端口 |
| `KADMIN_EDGE_DL_RATE_FAST` | `5m` | 0–1 个正在写出响应的连接时，每下载连接速率 |
| `KADMIN_EDGE_DL_RATE_MID` | `2560k` | 2–3 个正在写出响应的连接时，每下载连接速率 |
| `KADMIN_EDGE_DL_RATE_LOW` | `640k` | 4 个及以上时，每下载连接速率 |
| `KADMIN_EDGE_DL_MAXCONN` | `16` | 下载总并发上限，另有每 IP 1 个下载连接限制 |

下载分档作用于 `/datum/download/`，单位按 Nginx 的字节/秒解释；edge 容器另以 `tc` TBF 共享出口预算限制所有响应流量，默认 `8mbit`。直接连接 9033 或对象存储不会经过 edge 的限流，详见 [出口带宽说明](./deploy/README.md)。

修改 edge 环境参数后重新执行 `docker compose --profile edge up -d edge`，使容器重建并应用模板。公网入口的 HTTPS 需由部署环境另行配置。

### 数据持久化

仓库级 Compose 使用以下命名卷：

| 卷名 | 内容 |
| --- | --- |
| `pezmax-go-postgres-data` | PostgreSQL 数据 |
| `pezmax-go-redis-data` | Redis AOF 数据 |
| `pezmax-go-minio-data` | MinIO 对象 |
| `pezmax-go-kadmin-uploads` | 容器后端 `uploads` 文件 |

停止服务可执行 `docker compose --profile app --profile edge down`，命名卷会保留。`down -v` 会删除卷中数据，不应用于日常重启。升级前备份 PostgreSQL、MinIO 对象、上传文件及部署配置；若启用其他本地存储目录，也需单独挂载和备份。

## 文件存储与预览

### MinIO 内部地址与公开地址

后端保存对象时使用内部连接地址，数据库记录的 URL 则按以下规则构造：

```text
KADMIN_MINIO_PUBLIC_BASE / KADMIN_MINIO_BUCKET / objectKey
```

例如容器内上传连接 `http://minio:9000`，面向远程客户端的地址可为 `https://files.example.com`。开发机上的 `127.0.0.1` 仅适用于客户端和存储在同一台机器的场景。

资料下载通过后端流式代理，并记录下载行为、支持 Range / ETag。封面、头像及部分文件预览使用对象直链，所以仅保证后端能连接 MinIO 还不够。

### 桶读取策略

根目录 `minio-init` **仅创建桶，不设置匿名读策略**。直链返回 `AccessDenied` 时，需要在 MinIO 控制台配置适合部署场景的读取策略。

本地开发若允许整个 `ptmj` 桶公开读取，可在仓库根目录执行：

```powershell
docker compose run --rm --no-deps --entrypoint /bin/sh minio-init -c 'mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" && mc anonymous set download "local/$MINIO_BUCKET"'
```

整桶公开读也会允许通过已知直链访问未审核对象。需要私有资料权限的部署，应按对象前缀限制公开范围，或为相关预览配置受控代理。

### 格式与大小

- 资料上传白名单包括 DOC、DOCX、PPT、PPTX、PDF、JPG/JPEG、PNG、WEBP、TXT、MD、EPUB、MOBI、AZW3。
- DOC / DOCX / PPT / PPTX 转为 PDF 后存储；TXT / MD 必须使用 UTF-8 编码。
- 电子书使用独立 `ptmj_ebook` 目录及 `/datum/ebook/*` 接口，上传支持 PDF / EPUB / MOBI / AZW3。
- 默认单文件上限 200 MiB，可通过 `KADMIN_DATUM_UPLOAD_MAX_MB` 调整；Office 转 PDF 的输入和输出仍各限 200 MiB。外层反向代理的请求大小和超时也需配合。
- 新上传的资料和电子书进入待审核状态，公开列表展示已审核 / 已上架内容。

客户端本地文件位于 Electron 的 `app.getPath('userData')`：`ptmj-downloads.db` 保存下载与通知已读记录，`ptmj-settings.json` 保存客户端设置；实际下载目录由客户端设置决定。

## 构建与发布

### Go 后端

```powershell
# 从仓库根目录进入 Go 模块
Set-Location k_admin
go build -trimpath -o k_admin.exe .
./k_admin.exe
```

Linux 可执行文件使用 `go build -trimpath -o k_admin .`。部署时保持配置与工作目录一致，并提供 PostgreSQL、Redis、MinIO；需要 Office 转档时同时安装 LibreOffice。容器部署优先使用仓库级 Compose 和 `Dockerfile.app`。

### Web 管理后台

```powershell
# 从仓库根目录执行
Set-Location k_admin/admin-web
corepack pnpm build
```

产物位于 `k_admin/admin-web/apps/web-antd/dist/`。用 Nginx 等静态服务器部署，并配置 SPA 路由回退与 `/api` 到 Go 后端的代理。上文 `VITE_GLOB_API_URL=/api` 适合前后端同域部署。

仓库级 Compose 使用 `k_admin/admin-web/Dockerfile` 构建生产镜像并配置 `/api` 代理。子项目原有的 `scripts/deploy/Dockerfile` 是上游模板，不用于本项目部署。

### 桌面应用

发布前修改 `PezMax-Desktop/.env.production`：

```dotenv
VITE_APP_BASE_API=https://api.example.com
VITE_APP_TARGET_URL=https://api.example.com
```

这里填写后端或 edge 的**站点根地址**，不要追加 `/api`，桌面接口自行拼接 `/datum/*` 等路径。当前生产模板仍指向 `127.0.0.1:9033`，仅适合本机服务。

Windows 构建：

```powershell
# 从仓库根目录执行
Set-Location PezMax-Desktop
npm ci
npm run agent:setup
npm run build:win
```

| 命令 | 用途 |
| --- | --- |
| `npm run dev` / `npm run dev:client` | 客户端开发入口 |
| `npm run dev:admin` | 保留的 admin 模式开发入口，Web 后台仍需独立启动 |
| `npm run build` | 编译主进程、预加载及渲染进程到 `out/` |
| `npm run agent:setup` / `agent:test` | 初始化 Python 虚拟环境与依赖 / 测试 LangGraph 模块 |
| `npm run agent:build` | 构建当前平台的独立智能体程序，并校验运行时 |
| `npm run build:unpack` | 生成未打包的应用目录 |
| `npm run build:win` | 构建 Windows 安装程序 |
| `npm run start` | 预览已构建的 Electron 应用 |
| `npm run docs:dev` / `npm run docs:build` | 开发 / 构建 VitePress 用户文档 |

安装产物位于 `PezMax-Desktop/dist/`。macOS / Linux 构建需处理现有脚本中的 Windows 命令；在对应平台使用直接命令，例如：

```bash
# 在 PezMax-Desktop 中执行，先安装依赖
VITE_AUTH_ENTRY_MODE=client npx electron-vite dev
npx electron-vite build
npm run agent:setup
npm run agent:build
npx electron-builder --mac    # macOS 平台
# Linux 平台改为 npx electron-builder --linux
```

打包配置声明了 Windows NSIS、macOS DMG 及 Linux AppImage / Snap / DEB，实际构建需对应工具链；macOS 当前 `notarize: false`，正式分发前应自行配置签名与公证。

更新源由客户端设置、Electron 主进程 `PTMJ_UPDATE_*` 环境变量及打包配置解析。当前 `electron-builder.yml` 和生产模板保留既有发布源；自行分发时需替换为自己的更新地址，并提供对应平台的更新元数据和安装产物（Windows 包括 `latest.yml`、安装包及 `.blockmap`）。仅修改 Vite 环境文件中的 `PTMJ_*` 不应视为已完成主进程运行时配置，更新流程需使用打包版本验证。

## 接口与二次开发

### 接口入口

| 接口范围 | 用途 |
| --- | --- |
| `/api/auth/*` | 管理员验证码、登录、刷新、退出和权限码 |
| `/api/*` | Web 管理后台业务、系统与生成的 CRUD 接口 |
| `/datum/user/*` | 桌面注册、登录、验证码和密保找回 |
| `/datum/desktop/*` | 桌面个人资料、统计、下载及收藏记录 |
| `/datum/file/*` | 资料列表、树、搜索、上传及内容管理 |
| `/datum/bookmark/*` | 书签及书签收藏 |
| `/datum/ebook/*` | 电子书列表、内容预览、上传、收藏和举报 |
| `/datum/download/file?fileId=…` | 资料下载，桌面会话鉴权 |
| `/datum/download/ebook?ebookId=…` | 电子书下载，桌面会话鉴权 |
| `/datum/report/*`、`/datum/notification/*` | 举报与通知业务 |
| `/system/notification/user/popup`、`/scroll` | 桌面通知的兼容路径 |

JSON 接口通常返回 `{code, data, message, msg}`；列表存在不同的分页形状，下载 / 预览接口返回文件流。客户端请求封装已处理部分分页兼容，新增接口时应遵循对应消费端的契约。公开读取接口与用户写操作的权限不同，请按具体路由的鉴权中间件接入。

### Swagger

Swagger 默认关闭，本机可通过进程环境变量开启：

```powershell
# 在 k_admin 中执行
$env:KADMIN_SWAGGER_ENABLED = 'true'
go run .
```

访问 `http://127.0.0.1:9033/swagger/index.html`。修改接口注释后，在 `k_admin` 中执行 `go generate .`，生成文件位于 `internal/kadmin/docs/`。生产环境按实际需要控制文档入口访问。

### 开发边界

- 新增后端业务优先放在 `k_admin/internal/kadmin/modules/<业务域>/`，公共基础能力放在 `platform/`。
- Web 页面位于 `k_admin/admin-web/apps/web-antd/src/`；菜单、权限和组件映射需与服务端种子同步。
- 桌面接口位于 `PezMax-Desktop/src/renderer/api/`，涉及本机文件、下载和持久化的能力通过主进程 / preload 桥接。
- `k_admin/internal/kadmin/generated/` 及对应前端生成目录由代码生成器维护，修改前确认覆盖与冲突规则。
- 代码生成落盘需要源码目录可写；仅包含可执行文件的应用镜像适合运行服务，开发时可在后台使用预览 / ZIP 下载，或在源码环境生成后重新构建。

## 检查与测试

以下为修改相应模块时的常用验证命令。

```powershell
# Go：在 k_admin 中执行
go test . ./internal/kadmin/...
go build .
```

```powershell
# Web 后台：在 k_admin/admin-web 中执行
corepack pnpm test:unit
corepack pnpm check:type
corepack pnpm build
```

```powershell
# 桌面端：在 PezMax-Desktop 中执行
npm run lint
npm run build
```

`k_admin` 同时保留了上游框架的跨数据库与浏览器测试。`go test ./...` 和 `make test` 涉及更多测试环境及初始化动作，不等同于仅运行 PezMax 应用测试。

联调时建议检查：桌面注册 / 登录 → 上传资料 → 管理端审核 → 公开列表可见 → 预览 / 下载 → 收藏及个人记录；电子书另检查 PDF / EPUB 阅读与下载，书签检查封面和外部链接。

## 数据初始化与旧版迁移

首次创建 PostgreSQL 数据卷时，根目录 Compose 按顺序导入：

1. `k_admin/tests/data/admin_pg.sql`：管理后台表结构与账号、权限等种子。
2. `k_admin/sql/ptmj_schema.sql`：桌面业务表结构。

后端启动还会执行各模块的幂等建表 / 补齐及权限菜单同步。初始化 SQL 仅在空数据目录执行；已有卷不会因重新启动 Compose 而重新导入。更改 `.env` 中的数据库密码也不会自动改变已有 PostgreSQL 用户密码。初始化种子含 `postgres` 角色归属，若更改 `PEZMAX_DB_USER`，需同步处理种子 SQL 的角色和权限。

当前已有以下旧版 MySQL → PostgreSQL 脚本：

| 脚本 | 范围 |
| --- | --- |
| [etl_users.sh](./k_admin/sql/etl_users.sh) | `ptmj_user`、`ptmj_security`，保留 BCrypt 密码哈希 |
| [etl_bookmarks.sh](./k_admin/sql/etl_bookmarks.sh) | 书签、书签收藏、书签举报 |

脚本需要 Bash、MySQL 8 的 `mysql` / `mysqldump` 和 Docker；连接参数见脚本头部。默认目标为 `pezmax-go-postgres` 容器中的 `pezmax` 数据库，导入使用 `ON CONFLICT DO NOTHING` 并重置相关 identity 序列。它们覆盖上述模块，**并非全部业务数据的一键迁移工具**。

迁移前备份源库与目标库，保留旧主键，核对逐表行数、关联关系和序列；MinIO 对象需单独迁移，旧 URL 需按新公开地址核对。以“旧账号登录、预览 / 下载旧资料、上传新资料”为切换验证流程。

## 常见问题

| 现象 | 排查方式 |
| --- | --- |
| Go 后端连接数据库 / Redis 失败 | 检查是否从 `k_admin` 启动、是否加载正确 `.env`；确认未混用 15432 / 16379 的子项目默认值与 25432 / 26379 的仓库级配置 |
| 修改 `.env` 后配置未生效 | Go 需重启；进程环境变量优先于 `.env`；Compose 环境变化需重新 `up -d`；前端构建参数变化需重新启动或构建 |
| 访问 9033 看不到 Vue 管理后台 | 9033 是 Go 后端；Web 后台需独立运行，按配置访问 5666 或终端显示的端口 |
| 管理后台登录请求 404 | 确认应用 `.env` 中 `VITE_GLOB_API_URL=/api`，以及开发 / 生产代理配置正确 |
| 桌面无法连接后端 | 检查 `.env.development` 的代理目标或 `.env.production` 的站点根地址；远程部署不能使用客户端的 `127.0.0.1` |
| 管理员账号无法登录桌面端 | 桌面用户与管理用户属于独立账号体系，桌面端注册用户或导入旧 `ptmj_user` |
| Office 上传失败 | 查看启动日志中的 LibreOffice 探测结果，确认 `KADMIN_SOFFICE_BIN`、超时与文件本身有效 |
| 上传成功但封面 / 预览打不开 | 检查 MinIO 的公开地址、桶策略与对象 URL；`minio-init` 不会自动配置匿名读取 |
| 资料列表 / 电子书列表为空 | 新部署没有旧业务内容；新上传内容需审核 / 上架后才公开展示 |
| 文件树 / 排行 / 验证码异常 | 检查 Redis 健康、密码及连接；Redis 是会话与这些缓存接口的运行依赖 |
| edge 返回 502 | 确认后端已启动；容器后端使用 `kadmin:9033`，宿主机后端使用 `host.docker.internal:9033` |
| 下载未受限速影响 | 确认请求经过 edge 的 `/datum/download/`；直连后端或 MinIO 会绕过该位置的限制 |
| Swagger 返回 404 | 默认关闭，显式设置 `KADMIN_SWAGGER_ENABLED=true` 后重启服务 |
| macOS / Linux 执行 npm 脚本失败 | 现有部分脚本使用 Windows shell；使用上文直接的 electron-vite / electron-builder 命令 |

## 贡献与反馈

欢迎通过 [Issues](https://github.com/PezMax/PezMax-Go/issues) 反馈问题或提出建议，通过 Pull Request 提交改进。反馈时附上涉及的端、操作步骤、期望 / 实际行为、系统和工具版本，以及已脱敏的日志。

提交改动前运行相关模块的检查，接口或配置变更同步更新文档；涉及数据库结构时核对初始化 SQL 与启动补齐逻辑，涉及账号、审核及下载时验证跨端业务流程。

QQ 交流群：`1077605719`。

## 开源来源与许可

主项目采用 [MIT License](./LICENSE)，版权归属见许可证文件。仓库包含上游代码，各部分应遵守自身许可证和归属说明：

- Go 后端复用并修改 [GoAdmin](https://github.com/GoAdminGroup/go-admin)，相关代码遵循 [Apache License 2.0](./k_admin/LICENSE)。
- Web 管理后台基于 [Vben Admin](https://github.com/vbenjs/vue-vben-admin)，相关代码遵循 [MIT License](./k_admin/admin-web/LICENSE)。
- 桌面端保留部分 RuoYi 风格的接口适配和前端公共代码；分发时应保留所含代码及依赖的版权、许可证声明。

PezMax-Go 是独立维护的二次开发项目，不是 GoAdmin 或 Vben 团队的官方发行版。请保留仓库中的上游许可证、版权及归属说明。
