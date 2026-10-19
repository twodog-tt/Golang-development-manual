# 稳定币支付 → DeFi 资金运营端到端实战

把 `stablecoin-ops` 与 `onchain-execution` 的真实进程接起来：SIWE 商户会话、Temporal 审批、带订单引用的 ERC20 付款、Indexer canonical 证据、账本、资金冻结、Uniswap V2 换币、多资产入账和 Outbox/Inbox。两个原有独立验证入口继续保留。本模式不靠 SQL 把渠道改成 confirmed，不把预充值钱包余额当成某张订单已收款。

## 一键验证

需要 Go 1.24.2+、Node 22、npm、Python 3.10+、Docker、**Anvil 1.4.4**。全部命令在仓库根目录运行：

```bash
docker pull postgres:17-alpine
docker pull temporalio/temporal:1.5.1
python3 examples/stablecoin-ops/e2e/verify.py --report /tmp/payment-e2e/result.json
```

运行器使用新的 PostgreSQL 容器（独立 `lab` 支付库和 `chain` 索引库）、Temporal dev server、Anvil Cancun/31337，以及 Go API、provider、worker、gateway 四个进程。随机回环端口、随机本地数据库密码和渠道密钥，不挂载现有数据，不连接公网 RPC。Node harness 自动挖空块来推进演示确认深度。

锁定制品：Uniswap V2 Core 1.0.1、Periphery 1.1.0-beta.0、viem 2.57.7、solc 0.8.30/Paris target。`PaymentReceiver.sol` 和两种合成稳定币 LABUSD/LABDAI（均 6 decimals）实际部署到 Anvil；收款地址是网关签名账户，初始 Token 余额为零。钱包付款后才产生可归属的托管余额。

## 完整交互流程

```bash
# 终端一：启动全新可交互环境；不先执行破坏性故障矩阵
python3 examples/stablecoin-ops/e2e/verify.py --serve
# 终端二：复制运行器输出的真实 lab.env 路径（0600）
source /实际输出路径/lab.env
# 自动完成登录、建单、审批、钱包支付、入账、报价、换币、输出入账
node examples/onchain-execution/client/payment-cli.mjs demo 10
```

逐步运行时，记录 `create` 输出的订单 ID：

```bash
node examples/onchain-execution/client/payment-cli.mjs create order-001 10
node examples/onchain-execution/client/payment-cli.mjs approve PAYMENT_ID
node examples/onchain-execution/client/payment-cli.mjs pay PAYMENT_ID
# 等 show 中 payment.state=settled 后再换币
node examples/onchain-execution/client/payment-cli.mjs show PAYMENT_ID
node examples/onchain-execution/client/payment-cli.mjs convert PAYMENT_ID
node examples/onchain-execution/client/payment-cli.mjs show PAYMENT_ID
node examples/onchain-execution/client/payment-cli.mjs ops
```

`show` 返回原支付、固定 `convert_<payment_id>`、不可变换币参数、实际输出金额字符串及 tx/block/epoch 证据。10 LABUSD 输入不会凭空变成 10 LABDAI 账本余额；以实际 canonical Transfer 输出入账，交易手续费和价格影响反映在兑换结果中。CLI 每次通过公开本地钱包登录，不把 bearer 写入文件。

终端一 Enter/Ctrl-C 结束环境。发生错误也清理本次进程、容器和匿名卷；强杀 Python 或 Docker 异常可能留下资源，按 `knowledge-payment-e2e=<run_id>` 标签定位，只清理该次资源。`--report` 开始删除旧目标，只在所有断言通过后写成功报告；失败日志复制到报告目录，便于 CI 保存诊断。

## 跨服务的事实与提交边界

```text
SIWE / merchant scope
  → payment ID / pay:<id> / approval signal
  → 钱包 approve receiver + pay(bytes32 reference, amount)
  → Paid(reference,payer,amount) + Token Transfer
  → Indexer raw / canonical / checkpoint（独立 chain 库）
  → provider 绑定 reference/token/amount，深度 >= 2
  → ops.events Inbox（包含不可变链上证据）
  → settle:<id> 双分录 + Outbox（lab 库同事务）
  → convert_<id> 参数 + hold:<id>（同事务冻结）
  → Temporal → 持久签名 → 广播 / 原 hash 恢复
  → canonical Swap + 输入/输出 Transfer + 索引水位
  → swap-in:<id> + swap-out:<id> + conversion 状态（同事务）
  → 商户通知 / Inbox 去重 / ACK 后标记 published
```

| 账本 journal | 同资产分录 | 何时允许 |
|---|---|---|
| settle | channel_clearing → merchant，LABUSD | 订单引用、金额、Token、深度与索引证据一致 |
| hold | merchant → held:payment，LABUSD | 已入账、会话商户匹配、同订单只有一个 conversion |
| swap-in | held:payment → dex_input，LABUSD | canonical 成功且已索引输入/输出金额匹配 |
| swap-out | dex_output → merchant:out，LABDAI | 与 swap-in 和 conversion 状态同一个 COMMIT，金额取实际输出 |
| release | 原 hold 的反向分录 | 已索引 canonical reverted；未知状态不释放 |
| undo-out / undo-in / release / reorg | 依序冲正兑换，再冲正原支付 | 已入账支付与兑换都被索引重组移除 |

