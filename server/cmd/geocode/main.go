// Command geocode — канонический batch-геокодер (Story 3.1, AC1/AC2/AC3): читает контракты без гео-
// привязки (или с устаревшей auto), геокодит адрес через internal/geo (Nominatim, publicный или self-host),
// присваивает район по КАТО (точка → геометрическое членство ST_Contains; без точки → КАТО-префикс, зеркало
// 6.3) и кэширует результат в geo_objects (Nominatim после batch не дёргается, AR-6). Эфемерный batch
// (AR-22): контейнер гасится после прогона, никакой 24/7-зависимости.
//
// НЕ интерим: без build-tag, без гейт-флага/громкого warning (§6.1/§6.4 здесь неприменимы — официальный
// API-путь геокодинга, не парсинг портала). Физически вне tools/scrape.
//
// Запуск:
//
//	DATABASE_URL=postgres://... [NOMINATIM_URL=http://localhost:8080] go run ./cmd/geocode [-max=N]
//
// Пусто NOMINATIM_URL → публичный Nominatim (политика ≤1 req/sec, geo.PoliteDelayDefault). Self-host
// (AR-22, compose-профиль geocode) → без искусственной паузы.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/geo"
	"ashyqqala/server/internal/store/curation"
	"ashyqqala/server/internal/store/gen"
)

// contractSource — узкое чтение кандидатов геокодинга (удовлетворяет *gen.Queries напрямую — проекционное
// чтение contracts, отдельная роль от куратор-записи geoStore ниже, AR-4).
type contractSource interface {
	ListContractsForGeocode(ctx context.Context) ([]gen.ListContractsForGeocodeRow, error)
}

// geocoder — geo.Nominatim.GeocodeMatch (Match с confidence из importance, Task 2). Узкий интерфейс —
// тестируемость цикла без сети (0.7-паттерн).
type geocoder interface {
	GeocodeMatch(ctx context.Context, query string) (geo.Match, bool, error)
}

// geoStore — узкая запись/резолв геопривязки (curation.GeoObjectStore удовлетворяет). Для тестируемости
// цикла без БД.
type geoStore interface {
	UpsertGeoObject(ctx context.Context, p gen.UpsertGeoObjectParams) error
	FindDistrictIDByPoint(ctx context.Context, geomWKT string) (int64, error)
	FindDistrictIDByKATOPrefix(ctx context.Context, contractKato string) (int64, error)
}

// coverageStats — итог batch-прогона: автопокрытие как ФАКТ, НЕ гейт (живой ≥70%-вердикт FR-6/OQ-4 — на
// живых данных, Story 3.3). Samples — образцы пропусков для честной строки «систематические пропуски».
// Чистое разбиение (код-ревью): Matched+Unmatched+Errors == Total ВСЕГДА. Errors — контракты с
// транспортной/инфраструктурной ошибкой, которым НЕ писали строку (retryable следующим прогоном);
// Unmatched — ТОЛЬКО честный «адрес не распознан» (геокодер ответил, но не нашёл). Раньше (до фикса)
// ошибка геокодера тоже писалась как unmatched → контракт НАВСЕГДА выпадал из будущих прогонов
// (ListContractsForGeocode не пере-выбирает status=unmatched) — транзиентный сбой сети маскировался под
// «честно не найдено» и замораживался.
type coverageStats struct {
	Matched, Unmatched, Errors, Total int
	Samples                           []string
}

const maxMissSamples = 10

// addr — адресная строка контракта для геокодера: subject_ru + подсказка «, Астана» (viewbox/countrycodes
// уже сужают геокодер до Казахстана/Астаны), нормализованная (geo.NormalizeAddress — Task 2, долг 0.7
// deferred-work.md:128). Пусто, если у контракта нет subject_ru → honest unmatched, сеть не дёргаем.
func addr(c gen.ListContractsForGeocodeRow) string {
	if !c.SubjectRu.Valid || strings.TrimSpace(c.SubjectRu.String) == "" {
		return ""
	}
	return geo.NormalizeAddress(c.SubjectRu.String + ", Астана")
}

