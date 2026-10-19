# 链上交易执行与数据服务本地项目

三篇共用同一条可复现链路：**交易执行网关 → Indexer 查询与数据质量 → DeFi 协议接入**。

```text
viem 部署固定版本 Uniswap V2 + 合成 ERC20 → 注入流动性
  → Go 模拟调用 → PostgreSQL 意图 / nonce / 签名原文
  → 广播 / 同 nonce 提价 / 回执与确认 → Anvil 本地 EVM
  → 区块 + hash-scoped logs + receipt 交叉检查
  → raw 证据 / canonical 映射 / 原子水位 → 快照游标查询
```

## 与稳定币支付接通

新增 [完整端到端项目](../stablecoin-ops/e2e/README.md)，钱包通过实际 PaymentReceiver 付款后，接通支付入账、冻结、兑换、索引确认、双资产过账和商户 Inbox。

```bash
python3 examples/stablecoin-ops/e2e/verify.py --report /tmp/payment-e2e/result.json
```

原独立故障入口继续保留，负责提价、游标、ABI 等底层验证；两种入口的 Token decimals 和账务覆盖不同。

新增 [人工恢复实战](../stablecoin-ops/e2e/RECOVERY.md)：计划批准、同 nonce 取消、固定意图封存，以及两库/资金核对后的 signer 解封；独立 5 组可运行验证。

## 一键运行

