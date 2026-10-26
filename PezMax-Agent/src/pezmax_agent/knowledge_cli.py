"""Explicit local corpus operations, separate from model-invoked MCP tools."""
import argparse
import asyncio
import os
import sys
from pathlib import Path

import httpx

from .contracts import ProviderConfig, json_text
from .document_import import MAX_FILE_BYTES, extract_document
from .embeddings import Embeddings
from .errors import AgentError
from .knowledge import KnowledgeService, run_rag
from .knowledge_schema import SearchInput
from .knowledge_store import KnowledgeStore
from .provider import ChatCompletionsModel


def connection(prefix):
    return ProviderConfig.parse({'baseUrl': os.environ.get(prefix + '_BASE_URL'),
        'model': os.environ.get(prefix + '_MODEL'), 'apiKey': os.environ.get(prefix + '_KEY')})


async def execute(args):
    store = KnowledgeStore(args.database)
    async with httpx.AsyncClient(follow_redirects=False, trust_env=False) as client:
        encoder = Embeddings(client, connection('PEZMAX_AGENT_EMBEDDING')) if args.embedding else None
        service = KnowledgeService(store, args.owner, encoder)
        if args.action == 'import':
            path = Path(args.path)
            with path.open('rb') as stream:
                data = stream.read(MAX_FILE_BYTES + 1)
            file_format = args.format or {'.pdf': 'pdf', '.epub': 'epub', '.txt': 'text', '.md': 'text'}.get(path.suffix.lower())
            return await service.ingest(extract_document(data, args.title or path.stem, file_format))
        if args.action == 'list':
            return {'documents': store.list_documents(args.owner)}
        if args.action == 'delete':
            return {'deleted': store.delete(args.owner, args.document_id)}
        query = SearchInput(question=args.question, documentIds=args.document_ids,
                            mode='hybrid' if args.embedding else 'keyword')
        if args.action == 'ask':
            return await run_rag(ChatCompletionsModel(client, connection('PEZMAX_AGENT_GENERATION')), service, query)
        return await service.search(query)


def main(argv=None):
    sys.stdout.reconfigure(encoding='utf-8')
    parser = argparse.ArgumentParser(description='Explicit local knowledge import, search and cited QA')
    parser.add_argument('--database', type=Path, default=Path(os.environ.get('PEZMAX_AGENT_DB', 'data/knowledge.sqlite3')))
    parser.add_argument('--owner', required=True, help='Trusted local identity; must match HTTP/MCP identity')
    parser.add_argument('--embedding', action='store_true', help='Explicitly send chunks/question to configured BYOK embedding service')
    actions = parser.add_subparsers(dest='action', required=True)
    load = actions.add_parser('import')
    load.add_argument('path')
    load.add_argument('--format', choices=['pdf', 'epub', 'text'])
    load.add_argument('--title')
    actions.add_parser('list')
    delete = actions.add_parser('delete')
    delete.add_argument('document_id')
    for name in ('search', 'ask'):
        command = actions.add_parser(name)
        command.add_argument('question')
        command.add_argument('--document-ids', nargs='*', default=[])
    args = parser.parse_args(argv)
    import re
    if not re.fullmatch(r'[A-Za-z0-9_-]{1,64}', args.owner):
        parser.error('知识库身份格式无效')
    try:
        async def bounded():
            async with asyncio.timeout(180):
                return await execute(args)
        print(json_text(asyncio.run(bounded())))
    except AgentError as error:
        parser.exit(1, str(error) + '\n')
    except Exception:
        parser.exit(1, '知识库操作失败，请检查文件、参数和模型设置\n')


if __name__ == '__main__':
    main()