// wkt — WKT POINT(lon lat) для ST_GeomFromText. Batch-геокод по свободному адресу даёт ТОЛЬКО точку —
// LINESTRING (дороги, length_km) приходит вручную/Directus (3.2) или синтетик-seed, не этим пайплайном.
func wkt(lat, lon float64) string {
	return fmt.Sprintf("POINT(%g %g)", lon, lat)
}

// resolveDistrict — район по КАТО (AC2, «даже без точки»): при наличии точки — геометрическое членство
// (ST_Contains); иначе/не найдено геометрически — КАТО-префикс контракта (зеркало district.Catalog.
// NameByKATO/6.3). pgx.ErrNoRows на любом шаге = честное «нет совпавшего района» (districts частично
// заселены в S-0 — не все КАТО/полигоны подтверждены, Story 0.1/3.2) → district_id NULL, НЕ ошибка.
// Любая ДРУГАЯ ошибка (инфраструктурная) — наружу: вызывающий логирует и считает как error на контракте,
// НЕ маскирует под «района нет».
func resolveDistrict(ctx context.Context, store geoStore, matched bool, matchWKT, contractKato string) (pgtype.Int8, error) {
	if matched {
		id, err := store.FindDistrictIDByPoint(ctx, matchWKT)
		switch {
		case err == nil:
			return pgtype.Int8{Int64: id, Valid: true}, nil
		case errors.Is(err, pgx.ErrNoRows):
			// геометрически не нашли — честно падаем к КАТО-префиксу ниже.
		default:
			return pgtype.Int8{}, fmt.Errorf("FindDistrictIDByPoint: %w", err)
		}
	}
	if contractKato == "" {
		return pgtype.Int8{}, nil
	}
	id, err := store.FindDistrictIDByKATOPrefix(ctx, contractKato)
	switch {
	case err == nil:
		return pgtype.Int8{Int64: id, Valid: true}, nil
	case errors.Is(err, pgx.ErrNoRows):
		return pgtype.Int8{}, nil // честно: район ещё не импортирован/код не подтверждён (S-0)
	default:
		return pgtype.Int8{}, fmt.Errorf("FindDistrictIDByKATOPrefix: %w", err)
	}
}

// toGeoParams — ЧИСТЫЙ маппер контракт+результат → UpsertGeoObjectParams. matched → geom=WKT точки +
// confidence=importance, status=auto; unmatched → geom/confidence остаются NULL (НИКОГДА 0,0 — гардрейл
// AC2). length_km ВСЕГДА NULL — этот пайплайн даёт только точки, не линии (честно, не «длина=0»).
func toGeoParams(c gen.ListContractsForGeocodeRow, query string, m geo.Match, matched bool, districtID pgtype.Int8) gen.UpsertGeoObjectParams {
	p := gen.UpsertGeoObjectParams{
		ContractID: pgtype.Int8{Int64: c.ID, Valid: true},
		DistrictID: districtID,
		GeocodedBy: pgtype.Text{String: "nominatim", Valid: true},
	}
	if query != "" {
		p.AddressText = pgtype.Text{String: query, Valid: true}
	}
	if matched {
		p.GeocodeStatus = "auto"
		p.GeomWkt = pgtype.Text{String: wkt(m.Lat, m.Lon), Valid: true}
		if m.Confidence != nil {
			p.Confidence = pgtype.Float8{Float64: *m.Confidence, Valid: true}
		} // Confidence==nil (геокодер не отдал importance) → остаётся Invalid → NULL, НЕ выдуманный 0.0
	} else {
		p.GeocodeStatus = "unmatched" // GeomWkt/Confidence остаются Invalid → NULL (CHECK geom⟺unmatched доволен)
	}
	return p
}

// contractKato — КАТО-код контракта как plain string ("" если NULL/не задан).
func contractKato(c gen.ListContractsForGeocodeRow) string {
	if c.KatoCode.Valid {
		return c.KatoCode.String
	}
	return ""
}

