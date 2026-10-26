import argparse
import asyncio
import json
import logging
import os
import re
import sys

# The desktop never opts users into hosted traces. Set before graph imports so
# inherited development-machine settings cannot export questions or results.
os.environ["LANGSMITH_TRACING"] = "false"
os.environ["LANGCHAIN_TRACING_V2"] = "false"
logging.disable(logging.CRITICAL)

import httpx

from . import __version__
from .application import run_task
from .contracts import MAX_REQUEST_BYTES, PROTOCOL_VERSION, CatalogConfig, ProviderConfig, json_text, redact
from .errors import AgentError
from .provider import ChatCompletionsModel


async def execute(payload, emit):
    provider = ProviderConfig.parse(payload.get("provider"))
    testing = payload.get("test") is True
    task = payload.get("task", "search")
    if task not in ("search", "upload", "reading"):
        raise AgentError("请求的助手任务无效")
    catalog = CatalogConfig.parse(payload.get("catalog")) if not testing and task == "search" else None
    secrets = [provider.api_key, catalog.token if catalog else ""]

    def progress(data):
        emit("progress", data=redact(data, secrets))

    async with httpx.AsyncClient(follow_redirects=False) as client:
        model = ChatCompletionsModel(client, provider)
        result = await run_task(model, client, payload, progress)
    emit("result", data=redact(result, secrets))


def main(argv=None):
    parser = argparse.ArgumentParser(description="Local PezMax LangGraph runtime (NDJSON over stdio)")
    parser.add_argument("--check", action="store_true", help="Check packaged runtime without credentials or network calls")
    args = parser.parse_args(argv)
    sys.stdout.reconfigure(encoding="utf-8")
    if args.check:
        print(json_text({"version": PROTOCOL_VERSION, "engine": "langgraph", "packageVersion": __version__, "ready": True}), flush=True)
        return 0
    request_id = ""

    def emit(kind, **fields):
        print(json_text({"version": PROTOCOL_VERSION, "requestId": request_id, "type": kind, **fields}), flush=True)

    async def bounded_execute(payload):
        async with asyncio.timeout(180):
            await execute(payload, emit)

    try:
        raw = sys.stdin.buffer.readline(MAX_REQUEST_BYTES + 1)
        if len(raw) > MAX_REQUEST_BYTES:
            raise AgentError("请求内容过长，请开始新对话")
        try:
            payload = json.loads(raw)
        except (ValueError, UnicodeError):
            raise AgentError("请求格式不正确") from None
        if not isinstance(payload, dict) or type(payload.get("version")) is not int or payload["version"] != PROTOCOL_VERSION:
            raise AgentError("助手通信版本不兼容，请更新客户端和智能体模块")
        identifier = payload.get("requestId")
        if not isinstance(identifier, str) or not re.fullmatch(r"[a-zA-Z0-9_-]{1,100}", identifier):
            raise AgentError("请求标识无效")
        request_id = identifier
        asyncio.run(bounded_execute(payload))
        return 0
    except AgentError as error:
        emit("error", error=str(error), code=error.code)
    except TimeoutError:
        emit("error", error="助手处理超时，请缩小范围后重试", code="timeout")
    except (Exception, KeyboardInterrupt):
        # Never serialize tracebacks, network exceptions, request payloads or
        # third-party error strings; they may contain bearer credentials.
        emit("error", error="助手模块执行失败，请检查设置后重试", code="agent_error")
    return 1
