---
id: S-PAY-11
title: 稳定币渠道与资金运营：未知状态、Webhook、退款和对账闭环
module: web3-payments-stablecoin
level: senior
frequency: 5
go_version: "1.24.2+"
tags: [stablecoin, channel, webhook, reconciliation, operations, refund]
status: published
reviewed_at: "2026-10-19"
code_refs:
  - examples/onchain-execution/client/recovery.mjs
  - examples/stablecoin-ops/e2e/RECOVERY.md
  - examples/stablecoin-ops/e2e/verify.py
  - examples/stablecoin-ops/README.md
  - examples/stablecoin-ops/verify.py
  - examples/stablecoin-ops/client/src/integration.ts
sources:
  - https://www.rfc-editor.org/rfc/rfc9110
  - https://docs.stripe.com/webhooks
  - https://developers.circle.com/stablecoins
---

# 稳定币渠道与资金运营：未知状态、Webhook、退款和对账闭环

## 30 秒版（开场）

> 渠道请求、链上/支付确认、内部过账和商户通知是不同事实。运营系统应能查清每个支付的当前状态、渠道证据、账本分录与通知进度；提交超时进入未知状态处理，不能自动重新付款。本项目用一个本地渠道夹具串起确认、双分录、商户通知、退款、异常恢复与漂移检测。

## 3 分钟版（精讲深度）

1. **渠道适配**：稳定业务 ID、明确资产身份与整数金额、报价截止时间、状态查询，以及渠道错误映射。超时不能映射为“没有成功”。
2. **事件接入**：校验原始 body 的 HMAC 与时间窗口，再检查 payment、金额、资产和事件身份；同 ID 不同意图冲突，低序号事件不能覆盖新事实。
3. **资金运营**：人工审批、review 查询恢复、退款、通知补发、不可变账本和余额重建，各有独立入口及审计边界。

## 10 分钟版（原理 + 图示）

下表为**独立模拟渠道模式**；其 pending 表示渠道已受理，退款是模拟渠道操作。链上模式的 pending 仅表示等待匹配的链上收款证据，不证明付款已经发生，也不提供下表的退款操作。

| 状态 | 已知事实 | 允许操作 |
|---|---|---|
| created | 本地支付意图存在 | 审批，或等待超时 |
| pending | 渠道请求已受理 | 查询、接受验证后的事件 |
| expired | 审批窗口结束 | 不自动提交渠道 |
| review | 重试/观察窗口结束，外部事实不确定 | 人工发起查询型 reconcile，不自动退款 |
| settled | 确认事实已过账 | 全额退款、通知补发 |
| refund_pending | 退款开始，最终结果尚未确认 | 保持原业务 ID 核对与恢复 |
| refunded | 渠道退款已确认，账本完成反向分录 | 查询与通知补发 |
| review_reversed | 已结算支付发生 provider reversal，内部已冲正 | 人工处置，禁止再发一次退款 |

链上模式另有以下资金恢复状态，不能只按上表操作：

| 对象/状态 | 已知事实 | 允许操作 |
|---|---|---|
| payment=review_chain | 链证据异常或自动补偿失败，原记账阶段保留 | inspect/恢复计划；暂停同商户新增冻结和发送，禁止普通 resume 自动解除 |
| payment=review_reversed | 收款已因重组冲正 | 人工核对；收款重新出现也不自动入账 |
| payment=recovered_closed | 人工核对后完成关闭，原收款仍 orphaned | 查询、审计/通知；收款再出现需另行会计调整 |
| conversion=review | 执行或数据库观察未完成 | 仅 payment=settled 且证据一致时 resume 原意图 |
| conversion=recovered_cancelled | 已知取消或无意图封存支持的人工补偿完成 | 审计与核对；不重建原兑换 |

```mermaid
sequenceDiagram
  participant C as 渠道夹具
  participant A as Go API / Worker
  participant D as PostgreSQL
  participant M as 商户夹具
  C->>A: HMAC 回调 / 状态查询
  A->>D: Inbox + 单调事件快照
  A->>D: journal + entries + Outbox + payment 同事务
  A->>M: 固定 event ID 的 Webhook
  M->>M: Inbox 去重
  M-->>A: ACK（可能丢失）
  A->>D: ACK 后标记 published
```

### 三个幂等边界

