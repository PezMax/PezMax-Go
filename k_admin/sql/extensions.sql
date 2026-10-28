-- ============================================================================
-- PostgreSQL 扩展初始化（幂等）
--
-- 由根目录 docker-compose.yml 与 k_admin/docker-compose.yml 挂载到
-- /docker-entrypoint-initdb.d/，仅在数据卷为空、首次 initdb 后执行。
-- 已有数据卷不会重复执行；升级已存在的库请手动执行本文件或运行
-- deploy/cd/enable-pgvector.sh（生产环境）。
--
-- vector  ：pgvector，RAG 题目/试卷 embedding 的向量类型与 HNSW 索引
-- pg_trgm ：内容词法兜底召回（二元词切分后的相似匹配）
-- 详见 docs/task.md 的 P0-6 / P0-7。
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- 初始化时立即失败（而不是等到建表）以便启动日志直接暴露缺扩展的镜像。
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector') THEN
        RAISE EXCEPTION 'pgvector extension is unavailable; use the pgvector/pgvector:pg16 image';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm') THEN
        RAISE EXCEPTION 'pg_trgm extension is unavailable';
    END IF;
END
$$;
