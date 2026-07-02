package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/geo"
	"ashyqqala/server/internal/store/gen"
)

// --- fakes (0.7-паттерн: узкие интерфейсы, тестируемость без сети/БД) ---

// fakeContractSource — contractSource по срезу строк, либо ошибка. Код-ревью нашёл: contractSource был
// мёртвым интерфейсом (никогда не принимался параметром) — теперь run() реально его использует, и этот
// фейк даёт main()'s core логике (пустой список/-max-срез/all-errored-гейт) тестовое покрытие впервые.
type fakeContractSource struct {
	rows []gen.ListContractsForGeocodeRow
	err  error
}

func (f *fakeContractSource) ListContractsForGeocode(context.Context) ([]gen.ListContractsForGeocodeRow, error) {
	return f.rows, f.err
}

// fakeGeocoder — geocoder по карте query→(Match,ok); отсутствующий ключ → err (симулирует сетевую ошибку).
type fakeGeocoder struct {
	matches map[string]geo.Match
	fail    map[string]error
	calls   []string
}

func (f *fakeGeocoder) GeocodeMatch(_ context.Context, q string) (geo.Match, bool, error) {
	f.calls = append(f.calls, q)
	if err, ok := f.fail[q]; ok {
		return geo.Match{}, false, err
	}
	if m, ok := f.matches[q]; ok {
		return m, true, nil
	}
	return geo.Match{}, false, nil // честный не-матч
}

// fakeGeoStore — geoStore в памяти: districtsByPoint/districtsByKato — карты WKT/KATO→districtID;
// отсутствие ключа → pgx.ErrNoRows (честное «не найдено», как реальный :one без строк).
type fakeGeoStore struct {
	districtsByPoint map[string]int64
	districtsByKato  map[string]int64
	pointErr         error // если задан — FindDistrictIDByPoint всегда возвращает эту ошибку (не ErrNoRows)
	katoErr          error
	upserted         []gen.UpsertGeoObjectParams
	upsertErr        error
}

func (f *fakeGeoStore) UpsertGeoObject(_ context.Context, p gen.UpsertGeoObjectParams) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upserted = append(f.upserted, p)
	return nil
}

func (f *fakeGeoStore) FindDistrictIDByPoint(_ context.Context, geomWKT string) (int64, error) {
	if f.pointErr != nil {
		return 0, f.pointErr
	}
	if id, ok := f.districtsByPoint[geomWKT]; ok {
		return id, nil
	}
	return 0, pgx.ErrNoRows
}

func (f *fakeGeoStore) FindDistrictIDByKATOPrefix(_ context.Context, kato string) (int64, error) {
	if f.katoErr != nil {
		return 0, f.katoErr
	}
	if id, ok := f.districtsByKato[kato]; ok {
		return id, nil
	}
	return 0, pgx.ErrNoRows
}

func contract(id int64, gid, subjectRu, kato string) gen.ListContractsForGeocodeRow {
	r := gen.ListContractsForGeocodeRow{ID: id, GoszakupContractID: gid}
	if subjectRu != "" {
		r.SubjectRu = pgtype.Text{String: subjectRu, Valid: true}
	}
	if kato != "" {
		r.KatoCode = pgtype.Text{String: kato, Valid: true}
	}
	return r
}

// --- addr / wkt / contractKato (чистые мапперы) ---

func TestAddr(t *testing.T) {
	if got := addr(contract(1, "X-1", "", "710000000")); got != "" {
		t.Errorf("addr без subject_ru = %q, ожидалось «» (honest unmatched, не геокодим)", got)
	}
	if got := addr(contract(1, "X-1", "  ", "710000000")); got != "" {
		t.Errorf("addr с пробельным subject_ru = %q, ожидалось «»", got)
	}
	got := addr(contract(1, "X-1", "проспект  Абая", "710000000"))
	want := "проспект Абая, Астана"
	if got != want {
		t.Errorf("addr = %q, ожидалось %q (нормализация + «, Астана»)", got, want)
	}
}

