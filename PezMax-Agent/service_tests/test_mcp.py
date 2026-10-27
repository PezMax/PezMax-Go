import asyncio
import json
import sys
import tempfile
import unittest
from pathlib import Path

from mcp import Client
from mcp.client.stdio import StdioServerParameters

from pezmax_agent.knowledge_store import KnowledgeStore, prepare_document
from pezmax_agent.mcp_server import create_mcp


class McpTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.path = Path(self.temp.name) / 'knowledge.sqlite3'
        self.store = KnowledgeStore(self.path)
        self.doc = prepare_document({'title': 'Synthetic queue notes', 'format': 'text', 'sections': [
            {'location': 2, 'text': 'A bounded queue provides backpressure for producers.'}]})
        self.store.put('alice', self.doc)
        self.store.put('bob', prepare_document({'title': 'Private fixture', 'format': 'text', 'sections': [
            {'location': 1, 'text': 'Private transaction fixture visible only to Bob.'}]}))

    async def asyncTearDown(self):
        self.temp.cleanup()

    async def check_client(self, client):
        listing = await client.list_tools()
        self.assertEqual({t.name for t in listing.tools}, {'list_documents', 'search_documents', 'read_document_chunk'})
        for tool in listing.tools:
            self.assertTrue(tool.annotations.read_only_hint)
            self.assertIsNotNone(tool.output_schema)
            self.assertNotIn('owner', json.dumps(tool.input_schema))
            self.assertNotIn('apiKey', json.dumps(tool.input_schema))
        result = await client.call_tool('search_documents', {'question': 'backpressure'})
        self.assertFalse(result.is_error, result)
        data = result.structured_content
        self.assertEqual(data['chunks'][0]['documentId'], self.doc['documentId'])
        chunk_id = data['chunks'][0]['chunkId']
        read = await client.call_tool('read_document_chunk', {'document_id': self.doc['documentId'], 'chunk_id': chunk_id})
        self.assertEqual(read.structured_content['location'], 2)
        manifest = await client.read_resource('pezmax://documents')
        documents = json.loads(manifest.contents[0].text)['documents']
        self.assertEqual(len(documents), 1)
        self.assertEqual(documents[0]['title'], 'Synthetic queue notes')
        excerpt = await client.read_resource(f'pezmax://documents/{self.doc["documentId"]}/chunks/{chunk_id}')
        self.assertIn('backpressure', json.loads(excerpt.contents[0].text)['text'])
        missing = await client.call_tool('read_document_chunk', {'document_id': '0' * 32, 'chunk_id': '0' * 32})
        self.assertTrue(missing.is_error)

    async def test_official_in_process_client_tools_and_resources(self):
        async with Client(create_mcp(self.store, 'alice')) as client:
            await self.check_client(client)

    async def test_official_stdio_client_real_process_handshake_tools_and_resources(self):
        server = StdioServerParameters(command=sys.executable, args=['-m', 'pezmax_agent.mcp_server'],
            env={'PEZMAX_AGENT_DB': str(self.path), 'PEZMAX_AGENT_MCP_OWNER': 'alice', 'PYTHONIOENCODING': 'utf-8'})
        async with asyncio.timeout(30):
            async with Client(server) as client:
                await self.check_client(client)
