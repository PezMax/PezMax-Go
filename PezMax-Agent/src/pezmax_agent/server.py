"""Optional authenticated FastAPI adapter. No port is opened by desktop stdio."""
import argparse
import asyncio
import os
from contextlib import asynccontextmanager, suppress
from typing import Annotated, Any, Literal

os.environ['LANGSMITH_TRACING'] = 'false'
os.environ['LANGCHAIN_TRACING_V2'] = 'false'
os.environ['OTEL_SDK_DISABLED'] = 'true'

import httpx
from fastapi import Depends, FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from fastapi.security import HTTPBearer
from pydantic import Field, SecretStr

from . import __version__
from .application import run_task
from .contracts import redact
from .embeddings import Embeddings
from .errors import AgentError
from .knowledge import KnowledgeService, run_rag
from .knowledge_schema import DocumentInput, DocumentList, Identifier, IndexResult, KnowledgeChunk, KnowledgeOutput, SearchInput, SearchResult
from .knowledge_store import KnowledgeStore
from .provider import ChatCompletionsModel
from .reading_schema import StrictOutput
from .service_config import ServiceSettings


class Connection(StrictOutput):
    baseUrl: Annotated[str, Field(min_length=1, max_length=2048)]
    model: Annotated[str, Field(min_length=1, max_length=160)]
    apiKey: SecretStr = Field(repr=False)

    def config(self, settings):
        return settings.provider({'baseUrl': self.baseUrl, 'model': self.model, 'apiKey': self.apiKey.get_secret_value()})


class ImportRequest(StrictOutput):
    document: DocumentInput
    embedding: Connection | None = None


class SearchRequest(StrictOutput):
    query: SearchInput
    embedding: Connection | None = None


class AskRequest(SearchRequest):
    provider: Connection


class AskResult(KnowledgeOutput):
    content: Annotated[str, Field(max_length=8200)]
    retrieval: SearchResult
    diagnostics: dict[str, Any]


class RunRequest(StrictOutput):
    provider: Connection
    task: Literal['search', 'upload', 'reading'] = 'search'
    test: bool = False
    question: Annotated[str, Field(max_length=2000)] = ''
    history: Annotated[list[dict[str, Any]], Field(max_length=100)] = Field(default_factory=list)
    upload: dict[str, Any] | None = None
    reading: dict[str, Any] | None = None
    catalogToken: SecretStr | None = Field(default=None, repr=False)


class ServiceGuard:
    """Authenticate before parsing, bound even chunked bodies and active requests."""
    def __init__(self, app, settings):
        self.app, self.settings, self.active = app, settings, 0

    async def __call__(self, scope, receive, send):
        if scope['type'] != 'http' or not scope['path'].startswith('/v1/'):
            return await self.app(scope, receive, send)

        async def reject(status, message):
            await JSONResponse({'error': message}, status_code=status)(scope, receive, send)

        headers = dict(scope.get('headers', []))
        owner = self.settings.authenticate(headers.get(b'authorization', b'').decode('latin-1'))
        if owner is None:
            return await reject(401, '请提供有效的服务访问令牌')
        scope.setdefault('state', {})['owner'] = owner
        if self.active >= self.settings.concurrency:
            return await reject(429, '服务繁忙，请稍后重试')
        self.active += 1
        started = False

        async def guarded_send(message):
            nonlocal started
            if message['type'] == 'http.response.start':
                started = True
            await send(message)

        try:
            body = bytearray()
            async with asyncio.timeout(10):
                while True:
                    message = await receive()
                    if message['type'] == 'http.disconnect':
                        return
                    incoming = message.get('body', b'')
                    if len(body) + len(incoming) > self.settings.body_limit:
                        return await reject(413, '请求内容超出8 MiB限制')
                    body.extend(incoming)
                    if not message.get('more_body', False):
                        break
            delivered = False
            disconnected = asyncio.Event()

            async def replay():
                nonlocal delivered
                if not delivered:
                    delivered = True
                    return {'type': 'http.request', 'body': bytes(body), 'more_body': False}
                await disconnected.wait()
                return {'type': 'http.disconnect'}

            async def watch_disconnect():
                while True:
                    message = await receive()
                    if message['type'] == 'http.disconnect':
                        disconnected.set()
                        return

            operation = asyncio.create_task(self.app(scope, replay, guarded_send))
            watcher = asyncio.create_task(watch_disconnect())
            try:
                async with asyncio.timeout(self.settings.timeout):
                    done, _ = await asyncio.wait((operation, watcher), return_when=asyncio.FIRST_COMPLETED)
                    if operation in done:
                        await operation
                    else:
                        # The caller is gone: cancel pending upstream work and release admission.
                        operation.cancel()
                        with suppress(asyncio.CancelledError):
                            await operation
            finally:
                operation.cancel()
                watcher.cancel()
                await asyncio.gather(operation, watcher, return_exceptions=True)
        except TimeoutError:
            if not started:
                await reject(504, '请求超时，请缩小文档或问题范围')
        except Exception:
            if not started:
                await reject(500, '服务执行失败，请检查配置后重试')
        finally:
            self.active -= 1


