"""Small local corpus: transactional SQLite, CJK BM25, exact cosine and RRF.

No remote vector database and no filesystem paths accepted from tool callers.
The trusted adapter supplies owner; every query includes it before scoring.
"""
import hashlib
import json
import math
import re
import sqlite3
from collections import Counter
from contextlib import contextmanager
from pathlib import Path

from .contracts import json_text
from .errors import AgentError
from .knowledge_schema import DocumentInput
from .reading import normalized


def tokens(text):
    result = re.findall(r"[a-z0-9_]+|[\u4e00-\u9fff]", text.lower())
    for run in re.findall(r"[\u4e00-\u9fff]{2,}", text):
        result.extend(run[i:i + 2] for i in range(len(run) - 1))
    return result


def prepare_document(value):
    doc = value if isinstance(value, DocumentInput) else DocumentInput.model_validate(value)
    sections = [{"location": s.location, "text": normalized(s.text)}
                for s in sorted(doc.sections, key=lambda s: s.location)]
    digest = hashlib.sha256(json_text({"format": doc.format, "sections": sections}).encode()).hexdigest()
    chunks = []
    for section in sections:
        text = section["text"]
        for start in range(0, len(text), 1000):
            excerpt = text[start:start + 1200]
            chunk_id = hashlib.sha256(f'{digest}:{section["location"]}:{start}'.encode()).hexdigest()[:32]
            chunks.append({"chunkId": chunk_id, "location": section["location"], "text": excerpt})
            if start + 1200 >= len(text):
                break
    if not chunks:
        raise AgentError("文档没有可索引文字；扫描件暂不支持 OCR", "empty_document")
    if len(chunks) > 3000:
        raise AgentError("文档分块过多，请拆分后导入", "capacity")
    return {"documentId": digest[:32], "digest": digest, "title": doc.title.strip(), "format": doc.format,
            "characters": sum(len(s["text"]) for s in sections), "chunks": chunks}


def unit_vector(value):
    if not isinstance(value, list) or not 1 <= len(value) <= 4096:
        raise AgentError("向量维度无效", "embedding_contract")
    if any(type(x) not in (int, float) or not math.isfinite(x) for x in value):
        raise AgentError("向量数值无效", "embedding_contract")
    norm = math.hypot(*value)
    if not math.isfinite(norm) or norm <= 0:
        raise AgentError("向量不能为零", "embedding_contract")
    return [x / norm for x in value]


class KnowledgeStore:
    def __init__(self, path):
        self.path = Path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        with self.connect() as db:
            db.executescript("""
                CREATE TABLE IF NOT EXISTS documents (
                    owner TEXT NOT NULL, id TEXT NOT NULL, digest TEXT NOT NULL,
                    title TEXT NOT NULL, format TEXT NOT NULL, characters INTEGER NOT NULL,
                    profile TEXT NOT NULL, dimension INTEGER NOT NULL,
                    created TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
                    PRIMARY KEY(owner,id));
                CREATE TABLE IF NOT EXISTS chunks (
                    owner TEXT NOT NULL, document_id TEXT NOT NULL, id TEXT NOT NULL,
                    location INTEGER NOT NULL, text TEXT NOT NULL, vector TEXT,
                    PRIMARY KEY(owner,document_id,id),
                    FOREIGN KEY(owner,document_id) REFERENCES documents(owner,id) ON DELETE CASCADE);
                CREATE INDEX IF NOT EXISTS chunks_owner ON chunks(owner,document_id);
            """)

    @contextmanager
    def connect(self):
        db = sqlite3.connect(self.path, timeout=10)
        db.row_factory = sqlite3.Row
        db.execute("PRAGMA foreign_keys=ON")
        try:
            with db:
                yield db
        finally:
            db.close()

    def list_documents(self, owner):
        with self.connect() as db:
            rows = db.execute("""SELECT d.id AS documentId,d.title,d.format,d.characters,d.profile,
                d.dimension,d.created,COUNT(c.id) AS chunks FROM documents d
                LEFT JOIN chunks c ON c.owner=d.owner AND c.document_id=d.id
                WHERE d.owner=? GROUP BY d.owner,d.id ORDER BY d.created,d.id""", (owner,)).fetchall()
        return [dict(row) for row in rows]

    def existing(self, owner, document_id):
        return next((d for d in self.list_documents(owner) if d["documentId"] == document_id), None)

    def put(self, owner, document, *, vectors=None, profile=""):
        chunks = document["chunks"]
        if vectors is not None:
            if not profile or len(vectors) != len(chunks):
                raise AgentError("分块与向量数量不一致", "embedding_contract")
            vectors = [unit_vector(v) for v in vectors]
            if len({len(v) for v in vectors}) != 1:
                raise AgentError("向量维度不一致", "embedding_contract")
        elif profile:
            raise AgentError("未提供向量", "embedding_contract")
        dimension = len(vectors[0]) if vectors else 0
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            current = db.execute("SELECT profile,dimension FROM documents WHERE owner=? AND id=?",
                                 (owner, document["documentId"])).fetchone()
            if current:
                if current["profile"] != profile or current["dimension"] != dimension:
                    raise AgentError("同一文档已使用其他索引配置；请显式删除后重新导入", "index_conflict")
                return {**self.existing(owner, document["documentId"]), "inserted": False}
            count, chars = db.execute("SELECT COUNT(*),COALESCE(SUM(characters),0) FROM documents WHERE owner=?", (owner,)).fetchone()
            chunk_count = db.execute("SELECT COUNT(*) FROM chunks WHERE owner=?", (owner,)).fetchone()[0]
            if count >= 200 or chars + document["characters"] > 10_000_000 or chunk_count + len(chunks) > 10000:
                raise AgentError("本地知识库容量已达上限，请删除不再需要的文档", "capacity")
            db.execute("INSERT INTO documents(owner,id,digest,title,format,characters,profile,dimension) VALUES(?,?,?,?,?,?,?,?)",
                       (owner, document["documentId"], document["digest"], document["title"], document["format"],
                        document["characters"], profile, dimension))
            db.executemany("INSERT INTO chunks(owner,document_id,id,location,text,vector) VALUES(?,?,?,?,?,?)", [
                (owner, document["documentId"], c["chunkId"], c["location"], c["text"], json_text(vectors[i]) if vectors else None)
                for i, c in enumerate(chunks)])
        return {**self.existing(owner, document["documentId"]), "inserted": True}

    def delete(self, owner, document_id):
        with self.connect() as db:
            return db.execute("DELETE FROM documents WHERE owner=? AND id=?", (owner, document_id)).rowcount > 0

    def chunks(self, owner, document_ids=(), *, include_vectors=True):
        # Permission/selection filtering occurs before either ranking branch.
        where, args = "c.owner=?", [owner]
        if document_ids:
            where += " AND c.document_id IN (" + ",".join("?" for _ in document_ids) + ")"
            args.extend(document_ids)
        # Keyword searches need coverage metadata, not potentially large vector JSON.
        vector_column = 'c.vector' if include_vectors else 'CASE WHEN c.vector IS NOT NULL THEN 1 END AS vector'
        with self.connect() as db:
            rows = db.execute(f"""SELECT c.document_id AS documentId,c.id AS chunkId,c.location,c.text,{vector_column},
                d.title,d.format,d.profile,d.dimension FROM chunks c JOIN documents d
                ON d.owner=c.owner AND d.id=c.document_id WHERE {where} ORDER BY c.document_id,c.location,c.id""", args).fetchall()
        return [dict(row) for row in rows]

    def get_chunk(self, owner, document_id, chunk_id):
        # Direct parameterized lookup; does not load the entire corpus.
        with self.connect() as db:
            row = db.execute("""SELECT c.document_id AS documentId,c.id AS chunkId,c.location,c.text,d.title,d.format
                FROM chunks c JOIN documents d ON d.owner=c.owner AND d.id=c.document_id
                WHERE c.owner=? AND c.document_id=? AND c.id=?""", (owner, document_id, chunk_id)).fetchone()
        if not row:
            raise AgentError("文档片段不存在或不可访问", "not_found")
        return dict(row)


