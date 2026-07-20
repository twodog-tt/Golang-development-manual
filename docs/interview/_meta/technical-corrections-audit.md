# P0 技术纠错审计

> 累计范围：**75 篇高风险 P0 正文**
>
> 审计日期：**2026-07-18**
>
> 第一批：**21 篇修正，4 篇复核通过**
>
> 第二批：**24 篇修正，1 篇复核通过**
>
> 第三批：**18 篇修正，7 篇复核通过**
>
> 累计结果：**63 篇修正，12 篇复核通过且无需事实修改**

本轮目标不是扩充题量，而是避免在面试中把实现细节说成规范保证、把业务风险阈值说成协议
finality，或把本地示例说成生产验收。该轮简历定向补强结束时全库为 219 篇；新增预测市场
四篇预测市场专项正文后为 223 篇；新增四篇 Crypto Agent 生态 P0 正文后，当前为
**227 篇正文**。

## 审计方法与证据边界

1. 以语言规范、目标版本 release notes、数据库/协议官方文档和 RFC 为第一事实源。
2. 明确区分 **API/协议契约**、**特定版本实现**、**架构建议** 和 **业务策略**。
3. 对资金、签名、共识和恢复语义，必须写出失败状态、unknown 状态和重试/冲正边界。
4. 本轮是文档与源码静态核对，没有调用 Docker、数据库、链节点、HSM 或 MPC 服务，也没有
   因此新增 `external_acceptance` 证据。
5. “复核通过”只表示本轮未发现需要修改的实质性技术结论，不代表该正文永久免于版本复核。

## 第一批必须记住的纠错结论

| 原风险表达 | 安全面试表达 |
|------------|--------------|
| Go 1.25 工具链天然启用容器感知 `GOMAXPROCS` | 还要看模块语言版本与 `GODEBUG`；`go 1.24` 及以下默认保留旧兼容行为 |
| Go 1.26 的 `sync.Map` 仍是 read/dirty 双表 | Go 1.26 已切换为并发 hash-trie；read/dirty 是 1.25 及更早实现细节 |
| 普通 `int` 在 32 位平台可能撕裂，所以要 atomic | 普通无同步读写首先是 data race；大于机器字及多字结构才还有分步读写风险 |
| 普通 `VACUUM` 会维护 planner statistics | planner statistics 由 `ANALYZE` 更新；可单独执行或使用 `VACUUM (ANALYZE)` |
| 多列 B-tree 不带前导列也可能 skip scan | PostgreSQL 18 才引入该能力；17 及更早不能照搬 |
| group commit 会扩大已确认交易的 RPO | 若 barrier 完成后才 ACK，group commit 主要改变批量与延迟；barrier 前 ACK 才产生已确认数据丢失窗口 |
| parent lineage 连续就证明候选是主链 | 连续性只是必要条件，还要链特有 fork choice、共识证明或 canonical authority |
| N confirmations 可以叫 finalized watermark | 协议 finality 与业务 credit/risk watermark 必须分开 |
| OP fault proof 挑战期决定 L2 block finality | OP fault proof 约束 L1 提款声明；OP L2 finalized head 跟随 L1 origin finality |
| MPC 门限越高，业务授权天然越强 | 门限提高的是 share compromise 难度；业务授权仍来自身份、policy、审批和 intent 验证 |

## 第一批 25 篇逐项结果

