# Token Grab GUI — 项目书

> wangui 配套的 GUI 版 token 抓取工具，替代第三方 `TokenGrab.exe`（命令行）。
> 仅供 wangui 内部 ≤5 个朋友自用，跟 wangui 同一隐匿策略。

---

## 0. 现有 `TokenGrab.exe` 工作原理逆向

源码不可得，但根据它的运行日志 + 这类工具的通用模式，可以高置信度还原它的实现路径。

### 0.1 总体技术：HTTPS 中间人代理 (MITM)

目标 token 是 Bearer JWT，藏在签到接口请求的 `Authorization: Bearer <jwt>` HTTP header 里。学校 API 走 HTTPS，所以**必须解密 TLS 才能读到这一行**。

实现路径：在用户机器上跑一个本地代理 + 一张让本机信任的"假 CA"，让所有走代理的 HTTPS 流量被解密、被代理读到明文 header、再加密转发到真服务器，对客户端完全透明。

### 0.2 四步流程 ↔ 实际操作

| 它的输出 | 实际在做什么 |
|---|---|
| `[1/4] 生成临时证书` | 生成一对 RSA 密钥 + 自签一张 X.509 CA 证书（v3、CA=true、随机 serial、CN 随便），私钥只活在内存 |
| `[2/4] 安装证书` | 把 CA 公钥导入 Windows「受信任的根证书颁发机构」证书库。命令级是 `certutil -addstore Root <der>`，API 级是 `CertOpenStore + CertAddCertificateContextToStore`。可选 `LocalMachine`（需管理员）或 `CurrentUser`（普通权限即可） |
| `[3/4] 设置系统代理` | 改注册表 `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`：`ProxyEnable=1, ProxyServer=127.0.0.1:<port>`；调 `InternetSetOption(INTERNET_OPTION_SETTINGS_CHANGED) + (INTERNET_OPTION_REFRESH)` 通知 WinINet/Edge/Chromium 立即生效 |
| `[4/4] 启动抓包代理` | 本地监听 :port 当 HTTP CONNECT 代理；对每个 CONNECT 目标，**不**做透明 tunneling，而是用刚才的 CA 现签发一张 CN=目标域 的叶子证书，跟客户端做 TLS 握手 |

### 0.3 抓取阶段

1. 用户在微信 PC 端打开签到入口
2. 微信内嵌的 WebView2 (基于 Edge / Chromium) 加载这个页面
3. WebView2 **默认遵循系统代理** → 流量进入本地代理 (步骤 3 设的)
4. WebView2 **默认信任 Windows 系统根证书库** → 接受我们签发的叶子证书 (步骤 2 装的 CA 之下)
5. 用户登录 / 刷新页面 → 浏览器调 `/api/auth/...` 等接口，带 `Authorization: Bearer <jwt>` header
6. 这个请求在本地代理这一端被解密，明文 header 可读 → 代理提取 token
7. 同时代理把请求继续转发到真正的服务器，把响应原样返回给客户端 (对用户完全透明)

### 0.4 JWT 解析

- 不验签（HS256 密钥在学校服务器手里），**只读 claims**：
  - `parts := strings.Split(jwt, ".")` 取 `parts[1]`
  - `base64.RawURLEncoding.DecodeString(parts[1])` → JSON
  - `iss` 字段 = 学号对应的内部 user ID（输出里的 `4599`）
  - `exp` 字段 = 过期时间 (Unix epoch)，格式化后是 `2026-05-21 13:08`

### 0.5 输出 + 清理

抓到第一条命中目标域名的 token 后：
- 写到 `tokens.txt`
- 复制到 Windows 剪贴板（`OpenClipboard / EmptyClipboard / SetClipboardData(CF_UNICODETEXT)`）
- 在控制台美化输出（ANSI 颜色 + UTF-8 框线字符）

然后逆向清理：
- 关闭代理监听
- 把注册表的 `ProxyEnable` 恢复成抓取前的原值（或干脆置 0）
- 从根证书库删除刚装的 CA（`certutil -delstore Root <thumbprint>`）
- 删临时证书 / 密钥文件
- 程序退出

