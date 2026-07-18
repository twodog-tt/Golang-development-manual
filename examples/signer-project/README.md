# Durable signer fence store + HSM/FROST sandboxes

这个嵌套 Go module 展示一个可重启的 signer-side fencing 边界，以及两个真实密码
API sandbox：SoftHSM2/PKCS#11 P-256 ECDSA 和 FROST Taproot/BIP-340 2-of-3。

> **安全前置条件：`fence` 不是完整授权层。** 调用 `Sign` 前，可信 transport/control
> plane 必须已经认证 caller，并用 signed grant 或等价机制绑定、授权
> `key_id + owner + epoch + payload`。本项目没有实现旧 `signerfencing` 示例中的 signed
> grant 验证，也没有验证 authenticated caller。CLI 的 owner/epoch/message 参数只是
> demo 输入，绝不能直接映射成公网 HTTP API 或让不可信客户端自行填写。

核心依赖固定为：

- `go.etcd.io/bbolt v1.4.3`
- `go.etcd.io/etcd/client/v3 v3.6.12`
- `go.etcd.io/etcd/api/v3 v3.6.12`
- `github.com/ThalesGroup/crypto11 v1.5.0`
- `github.com/miekg/pkcs11 v1.1.1`
- `github.com/taurusgroup/multi-party-sig v0.7.0-alpha-2025-01-28`

`etcd/client/v3 v3.6.12` 的 module 声明要求 Go 1.25 或更高版本，所以本 module
的 `go` directive 是 1.25.0。启用 Go toolchain auto-selection 的较旧 Go 命令会
自动获取兼容 toolchain。把 directive 降到仓库根的 1.24.2 并不能消除该要求；禁用
auto-selection 时，低于 1.25 的 toolchain 会在加载这个固定依赖时失败。

## 快速运行

以下命令均在 `examples/signer-project/` 目录执行：

```bash
go test ./...
go run ./cmd/signer-demo
go run ./cmd/signer-demo # 同一绑定返回已持久化 receipt
go run ./cmd/frost-demo
docker compose config --quiet
```

也可以使用：

```bash
make test
make demo
make frost-demo
make compose-config
```

## 持久化 fence 状态机

`fence.Signer.Sign` 对同一个 key 持有进程内锁，并严格按以下顺序执行：

1. **A — bbolt commit**：原子检查并持久化 `highest epoch + same-epoch
   owner + request=PENDING`。request binding 是
   `key_id + owner + epoch + SHA-256(domain || length || payload)`。
2. **B — backend.Sign**：只在 A 成功提交后调用密码后端。
3. **C — bbolt commit**：把算法、公钥和签名组成的 receipt 持久化，并将 request
   改为 `COMPLETED`。
4. **D — release**：只有 C 成功提交，调用方才得到 receipt/signature。

A 和 C 是两个短事务，B 不在 bbolt 事务内。这样后端失败、进程退出或 C 提交失败
都不会回滚 A 中已经接受的更高 epoch。即使更高 epoch 的请求随后撞上已存在的
request ID，新的 epoch/owner 也会先提交，再把 binding 冲突返回给调用方；旧 epoch
不能借逻辑错误路径恢复签名资格。

持久化规则：

- `epoch < highest`：拒绝 `ErrStaleEpoch`；检查发生在历史 receipt 返回之前。
- `epoch == highest` 且 owner 不同：拒绝 `ErrOwnerConflict`。
- 相同 request ID 和完全相同 binding：`COMPLETED` 直接返回原 receipt；`PENDING`
  恢复后端流程。
- 相同 request ID 但 payload、owner 或 epoch 不同：拒绝
  `ErrRequestConflict`，不覆盖原记录。
- 后端失败：保留 `PENDING` 和已提交的 fence，不返回签名。
- B/C 窗口退出：恢复后同一 `PENDING` 可再次调用后端。系统不承诺后端只调用
  一次；是否得到相同密码结果、能否在后端去重，取决于后端的确定性或幂等能力。
- C 提交失败：本次后端结果不从 API 释放。恢复重试可能再次执行密码操作，最后只
  返回成功持久化的 receipt。
- 首个 `COMPLETED` 会把 backend algorithm + encoded public key 绑定到该 logical
  key 的 fence state；后续 backend identity 漂移在 C 阶段以
  `ErrBackendIdentity` 拒绝，替换 key 的签名不会释放。

测试覆盖并发同请求、并发 same-epoch 不同 owner、进程重启、后端失败、C 提交失败、
`PENDING` 恢复、pending/completed 内容冲突，以及“更高 epoch + request 冲突”路径。

这里的 backend signature **只签 payload digest**。`KeyID/Owner/Epoch/RequestID` 与
签名结果一起被 fence store 原子持久化，但它们不是 HSM/FROST 签名覆盖的独立
attestation。若 receipt 要脱离数据库供第三方验证全部 metadata，需要另行定义
canonical receipt encoding，并用独立 attestation key 签名；本 sandbox 没有实现它。

### bbolt 部署边界

