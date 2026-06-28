# Go 后端与区块链架构师面试手册

面向 **5 年+ Go 后端 + 区块链/Web3 架构师** 的面试知识库（**134 篇正文**）。

**在线阅读**：https://twodog-tt.github.io/Golang-development-manual/

![gopher](./gopher.png)

> **定位**：Go 运行时与系统设计 + 链上工程（Solidity）+ 链下工程（Go RPC/索引）+ 解决方案架构 + AI 工程。

## 快速开始

| 步骤 | 链接 |
|------|------|
| 1. 学习路线（4 周 / 8 周） | [docs/learning-path-senior.md](./docs/learning-path-senior.md) |
| 2. 面试题总索引 | [docs/interview-catalog.md](./docs/interview-catalog.md) |
| 3. 题单与元数据 | [docs/interview/_meta/questions.yaml](./docs/interview/_meta/questions.yaml) |
| 4. 代码 ↔ 题目映射 | [docs/interview/_meta/mapping.md](./docs/interview/_meta/mapping.md) |
| 5. 题源与引用规范 | [docs/sources.md](./docs/sources.md) |

## 模块概览（134 题）

### Go 核心（P0，55 题）

| 模块 | 题数 | 入口 |
|------|------|------|
| [01 并发与运行时](./docs/interview/01-runtime-concurrency/index.md) | 20 | GMP、Channel、Context、泄漏 |
| [02 内存与 GC](./docs/interview/02-memory-gc/index.md) | 15 | 三色标记、逃逸、pprof |
| [03 系统设计](./docs/interview/03-system-design/index.md) | 20 | 秒杀、幂等、缓存、MQ、多活 |

### 中间件与数据库（19 题）

见 [middleware/](./docs/interview/middleware/index.md)：MySQL(5)、Redis(3)、Kafka(1)、RocketMQ(3)、ES(3)、分布式事务(1)。

### 扩展模块

| 模块 | 题数 | 入口 |
|------|------|------|
| [06 网络与服务治理](./docs/interview/06-network-governance/index.md) | 5 | gRPC、Gin、JWT、WebSocket |
| [08 手写题](./docs/interview/08-coding-senior/index.md) | 5 | LRU、令牌桶、优雅关闭、errgroup |
| [10 AI 工程与编程](./docs/interview/10-ai-engineering/index.md) | 8 | LLM、RAG、Agent、MCP |
| [11 解决方案架构](./docs/interview/11-solution-architecture/index.md) | 8 | DDD、演进、评审、45min 白板 |
| [12 区块链与 Web3（Go）](./docs/interview/12-blockchain-web3/index.md) | 9 | RPC、索引、L2、4337、abigen |
| [13 Solidity 与合约工程](./docs/interview/13-solidity-contracts/index.md) | 8 | 安全、ERC、Proxy、DeFi |
| [14 DEX / CEX 交易所工程](./docs/interview/14-dex-cex-engineering/index.md) | 9 | 撮合、账务、AMM、MEV |
| [07 工程与领导力](./docs/interview/07-engineering-leadership/index.md) | 3 | 复盘、Code Review |
| [09 云原生](./docs/interview/09-cloud-native/index.md) | 8 | K8s、Docker、HPA、Ingress |

### Web3 架构师速查

| 链上（13） | 链下（12） | 交易所（14） |
|------------|------------|--------------|
| Solidity、ERC、升级、审计 | RPC、索引、签名、abigen | CEX 撮合/账务、DEX AMM/MEV |
| [13-solidity-contracts](./docs/interview/13-solidity-contracts/index.md) | [12-blockchain-web3](./docs/interview/12-blockchain-web3/index.md) | [14-dex-cex-engineering](./docs/interview/14-dex-cex-engineering/index.md) |

## 可运行代码

| 目录 | 说明 |
|------|------|
| [basis/](./basis/) | goroutine、channel、sync、struct |
| [gin-example/](./gin-example/) | Gin Web 示例 |
| [gorm/](./gorm/) | GORM、sqlx、事务 |
| [algorithm/](./algorithm/) | LeetCode 参考实现 |
| [examples/senior/](./examples/senior/) | LRU、限流、RAG、MCP、ethrpc 等 |
| [examples/solidity/](./examples/solidity/) | 合约示例（重入防护等） |

```bash
# 进入对应示例目录运行
cd basis/goroutine && go run .
```

## 本地预览文档

```bash
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements-docs.txt
mkdocs serve   # http://127.0.0.1:8000
```

## 引用来源

见 [docs/sources.md](./docs/sources.md)。
