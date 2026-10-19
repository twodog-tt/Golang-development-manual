# A2A：授权与持久恢复实战

需要 **Go 1.24.2+、Unix 文件锁**。本例只使用 Go 标准库、真实 loopback HTTP 和独立的本地/远端 JSON 文件日志，不需要模型、Docker、外部 Agent 或真实发布账号。

```bash
go test -race ./examples/senior/a2a/...
task_dir="$(mktemp -d)"
go run ./examples/senior/a2a/cmd/demo -dir "$task_dir" -step demo
```

`demo` 在夹具中分别模拟调用者 Alice 和人工 reviewer，正常输出 `TASK_STATE_COMPLETED`、`effects=1`。重复运行不会增加效果数。这里的 effect 是远端日志内的计数，不是真实 CMS 发布，也不证明外部系统 exactly-once。

## 协议与证据范围

支持 A2A **HTTP+JSON 1.0 的限定子集**：显式 `A2A-Version: 1.0`、Message/Task 两种响应、任务查询/列表/取消、认证和 owner 检查。请求的 `Part.data` 固定为本例的 `{action,payload}` 应用契约。未知字段明确拒绝；没有实现所有可选字段、分页、gRPC、JSON-RPC、多 SDK 兼容或完整规范错误对象。

- `echo` 直接返回 Message，无 task ID，不创建虚假任务。
- `publish` 创建 `auth-required`；`clarify` 创建 `input-required`。这两个状态可中断后继续，不是终态。
- **不支持流式事件回放**。`RecoverStream(..., "")` 查询快照；非空游标明确报错。Agent Card 声明 `streaming=false`。快照恢复不声称补齐了所有历史事件/Artifact。
- 标准 Push 自动投递尚未实现，Agent Card 明确声明 `pushNotifications=false`，标准 Push 配置入口返回 unsupported。本地扩展 `/fixture/tasks/{id}/pushNotificationConfigs` 与 `statusUpdate` 接收用于回调安全演练，配置最多一个，不能静默覆盖；`metadata` 中的 tenant、delivery_id、revision 是本地约定，不是 A2A 强制去重保证。通知只持久化提示，最终状态经认证 GET 核对。
- 两套文件日志通过 flock、临时文件 fsync、rename、目录 fsync 保持本地恢复边界。仅适用于单机可信目录，不提供分布式 HA、跨文件原子事务或密钥管理服务；保留 UNKNOWN 正是为了处理两边无法原子提交的窗口。

`-dedup=true` 是固定的**应用契约**：远端按 `(tenant, subject, messageId)` 持久去重并绑定完整消息摘要。本地重启后重发完全相同的消息，不能改变 task/context/payload。A2A 标准本身不保证 Send Message 幂等。此夹具不淘汰去重记录；生产必须另行约定保留期，超过该期限转人工核对。已使用日志拒绝切换 dedup 配置。本地 intent 同时绑定受信配置中的稳定 PeerID 与重试契约，恢复不能更换远端身份或临时扩大重试能力；生产 PeerID 还需由 TLS/服务注册等可信机制映射到端点。本例重启时 loopback 端口可改变。

## 分步运行与人工核对

构建二进制以保留准确退出码（`go run` 会包装子程序退出码）：

```bash
task_dir="$(mktemp -d)"
go build -o "$task_dir/a2a-demo" ./examples/senior/a2a/cmd/demo
"$task_dir/a2a-demo" -dir "$task_dir/state" -step submit
"$task_dir/a2a-demo" -dir "$task_dir/state" -step status
```

核对输出的 `task_id`、`review_input`、`review_digest` 和 `review_revision` 后，将显示的摘要和版本传给 `-step approve -digest ... -revision ...`。省略它们会被拒绝。然后运行：

```bash
"$task_dir/a2a-demo" -dir "$task_dir/state" -step continue
"$task_dir/a2a-demo" -dir "$task_dir/state" -step status
```

审批由独立 reviewer 身份经本地受信通道完成，未暴露成公网审批接口。未取得操作审批的合法调用仍返回 auth-required，并保存这次消息的待授权回执；之后批准也不会使重放的旧消息自动执行，必须明确发起新消息。

