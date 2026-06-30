package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/benchmark"
	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/district"
	"ashyqqala/server/internal/store/gen"
)

// medianFixedClock — Fixed «сейчас» для тестов медианы (окно 24 мес → cutoff 2024-06-01).
var medianFixedClock = clock.Fixed{T: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}

// mkSamples — ₸/км-выборка ВНУТРИ окна (2025 г.) с заданными ценами (медиана предсказуема).
func mkSamples(prices ...int64) []benchmark.Sample {
	out := make([]benchmark.Sample, len(prices))
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	for i, p := range prices {
		out[i] = benchmark.Sample{PricePerKM: p, SignDateUnix: at}
	}
	return out
}

// mockDistrictStore — фейковый DistrictStore: канонные строки/ошибки + захват LIKE-префикса и лимита.
// Реальную КАТО-фильтрацию/партиционирование проверяет integration-тест; здесь — wire/честные состояния/роутинг.
type mockDistrictStore struct {
	agg       gen.DistrictAggregatesRow
	flags     []gen.DistrictActiveFlagsByTypeRow
	objects   []gen.ListContractsByDistrictRow
	aggErr    error
	flagsErr  error
	objErr    error
	gotPrefix string
	gotLim    int32
	// samplesFn — ₸/км-выборка группы для медианы (6.4). nil → (nil,false): not_comparable (текущее
	// токен-независимое состояние — нет geo_objects.length_km). Тесты подменяют для negative-control.
	samplesFn   func(direction, katoPrefix string) ([]benchmark.Sample, bool, error)
	samplesErr  error
	gotSamplesQ []string // захват (direction|prefix) обращений — проверка ключей район/город
}

func (m *mockDistrictStore) DistrictAggregates(_ context.Context, p string) (gen.DistrictAggregatesRow, error) {
	m.gotPrefix = p
	return m.agg, m.aggErr
}

func (m *mockDistrictStore) DistrictActiveFlagsByType(_ context.Context, _ string) ([]gen.DistrictActiveFlagsByTypeRow, error) {
	if m.flagsErr != nil {
		return nil, m.flagsErr
	}
	return m.flags, nil
}

func (m *mockDistrictStore) ListContractsByDistrict(_ context.Context, arg gen.ListContractsByDistrictParams) ([]gen.ListContractsByDistrictRow, error) {
	m.gotLim = arg.Lim
	if m.objErr != nil {
		return nil, m.objErr
	}
	return m.objects, nil
}

func (m *mockDistrictStore) PricePerKMSamples(_ context.Context, direction, katoPrefix string) ([]benchmark.Sample, bool, error) {
	m.gotSamplesQ = append(m.gotSamplesQ, direction+"|"+katoPrefix)
	if m.samplesErr != nil {
		return nil, false, m.samplesErr
	}
	if m.samplesFn != nil {
		return m.samplesFn(direction, katoPrefix)
	}
	return nil, false, nil // дефолт: not_comparable (нет length_km)
}

func newDistrictRouter(store DistrictStore, cat *district.Catalog) http.Handler {
	r := chi.NewRouter()
	// Window/MethodologyVersion/Clock — из methodology_params (тестовые значения; окно 24 мес, версия v1.0).
	r.Get("/api/districts/{kato}", DistrictsHandler{
		Store: store, Districts: cat,
		Window: 24, MethodologyVersion: "v1.0", Clock: medianFixedClock,
	}.Get)
	return r
}

func getDistrict(t *testing.T, store DistrictStore, cat *district.Catalog, kato string) (*http.Response, []byte) {
	t.Helper()
	srv := httptest.NewServer(newDistrictRouter(store, cat))
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/api/districts/" + kato)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, body
}

func mkDistrictObject(gid string, amount int64, amountValid, flag bool) gen.ListContractsByDistrictRow {
	return gen.ListContractsByDistrictRow{
		GoszakupContractID: gid,
		SubjectRu:          pgtype.Text{String: "Ремонт автодороги " + gid, Valid: true},
		SubjectKk:          pgtype.Text{Valid: false}, // NULL → no_data
		AmountTng:          pgtype.Int8{Int64: amount, Valid: amountValid},
		KatoCode:           pgtype.Text{String: "710000000", Valid: true},
		Direction:          pgtype.Text{String: "road", Valid: true},
		HasActiveFlag:      flag,
	}
}