// geocodeContracts — цикл: на контракт — геокод адреса → резолв района (AC2) → UPSERT geo_objects.
// ТРАНСПОРТНАЯ/ИНФРАСТРУКТУРНАЯ ошибка (геокодер недоступен, БД недоступна на резолве района) на ОДНОМ
// контракте — логируется, считается error, контракт ПРОПУСКАЕТСЯ БЕЗ ЗАПИСИ (retryable следующим
// прогоном), batch НЕ обрывается (0.7-паттерн: партиальный прогресс лучше полного отказа). Код-ревью
// нашёл: раньше транспортная ошибка писалась как status=unmatched — а ListContractsForGeocode НЕ
// пере-выбирает unmatched-строки → один сетевой сбой НАВСЕГДА замораживал контракт вне будущих прогонов.
// Честный «адрес не распознан» (геокодер ОТВЕТИЛ, но не нашёл) — единственный случай, который пишется
// как unmatched. ctx прерывает цикл честно — уже записанное идемпотентно по contract_id (ON CONFLICT).
func geocodeContracts(ctx context.Context, contracts []gen.ListContractsForGeocodeRow, gc geocoder, store geoStore, delay time.Duration, diag io.Writer) (coverageStats, error) {
	st := coverageStats{Total: len(contracts)}
	for _, c := range contracts {
		if err := ctx.Err(); err != nil {
			return st, err
		}
		matched, called := false, false
		var m geo.Match
		q := addr(c)
		geocodeErr := false
		if q != "" {
			called = true
			res, ok, err := gc.GeocodeMatch(ctx, q)
			if err != nil {
				geocodeErr = true
				st.Errors++
				fmt.Fprintf(diag, "[warn] геокод %s: %v (retryable следующим прогоном)\n", c.GoszakupContractID, err)
			} else if ok {
				matched, m = true, res
			}
		}
		if geocodeErr {
			if called && delay > 0 {
				if err := sleepCtx(ctx, delay); err != nil {
					return st, err
				}
			}
			continue // НЕ пишем unmatched за транспортную ошибку — честный «не найдено» ≠ «не смогли спросить»
		}

		var matchWKT string
		if matched {
			matchWKT = wkt(m.Lat, m.Lon)
		}
		districtID, dErr := resolveDistrict(ctx, store, matched, matchWKT, contractKato(c))
		if dErr != nil {
			// Инфраструктурная ошибка резолва района — ТА ЖЕ логика: не писать частично-честную строку
			// (matched-точка была бы верной, но district_id молча стал бы NULL вместо «не смогли узнать»).
			// Пропускаем контракт целиком — следующий прогон честно повторит и геокод, и резолв района.
			st.Errors++
			fmt.Fprintf(diag, "[warn] район %s: %v (retryable следующим прогоном)\n", c.GoszakupContractID, dErr)
			if called && delay > 0 {
				if err := sleepCtx(ctx, delay); err != nil {
					return st, err
				}
			}
			continue
		}

		if err := store.UpsertGeoObject(ctx, toGeoParams(c, q, m, matched, districtID)); err != nil {
			return st, fmt.Errorf("UpsertGeoObject %s: %w", c.GoszakupContractID, err)
		}
		if matched {
			st.Matched++
		} else {
			st.Unmatched++
			if len(st.Samples) < maxMissSamples {
				s := c.GoszakupContractID
				if c.SubjectRu.Valid && c.SubjectRu.String != "" {
					s = c.SubjectRu.String
				}
				st.Samples = append(st.Samples, s)
			}
		}
		if called && delay > 0 { // пауза вежливости только когда был сетевой запрос
			if err := sleepCtx(ctx, delay); err != nil {
				return st, err
			}
		}
	}
	return st, nil
}