func TestWKT(t *testing.T) {
	if got := wkt(51.1605, 71.4704); got != "POINT(71.4704 51.1605)" {
		t.Errorf("wkt = %q, ожидалось WKT POINT(lon lat) (AR-19: наружу [lon,lat])", got)
	}
}

func TestContractKato(t *testing.T) {
	if got := contractKato(contract(1, "X", "s", "")); got != "" {
		t.Errorf("contractKato без kato_code = %q, ожидалось «»", got)
	}
	if got := contractKato(contract(1, "X", "s", "710000000")); got != "710000000" {
		t.Errorf("contractKato = %q, ожидалось 710000000", got)
	}
}

// --- toGeoParams (чистый маппер) ---

func TestToGeoParams_Matched(t *testing.T) {
	c := contract(42, "X-42", "ул. Абая", "710000000")
	conf := 0.8
	m := geo.Match{Lat: 51.1, Lon: 71.4, Confidence: &conf}
	districtID := pgtype.Int8{Int64: 7, Valid: true}
	p := toGeoParams(c, "ул. Абая, Астана", m, true, districtID)

	if p.ContractID != (pgtype.Int8{Int64: 42, Valid: true}) {
		t.Errorf("ContractID = %+v", p.ContractID)
	}
	if p.GeocodeStatus != "auto" {
		t.Errorf("GeocodeStatus = %q, ожидалось auto", p.GeocodeStatus)
	}
	if !p.GeomWkt.Valid || p.GeomWkt.String != "POINT(71.4 51.1)" {
		t.Errorf("GeomWkt = %+v, ожидалось валидный POINT(71.4 51.1)", p.GeomWkt)
	}
	if !p.Confidence.Valid || p.Confidence.Float64 != 0.8 {
		t.Errorf("Confidence = %+v, ожидалось 0.8", p.Confidence)
	}
	if p.DistrictID != districtID {
		t.Errorf("DistrictID = %+v, ожидалось %+v", p.DistrictID, districtID)
	}
	if !p.AddressText.Valid || p.AddressText.String != "ул. Абая, Астана" {
		t.Errorf("AddressText = %+v", p.AddressText)
	}
}

// TestToGeoParams_MatchedButNoImportance_NilConfidence — код-ревью нашёл: matched + geom.Confidence=nil
// (self-host не отдал importance) раньше писался как ЛИТЕРАЛЬНЫЙ 0.0 (неотличимо от «importance реально
// пришла нулевой»). Теперь честный NULL на проводе БД.
func TestToGeoParams_MatchedButNoImportance_NilConfidence(t *testing.T) {
	c := contract(1, "X-1", "адрес", "710000000")
	m := geo.Match{Lat: 51.1, Lon: 71.4, Confidence: nil}
	p := toGeoParams(c, "адрес, Астана", m, true, pgtype.Int8{})
	if p.GeocodeStatus != "auto" {
		t.Fatalf("GeocodeStatus = %q, ожидалось auto (координаты есть)", p.GeocodeStatus)
	}
	if p.Confidence.Valid {
		t.Errorf("Confidence = %+v, ожидалось NULL (геокодер не отдал importance — не выдумывать 0.0)", p.Confidence)
	}
}

func TestToGeoParams_Unmatched_NeverFakeCoords(t *testing.T) {
	c := contract(1, "X-1", "нечто", "710000000")
	p := toGeoParams(c, "нечто, Астана", geo.Match{}, false, pgtype.Int8{})

	if p.GeocodeStatus != "unmatched" {
		t.Errorf("GeocodeStatus = %q, ожидалось unmatched", p.GeocodeStatus)
	}
	if p.GeomWkt.Valid {
		t.Errorf("GeomWkt.Valid = true на unmatched — гардрейл AC2 нарушен (НЕ 0,0/фейк-точка)")
	}
	if p.Confidence.Valid {
		t.Error("Confidence.Valid = true на unmatched, ожидалось NULL")
	}
}

