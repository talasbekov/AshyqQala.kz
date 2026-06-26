package og

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"ashyqqala/server/internal/httpapi"
	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/render"
)

func okF(s string) httpapi.Field[string] {
	return httpapi.Field[string]{Value: &s, State: registry.StateOK}
}
func noF() httpapi.Field[string] { return httpapi.Field[string]{State: registry.StateNoData} }

func ogRenderer(t *testing.T) render.Renderer {
	t.Helper()
	reg, err := registry.Load("../../../registry")
	if err != nil {
		t.Fatalf("registry load: %v", err)
	}
	return render.Renderer{Reg: reg}
}

type mockProjector struct {
	dto httpapi.ContractDTO
	err error
}

func (m mockProjector) Projection(context.Context, string) (httpapi.ContractDTO, error) {
	return m.dto, m.err
}

func newOGServer(t *testing.T, m mockProjector) *httptest.Server {
	t.Helper()
	h := MetaHandler{Contracts: m, Renderer: ogRenderer(t), Version: "v1.0"}
	r := chi.NewRouter()
	r.Get("/og/contracts/{goszakup_id}", h.ServeHTTP)
	return httptest.NewServer(r)
}

func raisedDTO() httpapi.ContractDTO {
	return httpapi.ContractDTO{
		GoszakupContractID: "DEMO-0001",
		SubjectRu:          okF("Ремонт автодороги"),
		SubjectKk:          okF("Автожол жөндеу"),
		AmountTng:          okF("123456789"),
		SourceURL:          okF("https://goszakup.gov.kz/ru/contract/DEMO-0001"),
		Flags: []httpapi.ContractFlagDTO{
			{FlagID: "single_participant", State: registry.FlagRaised, MethodologyVersion: okF("v1.0"), DetectedAt: okF("2026-03-15T10:00:00Z")},
			{FlagID: "price_per_km", State: registry.FlagInsufficientData, MethodologyVersion: noF(), DetectedAt: noF()},
		},
	}
}

// noFlagsDTO — нет raised, но один флаг not_raised, другой insufficient_data (частично НЕ оценён).
// Честно (P0): такая карточка показывает «недостаточно сопоставимых данных», НЕ «активных сигналов нет»
// (иначе неоценённый флаг читался бы как «проверено — чисто»).
func noFlagsDTO() httpapi.ContractDTO {
	return httpapi.ContractDTO{
		GoszakupContractID: "DEMO-0002",
		SubjectRu:          okF("Водопровод"),
		SubjectKk:          okF("Сумен жабдықтау"),
		AmountTng:          okF("5000000"),
		SourceURL:          okF("https://goszakup.gov.kz/ru/contract/DEMO-0002"),
		Flags: []httpapi.ContractFlagDTO{
			{FlagID: "single_participant", State: registry.FlagNotRaised, MethodologyVersion: okF("v1.0"), DetectedAt: okF("2026-01-01T00:00:00Z")},
			{FlagID: "price_per_km", State: registry.FlagInsufficientData},
		},
	}
}

// allNotRaisedDTO — ВСЕ флаги not_raised: доказано оценено и сигнал не сработал → честно «активных сигналов
// нет» (единственный случай, когда «нет активных сигналов» правомерно, P0).
func allNotRaisedDTO() httpapi.ContractDTO {
	return httpapi.ContractDTO{
		GoszakupContractID: "DEMO-0003",
		SubjectRu:          okF("Водопровод"),
		SubjectKk:          okF("Сумен жабдықтау"),
		AmountTng:          okF("5000000"),
		SourceURL:          okF("https://goszakup.gov.kz/ru/contract/DEMO-0003"),
		Flags: []httpapi.ContractFlagDTO{
			{FlagID: "single_participant", State: registry.FlagNotRaised, MethodologyVersion: okF("v1.0"), DetectedAt: okF("2026-01-01T00:00:00Z")},
			{FlagID: "price_per_km", State: registry.FlagNotRaised, MethodologyVersion: okF("v1.0"), DetectedAt: okF("2026-01-01T00:00:00Z")},
		},
	}
}

