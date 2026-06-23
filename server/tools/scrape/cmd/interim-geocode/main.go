//go:build scrape

// Command interim-geocode — ⏳ ВРЕМЕННЫЙ (Story 0.7, трек «Парсер-мост») batch-геокодер: читает scraped-лоты
// из проекции `lots` (0.6), геокодит адрес (title_ru) через Nominatim и пишет координаты в interim_geo_lots
// для ранней карты (0.8). Стадия `geo` конвейера decode→lots→geo→map (downstream не меняется при swap на ows).
// ЗА build-tag `scrape` — вне дефолтного билда, физически вне cmd/api/cmd/importer (AC3, go-list-страж).
//
// Запуск (ТОЛЬКО осознанно, §6.1/§6.4):
//
//	ASHYQQALA_INTERIM_SCRAPE=1 DATABASE_URL=postgres://... [NOMINATIM_URL=http://localhost:8080] \
//	  go run -tags scrape ./tools/scrape/cmd/interim-geocode
//
// КРИТЕРИЙ УДАЛЕНИЯ: получен GOSZAKUP_TOKEN → канонический geo_objects/импорт (Epic 2/3) заменяет интерим;
// эта команда и interim_geo_lots удаляются (механизм internal/geo и данные мигрируют, downstream — нет).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/geo"
	"ashyqqala/server/internal/store/gen"
	"ashyqqala/server/internal/store/projection"
)

// interimFlag — обязательный гейт-флаг (тот же трек «Парсер-мост», что и 0.6). Без него команда отказывает.
const interimFlag = "ASHYQQALA_INTERIM_SCRAPE"

func interimEnabled(getenv func(string) string) bool { return getenv(interimFlag) == "1" }

// loudWarning — громкое предупреждение о временном отклонении §6.1/§6.4 + критерий удаления (AC3).
func loudWarning(w io.Writer) {
	bar := strings.Repeat("=", 76)
	fmt.Fprintln(w, bar)
	fmt.Fprintln(w, "⚠️  ВРЕМЕННЫЙ ГЕОКОДИНГ scraped-лотов (Story 0.7, трек «Парсер-мост»)")
	fmt.Fprintln(w, "    Геокодит лоты, добытые парсингом портала — продолжение временного отклонения")
	fmt.Fprintln(w, "    §6.1/§6.4 (Sprint Change Proposal 2026-06-20). Выборка keyword-смещена (keyword-bias) → доля")
	fmt.Fprintln(w, "    автопокрытия НЕпоказательна, помечать «предв.» (это НЕ гейт FR-6 — тот на живых данных).")
	fmt.Fprintln(w, "    КРИТЕРИЙ УДАЛЕНИЯ: получен GOSZAKUP_TOKEN → канонический geo_objects (Epic 3) заменяет;")
	fmt.Fprintln(w, "    эта команда и interim_geo_lots удаляются (internal/geo переиспользуется, downstream — нет).")
	fmt.Fprintln(w, bar)
}

// geoUpserter — узкий интерфейс записи гео (projection.GeoLotStore удовлетворяет). Для тестируемости цикла без БД.
type geoUpserter interface {
	UpsertGeoLot(ctx context.Context, p gen.UpsertGeoLotParams) error
}

// coverageStats — итог batch-прогона: автопокрытие как ФАКТ (не гейт). Samples — образцы названий
// негеокодированных лотов для честной строки «систематические пропуски» (AC1/Task 5, аналог Шага B+).
type coverageStats struct {
	Matched, Unmatched, Errors, Total int
	Samples                           []string
}

const maxMissSamples = 10

// addr — адресная строка лота для геокодера: title_ru + подсказка «, Астана» (viewbox/countrycodes уже сужают).
// Пусто, если у лота нет title (тогда — honest unmatched, не геокодим).
func addr(lot gen.Lot) string {
	if !lot.TitleRu.Valid || strings.TrimSpace(lot.TitleRu.String) == "" {
		return ""
	}
	return lot.TitleRu.String + ", Астана"
}

// toGeoParams — ЧИСТЫЙ маппер lot+результат → UpsertGeoLotParams. matched → lat/lon Valid + status auto;
// unmatched → lat/lon Valid=false (NULL), status unmatched. НИКОГДА не 0,0 за «без точки» (гардрейл AC2).
// query — РЕАЛЬНАЯ строка, поданная геокодеру → хранится в address_text (воспроизводимость).
func toGeoParams(lot gen.Lot, query string, lat, lon float64, matched bool) gen.UpsertGeoLotParams {
	p := gen.UpsertGeoLotParams{
		GoszakupLotID: lot.GoszakupLotID,
		KatoCode:      lot.KatoCode,
		// Confidence Valid=false (NULL) — importance не парсится в интериме (Story 0.7 Q6).
	}
	if query != "" {
		p.AddressText = pgtype.Text{String: query, Valid: true}
	}
	if matched {
		p.GeocodeStatus = "auto"
		p.Lat = pgtype.Float8{Float64: lat, Valid: true}
		p.Lon = pgtype.Float8{Float64: lon, Valid: true}
	} else {
		p.GeocodeStatus = "unmatched" // lat/lon Valid=false → NULL
	}
	return p
}

