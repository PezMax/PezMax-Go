"""BYOK embedding adapter with bounded batches and strict response validation."""
import hashlib

import httpx

from .errors import AgentError
from .http import read_json
from .knowledge_store import unit_vector


def embedding_status(status):
    messages = {400: '向量请求被拒绝，请确认模型支持 Embeddings 接口与输入格式',
                401: 'Embedding API Key 无效', 403: 'Embedding Key 没有模型访问权限',
                404: 'Embedding 模型或接口不存在', 429: 'Embedding 服务额度不足或请求过于频繁'}
    return AgentError(messages.get(status, 'Embedding 服务暂不可用，请稍后重试'), 'embedding_unavailable')


class Embeddings:
    def __init__(self, client, config):
        self.client, self.config = client, config
        self.profile = hashlib.sha256(f"{config.base_url}\n{config.model}".encode()).hexdigest()

    async def embed(self, texts):
        if not texts or len(texts) > 3000 or any(not isinstance(t, str) or not 1 <= len(t) <= 2000 for t in texts):
            raise AgentError("待向量化的片段超出限制", "embedding_contract")
        vectors = []
        for start in range(0, len(texts), 16):
            batch = texts[start:start + 16]
            try:
                data = await read_json(self.client, "POST", f"{self.config.base_url}/embeddings",
                    headers={"Authorization": f"Bearer {self.config.api_key}"},
                    json={"model": self.config.model, "input": batch, "encoding_format": "float"},
                    timeout=60, limit=4 * 1024 * 1024, status_error=embedding_status)
            except httpx.HTTPError:
                raise AgentError("无法完成向量请求，请检查 embedding 设置", "embedding_unavailable") from None
            rows = data.get("data")
            if (not isinstance(rows, list) or len(rows) != len(batch)
                    or any(not isinstance(r, dict) or type(r.get("index")) is not int for r in rows)
                    or {r["index"] for r in rows} != set(range(len(batch)))):
                raise AgentError("向量服务返回的索引或数量不正确", "embedding_contract")
            vectors.extend(unit_vector(r.get("embedding")) for r in sorted(rows, key=lambda r: r["index"]))
        if len({len(v) for v in vectors}) != 1:
            raise AgentError("向量服务返回的维度不一致", "embedding_contract")
        return vectors
