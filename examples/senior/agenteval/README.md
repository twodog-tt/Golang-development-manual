# Agent 离线评测门禁

对应 [S-AI-15](../../../docs/topics/10-ai-engineering/S-AI-15-agent-evaluation-release-gates.md)。
Go 标准库实现，不调用模型、API 或工具，不需要 API key。输入是已捕获的结构化任务轨迹。

从仓库根目录执行：

```bash
go test -race ./examples/senior/agenteval/...
go run ./examples/senior/agenteval/cmd/eval
# 预期拒绝：候选轨迹对注入任务执行了未授权 transfer
go run ./examples/senior/agenteval/cmd/eval \
  -candidate examples/senior/agenteval/testdata/candidate-unsafe.json
# 自动验证正反例退出码，且不启动 Docker
bash scripts/verify_reliability_examples.sh --offline
```

CLI 接受 `-suite`、`-baseline`、`-candidate`、`-min-success`、`-max-regression`。
成功率参数是 0～1 比例，回退是绝对比例差。默认成功率下限 1、容忍回退 0。
编译后二进制退出码：0=通过，1=门禁拒绝，2=输入/配置错误；`go run` 会包装程序退出状态。
输出 JSON 含逐 case 原因、套件哈希、关键失败、预算失败、总成本与 P95 延迟。

固定任务集覆盖普通查询、提示注入、跨租户访问、UNKNOWN 支付和等待审批。
缺失/重复/额外 case、版本不匹配和非法轨迹拒绝评分；工具名称、顺序、JSON 参数及结果都参与验证。
轨迹中的成本、延迟和调用列表必须显式提供，缺失或 null 不能当成免费运行或未执行工具；未知字段也会被拒绝。
即使工具数量不符，已执行调用的参数仍需检查，不能通过缩短轨迹隐藏跨租户调用。
安全失败单独阻断，即使降低整体通过率阈值也不能放行。

这套 fixture 证明评分器和门禁行为，**不代表真实 Agent 或模型的任务成功率**。
成本与延迟是示例输入。真实集成需由可信执行面采集轨迹、使用独立验收集，按风险切片重复运行，
并核验真实副作用。自由文本可采用经过校准的判分器，不能用本例 exact-match 概括所有任务。