| 题目 | 结果 | 本轮核对重点 |
|------|------|--------------|
| [S-CONC-04](../01-runtime-concurrency/S-CONC-04-gomaxprocs.md) | 已修正 | Go 1.25 模块语言版本、动态更新与 automaxprocs 边界 |
| [S-CONC-14](../01-runtime-concurrency/S-CONC-14-memory-model.md) | 已修正 | `sync.Once` 精确定义、word/multiword race 表达 |
| [S-CONC-19](../01-runtime-concurrency/S-CONC-19-netpoller.md) | 已修正 | epoll 仅为 Linux 实现、阻塞 M 与 P handoff、resolver 路径 |
| [S-CONC-20](../01-runtime-concurrency/S-CONC-20-go122-generics.md) | 已修正 | loopvar 按 package language version 生效，编译器诊断参数非稳定 API |
| [S-MEM-01](../02-memory-gc/S-MEM-01-tri-color-gc.md) | 已修正 | 堆/全局写屏障与普通栈写例外 |
| [S-MEM-03](../02-memory-gc/S-MEM-03-gogc-tuning.md) | 复核通过 | GOGC、GOMEMLIMIT、Green Tea 与 RSS 边界 |
| [S-MEM-04](../02-memory-gc/S-MEM-04-escape-analysis.md) | 已修正 | interface/any 不等于必然逃逸，必须看目标编译器与 benchmark |
| [S-MEM-06](../02-memory-gc/S-MEM-06-map-internals.md) | 已修正 | Go 1.24 Swiss map 与 Go 1.26 `sync.Map` hash-trie |
| [S-MEM-09](../02-memory-gc/S-MEM-09-oom-debug.md) | 已修正 | 32 KiB 仅为当前 runtime 小对象上限量级 |
| [S-GOENG-04](../16-go-production-engineering/S-GOENG-04-fuzz-benchmark-race.md) | 复核通过 | fuzz/race/benchmark 能证明与不能证明的范围 |
| [S-NET-06](../06-network-governance/S-NET-06-linux-fd-epoll-netpoll.md) | 复核通过 | FD/open file description、readiness 与 Go netpoll |
| [S-NET-07](../06-network-governance/S-NET-07-tcp-lifecycle-queues-timewait.md) | 复核通过 | backlog、TIME_WAIT、主动关闭与阶段化超时 |
| [S-PG-01](../middleware/postgresql/S-PG-01-mvcc-vacuum-indexes.md) | 已修正 | VACUUM/ANALYZE 分工、PostgreSQL 18 skip scan 版本 |
| [S-PG-02](../middleware/postgresql/S-PG-02-isolation-locking-ledger.md) | 已修正 | Read Committed 并发更新后的 `WHERE` 重检与不一致快照 |
| [S-PG-03](../middleware/postgresql/S-PG-03-wal-replication-pgx-ha.md) | 已修正 | 同步备为空时的提交保证、pgx rollback 独立有界 context |
| [S-EXCH-03](../14-dex-cex-engineering/S-EXCH-03-account-ledger.md) | 已修正 | 完整资源锁序、decimal/scale/rounding/overflow 边界 |
| [S-EXCH-17](../14-dex-cex-engineering/S-EXCH-17-runnable-deterministic-matching-engine.md) | 已修正 | 定点整数仍需 notional 与累计量 overflow 检查 |
| [S-EXCH-18](../14-dex-cex-engineering/S-EXCH-18-wal-snapshot-replay.md) | 已修正 | group commit 与异步 ACK/RPO 分离 |
| [S-PAY-04](../18-web3-payments-stablecoin/S-PAY-04-ledger-clearing-settlement-reconciliation.md) | 已修正 | 链、托管、银行与法律 finality 分层 |
| [S-NODE-02](../19-node-rpc-staking/S-NODE-02-rpc-ha-quorum-hedging-cache.md) | 已修正 | RPC quorum 只是分歧检测，不是共识证明 |
| [S-NODE-07](../19-node-rpc-staking/S-NODE-07-canonical-backfill-realtime-merge.md) | 已修正 | lineage 必要非充分、protocol finality 与 risk watermark |
| [S-BC-11](../12-blockchain-web3/S-BC-11-rollup-finality-da-proof-security.md) | 已修正 | OP L2 finality 与 fault-proof withdrawal challenge 分离 |
| [S-BC-12](../12-blockchain-web3/S-BC-12-cross-chain-message-bridge-security.md) | 已修正 | 同链原子回滚与链下 reservation/unknown 状态分离 |
| [S-WALLET-07](../17-multichain-wallet/S-WALLET-07-mpc-dkg-reshare-recovery.md) | 已修正 | share refresh 的销毁/威胁模型，threshold 不等于业务授权 |
| [S-SEC-02](../21-security-engineering/S-SEC-02-key-ceremony-signer-fencing-recovery.md) | 已修正 | 持久 fence 仍不能单独消除 HSM/MPC 调用完成窗口 |

