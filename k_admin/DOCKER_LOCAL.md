# 本地 Docker 服务

本项目采用“桌面端运行 Go 服务，Docker 只运行项目依赖服务”的方式，方便和其他项目分离管理。

## 启动

```powershell
docker compose up -d postgres redis
go run .
```

Go 服务默认使用以下配置：

- 后台地址：`http://127.0.0.1:9033/admin`
- PostgreSQL 地址：`127.0.0.1`
- PostgreSQL 端口：`15432`
- 数据库：`kadmin`
- 用户名：`postgres`
- 密码：`kadmin_dev_pwd`
- Redis 地址：`127.0.0.1`
- Redis 端口：`16379`
- Redis 密码：`kadmin_redis_pwd`
- Redis DB：`0`

首次启动时，Docker 会把 `tests/data/admin_pg.sql` 导入到 `kadmin` 数据库中。该导入只会在数据库 volume 为空时执行。

## 可选 MinIO 对象存储

MinIO 默认不启动，也不会影响 PostgreSQL、Redis 和 Go 服务启动。需要本地对象存储时再开启 `storage` profile：

```powershell
docker compose --profile storage up -d minio minio-init
```

打开控制台 `http://127.0.0.1:19001`，默认账号：

- 用户名：`kadmin_minio`
- 密码：`kadmin_minio_pwd`
- 默认 Bucket：`kadmin`
- API 地址：`http://127.0.0.1:19000`

如果 Go 服务需要读取 MinIO 配置，在 `.env` 中开启：

```dotenv
KADMIN_FILE_LOCAL_ROOT=data/files
KADMIN_MINIO_ENABLED=true
KADMIN_MINIO_ENDPOINT=127.0.0.1:19000
KADMIN_MINIO_ACCESS_KEY=kadmin_minio
KADMIN_MINIO_SECRET_KEY=kadmin_minio_pwd
KADMIN_MINIO_BUCKET=kadmin
KADMIN_MINIO_USE_SSL=false
```

不开启时保持 `KADMIN_MINIO_ENABLED=false` 或不设置该变量即可，通用文件会写入 `KADMIN_FILE_LOCAL_ROOT`。启用 MinIO 后通用文件上传不会静默降级到本地目录，存储不可用会返回 503；旧头像接口仍保持原有迁移期行为。当前服务不会在启动时强制连接 MinIO。

## Navicat 连接 PostgreSQL

先确认 PostgreSQL 容器已启动：

```powershell
docker compose up -d postgres
docker compose ps
```

在 Navicat 中新建 PostgreSQL 连接，填写：

- 主机：`127.0.0.1`
- 端口：`15432`
- 初始数据库：`kadmin`
- 用户名：`postgres`
- 密码：`kadmin_dev_pwd`
- SSL：关闭或默认

这里的 `15432` 是宿主机端口，对应 `docker-compose.yml` 中的 `"${KADMIN_DB_PORT:-15432}:5432"`。如果本机 `15432` 被占用，可以在 `.env` 中改成其他端口，例如：

```dotenv
KADMIN_DB_PORT=25432
```

然后重启 PostgreSQL 容器，并在 Navicat 中使用新端口：

```powershell
docker compose up -d postgres
```

## 可选数据库管理界面

```powershell
docker compose --profile tools up -d adminer
```

打开 `http://127.0.0.1:18080`，然后使用：

- 系统：`PostgreSQL`
- 服务器：`postgres`
- 用户名：`postgres`
- 密码：`kadmin_dev_pwd`
- 数据库：`kadmin`

## 常用命令

```powershell
docker compose ps
docker compose logs -f postgres
docker compose logs -f redis
docker compose --profile storage logs -f minio
docker compose exec postgres psql -U postgres -d kadmin
docker compose exec redis redis-cli -a kadmin_redis_pwd ping
go build .
```

## 重置本地 Docker 数据

该命令会删除本项目的 Docker volume，包括 PostgreSQL、Redis，以及开启过 MinIO 时的 MinIO 数据。

```powershell
docker compose down -v
docker compose up -d postgres redis
```
