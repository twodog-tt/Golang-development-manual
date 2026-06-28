# 09 云原生（扩展）

8 题 | P1～P2 | [返回索引](../README.md)

| ID | 题目 | 频率 |
|----|------|------|
| [S-CLOUD-01](./S-CLOUD-01-k8s-scheduling.md) | K8s 调度与 Go 资源 limit | ⭐⭐⭐⭐ |
| [S-CLOUD-02](./S-CLOUD-02-docker-multistage.md) | Docker 多阶段构建 | ⭐⭐⭐⭐ |
| [S-CLOUD-03](./S-CLOUD-03-opentelemetry.md) | OpenTelemetry 接入 | ⭐⭐⭐⭐ |
| [S-CLOUD-04](./S-CLOUD-04-rolling-update-probes-pdb.md) | 滚动发布、探针与 PDB | ⭐⭐⭐⭐⭐ |
| [S-CLOUD-05](./S-CLOUD-05-hpa-autoscaling.md) | HPA 与自定义指标扩缩容 | ⭐⭐⭐⭐ |
| [S-CLOUD-06](./S-CLOUD-06-ingress-gateway.md) | Ingress / Gateway API 南北向流量 | ⭐⭐⭐⭐ |
| [S-CLOUD-07](./S-CLOUD-07-k8s-troubleshooting.md) | OOMKilled / CrashLoop / Evicted 排查 | ⭐⭐⭐⭐⭐ |
| [S-CLOUD-08](./S-CLOUD-08-configmap-secret.md) | ConfigMap / Secret 与配置热更新 | ⭐⭐⭐⭐ |

## 适用场景

- 岗位 JD 含 **K8s / 云原生 / SRE**
- 二面问 **部署、镜像、可观测性、排障** 落地细节
- 与 [S-CONC-04 GOMAXPROCS](../01-runtime-concurrency/S-CONC-04-gomaxprocs.md)、[S-CODE-03 优雅关闭](../08-coding-senior/S-CODE-03-graceful-shutdown.md)、[S-ARCH-15 发布策略](../03-system-design/S-ARCH-15-release-strategy.md) 交叉复习

## 推荐刷题顺序

镜像构建(02) → 调度与 limit(01) → 探针与滚动(04) → Ingress(06) → HPA(05) → 配置(08) → 排障(07) → OTel(03)
