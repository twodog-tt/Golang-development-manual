# SBOM、Provenance 与发布准入本地演练

对应 [S-SEC-03](../../../docs/topics/21-security-engineering/S-SEC-03-sbom-provenance-release-admission.md)。所有命令在仓库根目录运行，需 Go 1.24.2+ 与 Python 3，无真实凭据、网络模块下载或部署。

```bash
go run ./examples/senior/releaseadmission/cmd/demo
go test -race -timeout 60s ./examples/senior/releaseadmission/...
go vet ./examples/senior/releaseadmission/...
python3 examples/senior/releaseadmission/verify_go_inputs.py
```

如系统的 `GOROOT` 与 Go 二进制不一致，可使用 `env -u GOROOT /opt/homebrew/bin/go ...`；其他系统使用本机 Go 路径。Python 演练支持 `GO_BIN` 指定本机 Go。

Demo 在临时目录创建输入，验证正常准入/重复提交、审批后 implementation 变化、无例外高危风险、有效例外被撤销四种流程，退出时清理：

```json
{"dry_run_effects":1,"passed":true,"real_deployments":0,"revoked_exception_blocked":true,"slsa_level_claimed":false,"stale_target_blocked":true,"unwaived_risk_blocked":true}
```

## 验证链与故障矩阵

`wire.go` 真实验签 DSSE PAE + Ed25519，按受信 key 配置核对 issuer/identity/role/environment；builder 还需独立映射。`policy.go` 校验实际制品 hash、SLSA-v1 provenance、受信测试声明、实际 SBOM/扫描字节、审批及精确风险例外。所有证据 subject 指向同一制品；审批/例外绑定完整目标、策略和 EvidenceDigest。

`gate.go` 将证据私有复制到 ticket，在 Prepare 和 Commit 两次执行检查；epoch 防目标/策略/撤销变化与 ABA，intent 不允许换请求，Commit 临界区保证并发重复请求效果为 1。每次 Commit 包括重复调用仍先检查当前有效性；已成功后撤销不会抹掉历史回执，也不会授权新的部署。策略版本不可回用；重启后旧 ticket 无效。重新审批使用新 intent，回滚也需当前策略下的有效证据。

| 顶层测试 | 故障/正向控制 |
|---|---|
| `TestHappyPathAndConcurrentIdempotency` | 正常准入、16 路并发 Commit、重新 Prepare 后相同 intent 重试，效果恰为 1 |
| `TestRequiredTestEvidence` | unit 失败、race 缺失、vet 跳过、测试缺失/无结果、错 invocation/commit/inputs/suite/platform、过期/未来时间、测后产物替换 |
| `TestSignatureAndAuthorization` | payload 篡改、错 key/多签/算法载荷类型、错角色/环境/issuer/identity/builder、构建者签审批、同 key 跨角色别名 |
| `TestProvenanceContract` | 错 predicate/statement/buildType/builder、来源/ref/commit/输入与参数不匹配、重复或缺失来源、过期；通用未知扩展接受 |
| `TestSBOMActualBytesAndProfile` | 报告实际字节改变、错版本/根包 checksum、缺 license/creator/package、重复 ID/关系、悬空/缺关系、过期和 namespace 错误 |
| `TestScanFailuresAndFreshness` | error/缺失/畸形报告、缺/null findings、未知 severity/重复风险、错库/过期/未来库时间、平台/工具/报告新鲜度 |
| `TestExceptionScopeAndHardGates` | 无例外、有界有效例外、错目标/证据/策略/风险、重复风险、过期/缺 owner/control、构建者自批、不能豁免失败测试/坏签名/缺报告 |
| `TestApprovalBinding` | 当前策略、完整目标、证据摘要、审批 ID 和有效时间 |
| `TestTargetChangesAndABA` | environment/image/platform/config/migration/flags/chain/proxy/implementation/codehash/inputs 变化及改走再改回 |
| `TestRevocationRollbackAndPolicy` | 密钥/审批/例外/制品撤销，策略升级与旧版本回用，已成功制品回滚仍执行当前撤销策略 |
| `TestCommitRevalidatesEvidenceAndInput` | 审批后到期、输入改变、目录不可用、取消、进程状态重建 |
| `TestPrivateCopiesAndIntentConflicts` | 调用者篡改原始证据/信任公钥不影响私有副本，同 intent 不能换新审批 |
| `TestCommitAndRevocationSerialize` | Commit 与撤销并发，按临界区顺序最多一次效果，后续尝试拒绝 |
| `TestStrictJSONAndBounds` | 重复/转义等价键、大小写别名、尾随 JSON、非 object、非法 surrogate、过深/超大载荷、无效公钥；合法 surrogate 正例 |
| `TestFingerprintCoversActualInputs` | vendor/local/workspace/generated/go.work/二进制内容及执行位变化 |
| `TestFingerprintRejectsUnavailableOrUnboundedInput` | 空/缺失根、symlink 文件/根、单文件/总量/文件数/目录数上限、取消 |

