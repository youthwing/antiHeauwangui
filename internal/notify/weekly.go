package notify

import (
	"context"
	"fmt"
	"time"

	"wangui/internal/store"
)

// WeeklyStats mirrors scheduler.WeeklyStats. Duplicated here to avoid an
// import cycle (notify is imported by scheduler, not the reverse).
type WeeklyStats struct {
	From          time.Time
	To            time.Time
	DaysSigned    int
	DaysFailed    int
	DaysSkipped   int
	TotalAttempts int
	BestDay       string
	BestStatus    string
}

// DispatchWeeklyDigest fans out a "本周战绩" notice via email + Server酱
// for the given user. Admin BCC piggy-backs on the email path; the admin
// gets one digest per user (good enough for a 5-friend group; we can
// aggregate later if it ever gets noisy). Non-blocking.
func (d *Dispatcher) DispatchWeeklyDigest(u *store.User, s WeeklyStats) {
	go d.dispatchWeeklyDigestSync(u, s)
}

func (d *Dispatcher) dispatchWeeklyDigestSync(u *store.User, s WeeklyStats) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := d.Store.GetSMTPConfig(ctx)
	if err != nil {
		return
	}

	subject, text, html := renderWeeklyDigestEmail(u, s)
	wechatTitle, wechatBody := renderWeeklyDigestServerChan(u, s)

	// Email path
	if cfg.Enabled && cfg.Host != "" && cfg.Username != "" {
		client := &EmailClient{Host: cfg.Host, Port: cfg.Port, Username: cfg.Username, Password: cfg.Password, From: cfg.From}
		if u.NotifyEnabled && u.NotifyEmail != "" {
			msg := Message{To: u.NotifyEmail, Subject: subject, Text: text, HTML: html}
			if cfg.AdminBcc != "" && cfg.AdminBcc != u.NotifyEmail {
				msg.Bcc = []string{cfg.AdminBcc}
			}
			if err := client.Send(msg); err != nil {
				d.log("weekly-digest email failed", "user", u.UserID, "err", err.Error())
			}
		} else if cfg.AdminBcc != "" {
			_ = client.Send(Message{To: cfg.AdminBcc, Subject: "[管理员日志] " + subject, Text: text, HTML: html})
		}
	}
	// Server酱 path
	if u.ServerChanEnabled && u.ServerChanKey != "" {
		_ = NewServerChan(u.ServerChanKey).Send(ctx, wechatTitle, wechatBody)
	}
	if cfg.AdminServerChanEnabled && cfg.AdminServerChanKey != "" {
		_ = NewServerChan(cfg.AdminServerChanKey).Send(ctx, "[全员] "+wechatTitle, wechatBody)
	}
}

