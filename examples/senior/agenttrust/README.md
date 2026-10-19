# ERC-8004 身份与信任信号本地实战

对应 S-AI-12。Go 1.24.2+，复用根模块已有 go-ethereum 做真实 EIP-712/EOA 验签；不连接公链、不部署合约、不需要资金或生产密钥。

```bash
go run ./examples/senior/agenttrust/cmd/demo
go test -race -timeout 60s ./examples/senior/agenttrust/...
```

CLI 依次运行 owner 默认钱包 → 新钱包 EOA 证明 → 转移清钱包 → 元数据地址抓取被拒 → 精确信誉均值 → 撤销 → 验证结果100降至0 → 回滚到祖先恢复100。
预期 `passed=true`、`initial_owner_wallet=true`、`transfer_cleared=true`、`ssrf_blocked=true`、`normalized_mean="1"`、`revoked_excluded=true`、`latest_score=0`、`reorg_restored_score=100`。
演示密钥从公开教学字符串导出，绝不能接收真实资产。演示不输出私钥。

## 抓取与域验证边界

`NewResolver().Fetch(ctx,url,expectedSHA256)` 仅支持 HTTPS/443，无 userinfo/fragment，拒绝 localhost、私网、链路本地、IPv4映射地址、CGNAT及特殊用途网段。初检和每次dial前重新解析；所有DNS结果都必须公开，最终dial固定到已检查IP，TLS仍验证原域名。禁用环境代理、重定向和压缩；总预算2秒、响应头16KiB、正文64KiB、HTTP200和application/json。

正文 SHA-256 是**本地内容版本指纹**；非零 expected 不匹配时要求重新审核，首次观察不等于内容可信。它不是 ERC-8004 feedbackHash/requestHash：后者按规范使用 Keccak-256。IPFS/data 虽为规范允许的 URI，本适配器明确拒绝，未实现 CID 解码/网关或data解析，不能把本地限制说成标准禁止。

`MatchRegistration(raw,ref)` 只核对双向 registrations 声明，拒绝重复键、所用身份字段的大小写别名/错误类型、尾随JSON、深度>16。域验证须由宿主从**端点自己的 origin**拼接 `/.well-known/agent-registration.json` 后安全抓取，不能用第三方注册文件冒充该域证明。本函数不是完整注册文件/schema校验器，也不是端点认证。未知扩展仍留在原始哈希正文中；A2A/MCP能力、对象归属、active状态与运行授权另行检查。

真实TLS测试使用 httptest证书根信任和私有测试dial缝隙连接loopback；生产构造器不暴露该绕过入口，不跳过TLS验证，不访问外部网络。

## 钱包绑定

- `NewIdentity(ref,owner)` 初始 Wallet=owner，修正“任何初始绑定都需要额外签名”的误解；Transfer 后归零，Unset/Transfer/Bind 都提升本地 revision。
- `BindingDigest` 使用明确的 **LocalAgentBinding / v1 / LocalWalletBinding** typed-data；绑定chain、registry、agentId、owner、wallet、revision、deadline。它不是 ERC-8004 部署合约的ABI/签名结构，真实接入必须核对目标部署的type/domain/权限/重放规则。
- 本地模型只允许owner变更，10分钟内deadline为本地策略；没有模拟所有ERC-721 operator权限。EOA是真实ECDSA恢复；ERC-1271为明确的可信适配器接口，测试返回值/错误/取消，不是RPC或合约执行验收。
- IsContract失败不能回退成EOA。1271必须返回0x1626ba7e，revert/错误magic/超时均失败；两步查询必须响应ctx。复核后以revision CAS写入，防止验签期间转移/清空及转移后回到旧owner。
- 钱包控制证明不是登录、业务支付授权、审批或持续有效的签名策略证明。Identity为串行安全内存模型，没有持久注册表或可恢复签名权限的声明。

## 信任投影

