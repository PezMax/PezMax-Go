import json
import re
from dataclasses import dataclass, field
from urllib.parse import urlsplit, urlunsplit

from .errors import AgentError

PROTOCOL_VERSION = 1
MAX_REQUEST_BYTES = 512 * 1024


def json_text(value) -> str:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"), allow_nan=False)


def base_url(value, *, provider=False) -> str:
    if not isinstance(value, str) or len(value) > 2048:
        raise AgentError("接口地址格式不正确")
    try:
        url = urlsplit(value.strip())
        local = url.hostname in ("localhost", "127.0.0.1", "::1")
        if not url.hostname or url.username or url.password or url.query or url.fragment:
            raise ValueError()
        if url.scheme not in ("http", "https") or (provider and url.scheme == "http" and not local):
            raise ValueError()
        _ = url.port
    except ValueError:
        raise AgentError("接口地址无效，模型服务需使用 HTTPS，本机服务可使用 HTTP") from None
    path = url.path.rstrip("/")
    if provider:
        path = re.sub(r"/chat/completions$", "", path)
    return urlunsplit((url.scheme, url.netloc, path, "", ""))


def secret(value, label) -> str:
    if not isinstance(value, str) or not value.strip() or len(value) > 8192 or re.search(r"[\r\n\x00]", value):
        raise AgentError(f"{label}格式不正确")
    return value.strip()


@dataclass(frozen=True)
class ProviderConfig:
    base_url: str
    model: str
    api_key: str = field(repr=False)

    @classmethod
    def parse(cls, value):
        if not isinstance(value, dict):
            raise AgentError("请填写模型设置")
        model = value.get("model")
        if not isinstance(model, str) or not model.strip() or len(model) > 160 or re.search(r"[\r\n]", model):
            raise AgentError("请填写有效的模型 ID")
        return cls(base_url(value.get("baseUrl"), provider=True), model.strip(), secret(value.get("apiKey"), "API Key"))


@dataclass(frozen=True)
class CatalogConfig:
    base_url: str
    token: str = field(repr=False)

    @classmethod
    def parse(cls, value):
        if not isinstance(value, dict):
            raise AgentError("请先登录再使用找资料助手")
        return cls(base_url(value.get("baseUrl")), secret(value.get("token"), "平台登录令牌"))


def assistant_message(value) -> dict:
    if not isinstance(value, dict) or value.get("role") != "assistant":
        raise AgentError("模型返回格式不正确，请重试")
    content = value.get("content") or ""
    if not isinstance(content, str):
        raise AgentError("模型文字格式不正确")
    message = {"role": "assistant", "content": content}
    calls = value.get("tool_calls")
    if calls:
        if not isinstance(calls, list) or len(calls) > 4:
            raise AgentError("模型请求的搜索次数过多")
        normalized, ids = [], set()
        for call in calls:
            function = call.get("function") if isinstance(call, dict) else None
            call_id = call.get("id") if isinstance(call, dict) else None
            if (not isinstance(call_id, str) or not 1 <= len(call_id) <= 160 or call_id in ids
                    or not isinstance(function, dict) or not isinstance(function.get("name"), str)
                    or len(function["name"]) > 80 or not isinstance(function.get("arguments"), str)
                    or len(function["arguments"]) > 4000):
                raise AgentError("模型工具调用格式不正确")
            ids.add(call_id)
            normalized.append({"id": call_id, "type": "function", "function": {
                "name": function["name"], "arguments": function["arguments"]}})
        message["tool_calls"] = normalized
    return message


def bounded_history(value) -> list[dict]:
    if not isinstance(value, list) or len(value) > 120 or len(json_text(value)) > 120000:
        raise AgentError("对话过长，请开始新对话")
    messages, pending = [], set()
    for item in value:
        if not isinstance(item, dict) or item.get("role") not in ("user", "assistant", "tool"):
            raise AgentError("对话格式不正确")
        role = item["role"]
        if role != "tool" and pending:
            raise AgentError("对话工具结果不完整，请开始新对话")
        if role == "assistant":
            message = assistant_message(item)
            pending = {call["id"] for call in message.get("tool_calls", [])}
        else:
            if not isinstance(item.get("content"), str):
                raise AgentError("对话内容格式不正确")
            message = {"role": role, "content": item["content"]}
            if role == "tool":
                call_id = item.get("tool_call_id")
                if not isinstance(call_id, str) or call_id not in pending:
                    raise AgentError("对话工具结果格式不正确")
                pending.remove(call_id)
                message["tool_call_id"] = call_id
        messages.append(message)
    if pending or (messages and (messages[0]["role"] != "user" or messages[-1]["role"] != "assistant")):
        raise AgentError("对话不完整，请开始新对话")
    while messages and (len(messages) > 10 or len(json_text(messages)) > 20000):
        next_user = next((i for i, item in enumerate(messages) if i and item["role"] == "user"), None)
        messages = messages[next_user:] if next_user is not None else []
    return messages


def redact(value, secrets):
    """Scrub even a provider that echoes a credential in a nominally successful reply."""
    if isinstance(value, str):
        for key in secrets:
            if key:
                value = value.replace(key, "[已隐藏密钥]")
        return value
    if isinstance(value, list):
        return [redact(item, secrets) for item in value]
    if isinstance(value, dict):
        return {key: redact(item, secrets) for key, item in value.items()}
    return value
