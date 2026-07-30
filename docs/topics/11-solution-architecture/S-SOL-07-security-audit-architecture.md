---
id: S-SOL-07
title: 安全与审计的全局架构
module: solution-architecture
level: architect
frequency: 4
go_version: "1.22+"
tags: [security-architecture, zero-trust, audit, pii, compliance, architect]
status: published
code_refs: []
sources:
  - https://owasp.org/www-project-application-security-verification-standard/
  - https://cheatsheetseries.owasp.org/
  - https://www.nist.gov/cyberframework
---

# 安全与审计的全局架构

## 30 秒版（开场）

> 架构师定义零信任与最小权限边界，并让审计日志 **追加写、访问受控、可检测篡改且满足留存策略**。mTLS 认证工作负载身份，但不自动授予业务权限；JWT、密钥、PII、供应链与恢复流程都要有独立控制。

## 3 分钟版（精讲深度）

1. **是什么**：安全架构 = 威胁建模 + 控制措施 + 可验证合规。
2. **为什么**：架构师对 **数据泄露、越权、供应链** 负最终设计责任；后端面 increasingly 问 security by design。
3. **怎么做**：STRIDE 威胁建模；分层防御（网关 WAF → 服务 RBAC → 数据加密）；集中审计日志到不可变存储。

## 10 分钟版（原理 + 图示）

```mermaid
flowchart TB
  User[用户] --> WAF[WAF / 网关]
  WAF --> Auth[IdP / OAuth2]
  Auth --> Svc[Go 微服务]
  Svc --> Policy[OPA / RBAC]
  Svc --> Vault[密钥管理]
  Svc --> Audit[审计日志 → SIEM]
  Svc --> DB[(加密 at rest)]
```

**架构师必讲控制面**

| 域 | 措施 |
|----|------|
| 身份 | SSO、MFA、服务账号 rotation |
| 授权 | RBAC/ABAC；[S-NET-04 JWT](../06-network-governance/S-NET-04-jwt-auth.md) 边界 |
| 数据 | 按兼容与合规策略设置 TLS 最低版本、敏感字段保护、脱敏日志 |
| 供应链 | 依赖扫描、最小 base 镜像、SBOM |
| 审计 | who/when/what/tenant；WORM 或 hash 链 |

**Go 落地清单**

- `crypto/tls` 设置组织批准的最低版本与 cipher policy
- JWT：使用经批准的非对称算法（如 RSA/ECDSA/EdDSA）、短 access + refresh rotation
- SQL：参数化；ORM 仍防 raw 拼接
- 容器：非 root、read-only rootfs

**威胁建模 STRIDE（简表）**

| 威胁 | 示例 | 缓解 |
|------|------|------|
| Spoofing | 伪造 JWT | 验签 + mTLS |
| Tampering | 改订单金额 | 服务端计价 |
| Repudiation | 抵赖操作 | 审计日志 |
| Info Disclosure | 日志泄露 PII | 脱敏 |
| DoS | 接口刷爆 | 限流 [S-ARCH-08](../03-system-design/S-ARCH-08-rate-limiting.md) |
| Elevation | 越权 tenant | [S-SOL-05](./S-SOL-05-multi-tenant-saas.md) |

## 生产场景

- **等保 / SOC2**：架构师输出控制矩阵映射到系统设计
- **密钥轮换**：Vault 动态 DB 凭证，Go 服务热加载
- **LLM 场景**：prompt 不进日志；RAG 文档 ACL

## 排查与工具

- OWASP ASVS L2 自检
- 渗透测试、SAST/DAST CI
- 审计：独立权限域、append-only/WORM 或签名/hash 链、集中 SIEM 与定期完整性验证

## 架构取舍

| 内网互信 | 零信任 |
|----------|--------|
| 简单 | 每调用都验身份 |

**合规行业**：独立安全架构师评审；后端架构师配合落地。

## 深挖问答

1. **服务间还要鉴权吗？** → 要；mTLS/service identity 先认证调用方，再由服务或 policy engine 做 operation/resource authorization。
2. **审计日志谁删？** → 运维无删权限；break-glass 双审批。
3. **GDPR 删除权？** → 按适用法规实现删除/匿名化工作流、索引与缓存清理、备份过期和法律留存例外；soft delete 本身不是物理擦除。
4. **Go supply chain？** → `go.sum` 校验模块内容一致性但不代表依赖可信或无漏洞；结合 `govulncheck`、依赖审查、构建 provenance/SBOM、签名与受控 proxy。

## 反模式与事故

- **`.env` 进 Git** → 密钥泄露
- **内网 HTTP 明文** → 横向移动
- **审计只打 info 无结构化** → 无法取证

## 代码示例

```go
// slog 脱敏
logger.Info("user login", "user_id", uid, "ip", ip) // 不打 phone/email
```

## 延伸阅读

- [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/)
- [OWASP Cheat Sheet Series](https://cheatsheetseries.owasp.org/)
