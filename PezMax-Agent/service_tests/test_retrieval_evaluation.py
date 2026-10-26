import json
import tempfile
import unittest
from pathlib import Path

from pezmax_agent.retrieval_evaluation import evaluate


class RetrievalEvaluationTests(unittest.IsolatedAsyncioTestCase):
    async def test_document_level_recall_and_reciprocal_rank(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / 'synthetic.json'
            path.write_text(json.dumps({'version': 1, 'description': 'Isolated synthetic fixture', 'documents': [
                {'id': 'hit', 'title': 'Queue fixture', 'text': 'A bounded queue applies backpressure.'}],
                'queries': [{'id': 'hit', 'question': 'backpressure', 'relevant': ['hit']},
                            {'id': 'miss', 'question': 'semaphore', 'relevant': ['hit']}]}), encoding='utf-8')
            report = await evaluate(path)
            self.assertEqual(report['results']['keyword']['recallAt3'], .5)
            self.assertEqual(report['results']['keyword']['mrrAt3'], .5)
            self.assertIsNone(report['embeddingModel'])
            self.assertEqual(len(report['datasetSha256']), 64)