func TestToGeoParams_EmptyQuery_NoAddressText(t *testing.T) {
	p := toGeoParams(contract(1, "X-1", "", ""), "", geo.Match{}, false, pgtype.Int8{})
	if p.AddressText.Valid {
		t.Error("AddressText.Valid = true при пустом query, ожидалось NULL (нечего было отправлять геокодеру)")
	}
}

// --- resolveDistrict ---

func TestResolveDistrict_MatchedByPoint(t *testing.T) {
	store := &fakeGeoStore{districtsByPoint: map[string]int64{"POINT(71.4 51.1)": 5}}
	id, err := resolveDistrict(context.Background(), store, true, "POINT(71.4 51.1)", "710000000")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if id != (pgtype.Int8{Int64: 5, Valid: true}) {
		t.Errorf("districtID = %+v, ожидалось 5 (по точке)", id)
	}
}

func TestResolveDistrict_MatchedButPointMiss_FallsBackToKATO(t *testing.T) {
	store := &fakeGeoStore{districtsByKato: map[string]int64{"710000000": 9}}
	id, err := resolveDistrict(context.Background(), store, true, "POINT(0 0)", "710000000")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if id != (pgtype.Int8{Int64: 9, Valid: true}) {
		t.Errorf("districtID = %+v, ожидалось 9 (фолбэк на КАТО-префикс, AC2)", id)
	}
}

func TestResolveDistrict_Unmatched_UsesKATOPrefix(t *testing.T) {
	store := &fakeGeoStore{districtsByKato: map[string]int64{"710000000": 3}}
	id, err := resolveDistrict(context.Background(), store, false, "", "710000000")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if id != (pgtype.Int8{Int64: 3, Valid: true}) {
		t.Errorf("districtID = %+v, ожидалось 3 (AC2: район даже без точки)", id)
	}
}

func TestResolveDistrict_NoKATO_NoDistrict_NotError(t *testing.T) {
	store := &fakeGeoStore{}
	id, err := resolveDistrict(context.Background(), store, false, "", "")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if id.Valid {
		t.Errorf("districtID = %+v, ожидалось NULL (нет КАТО — честно, не ошибка)", id)
	}
}

func TestResolveDistrict_KATONotFound_NotError(t *testing.T) {
	store := &fakeGeoStore{} // districtsByKato пуст → ErrNoRows на любой код
	id, err := resolveDistrict(context.Background(), store, false, "", "999999999")
	if err != nil {
		t.Fatalf("ErrNoRows должен трактоваться честно (nil), получено: %v", err)
	}
	if id.Valid {
		t.Errorf("districtID = %+v, ожидалось NULL (район ещё не импортирован — S-0 частичен)", id)
	}
}

func TestResolveDistrict_InfrastructureError_Propagates(t *testing.T) {
	boom := errors.New("boom: connection reset")
	store := &fakeGeoStore{pointErr: boom}
	_, err := resolveDistrict(context.Background(), store, true, "POINT(0 0)", "710000000")
	if !errors.Is(err, boom) {
		t.Fatalf("инфраструктурная ошибка должна прокидываться наружу (не маскироваться под «нет района»), получено: %v", err)
	}
}

func TestResolveDistrict_KATOInfrastructureError_Propagates(t *testing.T) {
	boom := errors.New("boom")
	store := &fakeGeoStore{katoErr: boom}
	_, err := resolveDistrict(context.Background(), store, false, "", "710000000")
	if !errors.Is(err, boom) {
		t.Fatalf("ошибка КАТО-резолва должна прокидываться, получено: %v", err)
	}
}

// --- geocodeContracts (цикл) ---