## 格式与边界

- SLSA v1.2 对应 `predicateType=https://slsa.dev/provenance/v1`；限定 buildType 的 externalParameters 只允许准确的 repository/ref，来源 commit 与快照 digest 放 resolvedDependencies。通用未知扩展忽略；JSON 重复键、非法类型和资源超限拒绝。
- SPDX **2.3 JSON 限定配置**：只允许 fixture 展示的文档/包/关系字段，最多 64 包，每包一个合法 SHA256 checksum，根包 checksum 等于制品；所有包 `filesAnalyzed=false`、download/license/copyright 为 `NOASSERTION`。只支持 DOCUMENT DESCRIBES Artifact、Artifact DEPENDS_ON 包，拒绝重复/悬空关系。不是通用 SPDX schema 或许可证表达式验证器，不保证组成清单完整。
- scan 为本地 `local-scan-v1`，仅 complete、明确 findings 数组和允许数据库/扫描器版本有效；high/critical 需要精确风险例外。报告/签名声明最大 128 KiB、JSON 深度 16/值数 2048；制品最大 1 MiB；报告默认 300 秒、数据库 86400 秒有效，拒绝未来时间。
- `Fingerprint` 包含指定根的相对路径、字节摘要和执行位，上限 128 文件、256 个遍历项（含目录）、每文件 2 MiB、总计 4 MiB；不跟随链接。必须使用可信、专用、不可变的输入快照，覆盖所有根及外部构建输入。算法不提供可变目录原子快照，也不认证 commit。内存 Gate 最多 128 张 ticket，无持久恢复/真实部署效果。
- 公开确定性教学密钥不能用于真实发布；issuer/identity 是受信配置标签，未校验证书/OIDC/透明日志。Fixture 的产物/commit/测试/SBOM/扫描是合成数据，未运行真实 producer，不能宣称安全扫描通过或任何 SLSA 等级。
- Target/Policy/撤销只由可信宿主设置，不是用户或模型工具。外部执行器必须在真实部署边界落地 fencing、持久幂等和 UNKNOWN 对账；此处不会访问注册表、Kubernetes、链或生产 IAM。有效离线证据可检查，证据缺失、错误、过期一律阻断。

## 离线 Go 输入篡改实验

`verify_go_inputs.py` 在临时目录生成 file:// 模块 proxy 和隔离缓存，运行真实 Go 编译/执行，分别改动 vendor、本地 replace 和工作区依赖。三例均要求：

1. 正常程序输出 `original`，缓存校验通过，保存整个输入根的 Fingerprint。
2. 修改实际编译的源码后，`go mod verify` 仍输出 `all modules verified`，程序变为 `tampered`。
3. `cmd/fingerprint -root ROOT -expected DIGEST` 输出 `matches=false` 并以 1 退出。重新 vendor 还能恢复原摘要。

输出 `passed=true`、`network_dependencies=0` 和三个案例；任一预期不符即失败。`GOSUMDB=off` 只作用于读取自造模块的子进程，GOVCS 禁用 VCS、GOPROXY 仅指向本地目录，工具自身以 GOPROXY=off 编译，不改变全局 Go 配置。工作区指纹覆盖 go.work 及两个 module 根；根外输入不在检查范围内。

统一入口：`bash scripts/verify_reliability_examples.sh --offline`（该入口的既有三语言检查另需 Node 22.15+）。
