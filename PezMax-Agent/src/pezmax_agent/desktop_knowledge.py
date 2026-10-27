"""Trusted desktop adapter; local paths/owner are supplied by Electron, never a model.

HTTP and MCP do not expose this adapter. They retain their own authenticated
contracts and reuse the same store, retrieval service and answer graph.
"""
import asyncio
import re
from pathlib import Path
from typing import Literal

import httpx
from pydantic import ValidationError, model_validator

from .contracts import CatalogConfig, ProviderConfig
from .document_import import MAX_FILE_BYTES, extract_document
from .embeddings import Embeddings
from .errors import AgentError
from .knowledge import KnowledgeService, run_rag
from .knowledge_schema import Identifier, SearchInput
from .knowledge_store import KnowledgeStore
from .provider import ChatCompletionsModel
from .platform_knowledge import PlatformKnowledge
from .reading_schema import StrictOutput


class DesktopKnowledgeInput(StrictOutput):
    action: Literal['list', 'import', 'delete', 'search', 'ask', 'read', 'sync', 'open']
    library: Literal['local', 'platform'] = 'local'
    mode: Literal['keyword', 'hybrid'] = 'keyword'
    query: SearchInput | None = None
    documentId: Identifier | None = None
    chunkId: Identifier | None = None

    @model_validator(mode='after')
    def required_fields(self):
        if self.action in ('search', 'ask') and (self.query is None or self.query.mode != self.mode):
            raise ValueError('Query required')
        if self.action in ('sync', 'open') and self.library != 'platform':
            raise ValueError('Platform required')
        if self.library == 'platform' and self.action in ('import', 'delete'):
            raise ValueError('Platform sources are managed by the catalog')
        if self.action in ('delete', 'read', 'open') and self.documentId is None:
            raise ValueError('Document required')
        if self.action in ('read', 'open') and self.chunkId is None:
            raise ValueError('Chunk required')
        return self


def read_selected_file(path):
    try:
        source = Path(path)
        formats = {'.pdf': 'pdf', '.epub': 'epub', '.txt': 'text', '.md': 'text', '.markdown': 'text'}
        if not source.is_absolute() or source.suffix.lower() not in formats or not source.is_file():
            raise ValueError('Invalid source')
        with source.open('rb') as stream:
            data = stream.read(MAX_FILE_BYTES + 1)
        return extract_document(data, source.stem[:200], formats[source.suffix.lower()])
    except AgentError:
        raise
    except Exception:
        raise AgentError('无法读取所选文档，请重新选择本机文件', 'invalid_document') from None


async def run_desktop_knowledge(payload, on_progress):
    try:
        request = DesktopKnowledgeInput.model_validate(payload.get('knowledge'))
        storage = payload.get('storage')
        if (not isinstance(storage, dict) or not isinstance(storage.get('owner'), str)
                or not re.fullmatch(r'[a-f0-9]{64}', storage['owner'])
                or not isinstance(storage.get('path'), str) or not Path(storage['path']).is_absolute()):
            raise ValueError('Invalid desktop storage')
    except (ValueError, TypeError, ValidationError):
        raise AgentError('请求的知识库参数无效', 'invalid_knowledge') from None

    platform_config = CatalogConfig.parse(payload.get('catalog')) if request.library == 'platform' else None
    store = await asyncio.to_thread(KnowledgeStore, storage['path'] + '.platform' if platform_config else storage['path'])
    owner = storage['owner']
    use_embedding = request.mode == 'hybrid' and request.action in ('import', 'sync', 'search', 'ask')
    async with httpx.AsyncClient(follow_redirects=False, trust_env=False) as client:
        embedding = Embeddings(client, ProviderConfig.parse(payload.get('embedding'))) if use_embedding else None
        service = KnowledgeService(store, owner, embedding)
        platform = PlatformKnowledge(client, platform_config, store, owner) if platform_config else None
        if platform:
            on_progress({'status': '正在核对平台文件状态…'})
            sources, coverage = await platform.reconcile()
        if request.action == 'sync':
            result = await platform.sync(sources, coverage, embedding, on_progress)
        elif request.action == 'list':
            result = {'documents': await asyncio.to_thread(store.list_documents, owner)}
        elif request.action == 'import':
            on_progress({'status': '正在本机提取文档文字…'})
            document = await asyncio.to_thread(read_selected_file, payload.get('localDocument'))
            on_progress({'status': '正在建立向量索引…' if embedding else '正在建立本地关键词索引…'})
            result = {'document': await service.ingest(document)}
        elif request.action == 'delete':
            result = {'deleted': await asyncio.to_thread(store.delete, owner, request.documentId)}
        elif request.action in ('read', 'open'):
            result = {'chunk': await platform.live_chunk(request.documentId, request.chunkId) if platform else
                      await asyncio.to_thread(store.get_chunk, owner, request.documentId, request.chunkId)}
        elif request.action == 'search':
            on_progress({'status': '正在检索文档片段…'})
            result = {'retrieval': await service.search(request.query)}
        else:
            on_progress({'status': '正在检索、生成回答并核对引用…'})
            model = ChatCompletionsModel(client, ProviderConfig.parse(payload.get('provider')))
            result = await run_rag(model, service, request.query)
        if platform:
            # Revalidate after generation too, before returning any evidence.
            if request.action in ('search', 'ask'):
                _, coverage = await platform.reconcile()
                result = await platform.enrich(result)
            result.setdefault('coverage', coverage)
    return {'content': '', 'messages': [], 'resources': [], 'reports': [], 'knowledge': result}