## 第二批必须记住的纠错结论

| 原风险表达 | 安全面试表达 |
|------------|--------------|
| goroutine 进入 syscall 都会立即把 P 交出去 | `entersyscallblock` 会主动 handoff；普通 `entersyscall` 由 sysmon 在满足条件时尝试 retake，不能混成一条固定时序 |
| `main` 返回但其他 goroutine 阻塞属于 deadlock | `main` 返回会结束进程；只有程序整体再无可运行工作且 runtime 判定无进展时才可能报全局 deadlock |
| buffered channel 总会把元素直接交给等待者 | unbuffered 可直接复制；buffered channel 的 send/receive 仍围绕 buffer 状态推进，不能套同一实现图 |
| `CancelFunc` 返回说明子任务已经退出 | cancel 传播/关闭 `Done` 不等于等待 worker 结束；调用方仍需 `WaitGroup`、`errgroup` 或明确 join |
| 有数据竞态就像 C/C++ 一样完全 undefined | data race 仍是必须修的错误并失去 DRF-SC 保证，但 Go 规范保留最低实现约束，不能照搬 C/C++ 话术 |
| 二级索引都要回表，`ALGORITHM=INPLACE` 就是无锁 | 只有非覆盖查询才回表；`INPLACE` 不等于 `INSTANT`/`LOCK=NONE`，metadata lock、旧事务与操作/版本差异仍可能阻塞 |
| GORM Hook 默认不在事务里；嵌套 Preload 是两条 SQL | Hook 收到的 `tx` 在当前写事务中，逃逸来自误用全局 DB；posts/comments/users 嵌套预加载通常是固定三条查询 |
| 分布式锁天然保证任意时刻只有一个执行者 | 租约、暂停与异步 failover 可能让新旧进程重叠；正确性路径还需 fencing、幂等或存储约束 |
| Kafka 4.x 的 producer 默认值适用于所有 Go 客户端 | `acks=all`、默认幂等、in-flight/linger 等必须绑定 Java client 与版本；Sarama、kafka-go、librdkafka 分别核对 |
| Solana `getSignatureStatuses` 返回 null 就能重签 | 默认只查 recent status cache；`null` 不是未执行证明，要结合历史查询、有效高度、provider 保留能力和业务状态 |
| Cosmos 交易永远必须递增 sequence | 传统交易需要 sequence；SDK 0.53+ 在目标链启用后可用 unordered 模式，其 sequence 为零并使用唯一 timeout timestamp 防重放 |
| Aptos v2 `@latest` 永久要求某个 Go 版本 | 只能陈述具体 tag/commit；2026-07-18 的 v2 `main` 要求 Go 1.25，不能替代检查所选 release 的 `go.mod` |
| Sui `sender+asset` 是协议 nonce，且 JSON-RPC 已在所有 provider 同时停用 | 它是钱包应用的额度预占键；JSON-RPC 是 deprecated/计划停用，公共端点与私有 provider 的实际时间要分别核对 |

## 第二批 25 篇逐项结果

