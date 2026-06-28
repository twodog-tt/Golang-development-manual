# 题源与引用规范

本手册面向 **5 年+ Go 后端 + 区块链/Web3 架构师** 面试准备。正文均为自研表述；外部资料仅作题源与延伸阅读，**不整段搬运**。

> 题单元数据：[interview/_meta/questions.yaml](interview/_meta/questions.yaml) · 代码映射：[interview/_meta/mapping/](interview/_meta/mapping/)

## 手册覆盖范围（134 题）

| 模块 | 题数 | 说明 |
|------|------|------|
| 01 并发与运行时 | 20 | GMP、Channel、Context、pprof |
| 02 内存与 GC | 15 | 三色标记、逃逸、sync.Pool |
| 03 系统设计 | 20 | 秒杀、缓存、MQ、多活、SLO |
| 中间件与数据库 | 19 | MySQL、Redis、Kafka、RocketMQ、ES、分布式事务 |
| 06 网络与服务治理 | 5 | gRPC、Gin、JWT、WebSocket |
| 08 手写题 | 5 | LRU、限流、优雅关闭、errgroup |
| 10 AI 工程与编程 | 8 | LLM、RAG、Agent、MCP |
| 11 解决方案架构 | 8 | DDD、演进、评审、白板 |
| 12 区块链与 Web3（Go） | 9 | RPC、索引、L2、4337、abigen |
| 13 Solidity 与合约 | 8 | 安全、ERC、Proxy、DeFi |
| 14 DEX / CEX 交易所 | 9 | 撮合、账务、AMM、MEV |
| 07 工程与领导力 | 3 | 复盘、技术债、Code Review |
| 09 云原生 | 8 | K8s、Docker、OTel、排障 |

## 主要参考来源

### Go 语言与运行时（官方优先）

| 来源 | 链接 | 用途 |
|------|------|------|
| Go 官方博客 | https://go.dev/blog/ | 版本事实、语言演进 |
| Go Memory Model | https://go.dev/ref/mem | happens-before、并发语义 |
| GC Guide | https://go.dev/doc/gc-guide | 三色标记、GOGC、pprof |
| Diagnostics | https://go.dev/doc/diagnostics | pprof、trace、race |
| Scheduler 设计 | https://go.dev/blog/scheduler | GMP 历史 |
| The Go GC | https://go.dev/blog/ismmkeynote | GC 设计 |
| Effective Go | https://go.dev/doc/effective_go | 语言惯例 |
| pkg.go.dev | https://pkg.go.dev/ | 标准库与 x/sync 等 |

### Go 后端面试题源（社区）

| 来源 | 链接 | 用途 |
|------|------|------|
| 2025 GO 开发岗位面试真题分析（168 道） | https://juejin.cn/post/7524308480909344806 | 领域占比、高频标签 |
| 2025 Go 面试八股（100 道含答案） | https://segmentfault.com/a/1190000046610680 | 覆盖面查漏 |
| 大厂 Go 后端 35 道深度解析 | https://developer.cloud.tencent.com/article/2647941 | 追问风格、大厂侧重点 |
| 2024 最全 Go 面经汇总 | https://juejin.cn/post/7434352545870184485 | 真实公司题目 |
| Top 20 Go Interview Questions (uByte) | https://www.ubyte.dev/blog/go-interview-questions | 代码示例结构 |
| Top 50 Go Interview Questions 2026 | https://papersadda.com/article/go-interview-questions-2026/ | 并发与手写题 |

### 系统设计、架构与工程实践

| 来源 | 链接 | 用途 |
|------|------|------|
| Martin Fowler | https://martinfowler.com/ | DDD、Strangler Fig、演进 |
| Microservices.io | https://microservices.io/ | Saga、BFF、边界模式 |
| Google SRE Workbook | https://sre.google/workbook/ | SLO、错误预算、容量 |
| ADR 实践 | https://adr.github.io/ | 架构决策记录 |
| AWS 架构博客 | https://aws.amazon.com/blogs/architecture/ | 多活、限流、发布 |

### 中间件与数据库

| 来源 | 链接 | 用途 |
|------|------|------|
| MySQL 官方文档 | https://dev.mysql.com/doc/ | 索引、事务、MVCC |
| Redis 文档 | https://redis.io/docs/ | 集群、分布式锁 |
| Kafka 文档 | https://kafka.apache.org/documentation/ | 消费语义、Rebalance |
| RocketMQ 文档 | https://rocketmq.apache.org/docs/ | 事务消息、顺序 |
| Elasticsearch 指南 | https://www.elastic.co/guide/ | 倒排索引、DSL |
| GORM 文档 | https://gorm.io/docs/ | ORM 陷阱与钩子 |

### AI 工程与 LLM 应用

