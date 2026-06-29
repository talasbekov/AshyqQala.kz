package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/apierr"
	"ashyqqala/server/internal/normalize"
	"ashyqqala/server/internal/store/gen"
)

// Story 6.1 (FR-15): фасетная фильтрация списка контрактов. Отдельный хендлер/стор от карточки
// (ContractsHandler) — разная ответственность, разный набор запросов.

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

// validDirections — закрытый список оси «направление» (совпадает с CHECK в migrations/0002_projection.sql).
var validDirections = map[string]bool{"road": true, "water": true, "other": true}

// ContractsListStore — что list-хендлеру нужно от слоя данных (подмножество *gen.Queries; мокается в тестах).
type ContractsListStore interface {
	ListContracts(ctx context.Context, arg gen.ListContractsParams) ([]gen.ListContractsRow, error)
	ResolveSupplierOrgID(ctx context.Context, bin string) (int64, error)
}

// ContractListItem — компактный wire-айтем списка (snake_case; деньги строкой; nullable — честный конверт
// {value,state}). has_active_flag — «есть сигнал, требующий проверки» (нейтрально, AC6): есть ли активный
// contract-флаг. id (внутр. bigint) на проводе НЕ отдаём — публичный id = natural goszakup_contract_id.
type ContractListItem struct {
	GoszakupContractID string        `json:"goszakup_contract_id"`
	SubjectRu          Field[string] `json:"subject_ru"`
	SubjectKk          Field[string] `json:"subject_kk"`
	AmountTng          Field[string] `json:"amount_tng"`
	SignDate           Field[string] `json:"sign_date"`
	Status             Field[string] `json:"status"`
	Direction          Field[string] `json:"direction"`
	KatoCode           Field[string] `json:"kato_code"`
	HasActiveFlag      bool          `json:"has_active_flag"`
}

// ContractListResponse — страница: items ([] при пустоте, НЕ null — честная деградация контейнера на фронте,
// AC3) + next_cursor (null ⇒ страниц больше нет).
type ContractListResponse struct {
	Items      []ContractListItem `json:"items"`
	NextCursor *string            `json:"next_cursor"`
}

func toContractListItem(r gen.ListContractsRow) ContractListItem {
	return ContractListItem{
		GoszakupContractID: r.GoszakupContractID,
		SubjectRu:          fromText(r.SubjectRu),
		SubjectKk:          fromText(r.SubjectKk),
		AmountTng:          fromInt8String(r.AmountTng),
		SignDate:           fromDate(r.SignDate),
		Status:             fromText(r.Status),
		Direction:          fromText(r.Direction),
		KatoCode:           fromText(r.KatoCode),
		HasActiveFlag:      r.HasActiveFlag,
	}
}

// listCursor — keyset-курсор последней строки страницы: дата подписания ("" ⇒ NULL sign_date, т.е. хвост
// NULLS LAST) + её goszakup_contract_id (уникален, NOT NULL — стабильный тай-брейк).
type listCursor struct {
	SignDate string `json:"sd"`
	Gid      string `json:"gid"`
}

func encodeCursor(c listCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (listCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return listCursor{}, err
	}
	var c listCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return listCursor{}, err
	}
	if c.Gid == "" {
		return listCursor{}, errors.New("cursor без gid")
	}
	return c, nil
}

func parseDateParam(s string) (pgtype.Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func parseAmountParam(s string) (pgtype.Int8, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return pgtype.Int8{}, errors.New("amount должен быть целым ≥ 0")
	}
	return pgtype.Int8{Int64: n, Valid: true}, nil
}

// ContractsListHandler — хендлер фасетного списка контрактов (GET /api/contracts).
type ContractsListHandler struct {
	Store ContractsListStore
	Log   *slog.Logger
}

