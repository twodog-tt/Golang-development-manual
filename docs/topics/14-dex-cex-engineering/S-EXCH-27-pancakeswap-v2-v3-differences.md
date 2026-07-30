---
id: S-EXCH-27
title: PancakeSwap V2 与 V3：AMM、流动性与后端集成差异
module: dex-cex-engineering
level: architect
frequency: 5
tags: [pancakeswap, dex, amm, v2, v3, concentrated-liquidity, lp, indexer]
status: published
resume_focus: true
code_refs: []
sources:
  - https://docs.pancakeswap.finance/products/pancakeswap-exchange/faq
  - https://docs.pancakeswap.finance/products/pancakeswap-exchange/pancakeswap-pools
  - https://docs.pancakeswap.finance/trade/pancakeswap-exchange/trade
  - https://github.com/pancakeswap/pancake-smart-contracts
  - https://github.com/pancakeswap/pancake-v3-contracts
---

# PancakeSwap V2 与 V3：AMM、流动性与后端集成差异

<a id="oral-card"></a>

## 要点卡

[返回模块索引](./index.md)

!!! abstract "30 秒回答"

    PancakeSwap V2 是全价格区间的恒定乘积 AMM：同一 Factory 下每个 token pair 通常只有
    一个 Pair，LP 份额是可替代的 ERC-20 LP token，报价主要读取两边 reserve。V3 把流动性
    集中到 LP 自选的 tick 区间，同一 token pair 可以按 fee tier 建多个 Pool，LP 头寸由
    Position Manager 表示为 NFT；报价要沿 `sqrtPriceX96`、当前 active liquidity 和已初始化
    tick 逐段计算。V3 在活跃区间内资本效率更高，但出区间后只剩单边资产且不赚 swap fee，
    还增加了选区间、收取费用、再平衡和索引复杂度。因此 V3 不是无条件优于 V2，Launchpad
    选型要看初始价格、长尾币波动、LP 运维能力、token 行为和目标链实际部署版本。

**3 分钟展开**

1. **池模型**：V2 的池键主要是 `token0 + token1`；V3 是
   `token0 + token1 + fee`，一个交易对可有多个费率池。
2. **流动性**：V2 资金覆盖近似全价格区间，LP token 同质化；V3 position 绑定
   `fee + tickLower + tickUpper`，通常由 ERC-721 NFT 表示。
3. **报价状态**：V2 看 `getReserves()` 并套含费恒定乘积公式；V3 看当前平方根价格、
   active liquidity 和 tick，跨 tick 时流动性会变化，不能用池子总余额直接报价。
4. **收益与风险**：V2 的 LP 费用进入池中、效果上自动复投；V3 仅 in-range 头寸赚费，
   费用单独累计，需要 collect，默认不会自动复投。
5. **工程差异**：Indexer 从 V2 的 Pair/`Sync` 模型升级为 V3 的 Pool + tick + position NFT
   模型；路由器还可能混合 V2、V3 和 StableSwap，不能只索引一种池。

| 记忆槽 | 内容 |
|--------|------|
| 一句话 | V2 是“一个 pair、一条全区间曲线、同质化 LP”；V3 是“pair + fee 多池、分段集中流动性、NFT position” |
| 三个 V3 状态 | `sqrtPriceX96`、active liquidity、initialized ticks |
| 项目落点 | Launchpad 毕业迁池时，V2 简单被动；V3 资本效率高但必须设计初始价、fee tier、区间、NFT 托管和再平衡 |
| 一个边界 | V3 仍基于恒定乘积思想和虚拟储备，不是“完全不用 `x·y=k`” |

**错误表达**

- ❌ “V3 抛弃了恒定乘积公式，所以和 V2 是完全不同的定价模型。”
- ✅ “V3 把恒定乘积流动性切成 tick 区间；区间内可用虚拟储备理解，跨 tick 时 active liquidity 改变。”
- ❌ “V3 一定比 V2 滑点低、收益高、Gas 低。”
- ✅ “只有当前价格附近存在足够 active liquidity 时，V3 才可能以更少资本提供更深深度；收益和 Gas 取决于区间、tick crossing、路由和操作类型。”

**自测追问**：如果 Launchpad Token 毕业到 V3 后价格迅速越过 LP 区间，用户还能否正常卖出？系统如何恢复有效双边流动性？

## 30 秒版（开场）

> PancakeSwap V2 是全价格区间恒定乘积 AMM，一个 token pair 通常对应一个 Pair，LP 份额
> 是 ERC-20 LP token；V3 使用按 fee tier 划分的集中流动性池，LP 自选 tick 区间并持有
> position NFT。V3 在活跃区间资本效率更高，但出区间不赚费且会变成单边资产，后端还要
> 索引 `sqrtPriceX96`、active liquidity、tick、fee growth 和 NFT position，不能继续只靠
> `getReserves()`。因此 Launchpad 迁池要在资本效率与运维复杂度之间做选择。