- 创建支付：`merchant + Idempotency-Key`；同 key 不同金额/资产返回冲突。
- 外部副作用：渠道以 payment ID 查询/重试，退款仍关联同一支付，不生成新的未知资金意图。
- 内部与通知：`settle:/refund:/reorg:` journal ID；过账与 Outbox 同事务，商户 Inbox 按 event ID 去重。

`confirmed` 是模拟渠道给出的业务确认，不是通用的区块 finality。真实渠道须明确链、token 合约、确认阈值、重组处理和法律/运营结算语义，不能直接复用本地状态名称作为保证。

### 退款与冲正

`refund` 是渠道新资金操作，必须确认后再抵消内部余额；provider `reversed` 是收到的外部逆转事实，内部反向过账并进入 review_reversed。两者不能同时作为新的出款指令。原模拟渠道入口没有用户消费/提现路径，故障夹具保留了足够余额供全额反向；不能据此省略余额检查。链上入口则验证输出被占用时整组补偿回滚并进入人工核对；生产资金可能已被使用，需冻结、负债/损失账户和人工处置。

## 生产场景

- 创建渠道记录后 504：保留相同 ID 重试，或者查询已有记录。
- 回调先后颠倒：记录事件身份，低序号不回退状态；序号只适用于此渠道契约，不能假设所有供应商提供序号。
- 回调完全丢失：Worker 主动查询渠道，不能只依赖 Webhook。
- 通知投递失败：支付仍可已结算；Outbox 保留待发，运营可用 notify 补发。
- 对账出现 +1：报告 drift，暂停自动修复假设，先核对期初快照和不可变分录。

## 排查与工具

`GET /ops` 返回当前商户余额、按期初/分录重建值、drift 和支付/渠道状态；测试还直接检查 journal、Outbox、Inbox。生产需补待处理年龄、投递次数、渠道延迟、积压、退款不确定金额等指标。

`GET /ops` 只检查本地账本投影并展示渠道状态。另一个端到端验证/人工解封入口会核对链上 custody 与商户可用、冻结、输出及未归属金额；这仍不包含银行流水或真实 PSP 结算文件的三方对账。供应商 webhook、分页、错误码、幂等有效期及签名算法都必须按目标厂商的实际文档适配。

## 可运行示例与故障验证

```bash
python3 examples/stablecoin-ops/verify.py --report /tmp/stablecoin-ops.json
```

运行真实 PostgreSQL、Temporal、Go API/Worker 和 TypeScript CLI，验证：报价过期拒绝、提交响应丢失、重复/乱序/伪造/过期回调、金额篡改、提交后 Activity 失败、商户 ACK 丢失、未知状态查询恢复、退款响应丢失、结算后逆转与账本漂移。

交互模式使用 `verify.py --serve`，按项目 README 初始化回环环境，再使用 CLI 的 `create / approve / get / refund / reconcile / notify / ops`。模拟渠道由隔离数据库 fixture 控制，没有连接任何真实厂商或资金账户。

## 链上收款与 DeFi 运营的会计闭环

