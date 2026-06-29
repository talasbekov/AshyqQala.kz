package outbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/store/gen"
)

// Enqueue пишет конверт события в notifications_outbox В ТРАНЗАКЦИИ ВЫЗЫВАЮЩЕГО (q — gen.DBTX, т.е. pgx.Tx
// бизнес-операции), чтобы запись события шла АТОМАРНО с бизнес-объектом (O-1: откат tx вызывающего ⇒ нет
// ни бизнес-строки, ни события — как orgnorm.Apply / BenchmarkStore.ReplaceSnapshot). Дедуп (O-3):
// ON CONFLICT (event_id) DO NOTHING — повторный Enqueue того же события no-op (не двоит у получателя).
func Enqueue(ctx context.Context, q gen.DBTX, e Event) error {
	if q == nil {
		return fmt.Errorf("outbox enqueue: tx (gen.DBTX) не задана")
	}
	if e.EventID.IsZero() {
		return fmt.Errorf("outbox enqueue: пустой event_id (нужен детерминированный id эмиттера, см. NewEventID)")
	}
	if e.Type == "" {
		return fmt.Errorf("outbox enqueue: пустой type (ожидается <aggregate>.<event>)")
	}
	if e.SubjectRef == "" {
		return fmt.Errorf("outbox enqueue: пустой subject_ref (ожидается URN <entity>:<public_id>, см. SubjectURN)")
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("outbox enqueue: нулевой occurred_at (честность: не пишем фабрикованный 0001-01-01)")
	}
	payload := e.Payload
	switch {
	case len(payload) == 0:
		payload = []byte("{}") // честный пустой объект: НЕ NULL и НЕ пустой slice (пустой → невалидный JSONB)
	case !json.Valid(payload):
		return fmt.Errorf("outbox enqueue: payload не валидный JSON (колонка JSONB)")
	case bytes.TrimSpace(payload)[0] != '{':
		// Конверт payload по контракту — JSON-ОБЪЕКТ {...} (колонка DEFAULT '{}'::jsonb). Валидный, но
		// не-объект (число/массив/строка) минул бы json.Valid и тихо лёг бы в JSONB → отвергаем.
		return fmt.Errorf("outbox enqueue: payload должен быть JSON-объектом {...} (JSONB-конверт)")
	}
	v := e.V
	if v == 0 {
		v = 1 // дефолт версии конверта (совпадает с DEFAULT 1 колонки v)
	}
	return gen.New(q).EnqueueEvent(ctx, gen.EnqueueEventParams{
		EventID:    pgtype.UUID{Bytes: e.EventID, Valid: true},
		Type:       e.Type,
		OccurredAt: pgtype.Timestamptz{Time: e.OccurredAt, Valid: true},
		SubjectRef: e.SubjectRef,
		Payload:    payload,
		V:          int32(v),
	})
}
