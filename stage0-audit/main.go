package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func main() {
	var (
		base      = flag.String("base", "https://ows.goszakup.gov.kz/v2", "базовый URL ows_v2")
		window    = flag.Int("window-months", 24, "окно аудита в месяцах")
		sample    = flag.Int("sample", 1000, "максимум записей на эндпоинт (0 = без лимита)")
		maxPages  = flag.Int("max-pages", 200, "предохранитель пагинации")
		delayMs   = flag.Int("delay-ms", 200, "пауза между запросами, мс")
		custBins  = flag.String("customer-bins", "", "БИН заказчиков Астаны через запятую (пусто = общая выборка, регион=unknown)")
		probeOnly = flag.Bool("probe", false, "только показать реальные поля эндпоинтов и выйти")

		source  = flag.String("source", "ows", "источник данных: ows|file|scrape")
		dataDir = flag.String("data-dir", "./data", "каталог дампов для -source file и -gen-fixtures")
		genFix  = flag.Bool("gen-fixtures", false, "сгенерировать синтетические фикстуры в -data-dir и выйти")
		fixN    = flag.Int("fixtures-n", 300, "сколько записей в фикстурах")

		scrapeBase    = flag.String("scrape-base", "https://goszakup.gov.kz/ru", "база публичного портала для -source scrape")
		scrapeUA      = flag.String("scrape-user-agent", "AshyqQala-stage0-audit/1.0", "User-Agent для парсера")
		scrapeDelayMs = flag.Int("scrape-delay-ms", 800, "пауза между запросами парсера, мс")

		geocoder    = flag.String("geocoder", "none", "геокодер для реального автопокрытия: none|nominatim")
		geocoderURL = flag.String("geocoder-url", "", "база геокодера (пусто = публичный Nominatim)")
		geocoderUA  = flag.String("geo-user-agent", "AshyqQala-stage0-audit/1.0 (+https://ashyqqala.kz)", "User-Agent для Nominatim")
		geoSample   = flag.Int("geo-sample", 200, "сколько объектов геокодировать")
		geoDelayMs  = flag.Int("geo-delay-ms", 1100, "пауза между запросами к геокодеру, мс (политика Nominatim ~1/сек)")
		viewbox     = flag.String("astana-viewbox", DefaultAstanaViewbox, "viewbox Астаны для геокодера")

		outFormat     = flag.String("format", "text", "формат вывода: text|json (json → машиночитаемый вердикт + exit-код для CI)")
		verdictOut    = flag.String("verdict-out", "", "путь baseline-артефакта (файл или каталог; пусто = только stdout)")
		verdictDate   = flag.String("verdict-date", "", "дата для имени stage0-verdict-YYYYMMDD.json (override; пусто = сегодня)")
		allowFallback = flag.Bool("allow-fallback", false, "разрешить частичный Go (go_with_fallback) при гео<гейта — требует sign-off владельца (Story 0.4)")
		gate          = flag.Bool("gate", false, "применять exit-код вердикта и в text-режиме (json гейтит всегда; по умолчанию text exit не меняет)")
	)
	flag.Parse()

	if *genFix {
		if err := genFixtures(*dataDir, *fixN); err != nil {
			fmt.Fprintln(os.Stderr, "ОШИБКА генерации фикстур:", err)
			os.Exit(1)
		}
		return
	}

	cfg := Config{
		BaseURL: *base, WindowMonths: *window,
		Sample: *sample, MaxPages: *maxPages,
		RequestDelay:      time.Duration(*delayMs) * time.Millisecond,
		MinSample:         5,
		MinGroupContracts: 5,
		GeoGate:           0.70,
		MonopolyShare:     0.5,
		DeviationFactor:   1.5,
		GeocoderKind:      *geocoder,
		GeocoderURL:       *geocoderURL,
		GeocoderUA:        *geocoderUA,
		AstanaViewbox:     *viewbox,
		GeoSample:         *geoSample,
		GeoDelay:          time.Duration(*geoDelayMs) * time.Millisecond,
	}

	var src Source
	switch strings.ToLower(*source) {
	case "ows":
		token := os.Getenv("GOSZAKUP_TOKEN")
		if token == "" {
			fmt.Fprintln(os.Stderr, "ОШИБКА: для -source ows задайте GOSZAKUP_TOKEN (или -source file / -gen-fixtures)")
			os.Exit(2)
		}
		cfg.Token = token
		src = owsSource{c: NewClient(cfg)}
	case "file":
		src = fileSource{dir: *dataDir}
	case "scrape":
		fmt.Fprintln(os.Stderr, "ВНИМАНИЕ: -source scrape парсит публичный портал — против принципа «официальный канал» (§6.1/§6.4); только для разовой выборки.")
		src = NewScrape(*scrapeBase, *scrapeUA, time.Duration(*scrapeDelayMs)*time.Millisecond, *maxPages)
	default:
		fmt.Fprintf(os.Stderr, "ОШИБКА: неизвестный -source %q (ows|file|scrape)\n", *source)
		os.Exit(2)
	}

	format := strings.ToLower(*outFormat)
	if format != "text" && format != "json" {
		fmt.Fprintf(os.Stderr, "ОШИБКА: неизвестный -format %q (text|json)\n", *outFormat)
		os.Exit(2)
	}
	// AC1: в JSON-режиме stdout — ВАЛИДНЫЙ JSON-объект, поэтому весь человекочитаемый
	// «шум» (баннер, геокодер, probe, [warn]) уходит в stderr; чистый JSON — в stdout.
	diag := io.Writer(os.Stdout)
	if format == "json" {
		diag = os.Stderr
	}

	bins := splitBins(*custBins)
	fmt.Fprintln(diag, "AshyqQala.kz — аудит данных goszakup (Этап 0)")
	fmt.Fprintf(diag, "source=%s  окно=%dмес  sample=%d  customerBins=%d\n\n", src.Name(), cfg.WindowMonths, cfg.Sample, len(bins))

	var geo Geocoder
	if strings.EqualFold(cfg.GeocoderKind, "nominatim") {
		geo = NewNominatim(cfg.GeocoderURL, cfg.GeocoderUA, cfg.AstanaViewbox, true)
		fmt.Fprintf(diag, "Геокодер: nominatim (sample=%d, delay=%v)\n\n", cfg.GeoSample, cfg.GeoDelay)
	}

	if *probeOnly {
		probeEndpoints(src) // явный диагностический режим — печать в stdout по запросу
		return
	}
	if format != "json" {
		probeEndpoints(src) // в text-режиме probe идёт перед аудитом (как раньше)
	}

	cutoff := time.Now().AddDate(0, -cfg.WindowMonths, 0)
	rep := runAudit(src, cfg, bins, cutoff, geo)

	opts := verdictOpts{
		allowFallback: *allowFallback,
		dataSource:    src.Name(),
		generatedAt:   time.Now().Format(time.RFC3339),
	}
	v, code := deriveVerdict(rep, cfg, opts)

	if format == "json" {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "ОШИБКА сериализации вердикта:", err)
			os.Exit(2)
		}
		out := string(b) + "\n"
		fmt.Print(out) // чистый JSON на stdout
		if *verdictOut != "" {
			if err := writeVerdictArtifact(*verdictOut, *verdictDate, out); err != nil {
				fmt.Fprintln(os.Stderr, "ОШИБКА записи артефакта:", err)
				os.Exit(2)
			}
			fmt.Fprintf(diag, "baseline-артефакт записан: %s\n", resolveVerdictPath(*verdictOut, artifactDate(*verdictDate)))
		}
		os.Exit(code) // json гейтит всегда (AC2): no_go → exit 1
	}

	rep.print(cfg)
	if *gate {
		os.Exit(code) // text-режим гейтит только по явному -gate (обратная совместимость)
	}
}

