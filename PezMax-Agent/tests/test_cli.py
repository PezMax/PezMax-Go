import json
import os
import subprocess
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def invoke(payload=None, *args):
    env = {**os.environ, "HTTP_PROXY": "", "HTTPS_PROXY": "", "ALL_PROXY": "", "NO_PROXY": "127.0.0.1"}
    result = subprocess.run([sys.executable, "-m", "pezmax_agent", *args],
                            input=None if payload is None else json.dumps(payload, ensure_ascii=False) + "\n",
                            encoding="utf-8", capture_output=True, timeout=30, env=env)
    return result, [json.loads(line) for line in result.stdout.splitlines()]


class CliTests(unittest.TestCase):
    def test_offline_startup_check(self):
        result, events = invoke(None, "--check")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(events[0]["engine"], "langgraph")
        self.assertTrue(events[0]["ready"])

    def test_bad_requests_and_oversized_input_never_echo_secrets(self):
        for payload in ({"version": 5, "apiKey": "fixture-secret"}, {"version": 1, "requestId": "bad\nsecret"},
                        {"version": 1, "requestId": "long", "question": "fixture-secret" * 50000}):
            result, events = invoke(payload)
            self.assertEqual(result.returncode, 1)
            self.assertEqual(events[-1]["type"], "error")
            self.assertNotIn("fixture-secret", result.stdout + result.stderr)

    def test_real_stdio_http_model_tool_model_round_trip(self):
        captured = []

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_args):
                pass

            def send_json(self, body):
                data = json.dumps(body, ensure_ascii=False).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                captured.append(("model", self.headers["Authorization"], body))
                message = {"role": "assistant", "content": "找到了 fixture-api-key"}
                if body["messages"][-1]["role"] == "user":
                    message = {"role": "assistant", "content": "", "tool_calls": [{"id": "c1", "type": "function", "function": {
                        "name": "search_resources", "arguments": '{"kinds":["file"],"keyword":"数学"}'}}]}
                self.send_json({"choices": [{"message": message, "finish_reason": "stop"}]})

            def do_GET(self):
                captured.append(("catalog", self.headers["Authorization"], self.path))
                self.send_json({"code": 0, "rows": [{"fileId": 17, "fileStatus": 1, "fileName": "数学试卷", "fileUrl": "/private/math.pdf"}], "total": 1})

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            address = f"http://127.0.0.1:{server.server_port}"
            result, events = invoke({"version": 1, "requestId": "round-trip", "provider": {
                "baseUrl": address + "/v1", "model": "fixture", "apiKey": "fixture-api-key"},
                "catalog": {"baseUrl": address, "token": "platform-secret"}, "history": [], "question": "数学"})
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(events[-1]["type"], "result")
            self.assertEqual(events[-1]["data"]["resources"][0]["node"]["rawId"], "17")
            self.assertEqual([call[0] for call in captured], ["model", "catalog", "model"])
            model_requests = [call for call in captured if call[0] == "model"]
            self.assertNotIn("platform-secret", json.dumps(model_requests))
            self.assertNotIn("/private/", json.dumps(model_requests))
            self.assertNotIn("fixture-api-key", result.stdout + result.stderr)
            self.assertNotIn("platform-secret", result.stdout + result.stderr)
            self.assertTrue(any(event["type"] == "progress" and event["data"].get("resources") for event in events))
        finally:
            server.shutdown()
            server.server_close()
            thread.join()


if __name__ == "__main__":
    unittest.main()