def create_app(settings=None, *, transport=None):
    settings = settings or ServiceSettings.from_env()
    store = KnowledgeStore(settings.database)

    @asynccontextmanager
    async def lifespan(app):
        async with httpx.AsyncClient(follow_redirects=False, trust_env=False, transport=transport) as client:
            app.state.client = client
            yield

    app = FastAPI(title='PezMax Agent API', version=__version__, lifespan=lifespan,
                  description='BYOK Agent 与本地知识库。导入的正文会持久化；向量模式会将分块发送给指定 embedding 服务。')
    app.add_middleware(ServiceGuard, settings=settings)
    app.state.store = store

    @app.exception_handler(RequestValidationError)
    async def invalid_request(_request, _error):
        # FastAPI's default errors include `input`, possibly including credentials.
        return JSONResponse({'error': '请求字段或类型不正确，请按 OpenAPI 契约填写'}, status_code=422)

    @app.exception_handler(AgentError)
    async def agent_error(_request, error):
        status = {'not_found': 404, 'provider_not_allowed': 403, 'index_conflict': 409, 'capacity': 409}.get(error.code, 400)
        return JSONResponse({'error': str(error), 'code': error.code}, status_code=status)

    async def identity(request: Request, _credentials=Depends(HTTPBearer(auto_error=False))):
        return request.state.owner

    def knowledge(request, owner, embedding):
        encoder = Embeddings(request.app.state.client, embedding.config(settings)) if embedding else None
        return KnowledgeService(store, owner, encoder)

    @app.get('/health')
    async def health():
        return {'ready': True, 'version': __version__}

    @app.get('/v1/knowledge/documents', response_model=DocumentList)
    async def documents(owner=Depends(identity)):
        return {'documents': await asyncio.to_thread(store.list_documents, owner)}

    @app.post('/v1/knowledge/documents', response_model=IndexResult)
    async def ingest(payload: ImportRequest, request: Request, owner=Depends(identity)):
        return await knowledge(request, owner, payload.embedding).ingest(payload.document)

    @app.delete('/v1/knowledge/documents/{document_id}')
    async def delete(document_id: Identifier, owner=Depends(identity)):
        if not await asyncio.to_thread(store.delete, owner, document_id):
            raise AgentError('文档不存在或不可访问', 'not_found')
        return {'deleted': True}

    @app.get('/v1/knowledge/documents/{document_id}/chunks/{chunk_id}', response_model=KnowledgeChunk)
    async def chunk(document_id: Identifier, chunk_id: Identifier, owner=Depends(identity)):
        return await asyncio.to_thread(store.get_chunk, owner, document_id, chunk_id)

    @app.post('/v1/knowledge/search', response_model=SearchResult)
    async def search(payload: SearchRequest, request: Request, owner=Depends(identity)):
        return await knowledge(request, owner, payload.embedding).search(payload.query)

    @app.post('/v1/knowledge/ask', response_model=AskResult)
    async def ask(payload: AskRequest, request: Request, owner=Depends(identity)):
        config = payload.provider.config(settings)
        service = knowledge(request, owner, payload.embedding)
        result = await run_rag(ChatCompletionsModel(request.app.state.client, config), service, payload.query)
        return redact(result, [config.api_key, payload.embedding.apiKey.get_secret_value() if payload.embedding else ''])

    @app.post('/v1/run')
    async def run(payload: RunRequest, request: Request, _owner=Depends(identity)):
        config = payload.provider.config(settings)
        value = payload.model_dump(exclude={'provider', 'catalogToken'})
        token = payload.catalogToken.get_secret_value() if payload.catalogToken else ''
        if not payload.test and payload.task == 'search':
            if not settings.catalog_url:
                raise AgentError('部署者尚未配置平台目录接口')
            value['catalog'] = {'baseUrl': settings.catalog_url, 'token': token}
        result = await run_task(ChatCompletionsModel(request.app.state.client, config), request.app.state.client, value)
        return redact(result, [config.api_key, token])

    return app


def main(argv=None):
    parser = argparse.ArgumentParser(description='Serve the optional PezMax Agent API')
    parser.add_argument('--host', default='127.0.0.1')
    parser.add_argument('--port', type=int, default=9044)
    args = parser.parse_args(argv)
    try:
        app = create_app()
    except ValueError as error:
        parser.error(str(error))
    import uvicorn
    uvicorn.run(app, host=args.host, port=args.port, access_log=False, log_level='warning')


if __name__ == '__main__':
    main()
