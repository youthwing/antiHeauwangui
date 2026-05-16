# wangui

> 学校晚归签到自动化系统 —— 内部自用、邀请制、≤5 人 + 偶尔几个一次性朋友。
> **不对外开放，不对外宣传，不接受陌生人申请。**

---

## ⚠️ 风险与边界

这个工具替你和你的朋友**在每天 22:00–22:30 期间自动调用学校签到接口**。它的存在前提是：

1. **你确实在校** —— 不在校时让脚本签到 = 谎报位置 = 违纪。后果由用户承担
2. **token 是用户自己的** —— 工具不会盗号，也不读其他人的信息
3. **仅供有限熟人** —— 不公开、不收费、不传播
4. **学校发现就关停** —— 学校发文要求停止此类工具时，立即停服

这是一个**为方便而存在的灰色工具**，不是一个产品。仓库私有就是这个原因。

---

## 0. 这是什么

```
┌────────────────────────────────────────────────────────┐
│                                                         │
│   学校晚归签到系统 (xhbcs)                              │
│      ▲                       ▲                          │
│      │ JWT Bearer + 签到 API │ OAuth2 微信扫码         │
│      │                       │                          │
│  ┌───┴──────────────────────┴───┐                       │
│  │  wangui server (Go + Vue)    │                       │
│  │  Docker on VPS               │  ←  朋友 浏览器       │
│  └────────┬─────────────────────┘     扫码 + 复制链接    │
│           │                                              │
│           │ 每天 22:00–22:10 替每个用户                   │
│           │ 调用学校签到 API                              │
│           ▼                                              │
│   邮件通知用户 + 管理员 BCC                              │
│                                                         │
└────────────────────────────────────────────────────────┘
              │
              │ 备用通道（罕用，当扫码异常时兜底）
              ▼
   wangui.exe (Wails GUI) —— 本地 MITM 抓 token
```

两个独立子项目：

| 子项目 | 路径 | 跑哪 | 作用 |
|---|---|---|---|
| **wangui server** | `cmd/wangui/` `internal/` `web/` | Linux VPS (Docker) | 多租户后端 + 用户 SPA + 管理员后台 + 微信 OAuth 换 token |
| **wangui.exe (tokengrab)** | `tokengrab/` | 用户 Windows | Wails GUI，本地 MITM 抓 JWT。**备用通道** —— 扫码不可用时才需要 |

---

## 1. 主要功能

### wangui server

#### 用户侧
- **两步式激活**：先填邀请码 + PIN + 免责（公开），通过后才显示微信扫码 OAuth 界面（路人看不到 OAuth UI）
- **微信扫码登录学校** → 复制回调链接 → wangui 后端 OAuth code 换 token（不需要装任何客户端）
- **Token 自动续期**：用户在「账号」页扫码 / 粘链接 / 粘 JWT 都可（多通道兜底）
- **个人签到时刻**：每个用户激活时随机分配 0–9 分钟 trigger + 60s jitter，全部在 22:00–22:10 内完成（万一系统出错，留 20 分钟手动补救窗口）
- **自选签到日期**：周几掩码（每天 / 工作日 / 周末 / 自定义）
- **邮件通知**：用户 + 管理员 BCC，包括签到结果 + token 即将到期警告

#### 管理员侧
- **邀请码**：永久制，给信任朋友
- **临时朋友 (Guest 模式)**：admin 后台代为创建，按具体日期签到，到期自动 cleanup —— 给不熟的朋友 / 一次性场景
- **宿舍楼**：管理员维护可选签到位置 + 签到载荷模式（仅坐标 / 含地址）
- **签到日志**：全员近 200 条记录，按状态过滤
- **每日自动备份** + 7 份滚动保留
- **手动备份**：`docker compose exec wangui /usr/local/bin/wangui backup-now -data /data`

#### 数据
- SQLite WAL + AES-256-GCM 加密 token / SMTP 密码
- DPAPI / OS 钥匙库未用 —— 主密钥在 `.env`
- schema idempotent 自动迁移

### wangui.exe (tokengrab GUI) —— 备用通道

- 一键 MITM 抓学校 JWT（不再是主流程，仅备用）
- 持久根 CA + DPAPI 加密私钥（**首启一次** Windows 安全确认）
- 抓到自动复制剪贴板 + 跳 wangui 激活页
- 详见 [`tokengrab/README.md`](./tokengrab/README.md) + [`tokengrab/PROJECT.md`](./tokengrab/PROJECT.md)（原理逆向）

---

## 2. 目录结构

