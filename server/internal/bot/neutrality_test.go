package bot

import (
	"encoding/json"
	"strings"
	"testing"

	"ashyqqala/server/internal/outbox"
	"ashyqqala/server/internal/registry"
)

// flagIDs — флаги MVP (Epic 4), чья проза идёт в Telegram-уведомление.
var flagIDs = []string{"single_participant", "price_per_km", "monopoly", "rnu"}

// TestTelegramNeutrality_FinalString (AC3) — per-surface нейтральность на ФИНАЛЬНОЙ строке (ПОСЛЕ MarkdownV2-
// экранирования / обрезки / эмодзи), НЕ на сырой прозе до диспатча. Для всех flag_state × обе локали:
// FindTaboo(финал) == ∅. Проза идёт ТОЛЬКО через render (formatEvent) — ноль литералов в боте (AC2).
func TestTelegramNeutrality_FinalString(t *testing.T) {
	rd := newRenderer(t)
	for _, loc := range registry.AllLocales() {
		for _, fid := range flagIDs {
			for _, fs := range registry.AllFlagStates() {
				payload, _ := json.Marshal(flagPayload{FlagID: fid, FlagState: string(fs)})
				e := outbox.Event{Type: outbox.TypeFlagRaised, Payload: payload}
				final := formatEvent(rd, e, loc)
				if final == "" {
					t.Errorf("[%s/%s/%s] пустой финал", loc, fid, fs)
				}
				if hits := rd.Reg.FindTaboo(loc, final); len(hits) > 0 {
					t.Errorf("[%s/%s/%s] taboo %v в финале: %q", loc, fid, fs, hits, final)
				}
				// Дыра-страж (review Edge#9c): нейтральность ГАРАНТИРУЕТСЯ и на СЫРОЙ прозе (до MarkdownV2-
				// экранирования) — иначе escaping reserved-символа ВНУТРИ taboo-корня (напр. «анти\-…») скрыл
				// бы его от FindTaboo на финале (substring по нормализованному тексту сохраняет '\').
				if hits := rd.Reg.FindTaboo(loc, renderEvent(rd, e, loc)); len(hits) > 0 {
					t.Errorf("[%s/%s/%s] taboo %v в СЫРОЙ прозе: %q", loc, fid, fs, hits, renderEvent(rd, e, loc))
				}
			}
		}
	}
	// contract.created (и неизвестный тип) → нейтральная рамка, тоже без taboo.
	for _, loc := range registry.AllLocales() {
		e := outbox.Event{Type: outbox.TypeContractCreated}
		final := formatEvent(rd, e, loc)
		if hits := rd.Reg.FindTaboo(loc, final); len(hits) > 0 {
			t.Errorf("[%s/contract.created] taboo %v: %q", loc, hits, final)
		}
	}
}

// TestTelegramNeutrality_NegativeControl — страж обязан КРАСНЕТЬ: taboo-матчер ловит запрещённый корень на
// ФИНАЛЬНОЙ (отформатированной) строке в ОБЕИХ локалях — экранирование/эмодзи не маскируют taboo. Иначе
// per-surface нейтральность фиктивна ([[guards-must-prove-red]]).
func TestTelegramNeutrality_NegativeControl(t *testing.T) {
	rd := newRenderer(t)
	if hits := rd.Reg.FindTaboo(registry.RU, FormatTelegram("это нарушение в контракте")); len(hits) == 0 {
		t.Error("negative-control (ru): FindTaboo не покраснел на «нарушение» в финале — страж фиктивен")
	}
	if hits := rd.Reg.FindTaboo(registry.KK, FormatTelegram("келісімшарттағы бұзушылық")); len(hits) == 0 {
		t.Error("negative-control (kk): FindTaboo не покраснел на «бұзушылық» в финале — kk-страж фиктивен")
	}
}

// TestRenderEvent_ProseViaRender (AC2) — проза события идёт через render (FlagLine): финал несёт summary
// флага из glossary (не пустой, не литерал в боте).
func TestRenderEvent_ProseViaRender(t *testing.T) {
	rd := newRenderer(t)
	payload, _ := json.Marshal(flagPayload{FlagID: "price_per_km", FlagState: string(registry.FlagRaised)})
	e := outbox.Event{Type: outbox.TypeFlagRaised, Payload: payload}
	final := formatEvent(rd, e, registry.RU)
	// Сравниваем с ЭКРАНИРОВАННОЙ FlagLine (финал MarkdownV2-экранирован) — иначе Contains прошёл бы лишь
	// по случайности (если в summary нет reserved-символов). Review Blind#4.
	want := escapeMarkdownV2(rd.FlagLine("price_per_km", registry.FlagRaised, registry.RU))
	if !strings.Contains(final, want) {
		t.Errorf("финал не несёт экранированную render.FlagLine: финал=%q, escaped=%q", final, want)
	}
}
