# 河南农业大学晚归自动签到系统 —— 计划书

> 起草日期：2026-05-14
> 作者：曹元桦 (学号 2521241784，软件学院 软工25-26)
> 状态：**v2 草稿，已接受红线约束，进入编码**

---

## 1. 项目目标

为本人及**少数受邀朋友**（≤ 5 人，邀请制）实现 **河南农业大学晚归签到系统** (`https://xhbcs.henau.edu.cn`) 的自动化打卡，避免错过 22:00–22:30 的 30 分钟窗口。

### 1.1 形态演进

- **Phase 1**：本机单租户 CLI，配置文件读 token，cron 触发，证明签到链路可走通。
- **Phase 2**：升级为 **多租户 Web 服务**（Go 后端 + Vue 前端 + SQLite），朋友自助接入：
  - 朋友自己用 TokenGrab 抓 token → 在 Web 端粘贴保存 → 后台自动签到。
  - 每位朋友的账号、token、责任都在他自己。
- **Phase 3**：云服务器部署，systemd 守护，HTTPS（Caddy / Cloudflare Tunnel）。

### 1.2 红线（已确认接受）

- **邀请制**：仅向私下认识的朋友开放，**不公开域名、不放 GitHub Public、不在群里宣传**。
- **不代签**：每个用户必须**自己抓自己的 token**，运维方（本人）不接触登录凭证以外的信息。
- **token 加密存储**：DB 中 token 字段 AES-GCM 加密，主密钥从环境变量 `WANGUI_MASTER_KEY` 读取，**不写入任何配置文件、不入仓库**。
- **强制免责声明**：用户首次保存 token 前必须勾选"我已了解此工具非官方、风险自担、不会用于不在校时签到"。
- **不在校禁签**：用户可一键开关「今日不签 / 长期不签」，运维方在 PLAN.md 中明确：**实际不在校时签到 = 谎报位置，性质最重，使用者全责**。

---

## 2. 系统侦察结论

### 2.1 站点性质

- 前端：Vite + Vue 3 SPA，"只能微信打开"是前端 `User-Agent` 检测，**后端不校验**。
- 后端：标准 REST + `Authorization: Bearer <JWT>`，**无签名 / 无 nonce / 无设备指纹**。
- 登录：微信网页授权（snsapi_userinfo），前端拿 code POST `/api/auth/oauth2/login` 换 JWT。

### 2.2 接口清单（base: `https://xhbcs.henau.edu.cn/api`）

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/auth/user` | 用户信息，**用于校验 token + 拿用户身份** |
| GET | `/auth/permissions` | 权限码 |
| GET | `/checkin/available-rules` | 当前可签规则列表（拿 ruleId） |
| GET | `/checkin/status?ruleId=` | 当前签到状态 |
| **POST** | **`/checkin`** | **核心：执行签到** |
| GET | `/checkin/records` | 历史记录（参数名待确认） |

### 2.3 当前唯一规则

```json
{ "ruleId": 1, "ruleName": "晚归签到考勤规则",
  "startTime": "22:00:00", "endTime": "22:30:00" }