这是明确的 **single-active-instance** 设计。bbolt 的文件锁阻止第二个正常进程同时
打开数据库，进程内按 key 锁负责串行 A/B/C；它不是共识系统、分布式数据库或多副本
signer HA。不要把同一数据库复制给多个 active signer，也不要把数据库放到文件锁/
fsync 语义不可靠的网络文件系统上。commit 的实际持久性仍依赖操作系统、文件系统和
存储设备正确兑现同步写语义。

需要多副本 HA 时，应改用具有线性化写入和明确故障模型的共享状态层，或由密码协议/
设备提供等价的全局 fence；不能让各副本各自维护一份 bbolt 文件。

### etcd 多副本 fence

`fence.EtcdSigner` 是默认编译的多副本实现。它使用 caller-owned etcd client、
lease-backed `concurrency.Session` 和每个 logical key 独立的
`concurrency.Mutex`，并在阶段 A/C 事务中同时比较 mutex owner、lease ID 与
state/request 的 revision 和完整值。普通状态读取保持默认线性一致模式，不使用
`WithSerializable`。

etcd 只线性化 signer 元数据，不能撤销已经进入 HSM/MPC 的设备侧操作：进程在后端成功
而 `COMPLETED` 未确认时退出，记录仍为 `PENDING`，恢复可能再次签名。生产部署还必须为
client/peer 启用 mTLS、按专用 prefix 配置 RBAC，并接受 C 提交响应丢失时只能靠相同
RequestID 线性读取判定结果。

三节点 v3.6.12 localnet、lease/mutex ownership loss、leader/follower 故障和
v3.6 → v3.7 滚动兼容步骤见
[etcd fence runbook](runbooks/etcd-fence.md)。集成测试：

```bash
docker compose -f compose.etcd.yaml up -d --wait
make etcd-compose-test
docker compose -f compose.etcd.yaml down -v
```

## Backend contract 与软件 backend

`fence.Backend` 接收 logical key ID 和固定 32-byte、domain-separated payload digest，
返回算法标识、编码后的公钥与签名：

```go
type Backend interface {
    Sign(context.Context, string, fence.Digest) (fence.BackendResult, error)
}
```

`backend/software` 是默认 Ed25519 测试 backend，私钥直接位于进程内存。CLI 使用固定
demo seed 以便重启复现；该 seed 和实现都不得用于真实资产。

软件 CLI 示例：

```bash
go run ./cmd/signer-demo \
  -db /tmp/signer-fence.db \
  -owner worker-a -epoch 7 \
  -request withdrawal-42 \
  -message 'chain=1,to=alice,amount=10'
```

owner/epoch 在 CLI 中是显式输入，仅用于展示数据面状态机。CLI 不是 server，更不能
包装后直接暴露到公网。生产入口必须从经过认证的 control-plane grant、mTLS/SPIFFE
identity 或等价可信通道取得并校验 caller、key、owner、epoch 和 payload 的绑定，还要
在进入 signer 前完成交易语义、额度、nonce/UTXO、policy 与链特有保护检查。

## SoftHSM2 / PKCS#11 sandbox

`backend/pkcs11` 使用 `crypto11 v1.5.0`：

- 同时按 CKA_ID + CKA_LABEL 查找对象；同 ID 但 label 漂移时失败，不静默复用或
  生成重复身份；
- `Open` 默认只允许既有对象；只有 SoftHSM demo 显式设置 `CreateIfMissing=true`，
  才会在 token 内生成 P-256 key pair；
- demo 生成时 crypto11 请求 `sensitive=true, extractable=false`；既有对象的属性不能
  靠调用方猜测，必须通过 deployment acceptance 或厂商工具验证；
- token 对 fence digest 签名，返回 ASN.1 DER ECDSA signature；
- backend 在把结果交给 fence 层前用导出的公钥验证签名。

运行完整 sandbox：

```bash
docker compose build softhsm-signer
docker compose run --rm softhsm-signer
```

Docker daemon 不可用时，仍可执行 `go test ./...` 验证代码编译，并用
`docker compose config --quiet` 静态校验 Compose model。详细初始化、重启、epoch
升级和清理步骤见 [SoftHSM2 runbook](runbooks/softhsm2.md)。

**SoftHSM2 是软件 token，不是硬件安全边界。** 它把 key object 存在普通 volume，
适合验证 PKCS#11 session、object 与签名路径；它不提供真实 HSM 的防拆、硬件密钥隔离、
认证运维流程、设备 HA 或厂商审计保证。Compose 中的 PIN 也是公开 demo 值。

SoftHSM token volume 与 fence DB 必须按同一个 logical key identity 作为恢复单元备份、
恢复和验证。若 token 丢失但 DB 保留，历史 `COMPLETED` receipt 仍可读取和用其中的旧
公钥验证；同名新 token key 不能作为原 key 继续签名。本实现的 backend identity 绑定
会在 C 阶段拒绝新公钥，但 A 中的 fence/PENDING 仍保持持久化，需人工恢复正确 token
或执行显式、审计化的 key migration，而不能删库绕过。

## 真实 HSM existing-key-only 接入

