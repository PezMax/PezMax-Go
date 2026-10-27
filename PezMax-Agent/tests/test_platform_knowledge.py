import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import httpx

from pezmax_agent.contracts import CatalogConfig
from pezmax_agent.desktop_knowledge import run_desktop_knowledge
from pezmax_agent.errors import AgentError
from pezmax_agent.knowledge_schema import SearchInput
from pezmax_agent.knowledge_store import KnowledgeStore, retrieve
from pezmax_agent.platform_knowledge import PlatformKnowledge, source_id


def source(kind='file', identifier='9007199254740993', revision='a' * 64):
    return {'kind': kind, 'id': identifier, 'revision': revision, 'title': 'Synthetic queue notes',
            'format': 'md', 'subject': 'Software', 'school': 'Synthetic campus', 'year': 2026}


class PlatformKnowledgeTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.store = KnowledgeStore(Path(self.temp.name) / 'index.sqlite3')
        self.sources = [source()]
        self.text = b'A queue uses first in first out ordering.'
        self.calls = []
        self.fail = False

        def handler(request):
            self.calls.append(request)
            self.assertEqual(request.url.host, 'catalog.example')
            self.assertEqual(request.headers.get('Authorization'), 'Bearer platform-secret')
            if self.fail:
                return httpx.Response(503)
            if request.url.path.endswith('/manifest'):
                return httpx.Response(200, json={'code': 0, 'data': {
                    'sources': self.sources, 'total': len(self.sources), 'partial': False}})
            if request.url.path.endswith('/content'):
                return httpx.Response(200, content=self.text)
            if request.url.path.endswith('/source'):
                if not self.sources:
                    return httpx.Response(404)
                return httpx.Response(200, json={'code': 0, 'data': self.sources[0]})
            raise AssertionError(str(request.url))

        self.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        self.addAsyncCleanup(self.client.aclose)
        self.platform = PlatformKnowledge(self.client, CatalogConfig('https://catalog.example', 'platform-secret'), self.store, 'owner')

    async def sync(self, embedding=None):
        sources, coverage = await self.platform.reconcile()
        return await self.platform.sync(sources, coverage, embedding, lambda _: None)

    async def test_sync_incremental_updates_deleted_sources_and_live_citations(self):
        result = await self.sync()
        doc = result['documents'][0]
        self.assertEqual(doc['documentId'], source_id(self.sources[0]))
        self.assertEqual(doc['source']['id'], '9007199254740993')
        chunk = retrieve(self.store, 'owner', SearchInput(question='first in first out'))['chunks'][0]
        self.assertEqual((await self.platform.live_chunk(doc['documentId'], chunk['chunkId']))['source'], self.sources[0])
        content_calls = sum(r.url.path.endswith('/content') for r in self.calls)
        self.assertEqual((await self.sync())['sync']['unchanged'], 1)
        self.assertEqual(sum(r.url.path.endswith('/content') for r in self.calls), content_calls)

        self.sources[0] = source(revision='b' * 64)
        self.text = b'A stack uses last in first out ordering.'
        updated = await self.sync()
        self.assertEqual(updated['sync']['indexed'], 1)
        self.assertEqual(len(updated['documents']), 1)
        self.assertEqual(updated['documents'][0]['documentId'], doc['documentId'])
        with self.assertRaises(AgentError):
            await self.platform.live_chunk(doc['documentId'], chunk['chunkId'])
        self.assertEqual(retrieve(self.store, 'owner', SearchInput(question='queue'))['chunks'], [])
        self.sources = []
        await self.platform.reconcile()
        self.assertEqual(self.store.list_documents('owner'), [])

    async def test_identical_content_keeps_distinct_business_sources_and_owners(self):
        self.sources.append(source('ebook', '42'))
        result = await self.sync()
        self.assertEqual(len(result['documents']), 2)
        self.assertEqual(len({d['documentId'] for d in result['documents']}), 2)
        self.assertEqual(self.store.list_documents('other'), [])
        self.sources.pop(0)
        await self.platform.reconcile()
        self.assertEqual(self.store.list_documents('owner')[0]['source']['kind'], 'ebook')

    async def test_catalog_outage_fails_closed_and_redirects_are_not_followed(self):
        await self.sync()
        self.fail = True
        with self.assertRaises(AgentError):
            await self.platform.reconcile()
        self.assertEqual(len(self.store.list_documents('owner')), 1)
        async with httpx.AsyncClient(transport=httpx.MockTransport(lambda _: httpx.Response(302, headers={'Location': 'https://outside.example'}))) as client:
            platform = PlatformKnowledge(client, self.platform.config, self.store, 'owner')
            with self.assertRaises(AgentError):
                await platform.manifest()
            with self.assertRaises(AgentError):
                await platform.content(self.sources[0])

    async def test_failed_extraction_is_visible_and_retryable_without_credentials(self):
        self.text = b'\xffinvalid UTF-8'
        failed = await self.sync()
        self.assertEqual(len(failed['sync']['failures']), 1)
        self.assertEqual(failed['documents'], [])
        self.text = b'A queue uses first in first out ordering.'
        self.assertEqual((await self.sync())['sync']['indexed'], 1)

    async def test_explicit_embedding_profile_upgrades_and_does_not_mix_vectors(self):
        await self.sync()
        class Embedding:
            profile = 'synthetic-profile'
            async def embed(self, text):
                return [[1, 0] for _ in text]
        result = await self.sync(Embedding())
        self.assertEqual(result['documents'][0]['dimension'], 2)
        self.assertEqual((await self.sync())['sync']['unchanged'], 1)
        self.assertEqual(self.store.list_documents('owner')[0]['dimension'], 2)
        self.assertEqual((await self.sync(Embedding()))['sync']['unchanged'], 1)

    async def test_real_desktop_adapter_separates_local_database_and_rechecks_before_return(self):
        payload = {'storage': {'path': str(Path(self.temp.name) / 'desktop.sqlite3'), 'owner': 'a' * 64},
                   'catalog': {'baseUrl': 'https://catalog.example', 'token': 'platform-secret'},
                   'knowledge': {'action': 'sync', 'library': 'platform'}}
        factory = httpx.AsyncClient
        def client_factory(**kwargs):
            return factory(transport=self.client._transport, **kwargs)
        with patch('pezmax_agent.desktop_knowledge.httpx.AsyncClient', client_factory):
            synced = (await run_desktop_knowledge(payload, lambda _: None))['knowledge']
            self.assertEqual(len(synced['documents']), 1)
            payload['knowledge'] = {'action': 'search', 'library': 'platform', 'query': {'question': 'queue'}}
            result = (await run_desktop_knowledge(payload, lambda _: None))['knowledge']
            self.assertEqual(result['retrieval']['chunks'][0]['source'], self.sources[0])
            self.sources = []
            result = (await run_desktop_knowledge(payload, lambda _: None))['knowledge']
            self.assertEqual(result['retrieval']['chunks'], [])
            payload['knowledge'] = {'action': 'list'}
            self.assertEqual((await run_desktop_knowledge(payload, lambda _: None))['knowledge']['documents'], [])

    async def test_reconcile_rejects_duplicate_sources_and_invalid_coverage(self):
        self.sources *= 2
        with self.assertRaisesRegex(AgentError, '响应无效'):
            await self.platform.reconcile()
