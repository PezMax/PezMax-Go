# Agent 0.2 服务与检索验证记录

2026-10-26，Windows x64 / Python 3.13。代码提供独立 FastAPI、官方 MCP SDK 2.x stdio 服务和跨文档 RAG。默认 Electron stdio v1 保持兼容；没有新增桌面知识库 UI，也没有将 MCP 暴露成 HTTP 端点。

## 统一检查

| 检查 | 结果 | 范围 |
| --- | --- | --- |
| `python -m unittest discover -s service_tests -v`（`.venv-service`） | 34/34 | 真实 Uvicorn 子进程、API 导入/检索/问答/删除、演示脚本、认证与身份隔离、请求体/并发限制、超时和客户端断开取消、引用校验、脚本 embedding、SQLite 持久化及幂等 |
| 官方 MCP `Client` | 包含在34项内，2种连接方式通过 | 进程内与真实 stdio 子进程；三个工具的结构化输出、只读标注、文档清单和片段资源、错误结果 |
| 原 Python 测试（`.venv`） | 38/38 | 原目录 Agent、上传建议、PDF/EPUB 阅读、隐私和预算 |
| 原阅读契约评测 | 60/60 | 脚本模型；36条无效首答未释放，18个预设可恢复案例恢复；非真实模型准确率 |
| `npm run test:system` | 105/105 | 当前共享工作区的桌面测试；包括已有的用户未提交改动，不是干净检出验证 |
| `python scripts/build.py`（`.venv`） | 通过 | Windows x64 PyInstaller 0.2.0 运行时，离线启动检查；服务依赖不进入原桌面冻结导入链 |
| `npm run test:agent-integration` | 2/2 | 源码与冻结程序分别经过 stdio、模拟 HTTP 模型和目录，验证搜索/阅读/纠正/取消 |
| `npm run build` | 通过 | Electron 主/预加载与 Vue 生产构建 |
| `.venv-service` 的 `pip check` | 通过 | `service-requirements.lock` 对应环境，没有依赖冲突 |
| Git whitespace 检查 | 通过 | 本轮提交文件 |

测试只使用明确标注的合成软件/数学资料、脚本模型与脚本向量。真实 Uvicorn 演示验证了 `examples/service_demo.py` 从本机 Markdown 提取、经 API 导入和查回原文。未读取现有用户 Key、未使用真实用户文档、未调用真实计费模型。

## 检索基线

数据集：`evals/retrieval-cases.json` v1；8份合成资料、10个手写相关性标签问题。UTF-8、LF 规范化 SHA256：`90b618294f52bf9f6b54df46d9b75e98adc280c5fa344036069166df27af87c3`。

```powershell
.venv-service\Scripts\python -m pezmax_agent.retrieval_evaluation --output evals/results/retrieval-local.json
```

| 模式 | Recall@3 | MRR@3 | 含义 |
| --- | --- | --- | --- |
| 中文/英文 BM25 | 1.00 | 0.95 | 仅以上小型合成集的文档级检索结果 |
| BYOK embedding + BM25 + RRF | 未进行真实服务评测 | 未进行真实服务评测 | 精确余弦、融合、模型/维度契约由脚本向量测试；没有测量真实语义召回收益 |

数据量很小，结果不能作为线上准确率、真实用户召回或混合检索提升的证据。`--embedding` 可用用户自己配置的真实 embedding 服务追加同数据集对照；会将合成正文/问题发送给该服务并计费。未经实测不补写提升百分比。

## 保留的边界

SQLite 保存明文正文与向量，当前按身份过滤后做有界精确扫描，未做大规模容量或云部署验证；没有 ANN、reranker、OCR、持久任务恢复或 token 流式输出。默认 `/docs` 公开接口 schema，业务 `/v1/` 需要令牌。MCP stdio 的授权来自宿主和文件系统权限，不能把它等同于网络多租户认证。

引文校验只能检查来源和原文存在，不能证明语义推论正确。API 返回的引用与检索内容属于本次请求的证据快照；数据库删除是逻辑删除。并发首次向量导入可能重复付费计算，最终由 SQLite 事务去重；现有同配置索引再次导入不调用 embedding。

本轮没有修改 Go 或用户原有 WIP。生产构建仍有已有 npm mirror/Node VM 提示，PyInstaller 仍提示可选 `tzdata` 缺失，已测链路通过。测试进程均结束；没有保持本地 API/MCP 服务运行。
