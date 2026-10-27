"""Explicit local file -> API import -> retrieval; generation is opt-in.

Credentials come from this user's environment, never CLI arguments or output.
The demo does not delete documents or read desktop credential storage.
"""
import argparse
import asyncio
import os
import sys
from pathlib import Path

import httpx

from pezmax_agent.contracts import base_url, json_text
from pezmax_agent.document_import import MAX_FILE_BYTES, extract_document
from pezmax_agent.knowledge_cli import connection


def wire(config):
    return {'baseUrl': config.base_url, 'model': config.model, 'apiKey': config.api_key}


async def demo(args):
    path = Path(args.path)
    with path.open('rb') as stream:
        data = stream.read(MAX_FILE_BYTES + 1)
    format = {'.pdf': 'pdf', '.epub': 'epub', '.md': 'text', '.txt': 'text'}.get(path.suffix.lower())
    document = extract_document(data, path.stem, format).model_dump()
    embedding = {'embedding': wire(connection('PEZMAX_AGENT_EMBEDDING'))} if args.embedding else {}
    async with httpx.AsyncClient(base_url=base_url(args.url), timeout=180, follow_redirects=False,
        trust_env=False, headers={'Authorization': 'Bearer ' + os.environ['PEZMAX_AGENT_DEMO_TOKEN']}) as client:
        response = await client.post('/v1/knowledge/documents', json={'document': document, **embedding})
        response.raise_for_status()
        indexed = response.json()
        print(json_text({'import': indexed}))
        payload = {'query': {'question': args.question, 'documentIds': [indexed['documentId']],
                            'mode': 'hybrid' if args.embedding else 'keyword'}, **embedding}
        if args.ask:
            payload['provider'] = wire(connection('PEZMAX_AGENT_GENERATION'))
        response = await client.post('/v1/knowledge/ask' if args.ask else '/v1/knowledge/search', json=payload)
        response.raise_for_status()
        print(json_text(response.json()))


def main():
    sys.stdout.reconfigure(encoding='utf-8')
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('path')
    parser.add_argument('question')
    parser.add_argument('--url', default='http://127.0.0.1:9044')
    parser.add_argument('--embedding', action='store_true', help='Send imported text/question to your embedding provider')
    parser.add_argument('--ask', action='store_true', help='Send retrieved excerpts to your generation provider')
    args = parser.parse_args()
    try:
        asyncio.run(demo(args))
    except Exception:
        parser.exit(1, '演示未完成，请检查文件、服务访问令牌和显式选择的模型设置\n')


if __name__ == '__main__':
    main()
