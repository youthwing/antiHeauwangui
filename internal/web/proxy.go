package web

import (
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
