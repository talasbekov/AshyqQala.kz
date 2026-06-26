package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/apierr"
	"ashyqqala/server/internal/store/gen"
)

// ErrorReportsHandler — ПЕРВЫЙ write-эндпоинт приложения (Story 5.4, FR-28): публичный безаккаунтный
// досудебный канал «сообщить об ошибке» (§7.1). Принимает обращение и кладёт в очередь error_reports
// (Directus читает/триажит). Защита публичного write: honeypot + лимит тела + in-memory rate-limit per-IP
// (без новых зависимостей — решение владельца 2026-06-26). Токен goszakup НЕ нужен.
type ErrorReportsHandler struct {
	Store   ErrorReportStore
	Log     *slog.Logger
	limiter *rateLimiter
}

// ErrorReportStore — что хендлеру нужно от слоя данных (реализует *gen.Queries; мокается в тестах).
type ErrorReportStore interface {
	InsertErrorReport(ctx context.Context, arg gen.InsertErrorReportParams) (gen.InsertErrorReportRow, error)
}

// NewErrorReportsHandler — хендлер с дефолтным rate-limit (20 обращений / час / IP — щедро для человека,
// глушит флуд). Лимитер можно переопределить в тестах (white-box, тот же пакет).
func NewErrorReportsHandler(store ErrorReportStore, log *slog.Logger) *ErrorReportsHandler {
	return &ErrorReportsHandler{Store: store, Log: log, limiter: newRateLimiter(20, time.Hour)}
}

// Лимиты входа (страховка: CHECK error_reports_message_nonempty_chk дублирует непустоту на уровне БД).
const (
	maxBodyBytes     = 16 << 10 // 16 КБ тела (http.MaxBytesReader)
	maxMessageLen    = 5000
	maxContactLen    = 200
	maxSubjectRefLen = 128
	maxSourceURLLen  = 512
)

// Доменные enum (зеркало CHECK-констрейнтов миграции 0011 + OpenAPI).
var (
	errReportKinds        = map[string]bool{"data_error": true, "flag_error": true, "geo_wrong_point": true}
	errReportSubjectTypes = map[string]bool{"contract": true, "contractor": true, "geo_object": true}
)

// errorReportRequest — wire-форма обращения (snake_case). website — HONEYPOT: настоящие люди его не видят/не
// заполняют (скрыт CSS); непустое значение = бот → молча принимаем без записи (не подсказываем боту).
type errorReportRequest struct {
	Kind        string `json:"kind"`
	SubjectType string `json:"subject_type"`
	SubjectRef  string `json:"subject_ref"`
	Message     string `json:"message"`
	Contact     string `json:"contact"`
	SourceURL   string `json:"source_url"`
	LeaveBlank  string `json:"leave_blank"` // honeypot — неавтозаполняемое имя; должно быть пустым
}

// errorReportResponse — честный ответ: принято + id (без публичного трекинга статуса — PRD §5.11).
type errorReportResponse struct {
	ID     int64  `json:"id,omitempty"`
	Status string `json:"status"` // "received"
}

// Create обслуживает POST /api/error-reports.
func (h *ErrorReportsHandler) Create(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !h.limiter.allow(ip) {
		apierr.Write(w, http.StatusTooManyRequests, apierr.CodeRateLimited,
			"слишком много обращений — попробуйте позже")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // контракт additionalProperties:false — лишние поля = 400 (парность с OpenAPI)
	var req errorReportRequest
	if err := dec.Decode(&req); err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "некорректное тело запроса")
		return
	}

	// Honeypot: бот заполнил скрытое поле → молча «принято» (201), но НЕ пишем в очередь и не подсказываем.
	if strings.TrimSpace(req.LeaveBlank) != "" {
		if h.Log != nil {
			h.Log.Warn("error_report_honeypot", "ip", ip)
		}
		writeJSON(w, http.StatusCreated, errorReportResponse{Status: "received"})
		return
	}

	if msg, ok := validateErrorReport(&req); !ok {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, msg)
		return
	}

	row, err := h.Store.InsertErrorReport(r.Context(), gen.InsertErrorReportParams{
		Kind:        req.Kind,
		SubjectType: req.SubjectType,
		SubjectRef:  strings.TrimSpace(req.SubjectRef),
		Message:     strings.TrimSpace(req.Message),
		Contact:     optText(req.Contact),
		SourceUrl:   optText(req.SourceURL),
	})
	if err != nil {
		if h.Log != nil {
			h.Log.Error("error_report_insert_failed", "error", err.Error())
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "не удалось принять обращение")
		return
	}

	writeJSON(w, http.StatusCreated, errorReportResponse{ID: row.ID, Status: "received"})
}

// validateErrorReport — строгая валидация входа. Возвращает (сообщение об ошибке, ok). Нейтрально (§7.1).
func validateErrorReport(req *errorReportRequest) (string, bool) {
	if !errReportKinds[req.Kind] {
		return "неизвестный вид обращения", false
	}
	if !errReportSubjectTypes[req.SubjectType] {
		return "неизвестный тип объекта", false
	}
	ref := strings.TrimSpace(req.SubjectRef)
	if ref == "" || utf8.RuneCountInString(ref) > maxSubjectRefLen {
		return "некорректная ссылка на объект", false
	}
	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		return "опишите, что не так", false
	}
	// Длина в СИМВОЛАХ, не байтах: кириллица 2 байта/символ; синхронно с клиентом и OpenAPI (maxLength — символы).
	if utf8.RuneCountInString(msg) > maxMessageLen {
		return "сообщение слишком длинное", false
	}
	if utf8.RuneCountInString(req.Contact) > maxContactLen {
		return "контакт слишком длинный", false
	}
	if utf8.RuneCountInString(req.SourceURL) > maxSourceURLLen {
		return "ссылка слишком длинная", false
	}
	return "", true
}

// optText — опциональное текстовое поле → pgtype.Text (пусто/пробелы → NULL, не выдумываем «»).
func optText(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// clientIP — реальный IP клиента. За единственным Caddy reverse-proxy реальный клиент — ПОСЛЕДНИЙ хоп
// X-Forwarded-For (Caddy дописывает его справа; любые ЛЕВЫЕ значения подделываемы клиентом — по ним нельзя
// лимитировать). Берём правый непустой токен; иначе RemoteAddr. (Доверять XFF можно лишь потому, что
// публичный вход — единственный наш прокси; при прямом доступе XFF подделываем — см. deferred: trusted_proxies.)
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			if ip := strings.TrimSpace(parts[i]); ip != "" {
				return ip
			}
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// rateLimiter — простой in-memory sliding-window лимитер per-key (IP). Без зависимостей. Защёлкивает флуд
// публичного write-канала. Память: карта растёт по числу уникальных IP в окне — для MVP-масштаба ок
// (defer: периодическая чистка/LRU при росте трафика).
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
	now    func() time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{hits: map[string][]time.Time{}, limit: limit, window: window, now: time.Now}
}

// allow — true, если в окне меньше limit обращений с этого ключа; иначе false (и НЕ засчитывает текущее).
func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := rl.now().Add(-rl.window)
	kept := rl.hits[key][:0]
	for _, t := range rl.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.limit {
		rl.hits[key] = kept
		return false
	}
	rl.hits[key] = append(kept, rl.now())
	return true
}
