# 人工核对与安全恢复

本指南承接 `review_chain` 和 `review_required`。恢复不是清除标记：先生成持久化计划，人工阅读，再按同一计划执行；执行时再次核对链、nonce、签名原文和账本。只连接回环地址的教学环境，使用已有 pg/viem，无新依赖，无管理 HTTP 写接口。

## 运行完整故障验证

```bash
python3 examples/stablecoin-ops/e2e/verify.py --recovery \
  --report /tmp/payment-recovery/result.json
```

新运行器复用环境构建，但使用全新的容器、数据库、Temporal、Anvil 和服务进程，不承接前一轮破坏性矩阵留下的敞口。默认端到端 12 组、原支付 11 组、原链上 10 组入口继续保留；本入口增加 5 组人工恢复验证。

| 组 | 实际注入及验收 |
|---|---|
| 原分录重新核对 | Swap 重组、输出被占用导致自动冲正失败；归还夹具余额、原始签名被网络重新包含后 rebind。篡改/过期/审批身份冲突拒绝；COMMIT 回包丢失后重试无新分录。Worker 重启、StartWorkflow ACK 丢失仍只有一个真实审计 workflow；余额不符/pending 交易时不能解封 |
| 同 nonce 取消 | 旧 Swap 缺失，计划过期或新分支使计划失效；取消原文先提交，响应丢失后杀 gateway，再通过 RPC 已接收但 ACK 丢失恢复原 hash。只有确认并索引的取消证据允许冲正，低确认深度与输出余额不足均拒绝 |
| 原子冲正及签名恢复 | 更新计划触发器抛错，使 undo-out/in/release 和业务状态整组回滚；移除故障后四路并发批准同计划只生成一组分录。注入审计依赖不可用，真实 Temporal 首次审计耗尽重试后仍不能解封；重启 Worker、修复依赖后同 Workflow ID 新 run 完成审计与 Outbox/Inbox，分录不增加。解封 COMMIT 回包丢失可重试，持续审计不重新隔离同一个有效取消回执 |
| 恢复后实际执行 | 两个数据库和 signer 均恢复后，新订单真实付款并完成 V2 兑换，验证恢复后的账户可以继续工作 |
| 从未创建的意图 | 有 hold 但没有 intent/attempt，Paid 被移除；先在 chain 库封存固定意图 ID，再在支付库冲正。支付 COMMIT 故障后封存仍生效，旧 Create 409；同计划重试完成关闭且分录/通知各一份，封存丢失时拒绝解封，最终托管/账本一致 |

“从未创建的意图”故障验证先等待该计划的 `monitor_started` 和 `monitor_checked` 均为 true，再移除封存行并断言 fence 门禁拒绝解封。商户 Inbox 已收件只证明通知完成，不能替代独立恢复审计完成；这两个前置条件需要分别核对。

[一次本地验证记录](recovery/local-verification.json) 与 CI 制品保留运行证据。报告只在全部断言成功后生成，并记录两模块源码哈希与实际工具版本。本轮所有资产均为本地合成 Token；取消消耗本地测试 ETH，未实现 gas 账本。

## 可交互演练

```bash
# 终端一：构造真实 review_chain + signer 隔离案例，并保留环境
python3 examples/stablecoin-ops/e2e/verify.py --recovery --serve
# 输出 payment/intent 和实际 lab.env 路径。复制到终端二：
source /实际路径/lab.env
node examples/onchain-execution/client/recovery.mjs inspect
```

`lab.env` 权限 0600，包含本次临时数据库凭据、配置路径和公开测试私钥；不要发送或提交该文件。演练中 Worker 暂停，输出占用的夹具已归还，但旧 Swap 仍然 orphaned，账本仍保留原 settled 分录，所以托管/账本暂不匹配。保存运行器输出的 PAYMENT_ID 与 INTENT_ID，依序执行：

