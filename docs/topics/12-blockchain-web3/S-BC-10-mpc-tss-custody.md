---
id: S-BC-10
title: MPC/TSS 与 CEX 托管签名架构
module: blockchain-web3
level: architect
frequency: 5
go_version: "1.22+"
tags: [mpc, tss, custody, cex, wallet, hsm, kms]
status: published
resume_focus: true
code_refs: []
sources:
  - https://ethereum.org/en/developers/docs/accounts/
  - https://docs.safe.global/
  - https://csrc.nist.gov/projects/threshold-cryptography
---

!!! tip "相关主题"
    场景地图见 [Web3 交易所与钱包](../../web3-exchange-wallet-focus.md)。

# MPC/TSS 与 CEX 托管签名架构

## 30 秒版（开场）

> CEX 热钱包不能依赖 **单点私钥**：用 **MPC/TSS（门限签名）** 让多个
> key share 协作签名，正常流程不重构完整私钥。Go 后端负责 **冻结交易意图、
> 提现状态机、调用签名服务、广播与确认**。生产关键词：**门限 m-of-n、
> DKG/resharing、KMS/HSM、策略审批、审计留痕**。

## 3 分钟版（精讲深度）

1. **是什么**：传统单私钥一旦泄漏会形成单点风险；TSS 让至少 m 个参与方以 key share 协作产生链可验证的普通签名。新密钥通常用 DKG 生成；导入既有私钥是否曾出现完整密钥，取决于具体导入/分片方案，不能一概宣称“从未重构”。
2. **为什么**：合规与资金安全；热钱包提现链路必须把审批、intent 与门限签名拆开。
3. **怎么做**：提现审核通过 → Signer 服务组装 tx → MPC 集群签名 → 广播 → 状态机追踪。

## 10 分钟版（架构）

```mermaid
flowchart LR
  API[Withdraw API] --> Risk[风控审核]
  Risk --> Signer[Signer Service]
  Signer --> MPC[MPC/TSS Cluster]
  MPC --> Broadcast[链上广播]
  Broadcast --> Indexer[确认监听]
```

| 方案 | 特点 |
|------|------|
| 单热钱包私钥 | 简单，风险极高 |
| HSM/云 KMS | 在硬件或托管边界内保护单个密钥；仍要处理权限、HA 与供应商风险 |
| MPC/TSS | 正常签名时各方持有 key share，支持 m-of-n；门限只说明密码学签名条件，不等于业务授权。协议安全模型、参与方隔离、会话状态和备份同样是风险面 |
| 冷钱包离线签 | 大额、慢，人工或 air-gap |

**Go 侧职责**（不实现密码学，但要讲清边界）：

- 规范化并冻结 **未签名 payload**（chainId、nonce/UTXO、fee、to、value、data）
- 各参与方/策略执行点应从规范化 payload 独立校验 chain、资产、收款方、金额、方法和审批结果；不能只信协调者传来的 digest，否则协调者被攻破后仍可能合法地凑齐门限
- 对 payload 求哈希并持久化策略版本；签名幂等键至少绑定
  `withdraw_id + payload_hash + policy_version`
- 签名会话还要绑定协议版本、key ID/epoch、参与方集合和不可复用 session ID；随机 nonce 或预签名材料必须一次性、原子消费，崩溃恢复不能重复使用
- 调用 MPC **签名 API**；超时后先查询签名会话状态，不能盲目重建 nonce/fee 后再签
- 已得到签名时持久化同一 signed payload/raw tx；重复广播同一 raw tx，而不是重新构造另一笔有效交易
- **永不**在业务 Pod 落盘私钥
- 签名结果 **审计日志**：审批主体、策略版本、payload hash、tx hash、时间和额度；
  日志不得包含 key share、助记词、敏感协议转录或可复用认证材料

## 生产场景

- **多链提现**：ETH/ERC20、BTC、TRC20 各自 Signer 适配（[S-EXCH-02](../14-dex-cex-engineering/S-EXCH-02-deposit-withdraw-wallet.md)）
- **大额走冷签**：状态机 `ColdSigning`，人工 + 多签
- **轮换/resharing**：区分“同一公钥下刷新或更换参与方”和“生成新公钥并迁移资产”；
  两者的链上地址、备份恢复和旧 share 销毁流程不同。DKG/reshare 要有认证信道、参与方身份与 transcript/session 绑定、可恢复的 ceremony 记录和旧材料销毁证明

## 深挖问答

1. **MPC vs 多签合约？** → MPC/TSS 通常为目标链产生原生可验证签名，策略主要在链下；Safe 类多签把阈值与模块策略放在合约执行路径上，权限更易链上观察，但费用、可用能力和风险取决于具体链、合约版本与调用，不能概括成永远“Gas 更高”。
2. **签名服务挂了？** → 队列积压、暂停提现、告警；不能降级到本地私钥。
3. **与 [S-BC-03](./S-BC-03-tx-signing-key-mgmt.md) 关系？** → BC-03 讲密钥形态；本题讲 **机构级托管架构**。
4. **TSS 签名延迟？** → 取决于算法、门限、参与方地域、硬件和审批流，不能承诺固定范围；提现 UX 应使用异步状态机、超时查询与通知。

## 反模式与事故

- MPC 碎片与业务 DB 同机备份 → 一起泄露
- 多个参与方部署在同一云账号、同一管理员或同一恢复密钥下 → 名义 m-of-n，实际仍是共同失效域
- 会话 nonce/预签名材料在超时重试后复用 → 某些门限协议中可直接破坏密钥安全；必须按具体协议实现原子状态机
- 跳过风控直接调 Signer → 内部人盗币
- 只按 `withdraw_id` 幂等、却允许 payload 变化 → 同一业务单可能签出不同 nonce/UTXO/收款参数
- 把“重复广播同一 raw tx”说成一定会双花 → 相同交易通常只是返回已知交易或同一 tx hash；真正风险是重复内部扣账，或重新构造并签出另一笔可生效交易

## 延伸阅读

- [S-EXCH-02 充提钱包](../14-dex-cex-engineering/S-EXCH-02-deposit-withdraw-wallet.md)
- [S-BC-03 交易签名](./S-BC-03-tx-signing-key-mgmt.md)