func getBody(t *testing.T, url string) (string, int) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return string(b), resp.StatusCode
}

func metaContent(t *testing.T, html, prop string) string {
	t.Helper()
	re := regexp.MustCompile(`<meta property="` + regexp.QuoteMeta(prop) + `" content="([^"]*)"`)
	m := re.FindStringSubmatch(html)
	if m == nil {
		t.Fatalf("meta %q не найден в ответе:\n%s", prop, html)
	}
	return m[1]
}

// TestMetaHandler_RaisedFlag_OGTags — серверный HTML с обязательными OG-тегами для raised-карточки (kk дефолт):
// заголовок/описание/картинка/локаль, ОБЯЗАТЕЛЬНЫЙ непустой og:image:alt, версионированный og:image (cache-bust),
// нейтральная рамка, защитные связи (методика + канал ошибки) — UX-DR36/UX-DR28/AR-20.
func TestMetaHandler_RaisedFlag_OGTags(t *testing.T) {
	srv := newOGServer(t, mockProjector{dto: raisedDTO()})
	defer srv.Close()
	body, status := getBody(t, srv.URL+"/og/contracts/DEMO-0001")
	if status != http.StatusOK {
		t.Fatalf("status=%d; want 200", status)
	}
	if ct := metaContent(t, body, "og:title"); ct == "" {
		t.Error("og:title пуст")
	}
	if alt := metaContent(t, body, "og:image:alt"); strings.TrimSpace(alt) == "" {
		t.Error("og:image:alt ОБЯЗАТЕЛЕН и непуст (AR-14/All-Surfaces)")
	}
	if loc := metaContent(t, body, "og:locale"); loc != "kk_KZ" {
		t.Errorf("og:locale=%q; want kk_KZ (дефолт)", loc)
	}
	img := metaContent(t, body, "og:image")
	if !strings.Contains(img, "v=v1.0") {
		t.Errorf("og:image не версионирован methodology_version (cache-bust AR-20): %q", img)
	}
	if !strings.Contains(img, "as_of=") {
		t.Errorf("og:image без as_of при raised-флаге (AR-20): %q", img)
	}
	// Нейтральная рамка (kk) для raised-флага
	if !strings.Contains(body, "тексеруді талап ететін сигнал") {
		t.Error("нет нейтральной рамки «сигнал, требующий проверки» (kk) для raised-флага")
	}
	// Защитные связи (UX-DR28): методика + канал ошибки достижимы
	if !strings.Contains(body, "Қалай есептелді") {
		t.Error("нет ссылки на методику (UX-DR28 путь а)")
	}
	if !strings.Contains(body, "Қате туралы хабарлау") || !strings.Contains(body, "?report") {
		t.Error("нет канала «Сообщить об ошибке» / deep-link ?report (UX-DR28 путь б)")
	}
	// Версия методики присутствует явно
	if v := metaContent(t, body, "og:type"); v == "" { // sanity: meta-парсер работает
		t.Error("og:type пуст")
	}
}

// TestMetaHandler_LocaleRu — ?lang=ru → og:locale ru_KZ, ru-предмет, ru-рамка.
func TestMetaHandler_LocaleRu(t *testing.T) {
	srv := newOGServer(t, mockProjector{dto: raisedDTO()})
	defer srv.Close()
	body, status := getBody(t, srv.URL+"/og/contracts/DEMO-0001?lang=ru")
	if status != http.StatusOK {
		t.Fatalf("status=%d; want 200", status)
	}
	if loc := metaContent(t, body, "og:locale"); loc != "ru_KZ" {
		t.Errorf("og:locale=%q; want ru_KZ", loc)
	}
	if !strings.Contains(body, "Ремонт автодороги") {
		t.Error("нет ru-предмета")
	}
	if !strings.Contains(body, "сигнал, требующий проверки") {
		t.Error("нет ru-рамки")
	}
}

