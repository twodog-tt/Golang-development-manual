# 真实 HSM 接入与验收

本手册用于把 `backend/pkcs11` 接到已经完成初始化和建钥仪式的真实 HSM。它不是采购、
初始化、FIPS 合规或密钥仪式的替代流程。

## 验收结论的边界

`cmd/hsm-acceptance` 会：

- 只查找已有的 P-256 私钥，不生成、删除、导入、导出或轮换对象；
- 固定 PKCS#11 module、token label/serial/slot 三选一、`CKA_ID`、`CKA_LABEL`；
- 要求调用方提供独立保存的 SubjectPublicKeyInfo DER SHA-256；
- 查询 `CKM_ECDSA` 和 `CKA_TOKEN/PRIVATE/SENSITIVE/ALWAYS_SENSITIVE/
  EXTRACTABLE/NEVER_EXTRACTABLE/SIGN`；
- 并发执行 challenge signing、在进程内验签，关闭并重新连接后再次校验 key identity；
- 输出不含 PIN、私钥和签名值的 JSON evidence report。

PKCS#11 module 可以是软件实现，也可以虚报属性，所以报告不能单独证明“这是物理 HSM”。
上线门禁必须把报告中的 manufacturer/model/serial/firmware 与厂商原生 inventory、采购
资产、FIPS certificate、HA topology 和审计日志交叉核对。

## HSM 侧前置条件

HSM 管理员通过双人控制或组织要求的 ceremony 创建密钥，并记录：

1. P-256/`secp256r1` key pair；
2. 精确 `CKA_ID` bytes 和 `CKA_LABEL`；
3. `CKA_TOKEN=true`、`CKA_PRIVATE=true`、`CKA_SENSITIVE=true`、
   `CKA_EXTRACTABLE=false`、`CKA_SIGN=true`；
4. token/partition/cluster identity、设备序列号、固件和 client library 版本；
5. 导出的公钥以及 SubjectPublicKeyInfo DER SHA-256；
6. key owner、备份/恢复策略、删除与轮换审批、审计日志接收方。

AWS CloudHSM 的官方最佳实践特别指出 key 默认可能是 extractable；不能因为“密钥在
HSM 中生成”就推断 `CKA_EXTRACTABLE=false`。必须在 ceremony 中显式设置并由本工具或
厂商工具复核。

## 凭据文件

工具只从普通文件读取 PIN/CU credential，拒绝 group/world-readable 文件：

```bash
install -m 0600 /dev/null /run/secrets/signer-hsm-pin
# 通过 secrets manager/受控交互写入，不要把值放进 shell history。
```

AWS CloudHSM 的 PKCS#11 `C_Login` PIN 格式是 `<CU_user_name>:<password>`；Luna 或
nShield 使用对应 partition/token 的认证值。不要把 PIN 放在命令行参数、环境变量、
Compose 文件或 Git 中。

## 运行通用验收

`CKA_ID` 必须按原始 bytes 转成十六进制。下面的 fingerprint 必须来自建钥仪式输出，
不能先信任当前 signer 读取的 key，再把同一次读取当成独立 pin：

```bash
cd examples/signer-project

go run ./cmd/hsm-acceptance \
  -module /vendor/pkcs11/library.so \
  -token-serial '<exact-token-serial>' \
  -pin-file /run/secrets/signer-hsm-pin \
  -logical-key treasury-p256-v1 \
  -object-id-hex '<hex-encoded-cka-id>' \
  -object-label treasury-p256-v1 \
  -expected-spki-sha256 '<64-lowercase-hex-chars>' \
  -max-sessions 8 \
  -pool-wait-timeout 10s \
  -concurrency 4 \
  -signatures 32 \
  -output /var/lib/signer/evidence/hsm-acceptance.json
```

label、serial 和 slot 必须只设置一个。优先选择在设备替换/重启语义下由厂商保证稳定的
selector；不要凭经验假设 slot number 永久不变。

默认 `-require-attribute-evidence=true`。若某厂商故意不公开
`CKA_ALWAYS_SENSITIVE/CKA_NEVER_EXTRACTABLE`，只有在厂商原生工具提供等价、可审计证据
时才允许使用 `-require-attribute-evidence=false`；任何明确返回的危险值仍会失败。

## 厂商 profile

### AWS CloudHSM Client SDK 5

