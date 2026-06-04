package scheduler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"wangui/internal/api"
	"wangui/internal/store"
)

type signDebugSnapshot struct {
	data map[string]any
}

func newSignDebugSnapshot(u *store.User, cfg api.ProxyConfig) *signDebugSnapshot {
	now := time.Now()
	normalized, cfgErr := api.NormalizeProxyConfig(cfg)
	if cfgErr != nil {
		normalized = cfg
		normalized.Scheme = strings.ToLower(strings.TrimSpace(normalized.Scheme))
		normalized.Host = strings.TrimSpace(normalized.Host)
		normalized.Username = strings.TrimSpace(normalized.Username)
	}
	if normalized.Scheme == "" {
		normalized.Scheme = "socks5"
	}
	remaining := time.Until(u.TokenExp)
	if remaining < 0 {
		remaining = 0
	}
	d := &signDebugSnapshot{data: map[string]any{
		"generatedAt": now.Format(time.RFC3339),
		"school": map[string]any{
			"baseURL": api.BaseURL,
			"ruleId":  DefaultRuleID,
		},
		"token": map[string]any{
			"expiresAt":    u.TokenExp.Unix(),
			"validUntil":   u.TokenExp.Format(time.RFC3339),
			"remainingSec": int64(remaining.Seconds()),
			"isValid":      now.Before(u.TokenExp),
			"authorization": func() string {
				if u.Token == "" {
					return ""
				}
				return "Bearer " + maskSecret(u.Token)
			}(),
		},
		"proxy": map[string]any{
			"enabled":     normalized.Enabled,
			"scheme":      normalized.Scheme,
			"host":        normalized.Host,
			"port":        normalized.Port,
			"outbound":    normalized.OutboundLabel(),
			"node":        normalized.Node,
			"usernameSet": normalized.Username != "",
			"passwordSet": normalized.Password != "",
		},
		"userConfig": map[string]any{
			"userId":            u.UserID,
			"userName":          u.UserName,
			"userNumber":        u.UserNumber,
			"autoSign":          u.AutoSign,
			"isDisabled":        u.IsDisabled,
			"isGuest":           u.IsGuest,
			"guestLabel":        u.GuestLabel,
			"dormId":            u.DormID,
			"latitude":          u.Lat,
			"longitude":         u.Lng,
			"locationAddress":   u.Address,
			"city":              u.City,
			"road":              u.Road,
			"poi":               u.Poi,
			"sendAddressFields": u.SendAddressFields,
			"deviceModel":       nonEmpty(u.DeviceModel, "iPhone"),
			"deviceSystem":      nonEmpty(u.DeviceSystem, "iOS"),
			"triggerMinute":     u.TriggerMinute,
			"jitterSec":         u.JitterSec,
			"retryCount":        u.RetryCount,
			"retryGapMin":       u.RetryGapMin,
			"signDays":          u.SignDays,
			"signDates":         decodeStringList(u.SignDates),
			"skipDates":         decodeStringList(u.SkipDates),
		},
	}}
	if cfgErr != nil {
		d.section("proxy")["configError"] = cfgErr.Error()
	}
	return d
}

func (d *signDebugSnapshot) section(name string) map[string]any {
	sec, ok := d.data[name].(map[string]any)
	if !ok {
		sec = map[string]any{}
		d.data[name] = sec
	}
	return sec
}

func (d *signDebugSnapshot) setStatusRequest(c *api.Client) {
	school := d.section("school")
	school["statusRequest"] = map[string]any{
		"method":  http.MethodGet,
		"path":    "/checkin/status",
		"url":     c.BaseURL + "/checkin/status?ruleId=" + strconv.Itoa(DefaultRuleID),
		"query":   map[string]any{"ruleId": DefaultRuleID},
		"headers": schoolHeaders(c, false),
	}
}

func (d *signDebugSnapshot) setStatusResponse(st *api.Status, elapsed time.Duration, err error) {
	resp := map[string]any{
		"elapsedMs": elapsed.Milliseconds(),
	}
	if err != nil {
		resp["error"] = err.Error()
	}
	if st != nil {
		resp["canCheckin"] = st.CanCheckin
		resp["hasCheckedIn"] = st.HasCheckedIn
		resp["isExempt"] = st.IsExempt
		resp["isBoarding"] = st.IsBoarding
		resp["message"] = st.Message
		resp["exemptReason"] = st.ExemptReason
		resp["minutesRemaining"] = st.MinutesRemaining
		resp["currentRule"] = st.CurrentRule
		resp["todayRecord"] = rawJSONValue(st.TodayRecord)
	}
	d.section("school")["statusResponse"] = resp
}

func (d *signDebugSnapshot) setSignRequest(c *api.Client, req api.SignRequest) {
	d.section("school")["signRequest"] = map[string]any{
		"method":  http.MethodPost,
		"path":    "/checkin",
		"url":     c.BaseURL + "/checkin",
		"headers": schoolHeaders(c, true),
		"body":    req,
	}
}

func (d *signDebugSnapshot) setSignResponse(data json.RawMessage, elapsed time.Duration, err error) {
	resp := map[string]any{
		"elapsedMs": elapsed.Milliseconds(),
	}
	if err != nil {
		resp["error"] = err.Error()
	}
	if len(data) > 0 {
		resp["data"] = rawJSONValue(data)
	}
	d.section("school")["signResponse"] = resp
}

func (d *signDebugSnapshot) detectOutboundIP(ctx context.Context, client *http.Client) {
	ipCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	start := time.Now()
	ip, endpoint, err := api.DetectOutboundIP(ipCtx, client)
	network := d.section("network")
	network["checkedAt"] = time.Now().Format(time.RFC3339)
	network["elapsedMs"] = time.Since(start).Milliseconds()
	network["ipEndpoint"] = endpoint
	network["outboundIP"] = ip
	if err != nil {
		network["ipError"] = err.Error()
	}
}

func (d *signDebugSnapshot) finish(status, message string) SignResult {
	result := d.section("result")
	result["status"] = status
	result["message"] = message
	result["finishedAt"] = time.Now().Format(time.RFC3339)
	return SignResult{
		Status:       status,
		Message:      message,
		RequestDebug: d.JSON(),
	}
}

func (d *signDebugSnapshot) JSON() string {
	raw, err := json.Marshal(d.data)
	if err != nil {
		return ""
	}
	return string(raw)
}

func schoolHeaders(c *api.Client, hasBody bool) map[string]string {
	headers := map[string]string{
		"User-Agent":      c.UA,
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "zh-CN,zh;q=0.9",
		"Origin":          "https://xhbcs.henau.edu.cn",
		"Referer":         "https://xhbcs.henau.edu.cn/",
	}
	if c.Token != "" {
		headers["Authorization"] = "Bearer " + maskSecret(c.Token)
	}
	if hasBody {
		headers["Content-Type"] = "application/json"
	}
	return headers
}

func rawJSONValue(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}

func decodeStringList(raw string) any {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var xs []string
	if err := json.Unmarshal([]byte(raw), &xs); err != nil {
		return raw
	}
	return xs
}

func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 10 {
		return strings.Repeat("*", len(s))
	}
	return s[:6] + "..." + s[len(s)-4:]
}
