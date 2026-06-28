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
---

!!! tip "⭐ 重点准备"
    与 **Digifinex 钱包 / MPC/TSS 提现** 履历高度匹配，见 [Gary 题单](../../resume-focus-gary.md)。

# MPC/TSS 与 CEX 托管签名架构

## 30 秒版（开场）

> CEX 热钱包不能 **单点私钥**：用 **MPC（多方计算）/ TSS（门限签名）** 把密钥拆成碎片，多方协同出签而不还原完整私钥。Go 后端负责 **提现状态机、调用签名服务、广播与确认**。生产关键词：**门限 m-of-n、KMS/HSM、冷签名延迟、审计留痕**。

## 3 分钟版（一面深度）

1. **是什么**：传统单私钥 → 热钱包被盗即全损；MPC/TSS 签名需 m 方参与，碎片分存 KMS/HSM/人。
2. **为什么**：合规与资金安全；Digifinex 类 JD 必问提现链路。
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
| HSM 单机 | 物理隔离，运维重 |
| MPC/TSS | 无完整密钥落地，m-of-n |
| 冷钱包离线签 | 大额、慢，人工或 air-gap |

**Go 侧职责**（不实现密码学，但要讲清边界）：

- 构造 **未签名 tx**（nonce、gas、to、data）
- 调用 MPC **签名 API**（超时、重试、幂等 `withdraw_id`）
- **永不**在业务 Pod 落盘私钥
- 签名结果 **审计日志**：who、when、tx_hash、额度

## 生产场景

- **多链提现**：ETH/ERC20、BTC、TRC20 各自 Signer 适配（[S-EXCH-02](../14-dex-cex-engineering/S-EXCH-02-deposit-withdraw-wallet.md)）
- **大额走冷签**：状态机 `ColdSigning`，人工 + 多签
- **碎片轮换**：定期 key ceremony，旧碎片作废

## 追问链

1. **MPC vs 多签合约？** → MPC 链上仍是普通 EOA 签；Gnosis Safe 多签是链上逻辑，Gas 更高、透明。
2. **签名服务挂了？** → 队列积压、暂停提现、告警；不能降级到本地私钥。
3. **与 [S-BC-03](./S-BC-03-tx-signing-key-mgmt.md) 关系？** → BC-03 讲密钥形态；本题讲 **机构级托管架构**。
4. **TSS 签名延迟？** → P99 数百 ms～数秒；提现 UX 要异步状态 + 推送。

## 反模式与事故

- MPC 碎片与业务 DB 同机备份 → 一起泄露
- 跳过风控直接调 Signer → 内部人盗币
- 不记 `withdraw_id` 幂等 → 重复广播双花提现

## 延伸阅读

- [S-EXCH-02 充提钱包](../14-dex-cex-engineering/S-EXCH-02-deposit-withdraw-wallet.md)
- [S-BC-03 交易签名](./S-BC-03-tx-signing-key-mgmt.md)
