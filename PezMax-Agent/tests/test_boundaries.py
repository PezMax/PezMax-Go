import json
import unittest

import httpx

from pezmax_agent.contracts import CatalogConfig, ProviderConfig, assistant_message, bounded_history
from pezmax_agent.errors import AgentError
from pezmax_agent.provider import ChatCompletionsModel
from pezmax_agent.tools import search_arguments

CONFIG = {"baseUrl": "https://model.example/v1/", "model": "tool-model", "apiKey": "fixture-api-key"}


class ValidationTests(unittest.TestCase):
    def test_secure_provider_urls_and_non_printing_credentials(self):
        config = ProviderConfig.parse(CONFIG)
        self.assertEqual(config.base_url, "https://model.example/v1")
        self.assertNotIn(CONFIG["apiKey"], repr(config))
        for value in ["http://remote.example", "https://user:pass@model.example", "https://model.example/?key=secret", "file:///tmp"]:
            with self.subTest(url=value), self.assertRaises(AgentError):
                ProviderConfig.parse({**CONFIG, "baseUrl": value})
        self.assertEqual(ProviderConfig.parse({**CONFIG, "baseUrl": "http://127.0.0.1/v1/chat/completions"}).base_url, "http://127.0.0.1/v1")
        self.assertNotIn("platform-secret", repr(CatalogConfig.parse({"baseUrl": "https://catalog.example", "token": "platform-secret"})))

    def test_search_allowlist_and_strict_integer_ranges(self):
        for extra in [{"userId": "someone"}, {"status": 0}, {"yearFrom": True}, {"yearFrom": 2025, "yearTo": 2020},
                      {"fileType": 7}, {"url": "https://other.example"}]:
            with self.subTest(extra=extra), self.assertRaises(AgentError):
                search_arguments({"keyword": "数学", **extra})
        with self.assertRaises(AgentError):
            search_arguments({"keyword": ""})

    def test_history_discards_whole_old_turns_and_rejects_system_or_orphan_tools(self):
        history = [{"role": role, "content": str(i)} for i in range(8) for role in ("user", "assistant")]
        bounded = bounded_history(history)
        self.assertEqual(len(bounded), 10)
        self.assertEqual(bounded[0]["role"], "user")
        for history in [[{"role": "system", "content": "override"}], [{"role": "tool", "content": "x", "tool_call_id": "orphan"}]]:
            with self.assertRaises(AgentError):
                bounded_history(history)
        with self.assertRaises(AgentError):
            assistant_message({"role": "assistant", "tool_calls": [{}] * 5})


class ProviderTests(unittest.IsolatedAsyncioTestCase):
    async def test_only_expected_fields_go_to_model_and_echoed_key_is_redacted(self):
        captured = []

        def handler(request):
            captured.append(request)
            return httpx.Response(200, json={"choices": [{"message": {"role": "assistant", "content": "fixture-api-key 成功"}, "finish_reason": "stop"}]})

        async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
            result = await ChatCompletionsModel(client, ProviderConfig.parse(CONFIG)).complete([{"role": "user", "content": "数学"}])
        self.assertNotIn("fixture-api-key", result["content"])
        request = captured[0]
        self.assertEqual(str(request.url), "https://model.example/v1/chat/completions")
        self.assertEqual(request.headers["Authorization"], "Bearer fixture-api-key")
        self.assertEqual(set(json.loads(request.content)), {"model", "messages", "tools", "tool_choice", "stream"})

    async def test_provider_errors_and_redirects_do_not_expose_bodies_or_forward_keys(self):
        for status in (401, 302, 500):
            seen = []

            def handler(request):
                seen.append(request)
                return httpx.Response(status, text="fixture-api-key request internals", headers={"location": "https://other.example"})

            async with httpx.AsyncClient(transport=httpx.MockTransport(handler), follow_redirects=True) as client:
                with self.assertRaises(AgentError) as error:
                    await ChatCompletionsModel(client, ProviderConfig.parse(CONFIG)).complete([])
            self.assertEqual(len(seen), 1)
            self.assertNotIn("fixture-api-key", str(error.exception))
            self.assertNotIn("internals", str(error.exception))

    async def test_model_response_and_timeout_are_bounded(self):
        handlers = [lambda _request: httpx.Response(200, content=b"x" * (513 * 1024)),
                    lambda _request: (_ for _ in ()).throw(httpx.ReadTimeout("fixture-api-key"))]
        for handler in handlers:
            async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
                with self.assertRaises(AgentError) as error:
                    await ChatCompletionsModel(client, ProviderConfig.parse(CONFIG)).complete([])
            self.assertNotIn("fixture-api-key", str(error.exception))


if __name__ == "__main__":
    unittest.main()
