# Prompt、Context 与结构化结果本地实战

对应 S-AI-04。Go 1.24.2+，标准库及仓库已有的授权/Mock；不调用模型或使用密钥。
从仓库根目录运行：

```bash
go run ./examples/senior/promptcontext/cmd/demo
go test -race ./examples/senior/promptcontext/...
```

CLI 输出 `passed=true`、`dropped=["old-tool"]`、`incomplete_rejected=true`、`duplicate_rejected=true`、`effects_executed=0`。
测试元数据表示可信夹具的状态，不是实际模型的质量或账单报告。

## Context 预算与完整历史

`Assemble(ctx, scope, groups, budget, counter)` 先检查当前身份、完整 namespace 与每个 group 的 scope，再裁剪。
Groups 及 Required/Kind 由可信宿主构建，不能直接采用模型给出的权限/优先级。
System 和当前任务必须且只能各有一个，均不可裁；历史/资料已由宿主完成资源授权。
工具交互以 group 保存 reasoning、call、result，CallID 全局唯一，结果必须关联本组尚未完成的调用。
未知类型、孤立结果、重复ID、未完成调用明确拒绝；pending/UNKNOWN/审批事实保存在权威工作流，不能靠删 prompt 恢复。

按输入顺序删除最旧的可选组，每次**重新计数**以容纳非加性的真实 tokenizer；不切字段或字符串、不修改输入历史。
无法保留所有必要组时返回 ErrBudget，不发起模型调用。Assembly绑定scope/身份/计数器和预算；CLI将裁剪后的Assembly实际送给ExtractAssembled，在最终role/资料引用格式化后再次计数；文本夹具不能表达仍保留的opaque工具项时明确拒绝，要求专用typed adapter，绝不静默压成普通文字：

```text
输入 + 可见输出预留 + 推理预留 + 余量 <= 窗口
可见输出预留 + 推理预留 <= 最大生成量
```

这些量必须同一模型/接口/计数器版本；组装及最终发送前计数的前后都核对版本，计数途中切换版本也拒绝。对 max_output_tokens 已包含 reasoning 的 API，发送两项预留的总和，不能再次重复计入推理。
`FixtureCounter` 是 UTF-8 字节加固定消息框架的**本地计数单位**，Version=fixture-bytes-v1，**不是实际 tokenizer，也不用于生产 token 容量或计费**。
真实计数器须覆盖格式化消息、工具定义/schema、多模态输入等；不可用时返回错误，不能悄悄换成词数。

## 结构化输出边界

`LocalFixture` 调用实际 `llmclient.MockClient.Complete(ctx, []Message)`，取得正文；另由可信宿主提供 Version/Status/Refusal。
`StructuredClient` 是新增、明确声明的本地类型接口，不冒充原有文本 Client 或任何厂商 SDK。
真实 provider adapter 必须从响应状态/typed events 归一化完成与拒绝，不能让模型在正文写一个 completed 字段就被当作成功。
`Extract` 先检查版本、completed、非拒绝和非空，再 ParseIntent；缺/null/重复（含转义等价）/未知/大小写别名键、尾随 JSON、错误类型和业务枚举/订单号一律拒绝。
`OrderSchema` 给出 required 与 additionalProperties=false；调用真实 API 时还须在对应 surface 启用 strict、核对支持的 schema 子集。

解析成功只得到 query/refund 意图，**不执行退款，也没有工具或资金入口**。真正执行前仍须查询权威订单归属/状态、当前权限和审批。

| 测试组 | 故障验证 |
|---|---|
| TestBudgetBoundaryAndAtomicToolGroup | 精确窗口临界、少一单位时整组裁剪、原历史不被修改 |
| TestRequiredGroupsAndOutputLimitsFailClosed | 必须内容超窗、输出+推理超过生成上限、余量非法、不可裁策略 |
| TestBrokenToolHistoryAndPendingCallsRejected | 孤立结果、未完成调用、重复ID、跨组关联、未知类型 |
| TestCounterVersionChangesFailBeforeModelDispatch | 组装/最终发送计数途中版本变化，模型调用次数保持0 |
| TestAuthorizationAndCounterFailures | 外租户资料组、缺计数器/计数错误/负数、计数中撤权 |
| TestBudgetAssemblyFeedsActualModelAndUnsupportedHistoryFailsClosed | 预算实际接入模型消息、保留opaque工具历史时不冒充文本协议、生成中撤权 |
| TestDispatchRechecksFinalFormattingAndAssemblyIdentity | 最终资料引用转义/框架开销再次计数、不能跨身份复用Assembly |
| TestActualClientInterfaceAndStrictIntent | 实际 Mock 签名、正常订单意图解析 |
| TestRefusalIncompleteEmptyAndVersion | 拒绝、incomplete、failed、旧版本不能成为正常意图 |
| TestMalformedJSONAndBusinessValues | 缺字段/null/重复/转义键/未知/大小写别名/非法枚举或订单号/尾随/超大输入 |
| TestCanceledExtractionDoesNotReturnIntent | 已取消调用不返回意图 |
| TestEmptyResultAndModelCannotSpoofStatus | 空结果、表面合法但未完成、正文自报状态均不能放行 |

本例不实现真实 tokenizer、模型 schema enforcement、生产 prompt registry 或摘要质量验收；相关要求在文章中作为生产适配条件说明。