// TestMetaHandler_PerSurfaceNeutrality — вся ГЕНЕРИРУЕМАЯ OG-проза (заголовок/описание/alt/метки) свободна
// от taboo в ОБЕИХ локалях и для raised/не-raised. + negative-control: матчер доказанно краснеет ([[guards-must-prove-red]]).
func TestMetaHandler_PerSurfaceNeutrality(t *testing.T) {
	rd := ogRenderer(t)
	h := MetaHandler{Renderer: rd, Version: "v1.0"}
	for _, loc := range registry.AllLocales() {
		for _, dto := range []httpapi.ContractDTO{raisedDTO(), noFlagsDTO(), allNotRaisedDTO()} {
			v := h.build(dto, loc)
			for _, s := range []string{v.Title, v.Description, v.ImageAlt, v.SourceLabel, v.MethodologyLabel, v.ReportLabel} {
				if hits := rd.Reg.FindTaboo(loc, s); len(hits) > 0 {
					t.Errorf("[%s] OG-проза %q содержит taboo %v", loc, s, hits)
				}
			}
		}
	}
	// negative-control: страж обязан доказать, что краснеет
	if hits := rd.Reg.FindTaboo(registry.RU, "нарушение в контракте"); len(hits) == 0 {
		t.Error("negative-control: FindTaboo не покраснел на явном taboo — страж не доказал, что краснеет")
	}
}

// TestMetaHandler_AllNotRaised_HonestSignalsNone — ВСЕ флаги not_raised (доказано оценено и чисто) →
// честное «активных сигналов нет» (не пустая «чистая», §7.4). Единственный правомерный случай этой строки.
func TestMetaHandler_AllNotRaised_HonestSignalsNone(t *testing.T) {
	h := MetaHandler{Renderer: ogRenderer(t), Version: "v1.0"}
	v := h.build(allNotRaisedDTO(), registry.RU)
	if !strings.Contains(v.Description, "Активных сигналов нет") {
		t.Errorf("все not_raised → ожидалось «Активных сигналов нет», got %q", v.Description)
	}
}

// TestMetaHandler_PartiallyUnevaluated_HonestInsufficient — P0/decision (вариант 1): карточка БЕЗ raised, но
// хотя бы с одним insufficient_data (НЕ оценён) НЕ должна выдавать «активных сигналов нет» (= «проверено,
// чисто»); честно «недостаточно сопоставимых данных». Урок A honesty-hole 5.1, §7.4.
func TestMetaHandler_PartiallyUnevaluated_HonestInsufficient(t *testing.T) {
	h := MetaHandler{Renderer: ogRenderer(t), Version: "v1.0"}
	v := h.build(noFlagsDTO(), registry.RU) // not_raised + insufficient_data
	if !strings.Contains(v.Description, "недостаточно сопоставимых данных") {
		t.Errorf("частично неоценённая карточка → ожидалось «недостаточно сопоставимых данных», got %q", v.Description)
	}
	if strings.Contains(v.Description, "Активных сигналов нет") {
		t.Errorf("insufficient НЕ должен маскироваться под «Активных сигналов нет» (honesty-hole): %q", v.Description)
	}
	// Растр обязан показать ТУ ЖЕ честную строку (не «активных сигналов нет»).
	if got := h.nonRaisedSummary(noFlagsDTO(), registry.RU); got != "недостаточно сопоставимых данных" {
		t.Errorf("nonRaisedSummary (insufficient) = %q; want «недостаточно сопоставимых данных»", got)
	}
	if got := h.nonRaisedSummary(allNotRaisedDTO(), registry.RU); got != "Активных сигналов нет" {
		t.Errorf("nonRaisedSummary (все not_raised) = %q; want «Активных сигналов нет»", got)
	}
}

