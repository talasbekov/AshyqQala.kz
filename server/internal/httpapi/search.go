package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/apierr"
	"ashyqqala/server/internal/normalize"
	"ashyqqala/server/internal/store/gen"
)

// Story 6.2 (FR-16): поиск по БИН и наименованию. Строится ПОВЕРХ фичи search (6.1): отдельный хендлер,
// единая гетерогенная выдача (организации → карточка подрядчика, контракты → карточка контракта) с
// дискриминатором kind. Токен-независим: работает на проекции organizations + contracts.

const (
	defaultSearchLimit = 20
	maxSearchLimit     = 100
	// minNameQueryLen — мин. длина терма для ветки поиска по ИМЕНИ (БИН — всегда ровно 12 цифр, точный).
	// 3 совпадает с реальным порогом включения trgm-индекса для ILIKE '%term%' (короче → seq-scan): ниже
	// минимума отвечаем 400 VALIDATION_FAILED, НЕ молчаливый скан всей таблицы (open-q №3, решение владельца).
	minNameQueryLen = 3
	// maxQueryLen — верхняя граница длины терма (рун): защита от unbounded ILIKE/similarity по всем строкам
	// (DoS-вектор; code review 6.2). Выше → 400, не запуск тяжёлого скана.
	maxQueryLen = 100

	kindOrganization = "organization"
	kindContract     = "contract"
)

// SearchStore — что нужно search-хендлеру от слоя данных (подмножество *gen.Queries; мокается в тестах).
type SearchStore interface {
	SearchOrganizations(ctx context.Context, arg gen.SearchOrganizationsParams) ([]gen.SearchOrganizationsRow, error)
	SearchContracts(ctx context.Context, arg gen.SearchContractsParams) ([]gen.SearchContractsRow, error)
}

// SearchOrgItem — результат-организация (kind=organization). Публичный id = natural bin → карточка подрядчика.
// has_geo — есть ли каноническая геоточка (Epic 3); до её наполнения честно false («без точки на карте»).
type SearchOrgItem struct {
	Kind    string        `json:"kind"`
	Bin     string        `json:"bin"`
	NameRu  Field[string] `json:"name_ru"`
	NameKk  Field[string] `json:"name_kk"`
	RegKato Field[string] `json:"reg_kato"`
	HasGeo  bool          `json:"has_geo"`
}

// SearchContractItem — результат-контракт (kind=contract). Форма ContractListItem (6.1) + kind + has_geo.
// Встраивание ContractListItem разворачивает его поля в JSON (goszakup_contract_id, subject_*, …, has_active_flag).
type SearchContractItem struct {
	Kind             string `json:"kind"`
	ContractListItem        // встроено: поля списка 6.1
	HasGeo           bool   `json:"has_geo"`
}