### 0.6 关键启发：为什么"在微信里打开"管用

- 微信 PC 端不用自己的网络栈，用 Edge WebView2 嵌入网页
- WebView2 是 Chromium 内核，**默认走 Windows 系统代理 + Windows 系统证书库**
- 所以一旦步骤 2/3 完成，微信打开的任何 HTTPS 网页都被透明 MITM

普通浏览器（Chrome / Edge）也同样可以抓到。Firefox 自带独立证书库，需要单独配置，所以这类工具习惯让你"在微信里打开"。

---

## 1. 项目目标

复刻同样的抓取能力，但：

1. **GUI 化**：替代黑底白字命令行，提供跟 wangui 主项目同款 emerald 主题
2. **更友好的提示**：每一步有图标和状态，新手不慌
3. **更强的清理保证**：异常退出、任务管理器杀死、Ctrl+C 都能完整回滚（命令行版可能漏）
4. **一键打开 wangui 激活页**：抓到 token 后剪贴板已就绪，下一步是粘贴到 wangui，UI 上提供按钮直接跳转（可选）

---

## 2. 风险与缓解

| 风险 | 严重 | 缓解 |
|---|---|---|
| 装根 CA = 把"任何人都能签发任意域名假证书"的权力给了这个 CA 的私钥持有者；如果私钥泄露，攻击者可以中间人这台机器上**所有** HTTPS | 高 | 一次性 CA：每次启动重新生成；私钥只活在内存，进程退出立即销毁；CA 在系统证书库的存活时间限定为代理监听期内（几十秒至两分钟）；抓到 token 立即卸载 |
| 杀毒软件可能告警（修改根证书 + 改系统代理 = 病毒特征） | 中 | 文档里如实说明；首启弹窗征得用户同意；签名 exe 可减少误报但 EV 证书要钱 |
| 系统代理来不及恢复导致后续上网中断 | 中 | 用 `defer` 风格层层注册清理；窗口关闭、Ctrl+C、panic 都触发；额外提供独立的「恢复」按钮兜底 |
| 目标域使用 Certificate Pinning | 低 | 学校系统通常没做 pinning；如果做了，MITM 失败 → 提示用户改用 Reqable 手抓 |
| 用户多开导致两个代理串扰 | 中 | 文件锁单例（`flock` 或 named mutex），同时只能跑一个 |
| 用户在抓取中关电脑 / 拔电源 | 低 | 下次启动时主动检查注册表里 `ProxyEnable` 和证书库残留，提示"上次未清理干净，立即恢复" |

---

## 3. 技术选型

| 维度 | 选 | 理由 |
|---|---|---|
| 语言 | **Go 1.25** | 跟 wangui 主项目一致；单二进制好分发；`golang.org/x/sys/windows` 写注册表 + 调 WinAPI 方便 |
| GUI 框架 | **Wails v2** | 前端用 Vue 3 + Tailwind，跟 wangui 完全同款；输出单 exe ~15 MB；Win10+ 用自带 WebView2，无需打包 Chromium |
| MITM 引擎 | **github.com/elazarl/goproxy** | 成熟、API 简单；支持 `OnRequest`/`OnResponse` 函数式拦截 + `MitmConnect` 动态签发叶子证书 |
| 证书生成 | **crypto/x509 标准库** | 无外部依赖；RSA-2048 自签 CA + 叶子证书 |
| Windows 证书库 | **golang.org/x/sys/windows + syscall** | 调 `crypt32.dll` 的 `CertOpenStore`、`CertAddCertificateContextToStore`、`CertDeleteCertificateFromStore`；fallback：`exec.Command("certutil", "-addstore", "-user", "Root", ...)` |
| 系统代理设置 | **golang.org/x/sys/windows/registry** + `InternetSetOptionW` | 改 InternetSettings；通知 WinINet 刷新 |
| 剪贴板 | **github.com/atotto/clipboard** | 跨平台，几行代码搞定 |
| 单例锁 | **github.com/gofrs/flock** | 跨平台文件锁，比 named mutex 简单 |

