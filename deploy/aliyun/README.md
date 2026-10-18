# 阿里云知识库部署

- 正式域名：`https://knowledgebase.wenyiapp.cn/`
- ECS：`i-2zee8lkcoh7clkcsv9ws`，北京，公网 `123.57.48.214`。
- DNS：阿里云 DNS 的 A 记录指向该 IP，无 AAAA；公共 DNS 查询已核对。
- 运行方式：Nginx 托管 MkDocs 静态文件；无需注册登录、应用数据库或搜索后端。
- 配置：[knowledgebase.nginx.conf](knowledgebase.nginx.conf)。站点地址、canonical 和 sitemap 均使用新域名。
- 发布目录：`/var/www/knowledgebase/releases/<release>/`，`/var/www/knowledgebase/current` 原子指向当前版本。
- 当前发布：`20261018T164817Z`，631 个站点文件及发布清单。压缩包 SHA-256：`a1dc2b600dad224ad2678a0548e350f75ac6241a93f67f946753c0df1e565ea4`。北京时间 2026-10-19 更新页眉、页脚及无障碍品牌名称为“工程知识库”，保留“松绿书房”。本次上一发布记录为 `/var/backups/knowledgebase/20261018T164817Z/previous-release`。
- 本次验收：[verification.md](verification.md)。旧域名的备案拦截记录保留在 [历史验收](verification-octo-agent-20261018.md)。

## 更新站点

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