func TestGeocodeContracts_MatchedAndUnmatched(t *testing.T) {
	contracts := []gen.ListContractsForGeocodeRow{
		contract(1, "M-1", "ул. Абая 10", "710000000"),
		contract(2, "U-1", "непонятный адрес xyz", "710000000"),
		contract(3, "N-1", "", "710000000"), // нет subject_ru → без сети, unmatched
	}
	conf := 0.7
	gc := &fakeGeocoder{matches: map[string]geo.Match{
		"ул. Абая 10, Астана": {Lat: 51.1, Lon: 71.4, Confidence: &conf},
	}}
	store := &fakeGeoStore{districtsByKato: map[string]int64{"710000000": 1}}

	st, err := geocodeContracts(context.Background(), contracts, gc, store, 0, io.Discard)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if st.Total != 3 || st.Matched != 1 || st.Unmatched != 2 {
		t.Errorf("st=%+v, ожидалось Total=3 Matched=1 Unmatched=2", st)
	}
	if len(store.upserted) != 3 {
		t.Fatalf("upserted = %d, ожидалось 3 (по одному на контракт)", len(store.upserted))
	}
	if store.upserted[0].GeocodeStatus != "auto" {
		t.Errorf("контракт M-1: status = %q, ожидалось auto", store.upserted[0].GeocodeStatus)
	}
	if store.upserted[1].GeocodeStatus != "unmatched" || store.upserted[2].GeocodeStatus != "unmatched" {
		t.Errorf("контракты U-1/N-1 должны быть unmatched: %+v / %+v", store.upserted[1], store.upserted[2])
	}
	// N-1 (пустой subject_ru) не должен был дёрнуть геокодер вообще (honest, без сети).
	if len(gc.calls) != 2 {
		t.Errorf("вызовов геокодера = %d, ожидалось 2 (M-1 и U-1 имеют адрес — U-1 не матчится, но запрос БЫЛ; N-1 без вызова)", len(gc.calls))
	}
}

// TestGeocodeContracts_GeocoderError_SkipsWriteRetryableNextRun — код-ревью нашёл: раньше транспортная
// ошибка геокодера писалась как status=unmatched, а ListContractsForGeocode НЕ пере-выбирает unmatched →
// один сетевой сбой НАВСЕГДА замораживал контракт. Теперь E-1 НЕ получает строку geo_objects вообще
// (upserted не растёт для него) — при следующем прогоне (geo_objects-строки нет) он снова будет выбран.
func TestGeocodeContracts_GeocoderError_SkipsWriteRetryableNextRun(t *testing.T) {
	contracts := []gen.ListContractsForGeocodeRow{
		contract(1, "E-1", "адрес1", ""),
		contract(2, "M-1", "адрес2", ""),
	}
	gc := &fakeGeocoder{
		fail:    map[string]error{"адрес1, Астана": errors.New("сеть недоступна")},
		matches: map[string]geo.Match{"адрес2, Астана": {Lat: 1, Lon: 2}},
	}
	store := &fakeGeoStore{}

	st, err := geocodeContracts(context.Background(), contracts, gc, store, 0, io.Discard)
	if err != nil {
		t.Fatalf("ошибка ОДНОГО контракта не должна обрывать batch: %v", err)
	}
	if st.Errors != 1 || st.Matched != 1 || st.Unmatched != 0 {
		t.Errorf("st=%+v, ожидалось Errors=1 Matched=1 Unmatched=0 (E-1 — ЧИСТО error, не вдобавок unmatched)", st)
	}
	if len(store.upserted) != 1 {
		t.Fatalf("upserted = %d, ожидалось 1 (только M-1 — E-1 НЕ записан, retryable следующим прогоном)", len(store.upserted))
	}
	if store.upserted[0].GeocodeStatus != "auto" {
		t.Errorf("единственная запись должна быть M-1/auto, получено %+v", store.upserted[0])
	}
}

