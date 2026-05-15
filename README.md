# wangui

> 学校晚归签到自动化系统 —— 内部自用、邀请制、≤5 人。
> **不对外开放，不对外宣传，不接受陌生人申请。**

---

## ⚠️ 风险与边界

这个工具替你和你的朋友**在每天 22:00–22:30 期间自动调用学校签到接口**。它的存在前提是：

1. **你确实在校** —— 不在校时让脚本签到 = 谎报位置 = 违纪。后果由用户承担
2. **token 是用户自己的** —— 工具不会盗号，也不读其他人的信息
3. **仅供 ≤5 个真实信任的朋友** —— 不公开、不收费、不传播
4. **学校发现就关停** —— 学校发文要求停止此类工具时，立即停服

这是一个**为方便而存在的灰色工具**，不是一个产品。仓库私有就是这个原因。

---

## 0. 这是什么

```
┌────────────────────────────────────────────────────────┐
│                                                         │
│   学校晚归签到系统 (xhbcs)                              │
│           ▲                                             │
│           │ JWT Bearer + 签到 API                       │
│           │                                             │
│   ┌───────┴────────┐    ┌─────────────────────────┐    │
│   │  wangui server │ ←  │  wangui.exe             │    │
│   │  (Go + Vue)    │    │  (Wails GUI)            │    │
│   │  Docker on VPS │    │  本地抓 Token 的工具    │    │
│   └────────┬───────┘    └─────────────────────────┘    │
│            │                                            │
│            │ 每天 22:00 替每个用户                       │
│            │ 调用学校签到 API                            │
│            ▼                                            │
│   邮件通知用户 + 管理员 BCC                              │
│                                                         │
└────────────────────────────────────────────────────────┘
```

两个独立子项目：

| 子项目 | 路径 | 跑哪 | 作用 |
|---|---|---|---|
| **wangui server** | `cmd/wangui/`, `internal/`, `web/` | Linux VPS (Docker) | 多租户后端 + 用户 SPA + 管理员后台 |
| **wangui (tokengrab)** | `tokengrab/` | 用户 Windows 电脑 | Wails GUI，抓取学校 JWT token |

---

## 1. 主要功能

### wangui server

- 邀请制注册（管理员发邀请码，用户拿邀请码 + 抓到的 token 激活）
- 每天 22:00–22:30 自动签到，jitter / retry / 重试间隔可配
- 自选签到日期（每天 / 工作日 / 周末 / 自定义周几）
- 多宿舍楼支持（管理员维护可选签到位置）
- 邮件通知（用户 + 管理员 BCC）：签到结果 + token 即将到期
- Token 自动续期：用户在「账号」页粘贴新 token 即可
- 管理员后台：用户管理、签到日志、SMTP 配置
- SQLite 存储 + AES-GCM 加密 token / SMTP 密码
- 每日自动备份 + 7 份滚动保留

### wangui (tokengrab GUI)

- 一键 MITM 抓取学校 token，免手抓包
- 持久根 CA + DPAPI 加密私钥（**首启一次** Windows 安全确认，之后再也不弹）
- 自动复制 token 到剪贴板 + 跳转 wangui 激活页（token / 邀请码 URL 预填）
- 调学校 `/auth/user` 自动拉用户卡片：学号 / 姓名 / 学院 / 班级

---

## 2. 目录结构