| 题目 | 结果 | 本轮核对重点 |
|------|------|--------------|
| [S-CONC-02](../01-runtime-concurrency/S-CONC-02-gmp-roles.md) | 已修正 | 普通 syscall 与 `entersyscallblock` 的 P handoff/retake、P/M 关系、GOMAXPROCS 下限 |
| [S-CONC-05](../01-runtime-concurrency/S-CONC-05-channel.md) | 已修正 | unbuffered/buffered slow path、select 规范保证与 runtime 实现、close 所有权 |
| [S-CONC-06](../01-runtime-concurrency/S-CONC-06-channel-deadlock.md) | 已修正 | `main` 返回不是 deadlock、全局 detector 的 timer/netpoll/cgo 边界 |
| [S-CONC-08](../01-runtime-concurrency/S-CONC-08-sync-primitives.md) | 已修正 | Mutex 公平性非 API 契约、atomic SC、`atomic.Value` 类型/复制约束、跨 goroutine Unlock |
| [S-CONC-12](../01-runtime-concurrency/S-CONC-12-context.md) | 已修正 | CancelFunc 不等待 worker、未 cancel 的 child/timer 引用、显式 join |
| [S-CONC-13](../01-runtime-concurrency/S-CONC-13-goroutine-leak.md) | 已修正 | Go 1.26 实验性 leak profile 能力边界、pprof 成本、closed-channel worker 退出 |
| [S-CONC-15](../01-runtime-concurrency/S-CONC-15-race-detector.md) | 已修正 | Go data-race 语义、只检测执行路径、可实际触发的示例 |
| [S-GOENG-01](../16-go-production-engineering/S-GOENG-01-errors-contract-panic-boundary.md) | 已修正 | recover 必须由同 goroutine 的 deferred function 直接调用，不能从 panic 点续跑 |
| [S-GOENG-03](../16-go-production-engineering/S-GOENG-03-testing-table-fake.md) | 已修正 | `t.Run` 同步边界、并行闭包的 Go 1.22 loopvar 按 package language version 生效 |
| [S-GOENG-05](../16-go-production-engineering/S-GOENG-05-modules-toolchain-reproducible.md) | 已修正 | MVS 选择、依赖模块 replace 无效、workspace replace 与 VCS/C toolchain 可复现边界 |
| [S-DB-01](../middleware/mysql/S-DB-01-mysql-index.md) | 已修正 | 聚簇索引 fallback、覆盖查询才免回表、在线 DDL 算法与锁边界 |
| [S-DB-02](../middleware/mysql/S-DB-02-transaction-mvcc.md) | 已修正 | RR 一致性读/锁定读分工、Read View 建立点、RC gap-lock 例外、GORM 事务边界 |
| [S-DB-05](../middleware/mysql/S-DB-05-gorm-pitfalls.md) | 已修正 | Hook `tx` 同事务、全局 DB 逃逸、嵌套 Preload 的实际查询数 |
| [S-DB-06](../middleware/mysql/S-DB-06-advanced-sql.md) | 复核通过 | JOIN 基数放大、CTE merge/materialize、window frame 与 `EXPLAIN ANALYZE` 实际执行 |
| [S-DB-07](../middleware/mysql/S-DB-07-financial-schema-locking.md) | 已修正 | 账本平衡必须按 book/asset/单位分域、完整资源锁序、entry/projection/outbox 同事务 |
| [S-DIST-02](../middleware/redis/S-DIST-02-distributed-lock.md) | 已修正 | 租约非绝对互斥、Redis 异步 failover、fencing、SCAN/GET/PTTL 正确命令 |
| [S-DIST-04](../middleware/kafka/S-DIST-04-kafka-semantics.md) | 已修正 | classic/consumer group protocol、cooperative 兼容性、max.poll 非通用 Go 配置 |
| [S-KAFKA-02](../middleware/kafka/S-KAFKA-02-producer-reliability.md) | 已修正 | Java 4.1 默认值边界、协议重试幂等范围、异步错误不可忽略 |
| [S-WALLET-02](../17-multichain-wallet/S-WALLET-02-bitcoin-utxo-psbt-fee-bump.md) | 已修正 | dust relay/economic threshold 分离、PSBTv0/v2、BIP125 policy 边界 |
| [S-WALLET-03](../17-multichain-wallet/S-WALLET-03-solana-account-pda-transaction.md) | 已修正 | account owner 修改规则、status cache miss、blockhash 过期后的历史/业务证明 |
| [S-WALLET-06](../17-multichain-wallet/S-WALLET-06-deposit-sweep-reservation-recovery.md) | 已修正 | 单次 not-found 非不存在证明、同 raw bytes 与不同 attempt 幂等边界、Sui 应用预占键 |
| [S-WALLET-08](../17-multichain-wallet/S-WALLET-08-solana-go-sdk-transaction.md) | 已修正 | 过期重建前的历史核对、`getSignatureStatuses` 查询窗口、Solana 不套用 Sui effects 术语 |
| [S-WALLET-09](../17-multichain-wallet/S-WALLET-09-cosmos-go-sdk-sign-mode-direct.md) | 已修正 | CheckTx/committed execution、ABCI++ 术语、SDK 0.53+ unordered capability |
| [S-WALLET-10](../17-multichain-wallet/S-WALLET-10-aptos-go-sdk-bcs-transaction.md) | 已修正 | v1/v2 与移动分支版本证据、orderless replay nonce/expiration、执行结果 |
| [S-WALLET-11](../17-multichain-wallet/S-WALLET-11-sui-go-capability-adapter.md) | 已修正 | Address Balance 并发额度、gRPC/GraphQL 分工、JSON-RPC 迁移时间与 1.72 事故边界 |

