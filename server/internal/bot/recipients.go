package bot

import (
	"context"

	"ashyqqala/server/internal/outbox"
)

// RecipientResolver — fan-out события в chat_id подписчиков (FR-24). Story 7.2 даст РЕАЛЬНУЮ таблицу
// подписок (telegram_chat_id ↔ район); здесь — ШОВ. Дефолт NoRecipients (подписок ещё нет) → 7.1
// token/7.2-независима: Send корректен, просто никому не шлёт. Тесты подставляют мок со списком chat_id.
type RecipientResolver interface {
	Recipients(ctx context.Context, e outbox.Event) ([]int64, error)
}

// NoRecipients — no-op резолвер: пустой список (таблицы подписок ещё нет — Story 7.2). ЯВНЫЙ no-op
// (анти-фиктивность: не nil-паника, а честное «нет получателей»).
type NoRecipients struct{}

// Recipients всегда возвращает пустой список (нет подписок до Story 7.2).
func (NoRecipients) Recipients(context.Context, outbox.Event) ([]int64, error) { return nil, nil }
