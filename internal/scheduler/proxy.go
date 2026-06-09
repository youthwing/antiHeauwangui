package scheduler

import (
	"context"

	"wangui/internal/api"
	"wangui/internal/store"
)

func schoolAPIClientForUser(u *store.User) (*api.Client, error) {
	return api.NewWithProxy(u.Token, proxyConfigForUser(u))
}

func (m *Multi) schoolAPIClientForUser(ctx context.Context, u *store.User) (*api.Client, error) {
	return api.NewWithProxy(u.Token, m.proxyConfigForUser(ctx, u))
}

func proxyConfigForUser(u *store.User) api.ProxyConfig {
	return api.ProxyConfig{
		Enabled:  u.ProxyEnabled,
		Scheme:   u.ProxyScheme,
		Host:     u.ProxyHost,
		Port:     u.ProxyPort,
		Username: u.ProxyUsername,
		Password: u.ProxyPassword,
		Node:     u.ProxyNode,
	}
}

func (m *Multi) proxyConfigForUser(ctx context.Context, u *store.User) api.ProxyConfig {
	cfg := proxyConfigForUser(u)
	if !cfg.UsesBuiltinMihomoProxy() {
		return cfg
	}
	enabled, err := m.store.GetMihomoBuiltinProxyEnabled(ctx)
	if err != nil {
		m.log.Warn("read mihomo builtin proxy switch", "err", err.Error())
		return cfg
	}
	if !enabled {
		cfg.Enabled = false
	}
	return cfg
}