## 第三批必须记住的纠错结论

| 原风险表达 | 安全面试表达 |
|------------|--------------|
| EIP-7702 只是本次交易临时挂载代码 | delegation indicator 是持久账户状态，直到后续授权替换/清除；外层执行 revert 也不会自动撤销已处理授权 |
| `to=nil` 一律表示创建合约 | 是否允许空 `to` 取决于交易类型；EIP-7702 set-code 交易要求非空 destination |
| `eth_getTransactionCount(pending)` 可以给多实例唯一分 nonce | 它只是某个 provider 的本地 pending 视图；生产还需持久 reservation、单写者或线性一致 nonce manager |
| 查历史 logs 就必须用 archive node | logs/receipts 与 historical state 是不同保留维度；history pruning/provider 窗口仍可能删旧 receipt |
| 看见正确 topic 的 event 就能入账，Swap event 价可作 oracle | event 是可重组执行输出；要绑定 chain/address/status/finality/token 语义，单笔成交价也不是抗操纵 oracle |
| 同一 salt 用 CREATE2 就能跨链同地址 | 地址还绑定 deployer/factory 与 init-code hash，并要求目标链具备一致部署条件 |
| ERC-4337 的 UserOp/EntryPoint/SDK schema 可混用 | 必须固定 EntryPoint、ERC-4337/ERC-7769 与链能力；当前规范还涉及 EIP-712 哈希和可选 EIP-7702 authorization |
| `bind.WaitMined` 返回即代表交易成功并最终确认 | 它只等 receipt；还要检查 `receipt.Status`，再按 safe/finalized 策略推进业务状态 |
| dynamic array/mapping 都“不连续且不 packing”，`delete` 会清空内部所有键 | 两者保留基准槽并用哈希寻址，数组元素仍可紧密打包；mapping 不可枚举，删除复合对象不会遍历清除历史 mapping 键 |
| ERC-20 一定有 name/symbol/decimals，ERC-721 tokenURI 是不可变锚点 | ERC-20 三者在原始 EIP 中可选；ERC-721 metadata 扩展可选且 URI 可变，身份需绑定 chain+contract+tokenId |
| OpenZeppelin 5.x UUPS 仍调用 `__UUPSUpgradeable_init()`/`upgradeTo` | 当前 UUPSUpgradeable 是 stateless，公开入口为 `upgradeToAndCall`；不同大版本 API 不能混用 |
| Chainlink feed 都用一个固定 staleness 秒数 | 要绑定具体 feed/资产对/decimals，依据其 heartbeat/deviation 和业务风险定阈值；适用 L2 还需 sequencer uptime 与恢复宽限期 |
| CometBFT 一出现 PoLC 就让全网锁定，remote signer 只存 H/R/S 即可 | 各验证者在 Precommit 步骤本地观察同轮 `+2/3 prevote` 后锁定；H/R/type 是顺序护栏，幂等与冲突检测还应保存 canonical sign bytes 与原签名 |
| finalized 以下在任何情况下都数学上不可变化 | 正常自动路径不得改写；严重共识故障、客户端 bug 或社会恢复属于超出正常假设的人工 fork/incident 决策，必须保留 lineage |

## 第三批 25 篇逐项结果

