//go:build scrape

// Command interim-import — ВРЕМЕННЫЙ (Story 0.6, трек «Парсер-мост») импортёр scraped-лотов Астаны в
// проекцию lots ЧЕРЕЗ ТОТ ЖЕ ingest/decode, что и боевой ows (swap = один флаг Source, AC3). ЗА
// build-tag `scrape` — вне дефолтного билда, физически вне cmd/api/cmd/importer (AC2, go-list-страж).
//
// Запуск (ТОЛЬКО осознанно, §6.1/§6.4):
//
//	ASHYQQALA_INTERIM_SCRAPE=1 DATABASE_URL=postgres://... \
//	  go run -tags scrape ./tools/scrape/cmd/interim-import -max 200
//
// КРИТЕРИЙ УДАЛЕНИЯ: получен GOSZAKUP_TOKEN → ows-источник (Story 2.1) заменяет scrape; команда и
// пакет tools/scrape удаляются. Путь восстановления §6.1/§6.4 — Sprint Change Proposal 2026-06-20.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/ingest/decode"
	"ashyqqala/server/internal/store/gen"
	"ashyqqala/server/internal/store/projection"
	"ashyqqala/server/tools/scrape"
)

// interimFlag — обязательный гейт-флаг. Без него команда ОТКАЗЫВАЕТ (осознанность отклонения §6.1/§6.4).
const interimFlag = "ASHYQQALA_INTERIM_SCRAPE"

// interimEnabled — строгий гейт: ровно "=1" включает интерим-импорт (любое иное значение → выключено).
func interimEnabled(getenv func(string) string) bool { return getenv(interimFlag) == "1" }

// loudWarning — громкое предупреждение о временном отклонении §6.1/§6.4 и критерии удаления (AC1/Task 5).
func loudWarning(w io.Writer) {
	bar := strings.Repeat("=", 76)
	fmt.Fprintln(w, bar)
	fmt.Fprintln(w, "⚠️  ВРЕМЕННЫЙ SCRAPE-ИМПОРТ (Story 0.6, трек «Парсер-мост»)")
	fmt.Fprintln(w, "    Парсинг публичного портала goszakup.gov.kz — осознанное ВРЕМЕННОЕ отклонение")
	fmt.Fprintln(w, "    от §6.1/§6.4 «официальный канал, не парсинг» (Sprint Change Proposal 2026-06-20).")
	fmt.Fprintln(w, "    Только ЛОТЫ Астаны; выборка keyword-смещена → направления помечать «предв.».")
	fmt.Fprintln(w, "    КРИТЕРИЙ УДАЛЕНИЯ: получен GOSZAKUP_TOKEN → swap Source scrape→ows (Story 2.1);")
	fmt.Fprintln(w, "    этот импортёр и пакет tools/scrape удаляются (downstream не меняется, AC3).")
	fmt.Fprintln(w, bar)
}

func main() {
	max := flag.Int("max", 0, "максимум лотов (0 = без лимита)")
	flag.Parse()

	if !interimEnabled(os.Getenv) {
		fmt.Fprintf(os.Stderr, "ОТКАЗ: интерим-импорт scrape выключен. Установите %s=1 (осознанное временное отклонение §6.1/§6.4).\n", interimFlag)
		os.Exit(2)
	}
	loudWarning(os.Stderr)

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "ОШИБКА: DATABASE_URL пуст (нужна проекционная БД для записи lots)")
		os.Exit(2)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ОШИБКА подключения к БД:", err)
		os.Exit(1)
	}
	defer pool.Close()

	// fail-fast: pgxpool.New ленив — пингуем БД ДО живого парсинга портала (не дёргать портал зря
	// и не нарушать вежливость, если БД недоступна). Зеркалит cmd/api.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := pool.Ping(pingCtx); err != nil {
		cancel()
		fmt.Fprintln(os.Stderr, "ОШИБКА: БД недоступна (ping):", err)
		os.Exit(1)
	}
	cancel()

	// scrape → ЕДИНЫЙ decode (тот же путь, что у боевого ows) → проекция lots. schema_hash фиксирует
	// контракт полей scrape-записи: дрейф → ErrSchemaDrift (стоп, не тихий 0).
	src := scrape.New("", "", 0, 0)
	hash := decode.SchemaHash(scrape.RecordKeys)
	lots, err := decode.DecodeLotsFrom(src, hash, *max)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ОШИБКА scrape→decode:", err)
		os.Exit(1)
	}

	store := projection.NewLotStore(pool)
	for _, l := range lots {
		if err := store.UpsertLot(ctx, toParams(l)); err != nil {
			fmt.Fprintf(os.Stderr, "ОШИБКА UpsertLot %s: %v\n", l.GoszakupLotID, err)
			os.Exit(1)
		}
	}
	fmt.Printf("интерим-импорт завершён: лотов записано/обновлено %d (source=%s)\n", len(lots), src.Name())
}

// toParams — decode.Lot → sqlc UpsertLotParams; пустые поля → NULL (честное «нет данных»).
// Зеркалит projection/lots_integration_test.go (единый маппинг доменного лота в проекцию).
func toParams(l decode.Lot) gen.UpsertLotParams {
	p := gen.UpsertLotParams{GoszakupLotID: l.GoszakupLotID}
	if l.TitleRu != "" {
		p.TitleRu = pgtype.Text{String: l.TitleRu, Valid: true}
	}
	if l.TitleKk != "" {
		p.TitleKk = pgtype.Text{String: l.TitleKk, Valid: true}
	}
	if l.Amount != nil {
		p.Amount = pgtype.Int8{Int64: *l.Amount, Valid: true}
	}
	if l.KatoCode != "" {
		p.KatoCode = pgtype.Text{String: l.KatoCode, Valid: true}
	}
	return p
}