// realCatalog — реестр районов репо (все kato=null) для проверки честного no_data имени.
func realCatalog(t *testing.T) *district.Catalog {
	t.Helper()
	c, err := district.Load("../../../registry")
	if err != nil {
		t.Fatalf("district.Load: %v", err)
	}
	return c
}

// confirmedCatalog — синтетический реестр с ПОДТВЕРЖДЁННЫМ кодом (имитация состояния после Story 0.1).
func confirmedCatalog(t *testing.T, kato, nameKk, nameRu string) *district.Catalog {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "values"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"version":"v1","districts":[{"slug":"x","name_kk":"` + nameKk + `","name_ru":"` + nameRu + `","kato":"` + kato + `"}]}`
	if err := os.WriteFile(filepath.Join(root, "values", district.DistrictsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := district.Load(root)
	if err != nil {
		t.Fatalf("district.Load(temp): %v", err)
	}
	return c
}

// TestDistrict_WireFormat — населённый район: агрегаты+флаги+объекты; ответ валиден против DistrictAggregates;
// деньги строкой; container_state ["not_geocoded"] (флаги есть → нет no_flags_raised); имя no_data (коды не подтверждены).
func TestDistrict_WireFormat_ValidatesAgainstOpenAPI(t *testing.T) {
	store := &mockDistrictStore{
		agg:   gen.DistrictAggregatesRow{ContractCount: 18, TotalAmountTng: 2_400_000_000, AmountKnownCount: 18},
		flags: []gen.DistrictActiveFlagsByTypeRow{{FlagType: "monopoly", N: 1}, {FlagType: "single_participant", N: 5}},
		objects: []gen.ListContractsByDistrictRow{
			mkDistrictObject("DEMO-0002", 240000000, true, true),
			mkDistrictObject("DEMO-0003", 0, false, false), // NULL amount → no_data
		},
	}
	resp, body := getDistrict(t, store, realCatalog(t), "710000000")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, ожидалось 200: %s", resp.StatusCode, body)
	}
	obj := jsonObj(t, body)
	if err := loadSchema(t, "DistrictAggregates").VisitJSON(obj); err != nil {
		t.Fatalf("ответ не валиден против DistrictAggregates: %v\n%s", err, body)
	}
	if obj["contract_count"] != "18" {
		t.Errorf("contract_count = %v, ожидалось \"18\"", obj["contract_count"])
	}
	if obj["active_flags_count"] != "6" {
		t.Errorf("active_flags_count = %v, ожидалось \"6\" (1 monopoly + 5 single)", obj["active_flags_count"])
	}
	amount := obj["total_amount_tng"].(map[string]any)
	if amount["state"] != "ok" || amount["value"] != "2400000000" {
		t.Errorf("total_amount_tng = %v, ожидалось ok/\"2400000000\"", amount)
	}
	// Имя района — честный no_data (КАТО-код не подтверждён, Story 0.1; имя НЕ выдумывается).
	nameRu := obj["name_ru"].(map[string]any)
	if nameRu["state"] != "no_data" || nameRu["value"] != nil {
		t.Errorf("name_ru должен быть no_data/null (код не подтверждён), получено %v", nameRu)
	}
	cs := toStringSlice(obj["container_state"])
	if len(cs) != 1 || cs[0] != "not_geocoded" {
		t.Errorf("container_state = %v, ожидалось [not_geocoded] (флаги есть, гео нет)", cs)
	}
	// 2-й объект: NULL amount → честный no_data, не «0».
	objects := obj["objects"].([]any)
	if len(objects) != 2 {
		t.Fatalf("objects = %d, ожидалось 2", len(objects))
	}
	o2amount := objects[1].(map[string]any)["amount_tng"].(map[string]any)
	if o2amount["state"] != "no_data" || o2amount["value"] != nil {
		t.Errorf("NULL amount объекта должен быть no_data/null, получено %v", o2amount)
	}
	// LIKE-префикс собран из КАТО + '%'; лимит проброшен.
	if store.gotPrefix != "710000000%" {
		t.Errorf("LIKE-префикс = %q, ожидалось \"710000000%%\"", store.gotPrefix)
	}
	if store.gotLim != districtObjectsLimit {
		t.Errorf("лимит объектов = %d, ожидалось %d", store.gotLim, districtObjectsLimit)
	}
}

// TestDistrict_Empty — пустой район: container_state ["no_contracts"]; count "0"; сумма no_data (НЕ «0 ₸»);
// массивы пустые (не null). Negative control honest-states: 0 контрактов ≠ «всё чисто».
func TestDistrict_Empty_NoContracts(t *testing.T) {
	store := &mockDistrictStore{agg: gen.DistrictAggregatesRow{ContractCount: 0, TotalAmountTng: 0, AmountKnownCount: 0}}
	resp, body := getDistrict(t, store, realCatalog(t), "799999999")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, ожидалось 200 (неизвестный-но-валидный КАТО → пусто, НЕ 404): %s", resp.StatusCode, body)
	}
	obj := jsonObj(t, body)
	if err := loadSchema(t, "DistrictAggregates").VisitJSON(obj); err != nil {
		t.Fatalf("пустой ответ не валиден: %v\n%s", err, body)
	}
	if obj["contract_count"] != "0" {
		t.Errorf("contract_count = %v, ожидалось \"0\"", obj["contract_count"])
	}
	amount := obj["total_amount_tng"].(map[string]any)
	if amount["state"] != "no_data" || amount["value"] != nil {
		t.Errorf("пустой район: сумма должна быть no_data/null (НЕ «0 ₸»), получено %v", amount)
	}
	cs := toStringSlice(obj["container_state"])
	if len(cs) != 1 || cs[0] != "no_contracts" {
		t.Errorf("container_state = %v, ожидалось [no_contracts]", cs)
	}
	if !strings.Contains(string(body), `"objects":[]`) || !strings.Contains(string(body), `"flags_by_type":[]`) {
		t.Errorf("пустые массивы должны быть [] не null: %s", body)
	}
}

// TestDistrict_ContractsNoFlags — объекты есть, активных флагов нет → container_state [no_flags_raised, not_geocoded].
// Negative control: «нет сигналов» отдельной строкой, не тишина.
func TestDistrict_ContractsButNoFlags(t *testing.T) {
	store := &mockDistrictStore{
		agg:     gen.DistrictAggregatesRow{ContractCount: 3, TotalAmountTng: 100, AmountKnownCount: 3},
		flags:   nil,
		objects: []gen.ListContractsByDistrictRow{mkDistrictObject("C1", 100, true, false)},
	}
	_, body := getDistrict(t, store, realCatalog(t), "710000000")
	obj := jsonObj(t, body)
	if obj["active_flags_count"] != "0" {
		t.Errorf("active_flags_count = %v, ожидалось \"0\"", obj["active_flags_count"])
	}
	cs := toStringSlice(obj["container_state"])
	if len(cs) != 2 || cs[0] != "no_flags_raised" || cs[1] != "not_geocoded" {
		t.Errorf("container_state = %v, ожидалось [no_flags_raised, not_geocoded]", cs)
	}
}

// TestDistrict_NameResolvesWhenCodeConfirmed — когда КАТО-код района подтверждён (после 0.1), имя резолвится.
// Доказывает, что no_data-имя выше — следствие отсутствия кода, а не всегда-no_data (страж не вакуумен).
func TestDistrict_NameResolvesWhenCodeConfirmed(t *testing.T) {
	store := &mockDistrictStore{agg: gen.DistrictAggregatesRow{ContractCount: 1, AmountKnownCount: 0}}
	cat := confirmedCatalog(t, "710512", "Есіл ауданы", "район Есиль")
	_, body := getDistrict(t, store, cat, "710512100") // префиксуется подтверждённым кодом
	obj := jsonObj(t, body)
	nameRu := obj["name_ru"].(map[string]any)
	if nameRu["state"] != "ok" || nameRu["value"] != "район Есиль" {
		t.Errorf("name_ru при подтверждённом коде = %v, ожидалось ok/\"район Есиль\"", nameRu)
	}
}

func TestDistrict_InvalidKato_400(t *testing.T) {
	cases := []struct{ name, kato string }{
		{"не цифры", "abc"},
		{"слишком короткий", "7"},
		{"слишком длинный", "710000000000"},
		{"метасимвол LIKE percent", "71%25"}, // %25 = '%' — должен быть отвергнут как не-цифра
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := &mockDistrictStore{}
			resp, body := getDistrict(t, store, realCatalog(t), c.kato)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, ожидалось 400 (%s): %s", resp.StatusCode, c.name, body)
			}
			if store.gotPrefix != "" {
				t.Errorf("стор НЕ должен вызываться при невалидном КАТО (got prefix %q)", store.gotPrefix)
			}
			var env map[string]struct {
				Code string `json:"code"`
			}
			_ = json.Unmarshal(body, &env)
			if env["error"].Code != "VALIDATION_FAILED" {
				t.Errorf("code = %q, ожидалось VALIDATION_FAILED", env["error"].Code)
			}
			if err := loadSchema(t, "Error").VisitJSON(jsonAny(t, body)); err != nil {
				t.Errorf("ошибка не валидна против Error: %v", err)
			}
		})
	}
}

// negative control: валидный КАТО (граница min-длины 2) НЕ должен давать 400 — иначе тест 400 прошёл бы мимо.
func TestDistrict_ValidKato_Not400(t *testing.T) {
	store := &mockDistrictStore{agg: gen.DistrictAggregatesRow{ContractCount: 0}}
	resp, body := getDistrict(t, store, realCatalog(t), "71")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("валидный КАТО «71» дал %d: %s", resp.StatusCode, body)
	}
}

func TestDistrict_StoreError_500(t *testing.T) {
	for _, c := range []struct {
		name  string
		store *mockDistrictStore
	}{
		{"agg error", &mockDistrictStore{aggErr: errors.New("db down")}},
		{"flags error", &mockDistrictStore{flagsErr: errors.New("db down")}},
		{"objects error", &mockDistrictStore{objErr: errors.New("db down")}},
	} {
		t.Run(c.name, func(t *testing.T) {
			resp, body := getDistrict(t, c.store, realCatalog(t), "710000000")
			if resp.StatusCode != http.StatusInternalServerError {
				t.Fatalf("status = %d, ожидалось 500: %s", resp.StatusCode, body)
			}
			if err := loadSchema(t, "Error").VisitJSON(jsonAny(t, body)); err != nil {
				t.Errorf("ошибка не валидна против Error: %v", err)
			}
		})
	}
}

// findMedian — median-запись района по направлению (helper для 6.4-тестов).
func findMedian(t *testing.T, obj map[string]any, dir string) map[string]any {
	t.Helper()
	arr, ok := obj["medians"].([]any)
	if !ok {
		t.Fatalf("medians не массив: %v", obj["medians"])
	}
	for _, e := range arr {
		m := e.(map[string]any)
		if m["direction"] == dir {
			return m
		}
	}
	t.Fatalf("median для направления %q не найдена среди %v", dir, arr)
	return nil
}

// TestDistrictMedian_NotComparableWithoutLength — ТЕКУЩЕЕ токен-независимое состояние (FR-18, нет length_km,
// Epic 3): медиана района/города по КАЖДОМУ направлению = not_comparable (честный skip), % дельты НЕТ; ключи
// сопоставимости зафиксированы (район = URL-КАТО, город = «71»); methodology_version на поверхности. Валидно
// против OpenAPI. Это «честно-пусто» прецедента 4.3 — НЕ выдуманные числа.
func TestDistrictMedian_NotComparableWithoutLength(t *testing.T) {
	store := &mockDistrictStore{agg: gen.DistrictAggregatesRow{ContractCount: 18, AmountKnownCount: 18}}
	resp, body := getDistrict(t, store, realCatalog(t), "710000000")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}
	obj := jsonObj(t, body)
	if err := loadSchema(t, "DistrictAggregates").VisitJSON(obj); err != nil {
		t.Fatalf("ответ с median-блоком не валиден против DistrictAggregates: %v\n%s", err, body)
	}
	if obj["median_methodology_version"] != "v1.0" {
		t.Errorf("median_methodology_version = %v, ожидалось v1.0 (фиксация версии на поверхности)", obj["median_methodology_version"])
	}
	if arr, _ := obj["medians"].([]any); len(arr) != 2 {
		t.Fatalf("medians = %v, ожидалось 2 направления (road, water)", obj["medians"])
	}
	for _, dir := range []string{"road", "water"} {
		m := findMedian(t, obj, dir)
		dm := m["district_median_tng"].(map[string]any)
		if dm["state"] != "not_comparable" || dm["value"] != nil {
			t.Errorf("%s: district медиана = %v, ожидалось not_comparable/null (нет length_km)", dir, dm)
		}
		cm := m["city_median_tng"].(map[string]any)
		if cm["state"] != "not_comparable" || cm["value"] != nil {
			t.Errorf("%s: city медиана = %v, ожидалось not_comparable/null", dir, cm)
		}
		pct := m["comparison_pct"].(map[string]any)
		if pct["state"] != "not_comparable" || pct["value"] != nil {
			t.Errorf("%s: comparison_pct = %v, ожидалось not_comparable/null (обе медианы не ok)", dir, pct)
		}
		if m["district_sample_size"] != "0" || m["city_sample_size"] != "0" {
			t.Errorf("%s: размеры выборок = %v/%v, ожидалось 0/0", dir, m["district_sample_size"], m["city_sample_size"])
		}
		if m["district_comparability_key"] != "direction="+dir+"|kato=710000000" {
			t.Errorf("%s: district key = %v", dir, m["district_comparability_key"])
		}
		if m["city_comparability_key"] != "direction="+dir+"|kato=71" {
			t.Errorf("%s: city key = %v, ожидалось …|kato=71", dir, m["city_comparability_key"])
		}
	}
	// Хендлер собирал выборки по группам: район-префикс «710000000%» и город-префикс «71%» для обоих направлений.
	joined := strings.Join(store.gotSamplesQ, ",")
	for _, want := range []string{"road|710000000%", "road|71%", "water|710000000%", "water|71%"} {
		if !strings.Contains(joined, want) {
			t.Errorf("ожидалось обращение к выборке %q, получено %v", want, store.gotSamplesQ)
		}
	}
}

// TestDistrictMedian_NegativeControl_SyntheticSamples — НЕГАТИВНЫЙ КОНТРОЛЬ (доказывает, что not_comparable
// выше — следствие ОТСУТСТВИЯ length_km, а НЕ всегда-not_comparable): при синтетических достаточных выборках
// (≥5) медиана = ЧИСЛО, а % дельта считается. road: район медиана 300, город 250 → +20%. water остаётся
// not_comparable (computable=false). Это шов: когда появится length_km, движок выдаст реальные числа.
func TestDistrictMedian_NegativeControl_SyntheticSamples(t *testing.T) {
	store := &mockDistrictStore{
		agg: gen.DistrictAggregatesRow{ContractCount: 9, AmountKnownCount: 9},
		samplesFn: func(direction, katoPrefix string) ([]benchmark.Sample, bool, error) {
			if direction != "road" {
				return nil, false, nil // water → not_comparable
			}
			if katoPrefix == "71%" { // город: медиана 250
				return mkSamples(50, 150, 250, 350, 450), true, nil
			}
			return mkSamples(100, 200, 300, 400, 500), true, nil // район: медиана 300
		},
	}
	_, body := getDistrict(t, store, realCatalog(t), "710000000")
	obj := jsonObj(t, body)
	if err := loadSchema(t, "DistrictAggregates").VisitJSON(obj); err != nil {
		t.Fatalf("ответ не валиден против DistrictAggregates: %v\n%s", err, body)
	}
	road := findMedian(t, obj, "road")
	dm := road["district_median_tng"].(map[string]any)
	if dm["state"] != "ok" || dm["value"] != "300" {
		t.Errorf("road district медиана = %v, ожидалось ok/\"300\"", dm)
	}
	cm := road["city_median_tng"].(map[string]any)
	if cm["state"] != "ok" || cm["value"] != "250" {
		t.Errorf("road city медиана = %v, ожидалось ok/\"250\"", cm)
	}
	pct := road["comparison_pct"].(map[string]any)
	if pct["state"] != "ok" || pct["value"] != "20" { // (300-250)*100/250 = 20
		t.Errorf("road comparison_pct = %v, ожидалось ok/\"20\" (+20%%)", pct)
	}
	if road["district_sample_size"] != "5" || road["city_sample_size"] != "5" {
		t.Errorf("road размеры выборок = %v/%v, ожидалось 5/5", road["district_sample_size"], road["city_sample_size"])
	}
	// water остаётся not_comparable — доказывает per-направление независимость.
	water := findMedian(t, obj, "water")
	wdm := water["district_median_tng"].(map[string]any)
	if wdm["state"] != "not_comparable" {
		t.Errorf("water district медиана = %v, ожидалось not_comparable (computable=false)", wdm)
	}
}

// TestDistrictMedian_DeltaHiddenWhenInsufficient — % дельта показывается ТОЛЬКО при обеих медианах ok: район
// с выборкой <5 (insufficient_sample) при городе ok → дельты НЕТ (not_comparable), но медиана города — число.
func TestDistrictMedian_DeltaHiddenWhenInsufficient(t *testing.T) {
	store := &mockDistrictStore{
		agg: gen.DistrictAggregatesRow{ContractCount: 4, AmountKnownCount: 4},
		samplesFn: func(direction, katoPrefix string) ([]benchmark.Sample, bool, error) {
			if direction != "road" {
				return nil, false, nil
			}
			if katoPrefix == "71%" {
				return mkSamples(50, 150, 250, 350, 450), true, nil // город ok (медиана 250)
			}
			return mkSamples(100, 200, 300), true, nil // район <5 → insufficient_sample
		},
	}
	_, body := getDistrict(t, store, realCatalog(t), "710000000")
	obj := jsonObj(t, body)
	road := findMedian(t, obj, "road")
	dm := road["district_median_tng"].(map[string]any)
	if dm["state"] != "insufficient_sample" || dm["value"] != nil {
		t.Errorf("район <5: district медиана = %v, ожидалось insufficient_sample/null", dm)
	}
	cm := road["city_median_tng"].(map[string]any)
	if cm["state"] != "ok" || cm["value"] != "250" {
		t.Errorf("город ok: city медиана = %v, ожидалось ok/\"250\"", cm)
	}
	pct := road["comparison_pct"].(map[string]any)
	if pct["state"] == "ok" || pct["value"] != nil {
		t.Errorf("дельта при insufficient районе НЕ должна показываться, получено %v", pct)
	}
	if road["district_sample_size"] != "3" {
		t.Errorf("район размер выборки = %v, ожидалось 3", road["district_sample_size"])
	}
}

// TestDistrictMedian_SamplesError_500 — сбой сбора ₸/км-выборки → честная 500 (как прочие store-ошибки).
func TestDistrictMedian_SamplesError_500(t *testing.T) {
	store := &mockDistrictStore{
		agg:        gen.DistrictAggregatesRow{ContractCount: 1, AmountKnownCount: 0},
		samplesErr: errors.New("db down"),
	}
	resp, body := getDistrict(t, store, realCatalog(t), "710000000")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, ожидалось 500: %s", resp.StatusCode, body)
	}
	if err := loadSchema(t, "Error").VisitJSON(jsonAny(t, body)); err != nil {
		t.Errorf("ошибка не валидна против Error: %v", err)
	}
}

// --- локальные хелперы (не пересекаются с другими тестами пакета) ---

func jsonObj(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, body)
	}
	return obj
}

func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		out = append(out, e.(string))
	}
	return out
}