体积估算：Go 二进制 + Wails runtime ≈ **15–20 MB**。

---

## 4. UI 设计

### 视觉风格
跟 wangui 同款：暗黑底 (`zinc-950`) + emerald-400 强调色 + HarmonyOS Sans SC / JetBrains Mono 字体 + 跟 wangui 主项目的 `Logo.vue` 同款 logo（房子 + 月亮）。

窗口尺寸：`560 × 720`，居中显示，不可调整大小（避免布局问题）。

### 三态切换

#### 态 A：首屏（idle）

```
┌─────────────────────────────────┐
│  🌒  Token Grab        × □ —    │
├─────────────────────────────────┤
│                                 │
│         [Logo 64×64]            │
│         Token Grab              │
│         给 wangui 用的           │
│                                 │
│  ─────────────────────────      │
│                                 │
│  使用步骤：                     │
│    1. 在微信 PC 端打开签到入口  │
│    2. 点下方按钮                │
│    3. 在微信里刷新一下页面      │
│                                 │
│  ┌───────────────────────────┐  │
│  │      开始抓取    ▸        │  │ ← emerald 大按钮
│  └───────────────────────────┘  │
│                                 │
│  ⓘ 首次使用会装一张临时根证书， │
│     退出时会自动卸载            │
│                                 │
└─────────────────────────────────┘
```

#### 态 B：抓取中（capturing）

```
┌─────────────────────────────────┐
│  🌒  Token Grab                 │
├─────────────────────────────────┤
│                                 │
│       ⏳ 抓取中…                │
│                                 │
│  ✓ 生成临时证书                 │
│  ✓ 信任 CA                      │
│  ✓ 设置系统代理                 │
│  ◌ 等待签到入口请求…  (32s)     │
│                                 │
│  ─────────────────────────      │
│                                 │
│  请现在在微信打开签到入口       │
│  并按 Ctrl+R 刷新一下           │
│                                 │
│  ┌───────────────────────────┐  │
│  │      取消并清理            │  │ ← 红色 outline 按钮
│  └───────────────────────────┘  │
└─────────────────────────────────┘
```

#### 态 C：抓到了（captured）

```
┌─────────────────────────────────┐
│  🌒  Token Grab                 │
├─────────────────────────────────┤
│                                 │
│       ✓ 抓取成功                │
│                                 │
│  用户 ID    4599                │
│  有效期     6 天 23 小时        │
│  过期时间   2026-05-21 13:08    │
│                                 │
│  ┌───────────────────────────┐  │
│  │ eyJhbGciOiJIUzI1NiIsInR5… │  │ ← mono 字体，单行截断
│  └───────────────────────────┘  │
│                                 │
│  ✓ 已复制到剪贴板               │
│                                 │
│  ┌──────────────┬─────────────┐ │
│  │ 复制         │ 再抓一次    │ │
│  └──────────────┴─────────────┘ │
│  ┌───────────────────────────┐  │
│  │ 打开 wangui 激活页 (可选) │  │
│  └───────────────────────────┘  │
└─────────────────────────────────┘
```

### 关键交互细节

- **首次启动弹安全说明 Modal**：告知会装根证书 + 设系统代理，退出会自动卸载，"我同意" 才能继续
- **任何关闭方式都触发清理**：Wails 的 `OnShutdown` + Go 的 `signal.NotifyContext` 双重保险
- **抓到第一个 token 立即停止**：不持续监听
- **60 秒超时**：还没抓到 → 自动停止 + 提示"请确认在微信打开了签到入口"
- **首屏的"上次未清理"红色横幅**：启动时若检测到注册表有残留代理设置或证书库有 wangui-tokengrab 签发的 CA → 显示"上次异常退出，点此清理"

---

## 5. 模块拆分

