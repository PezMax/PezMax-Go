import json
import unittest
from unittest.mock import AsyncMock

import httpx
from langsmith import get_tracing_context

from pezmax_agent.contracts import ProviderConfig
from pezmax_agent.errors import AgentError
from pezmax_agent.provider import ChatCompletionsModel
from pezmax_agent.reading import READING_MODES, build_reading_graph, parse_reading, run_reading_assistant, select_context, validate_answer


class ReadingTests(unittest.IsolatedAsyncioTestCase):
    def setUp(self):
        self.reading = {"question": "What does the derivative measure?", "pages": [
            {"page": 2, "text": "The derivative measures the rate of change."}]}
        self.answer = {"text": "It measures the rate of change.", "citations": [
            {"page": 2, "quote": "The derivative measures the rate of change."}]}

    async def test_graph_only_sends_selected_excerpts_and_question(self):
        captured, progress = [], []

        async def handler(request):
            captured.append(json.loads(request.content))
            self.assertFalse(get_tracing_context()["enabled"])
            return httpx.Response(200, json={"choices": [{"finish_reason": "stop", "message": {
                "role": "assistant", "content": json.dumps({"answers": [self.answer]})}}]})

        async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
            model = ChatCompletionsModel(client, ProviderConfig("https://model.example/v1", "fixture", "fixture-key"))
            self.assertEqual(set(build_reading_graph(model, self.reading, lambda _: None).nodes),
                             {"__start__", "retrieve", "answer", "validate", "repair"})
            result = await run_reading_assistant(model, {**self.reading, "url": "private-url", "token": "private-token"}, progress.append)
        self.assertEqual(result["answers"], [self.answer])
        self.assertEqual(result["contextPages"], [2])
        self.assertFalse(result["partial"])
        self.assertEqual(len(progress), 3)
        self.assertEqual(len(captured), 1)
        body = captured[0]
        self.assertNotIn("tools", body)
        self.assertNotIn("tool_choice", body)
        self.assertEqual(json.loads(body["messages"][1]["content"]), {
            "question": self.reading["question"], "excerpts": self.reading["pages"]})
        self.assertNotIn("private-", json.dumps(body))
        self.assertEqual(result["messages"], [])
        self.assertEqual(result["diagnostics"]["modelCalls"], 1)
        self.assertEqual(result["diagnostics"]["verifiedCitations"], 1)

    def test_invalid_input_and_limits(self):
        for pages in ([], [{"page": True, "text": "test"}], [{"page": 0, "text": "test"}],
                      [{"page": 10001, "text": "test"}], [{"page": 1, "text": "x" * 20001}],
                      [{"page": 1, "text": "test"}] * 2,
                      [{"page": i, "text": "test"} for i in range(1, 12)],
                      [{"page": i, "text": "x" * 20000} for i in range(1, 6)]):
            with self.subTest(pages=str(pages)[:80]), self.assertRaises(AgentError):
                parse_reading({**self.reading, "pages": pages})
        for question in (None, " ", "x" * 2001):
            with self.assertRaises(AgentError):
                parse_reading({**self.reading, "question": question})

    def test_exact_physical_page_quotations_required_for_whole_answer(self):
        chunks = self.reading["pages"]
        for citations in ([], [{"page": 1, "quote": "rate of change"}],
                          [{"page": 2, "quote": "fabricated evidence"}],
                          [{"page": 2, "quote": "The"}],
                          [self.answer["citations"][0], {"page": 9, "quote": "rate of change"}]):
            message = {"content": json.dumps({"answers": [{**self.answer, "citations": citations}]})}
            self.assertEqual(validate_answer(message, chunks), [])
        answer = {**self.answer, "citations": [{"page": 2, "quote": "rate\n of  change"}]}
        self.assertEqual(validate_answer({"content": json.dumps({"answers": [answer]})}, chunks)[0]["citations"][0]["quote"], "rate of change")

    def test_malformed_responses_fail_safely(self):
        for message in ({"content": None}, {"content": "[]"}, {"content": "{}"},
                        {"content": "not json"}, {"tool_calls": [{}]}, {"content": '{"answers": null}'}):
            with self.assertRaises(AgentError):
                validate_answer(message, self.reading["pages"])

    async def test_no_text_does_not_call_model_and_no_evidence_is_explicit(self):
        model = AsyncMock()
        result = await run_reading_assistant(model, {**self.reading, "pages": [{"page": 1, "text": " "}]})
        model.complete.assert_not_called()
        self.assertEqual(result["answers"], [])
        self.assertIn("没有足够", result["content"])
        self.assertEqual(result["diagnostics"]["modelCalls"], 0)
        model.complete.return_value = {"content": json.dumps({"answers": [{**self.answer, "citations": [{"page": 9, "quote": "nonexistent"}]}]})}
        result = await run_reading_assistant(model, self.reading)
        self.assertEqual(result["answers"], [])
        self.assertEqual(result["diagnostics"]["modelCalls"], 2)

    async def test_bounded_repair_uses_same_evidence_without_replaying_invalid_content(self):
        valid = {"content": json.dumps({"answers": [self.answer]})}
        for invalid in ({"content": "private-invalid-response"}, {"content": json.dumps({"answers": [
                {**self.answer, "citations": [{"page": 9, "quote": "private-invalid-response"}]}]})}):
            model = AsyncMock()
            model.complete.side_effect = [invalid, valid]
            result = await run_reading_assistant(model, self.reading)
            self.assertEqual(result["answers"], [self.answer])
            self.assertEqual(model.complete.call_count, 2)
            first, second = model.complete.call_args_list
            self.assertEqual(first.args, second.args)
            self.assertNotIn("private-invalid-response", json.dumps(second.kwargs))
            self.assertIn("上次输出未通过", second.kwargs["system"])
            self.assertEqual([step["node"] for step in result["diagnostics"]["steps"]], ["retrieve", "answer", "validate", "repair", "validate"])
            self.assertNotIn("rate of change", json.dumps(result["diagnostics"]))

    async def test_explicit_abstention_and_partial_valid_results_do_not_retry(self):
        for answers, expected in (([], []), ([self.answer, {**self.answer, "citations": [{"page": 9, "quote": "fabricated"}]}], [self.answer])):
            model = AsyncMock()
            model.complete.return_value = {"content": json.dumps({"answers": answers})}
            result = await run_reading_assistant(model, self.reading)
            self.assertEqual(result["answers"], expected)
            self.assertEqual(model.complete.call_count, 1)

    def test_strict_schema_rejects_coercion_extra_fields_and_preserves_valid_siblings(self):
        for invalid in ({**self.answer, "url": "synthetic-untrusted"},
                        {**self.answer, "citations": [{"page": "2", "quote": "rate of change"}]},
                        {**self.answer, "citations": [{"page": True, "quote": "rate of change"}]},
                        {**self.answer, "citations": [{"page": 2, "quote": "rate of change", "href": "untrusted"}]}):
            result = validate_answer({"content": json.dumps({"answers": [invalid, self.answer]})}, self.reading["pages"])
            self.assertEqual(result, [self.answer])

    def test_local_ranking_bounds_context_and_only_validates_supplied_chunks(self):
        value = parse_reading({"question": "needle", "pages": [
            {"page": page, "text": "ordinary " * 900 + (" needle evidence" if page == 9 else "")} for page in range(1, 10)]})
        chunks, partial = select_context(value)
        self.assertTrue(partial)
        self.assertLessEqual(len(chunks), 6)
        self.assertTrue(all(len(chunk["text"]) <= 1800 for chunk in chunks))
        self.assertTrue(any(chunk["page"] == 9 and "needle" in chunk["text"] for chunk in chunks))
        self.assertEqual(validate_answer({"content": json.dumps({"answers": [{"text": "unsupported", "citations": [{"page": 8, "quote": "ordinary"}]}]})}, chunks), [])

    async def test_summary_and_outline_for_both_formats_use_fixed_tasks_and_valid_evidence(self):
        for file_format in ("pdf", "epub"):
            key = "chapter" if file_format == "epub" else "page"
            items_key = "chapters" if file_format == "epub" else "pages"
            for mode in ("summary", "outline"):
                model = AsyncMock()
                answer = {"text": "The rate of change.", "citations": [{key: 2, "quote": "rate of change"}]}
                model.complete.return_value = {"content": json.dumps({"answers": [answer]})}
                value = {"format": file_format, "mode": mode, "question": "private-override", items_key: [{key: 2, "text": self.reading["pages"][0]["text"], "href": "private-path"}]}
                result = await run_reading_assistant(model, value)
                self.assertEqual(result["answers"], [answer])
                self.assertEqual(result["contextChapters" if file_format == "epub" else "contextPages"], [2])
                args, kwargs = model.complete.call_args
                user = json.loads(args[0][0]["content"])
                self.assertEqual(user["question"], READING_MODES[mode])
                self.assertNotIn("private-", json.dumps(user))
                self.assertFalse(kwargs["allow_tools"])
                self.assertEqual(kwargs["tools"], [])
                if file_format == "epub":
                    self.assertIn('"chapter":1', kwargs["system"])
                    self.assertNotIn('"page":1', kwargs["system"])

    def test_epub_citations_never_accept_pdf_pages_and_chapter_limits_are_separate(self):
        chunks = [{"chapter": 2, "text": self.reading["pages"][0]["text"]}]
        self.assertEqual(validate_answer({"content": json.dumps({"answers": [self.answer]})}, chunks, "chapter"), [])
        for change in ({"mode": []}, {"mode": "review"}, {"format": "other"},
                       {"chapters": [{"chapter": True, "text": "test"}]},
                       {"chapters": [{"chapter": 1, "text": "x" * 40001}]},
                       {"chapters": [{"chapter": i, "text": "test"} for i in range(1, 7)]}):
            with self.subTest(change=str(change)[:80]), self.assertRaises(AgentError):
                parse_reading({"format": "epub", "mode": "summary", "chapters": chunks, **change})

    def test_overview_samples_start_and_end_and_discloses_truncation(self):
        value = parse_reading({"mode": "outline", "pages": [{"page": 1, "text": "start-marker " + "middle " * 2000 + " tail-marker"}]})
        chunks, partial = select_context(value)
        self.assertEqual(len(chunks), 6)
        self.assertTrue(partial)
        self.assertIn("start-marker", chunks[0]["text"])
        self.assertIn("tail-marker", chunks[-1]["text"])
