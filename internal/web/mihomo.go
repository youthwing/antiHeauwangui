package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	mihomoDefaultBaseURL = "http://mihomo:9090"
	mihomoDefaultGroup   = "Proxies"
	mihomoDelayTestURL   = "https://xhbcs.henau.edu.cn/api/checkin/available-rules"
)

type mihomoGroup struct {
	Name string   `json:"name"`
	Now  string   `json:"now"`
	All  []string `json:"all"`
	Type string   `json:"type"`
}

type mihomoDelay struct {
	Name  string `json:"name"`
	Delay int    `json:"delay"`
}

func (h *handlers) proxyNodes(w http.ResponseWriter, r *http.Request) {
	group, err := h.mihomoGroup(r.Context(), mihomoDefaultGroup)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"available": false,
			"group":     mihomoDefaultGroup,
			"message":   err.Error(),
			"nodes":     []any{},
			"shared":    true,
		})
		return
	}
	writeJSON(w, http.StatusOK, mihomoNodesDTO(group, "", nil))
}

func (h *handlers) selectProxyNode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeErr(w, http.StatusBadRequest, "节点不能为空")
		return
	}
	group, err := h.mihomoGroup(r.Context(), mihomoDefaultGroup)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "mihomo 不可用: "+err.Error())
		return
	}
	if !containsString(group.All, name) {
		writeErr(w, http.StatusBadRequest, "节点不存在")
		return
	}
	if err := h.mihomoSelect(r.Context(), mihomoDefaultGroup, name); err != nil {
		writeErr(w, http.StatusBadGateway, "切换失败: "+err.Error())
		return
	}
	updated, err := h.mihomoGroup(r.Context(), mihomoDefaultGroup)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "读取节点失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mihomoNodesDTO(updated, name, nil))
}

func (h *handlers) autoSelectProxyNode(w http.ResponseWriter, r *http.Request) {
	group, err := h.mihomoGroup(r.Context(), mihomoDefaultGroup)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "mihomo 不可用: "+err.Error())
		return
	}
	candidates := mihomoCandidateNodes(group.All)
	if len(candidates) == 0 {
		writeErr(w, http.StatusBadGateway, "没有可测速的节点")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
	defer cancel()
	delays := h.mihomoDelays(ctx, candidates)
	if len(delays) == 0 {
		writeErr(w, http.StatusBadGateway, "没有测到可用节点")
		return
	}
	sort.Slice(delays, func(i, j int) bool { return delays[i].Delay < delays[j].Delay })
	pick := delays[0].Name
	if err := h.mihomoSelect(r.Context(), mihomoDefaultGroup, pick); err != nil {
		writeErr(w, http.StatusBadGateway, "切换失败: "+err.Error())
		return
	}
	updated, err := h.mihomoGroup(r.Context(), mihomoDefaultGroup)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "读取节点失败: "+err.Error())
		return
	}
	if len(delays) > 12 {
		delays = delays[:12]
	}
	writeJSON(w, http.StatusOK, mihomoNodesDTO(updated, pick, delays))
}

func (h *handlers) mihomoGroup(ctx context.Context, group string) (*mihomoGroup, error) {
	var out mihomoGroup
	if err := h.mihomoDo(ctx, http.MethodGet, "/proxies/"+url.PathEscape(group), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (h *handlers) mihomoSelect(ctx context.Context, group, name string) error {
	return h.mihomoDo(ctx, http.MethodPut, "/proxies/"+url.PathEscape(group), map[string]string{"name": name}, nil)
}

func (h *handlers) mihomoDelay(ctx context.Context, name string) (int, error) {
	var out struct {
		Delay int `json:"delay"`
	}
	path := "/proxies/" + url.PathEscape(name) + "/delay?timeout=5000&url=" + url.QueryEscape(mihomoDelayTestURL)
	if err := h.mihomoDo(ctx, http.MethodGet, path, nil, &out); err != nil {
		return 0, err
	}
	if out.Delay <= 0 {
		return 0, fmt.Errorf("节点无延迟结果")
	}
	return out.Delay, nil
}

func (h *handlers) mihomoDelays(ctx context.Context, names []string) []mihomoDelay {
	sem := make(chan struct{}, 8)
	out := make(chan mihomoDelay, len(names))
	var wg sync.WaitGroup
	for _, name := range names {
		name := name
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			delay, err := h.mihomoDelay(ctx, name)
			if err == nil {
				out <- mihomoDelay{Name: name, Delay: delay}
			}
		}()
	}
	wg.Wait()
	close(out)
	delays := make([]mihomoDelay, 0)
	for d := range out {
		delays = append(delays, d)
	}
	return delays
}

func (h *handlers) mihomoDo(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, mihomoBaseURL()+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if secret := strings.TrimSpace(os.Getenv("WANGUI_MIHOMO_SECRET")); secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("mihomo %s %s: HTTP %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func mihomoBaseURL() string {
	v := strings.TrimSpace(os.Getenv("WANGUI_MIHOMO_API"))
	if v == "" {
		v = mihomoDefaultBaseURL
	}
	return strings.TrimRight(v, "/")
}

func mihomoNodesDTO(group *mihomoGroup, picked string, delays []mihomoDelay) map[string]any {
	delayByName := map[string]int{}
	for _, d := range delays {
		delayByName[d.Name] = d.Delay
	}
	nodes := make([]map[string]any, 0, len(group.All))
	for _, name := range group.All {
		n := map[string]any{
			"name":    name,
			"current": name == group.Now,
		}
		if d, ok := delayByName[name]; ok {
			n["delayMs"] = d
		}
		nodes = append(nodes, n)
	}
	tested := make([]map[string]any, 0, len(delays))
	for _, d := range delays {
		tested = append(tested, map[string]any{
			"name":    d.Name,
			"delayMs": d.Delay,
			"current": d.Name == group.Now,
		})
	}
	return map[string]any{
		"available": true,
		"group":     group.Name,
		"current":   group.Now,
		"picked":    picked,
		"nodes":     nodes,
		"tested":    tested,
		"shared":    true,
	}
}

func mihomoCandidateNodes(nodes []string) []string {
	out := make([]string, 0, len(nodes))
	for _, name := range nodes {
		if strings.TrimSpace(name) == "" {
			continue
		}
		switch name {
		case "DIRECT", "REJECT", "PASS", "COMPATIBLE", "HK", "JP", "SG", "TW", "US":
			continue
		}
		if strings.HasPrefix(name, "Traffic:") || strings.HasPrefix(name, "Expire:") {
			continue
		}
		out = append(out, name)
	}
	if len(out) > 48 {
		return out[:48]
	}
	return out
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
