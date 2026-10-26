"""Application tasks shared by stdio and HTTP, independent of transports."""
from .catalog import CatalogSearch
from .contracts import CatalogConfig
from .errors import AgentError
from .graph import run_assistant
from .reading import run_reading_assistant
from .upload import run_upload_assistant


async def run_task(model, client, payload, on_progress=lambda _: None):
    task = payload.get('task', 'search')
    if task not in ('search', 'upload', 'reading'):
        raise AgentError('请求的助手任务无效')
    if payload.get('test') is True:
        message = await model.complete([{'role': 'user', 'content': '请简短回复“连接成功”，不要调用工具。'}], allow_tools=False)
        return {'content': message['content'], 'messages': [], 'resources': [], 'reports': []}
    if task == 'upload':
        return await run_upload_assistant(model, payload.get('upload'), on_progress=on_progress)
    if task == 'reading':
        return await run_reading_assistant(model, payload.get('reading'), on_progress=on_progress)
    catalog = CatalogConfig.parse(payload.get('catalog'))
    return await run_assistant(model, CatalogSearch(client, catalog), history=payload.get('history', []),
                               question=payload.get('question'), on_progress=on_progress)
