# 来源与引用规范

本知识库面向 **5 年+ Go 后端 + AI Agent Platform / Crypto Agent Ecosystem + 区块链/Web3 架构** 方向的工程知识沉淀。正文均为自研表述；外部资料仅作选题启发与延伸阅读，**不整段搬运**。

> 专题元数据：[topics/_meta/topics.yaml](topics/_meta/topics.yaml) ·
> [领域能力优先级与证据标签](topics/_meta/role-priority-matrix.md) ·
> 代码映射：[topics/_meta/mapping.md](topics/_meta/mapping.md)

## 知识库覆盖范围（235 篇）

| 模块 | 篇数 | 说明 |
|------|------|------|
| 01 并发与运行时 | 20 | GMP、Channel、Context、pprof |
| 02 内存与 GC | 15 | 三色标记、逃逸、sync.Pool |
| 16 Go 生产工程 | 6 | 错误契约、接口、测试、工具链、供应链 |
| 03 系统设计 | 21 | 秒杀、缓存、MQ、多活、SLO、CDC/Flink 实时风控 |
| 中间件与数据库 | 26 | MySQL、PostgreSQL、Redis、Kafka、RocketMQ、ES、分布式事务 |
| 06 网络与服务治理 | 7 | Linux/epoll/TCP、gRPC、Gin、JWT、WebSocket |
| 08 编码练习 | 7 | LRU、限流、连接池、Singleflight、有界批处理 |
| 10 AI 工程与编程 | 14 | 工作流/HITL、MCP/A2A、ERC-8004、x402/ERC-8183、开放平台/Launchpad |
| 11 解决方案架构 | 8 | DDD、演进、评审、白板 |
| 12 区块链与 Web3（Go） | 14 | EVM 公链全景、RPC、索引、Gas/Fee 多链计费、Rollup/DA/finality、跨链消息安全、4337、MPC |
| 17 多链钱包与托管 | 12 | Bitcoin、TRON/TRC20、Solana/Cosmos/Aptos/Sui Go 实战、归集 |
| 18 Web3 支付与稳定币 | 6 | 状态机、稳定币、账本、机构托管、DvP、RWA/ISO 20022 |
| 19 节点、RPC 与 Staking | 10 | EL/CL、RPC HA、canonical merge、trace、ClickHouse/lakehouse、在线 SDK |
| 20 协议、共识与安全 | 5 | 经典 vs 链上共识、PoS/BFT、fork choice、PeerDAS、状态迁移 |
| 21 Web3 安全工程 | 4 | 威胁模型、signer fencing、SBOM/provenance、事件响应 |
| 13 Solidity 与合约 | 8 | 安全、ERC、Proxy、DeFi |
| 14 DEX / CEX / 预测市场 | 31 | AMM/Uniswap/Pancake、Staking/Farm、DEX TL 白板、撮合/WAL、预测市场 |
| 15 微服务（交易所场景） | 6 | 服务拆分、gRPC、数据隔离、网关、事件总线 |
| 07 工程与领导力 | 5 | 复盘、技术债、Code Review、Staff 战略与迁移 |
| 09 云原生 | 10 | K8s、Docker、OTel、Terraform、Helm/GitOps |

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

### Go 后端社区选题启发

| 来源 | 链接 | 用途 |
|------|------|------|
| 2025 GO 开发岗位面试真题分析（168 道） | https://juejin.cn/post/7524308480909344806 | 领域占比、高频标签 |
| 2025 Go 面试八股（100 道含答案） | https://segmentfault.com/a/1190000046610680 | 覆盖面查漏 |
| 大厂 Go 后端 35 道深度解析 | https://developer.cloud.tencent.com/article/2647941 | 追问风格、大厂侧重点 |
| 2024 最全 Go 面经汇总 | https://juejin.cn/post/7434352545870184485 | 真实公司题目 |
| Top 20 Go Interview Questions (uByte) | https://www.ubyte.dev/blog/go-interview-questions | 代码示例结构 |
| Top 50 Go Interview Questions 2026 | https://papersadda.com/article/go-interview-questions-2026/ | 并发与编码练习 |

