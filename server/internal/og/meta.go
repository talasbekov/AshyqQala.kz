// Package og — OG-рендер карточки контракта из ТОЙ ЖЕ проекции, что и SPA (FR-27, UX-DR36, Story 5.5).
// Текст прозы флага — ТОЛЬКО через render (AR-14: «не второй страж», единый источник прозы). OG —
// адаптер представления (boundary: импортирует httpapi/render/registry; pure-core его НЕ импортирует).
//
// MetaHandler отдаёт СЕРВЕРНЫЙ HTML с OG-`<meta>`-тегами (не клиентский JS — краулеры соцсетей JS не
// исполняют). Динамическую PNG-картинку (image/draw + шрифт с казахскими глифами) добавляет отдельная
// работа (T5, зависит от golang.org/x/image + векторного шрифта — нужно одобрение): здесь og:image —
// версионированный по methodology_version URL (cache-bust, AR-20), сам PNG отдаёт будущий /og/.../*.png.
package og

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"ashyqqala/server/internal/httpapi"
	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/render"
)

// contractProjector — единственная зависимость от данных: ТА ЖЕ проекция, что у JSON-карточки (Get).
// Реализуется httpapi.ContractsHandler.Projection. Узкий интерфейс → лёгкий мок в тестах + структурная
// гарантия «цифры не расходятся с SPA» (AC-1): источник один и тот же.
type contractProjector interface {
	Projection(ctx context.Context, goszakupID string) (httpapi.ContractDTO, error)
}

// MetaHandler — обработчик GET /og/contracts/{goszakup_id} (HTML с OG-тегами).
type MetaHandler struct {
	Contracts contractProjector
	Renderer  render.Renderer
	Version   string // каноническая methodology_version — cache-bust og:image (AR-20)
	BaseURL   string // публичный origin для абсолютных og:url/og:image; "" → относительные пути
	Log       *slog.Logger
}

// ogView — данные шаблона OG-страницы. Все строки прозы — из render/glossary (единый источник, нейтральны).
type ogView struct {
	Lang             string // <html lang> — "kk"/"ru"
	OGLocale         string // og:locale — "kk_KZ"/"ru_KZ"
	Title            string
	Description      string
	ImageURL         string
	ImageAlt         string // ОБЯЗАТЕЛЕН и непуст (AR-14/All-Surfaces)
	URL              string
	Version          string
	SourcePresent    bool
	SourceURL        string
	SourceLabel      string
	MethodologyURL   string // UX-DR28 путь (а): методика/пересчёт
	MethodologyLabel string
	ReportURL        string // UX-DR28 путь (б): канал «Сообщить об ошибке» (deep-link 5.4)
	ReportLabel      string
}