```bash
# 生成计划；文件必须不存在。阅读 nonce/hash、签名参数、余额与证据再批准。
node examples/onchain-execution/client/recovery.mjs plan cancel INTENT_ID --out /tmp/cancel-plan.json
cat /tmp/cancel-plan.json
node examples/onchain-execution/client/recovery.mjs apply /tmp/cancel-plan.json --operator alice --ticket LAB-001
# apply 返回广播 hash；终端一首次 Enter 恢复 Worker/挖块，等待确认。
# 再 inspect 确认 execution=cancelled，准备完整冲正：
node examples/onchain-execution/client/recovery.mjs inspect
node examples/onchain-execution/client/recovery.mjs plan compensate PAYMENT_ID --out /tmp/compensate-plan.json
cat /tmp/compensate-plan.json
node examples/onchain-execution/client/recovery.mjs apply /tmp/compensate-plan.json --operator alice --ticket LAB-001
# 等待恢复计划的 monitor_started=true 且 monitor_checked=true，再准备 signer 解封：
node examples/onchain-execution/client/recovery.mjs inspect
node examples/onchain-execution/client/recovery.mjs plan release-signer signer --out /tmp/release-plan.json
cat /tmp/release-plan.json
node examples/onchain-execution/client/recovery.mjs apply /tmp/release-plan.json --operator alice --ticket LAB-001
node examples/onchain-execution/client/payment-cli.mjs demo 1
```

终端一第二次 Enter 清理环境。计划默认 300 秒有效；过期、链分支或资金事实变化时重做 inspect/plan，使用新文件。普通 head 增长不改变金融事实；epoch、receipt/block、nonce、余额、签名 attempt 或业务状态变化会拒绝旧计划。Indexer 落后 live head 时不解除 signer 隔离，可重试同一未过期计划；不要关闭校验。

如果 apply 显示 COMMIT/广播响应未知，重试**原计划文件、原 operator/ticket**；已经应用的计划返回历史执行回执，不会重新记账或签名。这个回执不代表当前仍健康；再次 inspect 才能查看当前事实。取消计划的原 hash 未确认前，资金与 signer 都继续隔离。

### 首次恢复审计失败时

`monitor_started=true, monitor_checked=false` 表示已经启动但尚未通过首次审计，不能据此解封。先检查 Temporal 中 `operator-audit:PLAN_ID` 的 Activity 错误，修复 RPC、Indexer、数据库或商户通知依赖，保持 Worker 运行。调度器会继续扫描尚未完成首次检查的计划：运行中的执行不另开；失败、超时、取消或终止的执行允许在同一 Workflow ID 下启动新 run；成功完成的执行不重开。停止自动重试应停 Worker，而不是清除资金隔离标记。

每个 run 的 Activity 有三次重试；耗尽后调度器仍会恢复未通过检查的计划。Worker 重启不会丢失这个待办。只有实际审计并排空通知成功，才写 `monitor_checked=true`；它不是持续健康探针，解封还要重新核对全局资金与 nonce。不要重新执行付款/兑换或手改这两个标记。故障矩阵用 `chain_audit_unavailable` 数据库夹具注入依赖失败，记录失败 run，并在修复后验证新 run、原计划和原资金分录保持关联。

## 支持的恢复决定

| 动作 | 允许的事实 | 改变 |
|---|---|---|
| rebind PAYMENT | review_chain；原 settle/swap 或 release 分录尚有效；当前收款和执行准确支持原金额、资产、记账阶段 | 更新当前证据，保留原 journal，恢复 settled，持久化审计启动任务 |
| cancel INTENT | 被隔离的支付意图；旧执行 orphaned；该 nonce 是 next unconsumed nonce；未有取消记录 | 同 nonce、给自身 0 value、21000 gas；费用为原最大费用/建议费用的 2 倍且不超过 100 gwei，先持久化原文再广播；保留隔离 |
| compensate PAYMENT | 确认并索引的取消回执，或从未创建且先封存的意图；收款仍有效或明确 orphaned；输出足够完整冲正 | 原子 undo-out/in/release；收款也移除时再 reverse settle，进入 recovered_closed，否则 settled；conversion=recovered_cancelled |
| release-signer signer | 所有相关业务核对完成、审计已持久启动并成功检查一次；所有已分配 nonce 都有已确认的已知 winner；无 pending/缺口/未知消耗；钱包与可用/冻结/输出/未归属资金完全一致 | 记录批准回执后清除 review_required。今后重组会再次隔离 |

