package web

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	apiclient "wangui/internal/api"
	"wangui/internal/store"
)

var proxyIPCheckEndpoints = []string{
	"https://api.ipify.org",
	"https://icanhazip.com",
}

func schoolAPIClientForUser(u *store.User) (*apiclient.Client, error) {
	return apiclient.NewWithProxy(u.Token, proxyConfigForUser(u))
}

func proxyConfigForUser(u *store.User) apiclient.ProxyConfig {
	return apiclient.ProxyConfig{
		Enabled:  u.ProxyEnabled,
		Scheme:   u.ProxyScheme,
		Host:     u.ProxyHost,
		Port:     u.ProxyPort,
		Username: u.ProxyUsername,
		Password: u.ProxyPassword,
	}
}

func defaultStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func proxyTestDTO(u *store.User, elapsed time.Duration, rules int, err error) map[string]any {
	cfg := proxyConfigForUser(u)
	out := map[string]any{
		"ok":        err == nil,
		"enabled":   cfg.Enabled,
		"outbound":  cfg.OutboundLabel(),
		"elapsedMs": elapsed.Milliseconds(),
		"endpoint":  "available-rules",
	}
	if err != nil {
		out["schoolStatus"] = "ERROR"
		out["schoolMessage"] = err.Error()
		return out
	}
	out["rules"] = rules
	out["schoolStatus"] = "SUCCESS"
	out["schoolMessage"] = "操作成功"
	return out
}

func proxyIPDTO(u *store.User, elapsed time.Duration, ip, endpoint string, err error) map[string]any {
	cfg := proxyConfigForUser(u)
	out := map[string]any{
		"ok":        err == nil,
		"enabled":   cfg.Enabled,
		"outbound":  cfg.OutboundLabel(),
		"elapsedMs": elapsed.Milliseconds(),
		"ip":        ip,
		"endpoint":  endpoint,
	}
	if err != nil {
		out["message"] = err.Error()
	} else {
		out["message"] = "探测成功"
	}
	return out
}

func detectOutboundIP(ctx context.Context, client *http.Client) (string, string, error) {
	var lastErr error
	for _, endpoint := range proxyIPCheckEndpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 128))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("%s HTTP %d", endpoint, resp.StatusCode)
			continue
		}
		ip := strings.TrimSpace(string(raw))
		if net.ParseIP(ip) == nil {
			lastErr = fmt.Errorf("%s 返回的 IP 格式异常", endpoint)
			continue
		}
		return ip, endpoint, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的 IP 探测端点")
	}
	return "", "", lastErr
}
