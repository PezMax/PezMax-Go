import asyncio
import json
import unittest
from unittest.mock import patch

from langsmith import get_tracing_context

from pezmax_agent.contracts import bounded_history
from pezmax_agent.graph import build_graph, run_assistant


def answer(content="完成", calls=None):
    result = {"role": "assistant", "content": content}
    if calls:
        result["tool_calls"] = calls
    return result


def tool(identifier="c1", name="search_resources", arguments=None):
    return {"id": identifier, "type": "function", "function": {
        "name": name, "arguments": json.dumps(arguments or {"kinds": ["file"], "keyword": "数学"})}}


class Model:
    def __init__(self, responses):
        self.responses, self.calls = iter(responses), []

    async def complete(self, messages, *, allow_tools):
        self.calls.append((list(messages), allow_tools))
        return next(self.responses)


class Catalog:
    def __init__(self):
        self.calls = []

    async def search(self, arguments):
        self.calls.append(arguments)
        metadata = {"id": "file-1", "kind": "file", "title": "数学"}
        return {"resources": [{**metadata, "node": {"url": "https://private.example/file"}}],
                "modelResult": {"resources": [metadata], "reports": [{"kind": "file", "shown": 1}]}}


class GraphTests(unittest.IsolatedAsyncioTestCase):
    async def test_inherited_tracing_is_disabled_inside_all_graph_nodes(self):
        contexts = []

        class LocalModel:
            async def complete(self, *_args, **_kwargs):
                contexts.append(get_tracing_context()["enabled"])
                return answer("本地执行")

        with patch.dict("os.environ", {"LANGSMITH_TRACING": "true", "LANGCHAIN_TRACING_V2": "true"}):
            await run_assistant(LocalModel(), Catalog(), history=[], question="数学")
        self.assertEqual(contexts, [False])

    async def test_real_graph_search_progress_follow_up_and_private_projection(self):
        model, catalog, events = Model([answer("", [tool()]), answer("找到一份数学资料")]), Catalog(), []
        graph = build_graph(model, catalog)
        self.assertEqual(set(graph.get_graph().nodes), {"__start__", "model", "search", "finalize", "__end__"})
        result = await run_assistant(model, catalog, history=[], question="数学", on_progress=events.append)
        self.assertEqual(result["content"], "找到一份数学资料")
        self.assertEqual(result["resources"][0]["node"]["url"], "https://private.example/file")
        self.assertNotIn("private.example", json.dumps(model.calls))
        self.assertTrue(any(event.get("resources") for event in events))
        second = Model([answer("只看2023年")])
        await run_assistant(second, catalog, history=result["messages"], question="只看2023年")
        self.assertEqual([m["content"] for m in second.calls[0][0] if m["role"] == "user"], ["数学", "只看2023年"])

    async def test_model_and_search_budgets_cannot_be_overridden_by_tool_calls(self):
        responses = [answer("", [tool(f"c{round_number}-{i}") for i in range(4)]) for round_number in range(5)]
        model, catalog = Model(responses), Catalog()
        result = await run_assistant(model, catalog, history=[], question="数学")
        self.assertEqual(len(model.calls), 5)
        self.assertEqual(len(catalog.calls), 8)
        self.assertFalse(model.calls[-1][1])
        self.assertIn("上限", result["content"])
        self.assertEqual(len(result["resources"]), 1)
        # Validation also detects dangling / unmatched function responses.
        bounded_history(result["messages"])

    async def test_unknown_tools_and_invalid_json_never_execute(self):
        invalid = tool("invalid")
        invalid["function"]["arguments"] = "not-json-secret-data"
        model = Model([answer("", [tool("other", "download_file"), invalid]), answer("请补充条件")])
        catalog = Catalog()
        await run_assistant(model, catalog, history=[], question="数学")
        self.assertEqual(catalog.calls, [])
        tool_results = [m["content"] for m in model.calls[1][0] if m["role"] == "tool"]
        self.assertIn("不支持此工具", tool_results[0])
        self.assertNotIn("not-json-secret-data", "".join(tool_results))

    async def test_cancelling_graph_cancels_pending_network_work(self):
        entered, cancelled = asyncio.Event(), asyncio.Event()

        class SlowModel:
            async def complete(self, *_args, **_kwargs):
                entered.set()
                try:
                    await asyncio.Future()
                finally:
                    cancelled.set()

        task = asyncio.create_task(run_assistant(SlowModel(), Catalog(), history=[], question="数学"))
        await entered.wait()
        task.cancel()
        with self.assertRaises(asyncio.CancelledError):
            await task
        self.assertTrue(cancelled.is_set())


if __name__ == "__main__":
    unittest.main()
