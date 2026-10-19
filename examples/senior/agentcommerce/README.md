# x402 与 ERC-8183 Commerce 本地恢复实战

对应 S-AI-13。Go 1.24.2+，复用根模块已有 go-ethereum 和 agentmemory 本地授权夹具；Linux/macOS文件锁，无公链、资金、真实Facilitator、厂商调用或模型密钥。

```bash
go run ./examples/senior/agentcommerce/cmd/demo
go test -race -timeout 60s ./examples/senior/agentcommerce/...
```

CLI：真实EIP-3009签名 → 请求正文替换拒绝 → 结算回包丢失 → UNKNOWN拦截重试 → 重建Engine/查询独立账本恢复 → 缓存交付 → 三种flow的本地顺序 → 预算变化拒绝 → 到期绕过失败Hook退款。
预期 `passed=true`、`settlements=1`、`ledger_entries=1`、`unknown_blocked_retry=true`、`budget_change_rejected=true`、`expiry_refund=20`。临时状态由demo创建并清理。演示用公开教学种子，禁止给其钱包转真实资产，不输出私钥。

## 协议与夹具分开

固定参考 [x402 revision faa1c694](https://github.com/x402-foundation/x402/tree/faa1c694c66a3732aaabbce93bdae140e521a67d/specs)（2026-10-15），复核2026-10-19；该规范的HTTP v2为base64 JSON头PAYMENT-REQUIRED、PAYMENT-SIGNATURE、PAYMENT-RESPONSE。这里实现的是**支付/请求授权与恢复控制面**，没有HTTP x402 wire parser/server、官方SDK兼容、RPC模拟或真实token合约验收。

`Mechanism`包含version/scheme/network/asset/facilitator/transferMethod/flow。实际付款Engine只接受v2、exact、eip155:84532、固定本地token、local-chain-v1、eip3009、authorization；仅该组合可省略transferMethod/flow并采用本地固定默认。未知flow/Permit2/其他网络/厂商均拒绝，不能套用这些默认到任意机制。

`FlowStages/RunFlow`为**独立、本地、无资金的编排夹具**，说明三类ordering：

| flow | 顺序 |
|---|---|
| authorization | verify → resource → settle |
| upfront | settle → resource |
| escrow | deposit settle → resource → capture settle |

每个失败点立即停止，不继续后续阶段。这不表示exact/EVM支持任意flow，也不提供upfront/SVM/escrow持久结算恢复；分阶段网络适配未验收时必须拒绝接入，不能重跑RunFlow冒充恢复。支付Engine的持久恢复仅覆盖其已声明的exact/EIP-3009/authorization组合。

## 钱包签名不是资源授权

`PaymentDigest`使用真实EIP-3009 TransferWithAuthorization EIP-712结构，domain固定为Local Fixture USD/v1、chain84532和固定token。from/to/value/validAfter/validBefore/32-byte nonce均参与签名，不包含HTTP租户/路径/正文。
`VerifyPayment`真实恢复EOA签名；仅整数最小单位10，禁止改收款人、换链/资产、过期或未生效。没有余额/allowance/合约simulate、Permit2、委托钱包或生产资产支持。

可信宿主根据已认证的scope/principal和实际请求字节调用 `NewBinding`，把tenant/subject/intent/method/resource/bodySHA256绑定到存储。Prepare须当前pay角色、匹配scope.SubjectID与配置payer，固定资源为https://reports.fixture/report、POST；不能让模型或普通HTTP客户端自己填写bodyHash/身份。

Execute对**实际请求**重建Binding并与持久记录全等比较。外租户、换subject/intent/path/method/body全部拒绝；签名在密码学上仍有效也不会获得业务授权。此处的tenant/scope是本地agentmemory凭据夹具，不是链上钱包登录或生产IAM。

重建Engine必须配置状态目录、付款账户；执行/恢复还须有Chain适配器，付款账户与原记录不符时拒绝，不能跨签名账户恢复。每次操作都检查当前grant；持久主体绑定tenant/subject/policy和完整scope，未绑定进程随机issuer，以便可信入口重启后恢复同一主体。真实入口必须认证同一issuer/组织并载入持久策略；不能从任意自建Authority或不可信请求接纳身份。撤权立即拒绝后续读写/交付，策略变化需显式重审，本例未提供越权“管理员恢复”入口。

## 幂等、UNKNOWN 与账本

Prepare将应用intent与 `(network,token,authorizer,authorization nonce)` 独占绑定，不能用换JSON排版/新intent绕过去重；该nonce不是Facilitator广播账户的交易nonce。最多128个intent。

payments.json用跨实例OS锁、每事务重读、临时文件fsync/rename/目录sync持久化，完整性错误不初始化为空库。调用外部结算前先落UNKNOWN及确定性的本地报告结果；网络调用在锁外，重复UNKNOWN调用一律阻断。报告生成是固定纯函数，不执行外部昂贵工作；真实业务工作仍需要独立可恢复的工作流/Outbox。

LocalChain在独立chain.json中保存结算凭证/nonce和effect计数，可注入“广播后回包丢失”；它不是EVM或真实资金账。结算回复里的success=false/超时不能推出未付款。Engine不自动换签名、不自动再次结算；Recover仅查询可信Chain适配器，恢复后返回原报告。

Observation必须匹配授权digest/network/token/from/to/value/nonce，并有已知、canonical、finalized、非零tx/block证据。对于尚未证实的付款，unseen、pending、RPC错误和非canonical证据均不能把UNKNOWN提升为成功；只有明确的canonical finalized canceled证据可转失败。实际RPC还须区分已消费/已取消授权，核对token日志与最终性，不能只看authorizationState=true。夹具用typed Observation模拟该差别，不声称提供通用Facilitator查询API。

内部Entry每条表达等额debit/credit资金腿；唯一effect的重复恢复不重复入账。已记账支付发生reorg时追加merchant→payer冲正，阻断新交付；重新确认在新block后追加新结算。相互竞争的恢复按单调递增revision及先前state/receipt做CAS；即使状态/回执变化后又回到相同值（ABA），旧查询也不能覆盖新恢复。已经paid的记录，暂查不到/未确认/缺少tx或block不是冲正证据；必须收到匹配原tx/block的已确认非canonical证据，才能按重组冲正。Result依据保存的结算凭证和当前grant，不承诺每次实时查链；RPC不可用本身不作为冲正证据，要求最新水位的高风险交付需宿主另加freshness gate。已交付内容无法收回，真实业务仍需敞口/补偿管理；这里没有完整金融总账、余额不足、税务或退款签名系统。

## ERC-8183 Job 模型

`Job`为单进程mutex保护的原子资金模型，无Solidity/ERC20合约。按2026-10-19规范正文的角色与expectedBudget规则实现：Client/Provider可在Open设预算；Client选Provider及出资；Provider提交；Evaluator完成或在Funded/Submitted拒绝；Client只能Open拒绝。

fund必须有provider、正预算、expectedBudget相等。本地额外策略禁止到期再fund。expiry只开放claimRefund资格，时钟不会自动改状态；Submitted到期后，在refund尚未成交前Evaluator仍可能完成。两者竞争由原子状态转换决定，最终paid+refunded=budget且escrow=0，不能双付。

Refund任何非空本地caller可触发，但钱永远记给Client；core退款不走Hook。HookFault仅模拟before/after revert，after失败回滚全部状态，不执行任意合约或重入。未模拟平台费、Evaluator费、Hook额外资产、代理升级/forwarder或真实链重组；后者由独立支付投影示例说明。

注意ERC-8183当前页面的规范正文与附带reference code存在差异：正文fund含expectedBudget，代码片段的签名不同；不能原样复制后宣称通过规范验收。真实接入须固定目标部署ABI、角色/费用/升级规则并单独验收。

## 故障矩阵

| 测试组（省略Test前缀） | 验证 |
|---|---|
| ActualEIP3009SignatureAndUnsupportedMechanisms | 真签名、改payer/payee/amount/nonce、过期/未生效，版本/网络/资产/Facilitator/scheme/transfer/flow拒绝 |
| ResourceTenantSubjectAndBodyBindingBeforeExecution | 签名仍有效但换租户/主体/资源/方法/正文/intent及外主体访问拒绝且不泄露快照 |
| ConcurrentReplayCachedResultAndNonceReservation | 16路并发、响应丢失后读原结果、新intent复用nonce拒绝，1次effect/1条结算腿，跨scope账务隔离 |
| LostSettlementReplyRestartAndCanonicalRecovery | settle回包丢失、重建入口与Engine、UNKNOWN不重付、凭证恢复 |
| RevocationAfterRemoteEffectDoesNotReturnResource | 已产生远端effect后撤权，不返回资源 |
| PendingUnseenCanceledAndMismatchedEvidence | 未知/未确认/非canonical/取消、错recipient/nonce/digest/RPC错，不误记成功 |
| ReorgReversalReconfirmationAndStaleRecoveryCAS | settlement→reversal→新block settlement、重复恢复及过期查询拒绝 |
| RecoveryRevisionFencesABAAndUnseenCannotReversePaid | 状态/回执ABA后的旧查询拒绝，暂查不到/未确认/无block/其他孤块不伪造冲正 |
| ReopenedEngineRequiresPayerAndChainConfiguration | 重启缺状态目录/Chain或付款账户变化时不执行/恢复 |
| PersistedStateCorruptionFailsClosed | 不完整状态不重置、不重付 |
| RealProcessCrashAfterSettlementRecoversWithoutRepayment | 子进程远端结算落盘后exit73，真实重启恢复，仍只有1次effect |
| V2FlowOrderingStopsOnPrepaymentAndCaptureFailure | 3种ordering及每阶段错误停止，未知flow不执行 |
| EscrowExpectedBudgetProviderAndRoleBoundaries | 预算变化、未选Provider、错误出资/提交/完成/退款角色 |
| ExpiryBoundaryHookRevertRollbackAndUnhookableRefund | 精确到期、过期fund拒绝、after回滚、Hook无法挡退款 |
| CompleteRefundRaceAndBothTransactionOrders | complete-first/refund-first及并发，两条资金路径只胜出一条 |
| RejectPermissionsAndAfterHookAtomicity | 各状态拒绝权限、after Hook不留部分退款 |

共16组顶层测试，另有1个真实崩溃子进程helper。所有结果都是本地确定性证据，不证明x402b厂商、主网/测试网、生产IAM或已部署ERC-8183互通。
