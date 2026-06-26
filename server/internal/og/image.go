package og

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strconv"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"ashyqqala/server/internal/httpapi"
	"ashyqqala/server/internal/registry"
)

// dejaVuTTF — встроенный шрифт (см. fonts/LICENSE.md): кириллица + казахские глифы + ₸. Рантайм не зависит
// от системных шрифтов. Текст прозы — из render/glossary (см. meta.go); здесь только отрисовка.
//
//go:embed fonts/DejaVuSans.ttf
var dejaVuTTF []byte

// ogFont — разобранный шрифт (один раз на процесс).
var ogFont *opentype.Font

func init() {
	f, err := opentype.Parse(dejaVuTTF)
	if err != nil {
		panic("og: разбор встроенного шрифта DejaVuSans.ttf: " + err.Error())
	}
	ogFont = f
}

// Размеры OG-картинки (рекомендация соцсетей 1.91:1).
const (
	ogW   = 1200
	ogH   = 630
	ogPad = 64
)

// Палитра: нейтральная, амбер для рамки сигнала — НИКОГДА алый (UX-DR39).
var (
	colBG     = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	colInk    = color.RGBA{0x1A, 0x1A, 0x1A, 0xFF}
	colMuted  = color.RGBA{0x6B, 0x72, 0x80, 0xFF}
	colAmberB = color.RGBA{0xFE, 0xF3, 0xC7, 0xFF} // amber-100 — фон пилюли
	colAmberI = color.RGBA{0x92, 0x40, 0x0E, 0xFF} // amber-800 — текст пилюли (тревожно-нейтрально, не алый)
	colRule   = color.RGBA{0xE5, 0xE7, 0xEB, 0xFF}
)

func face(pt float64) font.Face {
	f, err := opentype.NewFace(ogFont, &opentype.FaceOptions{Size: pt, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		// размеры — константы; ошибка тут невозможна на валидном шрифте, но не паникуем в рантайме рендера.
		f, _ = opentype.NewFace(ogFont, &opentype.FaceOptions{Size: 24, DPI: 72})
	}
	return f
}

// Image обслуживает GET /og/contracts/{goszakup_id}/image.png — серверный PNG-превью карточки (FR-27, AR-14:
// видимый короткий URL + версия методики «впечатаны» в картинку → пересланный скриншот остаётся проверяемым).
// Та же проекция, что у SPA/мета; текст — через render/glossary; честные состояния словами.
func (h MetaHandler) Image(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "goszakup_id")
	loc := resolveLocale(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dto, err := h.Contracts.Projection(ctx, id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "not found", http.StatusNotFound)
		return
	case err != nil:
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	img := h.renderImage(dto, loc)
	// Кодируем в буфер ДО заголовков: сбой Encode не должен оставить клиенту 200 с битым PNG (краулер
	// соцсети закэширует повреждённое превью). Только успешный PNG уходит с 200 + Content-Length.
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		if h.Log != nil {
			h.Log.Error("og_image_encode_failed", "error", err.Error(), "goszakup_id", id)
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600") // версия в URL (AR-20) отвечает за инвалидацию
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	_, _ = w.Write(buf.Bytes())
}

// renderImage рисует PNG из проекции. Все строки прозы — те же, что в HTML-мете (subjectText/amountText/FlagLine).
func (h MetaHandler) renderImage(dto httpapi.ContractDTO, loc registry.Locale) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, ogW, ogH))
	fillRect(img, img.Bounds(), colBG)

	x := ogPad
	contentW := ogW - 2*ogPad
	// Брендовая строка + версия методики (впечатана — AR-14).
	drawText(img, face(28), x, 92, colMuted, "AshyqQala.kz")
	if h.Version != "" {
		vw := textWidth(face(24), h.Version)
		drawText(img, face(24), ogW-ogPad-vw, 90, colMuted, h.Version)
	}
	hLine(img, x, ogW-ogPad, 120, colRule)

	// Заголовок (предмет), перенос по словам, максимум 3 строки.
	const titleTop, titleLH = 190, 66
	lines := wrapText(face(54), h.subjectText(dto, loc), contentW, 3)
	for i, line := range lines {
		drawText(img, face(54), x, titleTop+i*titleLH, colInk, line)
	}

	// Сумма — динамически под заголовком.
	amountY := titleTop + (len(lines)-1)*titleLH + 96
	drawText(img, face(40), x, amountY, colInk, h.amountText(dto.AmountTng, loc))

	// Сигнал: амбер-пилюля с ПОЛНОЙ нейтральной рамкой (raised, влезает в ширину) ИЛИ честное «активных
	// сигналов нет». Пилюля — однострочная, шрифт подобран так, чтобы рамка не обрезалась.
	pillTop := amountY + 36
	var flagLine string
	for _, f := range dto.Flags {
		if f.State == registry.FlagRaised {
			flagLine = h.Renderer.FlagLine(f.FlagID, f.State, loc)
			break
		}
	}
	if flagLine != "" {
		drawPill(img, x, pillTop, contentW, face(30), flagLine)
	} else {
		// Честно (как в HTML-мете): insufficient → «недостаточно сопоставимых данных» (неоценённое не
		// маскируется «чистым»); все not_raised → «активных сигналов нет». §7.4, урок A.
		drawText(img, face(30), x, pillTop+40, colMuted, h.nonRaisedSummary(dto, loc))
	}

	// Подвал: короткий URL + защитные связи (методика + канал ошибки — UX-DR28, видимы на картинке).
	hLine(img, x, ogW-ogPad, ogH-110, colRule)
	drawText(img, face(26), x, ogH-64, colMuted, "ashyqqala.kz/contracts/"+dto.GoszakupContractID)
	links := h.Renderer.Text(loc, "link.methodology") + "  ·  " + h.Renderer.Text(loc, "link.report_error")
	lw := textWidth(face(26), links)
	drawText(img, face(26), ogW-ogPad-lw, ogH-64, colMuted, links)

	return img
}

