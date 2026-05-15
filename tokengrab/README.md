# Token Grab

wangui 配套的 GUI 版 token 抓取工具。

> ⚠️ 仅供 wangui 内部 ≤5 人自用。详细技术原理 + 风险分析见 [PROJECT.md](./PROJECT.md)。

---

## 给最终用户

1. 双击 `tokengrab.exe`
2. 在微信 PC 端打开晚归签到入口
3. 在 Token Grab 里点「开始抓取」
4. 回微信，按 `Ctrl+R` 刷新页面
5. token 自动复制到剪贴板 + 显示在窗口里
6. 点窗口里的「打开 wangui 激活页」(可选)
7. 在 wangui 激活页粘贴 token

抓到 token 或关窗口时，工具会**自动**卸载临时证书 + 恢复系统代理。

—

### 杀毒软件可能误报

工具会在 Windows 信任的根证书库**临时**装一张本机一次性 CA，并设置系统代理。
正常杀软可能告警。给杀软添加白名单 / 信任此 exe 即可。

—

### 万一异常退出（断电 / 任务管理器杀死）

下次启动时窗口顶部会显示"上次未清理"红条，点「立即清理」即可恢复。

---

## 给开发者

### 依赖

| 工具 | 版本 |
|---|---|
| Go | 1.25+ |
| Node + npm | Node 20+ |
| Wails CLI | v2.10+ |

装 Wails CLI（一次性）：

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor    # 检查环境
```

### 开发模式（热重载）

```powershell
cd D:\fuckwangui\tokengrab
wails dev
```

浏览器打开 [http://localhost:34115](http://localhost:34115) 调试前端，同时 Wails 自带的窗口也会启动。改 Vue 文件秒级热更，改 Go 文件需重启 `wails dev`。

### 打包成 exe

```powershell
cd D:\fuckwangui\tokengrab
wails build
```

产物在 `build\bin\tokengrab.exe`，大约 **15–20 MB**。免安装、单文件分发。

#### 缩小体积（可选）

加 `-clean` 清旧缓存，加 `-trimpath` 去除路径符号，配合 UPX 可压到 ~6-8MB —— 但 UPX 会大幅提高杀毒软件误报率，不推荐：

```powershell
wails build -clean -trimpath
# 如果要再压（不建议）:
# upx --best build\bin\tokengrab.exe
```

### 项目结构

```
tokengrab/
├── PROJECT.md              # 项目书（含原理分析、风险、技术选型）
├── README.md               # 本文档
├── go.mod / go.sum
├── wails.json              # Wails 配置
├── main.go                 # Wails 入口 + 窗口选项
├── app.go                  # App 结构体，方法暴露给前端
├── internal/
│   ├── ca/                 # 临时 CA 生成 + 安装 + 卸载
│   ├── proxy/              # HTTPS MITM (goproxy 封装)
│   ├── sysproxy/           # Windows 系统代理读/写/恢复
│   ├── jwt/                # JWT 不验签解析
│   ├── clipboard/          # 剪贴板包装
│   └── lock/               # 单例文件锁
├── frontend/
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── index.html
│   └── src/
│       ├── main.ts
│       ├── App.vue         # 三态 UI
│       ├── wails.d.ts      # Wails-injected globals 类型声明
│       └── style.css       # Tailwind + 字体
└── build/
    └── windows/
        └── info.json       # exe 元数据
```

### 调试

- 后端日志：跑 `wails dev` 时直接在终端输出
- 前端日志：F12 打开 DevTools (Wails 默认带)
- IPC 事件：浏览器 console `window.runtime.EventsEmit('progress', {...})` 模拟

### 改目标域名 / 端口

`app.go` 顶部：

```go
const (
    proxyAddr      = "127.0.0.1:8888"
    targetDomain   = "xhbcs.henau.edu.cn"
    captureTimeout = 60 * time.Second
)
```

### 已知限制

- 仅 Windows（用了 certutil + 注册表 + WinINet API）
- 一次只能抓一个 token；抓到后自动停
- 60 秒超时；超时显示提示，需点「重试」
- 如果目标域启用 Certificate Pinning，MITM 失败 —— 学校系统通常没做

—

更详细的逆向分析、风险说明、工时估计见 [PROJECT.md](./PROJECT.md)。
