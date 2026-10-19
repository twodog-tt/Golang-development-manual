# 授权检索与无证据拒答

对应 S-AI-02。Go 1.24.2+，内存库与 MockClient，不使用模型服务或向量数据库。

```bash
go test -race ./examples/senior/rag/...
go run ./examples/senior/knowledge-demo
```

`Authority.Context` 由可信宿主换取身份；请求的 scope 必须与授权中完整 tenant/bot/scene/subject type/subject ID 匹配。
`Store.Add(ctx, Chunk...)` 校验写权限、policy、期限、来源；Readers 为空表示 namespace 所有已授权读者，非空则再按 subject 收紧。
`Service.Answer(ctx, scope, question)` 在评分前检查 namespace/policy/Readers/期限；模型返回后再核对活跃授权和片段版本。

本地召回是汉字双字/英文词项余弦，分数正且不低于阈值才入 Top-K，不是 embedding。默认 TopK=3、MinScore=0.1、上下文16KiB；TopK最多20，问题/片段正文各最多4096字节。
无证据返回固定“资料中未找到”且不调用模型。相似度与阈值只作教学参数，不能代替中文语义召回评测。

| 测试 | 故障/断言 |
|---|---|
| `TestChineseRelevanceNotInsertionOrder` | 改变入库顺序；怎么退款只命中正分退款片段 |
| `TestRefusalSkipsModelAndUnauthorizedCandidates` | 火星轨道周期与私有片段不返回，不调用模型 |
| `TestScopeAndLimits` | 跨租户/Bot/subject、缺身份、NaN/无限/零阈值、TopK和输入预算拒绝 |
| `TestChangedEvidenceOrRevokedAuthFailsClosed` | 模型生成期间删除来源、撤权、取消或模型失败，不返回旧答案/证据 |
| `TestExpiryPolicyAndUntrustedPrompt` | 过期/旧policy不召回；资料只作JSON数据，无工具执行通道 |

验证不依赖第一条数据碰巧相关；Mock 的固定回复不能证明真实 groundedness、引用准确或抗提示注入。
撤权不能收回已发送的上下文，生产须明确检查时点并治理数据驻留与并发失效。
