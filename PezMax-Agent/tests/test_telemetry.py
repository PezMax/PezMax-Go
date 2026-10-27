import asyncio
import json
import unittest

from pezmax_agent.telemetry import RunMetrics


class TelemetryTests(unittest.IsolatedAsyncioTestCase):
    async def test_allowlist_records_timing_without_input_output_or_exception_text(self):
        metrics = RunMetrics("reading")
        async def node(state):
            return {"secret": state["secret"]}
        await metrics.wrap("answer", node)({"secret": "private-key-and-text"})
        def failure(_state):
            raise RuntimeError("private-exception")
        with self.assertRaises(RuntimeError):
            await metrics.wrap("validate", failure)({})
        result = metrics.snapshot(modelCalls=1)
        self.assertNotIn("private", json.dumps(result))
        self.assertEqual([step["ok"] for step in result["steps"]], [True, False])
        self.assertTrue(all(type(step["durationMs"]) is int and step["durationMs"] >= 0 for step in result["steps"]))
        for counters in ({"question": 1}, {"modelCalls": "private"}, {"modelCalls": True}, {"modelCalls": -1}):
            with self.assertRaises(ValueError):
                metrics.snapshot(**counters)
        with self.assertRaises(ValueError):
            metrics.wrap("private-key", node)

    async def test_cancellation_propagates_and_only_records_failure_flag(self):
        metrics = RunMetrics("reading")
        async def cancelled(_state):
            raise asyncio.CancelledError()
        with self.assertRaises(asyncio.CancelledError):
            await metrics.wrap("answer", cancelled)({})
        self.assertFalse(metrics.snapshot()["steps"][0]["ok"])