需要 Go 1.24.2+、Node 22、npm、Python 3.10+、Docker，以及 **Foundry v1.4.4 的 Anvil**。已安装 foundryup 的机器可执行 `foundryup --install v1.4.4`；安装说明见 [Foundry 官方文档](https://www.getfoundry.sh/introduction/installation/)。运行器检查 Anvil 版本，不自动安装工具或拉取镜像。

```bash
docker pull postgres:17-alpine
python3 examples/onchain-execution/verify.py --report /tmp/onchain-execution/result.json
```

使用独立 Go module 与 npm lock，不更改仓库根目录 vendor。运行器执行 Go race 测试、`npm ci --ignore-scripts`、构建 Go 服务，再启动随机命名 PostgreSQL 容器、随机回环端口 Anvil 和真实 Go 进程。链明确固定 **31337 / Cancun**；geth 1.14.12 的 Header 类型不能据此声称兼容后续硬分叉字段。

协议制品来自 npm **@uniswap/v2-core 1.0.1 / @uniswap/v2-periphery 1.1.0-beta.0** 的锁定构建产物，实际部署 Factory、Router02、Pair 与 WETH9；不是自行仿写 AMM。测试 Token 由 **solc 0.8.30 / Paris target / optimizer 200** 编译，分别使用 6 与 18 位小数。`Token.sol` 是普通合成资产，没有真实 USDC 的冻结/升级/跨链等语义。V2 是本课固定协议范围，不代表当前 Uniswap 的最新版本。

## 故障矩阵与实际判定

| 自动验证组 | 注入与验收 |
|---|---|
| 协议部署 | 实际创建池、添加流动性，记录 Router 运行时代码哈希 |
| 意图恢复 | 8 路并发同 key 只有 1 行；同 key 改金额 409；代理先转发交易再返回 503；SIGKILL Go 进程后重启，原 hash/nonce 对应一次 Swap |
| Replacement | 关闭自动挖块，广播后同 nonce 2 倍 gas price 提价；只挖到新 attempt，旧 hash 无 receipt；已上链禁止 bump |
| 并发 nonce | 4 个不同意图并发创建，nonce 唯一且连续；按序广播全部上链 |
| 前置拒绝 | RPC 返回错误 chain ID、金额小数、过期 deadline、过高 minOut、零 allowance 均不创建意图 |
| 模拟与执行差异 | 模拟成功后由另一个账户改变池储备；原交易上链 status=0，输出资产余额不变（仍消耗 gas） |
| 查询快照 | 占用真实写锁并从 pg_locks 确认同步请求等待，断言尚未读取 head；等待期间挖块，释放锁后追上新 head 且 epoch 不误增。固定高度/hash，读下一页期间新增区块并同步；全量分页无重复/缺失且不混入新快照 |
| RPC 数据质量 | 代理删掉一个 Transfer 或篡改其 ABI payload；receipt 交叉校验或解码拒绝，水位不前移，持久化质量错误 |
| 数据库原子性 | canonical 写入触发器抛错；raw 和水位全部回滚，删除故障触发器后重扫成功 |
| 链重组 | Anvil snapshot/revert 创建另一分支；旧 raw 保留、canonical 排除旧成交、epoch 增加、旧游标 409，旧 confirmed 意图进入 orphaned_review；真实重新广播原签名被包含后，review_required 仍保留，新意图 409 |

`--report` 开始时删除该路径的旧报告，只在整组成功后写 `result=passed`，包含运行 ID、工具版本、镜像 ID、逐组事实和源文件 SHA-256。`local-verification.json` 是一次本地记录，不能当作每个环境都成功的证明。

数据库密码每次随机生成；不挂载已有数据库或项目目录。退出、失败、Ctrl-C 会终止本次进程组并删除本次容器和匿名卷；强杀 Python 或 Docker 故障可能留下资源，按 `knowledge-onchain-lab=<run_id>` 标签检查，只清理该次资源。

## 本地交互与接口

```bash
python3 examples/onchain-execution/verify.py --serve
# 跑完故障矩阵后保留环境，复制输出的实际 API 地址：
curl http://127.0.0.1:实际端口/health
curl 'http://127.0.0.1:实际端口/events?limit=2'
curl http://127.0.0.1:实际端口/intents/recover
curl -X POST http://127.0.0.1:实际端口/index/sync -d '{}'
```

| 接口 | 语义 |
|---|---|
| `POST /intents/{id}` | `{amount:"100000000",min_out:"...",deadline:unix_seconds}`；原子金额字符串，固定资产路径与接收者，模拟后持久签名，尚不广播 |
| `POST /intents/{id}/broadcast` | 广播已存原文；失败状态未知，先 GET 对账，不换业务 ID |
| `POST /intents/{id}/bump` | 最多两次提价，只持久新 attempt；仍需 broadcast；业务 calldata 与 nonce 不变 |
| `GET /intents/{id}` | 检查每个 attempt 的 canonical receipt，输出 signed/broadcast/mined/confirmed/reverted/orphaned_review |
| `POST /index/sync` | 显式轮询推进；单写者事务锁，最多回溯 64 块，超界停止 |
| `GET /events?limit=2&cursor=...` | 按 `(height,log_index)` keyset、固定 canonical 快照分页；reorg 返回 409，重新发起第一页 |
| `GET /health` | head/indexed/hash/lag/epoch、累计质量错误；不是生产 SLO 或自动修复器 |

422 表示本地请求/依赖验证失败（包含广播未知），409 表示意图或游标冲突，404 表示意图不存在。此教学 API 尚未把 RPC、数据库错误拆成生产标准错误码；调用方不能把 422 广播未知当成交易不存在。`quality_errors_total` 为历史累计数，不是当前未恢复错误数。

## 范围和生产接入边界

本例仅绑定回环 IP，使用公开 Anvil 测试私钥，**不可充值真实资产**。没有身份认证、租户隔离、KMS/HSM、访问限流或资金审批；静态知识站只部署文档，不上线此 API。单链、单签名账户、单 Router、单池、单向 exact-input；网关固定 calldata 路径与收款人，校验 chain ID 和 Router 代码哈希。业务幂等键是本地全局 key，生产必须增加租户作用域。

签名账户必须由网关独占。示例故障夹具在已无 pending 意图时才直接用该账户执行 approve；生产不能让另一个钱包应用并发占用 nonce。签名原文先入库，恢复只认相同 intent/nonce/attempt；新建、广播、提价与 reconcile 共用 signer advisory lock。未解决的 reorg 或 pending nonce 回落会暂停账户授权；review_required 独立持久化，重新包含不会自动解除人工审查。不自动释放 nonce、不自动跳过 stuck gap、不在 reorg 后自动创建新意图。

两块确认只演示业务深度阈值，不是 Ethereum PoS finality。Anvil rollback 是可控本地分支测试，不能替代 EL/CL、多 Provider、L2 sequencer、跨节点网络分区与 finalized 冲突验收。Indexer 用单一节点的 logs 与 receipts 交叉核对，可检测两接口不一致，不能识别同一节点一致伪造或同时漏报；上生产需独立来源、根/证明验证和覆盖率审计。

游标是透明编码，不是授权凭据；生产需请求作用域、过滤条件绑定与签名，限制历史快照和查询预算。raw 通过应用的 append-only 路径保留，数据库 lab 角色仍可修改；生产需最小权限和不可变存储策略。独立入口读取两类事件；e2e 入口可选配置第三类 Paid，提供 /quote 和收款/执行证据接口，支付侧另有账本和调度。本模块仍未实现 ClickHouse 或无限历史回填。

池内报价用于确定性测试，未实现独立价格源、价格偏离上限、MEV 保护或跨协议最佳路由。生产还需审核 Token 行为、授权额度与撤销、模拟区块新鲜度、交易费用总额上限、EIP-1559/L2 费用模型、权限与资金风险，不应直接运行这个公开测试签名器。