// TestMetaHandler_MissingFields_HonestNoData — отсутствующие поля → честное «нет данных», НЕ «0»/«0 ₸».
func TestMetaHandler_MissingFields_HonestNoData(t *testing.T) {
	h := MetaHandler{Renderer: ogRenderer(t), Version: "v1.0"}
	dto := httpapi.ContractDTO{
		GoszakupContractID: "DEMO-NULL",
		SubjectRu:          noF(), SubjectKk: noF(), AmountTng: noF(), SourceURL: noF(),
		Flags: []httpapi.ContractFlagDTO{
			{FlagID: "single_participant", State: registry.FlagInsufficientData},
			{FlagID: "price_per_km", State: registry.FlagInsufficientData},
		},
	}
	v := h.build(dto, registry.RU)
	if v.Title != "нет данных" {
		t.Errorf("title (NULL) = %q; want «нет данных»", v.Title)
	}
	if strings.Contains(v.Description, "0 ₸") {
		t.Errorf("деньги no_data НЕ должны стать «0 ₸»: %q", v.Description)
	}
	if !strings.Contains(v.Description, "нет данных") {
		t.Errorf("ожидалось честное «нет данных» в описании: %q", v.Description)
	}
	if v.SourcePresent {
		t.Error("source no_data → SourcePresent=false (не отдавать битую ссылку-первоисточник)")
	}
	// Оба флага insufficient_data → честное «недостаточно сопоставимых данных», НЕ «активных сигналов нет».
	if !strings.Contains(v.Description, "недостаточно сопоставимых данных") {
		t.Errorf("оба флага insufficient → ожидалось «недостаточно сопоставимых данных»: %q", v.Description)
	}
	if strings.Contains(v.Description, "Активных сигналов нет") {
		t.Errorf("неоценённая карточка НЕ должна показывать «Активных сигналов нет»: %q", v.Description)
	}
	if strings.TrimSpace(v.ImageAlt) == "" {
		t.Error("og:image:alt обязателен и непуст даже при no_data")
	}
}

// TestMetaHandler_EmptyVersion_NoEmptyVParam — пустая methodology_version НЕ попадает в og:image как «v=»
// (honesty hole 5.1: пустое не маскируется под «версия есть»). Negative-control: при "v1.0" v= ПРИСУТСТВУЕТ.
func TestMetaHandler_EmptyVersion_NoEmptyVParam(t *testing.T) {
	rd := ogRenderer(t)
	empty := MetaHandler{Renderer: rd, Version: ""}
	if got := empty.imageURL(raisedDTO(), registry.RU); strings.Contains(got, "v=") {
		t.Errorf("пустая версия не должна давать «v=» в og:image: %q", got)
	}
	ok := MetaHandler{Renderer: rd, Version: "v1.0"}
	if got := ok.imageURL(raisedDTO(), registry.RU); !strings.Contains(got, "v=v1.0") {
		t.Errorf("negative-control: версия v1.0 должна дать v=v1.0: %q", got)
	}
}

// TestMetaHandler_NotFound — контракт не найден → 404 (honest, не выдуманная карточка).
func TestMetaHandler_NotFound(t *testing.T) {
	srv := newOGServer(t, mockProjector{err: pgx.ErrNoRows})
	defer srv.Close()
	_, status := getBody(t, srv.URL+"/og/contracts/NOPE")
	if status != http.StatusNotFound {
		t.Fatalf("status=%d; want 404", status)
	}
}

