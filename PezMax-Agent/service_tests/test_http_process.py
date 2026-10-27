import asyncio
import json
import os
import socket
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import httpx


class HttpProcessTests(unittest.IsolatedAsyncioTestCase):
    async def test_real_uvicorn_process_serves_authenticated_knowledge_api(self):
        with tempfile.TemporaryDirectory() as temp:
            with socket.socket() as listener:
                listener.bind(('127.0.0.1', 0))
                port = listener.getsockname()[1]
            token = 'synthetic-process-token-' + 'x' * 32
            env = {**os.environ, 'PEZMAX_AGENT_DB': str(Path(temp) / 'knowledge.sqlite3'),
                'PEZMAX_AGENT_ACCESS_TOKENS': json.dumps({'fixture': token}), 'PEZMAX_AGENT_PROVIDER_URLS': '[]'}
            flags = subprocess.CREATE_NO_WINDOW if sys.platform == 'win32' else 0
            process = subprocess.Popen([sys.executable, '-m', 'pezmax_agent.server', '--port', str(port)],
                env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, creationflags=flags)
            try:
                async with httpx.AsyncClient(base_url=f'http://127.0.0.1:{port}', trust_env=False, timeout=2) as client:
                    for _ in range(100):
                        if process.poll() is not None:
                            self.fail('Service process exited during startup')
                        try:
                            if (await client.get('/health')).status_code == 200:
                                break
                        except httpx.HTTPError:
                            pass
                        await asyncio.sleep(.1)
                    else:
                        self.fail('Service did not become ready')
                    self.assertEqual((await client.get('/v1/knowledge/documents')).status_code, 401)
                    client.headers['Authorization'] = 'Bearer ' + token
                    response = await client.post('/v1/knowledge/documents', json={'document': {'title': 'Synthetic notes',
                        'format': 'text', 'sections': [{'location': 1, 'text': 'A bounded queue provides backpressure.'}]}})
                    self.assertEqual(response.status_code, 200, response.text)
                    found = await client.post('/v1/knowledge/search', json={'query': {'question': 'backpressure'}})
                    self.assertEqual(found.status_code, 200, found.text)
                    self.assertEqual(len(found.json()['chunks']), 1)
                    root = Path(__file__).resolve().parents[1]
                    demo = await asyncio.to_thread(subprocess.run, [sys.executable, str(root / 'examples/service_demo.py'),
                        str(root / 'examples/queue-notes.md'), '背压', '--url', f'http://127.0.0.1:{port}'],
                        env={**env, 'PEZMAX_AGENT_DEMO_TOKEN': token}, capture_output=True, timeout=15)
                    self.assertEqual(demo.returncode, 0, demo.stderr.decode('utf-8', errors='replace'))
                    outputs = [json.loads(line) for line in demo.stdout.decode('utf-8').splitlines()]
                    self.assertTrue(outputs[0]['import']['inserted'])
                    self.assertIn('背压', outputs[1]['chunks'][0]['text'])
            finally:
                process.terminate()
                try:
                    await asyncio.to_thread(process.wait, 5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    await asyncio.to_thread(process.wait, 5)
                process.communicate()