| 题目 | 结果 | 本轮核对重点 |
|------|------|--------------|
| [S-BC-01](../12-blockchain-web3/S-BC-01-blockchain-evm-basics.md) | 已修正 | EIP-7702 delegation 持久性、非空 destination、fork-aware signer |
| [S-BC-02](../12-blockchain-web3/S-BC-02-go-ethereum-rpc.md) | 复核通过 | RPC read/call/send 分层、historical state 与 logs/receipt 保留边界 |
| [S-BC-03](../12-blockchain-web3/S-BC-03-tx-signing-key-mgmt.md) | 已修正 | pending nonce 不是分配器、signer 版本、KMS/HSM secp256k1 能力验证 |
| [S-BC-04](../12-blockchain-web3/S-BC-04-contract-abi-events.md) | 已修正 | event 非独立状态证明、充值校验与 Swap 成交价/oracle 边界 |
| [S-BC-05](../12-blockchain-web3/S-BC-05-indexer-reorg.md) | 已修正 | `eth_getLogs` provider 限额、receipt/history pruning 与 archive state 分离 |
| [S-BC-06](../12-blockchain-web3/S-BC-06-defi-backend-patterns.md) | 已修正 | feed 身份/heartbeat/deviation、L2 sequencer uptime 与 grace period |
| [S-BC-07](../12-blockchain-web3/S-BC-07-l2-cross-chain-bridge.md) | 已修正 | 桥类型/状态机分层、流动性桥信任面、CREATE2 地址完整输入 |
| [S-BC-08](../12-blockchain-web3/S-BC-08-erc4337-account-abstraction.md) | 已修正 | ERC-4337/ERC-7769/EntryPoint 版本、EIP-712 与 EIP-7702 authorization |
| [S-BC-09](../12-blockchain-web3/S-BC-09-abigen-contract-bindings.md) | 已修正 | abigen legacy/v2 固定、WaitMined status/finality、精确 revert 回放边界 |
| [S-BC-10](../12-blockchain-web3/S-BC-10-mpc-tss-custody.md) | 已修正 | threshold 非业务授权、participant 独立验 intent、session/nonce/presignature 状态 |
| [S-SOLID-01](../13-solidity-contracts/S-SOLID-01-language-storage.md) | 已修正 | dynamic array packing、C3 base-ward 顺序、delete 对 mapping 的边界 |
| [S-SOLID-02](../13-solidity-contracts/S-SOLID-02-security-reentrancy.md) | 复核通过 | OWASP Smart Contract Top 10: 2026、CEI/重入、EIP-6780 SELFDESTRUCT 语义 |
| [S-SOLID-03](../13-solidity-contracts/S-SOLID-03-erc-standards.md) | 已修正 | ERC-20 可选 metadata、ERC-721 身份/可变 URI、unsafe transferFrom |
| [S-SOLID-04](../13-solidity-contracts/S-SOLID-04-upgradeable-proxy.md) | 已修正 | OpenZeppelin 5.x stateless UUPS、upgradeToAndCall/upgradeAndCall 版本 API |
| [S-SOLID-05](../13-solidity-contracts/S-SOLID-05-gas-optimization.md) | 已修正 | unchecked 需证明和 benchmark、CREATE2 salt 抢占成立条件 |
| [S-SOLID-06](../13-solidity-contracts/S-SOLID-06-testing-audit.md) | 复核通过 | unit/fuzz/invariant/fork/audit 能证明与不能证明的范围 |
| [S-SOLID-07](../13-solidity-contracts/S-SOLID-07-defi-patterns.md) | 已修正 | Oracle 身份、staleness 非通用常量、L2 sequencer 恢复边界 |
| [S-SOLID-08](../13-solidity-contracts/S-SOLID-08-contract-go-boundary.md) | 已修正 | tokenURI 非不可变锚、代理 ABI 按生效区块、revert 非稳定业务错误码 |
| [S-PROTO-01](../20-protocol-consensus-security/S-PROTO-01-ethereum-pos-fork-choice-finality.md) | 复核通过 | LMD-GHOST、Casper FFG、justified/finalized 与弱主观性边界 |
| [S-PROTO-02](../20-protocol-consensus-security/S-PROTO-02-bft-cometbft-round-lock-safety-liveness.md) | 已修正 | PoLC 的本地 lock/relock 时点、remote signer 持久证据 |
| [S-PROTO-03](../20-protocol-consensus-security/S-PROTO-03-blob-da-peerdas-security.md) | 复核通过 | Fusaka/PeerDAS 2025-12-03 上线、custody/sampling/归档边界 |
| [S-PROTO-04](../20-protocol-consensus-security/S-PROTO-04-protocol-upgrade-state-migration.md) | 复核通过 | fork 激活、状态迁移、回滚/兼容与治理边界 |
| [S-SEC-01](../21-security-engineering/S-SEC-01-web3-threat-model-iam-trust-boundaries.md) | 复核通过 | 资产/身份/信任边界、human/workload/access policy 分层 |
| [S-SEC-03](../21-security-engineering/S-SEC-03-sbom-provenance-release-admission.md) | 已修正 | SLSA v1.2 Build Track、SPDX 3.0/3.0.1 stable 与 3.1-RC1 pre-release |
| [S-SEC-04](../21-security-engineering/S-SEC-04-security-testing-incident-response.md) | 已修正 | finalized 不变量的正常假设、严重共识事件的人工 fork/lineage 处理 |

