package api

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultTimeout = 15 * time.Second

// ProxyConfig describes an optional per-user outbound proxy for school API calls.
type ProxyConfig struct {
	Enabled  bool
	Scheme   string
	Host     string
	Port     int
	Username string
	Password string
	Node     string
}

// NewWithProxy creates a client that routes school API calls through cfg when enabled.
func NewWithProxy(token string, cfg ProxyConfig) (*Client, error) {
	c := New(token)
	if !cfg.Enabled {
		return c, nil
	}
	httpClient, err := HTTPClientForProxy(cfg)
	if err != nil {
		return nil, err
	}
	c.HTTP = httpClient
	return c, nil
}

// HTTPClientForProxy returns a configured HTTP client for a validated proxy config.
func HTTPClientForProxy(cfg ProxyConfig) (*http.Client, error) {
	cfg, err := NormalizeProxyConfig(cfg)
	if err != nil {
		return nil, err
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	switch cfg.Scheme {
	case "http", "https":
		u := &url.URL{
			Scheme: cfg.Scheme,
			Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		}
		if cfg.Username != "" {
			u.User = url.UserPassword(cfg.Username, cfg.Password)
		}
		t.Proxy = http.ProxyURL(u)
	case "socks5":
		t.Proxy = nil
		dialer := &socks5Dialer{
			network:  "tcp",
			addr:     net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
			username: cfg.Username,
			password: cfg.Password,
		}
		t.DialContext = dialer.DialContext
	default:
		return nil, fmt.Errorf("不支持的代理协议: %s", cfg.Scheme)
	}
	var rt http.RoundTripper = t
	if cfg.MihomoNodeEnabled() {
		t.DisableKeepAlives = true
		rt = &mihomoNodeTransport{
			base: rt,
			node: cfg.Node,
		}
	}
	return &http.Client{Timeout: defaultTimeout, Transport: rt}, nil
}

// NormalizeProxyConfig trims and validates a proxy config.
func NormalizeProxyConfig(cfg ProxyConfig) (ProxyConfig, error) {
	cfg.Scheme = strings.ToLower(strings.TrimSpace(cfg.Scheme))
	if cfg.Scheme == "" {
		cfg.Scheme = "socks5"
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.Node = strings.TrimSpace(cfg.Node)
	if !cfg.Enabled {
		return cfg, nil
	}
	switch cfg.Scheme {
	case "socks5", "http", "https":
	default:
		return cfg, errors.New("代理协议仅支持 socks5 / http / https")
	}
	if cfg.Host == "" {
		return cfg, errors.New("代理主机地址不能为空")
	}
	if strings.ContainsAny(cfg.Host, "/?#") {
		return cfg, errors.New("代理主机地址只填写域名或 IP，不要带协议和路径")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return cfg, errors.New("代理端口必须在 1–65535 之间")
	}
	if len([]byte(cfg.Username)) > 255 || len([]byte(cfg.Password)) > 255 {
		return cfg, errors.New("SOCKS5 用户名和密码不能超过 255 字节")
	}
	return cfg, nil
}

// OutboundLabel returns a safe display string without proxy credentials.
func (cfg ProxyConfig) OutboundLabel() string {
	scheme := strings.ToLower(strings.TrimSpace(cfg.Scheme))
	if scheme == "" {
		scheme = "socks5"
	}
	host := strings.TrimSpace(cfg.Host)
	if host == "" || cfg.Port == 0 {
		return "未配置"
	}
	label := scheme + "://" + net.JoinHostPort(host, strconv.Itoa(cfg.Port))
	if node := strings.TrimSpace(cfg.Node); node != "" {
		label += " · " + node
	}
	return label
}

// MihomoNodeEnabled reports whether cfg targets the bundled Mihomo HTTP port
// and carries a per-user node preference. The node is applied immediately
// before each proxied request, so users can keep separate node preferences even
// though Mihomo itself exposes one selector state per group.
func (cfg ProxyConfig) MihomoNodeEnabled() bool {
	if !cfg.Enabled || strings.TrimSpace(cfg.Node) == "" {
		return false
	}
	scheme := strings.ToLower(strings.TrimSpace(cfg.Scheme))
	host := strings.ToLower(strings.TrimSpace(cfg.Host))
	return scheme == "http" && host == "mihomo" && cfg.Port == 7893
}

type mihomoNodeTransport struct {
	base http.RoundTripper
	node string
}

func (t *mihomoNodeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	mihomoSwitchMu.Lock()

	target := strings.TrimSpace(t.node)
	state, err := activateMihomoNode(req.Context(), target)
	if err != nil {
		restoreCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		restoreErr := restoreMihomoState(restoreCtx, target, state)
		cancel()
		mihomoSwitchMu.Unlock()
		if restoreErr != nil {
			return nil, fmt.Errorf("%w; 恢复 Mihomo 状态失败: %v", err, restoreErr)
		}
		return nil, err
	}
	resp, roundTripErr := base.RoundTrip(req)
	if roundTripErr == nil && resp != nil && resp.Body != nil {
		resp.Body = &mihomoRestoreBody{
			ReadCloser: resp.Body,
			target:     target,
			state:      state,
		}
		return resp, nil
	}
	restoreCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	restoreErr := restoreMihomoState(restoreCtx, target, state)
	cancel()
	if restoreErr != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if roundTripErr != nil {
			return nil, fmt.Errorf("%w; 恢复 Mihomo 状态失败: %v", roundTripErr, restoreErr)
		}
		return nil, fmt.Errorf("恢复 Mihomo 状态失败: %w", restoreErr)
	}
	mihomoSwitchMu.Unlock()
	return resp, roundTripErr
}

var mihomoSwitchMu sync.Mutex

type mihomoRestoreBody struct {
	io.ReadCloser
	target string
	state  mihomoState
	once   sync.Once
	err    error
}

func (b *mihomoRestoreBody) Close() error {
	closeErr := b.ReadCloser.Close()
	b.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		b.err = restoreMihomoState(ctx, b.target, b.state)
		cancel()
		mihomoSwitchMu.Unlock()
	})
	if closeErr != nil {
		return closeErr
	}
	return b.err
}

