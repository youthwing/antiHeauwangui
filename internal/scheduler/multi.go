package scheduler

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"wangui/internal/api"
	"wangui/internal/notify"
	"wangui/internal/store"
)

const (
	WindowHour        = 22
	WindowStartMinute = 0
	WindowEndMinute   = 30
	DefaultRuleID     = 1
)

// Multi is the multi-tenant scheduler.
// It wakes once per day for the 22:00–22:30 window, then fans out a goroutine
// per auto-signing user with per-user trigger/jitter/retry settings.
type Multi struct {
	store    *store.Store
	log      *slog.Logger
	notifier *notify.Dispatcher
}

func NewMulti(s *store.Store, l *slog.Logger) *Multi {
	return &Multi{
		store:    s,
		log:      l,
		notifier: &notify.Dispatcher{Store: s, Log: l},
	}
}

func (m *Multi) Start(ctx context.Context) { go m.loop(ctx) }

func (m *Multi) loop(ctx context.Context) {
	for {
		next := nextWindowStart(time.Now())
		m.log.Info("scheduler armed", "next_window_start", next.Format(time.RFC3339))
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}
		m.runWindow(ctx)
	}
}

func nextWindowStart(now time.Time) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(),
		WindowHour, WindowStartMinute, 0, 0, now.Location())
	if today.After(now) {
		return today
	}
	return today.Add(24 * time.Hour)
}

func windowEndOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(),
		WindowHour, WindowEndMinute, 0, 0, t.Location())
}

func (m *Multi) runWindow(ctx context.Context) {
	end := windowEndOf(time.Now())
	winCtx, cancel := context.WithDeadline(ctx, end.Add(30*time.Second))
	defer cancel()

	ids, err := m.store.ListAutoSignUsers(winCtx)
	if err != nil {
		m.log.Error("list auto-sign users failed", "err", err.Error())
		return
	}
	m.log.Info("window opened", "auto_sign_users", len(ids), "deadline", end.Format(time.RFC3339))

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(uid string) {
			defer wg.Done()
			m.runForUser(winCtx, uid, end)
		}(id)
	}
	wg.Wait()
	m.log.Info("window closed")
}