每种资产各自零和，不能让不同币种金额相加来“平账”。`channel_clearing` 和 `dex_output` 的有限期初余额是会计对手科目夹具，不是钱包资产或偿付能力证明。正常结算阶段，harness 对实际 custody Token 余额与商户可用/冻结/输出余额核对，并单列错误付款的 1 原子单位未归属资金，避免把不明收款计作收入。全链路冲正另外验证钱包与账本余额都回到故障前的值。

不存在跨两个 PostgreSQL 库与 EVM 的原子提交。证据是某个已提交观察版本；观察后仍可能重组。`ChainPaymentWorkflow` 在入账后继续审计，50 轮后 ContinueAsNew 限制历史增长。审计只查询已有事实，绝不重发交易。入账事务持久化 `settlement_evidence`；执行和收款证据按 reference、金额、Token、tx/block/height 比较，同一 epoch 才能组合使用。head 增长不算新事实，同一交易在另一区块重新包含则必须复核。Settle、SubmitConversion、FinishConversion 都重新同步并核验证据，不能把 Temporal 原 Activity 参数当成当前 canonical 事实。

同一商户的冻结、发送授权、换币提交与审计先持 PostgreSQL advisory lock，再锁订单行；gateway 广播也与 nonce 分配、提价、reconcile 共用锁，防止已观察到的 orphaned_review 被并发旧发送绕过。发现旧包含被移除（或 pending nonce 回落到已有包含的位置）时，整个签名账户暂停新建意图、广播和提价；不能简单取数据库 max(nonce)+1 跨过缺口，也不能自动复用旧 nonce。`review_required` 独立持久化：即使原始签名后来又被网络包含，reconcile 记录新事实也不会自动清除签名账户的人工审查门禁；独立 onchain harness 用真实原文重包含验证这一点。锁不能消除链在观察后继续重组的窗口。

旧 `PaymentWorkflow` 和 approval-audit 历史分支保持原路径。新 `ChainPaymentWorkflow` / `ChainReconcileWorkflow` 通过 `chain-monitor-after-notify` / `chain-reconcile-after-notify` 版本标记使初次通知失败后仍继续审计；SDK 测试同时覆盖新路径和 DefaultVersion 的旧分支，不将其表述为全量历史回放或生产滚动升级验收。

## 12 组故障验证

| 组 | 真实注入与判定 |
|---|---|
| 钱包支付与身份/入账 | 8 路同 key 只有一个订单；跨商户请求拒绝；停止 Worker 后付款，删除 RPC 返回的 Paid，恢复 Worker 仍不入账；质量修复后账本/商户 Inbox 各一份；默认模拟 webhook 和退款接口在 chain 模式拒绝 |
| 换币响应未知与进程恢复 | 5 路相同 convert 请求只冻结一次；不同参数 409；先广播成功再丢响应，SIGKILL gateway/worker；重启后仍同 intent/nonce/attempt；缺 Swap 时输出不入账、冻结不释放；恢复后实际输出只入账一次；COMMIT 后响应丢失也不重复 |
| 跨资产数据库故障 | conversion 更新触发器抛错，swap-in/out 和状态一起回滚；进入 review 保留冻结，移除故障后 resume 相同 intent，只产生一套分录 |
| 实际执行失败 | 模拟成功后让高费用市场交易先改变储备，原 swap canonical status=0；索引确认后 release hold，原支付保留 settled，无输出入账和外部退款 |
| 错误付款 | reference 对应订单但原子金额错误，不入账；1 单位单列未归属资金 |
| 已结算后全链路重组 | 快照恢复移除 Paid 和 Swap，另建分支；旧 raw 保留，先冲正输出/输入/冻结再冲正收款；重复审计不重复分录；未解决旧 nonce 时新意图 409，attempt 不增加；通知收件后丢 ACK 仍只有一条 Inbox |
| 对账 | 实际两种 Token custody 余额对应可用、冻结、输出及未归属资金；所有资产 journal 零和，投影无漂移 |
| 通知长时间失败 | 初次 Notify 重试耗尽后移除已入账 Paid；监控仍产生 reorg 分录，恢复通知后 Outbox 继续投递 |
| 输出已消费 | 转走全部输出余额，再移除已结算 Swap；冲正整组回滚，原 settled 分录保留，payment=review_chain；重复审计不增分录，商户其他订单也不能发起换币 |
| 收款分支被替换 | 先 Observe confirmed，再移除 Paid 并在新分支重新付款但仅 1 次确认；Settle 拒绝复用旧 Inbox，不产生 settle 分录 |
| 旧 Activity 参数 | 保存真实成功执行证据，移除 Swap 后直接重试 Finish；旧成功参数及改成 reverted 的失败参数都被拒绝，冻结不释放、输出不入账 |
| 有冻结但无网关意图 | 调用真实 Store 创建 hold、尚未启动转换 workflow，再移除 Paid；execution 404 不挡住审计，进入 review_chain，冻结保留，不能新建或广播 intent |

