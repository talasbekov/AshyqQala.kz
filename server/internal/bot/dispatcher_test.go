package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"ashyqqala/server/internal/outbox"
	"ashyqqala/server/internal/registry"
)

type sentMsg struct {
	chatID int64
	text   string
}

type mockSender struct {
	sent []sentMsg
	errs map[int64]error // ошибка на конкретный chat
	err  error           // глобальная ошибка (все chat)
}

func (m *mockSender) SendMessage(_ context.Context, chatID int64, text string) error {
	m.sent = append(m.sent, sentMsg{chatID, text})
	if m.errs != nil {
		if e, ok := m.errs[chatID]; ok {
			return e
		}
	}
	return m.err
}

type staticResolver struct {
	chats []int64
	err   error
}

func (s staticResolver) Recipients(context.Context, outbox.Event) ([]int64, error) {
	return s.chats, s.err
}

type recDeactivator struct {
	chats []int64
	err   error
}

func (r *recDeactivator) Deactivate(_ context.Context, chatID int64) error {
	r.chats = append(r.chats, chatID)
	return r.err
}

func flagEvent(t *testing.T) outbox.Event {
	t.Helper()
	payload, _ := json.Marshal(flagPayload{FlagID: "price_per_km", FlagState: string(registry.FlagRaised)})
	return outbox.Event{Type: outbox.TypeFlagRaised, Payload: payload}
}

// TestDispatcher_NoRecipients (AC1) — нет подписчиков (дефолт NoRecipients, таблицы 7.2 ещё нет) → nil,
// транспорт НЕ вызывается (честно никому не шлём).
func TestDispatcher_NoRecipients(t *testing.T) {
	s := &mockSender{}
	d := TelegramDispatcher{Sender: s, Renderer: newRenderer(t), Loc: registry.RU}
	if err := d.Send(context.Background(), flagEvent(t)); err != nil {
		t.Fatalf("Send без подписчиков: %v", err)
	}
	if len(s.sent) != 0 {
		t.Errorf("транспорт вызван без подписчиков: %v", s.sent)
	}
}

// TestDispatcher_DeliversToRecipients (AC1/AC2) — fan-out по chat_id; текст через render (эмодзи-префикс).
func TestDispatcher_DeliversToRecipients(t *testing.T) {
	s := &mockSender{}
	d := TelegramDispatcher{
		Sender:   s,
		Resolver: staticResolver{chats: []int64{10, 20}},
		Renderer: newRenderer(t),
		Loc:      registry.RU,
	}
	if err := d.Send(context.Background(), flagEvent(t)); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(s.sent) != 2 {
		t.Fatalf("ожидалось 2 доставки, получено %d", len(s.sent))
	}
	for _, m := range s.sent {
		if !strings.HasPrefix(m.text, notifyEmoji+" ") {
			t.Errorf("chat %d: нет эмодзи-префикса render-прозы: %q", m.chatID, m.text)
		}
	}
}

// TestDispatcher_TransientError (AC4) — transient-ошибка (429/5xx/сеть) → возврат ошибки (воркер растит
// attempts O-4), деактивации НЕТ.
func TestDispatcher_TransientError(t *testing.T) {
	dz := &recDeactivator{}
	s := &mockSender{err: errors.New("429 too many requests")}
	d := TelegramDispatcher{
		Sender:      s,
		Resolver:    staticResolver{chats: []int64{10}},
		Deactivator: dz,
		Renderer:    newRenderer(t),
		Loc:         registry.RU,
	}
	if err := d.Send(context.Background(), flagEvent(t)); err == nil {
		t.Error("transient-ошибка должна вернуться (воркер растит attempts)")
	}
	if len(dz.chats) != 0 {
		t.Errorf("transient НЕ должен деактивировать: %v", dz.chats)
	}
}