### 系统设计、架构与工程实践

| 来源 | 链接 | 用途 |
|------|------|------|
| Martin Fowler | https://martinfowler.com/ | DDD、Strangler Fig、演进 |
| Microservices.io | https://microservices.io/ | Saga、BFF、边界模式 |
| Google SRE Workbook | https://sre.google/workbook/ | SLO、错误预算、容量 |
| Software Engineering at Google | https://abseil.io/resources/swe-book/ | Design Doc、Large-Scale Change 与组织机制 |
| Google Engineering Practices | https://google.github.io/eng-practices/ | Review、冲突处理与小步变更 |
| ADR 实践 | https://adr.github.io/ | 架构决策记录 |
| AWS 架构博客 | https://aws.amazon.com/blogs/architecture/ | 多活、限流、发布 |

### 中间件与数据库

| 来源 | 链接 | 用途 |
|------|------|------|
| MySQL 官方文档 | https://dev.mysql.com/doc/ | 索引、事务、MVCC |
| PostgreSQL 官方文档 | https://www.postgresql.org/docs/current/ | MVCC/VACUUM、SSI、WAL 与复制 |
| pgx/pgxpool | https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool | Go 连接池与事务生命周期 |
| Linux man-pages | https://man7.org/linux/man-pages/ | FD、epoll、TCP、listen |
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
| A2A Protocol | https://a2a-protocol.org/latest/ | Agent 发现、任务生命周期与跨框架互操作 |
| ERC-8004 | https://eips.ethereum.org/EIPS/eip-8004 | Agent 身份、信誉和验证 Draft |
| x402 Foundation | https://github.com/x402-foundation/x402 | HTTP 支付协商、scheme/network 与结算 |
| ERC-8183 | https://eips.ethereum.org/EIPS/eip-8183 | Agent job escrow 与 Evaluator Draft |
| OpenTelemetry GenAI 语义约定 | https://opentelemetry.io/docs/specs/semconv/gen-ai/ | LLM 可观测性 |
| OpenAI Agents SDK | https://openai.github.io/openai-agents-python/ | Agent loop、HITL、Sessions |
| LangGraph 文档 | https://docs.langchain.com/oss/python/langgraph/overview | Persistence、Memory、Interrupts |
| ElizaOS 文档 | https://docs.elizaos.ai/plugins/architecture | Crypto Agent Runtime 与插件模型 |
| Virtuals GAME | https://whitepaper.virtuals.io/builders-hub/game-framework | Agent 决策框架与生态路线 |

### 云原生、容器与可观测性

| 来源 | 链接 | 用途 |
|------|------|------|
| Kubernetes 文档 | https://kubernetes.io/docs/ | 调度、探针、HPA、ConfigMap |
| Gateway API | https://gateway-api.sigs.k8s.io/ | 南北向流量 |
| Docker 多阶段构建 | https://docs.docker.com/build/building/multi-stage/ | Go 镜像实践 |
| Google distroless | https://github.com/GoogleContainerTools/distroless | 最小运行时镜像 |
| OpenTelemetry Go | https://opentelemetry.io/docs/languages/go/ | Traces、Metrics |
| uber-go/automaxprocs | https://github.com/uber-go/automaxprocs | 容器内 GOMAXPROCS |
| Terraform 文档 | https://developer.hashicorp.com/terraform | State、Module、Drift、Import |
| Helm 文档 | https://helm.sh/docs/ | Chart、CRD、Release 与 Rollback |
| Argo CD 文档 | https://argo-cd.readthedocs.io/en/stable/ | Auto-sync、Wave、Window 与健康 |

### 区块链、EVM 与 Go 链下工程