```

实际窗口 **22:00:00–22:30:00**（30 分钟）。

### 2.4 POST /checkin 请求体

```json
{
  "ruleId": 1,
  "latitude": 34.xxxxxx,
  "longitude": 113.xxxxxx,
  "deviceModel": "iPhone",
  "deviceSystem": "iOS",
  "locationAddress": "河南省郑州市...",
  "city": "...", "road": "...", "poi": "..."
}
```

后端只校验经纬度是否落在规则圆心半径内（半径未知，需实测）；不验证 GPS 真实性。

### 2.5 JWT

`HS256`，payload `{"iss": "<user_id>", "exp": <unix_ts>}`，有效期约 7 天。
**不尝试伪造，仅作为已签发凭证使用。**

---

## 3. 技术架构

### 3.1 单二进制部署

无论 Phase 1 还是 Phase 2，最终都是**一个 Go 可执行文件**，零外部依赖（modernc.org/sqlite 纯 Go 实现 SQLite，前端构建产物用 `embed.FS` 嵌入）。

### 3.2 目录结构

```
fuckwangui/
├── cmd/
│   └── wangui/
│       └── main.go              # 入口，子命令分发
├── internal/
│   ├── api/                     # 河农签到系统 API 客户端（Phase 1+2 共用）
│   │   ├── client.go            #   HTTP 客户端 + Bearer 注入 + 通用错误
│   │   ├── auth.go              #   /auth/user, /auth/permissions
│   │   └── checkin.go           #   /checkin/* 全部接口
│   ├── config/                  # Phase 1：YAML 配置读取
│   │   └── config.go
│   ├── store/                   # Phase 2：SQLite 多租户存储
│   │   ├── store.go             #   users / records / settings 三张表
│   │   └── crypto.go            #   AES-GCM token 加密
│   ├── scheduler/               # 调度器（Phase 1 单任务，Phase 2 多租户）
│   │   └── scheduler.go
│   ├── notify/                  # 通知抽象（Phase 1 日志，Phase 2 可扩展）
│   │   └── notify.go
│   └── web/                     # Phase 2：HTTP API + 静态文件 embed
│       ├── server.go
│       ├── handlers.go
│       └── middleware.go
├── web/                         # Phase 2：Vue 3 前端源码
│   ├── src/
│   ├── package.json
│   └── vite.config.ts
├── config.example.yaml
├── config.yaml                  # gitignore
├── data/                        # Phase 2：sqlite 文件目录（gitignore）
├── go.mod
├── PLAN.md
└── .gitignore
```

### 3.3 CLI 子命令（Phase 1 主战场）

```
wangui doctor                    # 自检：token 有效性、规则获取、配置完整
wangui status                    # 查 /checkin/status 当前状态
wangui sign                      # 立即签到一次（手动触发）
wangui daemon                    # 常驻进程，按 cron 自动签到
wangui serve                     # Phase 2：启动 Web 服务
```

### 3.4 调度策略（Phase 1 单租户）

- **22:02 + random(0,180)秒** 首次尝试
- 失败重试节奏：**22:08 / 22:15 / 22:22**
- 每次签到前先 `GET /checkin/status` 判断：
  - `canCheckin=false` → 跳过
  - `hasCheckedIn=true` → 跳过 + 日志「已签到」
  - 否则 → POST 签到
- 22:30 前仍未成功 → **日志 ERROR 级别 + Phase 2 起强通知**

### 3.5 Phase 2 Web 架构

**后端 (Go)**：

- Router：`chi`
- DB：`modernc.org/sqlite`（纯 Go，无 CGO）
- 会话：cookie 中放服务器签发的 web session token（独立于学校 JWT），**绝不把学校 JWT 暴露给浏览器**
- 关键流程：
  1. 朋友访问首页 → 看到免责声明 → 勾选 → 进入"绑定"页
  2. 粘贴学校 JWT → 后端调 `/auth/user` 校验 → 解 JWT 拿 `iss` 作 user_id
  3. AES-GCM 加密后入 `users` 表
  4. 颁发 web cookie session（有效期 30 天，与学校 JWT 解绑）
  5. 后续 Web 操作（看记录、改设置、更新 token）走 web cookie，**学校 JWT 仅用于后台调用学校 API**

**前端 (Vue 3 + Vite + Tailwind)**：

按你给的截图复刻五个模块：

```
┌─────────────────────────────────────┐
│  签到系统           首页   退出      │
├─────────────────────────────────────┤
│  学生信息（曹元桦/学号/学院/班级）   │
├─────────────────────────────────────┤
│  Token 状态                          │
│    ✅ 有效                           │
│    剩余 X天Y小时（进度条）           │
│    [更新 Token: 输入框 + 保存按钮]   │
├─────────────────────────────────────┤
│  签到配置                            │
│    自动签到: [开启(22:00) ▾]         │
├─────────────────────────────────────┤
│  签到记录                            │
│    日期 / 状态 / 位置                │
└─────────────────────────────────────┘
```

**数据库 schema（Phase 2）**：

```sql
CREATE TABLE users (
  user_id      TEXT PRIMARY KEY,       -- 来自 JWT 的 iss
  user_name    TEXT,                   -- /auth/user 拿
  user_number  TEXT,                   -- 学号
  user_section TEXT,                   -- 学院
  user_class   TEXT,                   -- 班级
  token_enc    BLOB NOT NULL,          -- AES-GCM(token)
  token_exp    INTEGER NOT NULL,       -- JWT exp
  auto_sign    INTEGER NOT NULL DEFAULT 1,
  lat          REAL,                   -- 用户保存的常用坐标
  lng          REAL,
  location_meta TEXT,                  -- JSON: city/road/poi/address
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);

CREATE TABLE sign_records (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      TEXT NOT NULL,
  rule_id      INTEGER NOT NULL,
  status       TEXT NOT NULL,          -- success / failed / skipped
  message      TEXT,
  occurred_at  INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(user_id)
);