审批绑定目标 owner、完整 action/payload digest、task revision 和有效期；执行前检查 reviewer 仍具有审批角色。自审、跨租户审批、过期、撤销、参数变化和旧版本均不放行。后续消息不自动继承审批。

可用 `-step revoke` 撤销，或 `-step cancel` 取消任务；已完成/失败/拒绝的任务返回 TaskNotCancelable，不将其改写为 canceled。`-payload` 修改待执行内容会使旧审批失效，再次进入 `auth-required`。权限、身份和凭据来自服务端夹具，模型输入不允许设置 tenant/user。

## 回包丢失与崩溃

在**新的 state 目录**执行：

```bash
# 远端创建已经持久化，故意丢弃回包；预期退出 2。
"$task_dir/a2a-demo" -dir "$task_dir/lost" -step submit -lose-reply
# 重复 submit 仍退出 2，UNKNOWN 不重新规划、不更换 messageId。
"$task_dir/a2a-demo" -dir "$task_dir/lost" -step submit
# 明确启动恢复；依据事先约定的去重能力找回同一个任务。
"$task_dir/a2a-demo" -dir "$task_dir/lost" -step reconcile
```

没有去重/查询契约时，`-dedup=false -lose-reply` 后，`-step reconcile -dedup=false` 也保留 UNKNOWN（退出 2），不能编造 task ID 或声称已自动恢复。人工应向远端核对消息/业务 intent 的回执；本例没有“直接清 UNKNOWN 重试”的后门。

`-crash before-send` 和 `-crash after-send` 会真实退出进程，退出码 77。after-send 可用于 submit 创建后或已批准 continue 执行后；重启并 reconcile 后，测试断言远端 task 数仍为 1，已完成 effect 数仍为 1。已知 task 的非终态快照也不能证明一个 UNKNOWN 后续消息没有被接受：没有去重契约就继续隔离，有契约才允许重发原消息；终态快照可完成核对。

## 回调安全与故障矩阵

`DeliverPush` 只允许 HTTPS/443；注册时和连接时验证 DNS 的全部地址，拨号固定到已验证 IP，禁止代理和重定向。拒绝 loopback、私网、link-local 和保留地址；IPv4-mapped IPv6 先还原后按真实 IPv4 地址校验。每个回调使用专用 `authentication.scheme=Bearer` 凭据，查询配置不返回凭据。`Receiver` 绑定预期 task、context、tenant 与专用 secret，Inbox 持久化后才 ACK；重复 delivery ID 的不同内容被拒绝。

| 测试组 | 故障与断言 |
|---|---|
| Message / Task | 直接 Message、auth-required、input-required 正确区分 |
| 首次回包丢失 | 持久 UNKNOWN、重复 submit 拒绝；有契约找回同任务，无契约保持隔离 |
| 每个任务操作 | 跨租户、同租户其他用户、scope 降权、凭据撤销，覆盖 get/list/cancel/subscribe/Push 配置/任务后续消息 |
| 人工审批 | 自审、其他租户、错误 digest/revision、过期、撤销、reviewer 降权、参数变化和正常恢复 |
| 并发/重试 | 同 intent 并发提交只创建一个任务，messageId 相同而 payload 改变被拒绝 |
| 恢复 | 未协商游标拒绝、快照查询、终态保护；3 次真实子进程退出 |
| 回调来源 | 错 secret/task/context/tenant、重复/冲突内容、乱序、ACK 丢失及 Inbox 重启 |
| 出站回调 | 私网/特殊地址、混合 DNS、DNS 变更、无环境代理、真实 TLS、禁止重定向 |
| 协议边界 | 明确版本、响应不包含内部 owner/审批/输入信息、错误 JSON/oneof 拒绝 |

TLS 正例在测试中将**已验证的公网地址拨号**定向到受控的本地 TLS 服务，不访问公网。生产 `DeliverPush` 没有这个定向入口。测试不等于任何第三方 Agent、OAuth issuer 或公网 Webhook 的验收。

统一入口：`bash scripts/verify_reliability_examples.sh --offline`，包含 race 测试和真实 CLI 正常、回包丢失、审批及执行崩溃流程。