// sleepCtx — прерываемая пауза вежливости: возвращает ctx.Err() немедленно при отмене вместо блокировки
// на полный delay (код-ревью: голый time.Sleep(delay) держал SIGTERM/Ctrl-C до конца паузы, несмотря на
// комментарий main() «Ctrl-C/SIGTERM прерывают чисто»).
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// run — тестируемое ядро команды: контракты → геокод → печать сводки → exit-код. Вынесено из main()
// (код-ревью: main() было единственным непротестированным местом — там жил double-counting баг до
// фикса geocodeContracts, а contractSource оставался мёртвым интерфейсом, никогда не принимаемым как
// параметр). Возвращает 0 (успех, включая честный «нет контрактов») или 1 (операционный сбой).
func run(ctx context.Context, contracts contractSource, gc geocoder, store geoStore, delay time.Duration, max int, stdout, stderr io.Writer) int {
	all, err := contracts.ListContractsForGeocode(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "ОШИБКА чтения контрактов:", err)
		return 1
	}
	if len(all) == 0 {
		fmt.Fprintln(stderr, "нет контрактов, ожидающих геокодинга (auto пере-выбираются; курация 3.2 — manual/verified/wrong_reported — и unmatched не пере-выбираются; либо contracts пуста).")
		return 0
	}
	if max > 0 && len(all) > max {
		fmt.Fprintf(stderr, "[info] -max=%d: геокодим первые %d из %d контрактов.\n", max, max, len(all))
		all = all[:max]
	}

	st, err := geocodeContracts(ctx, all, gc, store, delay, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "ОШИБКА/прерывание геокодинга:", err)
		return 1
	}

	// ОПЕРАЦИОННЫЙ сбой (НЕ coverage-гейт): все контракты дали ошибку → геокодер/резолв недоступны, это
	// НЕ честное 0% автопокрытие.
	if st.Total > 0 && st.Errors == st.Total {
		fmt.Fprintln(stderr, "ОШИБКА: все контракты дали ошибку — похоже, геокодер или БД недоступны (это НЕ честное 0% автопокрытие).")
		return 1
	}

	// Автопокрытие — ФАКТ (НЕ гейт FR-6/OQ-4 — живой ≥70%-вердикт на реальных данных, Story 3.3).
	// Errors — ОТДЕЛЬНАЯ ось от Matched/Unmatched (код-ревью: раньше формулировка подразумевала errors⊆
	// unmatched, но резолв района мог провалиться и на MATCHED контракте — Matched+Unmatched+Errors==Total
	// теперь чистое разбиение без пересечений, см. coverageStats).
	cov := 0.0
	if st.Total > 0 {
		cov = float64(st.Matched) / float64(st.Total)
	}
	fmt.Fprintf(stdout, "batch-геокодинг: контрактов=%d, сматчено=%d, без точки=%d, ошибок=%d (retryable следующим прогоном), автопокрытие=%.1f%% (ФАКТ, не FR-6-гейт — тот на живых данных, Story 3.3)\n",
		st.Total, st.Matched, st.Unmatched, st.Errors, cov*100)

	// Систематические пропуски — отдельной строкой (аналог 0.7 Шага B+): количество + примеры, чтобы был
	// виден КЛАСС адресов, который систематически не матчится. stderr — диагностика, stdout — сводка.
	if st.Unmatched > 0 {
		fmt.Fprintf(stderr, "систематические пропуски: %d контрактов без точки; примеры: %s\n",
			st.Unmatched, strings.Join(st.Samples, " | "))
	}
	return 0
}

func main() {
	max := flag.Int("max", 0, "максимум контрактов за прогон (0 = без лимита; вежливость к публичному Nominatim)")
	flag.Parse()
	if *max < 0 {
		fmt.Fprintln(os.Stderr, "ОШИБКА: -max не может быть отрицательным")
		os.Exit(2)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "ОШИБКА: DATABASE_URL пуст (нужна БД с contracts + geo_objects/districts)")
		os.Exit(2)
	}

	// Ctrl-C / SIGTERM прерывают долгий polite-batch чисто (записанное идемпотентно по contract_id).
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

	// NOMINATIM_URL пуст → публичный Nominatim (≤1 req/sec). Self-host (AR-22, профиль geocode) → без паузы.
	gc := geo.NewNominatim(os.Getenv("NOMINATIM_URL"), "", geo.AstanaViewbox, true)
	delay := geo.PoliteDelayDefault
	if os.Getenv("NOMINATIM_URL") != "" {
		delay = 0
	}

	os.Exit(run(ctx, gen.New(pool), gc, curation.NewGeoObjectStore(pool), delay, *max, os.Stdout, os.Stderr))
}

// Compile-time: *gen.Queries/curation.GeoObjectStore/*geo.Nominatim удовлетворяют своим узким интерфейсам
// (contractSource теперь РЕАЛЬНО принимается run() как параметр — код-ревью нашёл, что раньше этот
// интерфейс был мёртвой ceremony: main() звал gen.New(pool).ListContractsForGeocode(...) напрямую).
var _ contractSource = (*gen.Queries)(nil)
var _ geoStore = (*curation.GeoObjectStore)(nil)
var _ geocoder = (*geo.Nominatim)(nil)