// TestGeocodeContracts_DistrictResolutionError_SkipsWriteRetryableNextRun — та же логика для ошибки
// резолва района: НЕ пишем частично-честную строку (matched-точка была бы верной, а district_id молча
// стал бы NULL вместо «не смогли узнать»). Контракт целиком пропускается, retryable следующим прогоном.
func TestGeocodeContracts_DistrictResolutionError_SkipsWriteRetryableNextRun(t *testing.T) {
	contracts := []gen.ListContractsForGeocodeRow{contract(1, "D-1", "адрес", "710000000")}
	gc := &fakeGeocoder{matches: map[string]geo.Match{"адрес, Астана": {Lat: 1, Lon: 2}}}
	store := &fakeGeoStore{pointErr: errors.New("db down")} // matched=true → идёт по точке → ошибка инфраструктуры

	st, err := geocodeContracts(context.Background(), contracts, gc, store, 0, io.Discard)
	if err != nil {
		t.Fatalf("ошибка резолва района на ОДНОМ контракте не должна обрывать batch: %v", err)
	}
	if st.Errors != 1 || st.Matched != 0 || st.Unmatched != 0 {
		t.Errorf("st=%+v, ожидалось Errors=1 Matched=0 Unmatched=0", st)
	}
	if len(store.upserted) != 0 {
		t.Errorf("upserted = %d, ожидалось 0 (частично-честная строка НЕ пишется на инфраструктурной ошибке)", len(store.upserted))
	}
}

// TestGeocodeContracts_CleanPartition_ErrorsPlusMatchedPlusUnmatchedEqualsTotal — код-ревью нашёл:
// Matched/Unmatched/Errors раньше могли пересекаться (matched-контракт с district-ошибкой считался и в
// Matched, и в Errors). После фикса — чистое разбиение без пересечений на любом наборе исходов.
func TestGeocodeContracts_CleanPartition_ErrorsPlusMatchedPlusUnmatchedEqualsTotal(t *testing.T) {
	contracts := []gen.ListContractsForGeocodeRow{
		contract(1, "OK-1", "адрес-ок", ""),
		contract(2, "ERR-1", "адрес-ошибка", ""),
		contract(3, "MISS-1", "адрес-непонятный", ""),
	}
	gc := &fakeGeocoder{
		matches: map[string]geo.Match{"адрес-ок, Астана": {Lat: 1, Lon: 2}},
		fail:    map[string]error{"адрес-ошибка, Астана": errors.New("boom")},
	}
	store := &fakeGeoStore{}

	st, err := geocodeContracts(context.Background(), contracts, gc, store, 0, io.Discard)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if sum := st.Matched + st.Unmatched + st.Errors; sum != st.Total {
		t.Errorf("Matched(%d)+Unmatched(%d)+Errors(%d)=%d != Total(%d) — разбиение не чистое", st.Matched, st.Unmatched, st.Errors, sum, st.Total)
	}
}

func TestGeocodeContracts_UpsertError_AbortsWithError(t *testing.T) {
	contracts := []gen.ListContractsForGeocodeRow{contract(1, "X-1", "адрес", "")}
	gc := &fakeGeocoder{}
	store := &fakeGeoStore{upsertErr: errors.New("db down")}

	_, err := geocodeContracts(context.Background(), contracts, gc, store, 0, io.Discard)
	if err == nil {
		t.Fatal("ошибка записи в БД должна прерывать batch (не тихо глотаться)")
	}
}

func TestGeocodeContracts_CtxCancelled_StopsHonestly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // уже отменён до старта цикла
	contracts := []gen.ListContractsForGeocodeRow{contract(1, "X-1", "адрес", "")}
	st, err := geocodeContracts(ctx, contracts, &fakeGeocoder{}, &fakeGeoStore{}, 0, io.Discard)
	if err == nil {
		t.Fatal("отменённый ctx должен вернуть ошибку (честный выход, не тихое молчание)")
	}
	if st.Total != 1 {
		t.Errorf("st.Total = %d, ожидалось 1 (счётчик выставляется до цикла)", st.Total)
	}
}

func TestGeocodeContracts_PoliteDelay_OnlyWhenCalled(t *testing.T) {
	// N-1 без subject_ru не дёргает сеть → без паузы; при delay>0 весь тест не должен зависать на N-1.
	contracts := []gen.ListContractsForGeocodeRow{contract(1, "N-1", "", "")}
	start := time.Now()
	_, err := geocodeContracts(context.Background(), contracts, &fakeGeocoder{}, &fakeGeoStore{}, time.Hour, io.Discard)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("прогон занял %v — пауза вежливости применилась без сетевого вызова (баг)", elapsed)
	}
}

