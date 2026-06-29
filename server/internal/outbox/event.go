package outbox

import (
	"crypto/sha1" // UUIDv5 (RFC 4122 name-based) — детерминизм id, НЕ криптостойкость
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

// Имена типов событий (<aggregate>.<event>, lower-dot). В S-0 реальных эмиттеров НЕТ (LoggingDispatcher
// только логирует) — это forward-каталог для Epic 7. Реестр event_types + перекрёстный тест
// «event_types == эмиттеры» (architecture.md:645,792) откладываются на Epic 7, когда появятся реальные
// эмиттеры/Telegram; в S-0 имена зафиксированы здесь КОНСТАНТАМИ (Вопрос №1 — рекомендация принята).
const (
	TypeFlagRaised      = "flag.raised"
	TypeContractCreated = "contract.created"
)

// outboxNamespace — фиксированный namespace для UUIDv5 событий outbox (RFC 4122). Любой стабильный
// 16-байтный идентификатор; гарантирует, что NewEventID детерминирован между процессами и ре-импортами.
var outboxNamespace = [16]byte{
	0xa5, 0x29, 0x71, 0x0b, 0x2e, 0x47, 0x4c, 0x6f,
	0x8a, 0x71, 0x6f, 0x75, 0x74, 0x62, 0x6f, 0x78,
}

// EventID — 128-битный идентификатор события (UUID). Детерминируется эмиттером (NewEventID): повторная
// эмиссия ОДНОГО бизнес-события даёт тот же id → дедуп (O-3). Хранится в UUID-колонке event_id.
type EventID [16]byte

// NewEventID — ДЕТЕРМИНИРОВАННЫЙ UUIDv5 (name-based, SHA-1, RFC 4122) из частей: тот же вход → тот же id.
// Это «детерминированный id эмиттера» (Dev Notes §3): повтор события дедуплицируем (O-3) БЕЗ БД-random.
// stdlib-only (без uuid-зависимости). Части склеиваются через 0x1f (разделитель не даёт спутать границы:
// {"a","bc"} != {"ab","c"}).
func NewEventID(parts ...string) EventID {
	h := sha1.New()
	h.Write(outboxNamespace[:])
	h.Write([]byte(strings.Join(parts, "\x1f")))
	sum := h.Sum(nil)
	var id EventID
	copy(id[:], sum[:16])
	id[6] = (id[6] & 0x0f) | 0x50 // версия 5
	id[8] = (id[8] & 0x3f) | 0x80 // вариант RFC 4122
	return id
}

// IsZero — id не задан (нулевой). Enqueue отвергает нулевой id (нужен детерминированный id эмиттера).
func (id EventID) IsZero() bool { return id == EventID{} }

// String — канонический вид 8-4-4-4-12 (для логов/диагностики).
func (id EventID) String() string {
	var b [36]byte
	hex.Encode(b[0:8], id[0:4])
	b[8] = '-'
	hex.Encode(b[9:13], id[4:6])
	b[13] = '-'
	hex.Encode(b[14:18], id[6:8])
	b[18] = '-'
	hex.Encode(b[19:23], id[8:10])
	b[23] = '-'
	hex.Encode(b[24:36], id[10:16])
	return string(b[:])
}

// SubjectURN — URN субъекта события: <entity>:<public_id> (architecture.md:577). entity — singular
// (contract/organization/flag); public_id — ПУБЛИЧНЫЙ natural goszakup id (стабилен между ре-импортами,
// НЕ внутренний bigint; прецедент evidence_export 5.6).
func SubjectURN(entity, publicID string) string { return entity + ":" + publicID }

// Event — нейтральный конверт outbox (architecture.md#Communication Patterns 577-579). Несёт id/type/ref,
// НЕ прозу флага: нейтральность достигается через render на поверхности доставки (bot, Epic 7); здесь —
// только машиночитаемый конверт.
type Event struct {
	EventID    EventID         // дедуп-ключ (O-3); детерминируется эмиттером
	Type       string          // <aggregate>.<event>, lower-dot (TypeFlagRaised, …)
	OccurredAt time.Time       // момент бизнес-события (от эмиттера; в тестах — clock.Fixed)
	SubjectRef string          // URN <entity>:<public_id> (SubjectURN)
	Payload    json.RawMessage // нейтральный payload (id/ref); nil → '{}' при записи
	V          int             // версия payload (0 → 1 при записи)
}
