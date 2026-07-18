# 多副本线性一致 signer fence：etcd 运行与验收

本工作包把单机 bbolt fence 的状态机扩展为默认编译的多副本实现
`fence.EtcdSigner`。三节点故障集成测试仍使用 `etcd_integration` build tag，
避免普通单元测试依赖外部服务。

## 一致性边界

每个签名请求依次经过：

1. 副本内 per-key lock，避免同一个 etcd Session 被同进程并发重入；
2. `concurrency.Mutex` 获取跨副本 per-key 所有权；
3. 阶段 A：线性一致事务写入最高 `epoch/owner` 和请求 `PENDING`；
4. 阶段 B：调用 HSM/MPC `Backend.Sign`；
5. 阶段 C：线性一致事务写入 backend identity、签名回执和
   `COMPLETED`；
6. 只有阶段 C 得到确定的成功响应后才向调用方返回回执。

阶段 A/C 的事务都比较：

- `Mutex.IsOwner()` 对应的 lock create revision；
- lock key 仍绑定当前 Session lease；
- state/request 的 `ModRevision` 和完整原值，或不存在时的
  `Version == 0`。

普通 `State` 使用默认的线性一致 `Get`，`LookupRequest` 使用只读事务。
代码没有使用 `WithSerializable`。etcd watch 不作为授权或完成依据。

这些保证不等于 backend exactly-once：

- 进程在 B 成功、C 提交前退出时，etcd 中仍是 `PENDING`；
- C 已提交但响应丢失时，本次调用不会释放内存中的签名；重试通过线性一致
  读取恢复 `COMPLETED`；
- 对 `PENDING` 的恢复可能再次调用 HSM/MPC。当前 `Backend` 接口没有接收
  `RequestID`，无法把业务幂等键传给设备；
- lease 丢失或 Mutex owner compare 失败时，旧副本不能提交回执，但设备侧
  已发生的签名操作无法由 etcd 回滚。

## Go 依赖