| 来源 | 链接 | 用途 |
|------|------|------|
| Ethereum 开发者文档 | https://ethereum.org/en/developers/docs/ | 账户、交易、合约 |
| go-ethereum 文档 | https://geth.ethereum.org/docs/ | 节点、JSON-RPC |
| EIP 索引 | https://eips.ethereum.org/ | ERC、1559、4337、代理 |
| DeFi 概述 | https://ethereum.org/en/defi/ | 链上金融概念 |
| Chainlink 文档 | https://docs.chain.link/ | Oracle 与价格喂价 |
| Flashbots 文档 | https://docs.flashbots.net/ | MEV 与私有交易 |

### 多链钱包、支付与节点基础设施

| 来源 | 链接 | 用途 |
|------|------|------|
| Bitcoin Developer Guide | https://developer.bitcoin.org/devguide/ | UTXO、钱包、支付处理 |
| Bitcoin BIPs | https://github.com/bitcoin/bips | PSBT、RBF 等协议/策略背景 |
| Solana Docs | https://solana.com/docs | Account、PDA、交易与 RPC |
| Cosmos SDK Docs | https://docs.cosmos.network/sdk/ | Modules、交易、Sequence、升级 |
| IBC Protocol | https://ibcprotocol.dev/ | Light client、packet、ack/timeout |
| Sui Docs | https://docs.sui.io/ | Object model、交易与节点 |
| Aptos Docs | https://aptos.dev/ | Account、Resource、交易与节点 |
| Circle Developer Docs | https://developers.circle.com/ | 稳定币合约与 CCTP |
| OP Stack Docs | https://docs.optimism.io/ | Rollup finality、outage、fault proof |
| Arbitrum Nitro Whitepaper | https://docs.arbitrum.io/nitro-whitepaper.pdf | Rollup、delayed inbox 与强制包含 |
| Ethereum Consensus Specs | https://github.com/ethereum/consensus-specs | LMD-GHOST、FFG、弱主观性、Fulu |
| Ethereum Execution Specs | https://github.com/ethereum/execution-specs | 执行层升级与 fork 规则 |
| EIP-4844 / EIP-7594 | https://eips.ethereum.org/EIPS/eip-7594 | Blob、KZG、PeerDAS |
| CometBFT Consensus Spec | https://github.com/cometbft/cometbft/blob/main/spec/consensus/consensus.md | Round、Prevote、Precommit、Lock |
| Raft 论文 | https://raft.github.io/raft.pdf | Leader、日志复制、CFT 成员模型 |
| PBFT 论文 | https://pmg.csail.mit.edu/papers/osdi99.pdf | 拜占庭三阶段与视图变更谱系 |
| Cosmos SDK Upgrades | https://docs.cosmos.network/sdk/latest/guides/upgrades/upgrade | Upgrade handler 与 store migration |
| Aptos Go SDK | https://github.com/aptos-labs/aptos-go-sdk | 官方 Go SDK、BCS 与交易签名 |
| Sui Releases | https://github.com/MystenLabs/sui/releases | 协议版本与 Address Balance 能力演进 |
| Sui RPC Migration | https://docs.sui.io/references/sui-api | JSON-RPC 弃用、gRPC/GraphQL 能力迁移 |
| EIP-1898 | https://eips.ethereum.org/EIPS/eip-1898 | Block-hash 一致读取与 canonical 约束 |
| Geth EVM Tracing | https://geth.ethereum.org/docs/developers/evm-tracing/ | Trace 重执行、历史状态与 client 边界 |
| ClickHouse MergeTree | https://clickhouse.com/docs/engines/table-engines/mergetree-family/mergetree | 链数据排序键、分区、part 与 merge |
| Apache Iceberg | https://iceberg.apache.org/docs/latest/ | Raw lake、snapshot、schema/partition evolution |
| ISO 20022 Message Definitions | https://www.iso20022.org/iso-20022-message-definitions | pain/pacs/camt 消息与版本 |
| Swift ISO 20022 Standards | https://www.swift.com/standards/iso-20022/iso-20022-standards | CBPR+/HVPS+ 使用与消息/结算边界 |
| CPMI-IOSCO PFMI | https://www.bis.org/cpmi/publ/d101.htm | 清算、结算、finality 与 FMI 原则 |
| BIS Tokenisation | https://www.bis.org/cpmi/publ/d225.pdf | Tokenisation、settlement asset、DvP/PvP |
| FATF Virtual Assets | https://www.fatf-gafi.org/en/topics/virtual-assets.html | Travel Rule 与风险框架 |
| OFAC Virtual Currency Guidance | https://ofac.treasury.gov/system/files/126/virtual_currency_guidance_brochure.pdf | 制裁合规工程边界 |