```
.
├── README.md                  ← 你正在看
├── PLAN.md                    设计文档（迭代过的产品思路）
├── final.md                   最终版设计 (Phase 2)
├── fuck.md                    临时开发笔记
├── ubuntu.md                  服务器部署完整流程（必看）
│
├── go.mod / go.sum            主项目依赖
├── Dockerfile                 多阶段构建：Node SPA + Go binary + Alpine
├── docker-compose.yml         端口 / volume / .env
├── .dockerignore .gitignore .env.example
│
├── cmd/wangui/                main + go:embed SPA
│   ├── main.go
│   └── embed.go
│
├── internal/                  Go 业务代码（领域分包）
│   ├── api/                   学校 API client（GetUser / CheckinStatus / Sign）
│   ├── backup/                VACUUM INTO 每日备份 + 滚动保留
│   ├── config/                YAML 配置（doctor / daemon 模式用）
│   ├── notify/                邮件 dispatcher（用户 + 管理员 BCC 规则）
│   ├── scheduler/             多租户调度器（22:00 窗口 + 自选签到日期）
│   ├── store/                 SQLite + 加密 / users / dorms / codes / records / sessions
│   └── web/                   HTTP server + handlers + admin handlers + rate limit
│
├── web/                       Vue 3 + TS + Tailwind 4
│   ├── package.json vite.config.ts tsconfig.json index.html
│   ├── public/
│   │   ├── logo.svg           房子 + 月亮 emerald logo
│   │   └── fonts/             HarmonyOS Sans SC + JetBrains Mono
│   └── src/
│       ├── main.ts router.ts api.ts types.ts style.css
│       ├── components/        Logo / Avatar / SidebarNav / AdminLayout / UserLayout / ThemeToggle
│       ├── stores/            auth / theme (Pinia-less，自管 reactive)
│       ├── lib/               format / toast / clipboard
│       └── views/
│           ├── Login.vue Dashboard.vue Settings.vue Records.vue Account.vue
│           └── admin/         Dashboard / Codes / Users / Dorms / Logs / Settings / Login
│
└── tokengrab/                 ← 独立子项目，独立 go.mod
    ├── README.md PROJECT.md   独立文档
    ├── go.mod go.sum wails.json
    ├── main.go app.go         Wails 入口 + IPC 绑定
    ├── internal/
    │   ├── ca/                持久 CA：crypt32 API + DPAPI 加密 + 注册表存储
    │   ├── proxy/             goproxy MITM
    │   ├── sysproxy/          Windows 系统代理读写
    │   ├── schoolapi/         直连 /auth/user 拉用户卡片
    │   ├── jwt/               不验签 JWT 解析
    │   ├── clipboard/ lock/   工具
    ├── frontend/              Vue 3 + Tailwind，跟主项目同款风格
    └── build/
        ├── appicon.png        1024×1024 房子+月亮，wails build 时转 ICO
        └── windows/
            ├── icon.ico       多尺寸 16-256
            └── info.json      exe 元数据
```

---

## 3. 快速开始

### 3.1 服务器部署（首次）

**前提**：一台 Ubuntu 22.04+ VPS，2 GB+ 内存（Docker 构建期需要），1Panel 已装好且 80/443 反代可用。

```bash
# 服务器
git clone git@github.com:RoseKhlifa/wangui-henau.git /root/wangui
cd /root/wangui

cp .env.example .env
# 编辑 .env，填:
#   WANGUI_ADMIN_PASS       16+ 位随机字符
#   WANGUI_MASTER_KEY       openssl rand -hex 32 的输出（必须备份到密码管理器）
chmod 600 .env

docker compose up -d --build
docker compose logs -f wangui    # 看启动日志
```

期望日志：

```
store ready data_dir=/data
admin panel enabled
scheduler armed next_window_start=...
http server listening addr=0.0.0.0:5555
backup armed next=...
```

`Ctrl+C` 退出日志（容器继续跑）。

完整步骤（含 1Panel 反代 + 子域名 + 备份策略 + 升级流程 + 故障排查）见 [`ubuntu.md`](./ubuntu.md)。

### 3.2 tokengrab.exe 编译

