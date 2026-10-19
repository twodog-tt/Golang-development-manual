# Crypto Agent 开放平台本地实战

对应 S-AI-14。Go 1.24.2+、Python 3、Node 22.15+，无新增 Go/npm/Python 依赖；Linux/macOS 文件锁。不连接公链、厂商、生产钱包或真实付款服务。

仓库根目录运行：

```bash
go run ./examples/senior/agentplatform/cmd/demo
go test -race -timeout 60s ./examples/senior/agentplatform/...
python3 examples/senior/agentplatform/wire/verify.py
```

CLI 顺序：不可变制品与部署审批 → 配置替换拒绝 → claim 后停用 → native 插件拒绝/声明式 report → 委托预算耗尽 → 购买冻结版本/调用重试 → 发行回包丢失 → 重建入口查询原凭证 → 流动性和 Runtime 部署 → 发行重组/费用冲正 → 退款评价移除。

预期 `passed=true`、所有检查布尔值为 true、`launch_effects=3`、`fee_before_reorg=10`、`fee_after_reorg=0`、`launch_state="review_chain"`。demo 创建并清理临时目录。教学 Ed25519 种子公开，仅用于本地委托验签，与钱包签名无关。

## 版本、审批、隔离和委托

- `Register` 固定 tenant/version 的 artifact/manifest/Runtime/权限指纹。修改包须新版本；同包的配置/策略/env 变化可以重新审批，无须伪造新包。`Approve` 绑定完整 Binding，`Deploy` 必须精确匹配。`Stop` 清除该版本所有部署审批，Revoked 不可重审；Suspended 的相同制品须新审核。
- Control 仅模拟一个部署槽。每次 Deploy/Stop 提升 generation；Claim 保存绑定，Submit 模拟 **Signer 最终原子准入**，不是只在 Worker 内检查。停止后旧 epoch 无效，已准入的操作保留 UNKNOWN。Cancel 不撤销已签名/广播操作；独立可信 Reconcile 可在撤权后记录匹配 job/binding/generation 的 canonical finalized 回执，但不能重新开放执行。本例没有真实签名器或自动重组监控该 job，链上恢复由 Launchpad 夹具独立演示。
- 管理方法、Principal、当前时间与回执输入都来自可信宿主函数；本例没有 HTTP 登录/OAuth 或生产 IAM。`Binding` 指纹由宿主从真实制品/配置计算，本例不包含制品仓库下载器、secret manager 或审批有效期；生产还须绑定审批者、有效期和凭据版本，跨版本 checkpoint/数据库兼容另做门禁。
- 只接纳 `declarative-v1/report/local-report`，broker 只允许 `read:local-report`。第三方 native 包全部拒绝，避免把进程内库误称沙箱。下面的容器检查是独立的真实隔离配置验证，没有实现任意 ElizaOS 插件运行适配器。
- Delegator 对有界（最多4跳）固定结构 Grant 做真实 Ed25519 验签，签名字节含 `local-agent-delegation:v1` + 零字节域前缀，校验可信 root、签发者链、父摘要、Owner、tenant/aud/resource、权限子集、期限/额度收窄、循环与撤销。它不是 OAuth/JWT/Token Exchange wire 实现。
- 预算在 mutex 内一次性检查并预留所有祖先，intent 固定叶授权、实际调用 scope 与金额，重复请求不再扣预算，换叶/换 scope/换金额拒绝；UNKNOWN 不释放。当前授权、预算、部署、购买和信誉仅驻内存，重启必须重新建立控制状态，不提供跨进程持久预算/权益保证。生产必须使用事务持久化。

## Marketplace 与信誉

Offer 修订不可变；报价只接受已发布的完整 Offer，空对象不能利用 map 零值匹配绕过准入。服务端 Quote 固定买家/租户、卖方、能力/权限 digest、金额、资产域、单位、额度、条款与有效期。新 Offer 不替换旧权益。可信 PaymentReader 须先核验真实支付，再返回绑定完整 QuoteDigest 的证据；不能由 HTTP 客户端填充，也不能只相信链上支付签名。夹具不执行实际收款、金额拆分、退款或订单过期补偿。