def retrieve(store, owner, query, *, vector=None, profile=""):
    rows = store.chunks(owner, query.documentIds, include_vectors=query.mode == 'hybrid')
    query_terms = sorted(set(tokens(query.question)))
    counters = [Counter(tokens(row["text"])) for row in rows]
    lengths = [sum(count.values()) for count in counters]
    average = sum(lengths) / len(rows) if rows else 1
    dfs = {term: sum(term in count for count in counters) for term in query_terms}
    scored = []
    for i, count in enumerate(counters):
        score = 0
        for term in query_terms:
            tf = count[term]
            if tf:
                idf = math.log(1 + (len(rows) - dfs[term] + .5) / (dfs[term] + .5))
                score += idf * tf * 2.2 / (tf + 1.2 * (.25 + .75 * lengths[i] / max(average, 1)))
        if score > 0:
            scored.append((i, score))
    lexical = sorted(scored, key=lambda x: (-x[1], x[0]))[:30]
    semantic = []
    if query.mode == "hybrid":
        if vector is None or not profile:
            raise AgentError("混合检索需要本次请求的 embedding 设置和 Key", "embedding_required")
        vector = unit_vector(vector)
        indexed = [(i, row) for i, row in enumerate(rows) if row["vector"] is not None]
        if any(row["profile"] != profile or row["dimension"] != len(vector) for _, row in indexed):
            raise AgentError("向量模型或维度与索引不一致，请筛选同配置文档或重新导入", "embedding_mismatch")
        if rows and not indexed:
            raise AgentError("所选文档没有向量索引，请使用关键词模式或重新导入", "embedding_required")
        semantic = sorted(((i, sum(a * b for a, b in zip(vector, json.loads(row["vector"]))))
                           for i, row in indexed), key=lambda x: (-x[1], x[0]))[:30]
    fused = {}
    for ranking in (lexical, semantic):
        for rank, (index, _) in enumerate(ranking, 1):
            fused[index] = fused.get(index, 0) + 1 / (60 + rank)
    indexes = sorted(fused, key=lambda i: (-fused[i], i))[:query.limit]
    public = [{key: row[key] for key in ("documentId", "chunkId", "location", "text", "title", "format")}
              for row in (rows[i] for i in indexes)]
    return {"chunks": public, "mode": query.mode, "scannedChunks": len(rows),
            "vectorChunks": sum(row["vector"] is not None for row in rows),
            "partial": len(public) < len(rows), "fusion": "rrf-k60" if query.mode == "hybrid" else "bm25"}