```
.
├── README.md                  ← 你正在看
├── ubuntu.md                  服务器部署完整流程
├── func.md                    调度器细节 + 个人签到时刻设计
├── final.md / fuck.md         开发笔记
│
├── go.mod / go.sum            主项目依赖
├── Dockerfile                 多阶段：Node SPA → Go binary → Alpine
├── docker-compose.yml         端口 / volume / .env
├── .env.example .dockerignore .gitignore
│
├── cmd/wangui/                main + go:embed SPA
│
├── internal/                  Go 业务代码
│   ├── api/                   学校 API client（GetUser / OAuth2Login / CheckinStatus / Sign）
│   ├── backup/                VACUUM INTO 每日 + 7 份保留
│   ├── notify/                邮件 dispatcher（user / admin BCC / guest cleanup）
│   ├── scheduler/             调度器 + guest cleanup ticker
│   ├── store/                 SQLite + 加密；users / dorms / codes / records / sessions
│   └── web/                   handlers / admin_handlers / school_oauth / server / rate limit
│
├── web/                       Vue 3 + TS + Tailwind 4
│   └── src/views/             Login (两步激活) / Dashboard / Settings / ... / admin/Guests
│
├── tokengrab/                 备用 Wails GUI（独立 go.mod）
│
└── scripts/
    └── redistribute_triggers.sql   存量用户 trigger_minute 打散的一次性 SQL
```

---

## 3. 快速开始

### 3.1 服务器部署（首次）

**前提**：Ubuntu 22.04+ VPS，2 GB+ 内存（Docker 构建期需要），1Panel 已装好且 80/443 反代可用。

```bash
git clone git@github.com:RoseKhlifa/wangui-henau.git /root/wangui
cd /root/wangui

cp .env.example .env
# 编辑 .env，填:
#   WANGUI_ADMIN_PASS    16+ 位随机字符
#   WANGUI_MASTER_KEY    openssl rand -hex 32 的输出（必须备份到密码管理器）
chmod 600 .env

docker compose up -d --build
docker compose logs -f wangui
```

期望日志：

```
store ready data_dir=/data
admin panel enabled
scheduler armed next_window_start=...
guest cleanup armed next=...
http server listening addr=0.0.0.0:5555
backup armed next=...
```

完整步骤（含 1Panel 反代 / 子域名 / 备份策略 / 升级 / 故障排查）见 [`ubuntu.md`](./ubuntu.md)。

### 3.2 朋友使用流程（主路径：扫码）

发给朋友这几样：

1. **邀请码**（admin 后台「邀请码管理」生成）
2. **wangui 站点 URL**（你的部署地址）

朋友操作：

1. 浏览器打开 wangui 站点 → 「激活」tab
2. **Step 1**：填邀请码 + PIN + 同意免责 → 点「下一步：获取学校 Token」
3. 服务器 precheck 邀请码 → 通过 → 自动跳 Step 2 + 显示二维码
4. **Step 2**：用手机微信扫二维码 → 学校晚归页面登录 → 点页面右上「⋯」→ 「复制链接」
5. 把链接粘到 wangui 输入框 → 点「激活账号」
6. 进 wangui「配置」→ 选自己宿舍楼 → 开自动签到
7. 每天 22:00–22:10 自动签，邮件通知结果

> 备用：扫码失败（极少）→ 让 admin 编译 `wangui.exe`（[`tokengrab/`](./tokengrab/)）给朋友，本地 MITM 抓 token 后粘贴到 wangui 激活页

### 3.3 添加临时朋友（admin 代管）

不熟的朋友 / 一次性场景：

1. admin 后台 → 「临时朋友」→ 「+ 新增临时朋友」
2. modal 填：备注 + 签到日期（日历多选）+ 绑定宿舍楼
3. 弹出二维码 → admin **截图发给朋友**
4. 朋友微信扫码 → 学校登录 → 复制链接 → **发回 admin**
5. admin 把链接粘到 modal 的输入框 → 「创建临时朋友」
6. 朋友账号在选定的日期自动签到，最后一天结束后**次日 02:00 自动 cleanup**

朋友**完全不接触 wangui 网站**。Token 抓取 / 用户管理全由 admin 代理。

---

## 4. 开发流程

```powershell
# 改代码 -- 任何编辑器

# 后端编译
go build ./...

# 前端类型检查
cd web ; npx vue-tsc --noEmit ; cd ..

# tokengrab
cd tokengrab ; go build ./... ; cd frontend ; npx vue-tsc --noEmit
```

推送 + 服务器部署：

```powershell
git add -A
git diff --cached --stat
git commit -m "feat: ..."
git push
```

```bash
ssh root@<服务器>
cd /root/wangui && git pull && docker compose up -d --build
docker compose logs -f wangui
```

DB schema 自动迁移，数据零丢失。

---

## 5. 环境变量

写在服务器 `/root/wangui/.env`（不进仓库）。

| 变量 | 必填 | 说明 |
|---|---|---|
| `WANGUI_ADMIN_PASS` | ✓ | 管理员后台登录密码，16+ 位随机字符 |
| `WANGUI_MASTER_KEY` | ✓ | AES-256 主密钥，`openssl rand -hex 32`；**丢了不可恢复，必须备份** |
| `TZ` | docker compose 已默认 | `Asia/Shanghai`，scheduler 用国内时区 |

---