// SearchResponse — страница поиска. items — честный [] при пустоте (НЕ null). next_cursor — задел контракта
// (как 6.1); в MVP всегда null: relevance-порядок не keyset-дружелюбен, глубокая пагинация отложена (Dev Notes).
type SearchResponse struct {
	Items      []any   `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

func toSearchOrgItem(r gen.SearchOrganizationsRow) SearchOrgItem {
	return SearchOrgItem{
		Kind:    kindOrganization,
		Bin:     r.Bin,
		NameRu:  fromText(r.NameRu),
		NameKk:  fromText(r.NameKk),
		RegKato: fromText(r.RegKato),
		HasGeo:  false, // AC5: каноническая геоточка (Epic 3) ещё не наполнена — честно false, не выдуманная точка
	}
}

func toSearchContractItem(r gen.SearchContractsRow) SearchContractItem {
	return SearchContractItem{
		Kind: kindContract,
		ContractListItem: ContractListItem{
			GoszakupContractID: r.GoszakupContractID,
			SubjectRu:          fromText(r.SubjectRu),
			SubjectKk:          fromText(r.SubjectKk),
			AmountTng:          fromInt8String(r.AmountTng),
			SignDate:           fromDate(r.SignDate),
			Status:             fromText(r.Status),
			Direction:          fromText(r.Direction),
			KatoCode:           fromText(r.KatoCode),
			HasActiveFlag:      r.HasActiveFlag,
		},
		HasGeo: false, // AC5: см. toSearchOrgItem
	}
}

// SearchHandler — хендлер поиска (GET /api/search).
type SearchHandler struct {
	Store SearchStore
	Log   *slog.Logger
}

// Search обслуживает GET /api/search?q=<term>&limit=<n>.
//
// Ветвление (AC3): q канонизируется через normalize.CanonicalBIN. Валидный 12-зн БИН → ТОЛЬКО точный матч
// организации (мусорный «БИН» → CanonicalBIN="" → не порождает фантомную орг, уходит в ветку имени). Ветка
// имени: нечёткий substring по наименованию организаций + предмету контрактов, мин. длина терма 3 (ниже → 400,
// не seq-scan). Выдача (AC4): единый items с kind-дискриминатором, организации раньше контрактов, общий размер
// ≤ limit; next_cursor всегда null (relevance-порядок не keyset-дружелюбен — глубокая пагинация отложена).
func (h SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "q: обязательный непустой параметр")
		return
	}
	if utf8.RuneCountInString(q) > maxQueryLen {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "q: слишком длинный запрос (макс 100 символов)")
		return
	}

	// limit (дефолт 20, max 100) — как 6.1. Размер единой relevance-ранжированной выдачи.
	limit := defaultSearchLimit
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxSearchLimit {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "limit: целое в диапазоне [1,100]")
			return
		}
		limit = n
	}

	canonical := string(normalize.CanonicalBIN(q))

	items := make([]any, 0, limit)

	if canonical != "" {
		// БИН-ветка: точный матч организации (0..1 строка). Контракты по БИН не ищем (БИН — id организации).
		orgs, err := h.Store.SearchOrganizations(ctx, gen.SearchOrganizationsParams{
			BinExact: pgtype.Text{String: canonical, Valid: true},
			Q:        q,
			Lim:      int32(limit),
		})
		if err != nil {
			h.fail(w, "search_organizations_failed", err)
			return
		}
		for _, o := range orgs {
			items = append(items, toSearchOrgItem(o))
		}
		writeJSON(w, http.StatusOK, SearchResponse{Items: items, NextCursor: nil})
		return
	}

	// Ветка имени: нечёткий substring. Мин. длина (open-q №3): ниже минимума → 400 (не скан всей таблицы).
	if utf8.RuneCountInString(q) < minNameQueryLen {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "q: минимум 3 символа для поиска по наименованию")
		return
	}

	orgs, err := h.Store.SearchOrganizations(ctx, gen.SearchOrganizationsParams{
		BinExact: pgtype.Text{Valid: false}, // NULL ⇒ ветка ELSE (поиск по имени)
		Q:        q,
		Lim:      int32(limit),
	})
	if err != nil {
		h.fail(w, "search_organizations_failed", err)
		return
	}
	contracts, err := h.Store.SearchContracts(ctx, gen.SearchContractsParams{
		Q:   q,
		Lim: int32(limit),
	})
	if err != nil {
		h.fail(w, "search_contracts_failed", err)
		return
	}

	// Слияние round-robin (организация, контракт, организация, …) до limit — обе категории всегда видны,
	// контракты НЕ голодают за организациями (code review 6.2, решение владельца: round-robin interleave).
	// Каждая ветка уже упорядочена по релевантности со стабильным тай-брейком (search.sql); чередование
	// детерминировано. Когда одна ветка исчерпана, добор идёт из другой.
	for oi, ci := 0, 0; len(items) < limit && (oi < len(orgs) || ci < len(contracts)); {
		if oi < len(orgs) {
			items = append(items, toSearchOrgItem(orgs[oi]))
			oi++
		}
		if len(items) >= limit {
			break
		}
		if ci < len(contracts) {
			items = append(items, toSearchContractItem(contracts[ci]))
			ci++
		}
	}

	writeJSON(w, http.StatusOK, SearchResponse{Items: items, NextCursor: nil})
}

func (h SearchHandler) fail(w http.ResponseWriter, event string, err error) {
	if h.Log != nil {
		h.Log.Error(event, "error", err.Error())
	}
	apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
}
