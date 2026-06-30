package bot

import "context"

// SubscriptionDeactivator — деактивация подписки при ПОСТОЯННОМ отказе доставки (бот заблокирован / чат
// удалён, AC4): НЕ бесконечный ретрай. Story 7.2 даст реальную таблицу подписок; здесь — ШОВ. Дефолт
// NoopDeactivator → 7.1 token/7.2-независима. Прецедент RecalcHook (2.4) / PricePerKMSamples (6.4):
// движок реален, живое поведение ждёт данные/таблицу.
type SubscriptionDeactivator interface {
	Deactivate(ctx context.Context, chatID int64) error
}

// NoopDeactivator — no-op деактиватор (таблицы подписок ещё нет — Story 7.2). ЯВНЫЙ no-op
// (анти-фиктивность).
type NoopDeactivator struct{}

// Deactivate — no-op (нет таблицы подписок до Story 7.2).
func (NoopDeactivator) Deactivate(context.Context, int64) error { return nil }