付款要求已知的 canonical/final 回执及 tx/event/block，结算身份 `(Domain,Transaction,Event)` 只授予一次权益。Domain 在真实适配器中须完整标识网络/资产/机制，本例用不透明资产域字符串。过期后未完成购买须人工核对，不自动重付。Invoke 每次重验支付，退款/重组/查询错误阻止后续调用；按 quote+intent 原子扣额度，重复 intent 只返回准入成功，不再次扣额度。**Invoke 仅模拟准入，不执行外部副作用；实际交付仍需绑定请求正文/参数、持久工作流与结果缓存**，不能看到 nil 就重跑任意动作。

信誉输入来自可信交付/支付投影，Revision 单调递增，过期观察不能覆盖新状态，相同观察重放幂等。job、完整结算 PaymentKey 与已知经济主体不可替换；同一结算事件不能包装为另一个 job。重复评价不增权，退款/重组/失去确认不计权，恢复后仅恢复原信号，证据不释放。已知相同主体自评拒绝；这里只使用宿主提供的 Actor ID，没有实现地址聚类、真实交付唯一性或防 Sybil 系统，不承诺杜绝自成交。

## Launchpad 的可恢复边界

`Prepare` 将 tenant+launch ID 独占绑定到完整参数（chain/account/token/liquidity/runtime），每步固定 key 与参数摘要。发行/流动性/Runtime 三步共用同一恢复规则：外部调用前先落 UNKNOWN；返回丢失后只 Lookup 原操作。未见、未确认、RPC失败、参数/tx/block不符不能提升成功或重发。即使 Runtime 在发行后失败，也保留之前的资产与步骤，未证明无副作用时停在 UNKNOWN，交人工核对。

launch.json 与独立 remote.json 使用 OS flock、每事务重读、临时文件 fsync/rename/目录 sync；只模拟单机可信目录，不是多主机数据库或链节点。LocalRemote.Submit **故意没有幂等保证**，重复提交会增加 effects，因此恢复测试能检出意外重发；API 的幂等性由 Launcher 的持久状态控制。Lookup 是本地合成的按 operation key 查询能力，不表示 Bankr/ClawPump 或任意链天然提供这一接口。

每次 Advance 前对已执行步骤查证 canonical/finality；revision CAS 阻止旧查询覆盖新状态。匹配原 tx/block 的明确 orphan 证据使流水线转 review_chain，提交隔离/冲正后 Reconcile 明确返回阻断错误，包含最后一步发生重组的情形；查不到或 RPC 错不会冲正。三步完成后仅记录固定 +10 教学费用投影，重组追加一次 -10 冲正，重复事件不重复记账；不是收入计费规则、双分录总账或真实资金验收。已发行资产不自动销毁，未实现 Saga 回购/撤池、人工解封或再发行接口。真实重组仍可能发生在查证与外部执行之间，链上依赖/模拟及敞口管理由真实适配器完成。

## 跨语言编码

`wire/verify.py` 编译 Go CLI，并实际运行 Python 与 TypeScript（Node type stripping，无 tsc/npm），对照提交的 33 组正反向向量。输入是固定六字符串字段的 **local intent-v1**，不是通用 JCS 或完整三语言 SDK。

- 字段按 amount/domain/memo/operation/tenant/version 排序；所有字段必填，禁止未知/重复键、数字/null/对象、非法 UTF-8/孤立代理项及尾随内容。
- amount 为正 uint256 十进制字符串，无正号/前导零；version=1、domain=agent-platform/intent、operation=pay。
- UTF-8 字节≤8192；tenant 1..128 Unicode 码点，memo≤1024；JSON 控制字符转义，HTML/U+2028/U+2029不额外转义，Unicode不做归一化。
- digest=SHA256(`agent-platform:wire:v1` 的 UTF-8 + 一个零字节 + canonical UTF-8)。Go 控制面内部固定结构摘要用于本地绑定/委托，不冒充该 wire profile；生产其他操作须各自定义 schema/domain，不能复用 pay 结构。

## 真实容器负向验证（独立运行）

```bash
# 已有镜像可跳过拉取；验证器自身不拉镜像
docker pull alpine:3.22.1
python3 examples/senior/agentplatform/verify_sandbox.py
```

