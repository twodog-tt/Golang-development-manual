# 概念地图：多链钱包与托管签名

> 5 分钟目标：能画出充提/归集状态机，说清 **谁是事实源**，并区分 **托管信任模型** 与 **MPC 签名实现**。  
> 返回：[概念地图总览](./index.md)

## 0. 这是 CEX，还是 DEX？

**本图默认场景是 CEX 托管资金链路**：用户把币转到平台地址，余额记在内部账本；提现由平台热/冷钱包出签广播。  
**不是** DEX 协议工作（AMM / LP / Router / 用户自签 Swap）——那条线看 [14 DEX/CEX](../topics/14-dex-cex-engineering/index.md)（如 [S-EXCH-06](../topics/14-dex-cex-engineering/S-EXCH-06-dex-amm-liquidity.md)、[S-EXCH-30](../topics/14-dex-cex-engineering/S-EXCH-30-uniswap-v2-v3-protocol.md)）。

| | 本图（CEX 托管钱包） | DEX 协议侧 |
|--|---------------------|------------|
| 资产在哪 | 平台地址池 + 用户账本债权 | 用户钱包或 AMM 池合约 |
| 谁签名 | 平台 MPC/KMS/HSM（热签）或冷签 | 用户本地签，或 Router 代调合约 |
| 主流程 | 充值观察 → 入账；提现审批 → 出签 → 广播；归集 | Swap / 加撤池 / 报价与事件索引 |
| 重叠点 | 都依赖 Indexer、finality、多链差异 | 重叠 ≠ 同一套产品架构 |