// geocodeLots — цикл: для каждого лота геокодит адрес и пишет гео-результат. Ошибка геокодера на лоте —
// логируется и считается как unmatched+error (НЕ обрушивает весь batch). ctx прерывает цикл и HTTP
// (Ctrl-C/таймаут); уже записанное идемпотентно. diag — поток диагностики (stderr).
func geocodeLots(ctx context.Context, lots []gen.Lot, gc geo.Geocoder, store geoUpserter, delay time.Duration, diag io.Writer) (coverageStats, error) {
	st := coverageStats{Total: len(lots)}
	for _, lot := range lots {
		if err := ctx.Err(); err != nil {
			return st, err // отмена/таймаут — выходим честно (записанное идемпотентно)
		}
		matched, called := false, false
		var lat, lon float64
		q := addr(lot)
		if q != "" {
			called = true
			la, lo, ok, err := gc.Geocode(ctx, q)
			if err != nil {
				st.Errors++
				fmt.Fprintf(diag, "[warn] геокод %s: %v\n", lot.GoszakupLotID, err)
			} else if ok {
				matched, lat, lon = true, la, lo
			}
		}
		if err := store.UpsertGeoLot(ctx, toGeoParams(lot, q, lat, lon, matched)); err != nil {
			return st, fmt.Errorf("UpsertGeoLot %s: %w", lot.GoszakupLotID, err)
		}
		if matched {
			st.Matched++
		} else {
			st.Unmatched++
			if len(st.Samples) < maxMissSamples { // образец пропуска (название лота) для распределения
				s := lot.GoszakupLotID
				if lot.TitleRu.Valid && lot.TitleRu.String != "" {
					s = lot.TitleRu.String
				}
				st.Samples = append(st.Samples, s)
			}
		}
		if called && delay > 0 { // пауза вежливости только когда был сетевой запрос
			time.Sleep(delay)
		}
	}
	return st, nil
}

func main() {
	max := flag.Int("max", 0, "максимум лотов (0 = без лимита; вежливость к публичному Nominatim)")
	flag.Parse()

	if !interimEnabled(os.Getenv) {
		fmt.Fprintf(os.Stderr, "ОТКАЗ: интерим-геокодинг выключен. Установите %s=1 (осознанное временное отклонение §6.1/§6.4).\n", interimFlag)
		os.Exit(2)
	}
	loudWarning(os.Stderr)

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "ОШИБКА: DATABASE_URL пуст (нужна БД с lots + interim_geo_lots)")
		os.Exit(2)
	}

	// Ctrl-C / SIGTERM прерывают долгий polite-batch чисто (записанное идемпотентно по goszakup_lot_id).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ОШИБКА подключения к БД:", err)
		os.Exit(1)
	}
	defer pool.Close()

	// fail-fast: пингуем БД ДО живого геокодинга (не дёргать Nominatim зря, если БД недоступна).
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := pool.Ping(pingCtx); err != nil {
		cancel()
		fmt.Fprintln(os.Stderr, "ОШИБКА: БД недоступна (ping):", err)
		os.Exit(1)
	}
	cancel()

	lots, err := projection.NewLotStore(pool).ListLots(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ОШИБКА чтения lots:", err)
		os.Exit(1)
	}
	if len(lots) == 0 {
		fmt.Fprintln(os.Stderr, "нет лотов в проекции lots (Story 0.6 не наполнила?) — нечего геокодить.")
		return
	}
	if *max > 0 && len(lots) > *max {
		fmt.Fprintf(os.Stderr, "[info] -max=%d: геокодим первые %d из %d лотов.\n", *max, *max, len(lots))
		lots = lots[:*max]
	}

	// NOMINATIM_URL пуст → публичный Nominatim (≤1 req/sec). Self-host (AR-22) → без публичной паузы.
	gc := geo.NewNominatim(os.Getenv("NOMINATIM_URL"), "", geo.AstanaViewbox, true)
	delay := geo.PoliteDelayDefault
	if os.Getenv("NOMINATIM_URL") != "" {
		delay = 0
	}

	st, err := geocodeLots(ctx, lots, gc, projection.NewGeoLotStore(pool), delay, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ОШИБКА/прерывание гео:", err)
		os.Exit(1)
	}

	// ОПЕРАЦИОННЫЙ сбой (НЕ coverage-гейт): все лоты дали ошибку → геокодер недоступен, это НЕ честное 0%.
	if st.Total > 0 && st.Errors == st.Total {
		fmt.Fprintln(os.Stderr, "ОШИБКА: все лоты дали ошибку геокодера — похоже, геокодер недоступен (это НЕ честное 0% автопокрытие).")
		os.Exit(1)
	}

	// Автопокрытие — ФАКТ (НЕ гейт, НЕ exit-код на пороге): число + честная оговорка keyword-bias.
	// «без точки» включает ошибки (errors ⊆ unmatched) — явно помечено, чтобы счётчики не читались как непересекающиеся.
	cov := 0.0
	if st.Total > 0 {
		cov = float64(st.Matched) / float64(st.Total)
	}
	fmt.Printf("интерим-геокодинг: лотов=%d, сматчено=%d, без точки=%d (вкл. ошибок=%d), автопокрытие=%.1f%% (ПРЕДВ., keyword-bias — НЕ гейт FR-6)\n",
		st.Total, st.Matched, st.Unmatched, st.Errors, cov*100)

	// Систематические пропуски — отдельной строкой (AC1/Task 5, аналог Шага B+): количество + примеры названий
	// (видно, КАКОЙ класс адресов систематически не матчится). На stderr — диагностика, stdout остаётся сводкой.
	if st.Unmatched > 0 {
		fmt.Fprintf(os.Stderr, "систематические пропуски: %d лотов без точки; примеры названий: %s\n",
			st.Unmatched, strings.Join(st.Samples, " | "))
	}
}
