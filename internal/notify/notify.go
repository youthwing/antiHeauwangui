package notify

import "log/slog"

// Notifier is the abstract delivery channel for sign-in results.
// Phase 1: console/log only. Phase 2 will add Server酱 / email / webhook.
type Notifier interface {
	Info(msg string, kv ...any)
	Warn(msg string, kv ...any)
	Error(msg string, kv ...any)
}

type slogNotifier struct{ l *slog.Logger }

// NewLog returns a Notifier that writes through the given slog.Logger.
func NewLog(l *slog.Logger) Notifier {
	return &slogNotifier{l: l}
}

func (s *slogNotifier) Info(msg string, kv ...any)  { s.l.Info(msg, kv...) }
func (s *slogNotifier) Warn(msg string, kv ...any)  { s.l.Warn(msg, kv...) }
func (s *slogNotifier) Error(msg string, kv ...any) { s.l.Error(msg, kv...) }
