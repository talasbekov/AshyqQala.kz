package bot

import (
	"context"
	"errors"

	"ashyqqala/server/internal/outbox"
	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/render"
)

// TelegramDispatcher реализует outbox.Dispatcher (AC1) — НОЛЬ правок контракта 2.7 (var _ ниже). Композиция
// швов (все мокаемы / no-op по умолчанию → 7.1 строится и тестируется без bot-токена и таблицы подписок):
//   - Sender      — транспорт доставки (live HTTP за токеном; мок в тестах);
//   - Resolver    — fan-out события в chat_id подписчиков (Story 7.2; дефолт NoRecipients);
//   - Deactivator — деактивация подписки при постоянном отказе (Story 7.2; дефолт NoopDeactivator);
//   - Renderer    — проза ТОЛЬКО через render (AC2); Loc — локаль поверхности;
//   - Throttle    — вежливость к Telegram (AC1); nil → без троттла.
type TelegramDispatcher struct {
	Sender      MessageSender
	Resolver    RecipientResolver
	Deactivator SubscriptionDeactivator
	Renderer    render.Renderer
	Loc         registry.Locale
	Throttle    *Throttle
}

// var _ — compile-time контракт: TelegramDispatcher реализует outbox.Dispatcher (AC1, ноль правок 2.7).
var _ outbox.Dispatcher = TelegramDispatcher{}

// Send доставляет событие подписчикам (AC1/AC4):
//   - нет подписчиков (NoRecipients — таблицы подписок ещё нет, 7.2) → nil (успех, никому не шлём — честно);
//   - постоянный отказ chat (заблокирован/удалён) → Deactivator + пропуск, НЕ ретрай (AC4);
//   - transient-ошибка (429/5xx/сеть) → возврат ошибки → воркер растит attempts (O-4 backoff), при потолке
//     max-attempts помечает dead-letter (worker ProcessBatchN).
//
// Дедуп (O-3, event_id) и видимость/ретрай (O-4, Clock) — на уровне outbox-воркера; здесь — доставка.
func (d TelegramDispatcher) Send(ctx context.Context, e outbox.Event) error {
	if d.Sender == nil {
		return errors.New("bot: TelegramDispatcher.Sender не задан")
	}
	resolver := RecipientResolver(NoRecipients{})
	if d.Resolver != nil {
		resolver = d.Resolver
	}
	deact := SubscriptionDeactivator(NoopDeactivator{})
	if d.Deactivator != nil {
		deact = d.Deactivator
	}

	chats, err := resolver.Recipients(ctx, e)
	if err != nil {
		return err
	}
	if len(chats) == 0 {
		return nil
	}

	text := formatEvent(d.Renderer, e, d.Loc)
	var retryErr error
	for _, chatID := range chats {
		if d.Throttle != nil {
			d.Throttle.Wait()
		}
		serr := d.Sender.SendMessage(ctx, chatID, text)
		switch {
		case serr == nil:
			// доставлено
		case IsPermanent(serr):
			// Постоянный отказ: деактивируем подписку, НЕ ретраим этот chat (AC4). Сбой деактивации —
			// transient (попробуем весь батч снова), не маскируем.
			if derr := deact.Deactivate(ctx, chatID); derr != nil {
				retryErr = errors.Join(retryErr, derr)
			}
		default:
			// Transient (429/5xx/сеть): копим → возврат ошибки растит attempts у воркера (O-4).
			retryErr = errors.Join(retryErr, serr)
		}
	}
	return retryErr
}
