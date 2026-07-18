---
id: S-SEC-01
title: Web3 威胁建模、IAM 与信任边界
module: security-engineering
level: architect
frequency: 5
go_version: "1.24+"
tags: [security, threat-model, stride, attack-tree, iam, zero-trust, secrets]
status: published
resume_focus: true
code_refs: []
sources:
  - https://cheatsheetseries.owasp.org/cheatsheets/Threat_Modeling_Cheat_Sheet.html
  - https://csrc.nist.gov/pubs/sp/800/207/final
  - https://csrc.nist.gov/pubs/sp/800/53/r5/upd1/final
---

# Web3 威胁建模、IAM 与信任边界

## 30 秒版（开场）

> 威胁模型不是画一张 STRIDE 表，而是为一个确定版本列出资产、攻击者能力、信任边界、入口、滥用路径和必须保持的不变量。Web3 后端尤其要把“读到链数据、批准业务意图、构造交易、签名、广播、确认、账本入账”拆开：RPC、消息队列、CI 和运营后台都不是天然可信。IAM 应区分人、工作负载、签名者和 break-glass 身份，使用短期凭证、最小权限和双人高风险审批；HSM/MPC 保护密钥材料，但不替代交易策略和授权。

## 3 分钟版（一面深度）

一套可评审的威胁模型至少回答：

1. **保护什么**：私钥/份额、用户权益、账本、提现策略、链数据完整性、构建产物和审计证据。
2. **谁能攻击**：外部调用者、恶意用户、被接管的 worker/provider、内部人员、供应链攻击者、控制部分 MPC participant 的对手。
3. **穿过哪些边界**：Internet/API、服务到队列、业务域到 signer、云 IAM 到 HSM、链下到链上、CI 到生产。
4. **失败后果是什么**：盗签、双付、错误入账、隐私泄露、拒绝服务、合规证据缺失，而不只是 CVSS 分数。
5. **如何证明控制有效**：拒绝路径、审计关联、故障注入、恢复演练和 residual risk owner。

STRIDE、攻击树和 abuse case 是发现问题的工具，不是风险已经关闭的证据。

## 10 分钟版（原理 + 图示）

```mermaid
flowchart LR
  User["用户 / 运营"] --> API["API 与 IAM"]
  API --> Intent["不可变业务意图"]
  RPC["不可信 RPC / Indexer"] --> Policy["策略与链状态验证"]
  Intent --> Policy
  Policy --> Signer["隔离 signer / MPC / HSM"]
  Signer --> Broadcast["广播与交易跟踪"]
  Broadcast --> Chain["链上执行"]
  Chain --> Recon["账本 / 对账"]
  CI["源码、依赖、CI"] --> Artifact["签名产物 + provenance"]
  Artifact --> API
```

图中每条箭头都要标注身份、协议、认证、数据完整性、重放边界和失败模式。尤其不能因为流量位于 VPC 内就把它当成可信。

### 先写安全不变量

| 域 | 示例不变量 |
|----|------------|
| 托管签名 | 只签署经过当前策略批准的**准确交易意图**；旧 owner/epoch 永远不能继续签 |
| 充值入账 | orphan observation 不得成为最终可用余额；业务幂等与链 observation identity 分层 |
| 跨链 | destination 只能消费被认证的 source domain/emitter/payload/nonce，且一次 |
| 账本 | 每资产分录守恒；历史只能 reversal/adjustment，不能静默改写 |
| 发布 | 只运行来自允许 source/builder、digest 与 provenance 均验证通过的产物 |

控制是否充分，要看它能否维护不变量，而不是看产品清单是否包含 WAF、KMS 和 SIEM。

### IAM 不是一张 RBAC 表

- **Human identity**：SSO、强认证、JIT elevation、审批时限、会话录制；日常管理员和 break-glass 分离。
- **Workload identity**：按服务和环境发短期身份，不把共享 access key 写进镜像、配置中心或 CI 日志。
- **Signer identity**：调用方身份之外还要验证 intent、policy digest、epoch、request ID 和有效期。
- **Machine-to-machine authorization**：认证“是谁”后仍要判断“可对哪把 key、哪条链、哪种方法、多少额度做什么”。
- **Emergency identity**：封存、定期测试、使用即告警；break-glass 不是永久超级账号。

NIST Zero Trust 的重点是对每次资源访问进行显式决策，不是“什么都不信所以系统无法协作”。网络位置只能作为信号之一。

### 典型攻击树：盗签一笔提现

攻击者可能不需要拿到私钥：

- 接管运营账号后修改 allowlist 或审批规则；
- 控制 RPC，让策略引擎基于错误 chain ID、nonce、余额或合约状态构造交易；
- 重放一次合法审批，但替换 recipient/amount/calldata；
- 利用 coordinator 双主，让旧 leader 继续向 HSM/MPC 请求签名；
- 投毒 builder/decoder，使 UI 显示和实际签名 bytes 不一致；
- 窃取足够 MPC 份额，或诱导合法 quorum 对恶意 intent 协作。

因此控制必须覆盖治理、数据、业务语义和密码学四层。

## 生产场景

一次“新增链/新增 token”评审应更新 DFD、资产和 attacker capability，检查 chain ID、合约地址、decimal、finality、reorg、admin/freeze 权限、SDK 版本、RPC trust 和恢复流程。能力变化不是普通配置变更。

## 排查与工具

审计日志至少能从 `business_intent_id` 关联到 caller identity、审批、policy version/digest、构建的 raw bytes/digest、signer key/epoch、广播 attempt、链上 observation 和账本分录。日志应防篡改并限制敏感数据，不应记录私钥、seed、MPC share 或完整认证 token。

## 架构取舍

集中 policy engine 易于治理，却可能成为高价值单点；分散策略降低中心依赖，却容易版本漂移。常见做法是控制面统一发布签名策略版本，数据面 signer 本地 fail closed，并对高风险操作保留独立限额和 fencing。

## 追问链

1. **做了 STRIDE 就完成威胁模型吗？** → 没有；还要有范围、攻击者能力、风险优先级、控制 owner 和验证证据。
2. **VPC 内请求可信么？** → 不能据此推断；验证 workload identity、请求完整性、授权语义和重放边界。
3. **HSM 能防运营账号被盗吗？** → 只能保护密钥操作边界；若策略允许恶意 intent，HSM 仍可能忠实签名。
4. **服务账号最小权限是什么？** → 不只是 API action，还包括 key/tenant/chain/environment/resource condition 和时限。
5. **威胁模型多久更新？** → 重大架构、链协议、资产、权限、依赖和事故后更新；同时定期复审假设。

## 反模式与事故

- 把“内部服务”“多签”“链上不可篡改”直接等同于可信。
- 所有 worker 共用一个云凭证和 signer client certificate，无法撤销单个主体。
- 审批只绑定订单号，未绑定 recipient、amount、chain、calldata 和 fee ceiling。
- break-glass 账号长期启用且不演练，真正事故时凭证已失效或权限过大。
- 只列漏洞，不写资金、安全和可用性不变量，也不指定 residual risk owner。

## 延伸阅读

- [OWASP Threat Modeling Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Threat_Modeling_Cheat_Sheet.html)
- [NIST SP 800-207 Zero Trust Architecture](https://csrc.nist.gov/pubs/sp/800/207/final)
- [S-SOL-07 安全审计架构](../11-solution-architecture/S-SOL-07-security-audit-architecture.md)
- [S-BC-10 MPC/TSS 托管架构](../12-blockchain-web3/S-BC-10-mpc-tss-custody.md)
