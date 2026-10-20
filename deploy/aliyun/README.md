# 阿里云知识库部署

- 正式域名：`https://knowledgebase.wenyiapp.cn/`
- ECS：`i-2zee8lkcoh7clkcsv9ws`，北京，公网 `123.57.48.214`。
- DNS：阿里云 DNS 的 A 记录指向该 IP，无 AAAA；公共 DNS 查询已核对。
- 运行方式：Nginx 托管 MkDocs 静态文件；无需注册登录、应用数据库或搜索后端。
- 配置：[knowledgebase.nginx.conf](knowledgebase.nginx.conf)。站点地址、canonical 和 sitemap 均使用新域名。
- 发布目录：`/var/www/knowledgebase/releases/<release>/`，`/var/www/knowledgebase/current` 原子指向当前版本。
- 当前发布以站点 `/release-manifest.json` 为准，包含完整提交 SHA、Actions 运行及尝试编号、文件哈希。上次手工发布为 `20261018T164817Z`，其部署记录保留用于恢复。
- 本次验收：[verification.md](verification.md)。旧域名的备案拦截记录保留在 [历史验收](verification-octo-agent-20261018.md)。

## GitHub Actions 自动更新

每次推送到 `master`，或从 Actions 页面在 `master` 手动运行 `Deploy Docs`，都会严格构建并更新阿里云站点，同时保留原有 gh-pages 发布。其他分支不能通过手动触发发布生产站点。

流程：部署脚本故障测试 → 生成自测数据和重定向 → 元数据校验 → 严格构建 → 实际 search worker 中英文/代码标识查询验证 → 生成清单和压缩包 → SSH 上传并逐文件校验 → 原子切换 `current` → Chromium 公网验收 → gh-pages。生产任务串行执行，不取消正在进行的发布。

公网验收检查：当前发布编号、首页标题与文章数量、英文 `Outbox` 和中文“双分录”搜索及结果跳转、账本文章正文与 canonical、目录定位、代码复制按钮、无结果提示、搜索退出焦点和浏览器脚本错误。浏览器安装方式遵循 [Playwright CI 文档](https://playwright.dev/python/docs/ci)。每次验收的截图、trace 和 JSON 结果保存为 Actions artifact，保留 14 天。

验收失败时 Actions 保持红色，并请求回滚到此次切换前的版本。服务端只允许回滚仍然处于激活状态的这次发布，避免覆盖后续版本。若回滚 SSH 连接失败，任务会明确失败，需通过 Workbench 恢复；不要把失败任务当作已恢复。上传或文件校验失败时不切换线上版本。

### 专用账号与密钥

- `kbdeploy` 没有 sudo 权限；它只拥有 `/var/www/knowledgebase` 和 `releases` 目录的写权限，不拥有已有的历史文件、Nginx 配置或 TLS 私钥。
- SSH 公钥启用 `restrict` 和固定命令，仅接受 `deploy <release> <sha256>`、`rollback <release>`；不开放 shell、SCP、PTY、代理或端口转发。
- 固定入口为 root 所有的 `/usr/local/lib/knowledgebase/receive.py`。更新入口程序需管理员通过 Workbench 执行 `setup.sh`，Actions 不具备修改此程序的权限。
- 仓库 Secrets：`ALIYUN_DEPLOY_SSH_KEY`（专用私钥）、`ALIYUN_DEPLOY_KNOWN_HOSTS`（服务器 Ed25519 公钥，已通过管理连接核对）。不上传本机阿里云 AccessKey，保持严格 SSH 主机校验。
- 初始化或轮换时生成专用 Ed25519 密钥，通过 Workbench 上传公钥、`setup.sh` 和 `receive.py`，以 root 运行 `sh setup.sh <公钥路径> <入口程序路径>`，再通过 `gh secret set` 的标准输入更新两个 Secrets。私钥不进入仓库或命令行参数。

### 发布与恢复记录

发布目录名为 `<40 位提交 SHA>-<run_id>-<run_attempt>`。上版路径保存在 `/var/www/knowledgebase/.deploy/<release>.json`，旧发布不自动删除。恢复可使用原部署密钥执行 `rollback <release>`；也可由管理员通过 Workbench 执行：

```bash
sudo -u kbdeploy env SSH_ORIGINAL_COMMAND='rollback <release>' \
  python3 /usr/local/lib/knowledgebase/receive.py
```

服务端部署脚本的故障验证：

```bash
python3 -m unittest discover -s deploy/aliyun/tests -v
```

## 手工更新站点

使用现有本机 Workbench 配置，不将 AccessKey 或 SSH 私钥写入项目。

```bash
.venv/bin/python -m pip install -r requirements-docs.txt
.venv/bin/mkdocs build --strict
tar -czf /tmp/knowledgebase-<release>.tar.gz -C site .
~/.local/bin/workbench upload /tmp/knowledgebase-<release>.tar.gz /tmp/ \
  -i i-2zee8lkcoh7clkcsv9ws -r cn-beijing
```

只打包 `site/` 的生成内容。上传后核对 SHA-256，解压到新的发布目录，设置目录 755、静态文件 644，记录上版路径，通过临时软链接和 `mv -Tf` 原子切换 `current`。修改 Nginx 前保存配置，先执行 `nginx -t`，再重载。上线后核对首页、文章、搜索、canonical、sitemap 和发布清单。

## HTTPS 与续期

正式证书位于 `/etc/letsencrypt/live/knowledgebase.wenyiapp.cn/`，私钥只保存在服务器。HTTP 的 ACME 验证目录为 `/var/www/certbot/.well-known/acme-challenge/`，其他请求 301 跳转至新域名的 HTTPS，并保留路径和查询参数。

当前证书由 Let’s Encrypt YR2 签发，到期时间为 2027-01-16 14:46:24 UTC（北京时间 22:46:24）。本次新域名自动续期演练已通过；详细结果见验收记录。

`certbot.timer` 管理自动续期；部署钩子 `/etc/letsencrypt/renewal-hooks/deploy/knowledgebase-nginx` 在配置检查通过后重载 Nginx。续期演练应禁用 Certbot 默认随机等待，避免远程连接超时：

```bash
certbot renew --cert-name knowledgebase.wenyiapp.cn --dry-run --no-random-sleep-on-renew
systemctl list-timers certbot.timer
nginx -t
```

换域名时停用旧域名的续期配置，备份为 `/var/backups/knowledgebase/20261018T154347Z/retired-octo-agent-renewal.conf`。旧证书仍保留，未删除密钥。

## 回滚与历史

切换前的 Nginx 完整备份位于 `/var/backups/knowledgebase/20261018T154347Z/nginx-before-domain-change/`；上一发布路径保存在同目录的 `previous-release`，对应 `20261018T150126Z`。

普通内容回滚可原子切换 `current` 至保留版本。若要完整撤销本次域名迁移，还需恢复旧 Nginx 配置与旧域名续期配置，再检查并重载；旧域名已有备案拦截，不能将其恢复视为公网可用性修复。新域名证书与发布文件保留，便于再次恢复。