### 安全工程与软件供应链

| 来源 | 链接 | 用途 |
|------|------|------|
| OWASP Threat Modeling | https://cheatsheetseries.owasp.org/cheatsheets/Threat_Modeling_Cheat_Sheet.html | 资产、边界、攻击者与 abuse case |
| NIST SP 800-207 | https://csrc.nist.gov/pubs/sp/800/207/final | Zero Trust 与资源访问决策 |
| NIST SP 800-57 | https://csrc.nist.gov/pubs/sp/800/57/pt1/r5/final | 密钥生命周期与管理边界 |
| NIST SP 800-61r3 | https://nvlpubs.nist.gov/nistpubs/specialpublications/nist.sp.800-61r3.pdf | CSF 2.0 事件响应与恢复 |
| SLSA v1.2 | https://slsa.dev/spec/v1.2/build-track-basics | Build L1/L2/L3 与 provenance |
| SPDX specifications | https://spdx.dev/use/specifications/ | SBOM 标准与当前稳定版 |
| SPDX 3.1-RC1 announcement | https://spdx.dev/spdx-3-1-ontology-and-schema-available-for-review/ | 3.1 候选版状态与适用边界 |
| EIP-3030 | https://eips.ethereum.org/EIPS/eip-3030 | 远程 signer API 的验证与威胁边界 |
| EIP-3076 | https://eips.ethereum.org/EIPS/eip-3076 | Slashing protection 迁移与历史低水位 |

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
| Coinbase Matching Engine | https://docs.cdp.coinbase.com/exchange/concepts/matching-engine | 价格时间优先与 maker 价格 |
| FIX Session Layer | https://www.fixtrading.org/standards/fix-session-layer-online/ | MsgSeqNum、Resend、Gap Fill |
| Binance WebSocket Streams | https://developers.binance.com/docs/binance-spot-api-docs/web-socket-streams | Depth snapshot 与序号桥接 |
| Nasdaq Opening/Closing Cross | https://www.nasdaqtrader.com/content/ProductsServices/Trading/Crosses/openclose_faqs.pdf | 集合竞价清算价与不平衡 |
| Binance 永续合约 FAQ | https://www.binance.com/en/support/faq/perpetual-futures-contracts | 资金费率、强平 |
| 1inch 聚合协议 | https://docs.1inch.io/docs/aggregation-protocol/introduction | DEX 路由 |
| Gnosis Conditional Tokens | https://gnosis-conditional-tokens.readthedocs.io/en/latest/developer-guide.html | CTF condition、position、split/merge/redeem |
| EIP-712 | https://eips.ethereum.org/EIPS/eip-712 | 结构化签名与 replay protection 边界 |
| Polymarket CTF Exchange V2 | https://github.com/Polymarket/ctf-exchange-v2 | CLOB 链上结算与版本差异 |
| UMA Optimistic Oracle | https://docs.uma.xyz/protocol-overview/how-does-umas-oracle-work | 提案、bond、liveness、争议仲裁 |
| FATF 建议 | https://www.fatf-gafi.org/en/topics/fatf-recommendations.html | AML/KYT 合规框架 |
| 复式记账（百科） | https://en.wikipedia.org/wiki/Double-entry_bookkeeping | 交易所账务 |

## 引用规则

1. 每篇 YAML `sources` 字段至少 1 个外链或官方文档；详见 [专题撰写模板](topics/_meta/template.md)。
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
