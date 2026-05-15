package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"wangui-tokengrab/internal/ca"
	"wangui-tokengrab/internal/clipboard"
	"wangui-tokengrab/internal/jwt"
	"wangui-tokengrab/internal/proxy"
	"wangui-tokengrab/internal/schoolapi"
	"wangui-tokengrab/internal/sysproxy"
)

const (
	proxyAddr      = "127.0.0.1:8888"
	targetDomain   = "xhbcs.henau.edu.cn"
	captureTimeout = 60 * time.Second
)

// Phase is the current state of the capture lifecycle.
type Phase string

const (
	PhaseIdle      Phase = "idle"
	PhaseCapturing Phase = "capturing"
	PhaseCaptured  Phase = "captured"
	PhaseError     Phase = "error"
)

// ProgressEvent is emitted on the "progress" channel to drive the UI through
// the capture steps.
type ProgressEvent struct {
	Phase   Phase    `json:"phase"`
	Step    string   `json:"step"`
	Done    []string `json:"done"`
	Message string   `json:"message,omitempty"`
}

// CapturedEvent is emitted on "captured" when a token has been parsed.
type CapturedEvent struct {
	Token        string `json:"token"`
	UserID       string `json:"userId"`
	ExpiresAt    int64  `json:"expiresAt"`
	RemainingSec int64  `json:"remainingSec"`
	ClipboardOK  bool   `json:"clipboardOk"`

	UserName      string `json:"userName"`
	UserNumber    string `json:"userNumber"`
	UserSection   string `json:"userSection"`
	UserClass     string `json:"userClass"`
	UserAvatarURL string `json:"userAvatarUrl"`
}

// App bridges the Go backend and the Wails frontend.
type App struct {
	ctx     context.Context
	dataDir string

	mu           sync.Mutex
	phase        Phase
	caObj        *ca.CA            // persistent, loaded once at startup
	caInst       *ca.InstallState  // non-nil once we've confirmed (or just done) install
	caInstalled  bool              // mirror of caInst != nil
	sysSave      *sysproxy.State   // saved at Start, restored at Cleanup
	prx          *proxy.Server
	cancel       context.CancelFunc
}

func NewApp() *App {
	return &App{phase: PhaseIdle}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	dir, err := resolveDataDir()
	if err != nil {
		// We can still run — install will just fail with a clearer error.
		a.dataDir = ""
	} else {
		a.dataDir = dir
	}

	// Load (or generate) the persistent CA. On first run this creates a
	// fresh one; on subsequent runs we reuse the same one. Either way the
	// thumbprint is the source of truth for IsInstalled.
	if a.dataDir != "" {
		caObj, _, err := ca.LoadOrGenerate(a.dataDir)
		if err == nil {
			a.caObj = caObj
			thumb := thumbprintOf(caObj.DER)
			if ok, _ := ca.IsInstalled(thumb); ok {
				a.caInst = &ca.InstallState{Thumbprint: thumb}
				a.caInstalled = true
			}
		}
	}

	go a.checkResidual()
}

func (a *App) shutdown(_ context.Context) {
	// Don't uninstall the persistent CA on exit — that's the whole point of
	// persisting. Only clean up the proxy / sysproxy that might be live.
	_ = a.cleanupCapture()
}

func (a *App) emit(evt string, payload any) {
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, evt, payload)
	}
}

// ============================================================================
// methods exposed to the frontend
// ============================================================================

func (a *App) GetPhase() Phase {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.phase
}

// CAInstalled tells the frontend whether the persistent CA is currently
// trusted by Windows. Used to decide if the first run notice should appear.
func (a *App) CAInstalled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.caInstalled
}

// CheckResidual asks whether a previous crashed session left the system
// proxy pointing at us. (The CA itself is intentionally persistent now and
// is not treated as residue.)
func (a *App) CheckResidual() bool {
	return sysproxy.IsArmedToLocalhost()
}

func (a *App) CleanResidual() error {
	if sysproxy.IsArmedToLocalhost() {
		return sysproxy.Restore(&sysproxy.State{Enable: 0, Server: ""})
	}
	return nil
}

func (a *App) Start() error {
	a.mu.Lock()
	if a.phase == PhaseCapturing {
		a.mu.Unlock()
		return nil
	}
	a.phase = PhaseCapturing
	a.mu.Unlock()

	go a.run()
	return nil
}

func (a *App) Cancel() error {
	return a.cleanupCapture()
}

// UninstallPersistentCA removes the CA from Windows' trust store (this WILL
// trigger the deletion-confirmation dialog) and erases the local key files.
// Next launch will generate a fresh CA and prompt again.
func (a *App) UninstallPersistentCA() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.caInst != nil {
		if err := ca.Uninstall(a.caInst); err != nil {
			return fmt.Errorf("卸载根证书失败: %w", err)
		}
	}
	if a.dataDir != "" {
		_ = ca.Erase(a.dataDir)
	}
	a.caInst = nil
	a.caInstalled = false
	a.caObj = nil
	a.emit("ca-installed", false)
	return nil
}

func (a *App) Reset() {
	a.mu.Lock()
	a.phase = PhaseIdle
	a.mu.Unlock()
	a.emit("progress", ProgressEvent{Phase: PhaseIdle})
}

func (a *App) SetClipboard(s string) error {
	return clipboard.Set(s)
}

// OpenWanguiActivate launches the default browser at the user's wangui login
// page with whatever the user has captured pre-filled via URL fragment.
// Format: #activate=<token>&code=<invite>. invite may be empty (token-refresh
// flow, no code needed).
func (a *App) OpenWanguiActivate(baseURL, token, invite string) error {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		return fmt.Errorf("无效的 URL，需要以 http:// 或 https:// 开头")
	}
	full := baseURL + "/login"
	frag := url.Values{}
	if token = strings.TrimSpace(token); token != "" {
		frag.Set("activate", token)
	}
	if invite = strings.TrimSpace(invite); invite != "" {
		frag.Set("code", invite)
	}
	if encoded := frag.Encode(); encoded != "" {
		full += "#" + encoded
	}
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", full)
	return cmd.Start()
}