// List обслуживает GET /api/contracts — фасеты direction/period/amount/has_flag/supplier_bin + keyset.
// Невалидный параметр → 400 (не молчаливое игнорирование). Несуществующий/невалидный supplier_bin →
// честный пустой список (не 500: «нет такого подрядчика» ≠ ошибка сервера).
func (h ContractsListHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second) // per-request таймаут к БД
	defer cancel()
	q := r.URL.Query()

	params := gen.ListContractsParams{}

	// direction — comma-separated; OR внутри фасета (= ANY). Пустые отбрасываем; всё пусто → nil (нет фильтра,
	// НЕ '{}' — иначе ANY('{}') исключил бы всё).
	if raw := strings.TrimSpace(q.Get("direction")); raw != "" {
		var dirs []string
		for _, d := range strings.Split(raw, ",") {
			d = strings.TrimSpace(d)
			if d == "" {
				continue
			}
			if !validDirections[d] {
				apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "direction: ожидается road|water|other")
				return
			}
			dirs = append(dirs, d)
		}
		params.Directions = dirs
	}

	if v := strings.TrimSpace(q.Get("signed_from")); v != "" {
		d, err := parseDateParam(v)
		if err != nil {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "signed_from: дата в формате YYYY-MM-DD")
			return
		}
		params.SignedFrom = d
	}
	if v := strings.TrimSpace(q.Get("signed_to")); v != "" {
		d, err := parseDateParam(v)
		if err != nil {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "signed_to: дата в формате YYYY-MM-DD")
			return
		}
		params.SignedTo = d
	}
	// Кросс-полевая проверка: перевёрнутый диапазон дат → 400. Иначе SQL молча вернёт [], неотличимо от
	// настоящего «ничего не найдено» (фронт покажет «расширьте фильтр», скрыв реальную причину). [code review 6.1]
	if params.SignedFrom.Valid && params.SignedTo.Valid && params.SignedFrom.Time.After(params.SignedTo.Time) {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "signed_from не может быть позже signed_to")
		return
	}

	if v := strings.TrimSpace(q.Get("amount_min")); v != "" {
		n, err := parseAmountParam(v)
		if err != nil {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "amount_min: целое ≥ 0")
			return
		}
		params.AmountMin = n
	}
	if v := strings.TrimSpace(q.Get("amount_max")); v != "" {
		n, err := parseAmountParam(v)
		if err != nil {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "amount_max: целое ≥ 0")
			return
		}
		params.AmountMax = n
	}
	// Кросс-полевая проверка: amount_min > amount_max → 400 (см. выше про даты — не маскируем перевёрнутый
	// диапазон под пустой результат). [code review 6.1]
	if params.AmountMin.Valid && params.AmountMax.Valid && params.AmountMin.Int64 > params.AmountMax.Int64 {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "amount_min не может быть больше amount_max")
		return
	}

	if v := strings.TrimSpace(q.Get("has_flag")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "has_flag: true|false")
			return
		}
		params.HasFlagOnly = b
	}

	// limit (дефолт 20, max 100). Запрашиваем pageSize+1 — лишняя строка сигналит «есть следующая страница».
	pageSize := defaultListLimit
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxListLimit {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "limit: целое в диапазоне [1,100]")
			return
		}
		pageSize = n
	}
	params.Lim = int32(pageSize) + 1

	// cursor — непрозрачный keyset-курсор. Битый → 400 (CodeInvalidCursor). Пустая дата в курсоре ⇒ NULL-хвост.
	if v := strings.TrimSpace(q.Get("cursor")); v != "" {
		c, err := decodeCursor(v)
		if err != nil {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidCursor, "невалидный cursor")
			return
		}
		params.CursorGid = pgtype.Text{String: c.Gid, Valid: true}
		if c.SignDate != "" {
			d, derr := parseDateParam(c.SignDate)
			if derr != nil {
				apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidCursor, "невалидный cursor (дата)")
				return
			}
			params.CursorSd = d
		}
	}

	// supplier_bin → канонизация (12 цифр) → резолв в supplier_org_id. Невалидный/несуществующий → честно
	// пустой результат (не выдумываем и не 500). Резолв (вкл. early-return 200 + поход в БД) выполняем
	// ПОСЛЕДНИМ — после валидации всех чистых параметров: иначе битый limit/cursor/диапазон, поданный вместе
	// с несуществующим supplier_bin, молча дал бы 200 [] вместо 400 (непоследовательный контракт), плюс тратил
	// бы DB-резолв на заведомо невалидный запрос. [code review 6.1]
	if v := strings.TrimSpace(q.Get("supplier_bin")); v != "" {
		bin := string(normalize.CanonicalBIN(v))
		if bin == "" {
			writeJSON(w, http.StatusOK, ContractListResponse{Items: []ContractListItem{}, NextCursor: nil})
			return
		}
		orgID, err := h.Store.ResolveSupplierOrgID(ctx, bin)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			writeJSON(w, http.StatusOK, ContractListResponse{Items: []ContractListItem{}, NextCursor: nil})
			return
		case err != nil:
			if h.Log != nil {
				h.Log.Error("resolve_supplier_org_failed", "error", err.Error())
			}
			apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
			return
		}
		params.SupplierOrgID = pgtype.Int8{Int64: orgID, Valid: true}
	}

	rows, err := h.Store.ListContracts(ctx, params)
	if err != nil {
		if h.Log != nil {
			h.Log.Error("list_contracts_failed", "error", err.Error())
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
		return
	}

	// keyset: запросили pageSize+1; пришло больше pageSize ⇒ есть следующая страница. Курсор = последняя
	// строка ТЕКУЩЕЙ страницы; лишнюю строку отбрасываем (она вернётся первой на следующей странице).
	var next *string
	if len(rows) > pageSize {
		last := rows[pageSize-1]
		cur := listCursor{Gid: last.GoszakupContractID}
		if last.SignDate.Valid {
			cur.SignDate = last.SignDate.Time.Format("2006-01-02")
		}
		s := encodeCursor(cur)
		next = &s
		rows = rows[:pageSize]
	}

	items := make([]ContractListItem, 0, len(rows)) // [] (не null) при пустоте — честная деградация на фронте
	for _, row := range rows {
		items = append(items, toContractListItem(row))
	}
	writeJSON(w, http.StatusOK, ContractListResponse{Items: items, NextCursor: next})
}
