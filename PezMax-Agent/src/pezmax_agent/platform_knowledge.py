"""Business catalog -> bounded desktop cache; all URLs are fixed API routes.

The platform remains authoritative. No stored file URL is followed, and catalog
credentials are never given to a generation/embedding provider or graph state.
"""
import asyncio
import hashlib

import httpx
from pydantic import BaseModel, ConfigDict, Field

from .catalog import catalog_status
from .document_import import MAX_FILE_BYTES, extract_document
from .errors import AgentError
from .http import read_json
from .knowledge_store import prepare_document


class PlatformSource(BaseModel):
    model_config = ConfigDict(extra='forbid', strict=True)
    kind: str = Field(pattern=r'^(file|ebook)$')
    id: str = Field(pattern=r'^[1-9][0-9]{0,18}$')
    title: str = Field(min_length=1, max_length=255)
    format: str = Field(pattern=r'^(pdf|epub|txt|md)$')
    subject: str = Field(max_length=160)
    school: str = Field(max_length=160)
    year: int = Field(ge=0, le=9999)
    revision: str = Field(pattern=r'^[a-f0-9]{64}$')


def source_id(source):
    return hashlib.sha256(f"{source['kind']}:{source['id']}".encode()).hexdigest()[:32]


class PlatformKnowledge:
    def __init__(self, client, config, store, owner):
        self.client, self.config, self.store, self.owner = client, config, store, owner

    async def json(self, path, **kwargs):
        body = await read_json(self.client, 'GET', f'{self.config.base_url}/datum/knowledge/{path}',
            headers={'Authorization': f'Bearer {self.config.token}'}, timeout=15,
            limit=512 * 1024, status_error=catalog_status, **kwargs)
        if body.get('code') not in (0, 200) or not isinstance(body.get('data'), dict):
            raise AgentError('平台知识目录响应无效', 'platform_contract')
        return body['data']

    async def manifest(self):
        body = await self.json('manifest')
        try:
            sources = [PlatformSource.model_validate(s).model_dump() for s in body['sources']]
            total, partial = body['total'], body['partial']
            if (len(sources) > 200 or type(total) is not int or total < len(sources)
                    or type(partial) is not bool or partial != (total > len(sources))
                    or len({source_id(s) for s in sources}) != len(sources)):
                raise ValueError()
        except (ValueError, TypeError, KeyError):
            raise AgentError('平台知识目录响应无效', 'platform_contract') from None
        return sources, {'available': len(sources), 'total': total, 'partial': partial}

    async def reconcile(self):
        sources, coverage = await self.manifest()
        live = {source_id(s): s for s in sources}
        for doc in await asyncio.to_thread(self.store.list_documents, self.owner):
            current = live.get(doc['documentId'])
            if not current or doc.get('source', {}).get('revision') != current['revision']:
                await asyncio.to_thread(self.store.delete, self.owner, doc['documentId'])
        return sources, coverage

    async def content(self, source):
        params = {key: source[key] for key in ('kind', 'id', 'revision')}
        async with self.client.stream('GET', f'{self.config.base_url}/datum/knowledge/content',
                params=params, headers={'Authorization': f'Bearer {self.config.token}'},
                follow_redirects=False, timeout=30) as response:
            if response.status_code != 200:
                raise catalog_status(response.status_code)
            data = bytearray()
            async for piece in response.aiter_bytes():
                data.extend(piece)
                if len(data) > MAX_FILE_BYTES:
                    raise AgentError('平台文档超过16 MiB', 'invalid_document')
        return await asyncio.to_thread(extract_document, bytes(data), source['title'][:200],
            'text' if source['format'] in ('txt', 'md') else source['format'])

    async def sync(self, sources, coverage, embeddings, progress):
        indexed, unchanged, failures = 0, 0, []
        profile = embeddings.profile if embeddings else ''
        for index, source in enumerate(sources):
            progress({'status': f'正在同步平台资料 {index + 1}/{len(sources)}…'})
            identifier = source_id(source)
            current = await asyncio.to_thread(self.store.existing, self.owner, identifier)
            if current and (not embeddings or current['profile'] == profile):
                unchanged += 1
                continue
            try:
                document = await asyncio.to_thread(prepare_document, await self.content(source))
                document['documentId'] = identifier
                vectors = await embeddings.embed([c['text'] for c in document['chunks']]) if embeddings else None
                await asyncio.to_thread(self.store.put, self.owner, document, vectors=vectors,
                                        profile=profile, source=source)
                indexed += 1
            except (AgentError, httpx.HTTPError) as error:
                if isinstance(error, AgentError) and error.code == 'catalog_auth_expired':
                    raise
                # Never serialize provider errors, URLs or raw document bytes.
                failures.append({'title': source['title'][:200],
                    'reason': '无法建立索引，请检查格式、容量或模型设置后重试'})
        # A catalog change while parsing must not leave newly stale rows visible.
        _, coverage = await self.reconcile()
        return {'documents': await asyncio.to_thread(self.store.list_documents, self.owner),
                'coverage': coverage, 'sync': {'indexed': indexed, 'unchanged': unchanged, 'failures': failures}}

    async def live_chunk(self, document_id, chunk_id):
        doc = await asyncio.to_thread(self.store.existing, self.owner, document_id)
        if not doc or 'source' not in doc:
            raise AgentError('文档已变化，请同步平台资料后重试', 'not_found')
        source = doc['source']
        live = PlatformSource.model_validate(await self.json('source',
            params={k: source[k] for k in ('kind', 'id', 'revision')})).model_dump()
        if live != source:
            raise AgentError('文档已变化，请同步平台资料后重试', 'stale_source')
        chunk = await asyncio.to_thread(self.store.get_chunk, self.owner, document_id, chunk_id)
        return {**chunk, 'source': source}

    async def enrich(self, result):
        documents = {d['documentId']: d for d in await asyncio.to_thread(self.store.list_documents, self.owner)}
        for chunk in result.get('retrieval', {}).get('chunks', []):
            doc = documents.get(chunk['documentId'])
            if not doc:
                raise AgentError('文档已变化，请同步平台资料后重新检索', 'stale_source')
            chunk['source'] = doc['source']
        return result
