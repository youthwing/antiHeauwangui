package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GET /api/v1/airvel/events — Server-Sent Events stream of admin events.
//
// Clients use the browser's EventSource API to subscribe:
//
//	const es = new EventSource('/api/v1/airvel/events', { withCredentials: true })
//	es.addEventListener('message', e => { ... })
//	es.addEventListener('sign.result', e => { ... }) // typed
//
// Events emitted by the scheduler / notifier (sign.result, token.warn,
// guest.cleanup, school.rules, etc.) are fanned out to every connected
// admin browser. On reconnect, EventSource auto-retries — no resume
// semantics, this is best-effort live data, not an audit log (the DB
// remains the source of truth for replay).
//
// Connections stay open until the client disconnects or 4 hours pass
// (after which we close so the browser reconnects, refreshing auth).
func (h *handlers) adminEvents(w http.ResponseWriter, r *http.Request) {
	if h.bus == nil {
		writeErr(w, http.StatusServiceUnavailable, "event bus 未初始化")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "服务器不支持流式响应")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	// Disable nginx buffering if reverse-proxied.
	w.Header().Set("X-Accel-Buffering", "no")

	ch, cancel := h.bus.Subscribe()
	defer cancel()

	// Tell the client which retry interval to use if the stream drops.
	// 5 seconds is enough that we don't hammer the server on flaky networks.
	fmt.Fprintf(w, "retry: 5000\n\n")
	flusher.Flush()

	// Send an initial hello so the client knows the connection is live
	// even if no events arrive for a while.
	hello := map[string]any{
		"connected": true,
		"at":        time.Now().Unix(),
	}
	helloRaw, _ := json.Marshal(hello)
	fmt.Fprintf(w, "event: hello\ndata: %s\n\n", helloRaw)
	flusher.Flush()

	// Heartbeat ticker: send a comment line every 25s. This keeps NAT /
	// proxies from idling the connection out, without polluting the
	// EventSource onmessage handler (browsers ignore comments).
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	maxLife := time.After(4 * time.Hour)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-maxLife:
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprintf(w, ": ping %d\n\n", time.Now().Unix()); err != nil {
				return
			}
			flusher.Flush()
		case evt, ok := <-ch:
			if !ok {
				return
			}
			payload := evt.Payload
			if payload == nil {
				payload = json.RawMessage("null")
			}
			// SSE frame format. We emit both `event:` (typed listener) and
			// the default `message` channel by repeating the type inside
			// data so the catch-all handler still gets the type.
			body := map[string]any{
				"type":    evt.Type,
				"at":      evt.At,
				"payload": payload,
			}
			raw, _ := json.Marshal(body)
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, raw); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