const (
	mihomoDefaultBaseURL = "http://mihomo:9090"
	mihomoDefaultGroup   = "Proxies"
	mihomoGlobalGroup    = "GLOBAL"
)

type mihomoState struct {
	mode      string
	proxies   string
	global    string
	hasGlobal bool
}

func activateMihomoNode(ctx context.Context, target string) (mihomoState, error) {
	state, err := currentMihomoState(ctx)
	if err != nil {
		return state, err
	}
	if !state.hasGlobal {
		return state, errors.New("Mihomo 未暴露 GLOBAL 选择器，无法强制按用户节点出站")
	}
	if state.proxies != target {
		if err := selectMihomoNode(ctx, mihomoDefaultGroup, target); err != nil {
			return state, err
		}
	}
	if state.hasGlobal && state.global != target {
		if err := selectMihomoNode(ctx, mihomoGlobalGroup, target); err != nil {
			return state, err
		}
	}
	if !strings.EqualFold(state.mode, "global") {
		if err := setMihomoMode(ctx, "global"); err != nil {
			return state, err
		}
	}
	return state, nil
}

func currentMihomoState(ctx context.Context) (mihomoState, error) {
	mode, err := currentMihomoMode(ctx)
	if err != nil {
		return mihomoState{}, err
	}
	proxies, err := currentMihomoNode(ctx, mihomoDefaultGroup)
	if err != nil {
		return mihomoState{mode: mode}, err
	}
	state := mihomoState{mode: mode, proxies: proxies}
	if global, err := currentMihomoNode(ctx, mihomoGlobalGroup); err == nil {
		state.global = global
		state.hasGlobal = true
	}
	return state, nil
}

