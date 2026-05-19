package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"wangui/internal/store"
)

const (
	sessionCookie      = "wangui_session"
	adminSessionCookie = "wangui_admin"
	siteGateCookie     = "antiwg_gate"
)

type ctxKey int

const (
	userIDCtx ctxKey = iota + 1
	adminIDCtx
)

// userAuth ensures a valid non-admin web session.
func (h *handlers) userAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ck, err := r.Cookie(sessionCookie)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "未登录")
			return
		}
		sess, err := h.store.GetSession(r.Context(), ck.Value)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				clearCookie(w, sessionCookie)
				writeErr(w, http.StatusUnauthorized, "会话已过期")
				return
			}
			writeErr(w, http.StatusInternalServerError, "会话校验失败")
			return
		}
		if sess.IsAdmin {
			writeErr(w, http.StatusForbidden, "该接口仅供用户")
			return
		}
		// Check user isn't disabled.
		u, err := h.store.GetUser(r.Context(), sess.UserID)
		if err == nil && u.IsDisabled {
			_ = h.store.DeleteSession(r.Context(), sess.SessionID)
			clearCookie(w, sessionCookie)
			writeErr(w, http.StatusForbidden, "账号已被禁用")
			return
		}
		ctx := context.WithValue(r.Context(), userIDCtx, sess.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// adminAuth ensures a valid admin web session.
func (h *handlers) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ck, err := r.Cookie(adminSessionCookie)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "管理员未登录")
			return
		}
		sess, err := h.store.GetSession(r.Context(), ck.Value)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				clearCookie(w, adminSessionCookie)
				writeErr(w, http.StatusUnauthorized, "管理员会话已过期")
				return
			}
			writeErr(w, http.StatusInternalServerError, "会话校验失败")
			return
		}
		if !sess.IsAdmin {
			writeErr(w, http.StatusForbidden, "需要管理员权限")
			return
		}
		ctx := context.WithValue(r.Context(), adminIDCtx, sess.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *handlers) hasSiteGateAccess(r *http.Request) (bool, error) {
	if ck, err := r.Cookie(siteGateCookie); err == nil {
		ok, err := h.store.HasSiteGatePass(r.Context(), ck.Value)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	// Existing user sessions should keep working even if the gate cookie was
	// cleared; the gate is for discovery control, not normal account access.
	if ck, err := r.Cookie(sessionCookie); err == nil {
		sess, err := h.store.GetSession(r.Context(), ck.Value)
		if err == nil && !sess.IsAdmin {
			return true, nil
		}
	}
	return false, nil
}

func (h *handlers) siteGateAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, err := h.hasSiteGateAccess(r)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "访问门禁校验失败")
			return
		}
		if !ok {
			writeErr(w, http.StatusForbidden, "需要访问码")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func userIDOf(r *http.Request) string {
	v, _ := r.Context().Value(userIDCtx).(string)
	return v
}

func adminIDOf(r *http.Request) string {
	v, _ := r.Context().Value(adminIDCtx).(string)
	return v
}

func setSessionCookie(w http.ResponseWriter, name, sid string, exp time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    sid,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
