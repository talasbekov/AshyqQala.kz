package outbox

import (
	"context"
	"log/slog"
)

// Dispatcher — контракт доставки события получателю (O-2). Сигнатура ЗАФИКСИРОВАНА как контракт: в S-0
// единственная реализация — LoggingDispatcher; реальная Telegram-доставка + fan-out по подпискам
// (subscription_id) — Epic 7 (Вопрос №4 — 2-7 несёт МЕХАНИЗМ, не живую эмиссию/Telegram).
type Dispatcher interface {
	Send(ctx context.Context, e Event) error
}

// LoggingDispatcher — S-0-реализация Dispatcher: логирует НЕЙТРАЛЬНЫЙ конверт (event_id/type/subject_ref/v)
// и возвращает nil (успех). НЕ генерирует прозу флага и НЕ логирует payload как текст — нейтральность
// (CLAUDE.md#guardrails) достигается через render на поверхности bot (Epic 7), не здесь.
type LoggingDispatcher struct{ Log *slog.Logger }

// var _ — compile-time контракт: LoggingDispatcher реализует Dispatcher (AC3).
var _ Dispatcher = LoggingDispatcher{}

// Send логирует нейтральный конверт и возвращает nil. Log == nil → no-op (успех), чтобы вызов был безопасен
// в тестах/без сконфигурированного логгера.
func (d LoggingDispatcher) Send(ctx context.Context, e Event) error {
	if d.Log != nil {
		d.Log.InfoContext(ctx, "outbox_dispatch",
			"event_id", e.EventID.String(),
			"type", e.Type,
			"subject_ref", e.SubjectRef,
			"v", e.V,
		)
	}
	return nil
}
