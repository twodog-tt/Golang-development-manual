---
id: S-CLOUD-08
title: ConfigMap、Secret 与 Go 配置热更新
module: cloud-native
level: senior
frequency: 4
go_version: "1.22+"
tags: [kubernetes, configmap, secret, viper, hot-reload, twelve-factor]
status: published
code_refs: []
sources:
  - https://kubernetes.io/docs/concepts/configuration/configmap/
  - https://kubernetes.io/docs/concepts/configuration/secret/
---

# ConfigMap、Secret 与 Go 配置热更新

## 30 秒版（开场）

> **ConfigMap** 存非敏感配置，**Secret** 存密钥（Base64 非加密）；挂载为 **环境变量** 或 **Volume 文件**。Go 服务推荐：**启动读 env + 可选文件 watch 热更**；Secret **不进镜像**（[S-CLOUD-02](./S-CLOUD-02-docker-multistage.md)）。生产关键词：**12-Factor、Reloader、immutable Secret、敏感配置轮换**。

## 3 分钟版（一面深度）

1. **是什么**：K8s 原生配置分发；Deployment 变更 ConfigMap 不会自动重启 Pod（除非挂载 subPath 等特殊情况）。
2. **为什么**：面试问「改配置要不要发版」；错用 Secret 泄漏、subPath 不更新是常见坑。
3. **怎么做**：非敏感可热更（watch 文件）；敏感改 Secret + 滚动重启或 Reloader；Go 用 `viper`/自研 atomic.Value 热加载。

## 10 分钟版（原理 + 图示）

```mermaid
flowchart TB
  CM[ConfigMap / Secret] -->|env| Pod[Go Pod]
  CM -->|volume mount| File[/etc/config/app.yaml]
  File -->|fsnotify| App[配置热更]
  Ext[External Secrets / Vault] --> Secret[K8s Secret]
  Secret --> Pod
```

**挂载方式对比**

| 方式 | 热更新 | 适用 |
|------|--------|------|
| env | 否（需重启 Pod） | 简单、少量 key |
| volume 整目录 | 是（kubelet 同步，有延迟） | 配置文件 |
| volume subPath 单文件 | **否** | 避免使用若需热更 |
| Secret Store CSI | 依赖驱动 | 与 Vault/AWS SM 集成 |

**Deployment 片段**

```yaml
spec:
  template:
    spec:
      containers:
        - name: app
          envFrom:
            - configMapRef:
                name: app-config
          env:
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: app-secret
                  key: password
          volumeMounts:
            - name: config
              mountPath: /etc/config
              readOnly: true
      volumes:
        - name: config
          configMap:
            name: app-config
            items:
              - key: app.yaml
                path: app.yaml
```

**Go 热更新示例**

```go
type Config struct {
    RateLimitRPS int `yaml:"rate_limit_rps"`
    FeatureX     bool `yaml:"feature_x"`
}

var cfg atomic.Value // stores Config

func watchConfig(path string) {
    w, _ := fsnotify.NewWatcher()
    _ = w.Add(filepath.Dir(path))
    for {
        select {
        case ev := <-w.Events:
            if ev.Op&(fsnotify.Write|fsnotify.Create) != 0 {
                if c, err := loadConfig(path); err == nil {
                    cfg.Store(c)
                }
            }
        case err := <-w.Errors:
            slog.Error("config watch", "err", err)
        }
    }
}

func GetConfig() Config {
    return cfg.Load().(Config)
}
```

## 生产场景

- **风控规则 / 限流阈值**：ConfigMap 热更，无需重启（与 [S-ARCH-08](../03-system-design/S-ARCH-08-rate-limiting.md) 联动）
- **RPC 端点切换**：链上节点 URL 变更（[S-BC-02](../12-blockchain-web3/S-BC-02-go-ethereum-rpc.md)）
- **Secret 轮换**：DB 密码双写期 → 新 Secret 版本 + 滚动发布
- **多环境**：configMap 按 namespace 隔离；避免 prod secret 进 staging

## 排查与工具

- `kubectl get cm,secret -n NS`
- `kubectl describe cm app-config` → 是否被 Pod 引用
- Pod 内 `cat /etc/config/app.yaml` 对比期望
- [Reloader](https://github.com/stakater/Reloader) / Argo CD 触发滚动

## 架构取舍

| 方案 | 说明 |
|------|------|
| K8s ConfigMap | 简单、GitOps 友好 |
| Nacos / Apollo | 动态配置、灰度、审计 |
| Vault + CSI | 企业密钥治理 |
| 仅 env | 12-Factor 极简，无热更 |

**何时不用热更**：涉及结构变更、连接池重建 → 滚动发布更安全。

## 追问链

1. **Secret Base64 安全吗？** → 否，仅编码；靠 RBAC、etcd 加密 at rest、External Secrets。
2. **改 ConfigMap 后 Pod 何时感知？** → volume 默认 ~分钟级同步；可 inotify；env 必须重启。
3. **subPath 为什么不更新？** → subPath 绑定创建时 inode；用目录挂载或重启。
4. **Go 热更线程安全？** → `atomic.Value` / `RWMutex`；业务读配置走快照，避免半写。

## 反模式与事故

- **密钥写 Dockerfile / git** → 泄漏
- **ConfigMap 存 DB 密码** → 应用 Secret + 最小 RBAC
- **热更改连接串不重建连接池** → 仍连旧库
- **全量配置一个 giant yaml** → 难 review；按域拆分 CM

## 代码示例

```go
// 启动：env 覆盖文件（12-Factor）
func loadConfig(path string) (Config, error) {
    var c Config
    if b, err := os.ReadFile(path); err == nil {
        _ = yaml.Unmarshal(b, &c)
    }
    if v := os.Getenv("RATE_LIMIT_RPS"); v != "" {
        c.RateLimitRPS, _ = strconv.Atoi(v)
    }
    return c, nil
}
```

## 延伸阅读

- [ConfigMap](https://kubernetes.io/docs/concepts/configuration/configmap/)
- [Secrets](https://kubernetes.io/docs/concepts/configuration/secret/)
- [S-CLOUD-04 滚动发布](./S-CLOUD-04-rolling-update-probes-pdb.md)