// artifactDate — дата для имени артефакта: override или сегодня (YYYYMMDD).
func artifactDate(override string) string {
	if override != "" {
		return override
	}
	return time.Now().Format("20060102")
}

// writeVerdictArtifact пишет JSON вердикта БАЙТ-в-байт идентично stdout (тот же
// MarshalIndent + финальный \n) — пригодно для коммита как baseline под docs/ops/.
func writeVerdictArtifact(out, dateOverride, content string) error {
	path := resolveVerdictPath(out, artifactDate(dateOverride))
	return os.WriteFile(path, []byte(content), 0o644)
}

// resolveVerdictPath: если out — существующий каталог / оканчивается разделителем /
// не .json → дописывает имя stage0-verdict-YYYYMMDD.json; иначе пишет как есть.
func resolveVerdictPath(out, date string) string {
	name := "stage0-verdict-" + date + ".json"
	if fi, err := os.Stat(out); err == nil && fi.IsDir() {
		return filepath.Join(out, name)
	}
	if strings.HasSuffix(out, string(os.PathSeparator)) {
		return filepath.Join(out, name)
	}
	if strings.HasSuffix(strings.ToLower(out), ".json") {
		return out
	}
	return filepath.Join(out, name)
}

func splitBins(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// probeEndpoints печатает реальные поля первого объекта по ресурсам — чтобы сверить
// fieldCandidates в models.go с источником данных. Включает journal (подтверждение
// доступа, Story 0.1 AC2) и отдельный резолв источника числа участников (FR-19).
func probeEndpoints(src Source) {
	fmt.Println("== Схема (probe): поля первого объекта по ресурсам ==")
	for _, res := range []string{"journal", "contract", "lots", "trd-buy", "subject", "rnu", "acts"} {
		var keys []string
		found := false
		err := src.Fetch(res, nil, 1, func(items []map[string]any) error {
			if len(items) > 0 && !found {
				keys = recordKeys(items[0])
				found = true
			}
			return nil
		})
		switch {
		case err != nil:
			fmt.Printf("  %-10s ОШИБКА: %v\n", res, err)
		case found:
			fmt.Printf("  %-10s поля: %s\n", res, strings.Join(keys, ", "))
		default:
			fmt.Printf("  %-10s (пусто или нет данных)\n", res)
		}
	}
	fmt.Println()
	resolveParticipantField(src)
}

// resolveParticipantField подтверждает ИСТОЧНИК числа участников для флага FR-19
// «единственный участник». Это ПОЛЕ на trd-buy (резерв — lots), НЕ отдельный эндпоинт;
// /subject — реестр юрлиц, к участникам конкретной закупки отношения не имеет. Печатает
// имя сматченного кандидата (или сигнал, что источник не подтверждён).
func resolveParticipantField(src Source) {
	fmt.Println("== Резолв поля «число участников» (FR-19, флаг «единственный участник») ==")
	for _, res := range []string{"trd-buy", "lots"} {
		var matched, sample string
		err := src.Fetch(res, nil, 1, func(items []map[string]any) error {
			if len(items) > 0 {
				if v, k, ok := getField(items[0], "participants"); ok {
					matched, sample = k, toString(v)
				}
			}
			return nil
		})
		switch {
		case err != nil:
			fmt.Printf("  %-8s ОШИБКА: %v\n", res, err)
		case matched != "":
			fmt.Printf("  %-8s поле числа участников: %q (пример значения: %s)\n", res, matched, sample)
		default:
			fmt.Printf("  %-8s кандидат не найден → источник НЕ подтверждён; добавьте имя поля в fieldCandidates[\"participants\"]\n", res)
		}
	}
	fmt.Println()
}

type monopolyGroup struct {
	key         string
	topSupplier string
	topShare    float64
	contracts   int
	flagged     bool
}

// Report накапливает метрики аудита.
type Report struct {
	contractsTotal, contractsInWindow                                     int
	sumPresent, supplierPresent, customerPresent, datePresent, dateParsed int
	geocodeEnabled                                                        bool
	geocodeAttempted, geocodeSuccess, geocodeErrors                       int
	distinctSuppliers                                                     map[string]bool
	missingBIN                                                            int
	lotsTotal                                                             int
	dirCount                                                              map[string]int
	geoEligible, geoWithMarker                                            int
	annoMatched                                                           int
	medianGroups                                                          map[string]int
	monopoly                                                              []monopolyGroup
	rnuBINs, suppliersInRNU                                               int
	participantField                                                      string
	participantChecked, singleParticipantCount                            int
	singleParticipantShare                                                float64
}

func runAudit(src Source, cfg Config, bins []string, cutoff time.Time, geo Geocoder) Report {
	r := Report{
		distinctSuppliers: map[string]bool{},
		dirCount:          map[string]int{},
		medianGroups:      map[string]int{},
	}

	// ---- Шаг A/C: контракты (объём, полнота, БИН) ----
	type cinfo struct {
		supplier string
		sum      float64
	}
	contractsByAnno := map[string][]cinfo{}
	handleContracts := func(items []map[string]any) error {
		for _, rec := range items {
			r.contractsTotal++
			sup := getString(rec, "supplier_biin")
			cust := getString(rec, "customer_bin")
			sum, sumOK := getFloat(rec, "contract_sum")
			ds := getString(rec, "sign_date")
			anno := getString(rec, "anno")
			if sup != "" {
				r.supplierPresent++
				r.distinctSuppliers[sup] = true
			} else {
				r.missingBIN++
			}
			if cust != "" {
				r.customerPresent++
			}
			if sumOK && sum > 0 {
				r.sumPresent++
			}
			inWindow := true
			if ds != "" {
				r.datePresent++
				if t, ok := parseDate(ds); ok {
					r.dateParsed++
					inWindow = !t.Before(cutoff)
				}
			}
			if inWindow {
				r.contractsInWindow++
			}
			if anno != "" && sup != "" {
				contractsByAnno[anno] = append(contractsByAnno[anno], cinfo{supplier: sup, sum: sum})
			}
		}
		return nil
	}
	if err := src.Fetch("contract", bins, cfg.Sample, handleContracts); err != nil {
		fmt.Fprintf(os.Stderr, "  [warn] contract: %v\n", err)
	}

	// ---- Шаг B: лоты (направление, гео-прокси, группы медиан) ----
	type linfo struct{ dir, region string }
	lotsByAnno := map[string]linfo{}
	var geoNames []string
	handleLots := func(items []map[string]any) error {
		for _, rec := range items {
			r.lotsTotal++
			name := getString(rec, "name")
			kato := getString(rec, "kato")
			anno := getString(rec, "anno")
			dir := classifyDirection(name)
			r.dirCount[dir]++
			region := "unknown"
			if isAstanaKATO(kato) {
				region = "astana"
			} else if len(bins) > 0 {
				region = "astana(scoped)"
			}
			if dir == "road" || dir == "water" {
				r.geoEligible++
				if hasLocationMarker(name) {
					r.geoWithMarker++
				}
				r.medianGroups[dir+"|"+region]++
				if anno != "" {
					lotsByAnno[anno] = linfo{dir: dir, region: region}
				}
				if geo != nil && len(geoNames) < cfg.GeoSample {
					geoNames = append(geoNames, name)
				}
			}
		}
		return nil
	}
	if err := src.Fetch("lots", bins, cfg.Sample, handleLots); err != nil {
		fmt.Fprintf(os.Stderr, "  [warn] lots: %v\n", err)
	}

	// ---- Реальная автогеопривязка (геокодер) — опционально ----
	if geo != nil {
		r.geocodeEnabled = true
		for _, name := range geoNames {
			r.geocodeAttempted++
			loc := extractLocation(name)
			if loc == "" {
				continue // нет локационного сигнала → объект не геокодируется
			}
			if _, _, ok, err := geo.Geocode(loc + ", Астана"); err != nil {
				r.geocodeErrors++
			} else if ok {
				r.geocodeSuccess++
			}
			if cfg.GeoDelay > 0 {
				time.Sleep(cfg.GeoDelay)
			}
		}
	}

	// ---- Join контракт↔лот по №объявления → монополия по (направление,регион) ----
	type agg struct {
		total      float64
		bySupplier map[string]float64
		count      int
	}
	groups := map[string]*agg{}
	for anno, cis := range contractsByAnno {
		li, ok := lotsByAnno[anno]
		if !ok {
			continue
		}
		r.annoMatched++
		key := li.dir + "|" + li.region
		g := groups[key]
		if g == nil {
			g = &agg{bySupplier: map[string]float64{}}
			groups[key] = g
		}
		for _, ci := range cis {
			g.total += ci.sum
			g.bySupplier[ci.supplier] += ci.sum
			g.count++
		}
	}
	for key, g := range groups {
		if g.count < cfg.MinGroupContracts || g.total <= 0 {
			continue
		}
		top, topSum := "", 0.0
		for s, v := range g.bySupplier {
			if v > topSum {
				topSum, top = v, s
			}
		}
		share := topSum / g.total
		r.monopoly = append(r.monopoly, monopolyGroup{
			key: key, topSupplier: top, topShare: share, contracts: g.count,
			flagged: share >= cfg.MonopolyShare,
		})
	}
	sort.Slice(r.monopoly, func(i, j int) bool { return r.monopoly[i].topShare > r.monopoly[j].topShare })

	// ---- РНУ ----
	rnuSet := map[string]bool{}
	if err := src.Fetch("rnu", nil, cfg.Sample, func(items []map[string]any) error {
		for _, rec := range items {
			if b := getString(rec, "rnu_bin"); b != "" {
				rnuSet[b] = true
			}
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "  [warn] rnu: %v\n", err)
	}
	r.rnuBINs = len(rnuSet)
	for s := range r.distinctSuppliers {
		if rnuSet[s] {
			r.suppliersInRNU++
		}
	}

	// ---- Единственный участник (FR-19): поле числа участников в trd-buy ----
	if err := src.Fetch("trd-buy", nil, min(cfg.Sample, 500), func(items []map[string]any) error {
		for _, rec := range items {
			if v, k, ok := getField(rec, "participants"); ok {
				r.participantField = k
				if f, ok2 := toFloat(v); ok2 {
					r.participantChecked++
					if f == 1 {
						r.singleParticipantCount++
					}
				}
			}
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "  [warn] trd-buy: %v\n", err)
	}
	if r.participantChecked > 0 {
		r.singleParticipantShare = float64(r.singleParticipantCount) / float64(r.participantChecked)
	}

	return r
}

func (r Report) print(cfg Config) {
	fmt.Println("== Шаг A. Объём и полнота (контракты) ==")
	fmt.Printf("  Контрактов получено: %d (в окне %dмес: %d)\n", r.contractsTotal, cfg.WindowMonths, r.contractsInWindow)
	fmt.Printf("  Полнота суммы:        %.1f%% (%d/%d)\n", pct(r.sumPresent, r.contractsTotal), r.sumPresent, r.contractsTotal)
	fmt.Printf("  Полнота supplier BIN: %.1f%% (%d/%d)\n", pct(r.supplierPresent, r.contractsTotal), r.supplierPresent, r.contractsTotal)
	fmt.Printf("  Полнота customer BIN: %.1f%% (%d/%d)\n", pct(r.customerPresent, r.contractsTotal), r.customerPresent, r.contractsTotal)
	fmt.Printf("  Дата подписания:      %.1f%% присутствует (распознано %d)\n", pct(r.datePresent, r.contractsTotal), r.dateParsed)
	fmt.Println()

	fmt.Println("== Шаг B. Направления и гео-прокси (лоты) ==")
	fmt.Printf("  Лотов: %d | дороги: %d, вода: %d, прочее: %d\n", r.lotsTotal, r.dirCount["road"], r.dirCount["water"], r.dirCount["other"])
	fmt.Printf("  Гео-прокси (дороги+вода с адресным маркером в названии): %.1f%% (%d/%d)\n", pct(r.geoWithMarker, r.geoEligible), r.geoWithMarker, r.geoEligible)
	fmt.Println("  [ПРОКСИ] Это НЕ реальная автогеопривязка — лишь доля объектов с локационным сигналом в тексте.")
	fmt.Println("           Финальный гейт ≥70% требует прогона геокодера; см. README.")
	fmt.Println()

	if r.geocodeEnabled {
		fmt.Println("== Шаг B+. Реальная автогеопривязка (геокодер) ==")
		cov := pct(r.geocodeSuccess, r.geocodeAttempted)
		gate := "✓ Go"
		if cov < 100*cfg.GeoGate {
			gate = "✗ ниже порога"
		}
		fmt.Printf("  Геокодировано: %d, успешно: %d, ошибок: %d\n", r.geocodeAttempted, r.geocodeSuccess, r.geocodeErrors)
		fmt.Printf("  Автопокрытие: %.1f%%  (гейт ≥ %.0f%%: %s)\n", cov, 100*cfg.GeoGate, gate)
		fmt.Println("  Прим.: выборка — первые N подходящих лотов; запрос = название лота + \", Астана\".")
		fmt.Println()
	}

	fmt.Println("== Шаг C. Нормализация (контракты) ==")
	fmt.Printf("  Уникальных supplier BIN: %d\n", len(r.distinctSuppliers))
	fmt.Printf("  Записей без BIN (ручная обработка): %d (%.1f%%)\n", r.missingBIN, pct(r.missingBIN, r.contractsTotal))
	fmt.Println("  Прим.: в ows_v2 сущности уже ключуются по БИН → нормализация проще, чем опасались; главный риск = пустой/битый БИН.")
	fmt.Println()

	fmt.Println("== Шаг D. Достаточность для флагов/медиан ==")
	enough := 0
	keys := make([]string, 0, len(r.medianGroups))
	for k := range r.medianGroups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		n := r.medianGroups[k]
		mark := " "
		if n >= cfg.MinSample {
			mark, enough = "✓", enough+1
		}
		fmt.Printf("  [%s] группа %-18s лотов: %d\n", mark, k, n)
	}
	fmt.Printf("  Групп (направление×регион) с выборкой ≥ %d: %d из %d\n", cfg.MinSample, enough, len(r.medianGroups))
	fmt.Printf("  Join контракт↔лот по №объявления: %d совпадений\n", r.annoMatched)
	if len(r.monopoly) > 0 {
		fmt.Printf("  Монополия (топ-доля поставщика по сумме, ≥%d контрактов в группе):\n", cfg.MinGroupContracts)
		for _, m := range r.monopoly {
			f := ""
			if m.flagged {
				f = " ← сигнал"
			}
			fmt.Printf("    %-18s топ-доля %.0f%% (контрактов %d)%s\n", m.key, 100*m.topShare, m.contracts, f)
		}
	} else {
		fmt.Println("  Монополия: недостаточно сджойненных данных для оценки (см. join выше).")
	}
	if r.participantField != "" {
		fmt.Printf("  Единственный участник: поле '%s', доля закупок с 1 участником: %.1f%% (из %d)\n", r.participantField, 100*r.singleParticipantShare, r.participantChecked)
	} else {
		fmt.Println("  Единственный участник: поле числа участников НЕ найдено в /trd-buy — VERIFY (возможно нужен /trd-buy/{id} или endpoint предложений).")
	}
	fmt.Printf("  РНУ: записей %d; из аудированных поставщиков в РНУ: %d\n", r.rnuBINs, r.suppliersInRNU)
	fmt.Println()

	fmt.Println("== Таблица решения Go/No-Go (предварительная) ==")
	geoProxy := pct(r.geoWithMarker, r.geoEligible)
	printRow("Контрактов в окне > 0", fmt.Sprintf("%d", r.contractsInWindow), r.contractsInWindow > 0)
	printRow("Полнота суммы ≥ 95%", fmt.Sprintf("%.1f%%", pct(r.sumPresent, r.contractsTotal)), pct(r.sumPresent, r.contractsTotal) >= 95)
	printRow("Полнота supplier BIN ≥ 98%", fmt.Sprintf("%.1f%%", pct(r.supplierPresent, r.contractsTotal)), pct(r.supplierPresent, r.contractsTotal) >= 98)
	printRow("Гео-прокси ≥ 70% (НЕ финальный гейт)", fmt.Sprintf("%.1f%%", geoProxy), geoProxy >= 70)
	if r.geocodeEnabled {
		cov := pct(r.geocodeSuccess, r.geocodeAttempted)
		printRow(fmt.Sprintf("Автогеопривязка ≥ %.0f%% (геокодер)", 100*cfg.GeoGate), fmt.Sprintf("%.1f%%", cov), cov >= 100*cfg.GeoGate)
	}
	printRow("Групп для медиан ≥ 1", fmt.Sprintf("%d", enough), enough >= 1)
	if r.geocodeEnabled {
		fmt.Println("\n  Гейт геопривязки замерен реальным геокодером (Шаг B+).")
	} else {
		fmt.Println("\n  Финальное решение по геопривязке (≥70%) — после прогона реального геокодера (-geocoder nominatim).")
	}
}

func printRow(name, val string, ok bool) {
	s := "✗"
	if ok {
		s = "✓"
	}
	fmt.Printf("  [%s] %-40s %s\n", s, name, val)
}
