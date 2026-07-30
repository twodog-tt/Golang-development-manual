---
id: S-CLOUD-04
title: 滚动发布、探针与 PodDisruptionBudget
module: cloud-native
level: senior
frequency: 5
go_version: "1.22+"
tags: [kubernetes, rolling-update, probes, pdb, graceful-shutdown]
status: published
code_refs: [examples/senior/graceful_shutdown]
sources:
  - https://kubernetes.io/docs/concepts/workloads/controllers/deployment/
  - https://kubernetes.io/docs/tasks/run-application/configure-pdb/
---

# 滚动发布、探针与 PodDisruptionBudget

## 30 秒版（开场）

> K8s **Deployment 滚动发布** 用 `maxSurge` / `maxUnavailable` 控制替换速度；**readiness** 控制是否进入 Service 流量，**liveness** 失败会触发重启。**PDB 只约束通过 Eviction API 的部分自愿中断（如 drain），不约束 Deployment 自身滚动更新，也不能阻止非自愿故障。**

## 3 分钟版（精讲深度）

1. **是什么**：Deployment 逐步创建新 ReplicaSet Pod、缩旧 Pod；探针由 kubelet 定期 HTTP/TCP/exec 检查。
2. **为什么**：错误探针 → 502、反复重启、发布雪崩；无 PDB → 节点 drain 时服务全挂。
3. **怎么做**：startup 覆盖慢启动；liveness 只检查进程是否不可恢复地失活；readiness 判断实例能否服务并避免共享依赖抖动导致全体摘流量；`preStop`/SIGTERM 与 `terminationGracePeriodSeconds` 协同；PDB 按副本数、quorum 和 drain 需求设置。

## 10 分钟版（原理 + 图示）

```mermaid
sequenceDiagram
  participant Ctrl as Deployment Controller
  participant Old as Pod v1
  participant New as Pod v2
  participant Svc as Service
  Ctrl->>New: 创建并等待 readiness OK
  Svc->>New: 加入 Endpoints
  Ctrl->>Old: 缩减旧 ReplicaSet
  Old->>Old: kubelet 进入 Pod termination
  Old->>Old: 优雅 drain 连接
  Ctrl->>Old: 终止
```

**探针对比**

| 探针 | 失败后果 | Go 服务建议 |
|------|----------|-------------|
| **startup** | 重启（启动慢时用） | 冷启动 >30s 时启用 |
| **liveness** | 重启 Pod | 轻量 `/livez`，勿查外部依赖 |
| **readiness** | 从 Service 摘除 | `/readyz` 判断本实例是否可服务；依赖检查需考虑降级和级联摘流量 |

**推荐 Deployment 片段**

```yaml
spec:
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 0        # 关键服务：先启新再杀旧
  template:
    spec:
      terminationGracePeriodSeconds: 60
      containers:
        - name: app
          lifecycle:
            preStop:
              sleep:
                seconds: 5
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8080
            periodSeconds: 5
            failureThreshold: 3
          livenessProbe:
            httpGet:
              path: /livez
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 10
```

**PDB 示例**

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: api-pdb
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: api
```

PDB 影响 `kubectl drain` 等调用 Eviction API 的操作。直接删除 Pod/Deployment、Deployment rollout、HPA 修改副本数以及节点/硬件故障都不由 PDB 兜底；滚动发布可用性看 Deployment strategy、readiness、容量和应用优雅关闭。

## 生产场景

- **发布 502**：新 Pod readiness 未就绪已接流量 → 调 `minReadySeconds`、确保 readiness 严格
- **老 Pod 被强杀**：`terminationGracePeriodSeconds` 过短或 Go 未处理 SIGTERM → 见 [S-CODE-03](../08-coding-senior/S-CODE-03-graceful-shutdown.md)
- **节点维护**：`kubectl drain` 受 PDB 约束；与 [S-CLOUD-01](./S-CLOUD-01-k8s-scheduling.md) QoS 联动
- **WebSocket 服务**：滚动时需客户端重连或网关 sticky；见 [S-NET-05](../06-network-governance/S-NET-05-websocket-gateway.md)

## 排查与工具

- `kubectl rollout status deployment/api`
- `kubectl describe pod` → Readiness probe failed / Liveness probe failed
- `kubectl get endpoints` → 是否仍有就绪后端
- 对比发布前后错误率（与 [S-ARCH-15](../03-system-design/S-ARCH-15-release-strategy.md) 金丝雀指标一致）

## 架构取舍

| maxUnavailable 示例 | 取舍 |
|---------------------|------|
| 0 | 先扩后缩，需额外容量；仍要考虑配额和调度失败 |
| 正数/百分比 | 减少 surge 资源，但发布期间可用容量下降 |

具体值由最小安全容量、启动速度、配额与故障域决定，不能按“核心服务固定 0、普通服务固定 25%”背诵。

**何时不用 Deployment**：有状态单主 → StatefulSet；一次性任务 → Job。

## 深挖问答

1. **readiness 和 liveness 能查同一个 DB 吗？** → 不推荐；DB 抖动会 liveness 杀全体 Pod。
2. **preStop sleep 5s 干什么？** → 可给 EndpointSlice/LB 状态传播留缓冲，但只是经验性补偿。`preStop` 在 TERM 前执行，且其耗时计入同一个 termination grace period；极简镜像没有 shell 时不要用 `exec /bin/sh sleep`。
3. **PDB 和 HPA 冲突？** → HPA 直接修改 workload 的副本数，通常不受 PDB 阻止；应确保 `minReplicas`、PDB、实际最小安全容量三者逻辑一致，避免 drain 时无可驱逐空间。
4. **Go 1.22+ http.Server Shutdown 超时？** → 应小于 `terminationGracePeriodSeconds` 留余量给清理。

## 反模式与事故

- **readiness 永远 200** → DB 挂了仍接流量
- **liveness 查 `/readyz`** → 依赖故障导致无限重启
- 没有合理 PDB/拓扑分布时做 node drain → 可能同时损失过多副本；PDB 也不能防节点突然宕机
- **maxUnavailable=50% 核心服务** → 容量腰斩触发级联超时

## 代码示例

```go
func main() {
    srv := &http.Server{Addr: ":8080", Handler: router}
    go func() { _ = srv.ListenAndServe() }()

    signalCtx, stop := signal.NotifyContext(
        context.Background(),
        syscall.SIGTERM,
        syscall.SIGINT,
    )
    defer stop()
    <-signalCtx.Done()

    ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        _ = srv.Close()
    }
}
```

## 延伸阅读

- [Deployment 滚动更新](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)
- [PodDisruptionBudget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/)
- [S-CODE-03 优雅关闭](../08-coding-senior/S-CODE-03-graceful-shutdown.md)
