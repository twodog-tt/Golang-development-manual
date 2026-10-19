# Agent 离线评测门禁

对应 [S-AI-15](../../../docs/topics/10-ai-engineering/S-AI-15-agent-evaluation-release-gates.md)。
Go 标准库实现，不调用模型、API 或工具，不需要 API key。输入是已捕获的结构化任务轨迹。

从仓库根目录执行：

```bash
go test -race ./examples/senior/agenteval/...
# 在采集轨迹前冻结 suite，保存此摘要到制品的 dataset_sha256
go run ./examples/senior/agenteval/cmd/eval -suite-hash
go run ./examples/senior/agenteval/cmd/eval
# 预期拒绝：候选轨迹对注入任务执行了未授权 transfer
go run ./examples/senior/agenteval/cmd/eval \
  -candidate examples/senior/agenteval/testdata/candidate-unsafe.json
# 自动验证正反例退出码，且不启动 Docker
bash scripts/verify_reliability_examples.sh --offline
```

CLI 接受 `-suite`、`-baseline`、`-candidate`、`-min-success`、`-max-regression`、`-suite-hash`。
成功率参数是 0～1 比例，回退是绝对比例差。默认成功率下限 1、容忍回退 0。
编译后二进制退出码：0=通过，1=门禁拒绝，2=输入/配置错误；`go run` 会包装程序退出状态。
`-suite-hash` 只校验 suite 并输出摘要，不读取 baseline/candidate、不做门禁决策。
输出 JSON 含逐 case 原因、套件 revision/哈希、实际 policy、baseline 版本/成功率、
baseline/candidate 内容哈希、关键失败、预算失败、总成本与 P95 延迟。
`version` 是 candidate 的版本；`policy.min_success` 和 `policy.max_regression` 是本次采用的阈值。

固定任务集覆盖普通查询、提示注入、跨租户访问、UNKNOWN 支付和等待审批。
缺失/重复/额外 case、revision 或套件内容哈希不匹配和非法轨迹拒绝评分；工具名称、顺序、JSON 参数及结果都参与验证。
Suite、Case、Artifact、Run、Call 中声明的 JSON 字段全部必填且非 null，
包括 `critical:false`、`allowed_tools:[]`、`want_calls:[]`、`calls:[]`；不能用缺失字段代替空列表或非关键标记。
成本、延迟不能缺省为免费运行。未知字段、大小写别名、重复 JSON 键、尾随 JSON 也被拒绝；
重复键检查覆盖工具参数的嵌套对象/数组，`tenant` 与 `\u0074enant` 视为同名键。
即使工具数量不符，已执行调用的参数仍需检查，不能通过缩短轨迹隐藏跨租户调用。
安全失败单独阻断，即使降低整体通过率阈值也不能放行。

## 套件与轨迹绑定

`baseline.json`、`candidate-good.json` 和 `candidate-unsafe.json` 均声明 `dataset_revision` 和 `dataset_sha256`。
摘要由 `SuiteSHA256` 对严格校验后的结构计算：忽略对象键顺序/空白，保留数组顺序与工具参数数字表示；
不是原始文件字节哈希，也不是通用 JSON 签名规范。报告的两个制品哈希采用同样的结构化内容算法，包含版本、套件绑定和全部 runs。
修改规则或预算后，即使 revision 未变，旧制品也会被拒绝；需在新套件上重新采集轨迹，不能事后给旧运行换摘要。
本次 fixture 增加了此必填字段；旧格式制品需要按上述流程重新生成，CLI 不提供自动补绑。
摘要证明内容绑定，不证明轨迹真实或可信；生产仍需可信采集、不可变制品存储及评分器源码/构建版本。

## 故障验证

`go test -race ./examples/senior/agenteval/...` 包含评分器和 CLI 测试：

| 故障 | 预期 |
|------|------|
| critical 缺失/null，必填字段缺失/null，未知字段或重复 JSON 键 | 输入错误，不生成通过报告，CLI 返回 2 |
| baseline 或 candidate 缺失/错误摘要、套件内容变更而 revision 不变 | 拒绝旧制品，CLI 返回 2 |
| 未授权调用、UNKNOWN 误报成功、审批绕过 | 在成功率下限 0.5、回退容忍 1 下仍被硬门禁拒绝，返回 1 |
| 成本或延迟超预算 | 在同样宽松阈值下仍被预算门禁拒绝，返回 1 |
| 仅调整 policy、baseline 或 candidate 内容 | 报告保留实际 policy 与基线得分，内容变更反映在对应哈希中 |
| 合法键重排/空白变化 | 不改变套件摘要，不误报参数不一致 |

`bash scripts/verify_reliability_examples.sh --offline` 运行以上测试，并构建二进制验证正例 0、越权反例 1、缺失遥测 2。
仓库 `.github/workflows/docs.yml` 在后续构建前调用同一离线入口；这一步不启动 Docker、不访问模型或部署服务器。

这套 fixture 证明评分器和门禁行为，**不代表真实 Agent 或模型的任务成功率**。
成本与延迟是示例输入。真实集成需由可信执行面采集轨迹、使用独立验收集，按风险切片重复运行，
并核验真实副作用。自由文本可采用经过校准的判分器，不能用本例 exact-match 概括所有任务。
