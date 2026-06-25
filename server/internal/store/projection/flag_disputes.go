package projection

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/metrics"
	"ashyqqala/server/internal/store/gen"
)

// Статусы верификации флага (AR-28, Story 4.6): raised → disputed → confirmed (флаг подтверждён валидным
// сигналом) | withdrawn (флаг отозван как ложный → некорректный для SM-C1).
const (
	DisputeRaised    = "raised"
	DisputeDisputed  = "disputed"
	DisputeConfirmed = "confirmed"
	DisputeWithdrawn = "withdrawn"
)

// FlagDisputeStore — доступ к таблице flag_disputes (статус верификации флагов; питает SM-C1).
type FlagDisputeStore struct {
	pool *pgxpool.Pool
}

// NewFlagDisputeStore — конструктор.
func NewFlagDisputeStore(pool *pgxpool.Pool) *FlagDisputeStore { return &FlagDisputeStore{pool: pool} }

// DisputeInput — вход записи диспута флага (денормализованный субъект для стабильной ссылки — AR-4).
type DisputeInput struct {
	RiskFlagID  int64
	FlagType    string
	SubjectType string
	SubjectID   int64
	Status      string
	Note        string
	SourceURL   string
}

// validFlagTypes / validSubjectTypes — допустимые домены денормализованного субъекта. Дублируют CHECK в
// migrations/0009 (defense in depth): ранняя доменная ошибка вместо сырой ошибки констрейнта БД (и защита, если
// запись когда-либо пойдёт мимо БД-CHECK). Пустую строку NOT NULL не ловит — отвергаем здесь.
var (
	validFlagTypes    = map[string]bool{"single_participant": true, "price_per_km": true, "monopoly": true, "rnu": true}
	validSubjectTypes = map[string]bool{"contract": true, "contractor": true}
)

// RecordDispute ИДЕМПОТЕНТНО фиксирует/обновляет диспут флага (один на risk_flag_id). SM-C1 НЕ инкрементится
// здесь: он DB-derived (см. SMC1Counts) — считывается на scrape /metrics из flag_disputes, restart-safe и
// race-free, идемпотентен к повтору/откату статуса (решение ревью 4.6: источник истины = БД, не in-process
// счётчик). Денорм-субъект/статус/положительность id валидируются заранее (доменная ошибка, не сырой CHECK).
func (s *FlagDisputeStore) RecordDispute(ctx context.Context, in DisputeInput) error {
	switch in.Status {
	case DisputeRaised, DisputeDisputed, DisputeConfirmed, DisputeWithdrawn:
	default:
		return fmt.Errorf("flag_disputes: неизвестный статус %q (ожидался raised|disputed|confirmed|withdrawn)", in.Status)
	}
	if !validFlagTypes[in.FlagType] {
		return fmt.Errorf("flag_disputes: неизвестный flag_type %q (ожидался single_participant|price_per_km|monopoly|rnu)", in.FlagType)
	}
	if !validSubjectTypes[in.SubjectType] {
		return fmt.Errorf("flag_disputes: неизвестный subject_type %q (ожидался contract|contractor)", in.SubjectType)
	}
	if in.RiskFlagID <= 0 || in.SubjectID <= 0 {
		return fmt.Errorf("flag_disputes: risk_flag_id=%d, subject_id=%d — оба должны быть > 0", in.RiskFlagID, in.SubjectID)
	}

	if err := gen.New(s.pool).UpsertFlagDispute(ctx, gen.UpsertFlagDisputeParams{
		RiskFlagID:  in.RiskFlagID,
		FlagType:    in.FlagType,
		SubjectType: in.SubjectType,
		SubjectID:   in.SubjectID,
		Status:      in.Status,
		Note:        pgtype.Text{String: in.Note, Valid: in.Note != ""},
		SourceUrl:   pgtype.Text{String: in.SourceURL, Valid: in.SourceURL != ""},
	}); err != nil {
		return fmt.Errorf("flag_disputes: upsert (flag %d): %w", in.RiskFlagID, err)
	}
	return nil
}

// Get — диспут по флагу (строка + признак наличия).
func (s *FlagDisputeStore) Get(ctx context.Context, riskFlagID int64) (gen.FlagDispute, bool, error) {
	row, err := gen.New(s.pool).GetFlagDispute(ctx, riskFlagID)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.FlagDispute{}, false, nil
	}
	if err != nil {
		return gen.FlagDispute{}, false, fmt.Errorf("flag_disputes get (flag %d): %w", riskFlagID, err)
	}
	return row, true, nil
}

// CountByStatus — число диспутов статуса (SM-C1: confirmed+withdrawn — знаменатель, withdrawn — числитель).
func (s *FlagDisputeStore) CountByStatus(ctx context.Context, status string) (int64, error) {
	return gen.New(s.pool).CountFlagDisputesByStatus(ctx, status)
}

// SMC1Counts — DB-derived срез SM-C1 из flag_disputes (источник истины для /metrics): reviewed = прошедшие ручную
// верификацию (confirmed|withdrawn), incorrect = признанные некорректными (withdrawn). Restart-safe, race-free,
// идемпотентно (повтор/откат статуса отражается счётом из БД, а не императивным инкрементом). Подаётся в
// metrics.RegisterSMC1 — вычисляется на каждом scrape /metrics.
func (s *FlagDisputeStore) SMC1Counts(ctx context.Context) (metrics.SMC1Counts, error) {
	confirmed, err := s.CountByStatus(ctx, DisputeConfirmed)
	if err != nil {
		return metrics.SMC1Counts{}, err
	}
	withdrawn, err := s.CountByStatus(ctx, DisputeWithdrawn)
	if err != nil {
		return metrics.SMC1Counts{}, err
	}
	return metrics.SMC1Counts{Reviewed: float64(confirmed + withdrawn), Incorrect: float64(withdrawn)}, nil
}
