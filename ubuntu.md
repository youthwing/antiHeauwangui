# Ubuntu 服务器部署教程（Docker）

> 用 docker compose 在服务器上**从源码构建**并跑 wangui。
> 反向代理（HTTPS / 域名）交给 1Panel；也可以直接 `http://IP:5555` 访问。

---

## 0. 你需要准备的

| 项 | 必需 | 说明 |
|---|---|---|
| Ubuntu 22.04+ 服务器 | ✅ | 1 vCPU / 1GB / 5GB 磁盘起；构建阶段会临时多用 1.5GB |
| 能 SSH 进去（root） | ✅ | 整个流程都用 root |
| Docker + Docker Compose | ✅ | 见 §1 |
| Git 或 scp 把源码送上去 | ✅ | 二选一，见 §2 |
| 1Panel（或其它面板） | 可选 | 你已经在跑；只用来反代 |

> 没有任何预编译二进制 / `.exe` 需要上传。Docker 镜像在服务器上现场构建：node:20 → 编译 SPA，golang:1.25 → 编译 Go 二进制，alpine:3.20 → 运行。源码全程可审计。

---

## 1. 装 Docker

```bash
curl -fsSL https://get.docker.com | sh
systemctl enable --now docker

# 验证
docker --version
docker compose version
```

---

## 2. 把源码送到服务器

二选一。

### 2.1 git clone（推荐）

```bash
mkdir -p /root/wangui
cd /root/wangui
git clone <你的私库 URL> .
```

### 2.2 scp 整个项目

本地（项目根目录的上一级）：

```bash
scp -r fuckwangui/ root@your.server:/root/wangui/
```

scp 上去后服务器上长这样：

```
/root/wangui/
├── Dockerfile
├── docker-compose.yml
├── .env.example
├── .dockerignore
├── go.mod / go.sum
├── cmd/ internal/ web/
└── ... (源码)
```

> `.dockerignore` 已经屏蔽 `data/`、`*.db`、`.env`、本地 `wangui.exe`/`wangui-linux`、字体冗余副本 —— 这些就算 scp 上去了，也不会被 COPY 进镜像。

---

## 3. 配置环境变量

```bash
cd /root/wangui
cp .env.example .env
```

然后**生成主密钥**：

```bash
openssl rand -hex 32
```

会打印一行 64 个 hex 字符（形如 `a3f1c8...e9`）。

⚠️ **立刻把这串字符抄到密码管理器（1Password / Bitwarden）里再继续**。这个 KEY 服务器之外没有任何地方有副本；丢了之后所有用户 token + SMTP 密码就**永久解不开**了。

```bash
nano .env
```

把两行占位符都换成真值：

```ini
WANGUI_ADMIN_PASS=换成你自己的 16+ 位强密码
WANGUI_MASTER_KEY=刚刚 openssl 输出的 64 字符 hex
```

`Ctrl+O` 回车保存，`Ctrl+X` 退出。

权限收紧（只 root 可读）：

```bash
chmod 600 .env
```

---

## 4. 构建并启动

```bash
cd /root/wangui
docker compose up -d --build
```

第一次会拉 `node:20-alpine`、`golang:1.25-alpine`、`alpine:3.20` 三个基础镜像，然后跑构建。整体约 3–5 分钟（受网速 + CPU 影响）。

构建产物：

| 镜像 | 用途 | 是否驻留 |
|---|---|---|
| `node:20-alpine` (cache) | 编译 Vue SPA | 留下，下次构建复用 |
| `golang:1.25-alpine` (cache) | 编译 Go 二进制 | 留下 |
| `wangui:latest` | 运行时镜像 | **就靠这个跑**，约 30MB |

跑起来之后看状态：

```bash
docker compose ps              # 应该看到 wangui 容器 State=running
docker compose logs -f wangui  # 跟踪日志
```

期望看到这些日志：

```
INFO msg="store ready" data_dir=/data
INFO msg="admin panel enabled"
INFO msg="scheduler armed" next_window_start=...
INFO msg="http server listening" addr=0.0.0.0:5555
INFO msg="backup armed" next=...
```

`Ctrl+C` 退出 logs 跟踪（容器仍在跑）。

---

## 5. 验证

```bash
# 服务器本地
curl -s http://127.0.0.1:5555/ | head -3
# 应该返回 <!doctype html>...
```

浏览器：

- **直接 IP+端口**：`http://你的服务器IP:5555/`
- **走 1Panel 反代后**：`https://你绑的域名/`

两种都应该看到登录页。如果直接 IP 打不开 → §10 故障排查。

---

## 6. 1Panel 反代（可选但推荐）

如果你想用域名 + HTTPS：

