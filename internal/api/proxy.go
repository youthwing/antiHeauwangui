package api

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	return &http.Client{Timeout: defaultTimeout, Transport: t}, nil
}

// NormalizeProxyConfig trims and validates a proxy config.
func NormalizeProxyConfig(cfg ProxyConfig) (ProxyConfig, error) {
	cfg.Scheme = strings.ToLower(strings.TrimSpace(cfg.Scheme))
	if cfg.Scheme == "" {
		cfg.Scheme = "socks5"
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Username = strings.TrimSpace(cfg.Username)
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
	return scheme + "://" + net.JoinHostPort(host, strconv.Itoa(cfg.Port))
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