## 6. 常用管理操作

### 生成邀请码（永久制）

admin → 「邀请码管理」→ 「生成邀请码」→ 数量 + 备注 → 弹 modal 复制。

### 添加临时朋友（一次性）

admin → 「临时朋友」→ 「+ 新增临时朋友」。详见 §3.3。

### 添加宿舍楼

admin → 「宿舍楼管理」→ 「添加宿舍楼」→ 起名 + 手动填 WGS84 经纬度 + 选签到载荷模式。

> 推荐 **Bing Maps** (`bing.com/maps`) 切卫星 → 右键楼栋 → 弹出框显示 6 位 WGS84。
> **不要**用百度 / 高德 / 腾讯（GCJ02 加密，偏 100–500 米）。

### 配置 SMTP

admin → 「系统设置」→ 「邮件通知」。Gmail 推荐：

| 字段 | 值 |
|---|---|
| Host | `smtp.gmail.com` |
| Port | `587` (STARTTLS) |
| 用户名 | Gmail 邮箱 |
| 应用专用密码 | Google 账号 → 安全性 → App Passwords（需先开两步验证） |
| 管理员收件邮箱 | 你自己，作为 BCC 收所有签到结果 + 临时朋友 cleanup 通知 |

### 看签到日志

admin → 「签到日志」全员近 200 条记录，按状态过滤。

### 强制重抓某人的 token

让用户进 wangui「账号」页 → 重新扫码 + 粘链接。会同时刷新头像。

### 手动备份

```bash
docker compose exec wangui /usr/local/bin/wangui backup-now -data /data
# 备份生成在 /root/wangui/data/backups/，rsync 拉到本地长期保留
```

### 重新打散存量用户的签到时刻

存量用户在 `trigger_minute=2` 升级前默认值 → 都集中在 22:02，看着像批量行为。一次性 SQL 重洗：

```bash
docker compose stop
sqlite3 data/wangui.db < scripts/redistribute_triggers.sql
docker compose start
```

详见 [`scripts/redistribute_triggers.sql`](./scripts/redistribute_triggers.sql) 注释。

---

## 7. 故障排查

详见 [`ubuntu.md`](./ubuntu.md) §10。常见几个：

| 现象 | 原因 + 解 |
|---|---|
| 容器启动后立即退出 | `docker compose logs` 看；多半 `.env` 没填或 `WANGUI_MASTER_KEY` 不是 64 字符 hex |
| 浏览器打开 502 | 1Panel 反代目标 `http://127.0.0.1:5555` 错或容器没起 |
| 朋友激活点「下一步」报「邀请码不存在」 | 邀请码拼错 / 已禁用 / 已被他人激活（不是他自己原激活码） |
| 朋友 step 2 粘链接报「学校 OAuth 登录失败」 | 回调链接已被用过（同一 code 只能换一次 token），让他重扫一次二维码 |
| 朋友 22:00 没签 | admin 「日志」搜他的 user_id 看 message：多半 token 过期 / 没绑宿舍楼 / 不在签到日期内 |
| 测试邮件发不出 | Gmail 没开两步验证，或填了登录密码而不是 App Password |
| 22:00 全员没签 | scheduler 容器时区不对：`docker compose exec wangui date` 看是不是 CST |
| 头像兜底显示首字 | 该用户激活时学校 CDN 抽风 fetch 失败；让他重新更新一次 token（自动重拉头像）|

---

## 8. 安全模型

| 组件 | 措施 |
|---|---|
| 学校 token 落库 | AES-256-GCM 加密，密钥仅在 `.env` |
| SMTP 密码 | 同上加密 |
| 用户 PIN | bcrypt cost 12 |
| Session | 64-byte 随机 ID，HttpOnly + SameSite=Lax，DB 存 expires_at |
| Admin 路径 | 混淆为 `/rosekhlifa`（不是 `/admin`），防扫描 |
| 邀请码 | 一码绑一学号；释放后可重激活 |
| 限流 | login / activate / precheck 共享 5/min IP 桶 |
| **两步激活** | step 1 通过 (precheck 邀请码) 才显示 step 2 的 OAuth UI；路人无邀请码看不到 OAuth 流程 |
| Guest 私密性 | 临时朋友无 PIN（不能从 wangui 登录），无邮箱（不收邮件），cleanup 后数据彻底删除 |
| tokengrab CA 私钥 | DPAPI 加密落地，仅本机本用户能解 |

---

## 9. License

无 license。私有仓库，仅供仓库所有者和明确授权的个人使用。**禁止任何形式的转发、转载、二次分发**。

---

## 10. 维护者

- 仓库所有者：仅自己
- AI 协作者：开发期 90% 代码由 Claude（Anthropic）辅助生成 + 审阅 + 集成
- 用户：≤5 个真实信任的朋友（永久邀请码）+ 偶尔几个临时朋友（Guest 模式）

—

拿到代码 + 邀请码的人，意味着你被信任。**不要扩散**。
