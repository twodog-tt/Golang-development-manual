# Memory 规则审批、失效与删除传播

对应 S-AI-10。Go 1.24.2+、Linux/macOS 文件锁，标准库，无外部模型/数据库。

```bash
go test -race ./examples/senior/agentmemory/...
go run ./examples/senior/knowledge-demo
```

`Create → candidate → Approve → approved → Transition(active)` 是启用路径。Create/Edit 要 write，Approve 要 review，Transition 要 activate，Delete 要 delete，Get/Resolve 要 read。
所有角色受完整 namespace 限制；`Authority.Put/Context/Revoke` 是可信宿主入口，不能由模型或公网直接传任意 grant。
身份上下文绑定 issuer 随机世代与 subject revision，降权/撤权使旧上下文失效；授权服务重建会使未启用批准失效，需 Edit 回候选并重新审批。

作者不能自审；批准绑定 scope、正文、来源、policy、期限和版本摘要。启用时再检查 reviewer 活跃权限。编辑增加版本并清除批准；retired/rejected 为终态，恢复旧内容需新ID、新审批。
到期/退役/policy不符/错误版本的规则不会被 Resolve 注入上下文。审批者撤权阻止尚未启用的批准；已经启用的规则由当前policy、期限、退役/删除治理，不自动把每次reviewer人事变动当作全量退役。

## 持久状态与故障

同一个 state.json 原子保存主记录和待同步任务；flock 串行、临时文件 fsync/rename/目录 fsync。索引与缓存是同机文件内的派生投影夹具。
`SyncOne(true)` 在写投影后故意丢失ACK；任务仍pending，重启 `Sync()` 可重放。投影依主记录当前版本应用，旧job不能回写旧内容或删掉新active投影。

Delete 先原子落盘 tombstone、提升版本、清除主记录正文/来源并排队清理。历史只存 `Ref{ID,Version}`；Get/Resolve 始终查权威主记录，旧索引、缓存或历史引用不得绕过 tombstone。
同ID不能重建。派生文本需异步清理，外部trace/原始prompt/备份的物理删除与保留审计不在本地演示内。

| 测试 | 故障/断言 |
|---|---|
| `TestApprovalRevisionAndIsolation` | 自审、未批准启用、旧摘要/版本、编辑撤批、终态复活、跨scope拒绝 |
| `TestRevocationExpiryPolicyAndAuthorityRestart` | reviewer/operator撤权、规则过期、policy变化、issuer重建均阻止旧批准 |
| `TestDeleteFencesStaleIndexCacheHistoryAndLostACK` | 尚未清理的旧派生文本确实存在，但读不到；丢ACK、重启重放、重复清理幂等 |
| `TestStaleProjectionJobCannotEraseNewActiveRevision` | 重放旧job不删除新active版本 |
| `TestConcurrentEditOnlyOneWins` | 同版本并发编辑只有一个成功 |
| `TestProcessRestartAfterProjectionWriteBeforeACK` | 独立子进程写投影后退出23；父进程从pending恢复 |
| `TestReadRoleCannotApproveOrActivate` | 同policy读者不能审批或启用有效批准的版本 |
| `TestRoleDenialAndPolicyReadDoesNotReturnText` | read角色不能写/删，policy不符的错误返回不泄露正文 |
| `TestCorruptStateFailsClosed` | 损坏文件不按空库继续运行 |

此例是单机、可信目录下的故障教学，不是生产IAM、分布式Outbox/多机共享锁或物理介质擦除认证。
每个操作在授权检查处确定边界，不能撤销已经完成的读取；生产需要事务化策略版本、审计、备份及索引服务独立验收。