```
tokengrab/
├── go.mod                       # 独立 module，不依赖 wangui
├── go.sum
├── main.go                      # Wails app entry
├── app.go                       # GUI ↔ backend bindings
├── wails.json
├── internal/
│   ├── ca/
│   │   ├── ca.go                # 生成 CA + 叶子证书
│   │   └── install_windows.go   # 装 / 卸 系统证书库
│   ├── proxy/
│   │   ├── proxy.go             # goproxy 启动 + 域名过滤
│   │   └── filter.go            # 提取 Bearer token
│   ├── sysproxy/
│   │   └── sysproxy_windows.go  # 注册表读写 / 恢复
│   ├── jwt/
│   │   └── parse.go             # 不验签解析 claims
│   ├── clipboard/
│   │   └── clipboard.go         # 包装 atotto
│   └── lock/
│       └── lock.go              # 单例文件锁
├── frontend/
│   ├── package.json
│   ├── vite.config.ts
│   ├── tailwind.config.js
│   ├── src/
│   │   ├── main.ts
│   │   ├── App.vue              # 主页面，含三态
│   │   ├── components/
│   │   │   ├── Logo.vue         # 直接复用 wangui 的
│   │   │   └── StateXxx.vue     # 三个态分别一个组件
│   │   ├── lib/
│   │   │   └── format.ts        # 剩余时间格式化
│   │   └── styles.css           # tailwind + wangui 同款 design tokens
│   └── public/
│       └── fonts/               # HarmonyOS + JetBrains（复用 wangui）
└── build/
    └── windows/
        └── icon.ico             # exe 图标，从 wangui logo.svg 导出
```

---

## 6. 实施步骤 / 工时估计

| Phase | 任务 | 工时 |
|---|---|---|
| **P0** | `wails init` + 基础窗口 + 复用 wangui 字体 / 配色 / Logo | 2h |
| **P1** | `internal/ca`：自签 CA + 动态叶子 + Windows 装/卸载 | 3h |
| **P2** | `internal/proxy`：goproxy 起 MITM + 目标域过滤 + token 提取 | 3h |
| **P3** | `internal/sysproxy`：注册表 + InternetSetOption | 1h |
| **P4** | `internal/jwt` + `internal/clipboard` + `internal/lock` | 1h |
| **P5** | UI 接线：三态切换 + 进度文字 + 倒计时 + 错误提示 | 3h |
| **P6** | 清理保证：OnShutdown + signal + 启动残留检测 | 2h |
| **P7** | 打包：`wails build` 出单 exe + 测试免装运行 + 各杀软误报排查 | 2h |
| **总计** | | **~17h** |

---

## 7. 跟 wangui 的衔接策略

**轻量方案（推荐）**：tokengrab 抓到 token → 复制到剪贴板 → 提示用户"现在去 wangui 激活页粘贴"。结束。**不知道 wangui 跑在哪个域名**，朋友们可以各自配各自的。

**深耦合方案（不推荐）**：tokengrab GUI 里有个输入框填 wangui 域名，抓到后跳转到 `https://<域>/login?tab=activate&prefill=<token>`，激活页 URL 参数预填 token。需要 wangui 前端也加 URL query 处理。

推荐轻量方案，避免两个工具硬绑死。

---

## 8. 项目位置

`D:\fuckwangui\tokengrab\`，**独立 go.mod**（跟 wangui 主项目解耦）。

主项目的 `.dockerignore` 加一行：

```
tokengrab/
```

避免它进 wangui 的 Docker 镜像。

—

## 9. 待确认事项

1. **GUI 框架**：Wails v2（推荐，跟主项目栈一致）/ Fyne（更纯 Go 但难匹配 wangui 风格）/ 纯命令行带颜色（最简，但你说要 GUI）
2. **衔接深度**：剪贴板（推荐）/ URL 跳转激活页
3. **目标平台**：仅 Windows（推荐，微信 PC 端主战场）/ +macOS / +Linux
4. **签 exe**：跳过（容忍杀软误报）/ 给个数字签名（要花钱）

把这四个定下来我就动手。
