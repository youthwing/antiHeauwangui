package web

import (
	"net/http"
	"time"

	"wangui/internal/store"
)

// GET /api/v1/platform-stats — tiny public-ish endpoint surfacing the
// lifetime "wangui signed N times for everyone" counter shown in the user
// sidebar. No auth: it's a flat number, not user-scoped data. Cached lightly
// in the browser via no-store but with an implicit 60s app-level poll.
func (h *handlers) platformStats(w http.ResponseWriter, r *http.Request) {
	n, err := h.store.CountSuccessRecords(r.Context())
	if err != nil {
		// Don't fail the whole sidebar over a stats blip — return 0 so
		// the chip just shows nothing for a moment.
		writeJSON(w, http.StatusOK, map[string]any{"totalSigns": 0})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"totalSigns": n})
}

// stats computes per-user sign-in aggregates: current/longest streak, the
// in-progress month's signed-vs-expected count, and lifetime success/failure
// totals. Everything is derived on-the-fly from sign_records — no extra
// counters to keep in sync.
//
// "Streak" semantics:
//   - We only count days that the user's signDays mask says they SHOULD sign.
//   - "Did sign" means a success/already/exempt record exists for that local
//     date. exempt counts because the school explicitly granted a pass (请假
//     / 节假日 / 走读).
//   - Non-sign-days don't count toward the streak and don't break it either.
//   - failed/missing records on a sign-day → streak breaks.
//
// We look back up to 365 days to keep the query bounded.
func (h *handlers) stats(w http.ResponseWriter, r *http.Request) {
	uid := userIDOf(r)
	u, err := h.store.GetUser(r.Context(), uid)
	if err != nil {
		writeErr(w, http.StatusNotFound, "用户不存在")
		return
	}
	now := time.Now()
	from := now.AddDate(0, 0, -365)
	recs, err := h.store.ListRecordsBetween(r.Context(), uid, from, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	s := computeStats(recs, u.SignDays, u.CreatedAt, now)
	writeJSON(w, http.StatusOK, s)
}

type userStats struct {
	CurrentStreak int     `json:"currentStreak"`
	LongestStreak int     `json:"longestStreak"`
	MonthSigned   int     `json:"monthSigned"`
	MonthExpected int     `json:"monthExpected"`
	MonthRate     float64 `json:"monthRate"`
	TotalSuccess  int     `json:"totalSuccess"`
	TotalAlready  int     `json:"totalAlready"`
	TotalExempt   int     `json:"totalExempt"`
	TotalFailed   int     `json:"totalFailed"`
	TotalSkipped  int     `json:"totalSkipped"`
	FirstSignAt   int64   `json:"firstSignAt"` // unix seconds, 0 if none
}

// dateKey returns the YYYY-MM-DD string for t in t's location. Used as a
// hash key when grouping records by day.
func dateKey(t time.Time) string {
	return t.Format("2006-01-02")
}

// rankStatus picks the "best" outcome status for a day when multiple records
// exist (e.g. a failed retry followed by a manual success). Higher rank wins.
func rankStatus(s string) int {
	switch s {
	case "success":
		return 5
	case "already":
		return 4
	case "exempt":
		return 3
	case "failed":
		return 2
	case "skipped":
		return 1
	}
	return 0
}

func computeStats(recs []store.Record, signDays int, createdAt, now time.Time) userStats {
	out := userStats{}
	if len(recs) == 0 {
		return out
	}

	// Group by local date → best outcome. Also tally lifetime totals.
	byDay := map[string]string{}
	var firstAt int64
	for _, r := range recs {
		k := dateKey(r.OccurredAt)
		if prev, ok := byDay[k]; !ok || rankStatus(r.Status) > rankStatus(prev) {
			byDay[k] = r.Status
		}
		switch r.Status {
		case "success":
			out.TotalSuccess++
		case "already":
			out.TotalAlready++
		case "exempt":
			out.TotalExempt++
		case "failed":
			out.TotalFailed++
		case "skipped":
			out.TotalSkipped++
		}
		ts := r.OccurredAt.Unix()
		if firstAt == 0 || ts < firstAt {
			firstAt = ts
		}
	}
	out.FirstSignAt = firstAt

	// --- streaks ---
	// Walk backwards from today. For each day:
	//   - if it's a sign-day:
	//       success/already/exempt → streak++
	//       failed/missing → break
	//   - else: skip (no effect)
	loc := now.Location()
	cur := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	// Cap: don't walk past account creation, never beyond 365 days.
	floor := createdAt.AddDate(0, 0, -1)
	hardFloor := now.AddDate(0, 0, -365)
	if floor.Before(hardFloor) {
		floor = hardFloor
	}

	// Current streak — running from today backwards until break.
	current := 0
	broken := false
	for d := cur; !d.Before(floor); d = d.AddDate(0, 0, -1) {
		if !isSignDayBit(signDays, d) {
			continue
		}
		st := byDay[dateKey(d)]
		if isSignSuccess(st) {
			current++
		} else {
			// Today is special: if it's a sign-day but the window hasn't
			// closed yet (before 22:30) AND no record yet, don't count it
			// as a break — the user might still sign tonight.
			if sameLocalDate(d, now) && now.Hour() < 22 || (now.Hour() == 22 && now.Minute() < 30) {
				continue
			}
			broken = true
			break
		}
	}
	_ = broken
	out.CurrentStreak = current

	// Longest streak — sliding count across the same window.
	longest := 0
	run := 0
	for d := floor; !d.After(cur); d = d.AddDate(0, 0, 1) {
		if !isSignDayBit(signDays, d) {
			continue
		}
		st := byDay[dateKey(d)]
		if isSignSuccess(st) {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	if longest < current {
		// Current run can extend beyond the historical longest.
		longest = current
	}
	out.LongestStreak = longest

	// --- monthly ---
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	for d := monthStart; !d.After(cur); d = d.AddDate(0, 0, 1) {
		if !isSignDayBit(signDays, d) {
			continue
		}
		// Skip future days within the same month (won't happen since we
		// iterate up to today, but keeps the intent explicit).
		out.MonthExpected++
		if isSignSuccess(byDay[dateKey(d)]) {
			out.MonthSigned++
		}
	}
	if out.MonthExpected > 0 {
		out.MonthRate = float64(out.MonthSigned) / float64(out.MonthExpected)
	}
	return out
}

// isSignDayBit reports whether t falls on a weekday the user wants to sign.
// signDays is a 7-bit mask: bit 0 = Monday … bit 6 = Sunday. Mirrors the
// scheduler's isSignDay.
func isSignDayBit(signDays int, t time.Time) bool {
	if signDays&0x7f == 0 {
		return false
	}
	if signDays&0x7f == 0x7f {
		return true
	}
	bit := (int(t.Weekday()) + 6) % 7
	return signDays&(1<<bit) != 0
}

func isSignSuccess(status string) bool {
	switch status {
	case "success", "already", "exempt":
		return true
	}
	return false
}

func sameLocalDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}