[端到端入口](S-PAY-08-temporal-money-workflows.md#payment-e2e) 已把原四篇支付项目与交易执行、Indexer、DeFi 三篇接起来。运行器启动独立的支付库和索引库，真实 PaymentReceiver 收款、Uniswap V2 执行；不使用模拟渠道的手工 confirmed SQL。

| 事实 | 账务动作 | 不能做的事 |
|---|---|---|
| Paid 订单引用/Token/金额一致且已索引深度足够 | LABUSD 收款入账 + Outbox | 仅凭钱包总余额认领订单 |
| 已结算订单发起兑换 | merchant → held，参数和冻结同事务 | 并发重复冻结或换请求参数 |
| 广播响应未知 / 索引质量故障 | 保持冻结，查询原 intent/attempt | 将超时当作失败并退款 |
| canonical 成功 + 已索引 Swap/输入输出 Transfer | LABUSD 换出、LABDAI 实际输出换入及状态同事务 | 跨币种相加平账、按报价提前入账 |
| canonical reverted 且已被索引 | 反向 hold 释放；原收款仍 settled | 虚构输出到账或外部退款 |
| 已结算 Paid 与 Swap 被重组移除 | 先冲正输出、换出和冻结，再冲正收款 | 删除旧分录或自动重发新 nonce |
| 收款消失但 spend 仍存在 / 状态不一致 | 保留分录与资金 review，人工核对 | 假定兑换资产已经返回 |

链证据保存在 ops.events / conversion.evidence，冲正观察另存 audit_evidence。持续审计重扫 canonical 数据，只追加幂等反向 journal，不改旧账。通知以原 event ID 重投，审计也扫描未发布的链路 Outbox；收件后丢 ACK 不造成第二次经济动作。

```bash
python3 examples/stablecoin-ops/e2e/verify.py --report /tmp/payment-e2e/result.json
```

12 组故障与 custody 对账见 [完整运行指南](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/stablecoin-ops/e2e)。错误金额的 1 原子单位进入“未归属”核对项，不算商户收入。clearing/dex_output 的有限期初余额是账务夹具，不是偿付证明。chain 模式拒绝旧模拟退款和 webhook，未实现真实退款、部分兑换、提币、gas 费用会计及已消费余额的自动冲正。

## 架构取舍

原模式将业务、模拟渠道与商户 Inbox 放在同一 PostgreSQL；新端到端模式另有独立索引数据库，经 HTTP 与支付系统交互。它可以验证明确的进程与网络响应窗口，不能证明跨数据库、跨机房或跨组织一致性。生产适配器应独立凭据、权限和审计，先完成 sandbox 验收再讨论生产范围。

### 端到端重组恢复的人工边界

端到端实战将数据库暂时失败的 `conversion=review` 与链证据异常的 `payment=review_chain` 分开。后者保存 `audit_evidence`、保留实际记账阶段，暂停同商户新增冻结和发送；普通 reconcile/resume 不得把它恢复为 settled。输出已消费导致余额不足时，补偿 SAVEPOINT 撤销整组冲正，再持久化人工核对状态，不能部分冲正或因 Activity 一直重试而不留下告警状态。完整的 12 组验证见 [端到端运行指南](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/stablecoin-ops/e2e)。

新增 [人工恢复手册](https://github.com/twodog-tt/Golang-development-manual/blob/master/examples/stablecoin-ops/e2e/RECOVERY.md) 与独立 5 组演练：计划绑定资金事实、过期和批准身份，按 rebind/确认取消后原子冲正/无意图先封存分别处理。支付库 COMMIT 同时保存恢复审计任务；签名账户只有在两库核对完成、`monitor_started=true` 且首次审计/通知排空成功的 `monitor_checked=true`、nonce 与托管负债一致后才能受控解封。首次审计耗尽重试时继续隔离，依赖修复后调度器用同一 Workflow ID 重启失败执行；验证还覆盖中途重启 Worker，保证没有新增资金分录。状态恢复与账户解封是两次有审计记录的决定，任何未知敞口继续隔离。


## 深挖问答

1. **为何不把 504 标为 failed 后重建付款？** 对方可能已提交，重建会重复付款。
2. **回调已签名为什么还验金额和资产？** 签名只能证明来源和完整性，不能证明事件属于这笔业务。
3. **通知失败要冲正账本吗？** 不要；通知只是事实传播，资金状态须由真实资金事实决定。
4. **为什么不能在生产直接运行漂移修复 SQL？** 需要写入隔离、可信期初与外部对账证据，不能覆盖资金不一致。

## 反模式与事故

- 一个 success 同时表示渠道、链确认、账本与商户收件成功。
- 接到旧回调后直接覆盖当前状态。
- 商户通知每次重试换新 event ID。
- 超时后无证据地自动退款，或退款重试耗尽就解除资金约束。

## 证据边界与相关链接

原独立模式为单资产、全额退款和模拟渠道；新增模式实际接本地链收款/兑换、双资产账本。两种模式都没有少付/多付自动处置、真实 USDC、银行通道、FX、合规验收或生产资金 SLA。参考项目 [README](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/stablecoin-ops)，配套 [Temporal](S-PAY-08-temporal-money-workflows.md)、[钱包登录](S-PAY-09-wallet-auth-siwe.md)、[SDK 联调](S-PAY-10-go-typescript-web3-integration.md)、[已有账本](S-PAY-07-postgresql-double-entry-ledger-lab.md) 和 [支付状态机](S-PAY-01-payment-state-idempotency-reversal.md)。

## 延伸阅读

- [www.rfc-editor.org](https://www.rfc-editor.org/rfc/rfc9110)
- [docs.stripe.com](https://docs.stripe.com/webhooks)
- [developers.circle.com](https://developers.circle.com/stablecoins)