func (m *Multi) runForUser(ctx context.Context, userID string, deadline time.Time) {
	u, err := m.store.GetUser(ctx, userID)
	if err != nil {
		m.log.Error("get user failed", "user", userID, "err", err.Error())
		return
	}
	if !u.AutoSign || u.IsDisabled {
		return
	}
	if !isSignDay(u.SignDays, time.Now()) {
		// Today isn't in the user's sign_days bitmask — silently skip.
		// We deliberately don't write a sign_records row or send an email:
		// "today is a rest day" is the expected steady state, not an event.
		m.log.Info("skip non-sign-day", "user", userID, "sign_days", u.SignDays)
		return
	}

	// Wait for trigger_minute then add jitter.
	now := time.Now()
	primary := time.Date(now.Year(), now.Month(), now.Day(),
		WindowHour, WindowStartMinute+u.TriggerMinute, 0, 0, now.Location())
	jitter := time.Duration(rand.IntN(u.JitterSec+1)) * time.Second
	target := primary.Add(jitter)
	if target.Before(now) {
		target = now.Add(jitter)
	}
	select {
	case <-ctx.Done():
		return
	case <-time.After(time.Until(target)):
	}

	maxAttempts := 1 + u.RetryCount
	gap := time.Duration(u.RetryGapMin) * time.Minute

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if time.Now().After(deadline) {
			m.log.Warn("deadline passed", "user", userID, "attempt", attempt)
			return
		}
		// Re-read user (settings may have changed mid-window).
		cur, err := m.store.GetUser(ctx, userID)
		if err != nil {
			m.log.Error("get user (retry loop)", "user", userID, "err", err.Error())
			return
		}
		if !cur.AutoSign || cur.IsDisabled {
			return
		}
		res := m.SignOnce(ctx, cur)
		_ = m.store.AddRecord(ctx, &store.Record{
			UserID: userID, RuleID: DefaultRuleID,
			Status: res.Status, Message: res.Message,
		})
		m.log.Info("attempt",
			"user", userID, "attempt", attempt,
			"status", res.Status, "msg", res.Message)
		if res.Terminal() {
			// Send notification on terminal outcome (success / already / exempt).
			m.notifier.DispatchSignResult(cur, notify.SignResult{Status: res.Status, Message: res.Message})
			return
		}
		// On the FINAL failed attempt, notify once.
		if attempt == maxAttempts {
			m.notifier.DispatchSignResult(cur, notify.SignResult{Status: res.Status, Message: res.Message})
		}
		wait := gap
		if remain := time.Until(deadline); remain < wait {
			wait = remain
		}
		if wait <= 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// SignResult is the outcome of a single sign attempt.
type SignResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (r SignResult) Terminal() bool {
	switch r.Status {
	case "success", "already", "exempt":
		return true
	}
	return false
}

// SignOnce performs a single status + (optional) sign cycle for one user.
// It never writes records — the caller decides whether to persist.
func (m *Multi) SignOnce(ctx context.Context, u *store.User) SignResult {
	c := api.New(u.Token)
	st, err := c.CheckinStatus(ctx, DefaultRuleID)
	if err != nil {
		if api.IsAuthExpired(err) {
			return SignResult{Status: "failed", Message: "token 已失效，请更新"}
		}
		return SignResult{Status: "failed", Message: "状态获取失败: " + err.Error()}
	}
	if st.IsBoarding {
		return SignResult{Status: "exempt", Message: "外宿学生无需签到"}
	}
	if st.IsExempt != nil && *st.IsExempt {
		return SignResult{Status: "exempt", Message: nonEmpty(st.Message, "请假中")}
	}
	if st.HasCheckedIn != nil && *st.HasCheckedIn {
		return SignResult{Status: "already", Message: nonEmpty(st.Message, "今日已签到")}
	}
	if !st.CanCheckin {
		return SignResult{Status: "failed", Message: nonEmpty(st.Message, "当前无法签到")}
	}
	if u.Lat == 0 || u.Lng == 0 {
		return SignResult{Status: "failed", Message: "未配置打卡坐标"}
	}
	req := api.SignRequest{
		RuleID:       DefaultRuleID,
		Latitude:     u.Lat,
		Longitude:    u.Lng,
		DeviceModel:  nonEmpty(u.DeviceModel, "iPhone"),
		DeviceSystem: nonEmpty(u.DeviceSystem, "iOS"),
	}
	// If the dorm admin opted in to including address detail in the
	// sign request, fill those fields; otherwise leave blank → omitempty drops them.
	if u.SendAddressFields {
		req.LocationAddress = u.Address
		req.City = u.City
		req.Road = u.Road
		req.Poi = u.Poi
	}
	if _, err := c.Sign(ctx, req); err != nil {
		if api.IsAuthExpired(err) {
			return SignResult{Status: "failed", Message: "token 已失效，请更新"}
		}
		return SignResult{Status: "failed", Message: err.Error()}
	}
	return SignResult{Status: "success", Message: "签到成功"}
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// isSignDay reports whether t falls on a weekday the user wants to sign on.
// signDays is a 7-bit mask: bit 0 = Monday … bit 6 = Sunday. The mapping
// converts Go's time.Weekday (Sunday=0..Saturday=6) into that scheme.
func isSignDay(signDays int, t time.Time) bool {
	if signDays&0x7f == 0 {
		return false
	}
	if signDays&0x7f == 0x7f {
		return true
	}
	bit := (int(t.Weekday()) + 6) % 7 // Sun→6, Mon→0, …, Sat→5
	return signDays&(1<<bit) != 0
}