## 核心差异表

| 维度 | PancakeSwap V2 | PancakeSwap V3 |
|------|----------------|----------------|
| AMM 形态 | 全价格区间的经典恒定乘积池 | 集中流动性，把资金配置到一个或多个 tick 区间 |
| 池唯一键 | 同一 Factory 下通常为 `token0 + token1` | `token0 + token1 + fee`，同一 pair 可有多个费率池 |
| LP 凭证 | 可替代 ERC-20 LP token；同池份额可直接按数量合并 | 每个 position 的 fee/range/liquidity 可不同，Position Manager 通常以 ERC-721 NFT 表示 |
| 流动性状态 | 主要读取 `reserve0/reserve1` | `sqrtPriceX96`、当前 tick、active liquidity、ticks、fee growth |
| 费用档位 | PancakeSwap EVM V2 当前官方页面标示固定 0.25% | PancakeSwap EVM V3 当前官方页面列 0.01%、0.05%、0.25%、1%；应以目标链部署为准 |
| LP 费用 | LP 部分进入池储备，体现在 LP token 可赎回价值中 | 只分给成交路径覆盖到的 active position，单独累计，默认不自动复投 |
| 价格越界 | 没有 LP 自定义区间；随着价格移动仍沿全区间曲线交易，但深端滑点可极高 | position 出区间后变成单边资产、不参与交易且不赚 swap fee；其他 in-range position 仍可提供流动性 |
| 资本效率 | 被动、简单，但大量资金分布在当前价格很远的位置 | 当前价附近可形成更深流动性，但需要选区间和再平衡 |
| 报价方法 | reserve + 固定费率公式 | 按 sqrt price、liquidity 和 tick 逐段模拟；跨 tick 时更新 active liquidity |
| 索引重点 | Factory `PairCreated`；Pair `Mint/Burn/Swap/Sync` | Factory `PoolCreated`；Pool `Initialize/Mint/Burn/Collect/Swap`；Position Manager NFT 生命周期 |
| Token 兼容 | 部分 Router 路径专门支持 fee-on-transfer，但仍需按真实到账量处理；rebase 仍有额外风险 | 官方文档明确提示 EVM V3 不支持 fee-on-transfer 和 rebase token |
| 运维复杂度 | 低；适合简单 LP 锁仓、燃烧或按 LP token 管理 | 高；要管理 tokenId、区间、collect、decrease/increase、NFT 授权和再平衡 |

!!! warning "适用范围"

    表中的费率是 **2026-07-21 查阅的 PancakeSwap EVM Exchange 官方页面口径**，不是所有链、
    所有历史版本或 PancakeSwap fork 的永久常量。PancakeSwap 现在还存在 StableSwap、
    Infinity 等其他池型；回答 V2/V3 时不要把整个 PancakeSwap 协议简化成只有两种池。

## 1. V2：全区间恒定乘积与 LP token

V2 Pair 维护两种 token 的储备。按 PancakeSwap V2 当前固定 0.25% 输入费率，标准 token 的
简化 exact-input 报价可写成：

```text
amountInWithFee = amountIn × 9975
amountOut = amountInWithFee × reserveOut
            / (reserveIn × 10000 + amountInWithFee)
```

这是整数公式，必须明确：

- 使用 swap 前的 canonical reserves；
- Solidity 整数除法会向下取整；
- 多跳交易要逐池计算，每一跳都收取对应池费用；
- fee-on-transfer token 的实际到账量可能不等于用户输入量；
- 不能把后端计算结果当成成交承诺，链上仍需 `amountOutMin` 和 deadline。

V2 LP 的特点：

- 同一个 Pair 的 LP token 是同质化份额；
- 增加/移除流动性按池比例处理两种资产；
- LP 的 swap fee 部分加入池，LP 不需要逐笔 `collect`；
- 价格远离初始点时，LP 组合会自动偏向相对变弱的一侧资产，并承受相对持币的无常损失。

## 2. V3：集中流动性、tick 与 NFT position

V3 LP 选择 `[tickLower, tickUpper]`。当前价格在区间内时，该 position 提供 active
liquidity 并按份额赚取费用；价格越界后，position 主要变成其中一种资产，不再提供 active
liquidity，也不再赚 swap fee。价格回到区间后可重新激活，不一定要重新 mint。

V3 常见价格表示：