// TestOGProza_AllFlagStates_NoTaboo — AC-4 дословно: per-surface нейтральность OG по КАЖДОЙ генерируемой
// прозе-строке × обе локали × ВСЕ 4 состояния флага = 0 taboo. Закрывает пробел узкого raised+noFlags
// покрытия (price_per_km.summary и не-raised FlagLine ранее не проходили FindTaboo ни на одной поверхности).
// Картинка рисует ровно эти же render/glossary-строки (subjectText — сырые данные источника, правомерно вне
// taboo по UX-DR27), поэтому их нейтральность доказывает и нейтральность текста растра.
func TestOGProza_AllFlagStates_NoTaboo(t *testing.T) {
	rd := ogRenderer(t)
	for _, loc := range registry.AllLocales() {
		for _, flag := range []string{"single_participant", "price_per_km"} {
			for _, fs := range registry.AllFlagStates() {
				s := rd.FlagLine(flag, fs, loc)
				if hits := rd.Reg.FindTaboo(loc, s); len(hits) > 0 {
					t.Errorf("[%s] FlagLine(%s,%s)=%q содержит taboo %v", loc, flag, fs, s, hits)
				}
			}
		}
		// Карточные сводки сигналов (обе ветви nonRaisedSummary) + нейтральная рамка — тоже без taboo.
		for _, s := range []string{
			rd.Text(loc, "signals.none"),
			rd.RenderValueState(registry.StateInsufficientSample, loc),
			rd.Text(loc, "frame.signal"),
		} {
			if hits := rd.Reg.FindTaboo(loc, s); len(hits) > 0 {
				t.Errorf("[%s] OG-сводка %q содержит taboo %v", loc, s, hits)
			}
		}
	}
	// negative-control: матчер обязан доказать, что краснеет ([[guards-must-prove-red]]).
	if hits := rd.Reg.FindTaboo(registry.RU, "нарушение в контракте"); len(hits) == 0 {
		t.Error("negative-control: FindTaboo не покраснел на явном taboo — страж не доказал, что краснеет")
	}
}

// TestMetaHandler_MethodologyVersion_Pinned — methodology_version (5-й элемент ядра AC-4) пинится ЛИТЕРАЛОМ
// на OG-поверхности: cache-bust og:image (AR-20) + тег aq:methodology_version несут ОДНУ каноническую версию
// (та же params.MethodologyVersion, что читает web → версии поверхностей не расходятся). Дополняет
// render.TestFlagLine_CrossSurfaceGolden, где версии нет (FlagLine = рамка+саммари без версии).
func TestMetaHandler_MethodologyVersion_Pinned(t *testing.T) {
	srv := newOGServer(t, mockProjector{dto: raisedDTO()}) // newOGServer ставит Version: "v1.0"
	defer srv.Close()
	body, status := getBody(t, srv.URL+"/og/contracts/DEMO-0001")
	if status != http.StatusOK {
		t.Fatalf("status=%d; want 200", status)
	}
	if !strings.Contains(body, `<meta name="aq:methodology_version" content="v1.0">`) {
		t.Errorf("тег aq:methodology_version не запинен на v1.0:\n%s", body)
	}
	if img := metaContent(t, body, "og:image"); !strings.Contains(img, "v=v1.0") {
		t.Errorf("og:image cache-bust версия не v1.0: %q", img)
	}
}

// TestMetaHandler_AmountInHTML — позитив (FR-27 «карточка отдаёт сумму»): отформатированная СЕРВЕРНЫМ
// форматтером сумма (группировка U+00A0 + « ₸», как у SPA) ПРИСУТСТВУЕТ в OG-HTML. Дополняет негативные
// проверки (не «0 ₸» / «нет данных»).
func TestMetaHandler_AmountInHTML(t *testing.T) {
	h := MetaHandler{Renderer: ogRenderer(t), Version: "v1.0"}
	v := h.build(raisedDTO(), registry.RU) // AmountTng = "123456789"
	nb := " "
	want := "123" + nb + "456" + nb + "789 ₸"
	if !strings.Contains(v.Description, want) {
		t.Errorf("OG-описание не содержит сгруппированную сумму %q: %q", want, v.Description)
	}
	if !strings.Contains(v.ImageAlt, want) {
		t.Errorf("og:image:alt не содержит сумму %q: %q", want, v.ImageAlt)
	}
}
