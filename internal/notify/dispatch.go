package notify

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"wangui/internal/store"
)

// Dispatcher is the high-level helper the scheduler/handlers use to fire
// emails. It re-reads SMTP config every Send so admin updates take effect
// immediately without restart.
type Dispatcher struct {
	Store *store.Store
	Log   *slog.Logger
}

// SignResult mirrors scheduler.SignResult (we avoid an import cycle by
// duplicating the small struct here).
type SignResult struct {
	Status  string
	Message string
}

// DispatchSignResult queues an email for the user (if they enabled notify) and
// always copies the admin Bcc (if configured). Non-blocking from the caller's
// perspective — fires a goroutine.
func (d *Dispatcher) DispatchSignResult(u *store.User, res SignResult) {
	go d.dispatchSync(u, res)
}

func (d *Dispatcher) dispatchSync(u *store.User, res SignResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := d.Store.GetSMTPConfig(ctx)
	if err != nil {
		d.log("smtp config load failed", "err", err.Error())
		return
	}
	if !cfg.Enabled || cfg.Host == "" || cfg.Username == "" {
		return // notifications disabled at system level
	}

	client := &EmailClient{
		Host: cfg.Host, Port: cfg.Port,
		Username: cfg.Username, Password: cfg.Password,
		From: cfg.From,
	}

	subject, text, html := renderSignEmail(u, res)

	// Guests: admin owns the relationship, the friend never gave us an
	// email. Only ship to admin BCC.
	if u.IsGuest {
		if cfg.AdminBcc != "" {
			msg := Message{
				To:      cfg.AdminBcc,
				Subject: "[临时朋友] " + subject,
				Text:    text,
				HTML:    html,
			}
			if err := client.Send(msg); err != nil {
				d.log("email to admin (guest) failed", "to", cfg.AdminBcc, "err", err.Error())
			} else {
				d.log("admin guest log email sent", "to", cfg.AdminBcc, "status", res.Status, "guest", u.GuestLabel)
			}
		}
		return
	}

	// Regular user email
	if u.NotifyEnabled && u.NotifyEmail != "" {
		msg := Message{
			To:      u.NotifyEmail,
			Subject: subject,
			Text:    text,
			HTML:    html,
		}
		// Admin bcc piggy-backs on user mail when both apply.
		if cfg.AdminBcc != "" && cfg.AdminBcc != u.NotifyEmail {
			msg.Bcc = []string{cfg.AdminBcc}
		}
		if err := client.Send(msg); err != nil {
			d.log("email to user failed", "user", u.UserID, "to", u.NotifyEmail, "err", err.Error())
		} else {
			d.log("email sent", "user", u.UserID, "to", u.NotifyEmail, "status", res.Status)
		}
		return
	}

	// User didn't opt-in, but admin still wants the log.
	if cfg.AdminBcc != "" {
		msg := Message{
			To:      cfg.AdminBcc,
			Subject: "[管理员日志] " + subject,
			Text:    text,
			HTML:    html,
		}
		if err := client.Send(msg); err != nil {
			d.log("email to admin failed", "to", cfg.AdminBcc, "err", err.Error())
		} else {
			d.log("admin log email sent", "to", cfg.AdminBcc, "status", res.Status)
		}
	}
}

// DispatchGuestCleanup sends one summary email to admin listing the guests
// the cleanup ticker just deleted. Non-blocking.
func (d *Dispatcher) DispatchGuestCleanup(expired []store.ExpiredGuest) {
	if len(expired) == 0 {
		return
	}
	go d.dispatchCleanupSync(expired)
}

func (d *Dispatcher) dispatchCleanupSync(expired []store.ExpiredGuest) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := d.Store.GetSMTPConfig(ctx)
	if err != nil || !cfg.Enabled || cfg.Host == "" || cfg.AdminBcc == "" {
		return
	}
	client := &EmailClient{
		Host: cfg.Host, Port: cfg.Port,
		Username: cfg.Username, Password: cfg.Password,
		From: cfg.From,
	}

	var b strings.Builder
	for _, g := range expired {
		fmt.Fprintf(&b, "· %s (%s · %s)\n", g.Label, g.Name, g.UserID)
	}
	subject := fmt.Sprintf("[勿外传] 已自动清理 %d 个过期临时朋友", len(expired))
	text := fmt.Sprintf("清理时间：%s\n\n%s", time.Now().Format("2006-01-02 15:04"), b.String())
	if err := client.Send(Message{
		To:      cfg.AdminBcc,
		Subject: subject,
		Text:    text,
	}); err != nil {
		d.log("guest cleanup email failed", "err", err.Error())
	} else {
		d.log("guest cleanup email sent", "to", cfg.AdminBcc, "count", len(expired))
	}
}

func renderSignEmail(u *store.User, res SignResult) (subject, text, html string) {
	statusLabels := map[string]string{
		"success": "签到成功",
		"already": "今日已签",
		"exempt":  "免签",
		"failed":  "签到失败",
		"skipped": "跳过",
	}
	statusColor := map[string]string{
		"success": "#10b981",
		"already": "#3b82f6",
		"exempt":  "#71717a",
		"failed":  "#ef4444",
		"skipped": "#f59e0b",
	}
	label, ok := statusLabels[res.Status]
	if !ok {
		label = res.Status
	}
	color := statusColor[res.Status]
	if color == "" {
		color = "#71717a"
	}
	when := time.Now().Format("2006-01-02 15:04:05")

	subject = fmt.Sprintf("[勿外传] %s · %s", label, u.UserName)
	text = fmt.Sprintf("姓名：%s\n学号：%s\n结果：%s\n说明：%s\n时间：%s\n",
		u.UserName, u.UserNumber, label, res.Message, when)
	html = fmt.Sprintf(`<!doctype html><html><body style="font-family: -apple-system,Segoe UI,sans-serif; background:#fafafa; padding:24px; color:#18181b;">
<div style="max-width:560px;margin:0 auto;background:#fff;border:1px solid #e5e7eb;border-radius:12px;overflow:hidden;">
  <div style="padding:18px 22px;background:%s;color:#fff;">
    <div style="font-size:11px;letter-spacing:.1em;text-transform:uppercase;opacity:.8;">勿外传 · 签到结果</div>
    <div style="font-size:22px;font-weight:700;margin-top:4px;">%s</div>
  </div>
  <div style="padding:18px 22px;font-size:14px;line-height:1.7;">
    <div><strong style="color:#71717a;">姓名</strong>　%s</div>
    <div><strong style="color:#71717a;">学号</strong>　%s</div>
    <div><strong style="color:#71717a;">时间</strong>　%s</div>
    <div style="margin-top:10px;padding:10px 12px;background:#f4f4f5;border-radius:8px;font-size:13px;color:#3f3f46;">%s</div>
  </div>
  <div style="padding:12px 22px;font-size:11px;color:#a1a1aa;background:#fafafa;border-top:1px solid #e5e7eb;">
    勿外传 · 仅内部使用 · 自动发送，请勿回复
  </div>
</div>
</body></html>`,
		color, label, u.UserName, u.UserNumber, when, res.Message)
	return
}

func (d *Dispatcher) log(msg string, kv ...any) {
	if d.Log == nil {
		return
	}
	d.Log.Info("[notify] "+msg, kv...)
}
