package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"wangui/internal/events"
	"wangui/internal/store"
)

// validLevels are the allowed announcement style buckets. Anything else
// gets coerced to "info" on save so the frontend's CSS map can't miss.
var validLevels = map[string]bool{
	"info":     true,
	"success":  true,
	"warning":  true,
	"critical": true,
}

// GET /api/v1/announcements — user-facing list of currently-active
// announcements (newest first). Skips expired entries.
func (h *handlers) listAnnouncements(w http.ResponseWriter, r *http.Request) {
	list, err := h.store.ListActiveAnnouncements(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, a := range list {
		out = append(out, announcementDTO(&a))
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /api/v1/rosekhlifa/announcements — admin view, includes expired too.
func (h *handlers) adminListAnnouncements(w http.ResponseWriter, r *http.Request) {
	list, err := h.store.ListAllAnnouncements(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, a := range list {
		out = append(out, announcementDTO(&a))
	}
	writeJSON(w, http.StatusOK, out)
}

type announcementReq struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	Level     string `json:"level"`
	ExpiresAt *int64 `json:"expiresAt"` // unix sec; null/omitted = no expiry
}

func (h *handlers) adminCreateAnnouncement(w http.ResponseWriter, r *http.Request) {
	var req announcementReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	title := strings.TrimSpace(req.Title)
	content := strings.TrimSpace(req.Content)
	if title == "" {
		writeErr(w, http.StatusBadRequest, "标题不能为空")
		return
	}
	if content == "" {
		writeErr(w, http.StatusBadRequest, "内容不能为空")
		return
	}
	if len(title) > 200 {
		writeErr(w, http.StatusBadRequest, "标题过长（≤200 字）")
		return
	}
	if len(content) > 10000 {
		writeErr(w, http.StatusBadRequest, "内容过长（≤10000 字）")
		return
	}
	level := req.Level
	if !validLevels[level] {
		level = "info"
	}
	a := &store.Announcement{
		Title:   title,
		Content: content,
		Level:   level,
	}
	if req.ExpiresAt != nil && *req.ExpiresAt > 0 {
		t := time.Unix(*req.ExpiresAt, 0)
		a.ExpiresAt = &t
	}
	if err := h.store.CreateAnnouncement(r.Context(), a); err != nil {
		writeErr(w, http.StatusInternalServerError, "保存失败")
		return
	}
	h.log.Info("announcement created", "id", a.ID, "title", a.Title, "level", a.Level)
	// Broadcast so any open browser sees the new notice immediately.
	if h.bus != nil {
		h.bus.PublishJSON(events.TypeAnnouncementChanged, map[string]any{
			"action": "create",
			"id":     a.ID,
		})
	}
	writeJSON(w, http.StatusOK, announcementDTO(a))
}

func (h *handlers) adminUpdateAnnouncement(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "id 无效")
		return
	}
	cur, err := h.store.GetAnnouncement(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "公告不存在")
			return
		}
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	var req announcementReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if t := strings.TrimSpace(req.Title); t != "" {
		cur.Title = t
	}
	if c := strings.TrimSpace(req.Content); c != "" {
		cur.Content = c
	}
	if validLevels[req.Level] {
		cur.Level = req.Level
	}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt <= 0 {
			cur.ExpiresAt = nil
		} else {
			t := time.Unix(*req.ExpiresAt, 0)
			cur.ExpiresAt = &t
		}
	}
	if err := h.store.UpdateAnnouncement(r.Context(), cur); err != nil {
		writeErr(w, http.StatusInternalServerError, "保存失败")
		return
	}
	if h.bus != nil {
		h.bus.PublishJSON(events.TypeAnnouncementChanged, map[string]any{
			"action": "update",
			"id":     cur.ID,
		})
	}
	writeJSON(w, http.StatusOK, announcementDTO(cur))
}

func (h *handlers) adminDeleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "id 无效")
		return
	}
	if err := h.store.DeleteAnnouncement(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	if h.bus != nil {
		h.bus.PublishJSON(events.TypeAnnouncementChanged, map[string]any{
			"action": "delete",
			"id":     id,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func announcementDTO(a *store.Announcement) map[string]any {
	out := map[string]any{
		"id":        a.ID,
		"title":     a.Title,
		"content":   a.Content,
		"level":     a.Level,
		"createdAt": a.CreatedAt.Unix(),
		"updatedAt": a.UpdatedAt.Unix(),
	}
	if a.ExpiresAt != nil {
		out["expiresAt"] = a.ExpiresAt.Unix()
	} else {
		out["expiresAt"] = nil
	}
	return out
}
