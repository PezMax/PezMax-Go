# Agent 0.4 平台资料 RAG 验证记录

2026-10-27，Windows x64。功能与取舍见[平台资料 RAG](./platform-knowledge.md)。

| 检查 | 结果 | 范围 |
| --- | --- | --- |
| Go `go test ./internal/kadmin ./internal/kadmin/modules/datum` | 通过 | 原功能及新增4项：目录认证/字段投影、正文版本与下架、版本摘要、真实存储大小限制 |
| Python `.venv` 的 `unittest discover -s tests -v` | 49/49 | 新增7项平台同步测试，覆盖增量更新、内容相同的独立来源、账号隔离、平台失败、拒绝重定向、失败重试、向量升级、真实桌面适配器与重复来源校验 |
| Python `.venv-service` 的 `unittest discover -s service_tests -v` | 34/34 | 原 HTTP/MCP/共享检索与引用问答回归，SQLite source 字段迁移不改变个人知识库接口 |
| 桌面 `npm run test:system` | 118/118 | 新增4项平台契约/会话/导航/主进程测试、1项真实首页 SFC 引用事件测试；扩展知识库 SFC 测试验证默认平台同步、原文导航及切回个人导入 |
| `npm run test:agent-integration` | 6/6 | 原4项及新增源码/冻结平台链路2项：真实 PDF 读取、无Key增量同步、混合检索/引用问答、64位ID、原文读取、版本更新、生成中下架、账号和个人索引隔离 |
| `npm run build` | 通过 | Electron 主/预加载、Vue 生产构建及模板依赖 |
| `npm run agent:build` 与冻结程序 `--check` | 通过 | Windows x64 `packageVersion=0.4.0`，包含平台同步适配器 |
| 两个 Python 环境 `pip check` | 通过 | 无依赖冲突，没有新增运行依赖 |

Go 测试在真实 Gin 路由与认证上执行，业务数据库与存储用替身隔离；目录用例核查SQL过滤条件、投影与上限，不等同于实际 PostgreSQL/MinIO 联调。平台集成用本机 HTTP 脚本模拟业务目录、正文、对话与向量服务，通过真正的 stdio 源码/冻结 worker 执行。PDF 内容明确标注为合成软件队列示例，没有读取平台实际资料、用户私人文件或保存的 API Key，没有调用计费模型。

这些结果证明协议、生命周期和引用机制，不证明真实模型准确率、真实向量召回收益或全站性能。桌面 SFC 测试执行真实组件事件与 Vue 生命周期，但没有人工登录后点击验收、PDF/EPUB 渲染器视觉验收或生产部署。当前共享工作区含用户原有未提交改动；它们未纳入本次阶段提交，测试结果不等同于干净检出验证。

当前检索范围最多200篇、10000片段、1000万字符，按平台最近更新记录选取；界面展示范围和失败项。仍没有 OCR、ANN、重排、后台全站索引任务或跨设备共享向量。平台校验失败会停止返回缓存证据；此前已展示、保存或打开的公开资料无法撤回。
