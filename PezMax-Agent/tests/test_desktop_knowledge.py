import tempfile
import unittest
from pathlib import Path

from pezmax_agent.cli import execute
from pezmax_agent.desktop_knowledge import DesktopKnowledgeInput, read_selected_file
from pezmax_agent.errors import AgentError


class DesktopKnowledgeTests(unittest.IsolatedAsyncioTestCase):
    async def test_local_lifecycle_isolated_by_owner_without_model_credentials(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / 'synthetic-queue.md'
            source.write_text('A queue uses first in first out ordering. 队列使用先进先出顺序。', encoding='utf-8')
            payload = {'task': 'knowledge', 'storage': {'path': str(root / 'documents.sqlite3'), 'owner': 'a' * 64}}

            async def run(request, **extra):
                events = []
                await execute({**payload, 'knowledge': request, **extra}, lambda kind, **fields: events.append((kind, fields)))
                return events[-1][1]['data']['knowledge']

            imported = await run({'action': 'import'}, localDocument=str(source))
            document_id = imported['document']['documentId']
            self.assertTrue(imported['document']['inserted'])
            self.assertFalse((await run({'action': 'import'}, localDocument=str(source)))['document']['inserted'])
            self.assertEqual(len((await run({'action': 'list'}))['documents']), 1)
            result = await run({'action': 'search', 'query': {'question': '先进先出'}})
            chunk = result['retrieval']['chunks'][0]
            self.assertEqual(chunk['location'], 1)
            self.assertEqual((await run({'action': 'read', 'documentId': document_id, 'chunkId': chunk['chunkId']}))['chunk'], chunk)
            original = payload['storage']['owner']
            payload['storage']['owner'] = 'b' * 64
            self.assertEqual((await run({'action': 'list'}))['documents'], [])
            with self.assertRaises(AgentError):
                await run({'action': 'read', 'documentId': document_id, 'chunkId': chunk['chunkId']})
            self.assertFalse((await run({'action': 'delete', 'documentId': document_id}))['deleted'])
            payload['storage']['owner'] = original
            self.assertTrue((await run({'action': 'delete', 'documentId': document_id}))['deleted'])
            self.assertEqual((await run({'action': 'list'}))['documents'], [])

    async def test_missing_trusted_storage_and_malformed_inputs_are_rejected(self):
        for storage in (None, {}, {'path': 'relative.sqlite3', 'owner': 'a' * 64}, {'path': str(Path.cwd() / 'x'), 'owner': '../bad'}):
            with self.assertRaisesRegex(AgentError, '参数无效'):
                await execute({'task': 'knowledge', 'knowledge': {'action': 'list'}, 'storage': storage}, lambda *_args, **_kwargs: None)
        for request in ({'action': 'read'}, {'action': 'delete'}, {'action': 'search'},
                        {'action': 'search', 'mode': 'hybrid', 'query': {'question': 'queue'}},
                        {'action': 'import', 'localDocument': 'untrusted'}):
            with self.assertRaises(ValueError):
                DesktopKnowledgeInput.model_validate(request)

    async def test_paid_operations_require_separate_explicit_config(self):
        with tempfile.TemporaryDirectory() as directory:
            payload = {'task': 'knowledge', 'storage': {'path': str(Path(directory) / 'documents.sqlite3'), 'owner': 'a' * 64}}
            for request in ({'action': 'ask', 'query': {'question': 'queue'}},
                            {'action': 'search', 'mode': 'hybrid', 'query': {'question': 'queue', 'mode': 'hybrid'}}):
                with self.assertRaises(AgentError):
                    await execute({**payload, 'knowledge': request}, lambda *_args, **_kwargs: None)

    async def test_selected_file_errors_never_echo_path_or_document_bytes(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'private-secret.pdf'
            path.write_bytes(b'private-file-content')
            for value in (None, 'relative.md', 'https://example.test/private.md', str(path)):
                with self.assertRaises(AgentError) as error:
                    read_selected_file(value)
                self.assertNotIn('private', str(error.exception))