1. 1Panel 「网站」 → 新增网站 → 反向代理
2. 主域名填你的域名（DNS A 记录先解析到这台服务器）
3. 代理地址：`http://127.0.0.1:5555`
4. 启用 SSL → 1Panel 自动签 Let's Encrypt
5. 保存

为了让外网**只**能走 1Panel 域名（不暴露 5555 到公网），改 `docker-compose.yml` 的端口映射：

```yaml
ports:
  - "127.0.0.1:5555:5555"    # 只对 loopback 开放
```

然后重启容器：

```bash
docker compose up -d
```

可选：在 1Panel 反代里加几个安全 header 防搜索引擎收录：

```
X-Robots-Tag: noindex, nofollow
X-Frame-Options: DENY
Referrer-Policy: no-referrer
```

---

## 7. 首次使用流程

### 7.1 管理员登录

打开 `http://你的IP:5555/airvel/login`（或反代域名 `/airvel/login`）

用 `.env` 里 `WANGUI_ADMIN_PASS` 登录。

> 路径已经从 `/admin` 改成 `/airvel`（前后端均改），别敲老路径。

### 7.2 配置邮件通知（可选）

进入「系统设置」 → 「邮件通知 (SMTP)」section。

**Gmail 推荐配置**：

| 字段 | 值 |
|---|---|
| Host | `smtp.gmail.com` |
| Port | `587` (STARTTLS) |
| 发件人邮箱 | 你的 Gmail |
| 应用专用密码 | 见下方步骤 |
| From 显示名 | `勿外传 <you@gmail.com>` |
| 管理员收件邮箱 | 你自己邮箱（作为日志收件箱） |
| 总开关 | **打开** |

**生成 Gmail App Password**：

1. Google 账号 → 安全性 → 开启「两步验证」（必需）
2. 搜「应用专用密码 / App Passwords」→ 生成
3. 16 位密码（形如 `glht egbx rokr ktiu`，含不含空格都行，服务器会自动 strip）
4. 粘贴到「应用专用密码」输入框 → 保存 → 「发测试邮件」验证

**收件规则**：

| 用户开通知 | 管理员邮箱配了 | 行为 |
|---|---|---|
| ✓ | ✓ | 发给用户，bcc 管理员 |
| ✓ | ✗ | 只发给用户 |
| ✗ | ✓ | **以"管理员日志"发给管理员** |
| ✗ | ✗ | 不发 |

只要管理员邮箱配了，你就能在邮箱里看到所有用户每天的签到结果。

### 7.3 创建第一批卡密

「卡密」页 → 「生成卡密」 → 数量 5 张 → 备注 "首批"。生成后会弹出模态框，可以一键复制单张或全部。

### 7.4 添加宿舍楼

「宿舍楼」页 → 「添加宿舍楼」 → 起名 + 地图选点 → 「签到载荷」选「仅坐标」（推荐）→ 保存。

### 7.5 给朋友的指引

```
站点: http://你的IP:5555  或  https://你的域名/
卡密: ABC-DEF-XYZ9  (这张是给你专用的)

操作:
1. 在电脑微信打开晚归签到入口
2. 用 Reqable / Charles 抓包，拿到 Authorization 头里 Bearer 后的 Token
3. 打开上面的站点 → 「激活」tab
4. 填卡密 + Token + 设置 4-6 位 PIN
5. 进配置页选自己的宿舍楼
6. (可选) 在配置页填邮箱、开启「邮件通知」
7. 之后每天 22:00 自动签到
8. Token 大约 7 天过期，到时再来更新
```

---

## 8. 备份

### 8.1 自动每日快照

容器内部每天 23:00 自动跑 `VACUUM INTO`，写到 `/data/backups/`，保留 7 份。`/data` 已经挂载到宿主机的 `/root/wangui/data/`，所以备份文件直接在宿主机可见。

```bash
ls /root/wangui/data/backups/
```

### 8.2 把备份拉到本地（推荐）

本地电脑：

```bash
# crontab -e
30 23 * * * rsync -az -e ssh root@your.server:/root/wangui/data/backups/ ~/wangui-backups/
```

### 8.3 手动触发备份

```bash
docker compose exec wangui /usr/local/bin/wangui backup-now -data /data
```

### 8.4 主密钥备份

`.env` 里的 `WANGUI_MASTER_KEY` 必须单独备份到密码管理器。光备份 sqlite 没用 —— 没 key 解不开 token 和 SMTP 密码。

---

## 9. 升级（部署新版本）

源码改完后：

```bash
cd /root/wangui
git pull                       # 或者重新 scp 把改动同步上来
docker compose up -d --build   # 重新构建并替换容器
docker compose logs -f         # 看新容器是否正常启动
```

DB schema 是 idempotent ALTER TABLE 模式，新版本自动迁移老 DB，**数据零丢失**。

清理旧镜像（可选，释放磁盘）：

```bash
docker image prune -f
```

---