```text
rawPrice(token1/token0) = (sqrtPriceX96 / 2^96)^2
humanPrice = rawPrice × 10^(decimals0 - decimals1)
tickPrice ≈ 1.0001^tick
```

`token0/token1` 排序和 decimals 换算方向非常容易写反，后端应该用官方 SDK/经过测试的定点数
实现，并准备双向价格 golden case，而不是使用 `float64`。

V3 报价不能只看 Pool 合约中的两种 token 余额。原因是：

1. 只有当前价格附近的 active liquidity 参与本段成交；
2. Swap 推动价格到下一个 initialized tick 时，要结算本段并更新 active liquidity；
3. 之后继续下一段，直到输入耗尽或触达用户给定的价格边界；
4. fee tier 是池身份的一部分，`100/500/2500/10000` 分别对应当前 EVM 常见的
   `0.01%/0.05%/0.25%/1%`。

因此讲解时可以说 V3 使用“分段集中流动性 + 虚拟储备”的恒定乘积数学，但不要把它简化成
V2 的一条 reserve 公式。

## 3. 后端与 Indexer 如何分别接入

### V2 数据模型

```text
pool_key = chain_id + factory + token0 + token1
event_key = chain_id + tx_hash + log_index
```

至少维护：

- Pair 地址、token0/token1、decimals；
- canonical `reserve0/reserve1` 和最近同步区块；
- Mint/Burn/Swap/Sync 事件；
- LP totalSupply、用户 LP 余额或协议托管余额；
- volume、fee、TVL 和 K 线投影。

### V3 数据模型

```text
pool_key = chain_id + factory + token0 + token1 + fee
position_key = chain_id + position_manager + token_id
event_key = chain_id + tx_hash + log_index
```

至少维护：

- Pool 的 fee、tickSpacing、`sqrtPriceX96`、current tick、active liquidity；
- initialized ticks 的 liquidityGross/liquidityNet 与相关 fee growth；
- position NFT 的 owner、pool、tickLower/tickUpper、liquidity、tokensOwed；
- Mint/Increase、Decrease/Burn、Collect、Transfer、Swap 等事件；
- NFT 质押到 Farm/Manager 后的实际 owner 与受益人映射。

!!! danger "不能直接复用的 V2 假设"

    V3 没有一个可直接替代 `getReserves()` 的“全池有效储备”。用 ERC-20
    `balanceOf(pool)` 计算报价，会把不在当前区间内的资金也误当成可成交深度。生产报价应调用
    Quoter/Smart Router 或使用经过一致性测试的 tick simulator，并在固定 block tag 上计算。

两类索引都必须保留 block number/hash/parent lineage，在 reorg 时撤销 orphan event 并重建
投影，不能因为事件已有唯一键就忽略重组。参见
[S-BC-05](../12-blockchain-web3/S-BC-05-indexer-reorg.md)。

## 4. Launchpad 毕业迁池怎么选

| 场景 | 更倾向 V2 | 更倾向 V3 |
|------|-----------|-----------|
| 新发长尾币、价格发现高度不确定 | 是：全区间、运维简单，不会因项目方设错窄区间立即失活 | 风险较高；必须有宽区间/多区间与补流动性策略 |
| 稳定币或成熟高成交 pair | 资本效率较低 | 可在窄区间形成更深流动性，但需要持续管理 |
| LP 锁仓或永久流动性 | LP token 管理简单 | position NFT 的托管、授权和受益权更复杂 |
| 项目没有专业做市/再平衡能力 | 更合适 | 容易长期 out-of-range |
| 要按波动选择 fee tier | 固定费率，选择空间小 | 同 pair 多 fee pool，可匹配不同波动与订单流 |
| Token 带转账税或 rebase | 仍需专项兼容和真实到账核算 | PancakeSwap EVM V3 官方明确不支持，不应迁入 |

Launchpad 迁到 V3 时还要额外设计：

1. **初始价格**：V2 由首笔储备比例形成；V3 要显式初始化 `sqrtPriceX96`。创建、初始化、
   mint 最好在受控原子流程中完成，避免未初始化池或错误价格窗口。
2. **区间策略**：不能只给一个很窄的营销型区间；要按最大预期波动、库存和补仓能力设计
   base range/宽区间，必要时组合多个 position。
3. **NFT 权限**：明确 tokenId 由 timelock、多签、Vault 还是 LP 用户持有；谁能 decrease、
   collect、transfer 和 rebalance。
4. **费用归属**：V3 fee 单独累计，collect 后进入哪里必须可审计；不能沿用 V2“储备自然增长”
   的账务假设。
5. **路由与行情**：毕业后可能同时存在内盘、V2、多个 V3 fee pool；价格和成交量要按来源标识，
   防止重复计数和低流动性池污染主价格。

