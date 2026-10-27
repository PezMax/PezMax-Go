import asyncio
import json
import tempfile
import unittest
from pathlib import Path

import httpx

from pezmax_agent.server import create_app
from pezmax_agent.service_config import ServiceSettings


TOKEN_A = 'synthetic-alice-token-' + 'a' * 32
TOKEN_B = 'synthetic-bob-token-' + 'b' * 32
PROVIDER = {'baseUrl': 'https://model.example/v1', 'model': 'scripted-fixture', 'apiKey': 'synthetic-model-key'}
DOC = {'title': 'Synthetic SQL notes', 'format': 'text', 'sections': [
    {'location': 1, 'text': 'A transaction commits all changes atomically.'}]}


class ServerTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.calls = []

        async def handler(request):
            value = json.loads(request.content)
            self.calls.append(value)
            if request.url.path.endswith('/embeddings'):
                return httpx.Response(200, json={'data': [{'index': i, 'embedding': [1, 0]} for i in range(len(value['input']))]})
            try:
                user = json.loads(value['messages'][-1]['content'])
            except ValueError:
                user = {}
            if 'excerpts' in user:
                chunk = user['excerpts'][0]
                citation = {k: chunk[k] for k in ('documentId', 'chunkId', 'location')}
                citation['quote'] = 'commits all changes atomically'
                content = json.dumps({'answers': [{'text': 'Changes commit atomically.', 'citations': [citation]}]})
            else:
                content = '连接成功'
            return httpx.Response(200, json={'choices': [{'finish_reason': 'stop', 'message': {'role': 'assistant', 'content': content}}]})

        self.settings = ServiceSettings(Path(self.temp.name) / 'knowledge.sqlite3', {'alice': TOKEN_A, 'bob': TOKEN_B},
                                        provider_urls=('https://model.example/v1',))
        self.app = create_app(self.settings, transport=httpx.MockTransport(handler))
        self.lifespan = self.app.router.lifespan_context(self.app)
        await self.lifespan.__aenter__()
        self.client = httpx.AsyncClient(transport=httpx.ASGITransport(app=self.app), base_url='http://local.test',
                                        headers={'Authorization': f'Bearer {TOKEN_A}'})

    async def asyncTearDown(self):
        await self.client.aclose()
        await self.lifespan.__aexit__(None, None, None)
        self.temp.cleanup()

    async def ingest(self, **extra):
        response = await self.client.post('/v1/knowledge/documents', json={'document': DOC, **extra})
        self.assertEqual(response.status_code, 200, response.text)
        return response.json()['documentId']

    async def test_health_openapi_and_auth_before_parsing(self):
        self.assertEqual((await self.client.get('/health')).status_code, 200)
        schema = (await self.client.get('/openapi.json')).json()
        self.assertIn('HTTPBearer', schema['components']['securitySchemes'])
        self.assertNotIn(TOKEN_A, json.dumps(schema))
        response = await self.client.post('/v1/knowledge/documents', content=b'bad-json', headers={'Authorization': 'Bearer wrong'})
        self.assertEqual(response.status_code, 401)
        self.assertEqual(self.calls, [])

    async def test_full_import_search_cited_qa_read_and_delete_flow(self):
        document_id = await self.ingest()
        repeated = await self.client.post('/v1/knowledge/documents', json={'document': DOC})
        self.assertFalse(repeated.json()['inserted'])
        self.assertEqual(len((await self.client.get('/v1/knowledge/documents')).json()['documents']), 1)
        response = await self.client.post('/v1/knowledge/ask', json={'query': {'question': 'transaction'}, 'provider': PROVIDER})
        self.assertEqual(response.status_code, 200, response.text)
        result = response.json()
        self.assertEqual(result['diagnostics']['modelCalls'], 1)
        self.assertEqual(result['answers'][0]['citations'][0]['documentId'], document_id)
        chunk_id = result['answers'][0]['citations'][0]['chunkId']
        direct = await self.client.get(f'/v1/knowledge/documents/{document_id}/chunks/{chunk_id}')
        self.assertIn('atomically', direct.json()['text'])
        self.assertNotIn('synthetic-model-key', response.text)
        self.assertEqual((await self.client.delete(f'/v1/knowledge/documents/{document_id}')).status_code, 200)
        self.assertEqual((await self.client.get(f'/v1/knowledge/documents/{document_id}/chunks/{chunk_id}')).status_code, 404)

    async def test_embedding_ingest_hybrid_and_no_silent_fallback(self):
        missing = await self.client.post('/v1/knowledge/search', json={'query': {'question': 'atomic', 'mode': 'hybrid'}})
        self.assertEqual(missing.status_code, 400)
        await self.ingest(embedding=PROVIDER)
        response = await self.client.post('/v1/knowledge/search', json={
            'query': {'question': 'consistency', 'mode': 'hybrid'}, 'embedding': PROVIDER})
        self.assertEqual(response.status_code, 200, response.text)
        self.assertEqual(response.json()['fusion'], 'rrf-k60')
        self.assertEqual(len(self.calls), 2)
        self.assertEqual(len(response.json()['chunks']), 1)

    async def test_cross_owner_ids_cannot_read_delete_or_retrieve(self):
        document_id = await self.ingest()
        result = (await self.client.post('/v1/knowledge/search', json={'query': {'question': 'transaction'}})).json()
        chunk_id = result['chunks'][0]['chunkId']
        headers = {'Authorization': f'Bearer {TOKEN_B}'}
        self.assertEqual((await self.client.get('/v1/knowledge/documents', headers=headers)).json()['documents'], [])
        for url in (f'/v1/knowledge/documents/{document_id}/chunks/{chunk_id}',):
            self.assertEqual((await self.client.get(url, headers=headers)).status_code, 404)
        self.assertEqual((await self.client.delete(f'/v1/knowledge/documents/{document_id}', headers=headers)).status_code, 404)
        result = await self.client.post('/v1/knowledge/search', headers=headers,
            json={'query': {'question': 'transaction', 'documentIds': [document_id]}})
        self.assertEqual(result.json()['scannedChunks'], 0)

    async def test_invalid_input_error_never_echoes_body_or_secrets(self):
        for value in ({'provider': {**PROVIDER, 'apiKey': 123}, 'unknown': 'synthetic-sensitive-body'},
                      {'provider': PROVIDER, 'question': 123}, {'provider': PROVIDER, 'task': 'shell'}):
            response = await self.client.post('/v1/run', json=value)
            self.assertEqual(response.status_code, 422)
            self.assertNotIn('synthetic', response.text)
            self.assertNotIn('input', response.text)
        self.assertEqual(self.calls, [])

    async def test_unapproved_provider_and_catalog_override_rejected(self):
        response = await self.client.post('/v1/run', json={'provider': {**PROVIDER, 'baseUrl': 'http://127.0.0.1:1'}, 'test': True})
        self.assertEqual(response.status_code, 403)
        response = await self.client.post('/v1/run', json={'provider': PROVIDER, 'catalog': {'baseUrl': 'http://127.0.0.1:1'}})
        self.assertEqual(response.status_code, 422)
        self.assertEqual(self.calls, [])

    async def test_legacy_connection_test_uses_shared_application_service(self):
        response = await self.client.post('/v1/run', json={'provider': PROVIDER, 'test': True})
        self.assertEqual(response.status_code, 200, response.text)
        self.assertEqual(response.json()['content'], '连接成功')
        self.assertNotIn('synthetic-model-key', response.text)

    async def test_body_limit_with_small_config(self):
        settings = ServiceSettings(Path(self.temp.name) / 'small.sqlite3', {'alice': TOKEN_A}, body_limit=128)
        app = create_app(settings)
        async with httpx.AsyncClient(transport=httpx.ASGITransport(app=app), base_url='http://local.test') as client:
            response = await client.post('/v1/knowledge/documents', content=b'x' * 129, headers={'Authorization': f'Bearer {TOKEN_A}'})
            self.assertEqual(response.status_code, 413)

    async def test_timeout_cancels_upstream_request_without_error_details(self):
        cancelled = asyncio.Event()

        async def slow(_):
            try:
                await asyncio.sleep(30)
            finally:
                cancelled.set()

        settings = ServiceSettings(Path(self.temp.name) / 'slow.sqlite3', {'alice': TOKEN_A},
            provider_urls=self.settings.provider_urls, timeout=.03)
        app = create_app(settings, transport=httpx.MockTransport(slow))
        async with app.router.lifespan_context(app):
            async with httpx.AsyncClient(transport=httpx.ASGITransport(app=app), base_url='http://local.test') as client:
                response = await client.post('/v1/run', json={'provider': PROVIDER, 'test': True}, headers={'Authorization': f'Bearer {TOKEN_A}'})
        self.assertEqual(response.status_code, 504)
        self.assertTrue(cancelled.is_set())

    async def test_client_disconnect_cancels_pending_generation(self):
        entered, cancelled = asyncio.Event(), asyncio.Event()

        async def slow(_):
            entered.set()
            try:
                await asyncio.sleep(30)
            finally:
                cancelled.set()

        app = create_app(self.settings, transport=httpx.MockTransport(slow))
        events, responses = asyncio.Queue(), []
        await events.put({'type': 'http.request', 'body': json.dumps({'provider': PROVIDER, 'test': True}).encode(), 'more_body': False})

        async def send(message):
            responses.append(message)

        scope = {'type': 'http', 'asgi': {'version': '3.0'}, 'http_version': '1.1', 'method': 'POST',
            'scheme': 'http', 'path': '/v1/run', 'raw_path': b'/v1/run', 'root_path': '', 'query_string': b'',
            'headers': [(b'content-type', b'application/json'), (b'authorization', ('Bearer ' + TOKEN_A).encode())],
            'client': ('127.0.0.1', 1), 'server': ('local.test', 80)}
        async with app.router.lifespan_context(app):
            task = asyncio.create_task(app(scope, events.get, send))
            try:
                await asyncio.wait_for(entered.wait(), 3)
                await events.put({'type': 'http.disconnect'})
                await asyncio.wait_for(task, 3)
            finally:
                task.cancel()
                await asyncio.gather(task, return_exceptions=True)
        self.assertTrue(cancelled.is_set())
        self.assertEqual(responses, [])

    async def test_concurrency_rejects_instead_of_unbounded_queue(self):
        entered, release = asyncio.Event(), asyncio.Event()

        async def blocked(_):
            entered.set()
            await release.wait()
            return httpx.Response(200, json={'choices': [{'finish_reason': 'stop', 'message': {'role': 'assistant', 'content': 'ok'}}]})

        settings = ServiceSettings(Path(self.temp.name) / 'busy.sqlite3', {'alice': TOKEN_A},
            provider_urls=self.settings.provider_urls, concurrency=1)
        app = create_app(settings, transport=httpx.MockTransport(blocked))
        async with app.router.lifespan_context(app):
            async with httpx.AsyncClient(transport=httpx.ASGITransport(app=app), base_url='http://local.test',
                headers={'Authorization': f'Bearer {TOKEN_A}'}) as client:
                first = asyncio.create_task(client.post('/v1/run', json={'provider': PROVIDER, 'test': True}))
                await asyncio.wait_for(entered.wait(), 3)
                try:
                    self.assertEqual((await client.get('/v1/knowledge/documents')).status_code, 429)
                finally:
                    release.set()
                    self.assertEqual((await first).status_code, 200)
