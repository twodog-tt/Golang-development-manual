# Agent 工作流、审批与恢复夹具

对应 S-AI-03、S-AI-05、S-AI-09。Go 标准库实现，要求 Go 1.24.2+ 与 Linux/macOS
（使用 Unix file lock）。不调用模型、真实 OAuth 或发布 API，不需要密钥或 Docker。

## 直接运行

仓库根目录：

```bash
go test -race ./examples/senior/agentworkflow/...
go run ./examples/senior/agentworkflow/cmd/demo -dir "$(mktemp -d)"
bash scripts/verify_reliability_examples.sh --offline
```

默认 demo 用固定模型提出一条 publish proposal，暂停后由 fixture reviewer 批准并继续。
输出应包含 `job_status=succeeded`、`run_status=succeeded`、`external_effects=1`。
reviewer 是脚本中的独立测试身份，不是真实人类确认或登录系统。

## 分步审批与回包丢失

```bash
go build -o /tmp/agent-workflow-demo ./examples/senior/agentworkflow/cmd/demo
workflow_demo_dir="$(mktemp -d)"
/tmp/agent-workflow-demo -dir "$workflow_demo_dir" -step init
/tmp/agent-workflow-demo -dir "$workflow_demo_dir" -step plan
# 此时 run 暂停，job=pending，external_effects=0
/tmp/agent-workflow-demo -dir "$workflow_demo_dir" -step approve
# 模拟 provider 已接受但客户端没收到回包
/tmp/agent-workflow-demo -dir "$workflow_demo_dir" -step run -lose-reply
# job=unknown，external_effects=1，model_requests=1
/tmp/agent-workflow-demo -dir "$workflow_demo_dir" -step run
# 仍然 unknown，模型不能重规划，副作用仍为 1；工具尝试数和预留成本继续累计
/tmp/agent-workflow-demo -dir "$workflow_demo_dir" -step reconcile
/tmp/agent-workflow-demo -dir "$workflow_demo_dir" -step run
/tmp/agent-workflow-demo -dir "$workflow_demo_dir" -step inspect
# 最终 job/run=succeeded、external_effects=1、model_requests=2
```

`init` 要求新目录，拒绝覆盖旧 state；其他步骤重新打开同一文件，不重置审批、预算或 deadline。
demo 的 `plan` 等待审批、`run` 遇到 UNKNOWN 都是预期暂停，退出 0 并输出状态；必须核对 JSON 的 job/run 状态，
不能仅凭退出码认定成功。其他错误退出 1。统一验证脚本实际断言上述状态和副作用次数。

## 控制面与授权

- `control.json` 保存 proposal、approval、job、credential、run 与预算；`external.json` 独立保存 provider 效果和 refresh 轮换事实。
- 文件事务每次重读，在 OS 锁下原子替换并 fsync；网络调用不持状态事务锁。同一 run 的 Continue 另持非阻塞专属锁，进程退出后释放。
- `Authority` 的 bearer→Principal 映射是服务端配置；不读取模型自报 reviewer/tenant 来赋权。
  生产须接真实认证适配器。`SetGrant`、`SetCredential`、`Job`、`Credential` 是可信运维/检查入口，不能原样暴露成公共 API。
- review/execute 权限按 actor + tenant + provider + account + tool 查验；禁止自批。
  审批绑定 proposal ID/version、intent、内容、账号、凭据主体/scope、tool/workflow/policy version；本例仅支持 publish/v1。
- approve/edit 原子串行；编辑清除审批。claim 和 dispatch 再检查当前审批人权限、执行者权限、凭据和审批有效期/撤销状态。
  claim 后不可再编辑；撤销若发生在派发标记前可阻止调用，发生在其后不能承诺取消在途请求。

## 执行与恢复

| job 状态 | 含义与恢复 |
|----------|------------|
| pending | 等待当前 proposal 的有效审批 |
| ready | 已批准，可由唯一 claim 领取 |
| claimed | 持有 lease/epoch，尚未进入派发；过期后可封存旧 epoch 并返回 ready |
| unknown | 派发前已落盘的不确定窗口；timeout、进程崩溃和 lease 接管都不能推断“未执行” |
| manual | 非幂等 provider 查询未命中；停止重发，仍可核对迟到成功回执 |
| succeeded | 匹配 intent、binding、payload digest 的权威 receipt 已记录 |
| failed | provider 证明未执行且请求以后也不会生效；普通超时/5xx 不属于此类 |