| 来源 | 链接 | 用途 |
|------|------|------|
| OpenAI API 文档 | https://platform.openai.com/docs/ | 流式、Function Calling |
| Azure OpenAI 内容过滤 | https://learn.microsoft.com/en-us/azure/ai-services/openai/concepts/content-filter | 护栏与合规 |
| OWASP LLM Top 10 | https://owasp.org/www-project-top-10-for-large-language-model-applications/ | LLM 安全 |
| Model Context Protocol | https://modelcontextprotocol.io/ | MCP 协议与工具暴露 |
| MCP Go SDK | https://github.com/modelcontextprotocol/go-sdk | Go 实现 MCP Server |
| OpenTelemetry GenAI 语义约定 | https://opentelemetry.io/docs/specs/semconv/gen-ai/ | LLM 可观测性 |

### 云原生、容器与可观测性

| 来源 | 链接 | 用途 |
|------|------|------|
| Kubernetes 文档 | https://kubernetes.io/docs/ | 调度、探针、HPA、ConfigMap |
| Gateway API | https://gateway-api.sigs.k8s.io/ | 南北向流量 |
| Docker 多阶段构建 | https://docs.docker.com/build/building/multi-stage/ | Go 镜像实践 |
| Google distroless | https://github.com/GoogleContainerTools/distroless | 最小运行时镜像 |
| OpenTelemetry Go | https://opentelemetry.io/docs/languages/go/ | Traces、Metrics |
| uber-go/automaxprocs | https://github.com/uber-go/automaxprocs | 容器内 GOMAXPROCS |

### 区块链、EVM 与 Go 链下工程

| 来源 | 链接 | 用途 |
|------|------|------|
| Ethereum 开发者文档 | https://ethereum.org/en/developers/docs/ | 账户、交易、合约 |
| go-ethereum 文档 | https://geth.ethereum.org/docs/ | 节点、JSON-RPC |
| EIP 索引 | https://eips.ethereum.org/ | ERC、1559、4337、代理 |
| DeFi 概述 | https://ethereum.org/en/defi/ | 链上金融概念 |
| Chainlink 文档 | https://docs.chain.link/ | Oracle 与价格喂价 |
| Flashbots 文档 | https://docs.flashbots.net/ | MEV 与私有交易 |

### Solidity 与合约工程

| 来源 | 链接 | 用途 |
|------|------|------|
| Solidity 语言文档 | https://docs.soliditylang.org/ | 存储布局、Gas |
| OpenZeppelin Contracts | https://docs.openzeppelin.com/contracts/ | ERC、Proxy、安全组件 |
| Consensys Smart Contract Best Practices | https://consensys.github.io/smart-contract-best-practices/ | 重入、密钥、链上链下边界 |
| Foundry Book | https://book.getfoundry.sh/ | 测试、Fuzz、Fork |
| Slither | https://github.com/crytic/slither | 静态分析（结论向） |
| Uniswap 文档 | https://docs.uniswap.org/ | AMM、V2/V3 概念 |

### DEX / CEX 交易所业务

| 来源 | 链接 | 用途 |
|------|------|------|
| 撮合引擎概念（Investopedia） | https://www.investopedia.com/terms/m/matchingengine.asp | CEX 订单簿 |
| Binance 永续合约 FAQ | https://www.binance.com/en/support/faq/perpetual-futures-contracts | 资金费率、强平 |
| 1inch 聚合协议 | https://docs.1inch.io/docs/aggregation-protocol/introduction | DEX 路由 |
| FATF 建议 | https://www.fatf-gafi.org/en/topics/fatf-recommendations.html | AML/KYT 合规框架 |
| 复式记账（百科） | https://en.wikipedia.org/wiki/Double-entry_bookkeeping | 交易所账务 |

## 引用规则

1. 每题 YAML `sources` 字段至少 1 个外链或官方文档；详见 [题目撰写模板](interview/_meta/template/)。
2. 博客、面经类内容只链出，正文用自己的话归纳，**不整段搬运**。
3. 标注 `go_version` / Solidity 版本，避免泛型、loop 变量、PUSH0 等说法过时。
4. **系统设计 / 架构题**：注明假设（QPS、一致性、地域），便于读者复现推演。
5. **Web3 / 链上题**：注明链 ID、finality、信任模型（官方桥 / 多签 / 轻客户端）。
6. **交易所题**：区分 CEX（托管账本）与 DEX（链上协议）边界，避免混用术语。
7. **AI 题**：区分模型 API 版本；安全与 PII 处理引用 OWASP / 厂商护栏文档。

## 版权说明

- 本仓库代码示例遵循项目原有许可（见仓库根目录）。
- 文档内容为学习笔记性质，如有侵权请联系移除。
- 第三方商标与产品名称归各自所有者。
