package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ServerChanClient pushes a notification through Server酱 (方糖). One client
// is bound to one user's SendKey — admin and per-user clients are separate
// instances. The classic /sctapi.ftqq.com/<KEY>.send endpoint accepts both
// SCT-prefixed and SCU-prefixed keys; Server酱³ users using sctp-prefixed
// keys are routed to push.ft07.com.
type ServerChanClient struct {
	SendKey string
	HTTP    *http.Client
}

// NewServerChan returns a client with a sane HTTP timeout.
func NewServerChan(sendKey string) *ServerChanClient {
	return &ServerChanClient{
		SendKey: sendKey,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Send delivers a single push. `title` shows up as the wechat title;
// `desp` is the body in markdown (Server酱 renders it). Returns nil on a
// 2xx response with `data.errno == 0` (or no errno field).
func (c *ServerChanClient) Send(ctx context.Context, title, desp string) error {
	if c == nil || strings.TrimSpace(c.SendKey) == "" {
		return errors.New("server酱 SendKey 未配置")
	}
	endpoint := endpointFor(c.SendKey)

	form := url.Values{}
	form.Set("title", title)
	form.Set("desp", desp)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("server酱 HTTP %d: %s", res.StatusCode, snippet(body))
	}
	// Body is JSON like {"code":0,"message":"","data":{...}}.
	// Sct³ uses {"code":0,"message":"success","data":{"pushid":"...","readkey":"...","error":"SUCCESS","errno":0}}.
	// Detect failures: any non-zero code or top-level message saying "error".
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Errno int    `json:"errno"`
			Error string `json:"error"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err == nil {
		if env.Code != 0 {
			return fmt.Errorf("server酱 code=%d: %s", env.Code, env.Message)
		}
		if env.Data.Errno != 0 {
			return fmt.Errorf("server酱 errno=%d: %s", env.Data.Errno, env.Data.Error)
		}
	}
	return nil
}

// endpointFor picks the right HTTP target for the given key prefix.
// Classic Server酱 keys start with "SCT" (free) or "SCU" (paid) and use
// sctapi.ftqq.com. Server酱³ keys start with "sctp" and use push.ft07.com.
// Anything else falls back to the classic endpoint, which Server酱 itself
// also accepts for SCT/SCU.
func endpointFor(key string) string {
	k := strings.TrimSpace(key)
	if strings.HasPrefix(k, "sctp") {
		return "https://3.push.ft07.com/send/" + k + ".send"
	}
	return "https://sctapi.ftqq.com/" + k + ".send"
}

func snippet(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