先将本地镜像标签解析为不可变 image ID，再以该 ID 运行；输出 `passed=true` 和实际 image_id。配置为非 root、network=none、只读根、drop ALL capabilities、no-new-privileges、PID/CPU/内存上限、1MiB noexec/nosuid/nodev scratch。只挂载只读探针，不挂载 secret、Signer/Docker socket；公开测试环境变量不传给容器。

探针真实检查 UID、secret 未继承/挂载与 socket 不存在、根挂载 ro/写入失败、无外部路由与非 loopback 的 UP 接口、网络请求失败、CapEff=0、NoNewPrivs=1、临时盘可写。内核可有 DOWN 的隧道设备，不能把设备名存在误判为网络可用。Docker/镜像不可用直接失败，不算通过；这是该配置的边界探针，不证明内核无逃逸漏洞、资源耗尽测试通过或任意框架插件兼容。

## 故障矩阵

| 测试组（省略 Test 前缀） | 验证 |
|---|---|
| ReleaseApprovalPinsAllInputsAndImmutableVersion | 完整绑定替换、新配置重审、不可变制品、同 digest 恢复、新版本、撤销后回滚拒绝 |
| StopCancelAndABAFenceOldWorkerAtSignerAdmission | 停用、切回同版本 ABA、取消前后副作用边界、撤权后只读回执恢复 |
| ConcurrentStopAndSubmitHasOneAdmissionBoundary | 30轮 Stop/Submit 竞争，旧 epoch 不再准入，已准入的操作保留 UNKNOWN |
| NativePluginRejectionAndBrokerCapabilityBoundary | native、shell、secret、egress、Signer 拒绝；固定报告正例 |
| CommittedWireVectors | Go 直接验收共享正反向向量；三个语言另由 wire/verify.py 共同验证 |
| DelegationSignaturesAudienceAttenuationAndParentRevocation | 真实验签、错签发者/父摘要/tenant/aud/resource、扩权/续期/扩额/Owner/循环/到期与父撤销 |
| DelegationConcurrentChildrenShareAncestorReservation | 32个不同子授权竞争，共享祖先只批准额度8 |
| DelegationCannotMoveAnIntentToAnotherLeaf | 换叶/换 scope/换金额不能重解释已预留 intent，撤销后拒绝 |
| QuotePinsBuyerVersionPricePermissionsAndExpiry | 拒绝未发布/空 Offer，Offer 不可变、新修订不替换权益、买家/租户/到期边界 |
| MarketReceiptFinalityBindingRefundAndReuse | 错资产/Quote、缺tx/event/block、未确认/非canonical/退款，以及同结算事件跨报价复用 |
| MarketConcurrentQuotaAndIdempotentInvocation | 32路最多扣4额度，相同 intent 只扣1 |
| ReputationUniqueEconomicEvidenceRefundAndReorg | job/支付事件唯一、自评/冒名/未支付拒绝，退款/重组失权、恢复不产生新信号 |
| LaunchLostReplyRestartAndNoDuplicateIssuance | 回包丢失、重建 Launcher、多次核对和恢复，全流程 effects=3 |
| LaunchPendingUnseenWrongEvidenceAndUnavailableQuery | 查不到/未确认/错key或参数/缺tx或block/RPC失败不重发 |
| LaunchRuntimeFailureAfterIssuanceDoesNotReplayEarlierSteps | 发行与注资后部署失败，保持 UNKNOWN，不重跑前序、不提前计费 |
| LaunchReorgReversesFeeOnceAndStopsDownstream | 发行重组阻止注资，完成后重组只追加一次冲正，最后一步重组同样返回阻断 |
| LaunchStaleRecoveryCASAndAbsentReceiptCannotReverseCredit | 旧 revision 与错误孤块证据拒绝，查不到不伪造冲正 |
| LaunchConcurrentInstancesAndCorruptStateFailClosed | 24路独立入口共享文件锁、effects≤3，坏库不重置 |
| LaunchRealProcessCrashAfterRemoteCommit | 真实子进程远端落盘后 exit73，重新打开恢复原发行，最终 effects=3 |

共19组顶层测试，另有1个子进程 helper；加33组三语言向量与独立真实容器探针。上述证据不包含真实链、资金、SDK/框架互通、生产 IAM 或外部平台验收。
