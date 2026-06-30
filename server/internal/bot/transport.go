package bot

import (
	"context"
	"errors"
)

// MessageSender — транспорт доставки ОДНОГО сообщения в Telegram (Story 7.1, AC1). Live HTTP-реализация
// (sendMessage по bot-токену) — ЗА ТОКЕНОМ (шов, Epic 3/3.7); в тестах — мок. TelegramDispatcher не знает
// про HTTP/токен → форматирование+нейтральность+классификация отказов строятся и тестируются без токена.
type MessageSender interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// ErrChatUnavailable — ПОСТОЯННЫЙ отказ доставки (бот заблокирован / чат удалён / chat not found): ретрай
// бессмыслен → деактивация подписки, НЕ рост attempts (AC4). Live-транспорт оборачивает Telegram 403 /
// «chat not found» в эту ошибку; 429 / 5xx / сеть остаются ОБЫЧНОЙ (transient) ошибкой → воркер растит
// attempts (O-4 backoff). Классификация — единственная точка различения permanent vs transient.
var ErrChatUnavailable = errors.New("bot: chat unavailable (blocked/deleted)")

// IsPermanent — постоянный ли отказ доставки (деактивация vs ретрай). errors.Is по ErrChatUnavailable
// (live-транспорт оборачивает им 403/«chat not found»).
func IsPermanent(err error) bool { return errors.Is(err, ErrChatUnavailable) }
