import json

import httpx

from .errors import AgentError


async def read_json(client: httpx.AsyncClient, method, url, *, limit, status_error, **kwargs):
    # Streaming enforces the decoded byte cap even with a false Content-Length
    # or compressed responses. Redirects must never forward a bearer credential.
    async with client.stream(method, url, follow_redirects=False, **kwargs) as response:
        if response.status_code != 200:
            raise status_error(response.status_code)
        data = bytearray()
        async for chunk in response.aiter_bytes():
            data.extend(chunk)
            if len(data) > limit:
                raise AgentError("接口返回内容过大，请缩小搜索范围")
        try:
            result = json.loads(data)
        except (ValueError, UnicodeError):
            raise AgentError("接口返回格式不正确") from None
        if not isinstance(result, dict):
            raise AgentError("接口返回格式不正确")
        return result