不能把“未发现 receipt”当作可以释放资金。取消交易与原交易可能竞争，canonical 已知 winner 决定下一步；原交易先被包含时，取消广播会拒绝，重新核对后考虑 rebind。取消交易首次入库后只重试原计划/hash，本例没有取消交易自动提价、无限尝试或运营级取消 SLA。

不能通过此工具恢复已经冲正后重新出现的成功兑换、改变实际输出金额、覆盖余额漂移或跳过未知 nonce。缺少原始数据库/签名事实、签名账户被其他应用使用、无足够输出冲正、跨资产未解释敞口等情形继续隔离。已经冲正的收款重新出现时也不自动重新入账，需要另外设计带新审计身份的会计调整。

## 提交与互斥边界

- 每个计划存于对应数据库，并绑定 action/target/全部核对事实、过期时间与 SHA-256。批准必须有 operator/ticket；相同计划并发或丢回包重试只能得到同一回执，不允许换审批身份。
- 在线资金路径先拿 ops 维护门的 shared lock，再拿商户/订单锁；人工恢复拿 exclusive 维护门，再拿 chain signer 和 index writer lock。网络读取都设超时，不先拿 signer 再反向等待商户锁。
- 工具验证原文 hash、签名地址、chain ID、nonce、Router calldata、资产路径、原始金额/minOut/deadline。链上证据固定到已提交 canonical 水位，并再次检查锚点仍在当前分支。
- 支付状态、单调递增的渠道观察序号、当前证据、冲正 journal、Outbox、批准记录与 needs_monitor 同一支付库 COMMIT；旧 Inbox 和原分录继续保留，旧 Observe 重试不能覆盖新修订。Worker 用固定 `operator-audit:PLAN_ID` + `AllowDuplicateFailedOnly` 启动 RecoveryAuditWorkflow，未完成首次检查的计划持续调度；正在运行的不另开，失败执行可以新 run 恢复，成功完成的不重开。首次成功审计/排空通知后才写 monitor_checked，再接续只审计的监控流程；启动 ACK 丢失会恢复，不重发支付/换币。关闭案例也启动一次审计以排空 Outbox。
- 跨库没有 ACID。无 intent 时先通过独立提交写 `retired_intents`；即使后续支付事务失败，Create 仍被禁止。重试同计划识别自己的封存行。它是一条保守的单向屏障，不会为了回滚而删除。
- 解封时持两个数据库锁，重新检查全局业务、所有已知 nonce、签名 attempt 和资金对账后，只在 chain 库提交解封与批准回执。取消已确认不会自行解封；解封后同回执不重复隔离，新分支/重新包含会再次隔离。

这仍是本地操作员能力示例：数据库 lab 角色拥有管理权限，operator/ticket 是审计标识，不能冒充生产身份认证、双人审批或不可篡改审计。真实系统需要独立审批身份、最小权限、外部签名托管、不可变审计、终局策略和独立数据来源。两块是教学确认深度，不能据此宣布旧签名在公链永久失效。静态知识站不会部署这些资金服务。

## 旧环境

运行器创建全新 schema。自行保留的旧 lab 环境需备份后增加本次 schema 的 recovery_plans、intent_cancellations、retired_intents，并升级所有参与写入的 gateway/worker；旧进程不认识新屏障，不可混用。没有自动执行现有数据库迁移，也不提供直接 UPDATE review_required/state 的解封脚本。

会话表新增非空 `address` 外键和 `chain_id=31337` 约束；旧会话只有商户，不能可靠回填签名钱包。保留旧数据库时应停 API、撤销旧会话，再按新 schema 迁移并重新登录；不要按商户任意选择钱包回填。新的审计调度策略不改变既有 Workflow 的命令顺序；旧失败审计会以同 Workflow ID 的新 run 恢复。
