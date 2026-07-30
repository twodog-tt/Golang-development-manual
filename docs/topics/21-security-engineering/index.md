# 21 Web3 安全工程

4 篇 | 按钱包/节点/Staff 角色为 P0～P1 | [返回专题索引](../../topic-catalog.md) · [角色优先级](../_meta/role-priority-matrix.md)

> 从“用了 HSM、做了审计”下钻到可验证的安全边界：威胁模型、身份与密钥、签名机 fencing、供应链证明、安全测试和事件响应。

| ID | 标题 | 频率 |
|----|------|------|
| [S-SEC-01](./S-SEC-01-web3-threat-model-iam-trust-boundaries.md) | Web3 威胁建模、IAM 与信任边界 | ⭐⭐⭐⭐⭐ |
| [S-SEC-02](./S-SEC-02-key-ceremony-signer-fencing-recovery.md) | Key Ceremony、远程签名机 Fencing 与恢复 | ⭐⭐⭐⭐⭐ |
| [S-SEC-03](./S-SEC-03-sbom-provenance-release-admission.md) | SBOM、SLSA Provenance 与发布准入 | ⭐⭐⭐⭐ |
| [S-SEC-04](./S-SEC-04-security-testing-incident-response.md) | Fuzz/Property/Differential Test 与安全事件响应 | ⭐⭐⭐⭐⭐ |

## 可运行代码

| 题 ID | 目录 | 命令 |
|-------|------|------|
| S-SEC-02 | `examples/senior/signerfencing/` | `go test -race ./examples/senior/signerfencing/...` |
| S-SEC-02 | `examples/signer-project/` | `cd examples/signer-project && go test ./...` |

## 推荐顺序

威胁模型与身份 → 签名控制面和恢复 → 软件供应链 → 测试、演练与事件响应。