## 5. 安全与生产检查

- **Factory/Pool 验证**：使用目标链官方 Factory、Position Manager 和 Router 地址；验证
  `(token0, token1, fee)` 对应的真实 Pool，不能只相信前端传入地址。
- **Callback 验证**：V3 swap/mint callback 必须验证调用者确实是官方 Factory 推导出的 Pool，
  防止伪造 callback 拉走 token。
- **初始价与 decimals**：初始化前校验 token 顺序、decimals 和预期价格；用独立实现做
  round-trip 测试。
- **滑点与价格边界**：V2 设置 `amountOutMin/amountInMax`；V3 还可使用
  `sqrtPriceLimitX96`，但不能用零边界代替业务风控。
- **Tick crossing 压测**：大额 Swap 可能跨越大量 initialized ticks，Gas 与报价延迟取决于
  实际池状态，不能给出“V3 永远更省 Gas”的结论。
- **NFT 授权**：Position Manager 的 operator approval、permit、Farm 质押与 NFT 转移都应
  进入权限审计和资产盘点。
- **多 Router 共存**：前端/后端必须展示最终 calldata、route、fee 和 min-out；Smart Router
  可能混合 V2/V3/Stable 路径。

## 深挖问答

1. **V3 为什么资本效率高？** → 同样资金集中在当前成交区间，active liquidity 更深；代价是
   out-of-range、单边库存和再平衡成本。
2. **V3 还能用 `x·y=k` 吗？** → 可以用分段虚拟储备理解，但报价必须处理 sqrt price、tick、
   active liquidity 和 tick crossing，不能套 V2 reserve 公式。
3. **同一 pair 为什么有多个 V3 Pool？** → fee 是 Pool key 的组成部分；不同波动和订单流可选择
   不同费率，路由器比较扣除 fee/Gas 后的可执行结果。
4. **V3 position 为什么是 NFT？** → 每个 position 的 pool、fee、tick 区间、流动性和 fee
   快照可能不同，不能像同一个 V2 Pair 的 LP token 那样完全同质化。
5. **V3 出区间是不是资产丢了？** → 不是；position 通常变成单边资产并停止赚费，价格回区间可
   再激活，也可 decrease/collect 后重建区间。
6. **从 V2 迁 V3，Indexer 改什么？** → pool key 加 fee，增加 tick/active liquidity、NFT
   position 与 collect 账务；保留 canonical event/reorg 处理，双读对账后再切主价格源。
7. **Launchpad 为什么可能继续用 V2？** → 长尾币初期价格范围未知、没有专业再平衡者时，V2
   的全区间和同质化 LP 更容易锁仓、审计和运维；这不是“技术落后”，而是风险取舍。

## 反模式

- 用 `balanceOf(pool)` 代替 V3 active liquidity 和 tick 模拟；
- 把 fee tier 只当展示字段，索引键中没有 fee，导致多个 Pool 相互覆盖；
- 把 V3 NFT 当前 owner 直接当最终受益人，忽略 Farm/Vault 质押和代理管理；
- 只监听 V3 Pool `Swap`，不处理 Collect、position liquidity 变化与 NFT Transfer；
- Launchpad 创建 V3 Pool 后分多笔交易初始化和注资，暴露抢先初始化/错误价格窗口；
- 宣称窄区间一定提高收益，却不披露 out-of-range、无常损失、Gas 和再平衡成本；
- 背诵某个历史费率拆分，把它说成所有链和所有 PancakeSwap 版本的永久规则。

## 延伸阅读

- [PancakeSwap Trading FAQ：V3 集中流动性、fee tier 与 token 限制](https://docs.pancakeswap.finance/products/pancakeswap-exchange/faq)
- [PancakeSwap Liquidity Pools：V2/V3 LP 与费用](https://docs.pancakeswap.finance/products/pancakeswap-exchange/pancakeswap-pools)
- [PancakeSwap Token Swaps：当前 EVM V2/V3 费率](https://docs.pancakeswap.finance/trade/pancakeswap-exchange/trade)
- [PancakeSwap V2 合约仓库](https://github.com/pancakeswap/pancake-smart-contracts)
- [PancakeSwap V3 合约仓库](https://github.com/pancakeswap/pancake-v3-contracts)
- [S-EXCH-06 DEX AMM、流动性池与 LP 收益](./S-EXCH-06-dex-amm-liquidity.md)
- [S-EXCH-07 DEX 聚合路由、滑点与 Gas](./S-EXCH-07-aggregator-slippage.md)
- [S-EXCH-12 Token 发行、毕业与迁池](./S-EXCH-12-token-launch-rebate.md)