CREATE TABLE web_sessions (
  session_id   TEXT PRIMARY KEY,
  user_id      TEXT NOT NULL,
  expires_at   INTEGER NOT NULL
);
```

**Web API 端点**（前缀 `/api/v1`，全部走 web cookie 鉴权除注明外）：

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/bind`  | （无鉴权）粘贴学校 JWT，校验 + 入库 + 颁发 web session |
| GET  | `/me`    | 当前用户信息 + token 状态 |
| PUT  | `/token` | 更新学校 token |
| GET  | `/settings` | 读签到配置 |
| PUT  | `/settings` | 改签到配置（auto_sign / 坐标） |
| GET  | `/records` | 签到记录分页 |
| POST | `/sign-now` | 手动立即签到（测试用，仅在签到窗口内可调） |
| POST | `/logout` | 清会话 |

---

## 4. 安全与合规

| 项 | 措施 |
|---|---|
| Token 加密 | AES-GCM，主密钥仅在环境变量 |
| 日志脱敏 | 所有日志中 token 仅打印前 8 + 后 4 字符 |
| 会话安全 | web cookie 设置 `HttpOnly + Secure + SameSite=Lax` |
| CSRF | 改写型接口走 SameSite=Lax + Origin 校验 |
| 速率限制 | `/bind` 端点按 IP 限流，防止有人枚举尝试 |
| 数据隔离 | 所有 DB 查询强制带 user_id where 条件 |
| 备份 | sqlite 文件每天打 tar，留 7 份 |
| 注销 | 用户可一键删除自己的所有数据 |

---

## 5. 里程碑

### Phase 1（今晚目标：跑通签到链路）

- ✅ **M0 侦察**：接口逆向、字段确认、token 实测
- ✅ **M1 计划书 v2**：本文档
- 🔜 **M2 API 客户端**：`internal/api` 全部接口封装；`wangui doctor / status / sign` 可手动跑通
- 🔜 **M3 单租户调度**：`wangui daemon` 模式，cron + 重试
- 🔜 **M4 今晚 22:00 实战**：你抓真实坐标 → 填 config → 跑 `wangui sign` 验签到 → 启 `daemon`

### Phase 2（Web 多租户，本周内）

- 🔜 **M5 SQLite 存储 + AES-GCM 加密**
- 🔜 **M6 Web 后端 (chi)**：bind / me / settings / records / sign-now
- 🔜 **M7 Vue 前端**：复刻你给的 5 个卡片
- 🔜 **M8 多租户调度器**：每个用户独立 goroutine 监听其规则窗口
- 🔜 **M9 邀请码注册** + 强制免责声明

### Phase 3（部署）

- 🔜 **M10 云服务器** + systemd + 自动备份
- 🔜 **M11 HTTPS**（Caddy 自动证书 或 Cloudflare Tunnel）
- 🔜 **M12 监控**：token 过期、签到失败统计

---

## 6. 风险声明（Phase 2 已升级）

| 风险 | 等级 | 说明 |
|---|---|---|
| 实际不在校时仍自动签到 | 🔴 最高 | 性质为「谎报位置」，使用者全责。脚本必须有"今日不签"开关 |
| Token 数据库被脱 | 🟠 高 | AES-GCM 加密；主密钥隔离；sqlite 文件权限严格 |
| 学校发现并定性为"组织化绕过" | 🟠 高 | 邀请制 + 不公开 + ≤ 5 人；若被问及，运维方负责说明 |
| 学校改 API（如加签名/校验设备） | 🟡 中 | 监控异常返回；提前准备"暂停服务"流程 |
| 朋友 token 泄露后被他人滥用 | 🟡 中 | DB 加密 + web 端只允许 token 写入不允许读出 |
| 服务器被攻击 / 域名暴露 | 🟢 低 | 不公开域名，加 Cloudflare 防爬 |

---

## 7. 待你最终确认（编码前）

1. **PLAN v2 是否同意进入 M2**？同意我立即建 Go 项目骨架。
2. **Phase 2 启动时机**：先 M2→M3→今晚实战 → 再进 Phase 2 ✅；还是其他次序？
3. **服务器**：你已经有云服务器了吗？没有的话部署前我会告诉你最小配置（1c1g 5M 带宽 + Linux 即可）。
4. **域名**：Phase 3 部署需要一个域名（不能用 IP，浏览器不让存 cookie），有没有？

---

*v2 草稿。确认后进入 M2 编码。*
