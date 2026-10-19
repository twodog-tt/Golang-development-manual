# 只读 FAQ 缓存与 attempt 成本

对应 S-AI-06。Go 1.24.2+、标准库，只使用本地模型夹具。

```bash
go test -race ./examples/senior/llmops/...
go run ./examples/senior/knowledge-demo
```

应用缓存查找在模型请求之前，命中直接返回；provider prompt cache 是请求内部的前缀复用，两者分开统计。
`New(client, Config)` 的 prompt/model/corpus版本、TTL由可信宿主设置；只用于经评测可复用的只读FAQ，禁止当作带工具或资金动作的执行缓存。
候选必须先通过完整namespace、身份/issuer/auth revision、policy和版本及TTL边界，再进行词集合相似度比较。
这个matcher只演示词序变化，不是真实embedding；否定、个性化、实时状态等不能凭词集合判语义等价。

Cache命中与模型完成后均重查当前权限；失败/失权/空或超大回答不缓存。最多128条、TTL最多1小时；并发miss不合并，不承诺只生成一次。
CorpusVersion必须由宿主随底层资料更新，生产还需做资料版本和权限事务验证。

`Meter.Record` 接受每个HTTP attempt；未知用量保留Unknown，失败仍有费用可能。
示例 `teaching-v1` 每token费率(input,cached input,output)=(2,1,3)微美元，非真实市场报价。
10 input含4 cached、2 output，应为 `(10-4)*2+4*1+2*3=22` 微美元。cached input不可与全部input重复累加。
KnownMicroUSD只是已知小计，Unknown>0时完整成本尚未核对，不能当作零费用；负数/子集越界/溢出拒绝并计Unknown。

| 测试 | 故障/断言 |
|---|---|
| `TestCacheHitBypassesModel` | 词序变化命中，模型调用数不增加，task/hit/generation各自计数 |
| `TestBoundaryBeforeSimilarity` | tenant/Bot/scene/subject/identity/policy/auth revision及prompt/model/corpus版本、TTL变化在比较前排除旧条目 |
| `TestFailureAndRevocationNotCached` | 失败、生成中撤权和命中前撤权均不给答案，失败不写缓存 |
| `TestKnownFailedUsageStillCharged` | 失败attempt如有已知用量，仍计入已知成本 |
| `TestCostSubsetUnknownFailuresAndOverflow` | cached子集、失败/Mock未知费用、错误计数、乘加法溢出 |

综合CLI另验证429拒绝后成功：1次生成、2次HTTP attempts，接下来缓存命中不再请求模型。
此例只提供本地计数与成本聚合，不接入OTel exporter，不验证真实模型质量或provider账单。