独立示例的同 nonce 提价、游标失效、坏 ABI、SIWE 负例、退款夹具、旧 Temporal 历史正负回放仍由原先两套验证入口回归，不能用本表冒充那些检查。

## 接口与恢复

支付 API 沿用 SIWE bearer 与商户范围。新增 `GET /payments/{id}/conversion-quote`、`POST /payments/{id}/convert`（`min_out` 字符串、`deadline` 整数）、`GET /payments/{id}/conversion`、`POST /payments/{id}/conversion/resume`。同一订单只允许一个兑换意图，不能改参重试；resume 只处理 review，并保留原意图，不更新过期 deadline。

`CHAIN_GATEWAY_URL` 启用新模式；`PAYMENT_ASSET`、`LEDGER_ASSET`、`OUTPUT_LEDGER_ASSET`、`CHAIN_TOKEN_IN/OUT` 由该次本地部署配置。gateway 新增 `/quote`、`/evidence/deposits/{id}`、`/evidence/executions/{id}`，与原接口一样只用于本地服务。证据包括 reference、tx hash、block hash、金额、已索引高度、epoch；同一 channel sequence 的事件内容固定，不能因 head 增长而改变 Inbox payload。旧 confirmed 再观察到 pending 时返回冲突，不能回传缓存 confirmed 让未达深度的新分支入账。chain 模式 Ingest 的 reversed 只更新渠道观察，跨资产冲正统一走 AuditChain，避免旧单资产路径绕过换币账本。

未知发送、索引故障和未观察到交易不是释放资金的证据。数据库提交失败后的 conversion=review，且 payment 仍为 settled 时，修复依赖后可以 resume 原意图。已入账证据改变、释放依据被重组移除、spend 仍存在或冲正余额不足时，进入独立的 payment=review_chain，保存 audit_evidence，保留 conversion 的实际记账阶段。此状态禁止自动 reconcile/resume，也暂停该商户所有订单的新增冻结和发送；不会通过重复 convert 复活流程。审计在补偿 SAVEPOINT 内原子冲正，任一步失败撤销整组，再持久化人工核对状态。

review_chain 不提供自动解除入口；新增 [人工核对与安全恢复](RECOVERY.md) 提供 inspect → 持久化计划 → 批准执行 → 重启审计 → signer 解封的受控流程和独立 5 组故障验证。需要核对 canonical 分支、原始回执、全部签名 attempt、钱包和账本敞口，再按可恢复分支处理商户。本例不自动生成新 nonce、新订单或改 deadline 绕过审查。后 4 组破坏性测试故意保留待核对敞口，只断言冻结/隔离/无重复记账及投影无漂移；不能把它们称作最终托管资金已经全部匹配。通知单笔按 created_at 投递，商户 Inbox 按 event_id 去重；这不等于多个并发投递者的全局严格有序消费。

## 范围与上线前工作

这是本地集成教学项目，所有私钥为公开测试数据，不能充值真实资产。静态知识站只部署文档，**不部署这些资金 API**。没有真实 USDC/DAI、生产签名托管、外部 PSP、真实预言机或公链 finality 验收。2 块只是业务深度演示，Anvil rollback 不代表真实 PoS/L2 网络测试。

PaymentReceiver 为本课合成 Token 编写，未审计，未在链上注册订单金额/到期/付款人；错误金额可占用 reference，需人工处理未归属资金。它不是通用支付合约，真实 Token 的 fee/rebase/冻结/升级等行为需要专门验证。chain 模式故意禁用旧模拟渠道的退款入口，尚未实现真实链上退款、原生币 gas 会计、部分换币、部分退款、提币和已消费余额的自动冲正。

单节点 logs/receipts 交叉核对不能识别同源一致造假；继续推进需要独立数据来源、终局策略和确认预算。原 API 商户认证不等于分离审批职责，gateway 本地接口也未实现服务身份认证，生产必须补充网络隔离、mTLS、最小权限数据库和签名授权。报告 `integration_harness` 只覆盖本次工具版本与故障矩阵。

## 已有本地数据的兼容处理

运行器每次使用全新数据库，不覆盖现有数据；静态站部署不包含资金数据库迁移。如果自行保留了上一版 lab 数据，需要先备份并为 ops.payments 增加 nullable 的 settlement_evidence / audit_evidence（jsonb）列，再从可验证的原始入账事件恢复原证据。chain 库 intents 还需增加 review_required boolean NOT NULL DEFAULT false，并为已有 orphaned_review 记录设置 true。不能用“当前最新收款”回填原入账证据；缺少原证据的订单应停留人工核对或重建教学环境。旧 ChainPaymentWorkflow 历史走 DefaultVersion 分支，已结束的旧监控不会因升级自动复活，需明确启动新的审计；本次只验证新环境与 SDK 版本分支。
