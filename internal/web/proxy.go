package web

import (
	"net/http"
	"time"

	apiclient "wangui/internal/api"
	"wangui/internal/store"
)

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
		Node:     u.ProxyNode,
	}
}

func (h *handlers) proxyConfigForUser(r *http.Request, u *store.User) apiclient.ProxyConfig {
	cfg := proxyConfigForUser(u)
	if !cfg.UsesBuiltinMihomoProxy() {
		return cfg
	}
	enabled, err := h.store.GetMihomoBuiltinProxyEnabled(r.Context())
	if err != nil {
		h.log.Warn("read mihomo builtin proxy switch", "err", err.Error())
		return cfg
	}
	if !enabled {
		cfg.Enabled = false
	}
	return cfg
}

func (h *handlers) schoolAPIClientForUser(r *http.Request, u *store.User) (*apiclient.Client, error) {
	return apiclient.NewWithProxy(u.Token, h.proxyConfigForUser(r, u))
}

func defaultStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func proxyTestDTO(u *store.User, elapsed time.Duration, rules int, err error) map[string]any {
	cfg := proxyConfigForUser(u)
	return proxyTestForConfigDTO(cfg, elapsed, rules, err)
}

func proxyTestForConfigDTO(cfg apiclient.ProxyConfig, elapsed time.Duration, rules int, err error) map[string]any {
	cfg = normalizedProxyConfigForDisplay(cfg)
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
	return proxyIPForConfigDTO(cfg, elapsed, ip, endpoint, err)
}

func adminMihomoProxyConfig(enabled bool, node string) apiclient.ProxyConfig {
	return apiclient.BuiltinMihomoProxyConfig(enabled, node)
}

func proxyIPForConfigDTO(cfg apiclient.ProxyConfig, elapsed time.Duration, ip, endpoint string, err error) map[string]any {
	cfg = normalizedProxyConfigForDisplay(cfg)
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

func normalizedProxyConfigForDisplay(cfg apiclient.ProxyConfig) apiclient.ProxyConfig {
	normalized, err := apiclient.NormalizeProxyConfig(cfg)
	if err != nil {
		return cfg
	}
	return normalized
}
