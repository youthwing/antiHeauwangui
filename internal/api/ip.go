package api

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

var outboundIPCheckEndpoints = []string{
	"https://api.ipify.org",
	"https://icanhazip.com",
}

// DetectOutboundIP returns the public IP observed by a simple external probe.
// Pass the same HTTP client used for school API calls when proxy routing
// needs to be verified.
func DetectOutboundIP(ctx context.Context, client *http.Client) (string, string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	var lastErr error
	for _, endpoint := range outboundIPCheckEndpoints {
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