`RecoverExpired` 仅 fence 本地状态写入，旧 worker 仍可能在外部产生效果。测试让旧请求在接管后才到达非幂等
provider：新 owner 查询未命中后进入 manual，不重发；旧 worker 的本地提交被拒绝，随后对账保留真实成功。

`RetryUnknown` 只接受 owner 已封存、审批仍有效且 provider 明确保证同键/同 payload 的场景；沿用原 intent，
fixture 的幂等保留期覆盖整个实验。不支持该契约时 not-found 不是安全重试凭据。真实生产还要核对 provider 的
作用域、保留期、查询一致性及旧请求终止保证，本例不验证厂商契约。

## 工具历史、预算与凭据轮换

Runner 保存完整 fixture reasoning/message/function-call 项，结果关联原 call ID。
`Item` 是本地协议夹具；真实 SDK adapter 必须保留其输出项/opaque reasoning 或正确使用服务端响应续接。
模型不能伪造 function_call_output，重复 call ID 和不完整批次拒绝执行。

`MaxSteps`、`MaxCalls`、token/cost 预留与绝对 deadline 全部持久化。模型请求前扣留最坏情况预留；
每个工具尝试前累计计数和成本，包括等待审批或 UNKNOWN 的恢复尝试，不退还预留。
100 个调用超过剩余调用配额时整批拒绝；成本不足可能在执行合法前缀后暂停，不承诺整批事务。
所有数字都是 fixture 的保守单位，不是实际计费。模型超时/响应未落盘留下 ModelPending，拒绝自动再请求；
生产需人工审查或 provider 特定恢复，本例没有“清标记后免费重跑”入口。取消/deadline 不能撤销已发生的外部效果。

refresh 先持久化 refreshing/attempt/version，再出锁调用独立 rotator。回包丢失转 reauth_required；
进程崩溃遗留 refreshing 由 `RecoverRefresh` 隔离，均不重放旧 token。可信新授权通过 `SetCredential` 恢复；
credential subject 改变使旧审批失效，迟到响应用 version + attempt CAS 拒绝覆盖新授权。
Token 仅为 `NONSECRET-*` 标记：不得把生产 token 写进这个未加密的教学状态文件。
fixture 模拟旧 token 拒绝，不模拟完整 OAuth token family 撤销、真实授权码回调或 provider 恢复接口。

## 故障验证对应关系

| 原问题 | 可运行验证 |
|--------|------------|
| UNKNOWN 被当普通错误重规划 | `TestUnknownBlocksModelUntilReconciliation`，对账前模型只调用一次，外部效果只有一次 |
| 审批身份/资源绑定不完整 | `TestApprovalAuthorizationAndBinding`、`TestExecutionRechecksApprovalAndCredential` |
| 僵尸 worker、执行中崩溃与回执丢失 | `TestLateWorkerCannotTriggerAutomaticDuplicate`、`TestProcessCrashAndRestartRecovery` |
| 工具结果缺少来源历史 | `TestApprovalPauseResumePreservesProtocolAndBudgets`、`TestProtocolForgeryAndModelFailureFailClosed` |
| maxSteps 无法约束单批工具与恢复预算 | `TestBatchOverflowExecutesNoTools`、`TestDurableBudgetsAndDeadline` |
| refresh 轮换成功但回包/落盘丢失 | `TestRefreshAckLossQuarantinesCredential`、`TestRefreshCrashRequiresReauthorization`、`TestConcurrentRefreshAndStaleCompletion` |
| 缺少审批/恢复故障演练 | `TestConcurrentApproveEditAndClaim`、`TestConcurrentRunOwnership`，以及整个 race 套件和 CLI 状态断言 |

崩溃测试启动真实 Go 测试子进程，在边界处 `os.Exit(23)` 跳过所有 defer；父进程重新打开两份文件并核对结果。
这证明本地控制面的限定行为，不是生产数据库、厂商 SDK、真实 Agent、OAuth 安全体系或跨主机 HA 验收。