`cmd/hsm-acceptance` 面向 AWS CloudHSM、Thales Luna、Entrust nShield 或其他提供
PKCS#11 的设备。它不会创建或修改密钥，要求：

- token label、serial、slot 三选一；
- 精确 `CKA_ID + CKA_LABEL`；
- 独立保存的 SubjectPublicKeyInfo SHA-256 pin；
- `CKM_ECDSA` signing capability；
- 安全属性证据、并发 challenge signing、关闭重连后的 key identity 稳定。

PIN/CU credential 只能从 mode `0600` 的普通文件读取。JSON report 不包含 PIN、私钥或
签名值。完整命令、AWS/Luna/nShield profile 和故障验收矩阵见
[真实 HSM runbook](runbooks/hardware-hsm.md)。

这仍不等于“项目已在真实硬件上认证”。通用 PKCS#11 module 无法证明其背后一定是物理
设备；上线必须把 report 与厂商 inventory、设备/partition serial、固件/FIPS 状态、HA
拓扑和审计日志交叉验证。本仓库能执行 SoftHSM compatibility test，但不会把它作为硬件
evidence。

## FROST Taproot/BIP-340 2-of-3 sandbox

`backend/frost` 直接运行 `multi-party-sig` 的公开协议 handler：

1. `alice`、`bob`、`carol` 三方执行 `KeygenTaproot` DKG；
2. 使用库定义的 `threshold=1`；该参数表示最多可腐化/容忍的 participant 数，签名
   要求 `threshold+1` 方，因此这里是 **2-of-3**；
3. 签名阶段只启动 `alice` 和 `bob` 的 `SignTaproot` handler；
4. 最终产生一个链上可验证的 64-byte BIP-340 Schnorr signature，并由 DKG public
   key 验证。它不是普通 multisig。

```bash
go run ./cmd/frost-demo -message 'threshold signing payload'
go test -run TestThreePartyDKGAndTwoPartyTaprootSigning ./backend/frost
```

上游固定版本的 [README](https://github.com/taurushq-io/multi-party-sig/blob/v0.7.0-alpha-2025-01-28/README.md)
明确声明该项目 “needs further testing and auditing to be production-ready”。本 sandbox
继承这一边界。

`backend/frost`/`cmd/frost-demo` 仍是把三个 share 放在同一进程内存的最小协议演示。
生产化练习另提供 `backend/frostcluster`、`cmd/mpc-coordinator` 和
`cmd/mpc-participant`：

- coordinator 只路由 public session metadata 与 Taurus protocol bytes，不创建或加载
  `TaprootConfig`；
- Alice/Bob/Carol 是独立 OS 进程，每个进程只拥有自己的 share 文件；
- participant/control 与 participant/coordinator 通道支持 TLS 1.3 双向认证和证书身份
  映射，loopback bearer token 仅用于本地测试；
- share 使用 `0600`、同目录 fsync 和原子 no-replace 持久化；静态 AES-GCM 只是可选的
  文件级保护，不等价于 KMS/HSM 隔离；
- 每个 DKG/signing session ID 在启动协议前写入私有 bbolt ledger，失败会话在进程重启
  后仍不可复用；
- 绕过固定 Taurus 版本会吞掉 CBOR error 的 `UnmarshalBinary`，执行严格 CBOR 解码并
  再校验 upstream canonical binary form；
- 测试用临时 CA 完成真实 TLS 1.3 双向握手，验证 URI SAN 身份映射、缺少客户端证书时
  握手失败，以及 CA 已信任但未授权的证书在应用层被拒绝；
- 测试实际启动一个 coordinator 与三个 participant 子进程，完成三方 DKG、Alice+Bob
  2-of-3 签名、BIP-340 验签、掉线 deadline 和 replay/身份/消息边界测试。

完整拓扑、mTLS 参数和演练步骤见
[跨进程 FROST runbook](runbooks/frost-cluster.md)。这仍不是已审计的 custody MPC：
coordinator 是可观察且可信的 relay，上游 Taurus 版本仍标注需要进一步审计；share 的
KMS/HSM envelope、reshare、跨地域恢复、端到端消息签名和恶意 coordinator 模型仍未实现。

## 未覆盖的生产能力

- control-plane grant 签发与验证、mTLS/SPIFFE 身份提取；
- policy engine、审批、限额、nonce/UTXO reservation、链特有 slashing protection；
- etcd 跨地域拓扑、灾备恢复，以及把 fence token/session authority 强制下沉到
  HSM/MPC participant；当前 etcd 路径只线性化 metadata/receipt，不能撤销已发出的密码调用；
- HSM operator ceremony、真实设备/固件/FIPS/HA 与审计控制面的现场验收；仓库只提供
  existing-key-only 接入和可执行 acceptance suite；
- MPC share 的 KMS/HSM envelope、不可回滚备份、resharing、coordinator HA、跨地域恢复、
  恶意 coordinator 防护与端到端消息签名；
- 业务审计外送与监控告警。

这些能力不能由“换成 HSM/MPC backend”自动获得。