// ServeHTTP отдаёт серверный HTML с OG-тегами. Данные — из общей Projection (та же, что JSON-карточка).
func (h MetaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "goszakup_id")
	loc := resolveLocale(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dto, err := h.Contracts.Projection(ctx, id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		h.writeError(w, http.StatusNotFound, loc)
		return
	case err != nil:
		h.writeError(w, http.StatusInternalServerError, loc)
		return
	}

	// Рендерим в буфер ДО WriteHeader: сбой Execute не должен оставить клиенту 200 с усечённым HTML
	// (краулер соцсети закэширует битую страницу). Только успешный рендер уходит с 200.
	var buf bytes.Buffer
	if err := ogTemplate.Execute(&buf, h.build(dto, loc)); err != nil {
		if h.Log != nil {
			h.Log.Error("og_template_execute_failed", "error", err.Error(), "goszakup_id", id)
		}
		h.writeError(w, http.StatusInternalServerError, loc)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// build собирает ogView из проекции. Честные состояния явно (§7.4): отсутствующее поле → «нет данных»
// (не «0»); нет активных флагов → честное «активных сигналов нет» (не пустая «чистая» карточка).
func (h MetaHandler) build(dto httpapi.ContractDTO, loc registry.Locale) ogView {
	subject := h.subjectText(dto, loc)
	amount := h.amountText(dto.AmountTng, loc)
	signalsPart := h.signalsSummary(dto, loc)

	cardURL := h.abs("/contracts/" + url.PathEscape(dto.GoszakupContractID))
	source, sourcePresent := fieldValue(dto.SourceURL)

	return ogView{
		Lang:             string(loc),
		OGLocale:         ogLocale(loc),
		Title:            subject,
		Description:      subject + " · " + amount + " · " + signalsPart,
		ImageURL:         h.imageURL(dto, loc),
		ImageAlt:         signalsPart + ". " + subject + ", " + amount + ".",
		URL:              cardURL,
		Version:          h.Version,
		SourcePresent:    sourcePresent,
		SourceURL:        source,
		SourceLabel:      h.Renderer.Text(loc, "link.source"),
		MethodologyURL:   cardURL, // методика достижима с карточки (бейдж → методика 5.3)
		MethodologyLabel: h.Renderer.Text(loc, "link.methodology"),
		ReportURL:        cardURL + "?report", // канал ошибки (Story 5.4 deep-link)
		ReportLabel:      h.Renderer.Text(loc, "link.report_error"),
	}
}

// signalsSummary — честная сводка сигналов карточки для OG (HTML + растр). Гардрейл честности (§7.4,
// урок A honesty-hole 5.1): «не оценивалось» НИКОГДА не выдаётся за «оценено-чисто».
//   - есть raised        → перечисление raised-флагов (нейтральная рамка, render.FlagLine);
//   - иначе insufficient  → «недостаточно сопоставимых данных» (неоценённое видно, не маскируется «чистым»);
//   - все not_raised      → «активных сигналов нет» (ДОКАЗАНО оценено и чисто).
func (h MetaHandler) signalsSummary(dto httpapi.ContractDTO, loc registry.Locale) string {
	var raised []string
	for _, f := range dto.Flags {
		if f.State == registry.FlagRaised {
			raised = append(raised, h.Renderer.FlagLine(f.FlagID, f.State, loc))
		}
	}
	if len(raised) > 0 {
		return strings.Join(raised, "; ")
	}
	return h.nonRaisedSummary(dto, loc)
}

// nonRaisedSummary — честный текст карточки БЕЗ raised-флагов. Хотя бы один insufficient/not_published
// (флаг НЕ оценён) → «недостаточно сопоставимых данных» (не показываем «активных сигналов нет», т.к. это
// читалось бы как «проверено — чисто»). Все not_raised → «активных сигналов нет» (реально оценено, чисто).
func (h MetaHandler) nonRaisedSummary(dto httpapi.ContractDTO, loc registry.Locale) string {
	for _, f := range dto.Flags {
		if f.State == registry.FlagInsufficientData || f.State == registry.FlagNotPublished {
			return h.Renderer.RenderValueState(registry.StateInsufficientSample, loc)
		}
	}
	return h.Renderer.Text(loc, "signals.none")
}

// subjectText — предмет на выбранной локали; при отсутствии — фолбэк на вторую локаль (лучше показать
// данные на другом языке, чем «нет данных»; lang-пометка fallback — задел internal/lang). Иначе honest no_data.
func (h MetaHandler) subjectText(dto httpapi.ContractDTO, loc registry.Locale) string {
	primary, secondary := dto.SubjectKk, dto.SubjectRu
	if loc == registry.RU {
		primary, secondary = dto.SubjectRu, dto.SubjectKk
	}
	if v, ok := fieldValue(primary); ok {
		return v
	}
	if v, ok := fieldValue(secondary); ok {
		return v
	}
	return h.noData(loc)
}

// amountText — деньги как у SPA; пустое/битое → честная per-field деградация в «нет данных» (НЕ «0 ₸»).
func (h MetaHandler) amountText(f httpapi.Field[string], loc registry.Locale) string {
	v, ok := fieldValue(f)
	if !ok {
		return h.noData(loc)
	}
	s, err := formatMoney(v)
	if err != nil {
		if h.Log != nil {
			h.Log.Error("og_money_format_failed", "error", err.Error(), "value", v)
		}
		return h.noData(loc)
	}
	return s
}

// imageURL — версионированный URL OG-картинки: cache-bust по methodology_version (+as_of при raised-флаге) —
// при смене методики URL меняется → соцсети сбрасывают кэш превью (AR-20). Сам PNG отдаёт будущий эндпоинт (T5).
func (h MetaHandler) imageURL(dto httpapi.ContractDTO, loc registry.Locale) string {
	q := url.Values{}
	if h.Version != "" {
		q.Set("v", h.Version) // cache-bust по methodology_version (AR-20)
	} else if h.Log != nil {
		// Честность над домыслом (honesty hole 5.1, фикс 94b68b8): ПУСТАЯ версия НЕ попадает в URL как
		// «v=» — не выдаём фабрикованную «версия есть»; cache-bust по версии отсутствует, пока версия неизвестна.
		h.Log.Warn("og_image_no_methodology_version", "goszakup_id", dto.GoszakupContractID)
	}
	q.Set("lang", string(loc))
	if asOf, ok := firstRaisedAsOf(dto); ok {
		q.Set("as_of", asOf)
	}
	return h.abs("/og/contracts/" + url.PathEscape(dto.GoszakupContractID) + "/image.png?" + q.Encode())
}

func (h MetaHandler) noData(loc registry.Locale) string {
	return h.Renderer.RenderValueState(registry.StateNoData, loc)
}

func (h MetaHandler) abs(path string) string {
	if h.BaseURL == "" {
		return path
	}
	return strings.TrimRight(h.BaseURL, "/") + path
}

// fieldValue — значение честного конверта только при state=ok и непустом value (иначе «нет значения»).
func fieldValue(f httpapi.Field[string]) (string, bool) {
	if f.State == registry.StateOK && f.Value != nil && strings.TrimSpace(*f.Value) != "" {
		return *f.Value, true
	}
	return "", false
}

// firstRaisedAsOf — detected_at первого raised-флага (для as_of в URL картинки).
func firstRaisedAsOf(dto httpapi.ContractDTO) (string, bool) {
	for _, f := range dto.Flags {
		if f.State == registry.FlagRaised {
			if v, ok := fieldValue(f.DetectedAt); ok {
				return v, true
			}
		}
	}
	return "", false
}

// writeError — честная HTML-страница ошибки: 404 → «нет данных»; 5xx → честное состояние «ошибка».
func (h MetaHandler) writeError(w http.ResponseWriter, status int, loc registry.Locale) {
	msg := h.noData(loc)
	if status >= 500 {
		msg = h.Renderer.RenderValueState(registry.StateError, loc)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = errTemplate.Execute(w, struct{ Lang, Message string }{string(loc), msg})
}

var ogTemplate = template.Must(template.New("og").Parse(`<!DOCTYPE html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<meta property="og:type" content="website">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
<meta property="og:image" content="{{.ImageURL}}">
<meta property="og:image:alt" content="{{.ImageAlt}}">
<meta property="og:locale" content="{{.OGLocale}}">
<meta property="og:url" content="{{.URL}}">
<meta name="aq:methodology_version" content="{{.Version}}">
</head>
<body>
<h1>{{.Title}}</h1>
<p>{{.Description}}</p>
<ul>
{{if .SourcePresent}}<li><a href="{{.SourceURL}}" rel="nofollow noopener" target="_blank">{{.SourceLabel}} ↗</a></li>{{end}}
<li><a href="{{.MethodologyURL}}">{{.MethodologyLabel}}</a></li>
<li><a href="{{.ReportURL}}">{{.ReportLabel}}</a></li>
</ul>
</body>
</html>
`))

var errTemplate = template.Must(template.New("ogerr").Parse(`<!DOCTYPE html>
<html lang="{{.Lang}}"><head><meta charset="utf-8"><title>{{.Message}}</title></head>
<body><p>{{.Message}}</p></body></html>
`))