- 先按官方流程安装/配置 Client SDK 5、bootstrap cluster、创建最小权限 CU；
- Linux module 通常位于 `/opt/cloudhsm/lib/libcloudhsm_pkcs11.so`，以实际安装包为准；
- PIN 文件内容使用 `CU_username:password`；
- SDK 5 的 PKCS#11 实现遵循 2.40，并支持 `CKM_ECDSA`；
- cluster key availability、单 HSM 特殊配置和 SDK 版本支持窗口必须使用当前官方文档
  核验，不能固化为本项目的默认假设。

官方资料：

- [Client SDK 5 PKCS#11](https://docs.aws.amazon.com/cloudhsm/latest/userguide/pkcs11-library.html)
- [安装和配置](https://docs.aws.amazon.com/cloudhsm/latest/userguide/pkcs11-library-install.html)
- [PKCS#11 登录格式](https://docs.aws.amazon.com/cloudhsm/latest/userguide/pkcs11-pin.html)
- [机制列表](https://docs.aws.amazon.com/cloudhsm/latest/userguide/pkcs11-mechanisms.html)
- [key attributes](https://docs.aws.amazon.com/cloudhsm/latest/userguide/pkcs11-attributes.html)
- [key management best practices](https://docs.aws.amazon.com/cloudhsm/latest/userguide/bp-hsm-key-management.html)

### Thales Luna Network HSM

- 使用 Luna Client 已建立并验证的 client-to-partition connection；
- full client 常见路径为 `/usr/safenet/lunaclient/lib/libCryptoki2_64.so`；
  minimal client 的实际路径可能是 `/usr/local/luna/libs/64/libCryptoki2.so`；
- HA group/cluster、partition serial、firmware/client 兼容性和 FIPS mode 用 LunaCM/
  厂商控制面验证；
- 本工具不会配置 Luna HA group，也不会证明 failover 已启用。

官方资料：

- [Luna 最新产品文档与版本](https://thalesdocs.com/gphsm/luna/7/docs/network/Content/Home_Luna.htm)
- [Linux minimal client 文件布局](https://thalesdocs.com/gphsm/luna/7/docs/network/Content/install/client_install/linux_minimal_install_overview-and-prep.htm)
- [Luna HA groups](https://thalesdocs.com/gphsm/luna/7/docs/network/Content/admin_partition/ha/ha.htm)
- [升级路径](https://thalesdocs.com/gphsm/luna/7/docs/network/Content/admin_hsm/updates/upgrade.htm)

### Entrust nShield Security World

- Linux module 常见路径为 `/opt/nfast/toolkits/pkcs11/libcknfast.so`，以安装版本为准；
- OCS/K-of-N、preload、load-sharing/HSM Pool 等认证和可用性语义属于 nShield 控制面；
- nShield token object 可能以厂商定义的加密形式存放在 host disk，不能套用“所有 object
  bytes 都物理存于设备内”的表述。

官方资料：

- [nShield PKCS#11 reference](https://nshielddocs.entrust.com/security-world-docs/pkcs-11-reference-guide-for-nshield-security-world-v13-9-0.pdf)

## 上线故障验收

在 staging 使用与生产相同型号、固件、client library 和 HA topology，至少保存以下
evidence：

| 场景 | 必须观察到的结果 |
|---|---|
| 单个 signer 重启 | 重新连接后 SPKI fingerprint 不变，签名可验证 |
| 一个 HSM/partition member 不可用 | 只在厂商宣称的 HA 模式下继续；无 quorum/无 key 时 fail closed |
| signer 到 HSM 网络 timeout/reset | 请求返回明确 backend unavailable；不得把未确认签名当成功 |
| session pool 饱和 | 在 `pool-wait-timeout` 内成功或明确超时，不无限堆积 goroutine |
| client library/firmware rolling upgrade | N 和 N-1 组合都执行 acceptance；检查厂商 compatibility matrix |
| key identity 被替换 | expected SPKI pin 和 fence backend identity 都拒绝 |
| audit sink 中断 | 根据组织策略阻断签名或告警；不能静默丢失审计 |
| 备份恢复/灾备切换 | 恢复后 key identity、attributes、访问主体和审计连续性全部复核 |

HSM 调用一旦发给设备通常不能由 Go `context` 中止。timeout 只控制调用方等待，并不证明
设备没有完成签名；因此它与 signer fence 的 B/C 崩溃窗口一样，不能宣称 backend
exactly-once。
