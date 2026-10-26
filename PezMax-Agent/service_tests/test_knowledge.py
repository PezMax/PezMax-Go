import asyncio
import io
import json
import math
import tempfile
import unittest
import zipfile
from pathlib import Path
from unittest.mock import AsyncMock

import httpx
from pydantic import ValidationError

from pezmax_agent.contracts import ProviderConfig
from pezmax_agent.document_import import extract_document
from pezmax_agent.embeddings import Embeddings
from pezmax_agent.errors import AgentError
from pezmax_agent.knowledge import KnowledgeService, inspect_knowledge, run_rag
from pezmax_agent.knowledge_schema import DocumentInput, SearchInput
from pezmax_agent.knowledge_store import KnowledgeStore, prepare_document, retrieve, unit_vector


def document(text='导数描述函数的瞬时变化率。', location=3, title='数学样例', format='pdf'):
    return {'title': title, 'format': format, 'sections': [{'location': location, 'text': text}]}


class StoreTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.path = Path(self.temp.name) / 'corpus.sqlite3'
        self.store = KnowledgeStore(self.path)

    def tearDown(self):
        self.temp.cleanup()

    def test_persistence_dedup_and_delete_cascade(self):
        doc = prepare_document(document())
        self.assertTrue(self.store.put('alice', doc)['inserted'])
        reopened = KnowledgeStore(self.path)
        self.assertFalse(reopened.put('alice', doc)['inserted'])
        self.assertEqual(len(reopened.list_documents('alice')), 1)
        self.assertEqual(reopened.chunks('alice')[0]['location'], 3)
        self.assertTrue(reopened.delete('alice', doc['documentId']))
        self.assertEqual(reopened.chunks('alice'), [])
        self.assertFalse(reopened.delete('alice', doc['documentId']))

    def test_owner_filter_precedes_ranking_and_direct_read(self):
        first = prepare_document(document())
        second = prepare_document(document('private alice evidence'))
        self.store.put('alice', first)
        self.store.put('bob', second)
        self.assertEqual(self.store.list_documents('bob')[0]['documentId'], second['documentId'])
        self.assertEqual(retrieve(self.store, 'alice', SearchInput(question='private'))['chunks'], [])
        with self.assertRaises(AgentError):
            self.store.get_chunk('alice', second['documentId'], second['chunks'][0]['chunkId'])
        self.assertFalse(self.store.delete('alice', second['documentId']))
        self.assertEqual(len(self.store.chunks('bob')), 1)

    def test_cjk_and_english_bm25_and_document_filter(self):
        math_doc = prepare_document(document())
        code_doc = prepare_document(document('A database transaction commits atomically.', title='Code'))
        for doc in (math_doc, code_doc):
            self.store.put('alice', doc)
        for query, expected in [('瞬时变化率', math_doc), ('transaction', code_doc)]:
            result = retrieve(self.store, 'alice', SearchInput(question=query))
            self.assertEqual(result['chunks'][0]['documentId'], expected['documentId'])
            self.assertEqual(result['fusion'], 'bm25')
        result = retrieve(self.store, 'alice', SearchInput(question='transaction', documentIds=[math_doc['documentId']]))
        self.assertEqual(result['chunks'], [])
        self.assertEqual(result['scannedChunks'], 1)

    def test_chunk_overlap_location_and_stable_content_hash(self):
        self.assertEqual(len(prepare_document(document('A' * 1200))['chunks']), 1)
        prepared = prepare_document(document('A' * 3000))
        self.assertEqual(len(prepared['chunks']), 3)
        self.assertTrue(all(c['location'] == 3 and len(c['text']) <= 1200 for c in prepared['chunks']))
        self.assertEqual(prepared['chunks'][0]['text'][-200:], prepared['chunks'][1]['text'][:200])
        self.assertEqual(prepare_document(document('A' * 3000, title='Renamed'))['documentId'], prepared['documentId'])
        self.assertNotEqual(prepare_document(document('A' * 3000, location=4))['documentId'], prepared['documentId'])

    def test_exact_cosine_rrf_finds_semantic_only_document(self):
        first = prepare_document(document('A derivative is a local rate of change.'))
        second = prepare_document(document('A semaphore bounds concurrent tasks.'))
        self.store.put('alice', first, vectors=[[1, 0]], profile='fixture')
        self.store.put('alice', second, vectors=[[0, 1]], profile='fixture')
        self.assertEqual(retrieve(self.store, 'alice', SearchInput(question='slope'))['chunks'], [])
        result = retrieve(self.store, 'alice', SearchInput(question='slope', mode='hybrid', limit=1), vector=[1, 0], profile='fixture')
        self.assertEqual(result['chunks'][0]['documentId'], first['documentId'])
        self.assertEqual(result['fusion'], 'rrf-k60')
        self.assertTrue(all('vector' not in c for c in result['chunks']))

    def test_hybrid_requires_matching_model_dimension_and_explicit_mode(self):
        doc = prepare_document(document())
        self.store.put('alice', doc, vectors=[[1, 0]], profile='model-a')
        query = SearchInput(question='导数', mode='hybrid')
        for changes in ({}, {'vector': [1, 0], 'profile': 'model-b'}, {'vector': [1], 'profile': 'model-a'}):
            with self.assertRaises(AgentError):
                retrieve(self.store, 'alice', query, **changes)
        # Keyword queries do not require or expose embedding credentials.
        self.assertEqual(retrieve(self.store, 'alice', SearchInput(question='导数'))['mode'], 'keyword')
        self.assertEqual(self.store.chunks('alice', include_vectors=False)[0]['vector'], 1)
        with self.assertRaises(AgentError):
            self.store.put('alice', doc)

    def test_embedding_contract_rejects_nan_zero_and_inconsistent_vectors(self):
        for value in ([], [0, 0], [True, 0], ['1'], [math.nan], [math.inf]):
            with self.assertRaises(AgentError):
                unit_vector(value)
        with self.assertRaises(AgentError):
            self.store.put('alice', prepare_document(document('a' * 1500)), vectors=[[1], [1, 0]], profile='fixture')
        self.assertEqual(self.store.list_documents('alice'), [])

    def test_strict_input_rejects_duplicate_positions_coercion_and_empty_text(self):
        for value in ({**document(), 'sections': [{'location': True, 'text': 'text'}]},
                      {**document(), 'sections': [{'location': 1, 'text': 'x'}] * 2},
                      {**document(), 'owner': 'other'}):
            with self.assertRaises(ValidationError):
                DocumentInput.model_validate(value)
        with self.assertRaises(AgentError):
            prepare_document(document('  \n  '))

    def test_local_pdf_text_and_epub_extraction_preserve_positions(self):
        from pypdf import PdfWriter
        writer = PdfWriter()
        page = writer.add_blank_page(100, 100)
        # Blank pages retain their physical position; empty import is rejected later.
        stream = io.BytesIO()
        writer.write(stream)
        self.assertEqual(extract_document(stream.getvalue(), 'PDF', 'pdf').sections[0].location, 1)
        self.assertEqual(extract_document('测试 UTF8'.encode(), 'Text', 'text').sections[0].text, '测试 UTF8')
        archive = io.BytesIO()
        with zipfile.ZipFile(archive, 'w') as book:
            book.writestr('META-INF/container.xml', '<container><rootfile full-path="OPS/book.opf"/></container>')
            book.writestr('OPS/book.opf', '<package><manifest><item id="a" href="a.xhtml" media-type="application/xhtml+xml"/>'
                          '<item id="b" href="b.xhtml" media-type="application/xhtml+xml"/></manifest>'
                          '<spine><itemref idref="a"/><itemref idref="b"/></spine></package>')
            book.writestr('OPS/a.xhtml', '<html><body>First section<script>do not execute</script>visible tail</body></html>')
            book.writestr('OPS/b.xhtml', '<html><body>Second section</body></html>')
        result = extract_document(archive.getvalue(), 'EPUB', 'epub')
        self.assertEqual([s.location for s in result.sections], [1, 2])
        self.assertNotIn('do not execute', result.sections[0].text)
        self.assertIn('visible tail', result.sections[0].text)

    def test_invalid_archives_files_and_encrypted_documents_fail_safely(self):
        for data, format in ((b'broken', 'epub'), (b'broken', 'pdf'), (b'\xff', 'text'), (b'x', 'other')):
            with self.assertRaises(AgentError) as ctx:
                extract_document(data, 'Fixture', format)
            self.assertNotIn('broken', str(ctx.exception))


class AsyncKnowledgeTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.store = KnowledgeStore(Path(self.temp.name) / 'corpus.sqlite3')
        self.service = KnowledgeService(self.store, 'alice')
        await self.service.ingest(document())
        self.query = SearchInput(question='导数')
        self.chunk = (await self.service.search(self.query))['chunks'][0]
        self.answer = {'text': '导数是瞬时变化率。', 'citations': [{k: self.chunk[k] for k in ('documentId', 'chunkId', 'location')}
                        | {'quote': '导数描述函数的瞬时变化率。'}]}

    async def asyncTearDown(self):
        self.temp.cleanup()

    async def test_graph_validates_full_document_chunk_location_quote_tuple(self):
        model = AsyncMock()
        model.complete.return_value = {'content': json.dumps({'answers': [self.answer]})}
        result = await run_rag(model, self.service, self.query)
        self.assertEqual(result['answers'], [self.answer])
        self.assertEqual(result['diagnostics']['modelCalls'], 1)
        self.assertNotIn('导数', json.dumps(result['diagnostics'], ensure_ascii=False))
        args, kwargs = model.complete.call_args
        self.assertEqual(json.loads(args[0][0]['content'])['excerpts'], result['retrieval']['chunks'])
        self.assertFalse(kwargs['allow_tools'])

    async def test_bounded_repair_uses_same_evidence_and_does_not_replay_rejected_answer(self):
        model = AsyncMock()
        model.complete.side_effect = [{'content': 'synthetic-invalid-answer'}, {'content': json.dumps({'answers': [self.answer]})}]
        result = await run_rag(model, self.service, self.query)
        self.assertEqual(result['answers'], [self.answer])
        first, second = model.complete.call_args_list
        self.assertEqual(first.args, second.args)
        self.assertNotIn('synthetic-invalid-answer', json.dumps(second.kwargs))
        self.assertEqual([s['node'] for s in result['diagnostics']['steps']], ['retrieve', 'answer', 'validate', 'repair', 'validate'])

    async def test_cross_document_wrong_position_fabricated_and_coerced_citations_rejected(self):
        for change in ({'documentId': '0' * 32}, {'chunkId': '0' * 32}, {'location': 9},
                       {'location': '3'}, {'location': True}, {'quote': 'a fabricated quotation'}):
            model = AsyncMock()
            candidate = {**self.answer, 'citations': [{**self.answer['citations'][0], **change}]}
            model.complete.return_value = {'content': json.dumps({'answers': [candidate]})}
            result = await run_rag(model, self.service, self.query)
            self.assertEqual(result['answers'], [])
            self.assertEqual(model.complete.call_count, 2)

    async def test_empty_retrieval_skips_generation_and_partial_valid_answers_do_not_retry(self):
        model = AsyncMock()
        result = await run_rag(model, self.service, SearchInput(question='no-match'))
        self.assertEqual(result['diagnostics']['modelCalls'], 0)
        model.complete.assert_not_called()
        invalid = {**self.answer, 'citations': [{**self.answer['citations'][0], 'location': 1}]}
        model.complete.return_value = {'content': json.dumps({'answers': [self.answer, invalid]})}
        result = await run_rag(model, self.service, self.query)
        self.assertEqual(result['answers'], [self.answer])
        self.assertEqual(model.complete.call_count, 1)

    async def test_explicit_abstention_and_network_error_do_not_retry(self):
        model = AsyncMock()
        model.complete.return_value = {'content': '{"answers":[]}'}
        await run_rag(model, self.service, self.query)
        self.assertEqual(model.complete.call_count, 1)
        model.complete.side_effect = AgentError('API Key 无效')
        with self.assertRaises(AgentError):
            await run_rag(model, self.service, self.query)
        self.assertEqual(model.complete.call_count, 2)

    async def test_concurrent_idempotent_import_is_atomic(self):
        results = await asyncio.gather(*(self.service.ingest(document('Concurrent fixture.')) for _ in range(6)))
        self.assertEqual(sum(r['inserted'] for r in results), 1)
        self.assertEqual(len(self.store.list_documents('alice')), 2)

    async def test_embeddings_batches_indexes_and_dedup_avoid_extra_paid_calls(self):
        calls = []

        async def handler(request):
            body = json.loads(request.content)
            calls.append(body)
            return httpx.Response(200, json={'data': [{'index': i, 'embedding': [1, 0]} for i in reversed(range(len(body['input'])))]})

        async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
            encoder = Embeddings(client, ProviderConfig('https://embed.example/v1', 'fixture', 'synthetic-key'))
            service = KnowledgeService(self.store, 'bob', encoder)
            first = await service.ingest(document('vector test ' * 1700))
            second = await service.ingest(document('vector test ' * 1700))
            self.assertTrue(first['inserted'])
            self.assertFalse(second['inserted'])
            self.assertEqual(len(calls), 2)
            await service.search(SearchInput(question='slope', mode='hybrid'))
            self.assertEqual(len(calls), 3)
            self.assertNotIn('synthetic-key', self.store.path.read_bytes().decode('utf-8', errors='ignore'))

    async def test_bad_embedding_response_does_not_leave_partial_document(self):
        for response in ({'data': [{'index': 1, 'embedding': [1, 0]}]}, {'data': [{'index': 0, 'embedding': [0, 0]}]}):
            async with httpx.AsyncClient(transport=httpx.MockTransport(lambda _: httpx.Response(200, json=response))) as client:
                encoder = Embeddings(client, ProviderConfig('https://embed.example/v1', 'fixture', 'synthetic-key'))
                with self.assertRaises(AgentError):
                    await KnowledgeService(self.store, 'bob', encoder).ingest(document())
            self.assertEqual(self.store.list_documents('bob'), [])

    async def test_embedding_service_errors_do_not_expose_body_or_credentials(self):
        for status in (400, 401, 429, 503):
            async with httpx.AsyncClient(transport=httpx.MockTransport(
                lambda _: httpx.Response(status, text='synthetic-sensitive-provider-body synthetic-key'))) as client:
                encoder = Embeddings(client, ProviderConfig('https://embed.example/v1', 'fixture', 'synthetic-key'))
                with self.assertRaises(AgentError) as error:
                    await encoder.embed(['Synthetic input'])
                self.assertNotIn('synthetic', str(error.exception))