func fillRect(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	for yy := rect.Min.Y; yy < rect.Max.Y; yy++ {
		for xx := rect.Min.X; xx < rect.Max.X; xx++ {
			img.SetRGBA(xx, yy, c)
		}
	}
}

func hLine(img *image.RGBA, x0, x1, y int, c color.RGBA) {
	fillRect(img, image.Rect(x0, y, x1, y+2), c)
}

func drawText(img *image.RGBA, f font.Face, x, baseline int, c color.RGBA, s string) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: f, Dot: fixed.P(x, baseline)}
	d.DrawString(s)
}

func textWidth(f font.Face, s string) int {
	return font.MeasureString(f, s).Round()
}

// drawPill — амбер-пилюля под нейтральную рамку сигнала; однострочная, по ширине maxW (с усечением «…»,
// если текст экстремально длинный — для штатных флагов влезает целиком). topY — верх пилюли.
func drawPill(img *image.RGBA, x, topY, maxW int, f font.Face, s string) {
	const padX, padY = 24, 14
	line := fitLine(f, s, maxW-2*padX)
	tw := textWidth(f, line)
	m := f.Metrics()
	asc, desc := m.Ascent.Round(), m.Descent.Round()
	h := asc + desc + 2*padY
	fillRect(img, image.Rect(x, topY, x+tw+2*padX, topY+h), colAmberB)
	drawText(img, f, x+padX, topY+padY+asc, colAmberI, line)
}

// fitLine — усекает строку с «…», если она шире maxW (px). Если влезает — возвращает как есть.
func fitLine(f font.Face, s string, maxW int) string {
	if textWidth(f, s) <= maxW {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && textWidth(f, string(r)+"…") > maxW {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// wrapText — жадный перенос по словам до maxW (px), максимум maxLines (последняя усекается «…»).
func wrapText(f font.Face, s string, maxW, maxLines int) []string {
	if s == "" {
		return []string{""}
	}
	var words []string
	cur := ""
	for _, r := range s {
		// Разбиваем по ЛЮБОМУ пробельному символу Unicode (вкл. U+00A0 NBSP, таб, перенос строки) —
		// госданные часто содержат NBSP; иначе весь заголовок стал бы одним «словом» и выехал за край.
		if unicode.IsSpace(r) {
			if cur != "" {
				words = append(words, cur)
				cur = ""
			}
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		words = append(words, cur)
	}

	var lines []string
	line := ""
	for _, wd := range words {
		try := wd
		if line != "" {
			try = line + " " + wd
		}
		if textWidth(f, try) <= maxW {
			line = try
			continue
		}
		// wd не влезает к текущей строке.
		if len(lines) == maxLines-1 {
			// Уже заполняем ПОСЛЕДНЮЮ допустимую строку: новых строк нет. Сохраняем максимально
			// заполненную last-строку (а не обрезаем её до одного слова) — остаток усечётся «…» ниже.
			break
		}
		if line != "" {
			lines = append(lines, line)
		}
		line = wd
	}
	if line != "" && len(lines) < maxLines {
		lines = append(lines, line)
	}
	// Усечение последней строки многоточием, если текст не влез.
	if len(lines) == maxLines {
		last := lines[maxLines-1]
		for textWidth(f, last+"…") > maxW && last != "" {
			last = trimLastRune(last)
		}
		joined := 0
		for _, l := range lines {
			joined += len([]rune(l)) + 1
		}
		if joined < len([]rune(s))+1 {
			lines[maxLines-1] = last + "…"
		}
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

func trimLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}

// glyphPresent — есть ли в шрифте глиф руны (idx 0 == .notdef). Для тестов покрытия казахских букв + ₸.
func glyphPresent(r rune) bool {
	var buf sfnt.Buffer
	idx, err := ogFont.GlyphIndex(&buf, r)
	return err == nil && idx != 0
}