func restoreMihomoState(ctx context.Context, target string, state mihomoState) error {
	var errs []string
	if state.hasGlobal && state.global != "" && state.global != target {
		if current, err := currentMihomoNode(ctx, mihomoGlobalGroup); err != nil {
			errs = append(errs, err.Error())
		} else if current == target {
			if err := selectMihomoNode(ctx, mihomoGlobalGroup, state.global); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}
	if state.proxies != "" && state.proxies != target {
		if current, err := currentMihomoNode(ctx, mihomoDefaultGroup); err != nil {
			errs = append(errs, err.Error())
		} else if current == target {
			if err := selectMihomoNode(ctx, mihomoDefaultGroup, state.proxies); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}
	if state.mode != "" && !strings.EqualFold(state.mode, "global") {
		if current, err := currentMihomoMode(ctx); err != nil {
			errs = append(errs, err.Error())
		} else if strings.EqualFold(current, "global") {
			if err := setMihomoMode(ctx, state.mode); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func currentMihomoMode(ctx context.Context) (string, error) {
	var out struct {
		Mode string `json:"mode"`
	}
	if err := mihomoDo(ctx, http.MethodGet, "/configs", nil, &out); err != nil {
		return "", fmt.Errorf("读取 Mihomo 模式失败: %w", err)
	}
	return strings.TrimSpace(out.Mode), nil
}

func setMihomoMode(ctx context.Context, mode string) error {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return nil
	}
	switch strings.ToLower(mode) {
	case "global":
		mode = "global"
	case "rule":
		mode = "rule"
	case "direct":
		mode = "direct"
	}
	return mihomoDo(ctx, http.MethodPatch, "/configs", map[string]string{"mode": mode}, nil)
}

func currentMihomoNode(ctx context.Context, group string) (string, error) {
	var out struct {
		Now string `json:"now"`
	}
	if err := mihomoDo(ctx, http.MethodGet, "/proxies/"+url.PathEscape(group), nil, &out); err != nil {
		return "", fmt.Errorf("读取 Mihomo 当前节点失败: %w", err)
	}
	return strings.TrimSpace(out.Now), nil
}

func selectMihomoNode(ctx context.Context, group, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	return mihomoDo(ctx, http.MethodPut, "/proxies/"+url.PathEscape(group), map[string]string{"name": name}, nil)
}

func mihomoDo(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, mihomoBaseURL()+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if secret := strings.TrimSpace(os.Getenv("WANGUI_MIHOMO_SECRET")); secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("mihomo %s %s: HTTP %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func mihomoBaseURL() string {
	v := strings.TrimSpace(os.Getenv("WANGUI_MIHOMO_API"))
	if v == "" {
		v = mihomoDefaultBaseURL
	}
	return strings.TrimRight(v, "/")
}

type socks5Dialer struct {
	network  string
	addr     string
	username string
	password string
}

func (d *socks5Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if d.network != "" {
		network = d.network
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, network, d.addr)
	if err != nil {
		return nil, err
	}
	if err := d.handshake(conn, address); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func (d *socks5Dialer) handshake(conn net.Conn, target string) error {
	if deadline, ok := connDeadlineFromTimeout(defaultTimeout); ok {
		_ = conn.SetDeadline(deadline)
		defer conn.SetDeadline(time.Time{})
	}
	methods := []byte{0x00}
	if d.username != "" || d.password != "" {
		methods = []byte{0x00, 0x02}
	}
	if _, err := conn.Write([]byte{0x05, byte(len(methods))}); err != nil {
		return err
	}
	if _, err := conn.Write(methods); err != nil {
		return err
	}
	var methodResp [2]byte
	if _, err := io.ReadFull(conn, methodResp[:]); err != nil {
		return err
	}
	if methodResp[0] != 0x05 {
		return errors.New("SOCKS5 代理响应版本异常")
	}
	switch methodResp[1] {
	case 0x00:
	case 0x02:
		if err := d.authenticate(conn); err != nil {
			return err
		}
	case 0xff:
		return errors.New("SOCKS5 代理不接受当前认证方式")
	default:
		return fmt.Errorf("SOCKS5 代理选择了不支持的认证方式: 0x%02x", methodResp[1])
	}

	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return fmt.Errorf("解析目标地址失败: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("目标端口异常: %s", portStr)
	}
	req := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			req = append(req, 0x01)
			req = append(req, v4...)
		} else {
			req = append(req, 0x04)
			req = append(req, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return errors.New("目标域名过长")
		}
		req = append(req, 0x03, byte(len(host)))
		req = append(req, []byte(host)...)
	}
	var p [2]byte
	binary.BigEndian.PutUint16(p[:], uint16(port))
	req = append(req, p[:]...)
	if _, err := conn.Write(req); err != nil {
		return err
	}
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return err
	}
	if header[0] != 0x05 {
		return errors.New("SOCKS5 连接响应版本异常")
	}
	if header[1] != 0x00 {
		return fmt.Errorf("SOCKS5 CONNECT 失败: %s", socks5ReplyMessage(header[1]))
	}
	if err := discardSocks5Addr(conn, header[3]); err != nil {
		return err
	}
	var bindPort [2]byte
	_, err = io.ReadFull(conn, bindPort[:])
	return err
}

func (d *socks5Dialer) authenticate(conn net.Conn) error {
	if len([]byte(d.username)) > 255 || len([]byte(d.password)) > 255 {
		return errors.New("SOCKS5 用户名和密码不能超过 255 字节")
	}
	req := []byte{0x01, byte(len([]byte(d.username)))}
	req = append(req, []byte(d.username)...)
	req = append(req, byte(len([]byte(d.password))))
	req = append(req, []byte(d.password)...)
	if _, err := conn.Write(req); err != nil {
		return err
	}
	var resp [2]byte
	if _, err := io.ReadFull(conn, resp[:]); err != nil {
		return err
	}
	if resp[0] != 0x01 || resp[1] != 0x00 {
		return errors.New("SOCKS5 用户名或密码认证失败")
	}
	return nil
}

func discardSocks5Addr(r io.Reader, atyp byte) error {
	switch atyp {
	case 0x01:
		_, err := io.CopyN(io.Discard, r, 4)
		return err
	case 0x03:
		var l [1]byte
		if _, err := io.ReadFull(r, l[:]); err != nil {
			return err
		}
		_, err := io.CopyN(io.Discard, r, int64(l[0]))
		return err
	case 0x04:
		_, err := io.CopyN(io.Discard, r, 16)
		return err
	default:
		return fmt.Errorf("SOCKS5 绑定地址类型异常: 0x%02x", atyp)
	}
}

func socks5ReplyMessage(code byte) string {
	switch code {
	case 0x01:
		return "代理服务器一般性失败"
	case 0x02:
		return "规则不允许连接"
	case 0x03:
		return "网络不可达"
	case 0x04:
		return "目标主机不可达"
	case 0x05:
		return "连接被拒绝"
	case 0x06:
		return "TTL 过期"
	case 0x07:
		return "不支持的命令"
	case 0x08:
		return "不支持的地址类型"
	default:
		return fmt.Sprintf("未知错误 0x%02x", code)
	}
}

func connDeadlineFromTimeout(timeout time.Duration) (time.Time, bool) {
	if timeout <= 0 {
		return time.Time{}, false
	}
	return time.Now().Add(timeout), true
}
