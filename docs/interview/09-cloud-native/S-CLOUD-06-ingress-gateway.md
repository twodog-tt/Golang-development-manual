---
id: S-CLOUD-06
title: Ingress、Gateway API 与南北向流量
module: cloud-native
level: senior
frequency: 4
go_version: "1.22+"
tags: [kubernetes, ingress, gateway-api, nginx, tls, load-balancer]
status: published
code_refs: []
sources:
  - https://kubernetes.io/docs/concepts/services-networking/ingress/
  - https://gateway-api.sigs.k8s.io/
---

# Ingress、Gateway API 与南北向流量

## 30 秒版（开场）

> **南北向流量** 从公网进入集群：可用传统 **Ingress** 或 **Gateway API**。Go 服务通常通过 ClusterIP 接入网关。TLS、超时、body 限制、WebSocket 和 gRPC 的具体配置取决于 controller；Gateway API 可用 HTTPRoute/GRPCRoute 表达标准化路由，不能把某个 nginx annotation 当成 Kubernetes 通用能力。

## 3 分钟版（一面深度）

1. **是什么**：Ingress Controller 把外部 HTTP(S) 路由到 Service；Gateway API 用 Gateway + HTTPRoute 替代注解魔法。
2. **为什么**：面试常问 Ingress vs Service LoadBalancer、如何做灰度 path、WebSocket 怎么配。
3. **怎么做**：Go API 用 host/path 路由；长连接单独评估 idle timeout、每次写 deadline 与 drain；网关和应用超时按各自语义形成端到端 deadline，不能简单比较两个数字；业务鉴权/限流放 API 网关或应用层。

## 10 分钟版（原理 + 图示）

```mermaid
flowchart LR
  Client[客户端] --> LB[云 LB / Ingress Controller]
  LB --> Ing[Ingress / HTTPRoute]
  Ing --> Svc[Service ClusterIP]
  Svc --> Pod1[Go Pod]
  Svc --> Pod2[Go Pod]
```

**Service 类型速查**

| 类型 | 场景 |
|------|------|
| ClusterIP | 集群内互访（默认） |
| NodePort | 调试、边缘 |
| LoadBalancer | 云厂商直出（贵） |
| ExternalName | DNS 别名 |

**Ingress 示例（nginx）**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: api
  annotations:
    nginx.ingress.kubernetes.io/proxy-read-timeout: "60"
    nginx.ingress.kubernetes.io/proxy-body-size: "10m"
spec:
  ingressClassName: nginx
  tls:
    - hosts: [api.example.com]
      secretName: api-tls
  rules:
    - host: api.example.com
      http:
        paths:
          - path: /v1
            pathType: Prefix
            backend:
              service:
                name: go-api
                port:
                  number: 8080
```

**Gateway API（HTTPRoute 片段）**

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api-route
spec:
  parentRefs:
    - name: public-gateway
  hostnames:
    - api.example.com
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /v1
      backendRefs:
        - name: go-api
          port: 8080
```

## 生产场景

- **WebSocket 行情推送**：确认 controller 支持 Upgrade/extended connect，并把 idle timeout 与心跳、写 deadline 对齐；或独立域名 + [S-NET-05](../06-network-governance/S-NET-05-websocket-gateway.md)
- **gRPC**：确认入口到 backend 的 HTTP/2/gRPC 支持；Gateway API 可用 GRPCRoute，传统 Ingress 常需要 controller-specific 配置
- **大文件上传**：`client_max_body_size` / Go `MaxBytesReader` 对齐
- **多环境**：staging/prod 不同 host；cert-manager 自动续期 TLS

## 排查与工具

- `kubectl describe ingress` → Address、Events、backend 404
- `curl -v https://api.example.com/v1/healthz` 对比 `kubectl port-forward`
- Controller 日志：nginx-ingress / ALB controller
- 502/504：上游 Pod 未 ready、超时过短、Go panic

## 架构取舍

| 方案 | 适用 |
|------|------|
| Ingress + nginx | 成熟、文档多 |
| Gateway API | 多租户路由、Canary 原生 |
| 云 ALB/NLB | 免运维 Controller |
| 应用外 API 网关（Kong/APISIX） | 复杂鉴权、插件、WAF |

**何时不用 Ingress**：纯内网 gRPC 东西向 → Service + Mesh（[S-SOL-04](../11-solution-architecture/S-SOL-04-bff-gateway-mesh.md)）。

## 追问链

1. **Ingress 和 Service 区别？** → Service 集群内负载均衡；Ingress 七层路由 + 域名/TLS。
2. **如何做金丝雀？** → Ingress weight 注解、Gateway API HTTPRoute 权重、或 Flagger（[S-ARCH-15](../03-system-design/S-ARCH-15-release-strategy.md)）。
3. **TLS 在哪终止？** → 多在 Ingress；Pod 内 mTLS 另说（Mesh）。
4. **Go 反向代理超时怎么配？** → 先区分连接、读 header、上游响应 header、响应空闲和请求总 deadline。外层 deadline 应给内层留清理余量；streaming/WebSocket 不能套普通请求的固定 `WriteTimeout`。

## 反模式与事故

- **Ingress 超时 60s 但 Go 长轮询 120s** → 504
- **WebSocket 走默认短 timeout** → 频繁断连
- 路径规则重叠且未按规范/目标 controller 验证 → 升级 controller 后路由行为变化
- **TLS Secret 过期未监控** → 全站 HTTPS 失败

## 代码示例

```go
srv := &http.Server{
    Addr:              ":8080",
    Handler:           router,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       30 * time.Second,
    WriteTimeout:      60 * time.Second, // 普通 API 示例；流式接口需单独设计
    IdleTimeout:       120 * time.Second,
}
```

## 延伸阅读

- [Kubernetes Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/)
- [Gateway API](https://gateway-api.sigs.k8s.io/)
- [S-NET-05 WebSocket 网关](../06-network-governance/S-NET-05-websocket-gateway.md)
