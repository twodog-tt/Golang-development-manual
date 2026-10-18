# 新域名部署验收（2026-10-18）

2026-10-19（北京时间）品牌更新：发布 `20261018T164817Z`，将页眉、页脚和无障碍标签中的“Octo 知识库”统一为“工程知识库”，保留“松绿书房”。严格构建通过；293 个内容页面均验证无旧品牌，280 个纯跳转页面无旧品牌。服务器 631 个文件哈希、Nginx 配置、公网首页与文章品牌、搜索索引及发布清单均验证通过。截图与发布包位于 `/Users/wangxintong/.codex/visualizations/2026/10/19/knowledgebase-brand-deployment/`。以下为初次域名迁移验收记录。

结果：`https://knowledgebase.wenyiapp.cn/` 已部署至阿里云，公网首页、文章、中英文搜索、HTTP 跳转、HTTPS 与证书续期演练通过。

- 目标：`i-2zee8lkcoh7clkcsv9ws` / `123.57.48.214`。
- DNS：A 记录为目标 IP，无 AAAA；权威 NS 为 `dns15.hichina.com`、`dns16.hichina.com`。公共 DNS 核对通过，无需更改解析。
- 发布：`20261018T154347Z`，631 个站点文件及发布清单；压缩包 SHA-256 为 `1c7ee21b69364cdd04d9812db7fb6e88fc9f875f3f87c1c81cc28d599f0482f6`。上传后校验压缩包和全部文件哈希通过，`current` 已指向该发布。
- 构建：MkDocs 严格构建通过；站点地址、canonical、sitemap 和发布清单均使用新域名；公网 sitemap 不含旧域名。
- 证书：Let’s Encrypt YR2，SAN 为 `knowledgebase.wenyiapp.cn`，到期 2027-01-16 14:46:24 UTC（北京时间 22:46:24）。验证时保留客户端证书校验。
- Nginx：配置检查通过、服务 active；启用 TLS 1.2/1.3。公网 HTTPS 首页返回 200；HTTP 文章地址返回 301，跳转至同路径的 HTTPS，查询参数保留。不存在页面返回 404。
- 首页：公网浏览器展示松绿书房和 242 篇正文；发布清单与当前版本一致。
- 文章：浏览器打开资金账本文章，标题、正文、SVG 图表和代码复制按钮正常；canonical 使用新域名。
- 搜索：浏览器输入英文 `Outbox`，显示 36 条匹配，首项为 Outbox/Inbox 文章；输入中文“双分录”可检索到账本文章并点击进入；无匹配查询显示正确空状态。检查期间未记录到站点脚本错误。
- 自动续期：`certbot.timer` 已启用且 active；部署钩子先执行 `nginx -t` 再重载。执行 `certbot renew --cert-name knowledgebase.wenyiapp.cn --dry-run --no-random-sleep-on-renew` 成功，退出码 0。首次演练遇到 ACME staging API 连接超时，重试通过，没有将网络超时记录为成功。
- 旧域名：续期配置已移至备份，证书与密钥保留；旧域名 DNS 未修改。旧域名的历史拦截情况见 [历史验收](verification-octo-agent-20261018.md)，不代表新域名的当前状态。

## 证据与恢复

本机证据目录：`/Users/wangxintong/.codex/visualizations/2026/10/18/knowledgebase-wenyiapp-deployment/`，包含首页、文章、中英文搜索截图，以及发布包和 `release.json`。

服务器续期演练日志及退出码保存在 `/var/backups/knowledgebase/20261018T154347Z/renewal-dry-run.log`、`renewal-dry-run.exit`。切换前配置、上一发布链接和停用的旧域名续期配置也保存在该备份目录。恢复步骤见 [部署说明](README.md)。