## 10. 故障排查

| 现象 | 怎么查 |
|---|---|
| `docker compose up` 直接报 "WANGUI_ADMIN_PASS must be set in .env" | `.env` 没填或没拷贝；检查 `cat .env` |
| 构建 `npm ci` 失败 | 网络问题；可以在 `Dockerfile` stage 1 加 `npm config set registry https://registry.npmmirror.com`（国内镜像）后重新 `docker compose up -d --build` |
| 构建 `go mod download` 慢 | 同上，加 `ENV GOPROXY=https://goproxy.cn,direct` 到 stage 2 |
| 容器启动后立刻退出 | `docker compose logs wangui` 看具体错。常见：master key 格式错（不是 64 hex 字符）/ 数据目录权限不对 |
| 浏览器 IP:5555 加载白屏 | F12 → Network 看 `/` 返回 HTML 没？`/assets/*.js` 有没有 404？再贴报错给我 |
| 浏览器 IP:5555 完全打不开（转圈） | 1) 服务器云厂商安全组没放行 5555；2) ufw 没放行；3) docker-compose.yml 把 ports 改成了 `127.0.0.1:5555:5555` 但你试图从外网访问 |
| 1Panel 反代后 502 | 1Panel 反代目标地址是 `http://127.0.0.1:5555`？容器是不是真在跑（`docker compose ps`）？ |
| 数据目录权限报错 | `chown -R 100:101 /root/wangui/data`（容器内 wangui 用户的 uid:gid，alpine 默认 100:101） |
| 数据没了 | 一定没改 `docker-compose.yml` 的 `volumes: - ./data:/data`？**永远别**跑 `docker compose down -v`（`-v` 删卷） |
| 时区不对（签到没在 22:00） | `docker compose exec wangui date` 看；应该是 CST。compose 已设 `TZ=Asia/Shanghai` |
| 朋友说"Token 校验失败" | 他的 Token 已过期或抄错了 |
| 朋友 22:00 没签到 | admin 「日志」 → 找他的失败记录看 `message` 字段（多半 token 过期 / 未配宿舍楼） |
| 测试邮件发不出 | admin 「系统设置」看 toast 报错；常见：Gmail 未开两步验证 / 用了登录密码而非 App Password / 587 误填 465 |
| 自动签到没收到邮件 | 用户在 `/settings` 填了邮箱且打开「邮件通知」开关？SMTP 总开关是否打开？`sign-now` **不**发邮件（设计如此），等当晚 22:00 |
| 想从备份恢复 | `docker compose stop wangui`<br>`cp /root/wangui/data/backups/wangui-XXX.db /root/wangui/data/wangui.db`<br>`docker compose start wangui` |

---

## 11. 关停 / 卸载

```bash
cd /root/wangui

# 停容器（保留数据 + 镜像）
docker compose stop

# 停 + 删容器（保留数据卷 + 镜像）
docker compose down

# 停 + 删容器 + 删镜像（数据仍在 /root/wangui/data/）
docker compose down --rmi all

# 彻底清掉（数据也删，谨慎！备份好 master.key 之后再做）
docker compose down --rmi all
rm -rf /root/wangui
```

> ⚠️ **永远不要** `docker compose down -v` —— `-v` 会同时删数据卷！

别忘了去 1Panel 把反向代理站点也清掉。

---

## 12. 最佳实践

1. **必须改默认密码**：`.env` 里的 `WANGUI_ADMIN_PASS` 用 16+ 位随机字符。所有在仓库 / 文档 / 聊天记录中出现过的占位密码视同已泄露，**不要原样用**
2. **主密钥单独备份**：`WANGUI_MASTER_KEY` 抄一份到密码管理器；丢了不可逆
3. **`.env` 权限 600**：`chmod 600 .env` —— 别让其它用户读到
4. **管理路径已经混淆**：admin 路径是 `/airvel` 而不是 `/admin`。**不要在公开渠道提到这个路径**
5. **走 1Panel 反代时**：`docker-compose.yml` port 映射改成 `127.0.0.1:5555:5555`，外面只暴露 1Panel 的 80/443；安全组只放行 22 + 80 + 443
6. **直接 IP+端口访问时**：必须接受明文 HTTP 风险 —— 学号、PIN、学校 Token 都走明文。最好只自己用、限信任的朋友、别让 5555 端口被搜索引擎扫到
7. **用 SSH 密钥登录**：禁用密码登录（`sshd_config` 里 `PasswordAuthentication no`）；可选 fail2ban
8. **定期 token 续期提醒**：朋友们的学校 token 一般 7 天过期，可以加个 cron 每周提醒他们
9. **监控**：服务器宕机自己得知道。加个 uptime 监控；或者只看 admin 邮箱（管理员 BCC 收所有签到结果，连续几天没邮件就说明服务挂了）