// TestDispatcher_PermanentDeactivates (AC4) — постоянный отказ (бот заблокирован) → деактивация подписки +
// NIL (НЕ ретрай: poison-chat не крутится вечно).
func TestDispatcher_PermanentDeactivates(t *testing.T) {
	dz := &recDeactivator{}
	s := &mockSender{err: fmt.Errorf("forbidden: %w", ErrChatUnavailable)}
	d := TelegramDispatcher{
		Sender:      s,
		Resolver:    staticResolver{chats: []int64{77}},
		Deactivator: dz,
		Renderer:    newRenderer(t),
		Loc:         registry.RU,
	}
	if err := d.Send(context.Background(), flagEvent(t)); err != nil {
		t.Errorf("постоянный отказ НЕ должен возвращать ошибку (нет ретрая): %v", err)
	}
	if len(dz.chats) != 1 || dz.chats[0] != 77 {
		t.Errorf("ожидалась деактивация chat 77, получено %v", dz.chats)
	}
}

// TestDispatcher_MixedFailures (AC4) — chat ok / permanent / transient: деактивируется только permanent,
// возвращается ошибка из-за transient (батч ретраится).
func TestDispatcher_MixedFailures(t *testing.T) {
	dz := &recDeactivator{}
	s := &mockSender{errs: map[int64]error{
		2: fmt.Errorf("blocked: %w", ErrChatUnavailable),
		3: errors.New("503 service unavailable"),
	}}
	d := TelegramDispatcher{
		Sender:      s,
		Resolver:    staticResolver{chats: []int64{1, 2, 3}},
		Deactivator: dz,
		Renderer:    newRenderer(t),
		Loc:         registry.RU,
	}
	err := d.Send(context.Background(), flagEvent(t))
	if err == nil {
		t.Error("ожидалась ошибка из-за transient chat 3")
	}
	if len(dz.chats) != 1 || dz.chats[0] != 2 {
		t.Errorf("должен деактивироваться только permanent chat 2, получено %v", dz.chats)
	}
}

// TestDispatcher_NilSender — без транспорта → честная ошибка (не паника).
func TestDispatcher_NilSender(t *testing.T) {
	d := TelegramDispatcher{Resolver: staticResolver{chats: []int64{1}}, Renderer: newRenderer(t)}
	if err := d.Send(context.Background(), flagEvent(t)); err == nil {
		t.Error("nil Sender должен дать ошибку")
	}
}

// manualClock — часы, продвигаемые мок-Sleep'ом (симуляция реального хода времени во сне). Нужны, чтобы
// тест throttle ловил недо-троттл: с замороженными Fixed-часами фикс `last=now+wait` неотличим от бага.
type manualClock struct{ t time.Time }

func (c *manualClock) Now() time.Time { return c.t }

// TestThrottle_MinGap (AC1) — троттл вежливости: первая отправка без ожидания, последующие ждут MinGap.
// Часы продвигаются сном (реалистично) → если бы `last` фиксировался ДО сна (баг), 3-я отправка увидела бы
// gap==MinGap и НЕ ждала бы (получилось бы 1 ожидание). Корректный фикс даёт РОВНО 2 ожидания по MinGap.
func TestThrottle_MinGap(t *testing.T) {
	var slept []time.Duration
	clk := &manualClock{t: time.Unix(1000, 0)}
	th := &Throttle{
		Clock:  clk,
		MinGap: 50 * time.Millisecond,
		Sleep:  func(d time.Duration) { slept = append(slept, d); clk.t = clk.t.Add(d) }, // сон продвигает часы
	}
	th.Wait() // первая — без ожидания
	th.Wait() // вторая — ждёт MinGap
	th.Wait() // третья — ждёт MinGap (gap «съеден» сном, не back-to-back)
	if len(slept) != 2 {
		t.Fatalf("ожидалось 2 ожидания (2-я и 3-я отправки), получено %d: %v", len(slept), slept)
	}
	for i, d := range slept {
		if d != 50*time.Millisecond {
			t.Errorf("ожидание[%d] = %v; want 50ms", i, d)
		}
	}
	// Контроль: если внешне время уже прошло ≥ MinGap, ожидания НЕТ.
	clk.t = clk.t.Add(time.Second)
	before := len(slept)
	th.Wait()
	if len(slept) != before {
		t.Errorf("при прошедшем ≥MinGap не должно быть ожидания, добавилось: %v", slept[before:])
	}
}