// --- run (код-ревью: вынесено из main() специально, чтобы это ядро стало тестируемым) ---

func TestRun_ContractSourceError_ReturnsOne(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), &fakeContractSource{err: errors.New("db down")}, &fakeGeocoder{}, &fakeGeoStore{}, 0, 0, &stdout, &stderr)
	if code != 1 {
		t.Errorf("code = %d, ожидался 1 (чтение контрактов упало)", code)
	}
	if !strings.Contains(stderr.String(), "ОШИБКА чтения контрактов") {
		t.Errorf("stderr = %q, ожидалось упоминание ошибки чтения", stderr.String())
	}
}

func TestRun_NoContracts_ReturnsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), &fakeContractSource{}, &fakeGeocoder{}, &fakeGeoStore{}, 0, 0, &stdout, &stderr)
	if code != 0 {
		t.Errorf("code = %d, ожидался 0 (честное «нет контрактов» — не ошибка)", code)
	}
	if !strings.Contains(stderr.String(), "нет контрактов") {
		t.Errorf("stderr = %q, ожидалось сообщение «нет контрактов»", stderr.String())
	}
}

// TestRun_MaxSlicing — код-ревью нашёл: contractSource никогда не принимался параметром, поэтому эта
// -max-логика (единственное место, где она живёт) была непротестирована.
func TestRun_MaxSlicing(t *testing.T) {
	rows := []gen.ListContractsForGeocodeRow{
		contract(1, "A", "", ""), contract(2, "B", "", ""), contract(3, "C", "", ""),
	}
	src := &fakeContractSource{rows: rows}
	store := &fakeGeoStore{}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), src, &fakeGeocoder{}, store, 0, 2, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, ожидался 0", code)
	}
	if len(store.upserted) != 2 {
		t.Errorf("upserted = %d, ожидалось 2 (-max=2 из 3 контрактов)", len(store.upserted))
	}
	if !strings.Contains(stderr.String(), "-max=2") {
		t.Errorf("stderr = %q, ожидалось упоминание -max=2", stderr.String())
	}
}

// TestRun_AllErrored_ReturnsOne — операционный сбой (геокодер/резолв недоступны на ВСЕХ контрактах) —
// НЕ честное 0% автопокрытие, отдельный код возврата.
func TestRun_AllErrored_ReturnsOne(t *testing.T) {
	rows := []gen.ListContractsForGeocodeRow{contract(1, "A", "адрес", ""), contract(2, "B", "адрес2", "")}
	src := &fakeContractSource{rows: rows}
	gc := &fakeGeocoder{fail: map[string]error{
		"адрес, Астана":  errors.New("boom"),
		"адрес2, Астана": errors.New("boom"),
	}}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), src, gc, &fakeGeoStore{}, 0, 0, &stdout, &stderr)
	if code != 1 {
		t.Errorf("code = %d, ожидался 1 (все контракты дали ошибку — операционный сбой, не 0%% coverage)", code)
	}
}

// TestRun_Success_PrintsCleanSummary — код-ревью нашёл: старая формулировка «без точки (вкл. ошибок)»
// подразумевала errors⊆unmatched, что после фикса неверно (Errors — отдельная ось). Новая строка честно
// разделяет три числа без намёка на пересечение.
func TestRun_Success_PrintsCleanSummary(t *testing.T) {
	rows := []gen.ListContractsForGeocodeRow{contract(1, "A", "адрес", "")}
	src := &fakeContractSource{rows: rows}
	gc := &fakeGeocoder{matches: map[string]geo.Match{"адрес, Астана": {Lat: 1, Lon: 2}}}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), src, gc, &fakeGeoStore{}, 0, 0, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, ожидался 0", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "сматчено=1") || !strings.Contains(out, "ошибок=0") {
		t.Errorf("stdout = %q, ожидались сматчено=1 и ошибок=0", out)
	}
}
