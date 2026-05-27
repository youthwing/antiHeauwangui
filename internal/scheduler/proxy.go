package scheduler

import (
	"wangui/internal/api"
	"wangui/internal/store"
)

func schoolAPIClientForUser(u *store.User) (*api.Client, error) {
	return api.NewWithProxy(u.Token, proxyConfigForUser(u))
}

func proxyConfigForUser(u *store.User) api.ProxyConfig {
	return api.ProxyConfig{
		Enabled:  u.ProxyEnabled,
		Scheme:   u.ProxyScheme,
		Host:     u.ProxyHost,
		Port:     u.ProxyPort,
		Username: u.ProxyUsername,
		Password: u.ProxyPassword,
	}
}