## 第一批主要事实源

- [Go 1.25 container-aware GOMAXPROCS](https://go.dev/doc/go1.25#container-aware-gomaxprocs)
- [Go 1.26 release notes](https://go.dev/doc/go1.26)
- [Go 1.26 `sync.Map` source](https://github.com/golang/go/blob/go1.26.0/src/sync/map.go)
- [Go memory model](https://go.dev/ref/mem)
- [PostgreSQL 18 transaction isolation](https://www.postgresql.org/docs/18/transaction-iso.html)
- [PostgreSQL 18 routine vacuuming](https://www.postgresql.org/docs/18/routine-vacuuming.html)
- [PostgreSQL 18 WAL configuration](https://www.postgresql.org/docs/18/runtime-config-wal.html)
- [OP Stack transaction finality](https://docs.optimism.io/op-stack/transactions/transaction-finality)
- [OP Stack protocol differences](https://docs.optimism.io/op-stack/protocol/differences)
- [RFC 9591 FROST](https://www.rfc-editor.org/rfc/rfc9591)
- [EIP-3076 slashing protection interchange](https://eips.ethereum.org/EIPS/eip-3076)

## 第二批主要事实源

- [Go specification](https://go.dev/ref/spec)
- [Go memory model](https://go.dev/ref/mem)
- [Go runtime scheduler source](https://go.dev/src/runtime/proc.go)
- [Go context package](https://pkg.go.dev/context)
- [Go sync package](https://pkg.go.dev/sync)
- [Go atomic package](https://pkg.go.dev/sync/atomic)
- [Go data race detector](https://go.dev/doc/articles/race_detector)
- [Go modules reference](https://go.dev/ref/mod)
- [Go toolchains](https://go.dev/doc/toolchain)
- [MySQL 8.4 clustered and secondary indexes](https://dev.mysql.com/doc/refman/8.4/en/innodb-index-types.html)
- [MySQL 8.4 online DDL operations](https://dev.mysql.com/doc/refman/8.4/en/innodb-online-ddl-operations.html)
- [MySQL 8.4 transaction isolation](https://dev.mysql.com/doc/refman/8.4/en/innodb-transaction-isolation-levels.html)
- [GORM Preloading](https://gorm.io/docs/preload.html)
- [GORM Hooks](https://gorm.io/docs/hooks.html)
- [Redis distributed locks](https://redis.io/docs/latest/develop/use/patterns/distributed-locks/)
- [etcd Concurrency API](https://etcd.io/docs/v3.6/dev-guide/api_concurrency_reference_v3/)
- [Kafka 4.1 Producer Configs](https://kafka.apache.org/41/configuration/producer-configs/)
- [Kafka 4.1 Consumer Configs](https://kafka.apache.org/41/configuration/consumer-configs/)
- [Kafka 4.1 delivery semantics](https://kafka.apache.org/41/design/design/)
- [BIP 174 PSBT](https://github.com/bitcoin/bips/blob/master/bip-0174.mediawiki)
- [BIP 370 PSBTv2](https://github.com/bitcoin/bips/blob/master/bip-0370.mediawiki)
- [Solana accounts](https://solana.com/docs/core/accounts)
- [Solana getSignatureStatuses](https://solana.com/docs/rpc/http/getsignaturestatuses)
- [Cosmos SDK transactions](https://docs.cosmos.network/sdk/latest/node/txs)
- [Aptos Go SDK](https://github.com/aptos-labs/aptos-go-sdk)
- [Aptos orderless transactions](https://aptos.dev/build/guides/orderless-transactions)
- [Sui JSON-RPC migration](https://docs.sui.io/develop/accessing-data/json-rpc-migration)
- [Sui GraphQL RPC](https://docs.sui.io/develop/accessing-data/graphql/graphql-rpc)
- [Sui 1.72 mainnet incident review](https://blog.sui.io/sui-mainnet-halts-resolved-after-major-upgrade/)

## 第三批主要事实源

- [EIP-7702 Set Code for EOAs](https://eips.ethereum.org/EIPS/eip-7702)
- [go-ethereum transaction signers](https://pkg.go.dev/github.com/ethereum/go-ethereum/core/types)
- [Geth history pruning](https://geth.ethereum.org/docs/fundamentals/historypruning)
- [EIP-1014 CREATE2](https://eips.ethereum.org/EIPS/eip-1014)
- [ERC-4337 Account Abstraction](https://eips.ethereum.org/EIPS/eip-4337)
- [ERC-7769 Bundler RPC](https://eips.ethereum.org/EIPS/eip-7769)
- [go-ethereum native bindings v2](https://geth.ethereum.org/docs/developers/dapp-developer/native-bindings-v2)
- [go-ethereum bind package](https://pkg.go.dev/github.com/ethereum/go-ethereum/accounts/abi/bind)
- [Solidity storage layout](https://docs.soliditylang.org/en/latest/internals/layout_in_storage.html)
- [ERC-20](https://eips.ethereum.org/EIPS/eip-20)
- [ERC-721](https://eips.ethereum.org/EIPS/eip-721)
- [OpenZeppelin Contracts 5.x Proxy API](https://docs.openzeppelin.com/contracts/5.x/api/proxy)
- [Chainlink data feed integration](https://docs.chain.link/data-feeds/using-data-feeds)
- [Chainlink L2 sequencer feeds](https://docs.chain.link/data-feeds/l2-sequencer-feeds)
- [OWASP Smart Contract Top 10: 2026](https://scs.owasp.org/sctop10/)
- [CometBFT Byzantine consensus algorithm](https://docs.cosmos.network/cometbft/latest/spec/consensus/Byzantine-Consensus-Algorithm)
- [CometBFT validator signing](https://docs.cosmos.network/cometbft/latest/spec/consensus/Validator-Signing)
- [EIP-7594 PeerDAS](https://eips.ethereum.org/EIPS/eip-7594)
- [Fusaka mainnet announcement](https://blog.ethereum.org/2025/11/06/fusaka-mainnet-announcement)
- [SLSA v1.2 Build Track](https://slsa.dev/spec/v1.2/build-track-basics)
- [SPDX current specifications](https://spdx.dev/use/specifications/)
- [SPDX specification releases](https://github.com/spdx/spdx-spec/releases)
- [NIST SP 800-61r3 announcement](https://www.nist.gov/news-events/news/2025/04/nist-revises-sp-800-61-incident-response-recommendations-and-considerations)

## 下一批审计原则

下一批不按目录平均抽取，而按错误代价排序：资金/签名/共识错误优先于普通知识点，版本敏感
正文优先于稳定概念，角色 P0 优先于 P1/P2。每轮继续保持 20～25 篇，先修正文，再更新本记录，
不通过新增文章掩盖已有内容问题。