去中心化/共管钱包（厂商不能单方面转走）是另一类产品；不要和本图的 CEX 托管热签混称。见 [托管 ≠ MPC](./confusion-cards.md#custody-vs-mpc)。

## 1. CEX 托管钱包架构图

```mermaid
flowchart TB
  subgraph users [用户侧]
    U[CEX App / API]
  end

  subgraph control [控制面 · 该不该动钱]
    Risk[风控 / 审批 / 白名单]
    Policy[Policy + Intent 冻结]
    Ledger[账本 Ledger<br/>可用 / 冻结 / 冲正]
  end

  subgraph observe [观察面 · 链上发生了什么]
    Idx[Indexer<br/>扫块水位 + reorg]
    RPC[RPC / 节点池]
  end

  subgraph wallet [钱包执行面 · 怎么把钱转出去]
    Addr[充值地址池 / HD·memo]
    Hot[热钱包余额水位]
    Cold[温冷钱包 / 大额]
    Adapter[Chain Adapter<br/>按链组交易]
    Reserve[Reservation<br/>nonce / UTXO / object]
    TxMgr[Tx Manager<br/>广播 · UNKNOWN · 替换]
    Signer[Signer Service]
    MPC[MPC / KMS / HSM]
  end

  subgraph chain [各链]
    L1[ETH / TRON / BTC / ...]
  end

  U -->|充值到平台地址| L1
  L1 --> RPC --> Idx
  Idx -->|DepositObserved → Confirmed| Ledger
  Ledger -->|credit 用户可用| U

  U -->|提现申请| Risk --> Policy
  Policy -->|冻结账本| Ledger
  Policy --> Adapter
  Adapter --> Reserve --> Signer
  Signer --> MPC
  MPC -->|已签 raw tx| TxMgr
  TxMgr -->|广播| L1
  Idx -->|出金 receipt| Ledger
  Ledger -->|解冻扣减 / 失败冲正| U

  Addr -.->|余额达阈值| Hot
  Hot -->|归集 sweep| Adapter
  Cold -.->|大额出金| Signer
```

**读图顺序（CEX）**

先分清图上三块职责，再跟三条业务线走：

| 平面 | 图中框 | 只回答什么 |
|------|--------|------------|
| 控制面 | 风控 / Policy·Intent / 账本 | **该不该动钱、账上怎么记** |
| 观察面 | RPC → Indexer | **链上正规链发生了什么**（含 reorg） |
| 执行面 | Adapter → 预占 → Signer/MPC → Tx Manager | **怎么组交易、出签、广播** |

#### 1. 充值（用户 → 链 → 观察面 → 账本 → 用户）

1. 用户把币转到 **充值地址池**（图中 `Addr`；可能是 HD 派生地址或 memo/tag 币）。  
2. 钱到链上后，**不会**立刻改「用户可提余额」：先经 `RPC` 被 `Indexer` 扫到（`DepositObserved`）。  
3. Indexer 按该链 finality（`safe` / `finalized` 或确认数策略）升级到可入账事件。  
4. **账本**追加 credit 分录（可用 ↑）；用户 App 看到的余额来自账本，不是 Indexer 表。  
5. 若之后 reorg：Indexer 回退投影 → 账本用 **冲正分录**，不改历史流水。  

对应图中箭头：`U → L1 → RPC → Idx → Ledger → U`。

#### 2. 提现（用户 → 控制面 → 执行面 → 链 → 观察面 → 账本）

1. 用户发起提现：先过 **风控 / 审批 / 白名单**（大额、黑地址、频控等）。  
2. 通过后冻结 **Policy + Intent**（chain、to、asset、amount、fee 上限、版本等写死），同时 **账本冻结** 占用额度（可用 ↓、冻结 ↑）。  
3. **Chain Adapter** 按目标链能力组未签交易（禁止假想成统一的 `SendTransaction`）。  
4. **Reservation** 预占 nonce / UTXO / object，防止并发双花或崩溃后双发。  
5. **Signer** 只对已校验 Intent 出签；热路径走 **MPC/KMS/HSM**，大额可走 **温冷**（图中 `Cold` 虚线）。  
6. **Tx Manager** 广播：超时先标 **UNKNOWN**，多节点核对后再同 Intent 替换/fee bump，禁止盲目换 nonce 另造一笔。  
7. 链上有明确 receipt 后，Indexer 回报出金结果 → 账本 **解冻并扣减**（成功）或 **解冻/冲正退回**（失败且确认未出金）。  

对应图中箭头：`U → Risk → Policy → Ledger`（冻结）与 `Policy → Adapter → Reserve → Signer → MPC → TxMgr → L1`，再经 `Idx → Ledger → U`。

#### 3. 归集（地址池 / 热钱包 → 执行面 → 链）

1. 充值地址或热钱包余额达到阈值：进入 sweep，而不是当普通「用户提现」。  
2. 常先 **补 gas / Energy**（有 token 无手续费资产时），再 `reserve` → Adapter 组归集交易 → 出签广播。  
3. 归集与用户提现 **争用同一热钱包流动性**：要水位、优先级和熔断，避免「有钱就扫」把提现扫干。  
4. 归集结果仍要对账（地址余额 ↔ 账本/托管总览），失败按 Intent 重试，不另开无关新单。  

对应图中虚线/实线：`Addr -.-> Hot -->|sweep| Adapter`（之后与提现共用预占与出签链路）。

#### 4. MPC 在图里的位置（容易读错）

- MPC/KMS/HSM 只在 **执行面出签框**：解决「热签不要单点私钥」。  
- **不**替代风控、Intent、账本；也 **不**把 CEX 变成「用户自持密钥的去中心化钱包」。  
- 托管模型仍然是：平台按规则可以转走用户托管资产（受审批与额度约束）；MPC 只改变签名怎么凑出来。  

读完全图后自检：能否指着三个平面说出「充值为何先观察再入账」「提现为何先冻 Intent 再出签」「归集为何不是普通提现」。

## 2. 核心对象

| 对象 | 含义 |
|------|------|
| 充值地址 / 热冷钱包 | 观察入账与出金地址池；热路径高频，冷路径高审批 |
| Intent / Policy | 业务授权：chain、to、asset、amount、calldata、fee ceiling、version |
| Signer（MPC / KMS / HSM） | 只对已校验 intent 出签；不替代审批 |
| Reservation | nonce / UTXO / object 预占，防并发双花与崩溃双发 |
| Tx Manager | 广播、UNKNOWN、同 intent replacement / fee bump |
| Chain Adapter | 按链能力矩阵构造交易，禁止统一 `SendTransaction` 幻想 |

## 3. 权威事实源

| 问题 | 事实源 |
|------|--------|
| 链上是否到账 / 是否确认 | **Canonical chain / receipt**（按链 finality 策略） |
| 用户可用余额 | **不可变账本**（只追加冲正，不改历史流水） |
| 该不该转 | **审批 + Policy/Intent**，不是签名集群自己决定 |
| 交易是否已生效 | 多节点核对 txid/nonce/UTXO/receipt；超时先 **UNKNOWN** |

## 4. 主状态机（可手画）

```mermaid
flowchart TB
  subgraph deposit [充值]
    D1[扫块/事件] --> D2[pending]
    D2 --> D3[finality 入账]
    D3 --> D4[reorg 冲正重放]
  end
  subgraph withdraw [提现]
    W1[审批冻结] --> W2[Build+Policy]
    W2 --> W3[Sign MPC/KMS]
    W3 --> W4[广播/UNKNOWN]
    W4 --> W5[receipt 解冻扣减]
  end
  subgraph sweep [归集]
    S1[发现余额] --> S2[reserve]
    S2 --> S3[gas/Energy top-up]
    S3 --> S4[sweep]
    S4 --> S5[对账]
  end
```

## 5. 典型失败模式

| 失败 | 正确处理 | 反模式 |
|------|----------|--------|
| 入账后 reorg | 冲正分录 + 回退投影 + 按新 canonical 重放 | 改历史流水 |
| 广播超时 | 保留 raw tx/intent，核对后再同 intent 替换；见 [UNKNOWN 决策表](../topics/19-node-rpc-staking/S-NODE-05-relayer-transaction-manager.md#unknown-replacement) | 盲目换 nonce 再发无关新单 |
| 有 token 无 gas | 归集队列 top-up + reservation | 当普通提现硬发 |
| 签名方故障 | 会话重开同 intent；已广播走链上核对 | 当作没发生再造一笔 |
| 归集挤兑提现 | 提现水位 + 优先级 + 熔断；见 [争用](../topics/17-multichain-wallet/S-WALLET-06-deposit-sweep-reservation-recovery.md#sweep-vs-withdraw) | 共享一个「有钱就扫」 |

## 6. 易混点（本域）

先读 [托管 ≠ MPC](./confusion-cards.md#custody-vs-mpc)。  
一句话：**用户托管账户** 描述信任模型；**MPC** 描述平台热签怎么出签。

## 7. 推荐阅读（先这几篇）

| 顺序 | 文章 | 证据边界 |
|-----:|------|----------|
| 1 | [充值、提现与链上钱包体系](../topics/14-dex-cex-engineering/S-EXCH-02-deposit-withdraw-wallet.md) | explanation |
| 2 | [充值地址、归集、Nonce/UTXO 预占与恢复](../topics/17-multichain-wallet/S-WALLET-06-deposit-sweep-reservation-recovery.md) | explanation |
| 3 | [MPC/TSS 与 CEX 托管签名架构](../topics/12-blockchain-web3/S-BC-10-mpc-tss-custody.md) | explanation |
| 4 | [MPC DKG、Reshare 与故障恢复](../topics/17-multichain-wallet/S-WALLET-07-mpc-dkg-reshare-recovery.md) | explanation |
| 5 | [Key Ceremony、Signer Fencing 与恢复](../topics/21-security-engineering/S-SEC-02-key-ceremony-signer-fencing-recovery.md) | integration_harness（示例，≠真实 HSM 验收） |
| 6 | [Relayer / Tx Manager](../topics/19-node-rpc-staking/S-NODE-05-relayer-transaction-manager.md#unknown-replacement) | explanation |
| 7 | [多链 Chain Adapter 能力矩阵](../topics/17-multichain-wallet/S-WALLET-01-chain-adapter-capability-matrix.md) | explanation |
| 8 | [Gas/Fee 多链差异](../topics/12-blockchain-web3/S-BC-13-gas-fee-multichain.md) · [TRON/TRC20](../topics/17-multichain-wallet/S-WALLET-12-tron-trc20-resource-transaction.md) | explanation |

专题目录：[17 多链钱包](../topics/17-multichain-wallet/index.md)

## 8. 与相邻域

- 入账观察依赖 [Indexer / 节点数据](./indexer-node-data.md)
- 余额与冲正进入 [交易所资金与对账](./exchange-funds.md)
- Agent 若要动钱，必须经本域的 Policy/Signer，见 [Agent 控制面](./agent-control-plane.md)
- DEX 协议与链上 Swap 不在本图范围 → [14 DEX/CEX](../topics/14-dex-cex-engineering/index.md)