func renderWeeklyDigestEmail(u *store.User, s WeeklyStats) (subject, text, html string) {
	span := s.From.Format("01/02") + " – " + s.To.Format("01/02")
	subject = fmt.Sprintf("[antiWG] 本周战报 · %s · 签到 %d 天", u.UserName, s.DaysSigned)
	text = fmt.Sprintf("本周战报 (%s)\n\n姓名：%s\n学号：%s\n\n签到天数：%d\n失败天数：%d\n跳过天数：%d\n",
		span, u.UserName, u.UserNumber, s.DaysSigned, s.DaysFailed, s.DaysSkipped)
	if s.BestDay != "" {
		text += "最近成功：" + s.BestDay + "\n"
	}

	// Visual digest — emerald-toned to match the in-app stats strip.
	html = fmt.Sprintf(`<!doctype html><html><body style="font-family:-apple-system,Segoe UI,sans-serif;background:#fafafa;padding:24px;color:#18181b;">
<div style="max-width:580px;margin:0 auto;background:#fff;border:1px solid #e5e7eb;border-radius:12px;overflow:hidden;">
  <div style="padding:20px 24px;background:linear-gradient(135deg,#10b981 0%%,#3b82f6 100%%);color:#fff;">
    <div style="font-size:11px;letter-spacing:.1em;text-transform:uppercase;opacity:.85;">antiWG · 本周战报</div>
    <div style="font-size:22px;font-weight:700;margin-top:4px;">%s 的一周</div>
    <div style="font-size:12px;margin-top:4px;opacity:.85;">%s</div>
  </div>
  <div style="padding:8px 24px 4px;display:flex;gap:8px;flex-wrap:wrap;">
    <div style="flex:1;min-width:90px;background:#f0fdf4;border:1px solid #bbf7d0;border-radius:10px;padding:12px;">
      <div style="font-size:10px;color:#15803d;text-transform:uppercase;letter-spacing:.1em;">签到</div>
      <div style="font-size:28px;font-weight:700;color:#15803d;line-height:1.1;margin-top:4px;">%d <span style="font-size:12px;font-weight:500;color:#16a34a;">天</span></div>
    </div>
    <div style="flex:1;min-width:90px;background:#fef2f2;border:1px solid #fecaca;border-radius:10px;padding:12px;">
      <div style="font-size:10px;color:#b91c1c;text-transform:uppercase;letter-spacing:.1em;">失败</div>
      <div style="font-size:28px;font-weight:700;color:#b91c1c;line-height:1.1;margin-top:4px;">%d <span style="font-size:12px;font-weight:500;color:#dc2626;">天</span></div>
    </div>
    <div style="flex:1;min-width:90px;background:#fafafa;border:1px solid #e4e4e7;border-radius:10px;padding:12px;">
      <div style="font-size:10px;color:#52525b;text-transform:uppercase;letter-spacing:.1em;">跳过</div>
      <div style="font-size:28px;font-weight:700;color:#52525b;line-height:1.1;margin-top:4px;">%d <span style="font-size:12px;font-weight:500;color:#71717a;">天</span></div>
    </div>
  </div>
  <div style="padding:14px 24px;font-size:13px;color:#3f3f46;line-height:1.7;">
    <div><strong style="color:#71717a;">学号</strong>　%s</div>
    %s
  </div>
  <div style="padding:12px 24px;font-size:11px;color:#a1a1aa;background:#fafafa;border-top:1px solid #e5e7eb;">
    每周日晚 21:00 自动发送 · 你可以在「配置」里关掉邮件通知
  </div>
</div></body></html>`,
		u.UserName, span, s.DaysSigned, s.DaysFailed, s.DaysSkipped, u.UserNumber, bestDayBlock(s),
	)
	return
}

func bestDayBlock(s WeeklyStats) string {
	if s.BestDay == "" {
		return ""
	}
	label := s.BestStatus
	switch s.BestStatus {
	case "success":
		label = "成功"
	case "already":
		label = "已签"
	case "exempt":
		label = "免签"
	}
	return fmt.Sprintf(`<div><strong style="color:#71717a;">最近一次</strong>　%s · %s</div>`,
		s.BestDay, label)
}

func renderWeeklyDigestServerChan(u *store.User, s WeeklyStats) (title, body string) {
	span := s.From.Format("01/02") + "–" + s.To.Format("01/02")
	title = fmt.Sprintf("📊 本周战报 · %s · 签到 %d 天", u.UserName, s.DaysSigned)
	body = fmt.Sprintf(
		"## %s 的本周战绩 (%s)\n\n"+
			"- ✅ **签到** %d 天\n"+
			"- ❌ **失败** %d 天\n"+
			"- ⏭ **跳过** %d 天\n\n"+
			"**学号**：`%s`",
		u.UserName, span, s.DaysSigned, s.DaysFailed, s.DaysSkipped, u.UserNumber,
	)
	if s.BestDay != "" {
		body += "\n\n**最近成功**：" + s.BestDay
	}
	body += "\n\n— 每周日 21:00 自动发送"
	return
}