`NewProjection(ref,owner,reputationRegistry,validationRegistry)` 固定可信注册表与Agent复合键。输入来自宿主已解码的可信canonical收据适配器，不能接受模型或普通客户端直接提交的“链事件”。这个夹具固定owner，不实现完整Identity事件索引或owner/operator更新；真实所有权变化需重建授权上下文并失效旧审核。

`Apply(Block)` 按block number/hash/parent串联，整块校验后原子更新；同hash相同内容重放幂等，冲突重放拒绝。每块≤256事件/128KiB，最多128块。`Rewind(ancestor)` 只回到已知祖先（零值表示全部撤回），然后从保留原始事件重建，未知祖先失败；不会猜测跨越保留窗口的恢复。

- Feedback主键=当前Ref+client+feedbackIndex；值按int128范围、decimals0..18校验，使用big.Rat算精确均值。reviewer集合必须非空且tag匹配；本例要求非空tag、拒绝owner自评，未实现operator列表、Sybil评分、原生getSummary ABI或反馈证据下载。
- revoke后不计入均值；重组撤回revoke后重新计入；不同tag不混算。原始事件仍保留。
- request必须由本地owner发起，requestHash=Keccak256(payload)，同hash不重建不同请求；response必须来自原validator，匹配registry/ref/hash，值0..100。
- 多次response按canonical事件顺序采用最新值（包括从100降至0），不是取历史最大值。未响应不当作0分结论；撤回响应后不返回验证结果。该策略不是通用最终性证明，tag语义/证据hash校验与validator信任仍由真实适配器处理。

## 故障矩阵

| 测试组（省略Test前缀） | 验证 |
|---|---|
| WalletInitialDefaultEOAProofAndTransferClear | 初始owner钱包、真实EOA证明、重复证明、转移清空 |
| WalletDomainOwnerEpochExpiryAnd1271Failures | 错链/registry/agent/owner/revision、过期/验签中到期、转移离开后返回、RPC错误/1271 revert/错误magic/超时、校验期间状态变化 |
| 1271MagicAndStateDependentRevalidation | 正确magic、后续策略失效、无权清空 |
| ResolverTLSHashAndReciprocalRegistration | 真实验证TLS抓取、内容hash、双向身份匹配与变化拒绝 |
| ResolverDeniesSpecialTargetsBeforeDial | 私网/元数据/映射地址/CGNAT/保留网段、userinfo、非443、未支持scheme |
| ResolverRebindingMixedDNSAndNoProxy | 校验后DNS变私网、混合DNS、代理不绕过 |
| ResolverRedirectTypeSizeAndSlowBody | 重定向不跟随、错误类型/编码、超限、慢流取消 |
| RegistrationMalformedAndDuplicateKeys | 缺身份、重复键、尾随、深度超限 |
| RegistrationCaseAliasesAndWrongTypesFailClosed | registrations/agentId/agentRegistry的大小写别名与非预期类型拒绝 |
| FeedbackExactDecimalsRevocationDuplicateAndReorg | 精度/负数/量纲、重放、撤销与撤销重组 |
| SignalInvalidEventsAreAtomicAndBounded | 错注册表/身份、自评、decimals/int128越界、重复身份与孤立撤销；整块无部分提交 |
| ValidationCallerIdentityDowngradeAndCanonicalReplay | 请求owner/内容承诺/validator缺失、重复请求、冒名/错来源/旧任务、越界结果、降级、祖先回退及新canonical重放 |
| ProjectionLineageConflictAndCallerMutation | 错parent、冲突重放、调用方修改原slice、未知祖先 |

上述为13组本地测试，不证明真实部署ABI、ERC-1271链上行为、节点finality、IPFS互通或生产IAM。规范参考：[ERC-8004](https://eips.ethereum.org/EIPS/eip-8004)、[EIP-712](https://eips.ethereum.org/EIPS/eip-712)、[ERC-1271](https://eips.ethereum.org/EIPS/eip-1271)，复核于2026-10-19。
