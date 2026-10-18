# 部署验收（2026-10-18）

结果：服务器部署与 HTTPS 功能验收通过；公网稳定访问与自动续期被阿里云 ICP 备案接入拦截阻塞。

- 目标：`i-2zee8lkcoh7clkcsv9ws` / `123.57.48.214` / `knowledgebase.octo-agent.com`。
- 发布：`20261018T150126Z`，631 个站点文件及发布清单；包 SHA-256 为 `3681ef4c243f1771ded2f5262c06f8612ef640e9b900c1914a08b637b733e13a`。上传与解压后逐文件校验通过。
- DNS：Cloudflare A 指向目标 IP，未配置 AAAA，保持仅 DNS；另通过公共 DoH 核对。
- 证书：Let’s Encrypt YR1，SAN 为目标域名；到期 2027-01-16 13:55:44 UTC。没有使用自签证书，也没有关闭客户端证书校验。
- 服务器：Nginx 配置校验通过、服务 active；TLS 1.2/1.3、HTTPS 站点、HTTP 301 配置、ACME 独立目录、续期 timer 与重载钩子已配置。续期 timer enabled。
- 服务器 HTTPS 检查：使用域名 SNI 和正常证书校验访问本机 Nginx，首页、账本文章、搜索索引和发布清单通过；不存在页面返回 404。
- 公网浏览器：首页展示松绿书房和 242 篇正文；中文“双分录”可检索到账本文章并进入；英文 Outbox 首项为 Outbox/Inbox 文章，点击进入成功；无结果查询正确显示空状态；账本页 SVG 图表和代码复制按钮存在；没有站点脚本错误。浏览器扩展自身的告警不归为站点错误。
- 搜索修正：补充构建期 `jieba==0.42.1`；兼容粘贴与中文输入完成事件。沿用 Material 原有模糊搜索规则，没有另建搜索服务。
- 严格构建与元数据：通过；242 条已发布元数据和 242 个正文文件。

## 外部阻碍

公网 HTTP 返回 `403`，响应头 `Server: Beaver`，页面标题 `Non-compliance ICP Filing`，内嵌阿里云备案拦截页。服务器本机 HTTP 返回预期的 301，说明阻断发生在服务器入口之外。

首次正式证书申请成功；随后执行带 `--no-random-sleep-on-renew` 的续期演练，CA 对 `/.well-known/acme-challenge/...` 返回 `unauthorized / Invalid response / 403`，演练失败。没有将定时器已启用等同于续期通过。

一次较早的演练遇到 Certbot 默认随机延迟，远程命令到时关闭后出现 Broken pipe；已使用关闭随机延迟的命令重新演练，确认真正的外部阻碍是挑战地址收到 403。

需要域名所有者在阿里云完成备案/接入后，再验证公网 HTTP 跳转和证书续期。没有修改其他域名、没有绕过备案拦截，也没有将正式证书替换为测试证书。

## 证据

浏览器实拍位于 `/Users/wangxintong/.codex/visualizations/2026/10/18/knowledgebase-deployment/`：

- `home-https.png`
- `article-https.png`
- `search-chinese-https.png`
- `search-english-https.png`

对应发布包与 `release.json` 也保存在该目录。服务器保留原始配置与前一发布版本，可按部署说明回滚。