生产基线采用仍受支持的 N-1 minor：`v3.6.12`。etcd 只维护 current 和
previous minor；在 2026-07 的 current `v3.7.0` 发布后，不应再以 v3.5
作为新生产基线。依据：[官方版本维护策略](https://etcd.io/docs/v3.7/op-guide/versioning/)
和 [v3.6 → v3.7 升级指南](https://etcd.io/docs/v3.7/upgrades/upgrade_3_7/)。

`client/v3 v3.6.12` 要求 Go 1.25。本模块已把 Go directive 升级为
`1.25.0`，并固定以下直接依赖：

```bash
go.etcd.io/etcd/client/v3 v3.6.12
go.etcd.io/etcd/api/v3 v3.6.12
```

`client/v3/concurrency` 属于同一个模块；代码还直接使用
`go.etcd.io/etcd/api/v3/mvccpb` 读取事务返回的 revision/lease 元数据，
因此 `client/v3` 和 `api/v3` 都应固定为 `v3.6.12`。

`v3.7.0` 是升级兼容候选，不是本工作包的初始生产基线。升级验收至少覆盖：

1. `v3.6.12` client + 三节点 `v3.6.12` server；
2. `v3.6.12` client + `v3.6.12/v3.7.0` rolling mixed cluster；
3. `v3.6.12` client + 全量 `v3.7.0` server。

官方要求从 v3.6 升级到 v3.7 前所有成员至少为 v3.6.11，且只支持逐 minor
滚动升级。升级前必须快照并完整运行本页的 fence 故障验收。

etcd fence 的无外部服务单元测试属于默认测试集：

```bash
go test ./fence
```

## 启动本地三节点 etcd

仓库提供固定版本和镜像 digest 的 Compose 集群，宿主端口为
`12379/22379/32379`：

```bash
docker compose -f compose.etcd.yaml up -d --wait
docker compose -f compose.etcd.yaml ps
```

下面的手工命令用于演示 etcd 官方静态 bootstrap 形态；日常验收优先使用上面的
Compose 文件。数据卷会持久化，不要复用生产名称或生产数据。

```bash
export ETCD_VERSION=v3.6.12
export ETCD_IMAGE=gcr.io/etcd-development/etcd:${ETCD_VERSION}
export ETCD_CLUSTER_TOKEN=signer-fence-localnet-v1
export ETCD_CLUSTER='signer-etcd-1=http://signer-etcd-1:2380,signer-etcd-2=http://signer-etcd-2:2380,signer-etcd-3=http://signer-etcd-3:2380'

docker network create signer-etcd-net
docker volume create signer-etcd-1
docker volume create signer-etcd-2
docker volume create signer-etcd-3

docker run -d --name signer-etcd-1 --network signer-etcd-net \
  -p 12379:2379 -v signer-etcd-1:/etcd-data \
  "${ETCD_IMAGE}" /usr/local/bin/etcd \
  --name signer-etcd-1 --data-dir /etcd-data \
  --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://127.0.0.1:12379 \
  --listen-peer-urls http://0.0.0.0:2380 \
  --initial-advertise-peer-urls http://signer-etcd-1:2380 \
  --initial-cluster "${ETCD_CLUSTER}" \
  --initial-cluster-state new \
  --initial-cluster-token "${ETCD_CLUSTER_TOKEN}"

docker run -d --name signer-etcd-2 --network signer-etcd-net \
  -p 22379:2379 -v signer-etcd-2:/etcd-data \
  "${ETCD_IMAGE}" /usr/local/bin/etcd \
  --name signer-etcd-2 --data-dir /etcd-data \
  --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://127.0.0.1:22379 \
  --listen-peer-urls http://0.0.0.0:2380 \
  --initial-advertise-peer-urls http://signer-etcd-2:2380 \
  --initial-cluster "${ETCD_CLUSTER}" \
  --initial-cluster-state new \
  --initial-cluster-token "${ETCD_CLUSTER_TOKEN}"

docker run -d --name signer-etcd-3 --network signer-etcd-net \
  -p 32379:2379 -v signer-etcd-3:/etcd-data \
  "${ETCD_IMAGE}" /usr/local/bin/etcd \
  --name signer-etcd-3 --data-dir /etcd-data \
  --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://127.0.0.1:32379 \
  --listen-peer-urls http://0.0.0.0:2380 \
  --initial-advertise-peer-urls http://signer-etcd-3:2380 \
  --initial-cluster "${ETCD_CLUSTER}" \
  --initial-cluster-state new \
  --initial-cluster-token "${ETCD_CLUSTER_TOKEN}"
```

健康检查：

```bash
export ETCD_ENDPOINTS='http://127.0.0.1:12379,http://127.0.0.1:22379,http://127.0.0.1:32379'
export ETCD_DOCKER_ENDPOINTS='http://signer-etcd-1:2379,http://signer-etcd-2:2379,http://signer-etcd-3:2379'
docker exec signer-etcd-1 /usr/local/bin/etcdctl \
  --endpoints="${ETCD_DOCKER_ENDPOINTS}" endpoint health
docker exec signer-etcd-1 /usr/local/bin/etcdctl \
  --endpoints="${ETCD_DOCKER_ENDPOINTS}" endpoint status -w table
```

Go 集成测试从宿主机使用 `ETCD_ENDPOINTS` 的三个映射端口；容器内健康检查使用
Docker DNS 名称组成的 `ETCD_DOCKER_ENDPOINTS`。这里已经显式列出三个 endpoint，
不要添加 `--cluster`：该选项会改用成员发布的宿主机映射 URL，而这些 URL 在容器
网络命名空间内不可达。

## opt-in 集成测试

测试会为每个用例生成唯一前缀并清理数据。`ETCD_REQUIRE_THREE_MEMBERS=1`
确保没有误用单节点实例。

```bash
cd examples/signer-project
export ETCD_ENDPOINTS='http://127.0.0.1:12379,http://127.0.0.1:22379,http://127.0.0.1:32379'
export ETCD_REQUIRE_THREE_MEMBERS=1
go test -tags etcd_integration ./fence \
  -run '^TestEtcd' -count=1 -v
```

覆盖项：

- 同一个 signer Session 上 16 个 goroutine 并发访问同一 key，由本地
  per-key lock 串行，避免 `concurrency.Mutex` 的 Session 级重入；
- 两个独立 client/Session signer replica 并发处理同一 request，正常持有 lease
  时只调用一次 backend；
- 同 epoch 不同 owner 只有一个成功；
- 高 epoch 持久化后拒绝旧 epoch 和同 epoch 旧 owner；
- backend 执行期间主动 revoke lease，旧副本不提交/返回 receipt，记录保持
  `PENDING`，新副本可恢复；
- 保留 Session lease、只删除 Mutex owner key，阶段 C 的 `Mutex.IsOwner()`
  compare 仍会拒绝提交 receipt；
- 已完成 request 在新副本上直接恢复同一 receipt，不再次调用 backend。

## v3.7.0 升级兼容候选

保持 Go client 依赖为 `v3.6.12`，按一次一个 member 的顺序替换 server
image。每次替换后必须先通过健康检查和集成测试，再继续下一个 member：

```bash
export ETCD_UPGRADE_VERSION=v3.7.0
export ETCD_UPGRADE_IMAGE=gcr.io/etcd-development/etcd:${ETCD_UPGRADE_VERSION}
docker pull "${ETCD_UPGRADE_IMAGE}"

upgrade_etcd_member() {
  name="$1"
  host_port="$2"
  docker rm -f "${name}"
  docker run -d --name "${name}" --network signer-etcd-net \
    -p "${host_port}:2379" -v "${name}:/etcd-data" \
    "${ETCD_UPGRADE_IMAGE}" /usr/local/bin/etcd \
    --name "${name}" --data-dir /etcd-data \
    --listen-client-urls http://0.0.0.0:2379 \
    --advertise-client-urls "http://127.0.0.1:${host_port}" \
    --listen-peer-urls http://0.0.0.0:2380 \
    --initial-advertise-peer-urls "http://${name}:2380" \
    --initial-cluster "${ETCD_CLUSTER}" \
    --initial-cluster-state existing \
    --initial-cluster-token "${ETCD_CLUSTER_TOKEN}"
}

upgrade_etcd_member signer-etcd-1 12379
# health + integration suite
upgrade_etcd_member signer-etcd-2 22379
# health + integration suite
upgrade_etcd_member signer-etcd-3 32379
# health + integration suite
```

mixed cluster 的 `STORAGE VERSION` 应保持 `3.6.0`；全部 member 升级后才变为
`3.7.0`。如果某一步失败，停止继续升级并按官方升级指南处理；不要跨 minor
跳级，也不要在没有快照的情况下直接改写数据卷。

## 三节点故障验收

每一步先记录 `endpoint status --cluster -w table`，并用全新的测试前缀运行，避免把
上一步的 `COMPLETED` 当成当前成功。

| 故障 | 操作 | 必须观察到的结果 |
| --- | --- | --- |
| 单 follower 退出 | `docker stop signer-etcd-2`（先确认它不是 leader） | 集群仍有 quorum；集成测试通过；最高 epoch 和 receipt 在节点恢复后仍可读 |
| leader 退出 | 从 status 表确认 leader 对应容器并 `docker stop` | 选举窗口内请求可以返回超时/Unavailable，但不得在阶段 A 未确认时调用 backend，也不得释放阶段 C 未确认的签名；新 leader 产生后重试通过 |
| 丢失 quorum | 再停止第二个节点 | 新的线性一致读写和 Mutex 获取必须失败或超时；调用方收到错误而不是 receipt；恢复任一节点后按同一 RequestID 重试 |
| lease 丢失 | 运行 `TestEtcdLeaseLossCannotCommitReceipt` | backend 已进入后 revoke lease；旧副本返回 `ErrEtcdOwnershipLost`，etcd 记录仍为 `PENDING` 且没有 receipt |
| B/C 崩溃窗 | 在 HSM/MPC 已完成后、C 提交前终止 signer 进程 | 重启后为 `PENDING`；旧 epoch 已被永久 fence；恢复允许再次调用 backend，因此设备审计中可能出现两次签名 |
| C 响应丢失 | C 事务可能已提交时切断 client 网络 | 本次调用不得返回内存签名；恢复网络后同 RequestID 线性读取，若为 `COMPLETED` 则返回已持久化 receipt，否则从 `PENDING` 恢复 |

恢复节点：

```bash
docker start signer-etcd-1 signer-etcd-2 signer-etcd-3
```

清理 Compose localnet：

```bash
docker compose -f compose.etcd.yaml down -v
```

如果使用了上面的手工 `docker run` 流程，则清理：

```bash
docker rm -f signer-etcd-1 signer-etcd-2 signer-etcd-3
docker volume rm signer-etcd-1 signer-etcd-2 signer-etcd-3
docker network rm signer-etcd-net
```

## 生产部署检查

- 使用独立三或五成员 etcd 集群，跨故障域部署，禁止 signer 自行变更成员；
- client 和 peer 全部使用双向 TLS，etcd RBAC 只允许该 signer deployment
  访问专用 prefix；
- 告警覆盖 leader change、proposal failure、fsync latency、DB quota、
  lease keepalive、Mutex 等待和 `ErrEtcdConcurrentMutation`；
- 定期 snapshot/restore 演练。恢复点回退可能让已用过的 epoch 消失，因此灾备切换
  前必须由外部控制面签发严格更高的新 epoch，不能把旧快照直接当作当前授权状态；
- HSM/MPC 仍需独立执行 key policy、请求授权和审计。etcd 只证明 signer
  协调状态，不证明设备侧 exactly-once。
