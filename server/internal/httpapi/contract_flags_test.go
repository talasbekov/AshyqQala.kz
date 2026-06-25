package httpapi

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/store/gen"
)

func activeFlag(ft, ver, ev string) gen.RiskFlag {
	return gen.RiskFlag{
		FlagType:           ft,
		ContractID:         pgtype.Int8{Int64: 1, Valid: true},
		IsActive:           true,
		Evidence:           []byte(ev),
		MethodologyVersion: ver,
	}
}

func clearedFlag(ft, ver string) gen.RiskFlag {
	return gen.RiskFlag{
		FlagType:           ft,
		ContractID:         pgtype.Int8{Int64: 1, Valid: true},
		IsActive:           false,
		Evidence:           []byte(`{"stale":true}`), // снятая строка может хранить старый evidence — НЕ публикуем его
		MethodologyVersion: ver,
	}
}

func flagByID(flags []ContractFlagDTO, id string) (ContractFlagDTO, bool) {
	for _, f := range flags {
		if f.FlagID == id {
			return f, true
		}
	}
	return ContractFlagDTO{}, false
}

// TestResolveContractFlags_HonestReconstruction — ядро AC4: реконструкция состояний на ЧТЕНИИ.
// Стор хранит только raised/снятые строки; правило должно ОТЛИЧАТЬ «нет данных» от «проверено, сигнала нет».
func TestResolveContractFlags_HonestReconstruction(t *testing.T) {
	rows := []gen.RiskFlag{
		activeFlag("single_participant", "v1.0", `{"participant_count":1}`),
		clearedFlag("price_per_km", "v1.0"),
	}
	got := resolveContractFlags(rows)

	if len(got) != len(contractFlagTypes) {
		t.Fatalf("ожидался дескриптор на каждый contractFlagType (%d), got %d", len(contractFlagTypes), len(got))
	}

	sp, ok := flagByID(got, "single_participant")
	if !ok {
		t.Fatal("нет дескриптора single_participant")
	}
	if sp.State != registry.FlagRaised {
		t.Fatalf("активная строка → raised, got %q", sp.State)
	}
	if sp.MethodologyVersion.State != registry.StateOK || sp.MethodologyVersion.Value == nil || *sp.MethodologyVersion.Value != "v1.0" {
		t.Fatalf("raised: methodology_version должен быть value=v1.0/ok, got %+v", sp.MethodologyVersion)
	}
	if string(sp.Evidence) != `{"participant_count":1}` {
		t.Fatalf("raised: evidence пробрасывается сырым, got %s", sp.Evidence)
	}

	pk, ok := flagByID(got, "price_per_km")
	if !ok {
		t.Fatal("нет дескриптора price_per_km")
	}
	if pk.State != registry.FlagNotRaised {
		t.Fatalf("снятая строка → not_raised, got %q", pk.State)
	}
	if pk.Evidence != nil {
		t.Fatalf("not_raised: НЕ публикуем устаревший evidence, got %s", pk.Evidence)
	}
}

// TestResolveContractFlags_MissingRowIsInsufficientNotClean — NEGATIVE CONTROL (см. [[guards-must-prove-red]]).
// Несущий гардрейл честности: ОТСУТСТВИЕ строки флага НЕ должно превратиться в not_raised («всё чисто»). Если
// кто-то «оптимизирует» missing→not_raised, этот тест обязан ПОКРАСНЕТЬ. Доказывает РЕАЛЬНОЕ поведение правила,
// а не просто «вернулось 2 элемента».
func TestResolveContractFlags_MissingRowIsInsufficientNotClean(t *testing.T) {
	got := resolveContractFlags(nil) // строк нет вовсе

	if len(got) != len(contractFlagTypes) {
		t.Fatalf("ожидался дескриптор на каждый тип даже без строк, got %d", len(got))
	}
	for _, f := range got {
		if f.State == registry.FlagNotRaised {
			t.Fatalf("РЕГРЕСС честности: отсутствие строки %q дало not_raised (молчаливое «всё чисто») — должно быть insufficient_data", f.FlagID)
		}
		if f.State != registry.FlagInsufficientData {
			t.Fatalf("нет строки → insufficient_data, got %q для %q", f.State, f.FlagID)
		}
		if f.MethodologyVersion.State != registry.StateNoData || f.MethodologyVersion.Value != nil {
			t.Fatalf("insufficient: methodology_version честно no_data, got %+v", f.MethodologyVersion)
		}
		if f.Evidence != nil {
			t.Fatalf("insufficient: evidence должен быть null, got %s", f.Evidence)
		}
	}
}

// TestResolveContractFlags_DeterministicOrder — порядок дескрипторов стабилен (стабильность wire/golden).
func TestResolveContractFlags_DeterministicOrder(t *testing.T) {
	got := resolveContractFlags([]gen.RiskFlag{
		activeFlag("price_per_km", "v1.0", `{}`),
		activeFlag("single_participant", "v1.0", `{}`),
	})
	for i, ft := range contractFlagTypes {
		if got[i].FlagID != ft {
			t.Fatalf("порядок дескрипторов нестабилен: [%d] = %q, ожидалось %q", i, got[i].FlagID, ft)
		}
	}
}
