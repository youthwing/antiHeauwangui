// Package events is an in-memory pub/sub bus that lets backend components
// (scheduler, notifier, web handlers) emit lightweight notifications that
// the admin SSE endpoint can stream to connected browsers.
//
// Scope is intentionally tiny:
//   - Single process. Multi-instance deployments would need Redis or NATS;
//     wangui runs as one Docker container, so in-memory is fine.
//   - Best-effort: if a subscriber's buffer fills up, we drop events for
//     that subscriber rather than block the publisher. Admin dashboards
//     can reconcile by re-pulling the full state.
//   - No persistence: events disappear on restart. The DB is the source
//     of truth for everything that matters.
package events

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// Event is a single broadcast payload. Type lets the frontend route the
// event; Payload is opaque JSON, decoded client-side per type.
type Event struct {
	Type      string          `json:"type"`
	At        int64           `json:"at"` // unix seconds
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// Common event-type constants. New types are free to be added — the
// frontend just ignores unknown ones.
const (
	TypeSignResult     = "sign.result"      // a user finished a sign attempt
	TypeTokenWarn      = "token.warn"       // a token-expiry warning went out
	TypeGuestCleanup   = "guest.cleanup"    // expired guests purged
	TypeRulesChanged   = "school.rules"     // school rules diff detected
	TypeStatsUpdated   = "stats.update"     // admin stats may have changed
	TypeWindowOpen     = "window.open"      // 22:00 sign window armed
	TypeWindowClose    = "window.close"     // 22:30 sign window finished
)

// Bus is the singleton-per-process pub/sub broker.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[uint64]chan Event
	nextID      atomic.Uint64
}

// New returns an empty bus ready to publish.
func New() *Bus {
	return &Bus{subscribers: map[uint64]chan Event{}}
}

// Subscribe registers a new listener. Returns the channel to read from and
// a cancel func that unregisters it. Buffer size 32 is enough for normal
// activity bursts; a slow consumer that backs up will start dropping
// events (the publisher will not block).
func (b *Bus) Subscribe() (<-chan Event, func()) {
	id := b.nextID.Add(1)
	ch := make(chan Event, 32)
	b.mu.Lock()
	b.subscribers[id] = ch
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		if c, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(c)
		}
		b.mu.Unlock()
	}
}

// Publish sends an event to every current subscriber. Best-effort: a slow
// subscriber whose buffer is full simply doesn't receive this one.
func (b *Bus) Publish(evt Event) {
	if evt.At == 0 {
		evt.At = time.Now().Unix()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- evt:
		default:
			// Subscriber is behind — drop for them, don't block everyone.
		}
	}
}

// PublishJSON marshals payload and publishes. Convenience for callers
// that have a typed struct rather than a pre-encoded RawMessage.
func (b *Bus) PublishJSON(eventType string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = []byte("null")
	}
	b.Publish(Event{Type: eventType, Payload: raw})
}

// Count returns the current subscriber count (for diagnostics / log lines).
func (b *Bus) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}
