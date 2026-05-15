// Package schoolapi calls a tiny subset of the school API directly (i.e. NOT
// through the MITM proxy we just installed), to enrich the captured token with
// user info — name, number, faculty, class, avatar.
package schoolapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	apiBase = "https://xhbcs.henau.edu.cn/api"
	// Match the User-Agent the school front-end sends from inside WeChat,
	// in case the backend filters by UA.
	userAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) " +
		"AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 " +
		"MicroMessenger/8.0.40(0x18002834) NetType/WIFI Language/zh_CN"
)

// User mirrors the school /auth/user payload (only the fields we display).
type User struct {
	UserName      string `json:"userName"`
	UserNumber    string `json:"userNumber"`
	UserSection   string `json:"userSection"`
	UserClass     string `json:"userClass"`
	UserAvatarURL string `json:"userAvatarUrl"`
}

type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// directClient deliberately bypasses the system proxy — we just set it to
// ourselves, so using it would route our own outgoing call back through our
// MITM listener and (worse) make us trust our own forged leaf for the school
// domain. Proxy: nil also short-circuits HTTP_PROXY / HTTPS_PROXY env vars.
var directClient = &http.Client{
	Transport: &http.Transport{
		Proxy: nil,
	},
	Timeout: 10 * time.Second,
}

// GetUser fetches the user profile via /auth/user. Returns an error rather
// than panicking; callers should fall back gracefully if it fails.
func GetUser(ctx context.Context, token string) (*User, error) {
	if token == "" {
		return nil, errors.New("token is empty")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", apiBase+"/auth/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Origin", "https://xhbcs.henau.edu.cn")
	req.Header.Set("Referer", "https://xhbcs.henau.edu.cn/")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := directClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode envelope (status=%d): %w", resp.StatusCode, err)
	}
	if env.Code != 200 {
		return nil, fmt.Errorf("/auth/user: code=%d msg=%q", env.Code, env.Message)
	}
	var u User
	if err := json.Unmarshal(env.Data, &u); err != nil {
		return nil, fmt.Errorf("decode user: %w", err)
	}
	return &u, nil
}
