import json
import unittest

import httpx

from pezmax_agent.contracts import ProviderConfig
from pezmax_agent.errors import AgentError
from pezmax_agent.provider import ChatCompletionsModel
from pezmax_agent.upload import build_upload_graph, parse_upload, run_upload_assistant, validate_suggestions


class UploadTests(unittest.IsolatedAsyncioTestCase):
    async def test_real_graph_only_sends_names_and_has_no_tools_or_catalog(self):
        captured, progress = [], []
        names = ["示例大学_2026_高等数学_期末.pdf"]
        async def handler(request):
            body = json.loads(request.content)
            captured.append(body)
            return httpx.Response(200, json={"choices": [{"finish_reason": "stop", "message": {
                "role": "assistant", "content": json.dumps({"suggestions": [
                    {"field": "fileName", "value": "2026 高等数学期末", "evidence": "高等数学"},
                    {"field": "fileSchool", "value": "示例大学", "evidence": "示例大学"},
                    {"field": "fileYear", "value": "2026", "evidence": "2026"},
                    {"field": "fileType", "value": 1, "evidence": "期末"}
                ]}, ensure_ascii=False)}}]})
        async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
            model = ChatCompletionsModel(client, ProviderConfig("https://model.example/v1", "fixture", "fixture-key"))
            value = {"kind": "exam", "folder": False, "names": names, "path": "C:/private", "token": "platform-secret", "form": {"fileSchool": "hidden"}}
            self.assertEqual(set(build_upload_graph(model, parse_upload(value), lambda _: None).nodes), {"__start__", "suggest", "validate"})
            result = await run_upload_assistant(model, value, on_progress=progress.append)
        self.assertEqual(len(result["suggestions"]), 4)
        self.assertEqual(len(progress), 2)
        self.assertEqual(len(captured), 1)
        self.assertNotIn("tools", captured[0])
        self.assertNotIn("tool_choice", captured[0])
        self.assertEqual(json.loads(captured[0]["messages"][1]["content"]), {"kind": "exam", "folder": False, "names": names})
        self.assertNotIn("platform-secret", json.dumps(captured))
        self.assertEqual(result["messages"], [])

    def test_invalid_inputs_and_limits(self):
        base = {"kind": "exam", "folder": False, "names": ["paper.pdf"]}
        for change in ({"names": ["C:\\private\\paper.pdf"]}, {"names": ["../file.pdf"]}, {"names": ["bad\n.pdf"]},
                       {"names": []}, {"names": ["a"] * 21, "folder": True}, {"names": ["x" * 241]},
                       {"kind": "other"}, {"folder": True, "kind": "ebook"}, {"folder": 1}):
            with self.subTest(change=change), self.assertRaises(AgentError):
                parse_upload({**base, **change})

    def test_unknown_fields_unsupported_evidence_duplicates_types_and_ebook_rules(self):
        upload = parse_upload({"kind": "ebook", "folder": False, "names": ["高等数学教材.pdf"]})
        items = [{"field": "fileSubject", "value": "高等数学", "evidence": "高等数学"},
                 {"field": "fileSubject", "value": "数学", "evidence": "数学"},
                 {"field": "fileSchool", "value": "某大学", "evidence": "数学"}]
        self.assertEqual(validate_suggestions({"content": json.dumps({"suggestions": items})}, upload), items[:1])
        for field, value, evidence in (("author", "编造作者", "数学"), ("ebookType", True, "教材"),
                                       ("ebookType", "1", "教材"), ("ebookType", 9, "教材"),
                                       ("fileName", "C:/private", "教材"), ("fileSubject", "数学", "不存在")):
            self.assertEqual(validate_suggestions({"content": json.dumps({"suggestions": [{"field": field, "value": value, "evidence": evidence}]})}, upload), [])

    def test_folder_requires_shared_evidence_and_unambiguous_years(self):
        upload = parse_upload({"kind": "exam", "folder": True, "names": ["2024高等数学期末.pdf", "2025高等数学期末.pdf"]})
        items = [{"field": "fileName", "value": "改名", "evidence": "期末"},
                 {"field": "fileYear", "value": "2024", "evidence": "20"},
                 {"field": "fileSchool", "value": "编造大学", "evidence": "期末"},
                 {"field": "fileSubject", "value": "高等数学", "evidence": "高等数学"},
                 {"field": "fileType", "value": 1, "evidence": "期末"}]
        self.assertEqual(validate_suggestions({"content": json.dumps({"suggestions": items})}, upload), items[-2:])
        upload["names"] = ["2024-2025高等数学.pdf"]
        self.assertEqual(validate_suggestions({"content": json.dumps({"suggestions": [items[1]]})}, upload), [])

    def test_bad_response_fails_instead_of_writing_anything(self):
        upload = parse_upload({"kind": "exam", "folder": False, "names": ["scan.pdf"]})
        for message in ({"content": "not JSON"}, {"content": "[]"}, {"content": '{"suggestions":null}'}, {"tool_calls": [{}]}):
            with self.assertRaises(AgentError):
                validate_suggestions(message, upload)
