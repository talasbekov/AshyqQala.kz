package og

import (
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"ashyqqala/server/internal/httpapi"
	"ashyqqala/server/internal/registry"
)

// TestDumpSampleImages — dev-утилита визуальной проверки: при OG_DUMP_DIR=<dir> пишет образцы PNG.
// По умолчанию ПРОПУСКАЕТСЯ (в CI файлов не создаёт).
func TestDumpSampleImages(t *testing.T) {
	dir := os.Getenv("OG_DUMP_DIR")
	if dir == "" {
		t.Skip("установите OG_DUMP_DIR, чтобы выгрузить образцы OG-картинок")
	}
	h := MetaHandler{Renderer: ogRenderer(t), Version: "v1.0"}
	longSubj := raisedDTO()
	longSubj.GoszakupContractID = "DEMO-LONG"
	longSubj.SubjectRu = okF("Капитальный ремонт автомобильной дороги областного значения на участке от посёлка до развязки с реконструкцией водопропускных сооружений")
	cases := map[string]struct {
		dto httpapi.ContractDTO
		loc registry.Locale
	}{
		"raised_kk":       {raisedDTO(), registry.KK},
		"raised_ru":       {raisedDTO(), registry.RU},
		"noflags_ru":      {noFlagsDTO(), registry.RU},      // частично неоценён → «недостаточно сопоставимых данных»
		"allnotraised_ru": {allNotRaisedDTO(), registry.RU}, // все not_raised → «активных сигналов нет»
		"raised_long_ru":  {longSubj, registry.RU},          // заголовок на 3 строки (проверка wrapText)
	}
	for name, c := range cases {
		f, err := os.Create(filepath.Join(dir, "og_"+name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, h.renderImage(c.dto, c.loc)); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
}

func newImageServer(t *testing.T, m mockProjector) *httptest.Server {
	t.Helper()
	h := MetaHandler{Contracts: m, Renderer: ogRenderer(t), Version: "v1.0"}
	r := chi.NewRouter()
	r.Get("/og/contracts/{goszakup_id}/image.png", h.Image)
	return httptest.NewServer(r)
}

// TestOGFont_CoversKazakhAndTenge — ГАРАНТИЯ, что встроенный шрифт содержит казахские глифы и ₸ (иначе текст
// отрендерится квадратами .notdef). Это и есть проверка, ради которой выбирали шрифт с покрытием kk.
func TestOGFont_CoversKazakhAndTenge(t *testing.T) {
	for _, r := range []rune{'ә', 'ғ', 'қ', 'ң', 'ө', 'ұ', 'ү', 'һ', 'і', '₸', '—', '·'} {
		if !glyphPresent(r) {
			t.Errorf("шрифт DejaVuSans не содержит глиф %q (U+%04X) — отрендерится .notdef", r, r)
		}
	}
}

// TestImageHandler_RendersPNG — эндпоинт отдаёт валидный PNG 1200×630, Content-Type image/png, непустой.
func TestImageHandler_RendersPNG(t *testing.T) {
	srv := newImageServer(t, mockProjector{dto: raisedDTO()})
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/og/contracts/DEMO-0001/image.png?v=v1.0&lang=kk")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d; want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type=%q; want image/png", ct)
	}
	img, err := png.Decode(resp.Body)
	if err != nil {
		t.Fatalf("ответ не декодируется как PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != ogW || b.Dy() != ogH {
		t.Errorf("размеры %dx%d; want %dx%d", b.Dx(), b.Dy(), ogW, ogH)
	}
	if blank(img) {
		t.Error("картинка пустая (только фон) — текст/элементы не отрисованы")
	}
}

// TestRenderImage_HonestNoData_AllLocales — рендер не паникует для no_data/raised/no-flags в обеих локалях
// (одно битое/пустое поле не валит картинку; честные состояния — словами).
func TestRenderImage_HonestNoData_AllLocales(t *testing.T) {
	h := MetaHandler{Renderer: ogRenderer(t), Version: "v1.0"}
	noData := httpapi.ContractDTO{
		GoszakupContractID: "DEMO-NULL",
		SubjectRu:          noF(), SubjectKk: noF(), AmountTng: noF(), SourceURL: noF(),
		Flags: []httpapi.ContractFlagDTO{
			{FlagID: "single_participant", State: registry.FlagInsufficientData},
			{FlagID: "price_per_km", State: registry.FlagInsufficientData},
		},
	}
	longSubj := raisedDTO()
	longSubj.SubjectKk = okF("Бір қатысушы тіркелді кезінде автомобиль жолын күрделі жөндеу және сумен жабдықтау желілерін қайта жаңарту жөніндегі ұзақ мерзімді мемлекеттік келісімшарт")
	for _, dto := range []httpapi.ContractDTO{noData, raisedDTO(), noFlagsDTO(), longSubj} {
		for _, loc := range registry.AllLocales() {
			img := h.renderImage(dto, loc)
			if img.Bounds().Dx() != ogW || img.Bounds().Dy() != ogH {
				t.Fatalf("неверные размеры для %s", loc)
			}
		}
	}
}

// TestImageHandler_NotFound — контракт не найден → 404 (не выдуманная картинка).
func TestImageHandler_NotFound(t *testing.T) {
	srv := newImageServer(t, mockProjector{err: pgx.ErrNoRows})
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/og/contracts/NOPE/image.png")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d; want 404", resp.StatusCode)
	}
}

// blank — true, если на картинке только фоновый цвет (ничего не нарисовано).
func blank(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += 5 {
		for x := b.Min.X; x < b.Max.X; x += 5 {
			r, g, bl, _ := img.At(x, y).RGBA()
			if !(r>>8 == 0xFF && g>>8 == 0xFF && bl>>8 == 0xFF) {
				return false
			}
		}
	}
	return true
}