**前提**：装 [Go 1.25+](https://go.dev/dl/) + [Node 20+](https://nodejs.org/) + Wails CLI。

```powershell
# 一次性装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor    # 检查环境

# 编译
cd tokengrab
wails build -clean
# 产物在 build\bin\wangui.exe，约 15 MB，免装单文件
```

详细见 [`tokengrab/README.md`](./tokengrab/README.md) 和 [`tokengrab/PROJECT.md`](./tokengrab/PROJECT.md)（含 MITM 原理逆向）。

### 3.3 朋友使用流程

发给朋友这几样：

1. **邀请码**（管理员后台 → 「邀请码管理」→ 生成 1 张，给他）
2. **`wangui.exe`** 文件（在 `tokengrab/build/bin/` 编译产物）
3. **wangui 站点 URL**（你的部署地址，比如 `https://wangui.gptcodex.top`）

朋友操作步骤：

1. 双击 `wangui.exe` → 在微信 PC 端打开晚归签到入口 + 刷新 → 自动抓到 token
2. 填入邀请码（首次激活才需要）→ 点「打开 wangui 激活页」
3. wangui 网站自动切到激活页 + token / 邀请码已填好 → 设个 4–6 位 PIN → 激活完成
4. 进「配置」→ 选自己宿舍楼 → 「自动签到」打开 → 完成
5. 每天 22:00 自动签，邮件通知结果

---

## 4. 开发流程

### 4.1 本地编辑

```powershell
# 改代码 -- 任何编辑器

# 后端编译验证
go build ./...

# 前端类型检查
cd web; npx vue-tsc --noEmit; cd ..

# tokengrab
cd tokengrab; go build ./...; cd frontend; npx vue-tsc --noEmit
```

### 4.2 推送 + 部署

```powershell
git add -A
git status                   # 检查 staged 列表
git diff --cached            # 看清楚改了啥
git commit -m "feat: ..."
git push
```

服务器部署（在仓库根目录）：

```bash
ssh root@<服务器>
cd /root/wangui
git pull
docker compose up -d --build
docker compose logs -f wangui   # 验证启动正常
```

DB schema 自动迁移（idempotent ALTER TABLE），数据零丢失。

---

## 5. 环境变量

写在服务器的 `/root/wangui/.env`（不进仓库）。

| 变量 | 必填 | 说明 |
|---|---|---|
| `WANGUI_ADMIN_PASS` | ✓ | 管理员后台登录密码，16+ 位随机字符 |
| `WANGUI_MASTER_KEY` | ✓ | AES-256 主密钥，`openssl rand -hex 32` 生成；丢了所有 token + SMTP 密码不可恢复，**必须备份** |
| `TZ` | docker compose 已默认 | `Asia/Shanghai`，让 scheduler 用国内时区 |

---

## 6. 常用管理操作

### 生成邀请码

管理员后台 → 「邀请码管理」→ 「生成邀请码」→ 数量 + 备注 → 弹出 modal 一键复制。

### 添加宿舍楼

管理员后台 → 「宿舍楼管理」→ 「添加宿舍楼」→ 起名 + 手动填 WGS84 经纬度（FAQ 在 modal 内）→ 选「签到载荷」模式 → 保存。

> 推荐用 **Bing Maps** (`bing.com/maps`) 切卫星图 → 右键楼栋 → 弹出气泡显示 6 位 WGS84。**别用百度/高德/腾讯**（GCJ02 加密偏 100–500 米）。

### 配置 SMTP

管理员后台 → 「系统设置」→ 「邮件通知」section。

Gmail 推荐：

| 字段 | 值 |
|---|---|
| Host | `smtp.gmail.com` |
| Port | `587` (STARTTLS) |
| 用户名 | Gmail 邮箱 |
| 应用专用密码 | 在 Google 账号 → 安全性 → App Passwords 生成（需先开两步验证）|
| 管理员收件邮箱 | 你自己邮箱，作为 BCC 收所有用户签到结果 |

### 看签到日志

管理员后台 → 「签到日志」可看全员近 200 条 sign_records，按状态过滤（成功/已签/免签/失败/跳过）。

### 强制重抓某人的 token

后端没暴露这个操作 —— 让用户自己跑 `wangui.exe` 抓新 token，回 wangui Account 页粘贴更新即可。

### 手动备份

```bash
docker compose exec wangui /usr/local/bin/wangui backup-now -data /data
# 备份生成在 /root/wangui/data/backups/，rsync 拉到本地长期保留
```

---

## 7. 故障排查

详见 [`ubuntu.md`](./ubuntu.md) §10。常见几个：

| 现象 | 原因 + 解 |
|---|---|
| 容器启动后立即退出 | `docker compose logs` 看错；多半 `.env` 没填或 `WANGUI_MASTER_KEY` 不是 64 字符 hex |
| 浏览器打开 `https://站点/` 502 | 1Panel 反代目标 `http://127.0.0.1:5555` 没改对，或容器没起 |
| 朋友 22:00 没签 | admin 「日志」搜他的 user_id，看 message。多半 token 过期 / 没绑宿舍楼 / 不在签到日期内 |
| 测试邮件发不出 | Gmail 没开两步验证，或填了登录密码而不是 App Password |
| 22:00 没人能签 | scheduler 容器时区不对：`docker compose exec wangui date` 看是不是 CST |
| `wangui.exe` 抓不到 token | 微信 PC 端被防火墙 / 杀软干扰，把 `wangui.exe` 加白名单；或重启微信再试 |

---

## 8. 安全模型

| 组件 | 措施 |
|---|---|
| 学校 token 落库 | AES-256-GCM 加密，密钥仅在 `.env` 里（`WANGUI_MASTER_KEY`） |
| SMTP 密码落库 | 同上，加密存 system_config |
| 用户登录 PIN | bcrypt cost 12 |
| Session | 64-byte 随机 ID，HttpOnly + SameSite=Lax cookie，DB 存 expires_at |
| Admin 路径 | 已混淆成 `/rosekhlifa`（不是 `/admin`），防扫描 |
| 邀请码 | 一码绑一学号；释放后可重激活 |
| 限流 | 登录接口 5/min（防爆破） |
| tokengrab CA 私钥 | DPAPI 加密落地，仅本机本用户能解 |
| tokengrab token | 不落地，抓到立即复制剪贴板 + 走 fragment (#) 传 wangui |

---

## 9. License

无 license。私有仓库，仅供仓库所有者和明确授权的个人使用。**禁止任何形式的转发、转载、二次分发**。

---

## 10. 维护者

- 仓库所有者：仅自己
- AI 协作者：开发期 90% 代码由 Claude（Anthropic）辅助生成 + 审阅 + 集成
- 用户：≤5 个真实信任的朋友（邀请制）

—

如果你（朋友）拿到了这份代码 + 邀请码，意味着你被信任。**不要扩散**。
