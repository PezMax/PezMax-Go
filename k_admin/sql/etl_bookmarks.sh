#!/usr/bin/env bash
# ============================================================================
# 书签模块数据 ETL：本地 MySQL(pezmax) -> pezmax-go PostgreSQL(pezmax)
# 范围：ptmj_bookmark / ptmj_bookmark_favorite / ptmj_bookmark_report
#
# 用法：
#   MYSQL_PWD='数据库密码' ./etl_bookmarks.sh
#   可选环境变量：MYSQL_HOST MYSQL_PORT MYSQL_USER MYSQL_DB PG_CONTAINER PG_DB
#
# 幂等性：INSERT ... ON CONFLICT DO NOTHING，重复执行不报错、不产生重复行；
#         每次执行后按 max(id)+1 重置 identity 序列（下限为原库 AUTO_INCREMENT）。
# 依赖：本机 mysqldump/mysql（MySQL Server 8.0）、docker（执行 psql 导入）。
# ============================================================================
set -euo pipefail

MYSQL_HOST="${MYSQL_HOST:-127.0.0.1}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_DB="${MYSQL_DB:-pezmax}"
PG_CONTAINER="${PG_CONTAINER:-pezmax-go-postgres}"
PG_DB="${PG_DB:-pezmax}"

: "${MYSQL_PWD:?请先设置 MYSQL_PWD 环境变量}"

dump=$(mktemp)
trap 'rm -f "$dump"' EXIT

mysqldump -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" \
  --no-create-info --skip-triggers --skip-add-locks --skip-lock-tables \
  --complete-insert --default-character-set=utf8mb4 \
  "$MYSQL_DB" ptmj_bookmark ptmj_bookmark_favorite ptmj_bookmark_report > "$dump"

if ! grep -q "^INSERT INTO" "$dump"; then
  echo "错误：dump 中没有 INSERT 语句，请检查 MySQL 连接与源库" >&2
  exit 1
fi

{
  echo "SET client_encoding = 'UTF8';"
  # mysqldump 在字符串里使用 \' 与 \n 等 MySQL 风格转义，关闭 PG 的标准字符串
  # 语义后按同样规则解释，避免反斜杠被当作字面字符
  echo "SET standard_conforming_strings = off;"
  echo "BEGIN;"
  # 仅保留 INSERT 行；反引号标识符改双引号；语句尾加 ON CONFLICT DO NOTHING
  grep "^INSERT INTO" "$dump" | sed 's/`/"/g; s/);$/) ON CONFLICT DO NOTHING;/'
  echo "SELECT setval(pg_get_serial_sequence('ptmj_bookmark','id'),"
  echo "  GREATEST((SELECT COALESCE(MAX(id),0) FROM ptmj_bookmark)+1, 22), false);"
  echo "SELECT setval(pg_get_serial_sequence('ptmj_bookmark_report','report_id'),"
  echo "  GREATEST((SELECT COALESCE(MAX(report_id),0) FROM ptmj_bookmark_report)+1, 20), false);"
  echo "COMMIT;"
} | docker exec -i "$PG_CONTAINER" psql -v ON_ERROR_STOP=1 -U postgres -d "$PG_DB"

echo "=== 导入后行数核对（应与 MySQL 源一致：17 / 4 / 2） ==="
docker exec "$PG_CONTAINER" psql -U postgres -d "$PG_DB" -c \
  "SELECT 'ptmj_bookmark' AS 表, count(*) AS 行数 FROM ptmj_bookmark
   UNION ALL SELECT 'ptmj_bookmark_favorite', count(*) FROM ptmj_bookmark_favorite
   UNION ALL SELECT 'ptmj_bookmark_report', count(*) FROM ptmj_bookmark_report;"
