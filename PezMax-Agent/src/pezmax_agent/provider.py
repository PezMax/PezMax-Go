import httpx

from .contracts import ProviderConfig, assistant_message, redact
from .errors import AgentError
from .http import read_json
from .prompts import system_prompt
from .tools import SEARCH_TOOL


def provider_status(status):
    labels = {
        400: "请求被模型服务拒绝，请确认模型支持工具调用和 Chat Completions",
        401: "API Key 无效，请检查模型设置",
        403: "API Key 没有访问该模型的权限",
        404: "模型或接口路径不存在，请检查接口地址和模型 ID",
        429: "模型服务额度不足或请求过于频繁，请稍后重试",
    }
    return AgentError(labels.get(status, f"模型服务暂不可用（HTTP {status}），请稍后重试"))


class ChatCompletionsModel:
    """Provider adapter; graph and catalog never depend on a vendor SDK."""

    def __init__(self, client: httpx.AsyncClient, config: ProviderConfig):
        self.client, self.config = client, config

    async def complete(self, messages: list[dict], *, allow_tools=True) -> dict:
        config = self.config
        try:
            data = await read_json(
                self.client, "POST", f"{config.base_url}/chat/completions",
                headers={"Authorization": f"Bearer {config.api_key}"},
                json={"model": config.model, "messages": [{"role": "system", "content": system_prompt()}, *messages],
                      "tools": [SEARCH_TOOL], "tool_choice": "auto" if allow_tools else "none", "stream": False},
                timeout=60, limit=512 * 1024, status_error=provider_status,
            )
        except httpx.TimeoutException:
            raise AgentError("模型响应超时，请稍后重试") from None
        except httpx.HTTPError:
            raise AgentError("无法连接模型服务，请检查接口地址、网络或系统代理") from None
        choices = data.get("choices")
        choice = choices[0] if isinstance(choices, list) and choices else None
        if not isinstance(choice, dict) or choice.get("finish_reason") in ("length", "content_filter"):
            raise AgentError("模型未返回完整结果，请简化需求后重试")
        # Neither provider exception text nor arbitrary response fields escape.
        return redact(assistant_message(choice.get("message")), [config.api_key])
