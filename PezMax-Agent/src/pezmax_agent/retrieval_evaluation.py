"""Reproducible retrieval baseline; optional real BYOK embedding comparison."""
import argparse
import asyncio
import hashlib
import json
import tempfile
from pathlib import Path

import httpx

from .contracts import json_text
from .embeddings import Embeddings
from .knowledge import KnowledgeService
from .knowledge_cli import connection
from .knowledge_schema import SearchInput
from .knowledge_store import KnowledgeStore


async def evaluate(dataset_path, *, embedding_config=None, transport=None):
    raw = Path(dataset_path).read_text(encoding='utf-8').replace('\r\n', '\n')
    dataset = json.loads(raw)
    modes = ['keyword', 'hybrid'] if embedding_config else ['keyword']
    with tempfile.TemporaryDirectory(prefix='pezmax-retrieval-eval-') as temp:
        store = KnowledgeStore(Path(temp) / 'synthetic.sqlite3')
        async with httpx.AsyncClient(follow_redirects=False, trust_env=False, transport=transport) as client:
            encoder = Embeddings(client, embedding_config) if embedding_config else None
            service = KnowledgeService(store, 'synthetic-evaluation', encoder)
            ids = {}
            for doc in dataset['documents']:
                result = await service.ingest({'title': doc['title'], 'format': 'text',
                    'sections': [{'location': 1, 'text': doc['text']}]})
                ids[doc['id']] = result['documentId']
            report = {'datasetVersion': dataset['version'], 'datasetSha256': hashlib.sha256(raw.encode()).hexdigest(),
                      'scope': dataset['description'], 'embeddingModel': embedding_config.model if embedding_config else None,
                      'embeddingProfile': encoder.profile if encoder else None, 'k': 3, 'results': {}}
            for mode in modes:
                cases = []
                for case in dataset['queries']:
                    result = await service.search(SearchInput(question=case['question'], mode=mode, limit=3))
                    # Metrics operate at document level; duplicate chunks cannot inflate recall.
                    found = list(dict.fromkeys(c['documentId'] for c in result['chunks']))
                    relevant = {ids[key] for key in case['relevant']}
                    ranks = [i for i, key in enumerate(found, 1) if key in relevant]
                    cases.append({'id': case['id'], 'recallAt3': len(set(found) & relevant) / len(relevant),
                                  'reciprocalRankAt3': 1 / ranks[0] if ranks else 0, 'retrievedDocuments': found})
                report['results'][mode] = {'queries': len(cases),
                    'recallAt3': round(sum(c['recallAt3'] for c in cases) / len(cases), 4),
                    'mrrAt3': round(sum(c['reciprocalRankAt3'] for c in cases) / len(cases), 4), 'cases': cases}
    return report


def main(argv=None):
    parser = argparse.ArgumentParser(description='Synthetic retrieval baseline; --embedding incurs BYOK embedding calls')
    parser.add_argument('--dataset', type=Path, default=Path(__file__).resolve().parents[2] / 'evals' / 'retrieval-cases.json')
    parser.add_argument('--output', type=Path)
    parser.add_argument('--embedding', action='store_true', help='Send synthetic corpus/questions to the configured embedding provider')
    args = parser.parse_args(argv)
    try:
        async def bounded():
            async with asyncio.timeout(180):
                return await evaluate(args.dataset, embedding_config=connection('PEZMAX_AGENT_EMBEDDING') if args.embedding else None)
        report = asyncio.run(bounded())
    except Exception:
        parser.exit(1, '检索评测失败，请检查数据集和可选的 embedding 设置\n')
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    print(json_text({k: v for k, v in report.items() if k != 'results'} | {
        'results': {mode: {k: v for k, v in result.items() if k != 'cases'} for mode, result in report['results'].items()}}))


if __name__ == '__main__':
    main()
