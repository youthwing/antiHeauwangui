package web

import (
	"encoding/json"

	"wangui/internal/store"
)

func signRecordDTO(rec store.Record) map[string]any {
	out := map[string]any{
		"id":         rec.ID,
		"ruleId":     rec.RuleID,
		"status":     rec.Status,
		"message":    rec.Message,
		"occurredAt": rec.OccurredAt.Unix(),
	}
	if rec.RequestDebug != "" {
		out["requestDebug"] = requestDebugValue(rec.RequestDebug)
	}
	return out
}

func adminSignRecordDTO(rec store.Record) map[string]any {
	out := signRecordDTO(rec)
	out["userId"] = rec.UserID
	out["userName"] = rec.UserName
	return out
}

func requestDebugValue(raw string) any {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	return v
}