// ============================================================================
// capture goroutine
// ============================================================================

func (a *App) run() {
	done := []string{}
	emitStep := func(label string) {
		a.emit("progress", ProgressEvent{
			Phase: PhaseCapturing,
			Step:  label,
			Done:  append([]string{}, done...),
		})
	}
	finishStep := func(label string) {
		done = append(done, label)
	}
	abort := func(label string, err error) {
		a.mu.Lock()
		a.phase = PhaseError
		_ = a.cleanupCaptureLocked()
		a.mu.Unlock()
		a.emit("progress", ProgressEvent{
			Phase:   PhaseError,
			Step:    label,
			Done:    done,
			Message: err.Error(),
		})
	}

	// 1. CA — either reuse the persisted one or (rare) regenerate.
	emitStep("加载临时证书")
	a.mu.Lock()
	if a.caObj == nil {
		if a.dataDir == "" {
			a.mu.Unlock()
			abort("加载临时证书", fmt.Errorf("无法定位数据目录"))
			return
		}
		c, _, err := ca.LoadOrGenerate(a.dataDir)
		if err != nil {
			a.mu.Unlock()
			abort("加载临时证书", err)
			return
		}
		a.caObj = c
	}
	caObj := a.caObj
	caInstalled := a.caInstalled
	a.mu.Unlock()
	finishStep("加载临时证书")

	// 2. Install — first run only. Subsequent runs skip and never prompt.
	emitStep("信任 CA")
	if !caInstalled {
		inst, err := caObj.Install()
		if err != nil {
			abort("信任 CA", err)
			return
		}
		a.mu.Lock()
		a.caInst = inst
		a.caInstalled = true
		a.mu.Unlock()
		a.emit("ca-installed", true)
	}
	finishStep("信任 CA")

	// 3. System proxy.
	emitStep("设置系统代理")
	save, err := sysproxy.Snapshot()
	if err != nil {
		abort("设置系统代理", err)
		return
	}
	a.mu.Lock()
	a.sysSave = save
	a.mu.Unlock()
	if err := sysproxy.Set(proxyAddr); err != nil {
		abort("设置系统代理", err)
		return
	}
	finishStep("设置系统代理")

	// 4. MITM proxy + wait.
	emitStep("等待签到入口请求")
	tokenCh := make(chan string, 1)
	prx := proxy.New(proxyAddr, caObj, targetDomain, func(h proxy.Hit) {
		select {
		case tokenCh <- h.Token:
		default:
		}
	})
	if err := prx.Start(); err != nil {
		abort("启动代理", err)
		return
	}
	a.mu.Lock()
	a.prx = prx
	a.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), captureTimeout)
	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()
	defer cancel()

	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			abort("等待签到入口请求",
				fmt.Errorf("60 秒内没收到学校 API 请求；请确认在微信里打开了签到入口并刷新页面"))
			return
		}
		// Cancelled by user via Cancel/Cleanup
		a.emit("progress", ProgressEvent{Phase: PhaseIdle})

	case tok := <-tokenCh:
		claims, err := jwt.Parse(tok)
		if err != nil {
			abort("解析 token", err)
			return
		}
		clipboardOK := clipboard.Set(tok) == nil

		evt := CapturedEvent{
			Token:        tok,
			UserID:       claims.Iss,
			ExpiresAt:    claims.Exp.Unix(),
			RemainingSec: claims.Exp.Unix() - time.Now().Unix(),
			ClipboardOK:  clipboardOK,
		}
		fetchCtx, cancelFetch := context.WithTimeout(context.Background(), 5*time.Second)
		if u, err := schoolapi.GetUser(fetchCtx, tok); err == nil && u != nil {
			evt.UserName = u.UserName
			evt.UserNumber = u.UserNumber
			evt.UserSection = u.UserSection
			evt.UserClass = u.UserClass
			evt.UserAvatarURL = u.UserAvatarURL
		}
		cancelFetch()

		a.mu.Lock()
		a.phase = PhaseCaptured
		a.mu.Unlock()
		a.emit("captured", evt)

		// Teardown the proxy + sysproxy asynchronously — keep the CA
		// installed for next time.
		go func() {
			a.mu.Lock()
			_ = a.cleanupCaptureLocked()
			a.mu.Unlock()
		}()
	}
}

// cleanupCapture stops the proxy and restores the system proxy. It does NOT
// uninstall the CA (that's the whole point of the persistent CA model).
func (a *App) cleanupCapture() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cleanupCaptureLocked()
}

func (a *App) cleanupCaptureLocked() error {
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	if a.prx != nil {
		a.prx.Stop()
		a.prx = nil
	}
	if a.sysSave != nil {
		_ = sysproxy.Restore(a.sysSave)
		a.sysSave = nil
	}
	if a.phase != PhaseCaptured && a.phase != PhaseError {
		a.phase = PhaseIdle
	}
	return nil
}

func (a *App) checkResidual() {
	time.Sleep(500 * time.Millisecond)
	if a.CheckResidual() {
		a.emit("residual", true)
	}
}

// ============================================================================
// helpers
// ============================================================================

func resolveDataDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		// Fall back to user config dir; on Windows that's APPDATA (roaming),
		// which works but is less ideal (we don't want this synced).
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(base, "wangui-tokengrab"), nil
}

func thumbprintOf(der []byte) string {
	h := sha1.Sum(der)
	return strings.ToUpper(hex.EncodeToString(h[:]))
}
